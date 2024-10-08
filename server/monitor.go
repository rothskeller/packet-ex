package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
)

//go:embed monitor.html
var monitorHTML []byte

type ManualTrigger struct {
	Type    definition.EventType
	Station string
	Name    string
}
type eventID struct {
	Type    definition.EventType
	Station string
	Name    string
}
type Monitor struct {
	def  *definition.Definition
	st   *state.State
	mtch chan<- ManualTrigger
	// groups is the ordered list of event group names.  The first one is
	// always "UNKNOWN".  The empty string, if present at all, is always
	// last.
	groups []string
	// idle is the set of timers belonging to connections in idle wait.
	idle map[*time.Timer]struct{}
	// conns is the per-connection set of events for which updates need to
	// be sent to that connection.
	conns map[*websocket.Conn]map[eventID]struct{}
	// smap maps from station name (including "UNKNOWN") to grid column
	// number.
	smap map[string]int
	// emap maps from event type and message name to a group number and row
	// number within that group.
	emap map[definition.EventType]map[string]gr
	// events maps from event characteristics to the actual event object for
	// all events.
	events map[eventID]*state.Event
	// unknown is the per-station list of unknown messages received
	// (actually, the list of "reject" events for same).
	unknown map[string][]*state.Event
	// cheads, rheads, and grid are the static content for the column
	// headings, row headings, and grid area, respectively.
	cheads string
	rheads string
	grid   string
	// mutex controls all access to anything in the structure.
	mutex sync.Mutex
}
type gr struct{ g, r int }

// NewMonitor creates a new sub-server for rendering monitor pages.
// mtch is the channel onto which the server should write any manual triggers
// invoked by the user.
func NewMonitor(def *definition.Definition, st *state.State, mtch chan<- ManualTrigger) (m *Monitor) {
	var (
		wantReceipts bool
	)
	m = &Monitor{
		def:     def,
		st:      st,
		mtch:    mtch,
		idle:    make(map[*time.Timer]struct{}),
		conns:   make(map[*websocket.Conn]map[eventID]struct{}),
		events:  make(map[eventID]*state.Event),
		unknown: make(map[string][]*state.Event),
	}
	// Build the maps and grid.
	wantReceipts = m.buildStationMap()
	m.buildGroupList()
	m.buildEventMap(wantReceipts)
	// Register the web server handlers.
	http.Handle("/{$}", http.HandlerFunc(m.ServeHTTP))
	http.Handle("/ws", http.HandlerFunc(m.ServeWS))
	http.Handle("POST /manualTrigger", http.HandlerFunc(m.serveManualTrigger))
	return m
}

// buildStationMap builds a map from station name to column number, and also
// renders the column headings.  It returns whether there are any stations that
// expect delivery receipts.
func (m *Monitor) buildStationMap() (wantReceipts bool) {
	var sb strings.Builder

	// Build the new station map and column headings.
	m.smap = make(map[string]int)
	sb.WriteString(`<div class="column unknownStation">UNKNOWN</div>`)
	m.smap["UNKNOWN"] = 0
	for i, s := range m.def.Stations {
		m.smap[s.CallSign] = i + 1
		if s.ReceiptDelay != 0 {
			wantReceipts = true
		}
		if s.FCCCall != "" && s.FCCCall != s.CallSign {
			fmt.Fprintf(&sb, `<div class=column>%s<div class=fcc>%s</div></div>`, s.CallSign, s.FCCCall)
		} else {
			fmt.Fprintf(&sb, `<div class=column>%s</div>`, s.CallSign)
		}
	}
	m.cheads = sb.String()
	return wantReceipts
}

// buildGroupList scans the defined events and builds a list of the event groups
// in the order they appear.  UNKNOWN is always first even if not explicitly
// referenced (which it usually isn't).  "" is always last, if referenced.
func (m *Monitor) buildGroupList() {
	var seen = make(map[string]bool)

	m.groups = append(m.groups[:0], "UNKNOWN")
	seen["UNKNOWN"] = true
	for _, e := range m.def.Events {
		if e.Group != "" && !seen[e.Group] {
			m.groups = append(m.groups, e.Group)
			seen[e.Group] = true
		}
	}
	if slices.ContainsFunc(m.def.Events, func(e *definition.Event) bool { return e.Group == "" }) {
		m.groups = append(m.groups, "")
	}
}

// buildEventMap builds the map from event type and name to location in the
// grid.  It also generates the row headings.
func (m *Monitor) buildEventMap(wantReceipts bool) {
	var rsb strings.Builder
	var gsb strings.Builder

	// Build the new event map and row headings.
	m.emap = make(map[definition.EventType]map[string]gr)
	for g, group := range m.groups {
		m.buildEventMapGroup(&rsb, &gsb, g, group, wantReceipts)
	}
	m.rheads = rsb.String()
	m.grid = gsb.String()
}

// buildEventMapGroup populates the event map for the events in a particular
// group, and generates the row headings for that group's events.
func (m *Monitor) buildEventMapGroup(rsb, gsb *strings.Builder, g int, group string, wantReceipts bool) {
	var events []*definition.Event

	// Get the set of events in the group.  Autogenerate "reject UNKNOWN" at
	// the start of the UNKNOWN group, and "receipt XXX" after any "send
	// XXX" when wantReceipts is true.
	if group == "UNKNOWN" {
		events = append(events, &definition.Event{Group: group, Type: definition.EventReject, Name: group})
	}
	for _, e := range m.def.Events {
		if e.Group == group {
			events = append(events, e)
			if wantReceipts && e.Type == definition.EventSend {
				e2 := *e
				e2.Type = definition.EventReceipt
				events = append(events, &e2)
			}
		}
	}
	// Populate the map for the events in the group.
	for i, e := range events {
		if m.emap[e.Type] == nil {
			m.emap[e.Type] = make(map[string]gr)
		}
		m.emap[e.Type][e.Name] = gr{g, i}
	}
	// Generate the row headings.
	if group == "UNKNOWN" {
		fmt.Fprintf(rsb, `<div class="group unknownMessage" style="--span:%d"><div class=groupName>?</div>`, len(events))
		gsb.WriteString(`<div class="group unknownMessage">`)
	} else if group != "" {
		fmt.Fprintf(rsb, `<div class=group style="--span:%d"><div class=groupName>%s</div>`, len(events), html.EscapeString(group))
		gsb.WriteString(`<div class=group>`)
	} else {
		rsb.WriteString(`<div class=group>`)
		gsb.WriteString(`<div class=group>`)
	}
	for i, e := range events {
		if i != 0 && events[i-1].Group == e.Group && events[i-1].Name == e.Name {
			fmt.Fprintf(rsb, `<div class="eventName ditto">"</div>`)
		} else {
			fmt.Fprintf(rsb, `<div class=eventName>%s</div>`, html.EscapeString(e.Name))
		}
		fmt.Fprintf(rsb, `<div class=eventType>%s</div>`, e.Type)
		gsb.WriteString(`<div class=event><div class="cell unknownStation"></div>`)
		for range m.def.Stations {
			gsb.WriteString(`<div class=cell></div>`)
		}
		gsb.WriteString(`</div>`)
	}
	rsb.WriteString(`</div>`)
	gsb.WriteString(`</div>`)
}

// OnEventChange receives notification of a new or updated event, and uses it to
// update the monitor.
func (m *Monitor) OnEventChange(e *state.Event) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Save the event in our internal table.
	eid, ok := m.saveEvent(e)
	if !ok {
		return // not an event we're tracking
	}
	// Notify all connection handlers that a particular table cell needs
	// update.
	for _, conn := range m.conns {
		conn[eid] = struct{}{}
	}
	// For all connections that are in idle timeout, reset their timer to
	// the debounce time.
	for timer := range m.idle {
		timer.Reset(debounceTime)
		delete(m.idle, timer)
	}
}

// saveEvent updates the monitor server's internal cache of events.
func (m *Monitor) saveEvent(e *state.Event) (eid eventID, ok bool) {
	// Special case handling for unknown messages.  Add them to the
	// per-station unknown message list, and return the cell that list gets
	// shown in.
	if e.Station() == "UNKNOWN" || e.Name() == "UNKNOWN" {
		if !slices.Contains(m.unknown[e.Station()], e) {
			m.unknown[e.Station()] = append(m.unknown[e.Station()], e)
		}
		return eventID{e.Type(), e.Station(), "UNKNOWN"}, true
	}
	// Otherwise, add the event to the map, replacing anything already
	// there (which ought to be the same event anyway).
	eid = eventID{e.Type(), e.Station(), e.Name()}
	m.events[eid] = e
	return eid, true
}

// ServeHTTP serves the page HTML.
func (m *Monitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "nostore")
	w.Write(monitorHTML)
}

// ServeWS accepts and serves the websocket connection from the page.
func (m *Monitor) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"monitor"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: websocket accept: %s\n", err)
		return
	}
	m.mutex.Lock()
	m.conns[conn] = make(map[eventID]struct{})
	m.mutex.Unlock()
	go m.followEvents(conn)
}

// followEvents is the goroutine that sends updates to a client over its
// websocket.
func (m *Monitor) followEvents(conn *websocket.Conn) {
	var timer = time.NewTimer(time.Millisecond) // send first "update" immediately
	var first = true
	for range timer.C {
		var buf []byte

		// Render the data.
		if first {
			buf = m.renderInitial()
			first = false
		} else {
			// Figure out what needs to be sent to this client.
			m.mutex.Lock()
			tosend := slices.Collect(maps.Keys(m.conns[conn]))
			clear(m.conns[conn])
			m.mutex.Unlock()
			buf = m.renderUpdate(tosend)
		}
		// Send the data.
		err := conn.Write(context.Background(), websocket.MessageText, buf)
		m.mutex.Lock()
		if err != nil {
			// Send failed.  Remove the connection from our list.
			delete(m.conns, conn)
			delete(m.idle, timer)
			m.mutex.Unlock()
			fmt.Fprintf(os.Stderr, "ERROR: websocket write: %s\n", err)
			return
		}
		// If there's new stuff to send already, set a debounce timer,
		// otherwise set an idle timer.
		if len(m.conns[conn]) != 0 {
			timer.Reset(debounceTime)
			delete(m.idle, timer)
		} else {
			timer.Reset(keepAliveTime)
			m.idle[timer] = struct{}{}
		}
		m.mutex.Unlock()
	}
}

// updateEntry is the entry for a single event in the monitor table.  The field
// names are short to reduce JSON encoding overhead.  G, R, and C are the group
// number, row number within the group, and column number of the event, all
// zero-based.  H is the innerHTML for the cell at that location.  S is the
// severity class to be applied to that cell.
type updateEntry struct {
	G, R, C int
	H, S    string
}

// update is the structure of the JSON data we send to a client over the
// websocket.
type update struct {
	// All updates contain the current time of day.
	Clock string
	// Title is the monitor title bar.  Its presence indicates that this is
	// a first-time update.
	Title string `json:",omitempty"`
	// RHeads is the innerHTML of the row headings.  It is present in a
	// first-time update only.
	RHeads string `json:",omitempty"`
	// CHeads is the innerHTML of the column headings.  It is present in a
	// first-time update only.
	CHeads string `json:",omitempty"`
	// Grid is the initial innerHTML of the cell grid, creating its
	// framework.  It is present in a first-time update only.
	Grid string `json:",omitempty"`
	// Cells is the set of cells whose content needs to be updated.  In a
	// first-time update, it will be all cells with content.
	Cells []*updateEntry
}

// renderInitial renders the first-time update.
func (m *Monitor) renderInitial() (buf []byte) {
	var update update

	update.Clock = m.st.Now().Format("15:04")
	update.Title = fmt.Sprintf("%s %s", m.def.Exercise.Activation, m.def.Exercise.Incident)
	update.RHeads = m.rheads
	update.CHeads = m.cheads
	update.Grid = m.grid
	for stn := range m.smap {
		for etype, etm := range m.emap {
			for name := range etm {
				if ue := m.renderEvent(eventID{etype, stn, name}); ue != nil {
					update.Cells = append(update.Cells, ue)
				}
			}
		}
	}
	buf, _ = json.Marshal(update)
	return buf
}

// renderUpdate renders a non-first-time update of the specified cells.
func (m *Monitor) renderUpdate(cells []eventID) (buf []byte) {
	var update update
	update.Clock = m.st.Now().Format("15:04")
	for _, eid := range cells {
		if ue := m.renderEvent(eid); ue != nil {
			update.Cells = append(update.Cells, ue)
		}
	}
	buf, _ = json.Marshal(update)
	return buf
}

// renderEvent renders a single cell.
func (m *Monitor) renderEvent(eid eventID) (ue *updateEntry) {
	var (
		gr  gr
		col int
		sb  strings.Builder
		e   *state.Event
		sev string
		ok  bool
	)
	// Unknown message cells are handled specially.
	if eid.Name == "UNKNOWN" {
		return m.renderUnknownEvents(eid.Station)
	}
	// We should have a column for the station, but if somehow we don't,
	// skip it.  Also skip cells in the unknown station column.
	if col, ok = m.smap[eid.Station]; !ok || col == 0 {
		return nil
	}
	// We should have a grid location for the event, but if somehow we
	// don't, skip it.
	if emap, ok := m.emap[eid.Type]; !ok {
		return nil
	} else if gr, ok = emap[eid.Name]; !ok {
		return nil
	}
	e = m.events[eid]
	switch {
	case e == nil:
		// This event has neither occurred nor been scheduled.  We'll
		// skip it, unless it's one that's supposed to be manually
		// triggered.
		if edef := m.def.Event(eid.Type, eid.Name); edef == nil || edef.TriggerType != definition.EventManual {
			return nil
		}
		sev = "pending"
		sb.WriteString(`<svg><use href="#clock"/></svg> MANUAL`)
	case slices.ContainsFunc(e.Notes(), func(s string) bool { return strings.HasPrefix(s, "ERROR:") }):
		sev = "error"
		sb.WriteString(`<svg><use href="#cross"/></svg> ERROR`)
	case e.Score() != 0 && e.Score() < 90:
		sev = "error"
		fmt.Fprintf(&sb, `<svg><use href="#cross"/></svg> %d%%`, e.Score())
	case e.Overdue() && !e.Occurred().IsZero():
		sev = "error"
		sb.WriteString(`<svg><use href="#cross"/></svg> LATE`)
	case e.Overdue():
		sev = "error"
		sb.WriteString(`<svg><use href="#clock"/></svg> OVERDUE`)
	case slices.ContainsFunc(e.Notes(), func(s string) bool { return strings.HasPrefix(s, "WARNING:") }):
		sev = "warning"
		sb.WriteString(`<svg><use href="#warning"/></svg> WARNING`)
	case e.Score() != 0 && e.Score() != 100:
		sev = "warning"
		fmt.Fprintf(&sb, `<svg><use href="#warning"/></svg> %d%%`, e.Score())
	case e.Occurred().IsZero():
		sev = "pending"
		switch e.Type() {
		case definition.EventBulletin, definition.EventInject, definition.EventSend:
			fmt.Fprintf(&sb, `<svg><use href="#clock"/></svg> at %s`, e.Expected().Format("15:04"))
		default:
			fmt.Fprintf(&sb, `<svg><use href="#clock"/></svg> by %s`, e.Expected().Format("15:04"))
		}
	default:
		sev = "success"
		fmt.Fprintf(&sb, `<svg><use href="#check"/></svg> at %s`, e.Occurred().Format("15:04"))
	}
	m.renderDialog(&sb, eid, e)
	return &updateEntry{G: gr.g, R: gr.r, C: col, H: sb.String(), S: sev}
}

// renderUnknownEvents renders the unknown messages cell for a station.
func (m *Monitor) renderUnknownEvents(stn string) (ue *updateEntry) {
	unk := m.unknown[stn]
	if len(unk) == 0 {
		// No unknown events for this station, so nothing to render.
		return nil
	}
	ue = &updateEntry{G: 0, R: 0, C: m.smap[stn], S: "error"}
	var sb strings.Builder
	sb.WriteString(`<svg><use href="#cross"/></svg> `)
	if len(unk) == 1 {
		sb.WriteString(`1 msg`)
	} else {
		fmt.Fprintf(&sb, `%d msgs`, len(unk))
	}
	sb.WriteString(`<div class=dialog style=display:none><p>`)
	if sd := m.def.Station(stn); sd == nil {
		if len(unk) == 1 {
			sb.WriteString(`The engine has received 1 message from an unknown station.`)
		} else {
			fmt.Fprintf(&sb, `The engine has received %d messages from unknown stations.`, len(unk))
		}
	} else {
		if len(unk) == 1 {
			sb.WriteString(`The engine has received 1 unrecognized message from `)
			m.renderStation(&sb, sd)
			sb.WriteByte('.')
		} else {
			fmt.Fprintf(&sb, `The engine has received %d unrecognized messages from `, len(unk))
			m.renderStation(&sb, sd)
			sb.WriteByte('.')
		}
	}
	for _, e := range unk {
		m.renderViewButton(&sb, "View "+e.LMI(), e.LMI())
	}
	sb.WriteString(`</div>`)
	ue.H = sb.String()
	return ue
}
