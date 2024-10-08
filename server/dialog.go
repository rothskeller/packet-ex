package server

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
)

// renderDialog renderes the contents of the popup dialog for an event.
func (m *Monitor) renderDialog(sb *strings.Builder, eid eventID, e *state.Event) {
	var stn = m.def.Station(eid.Station)

	sb.WriteString(`<div class=dialog style=display:none><p>`)
	switch eid.Type {
	case definition.EventAlert:
		m.renderStation(sb, stn)
		if !e.Occurred().IsZero() {
			sb.WriteString(` alerted `)
		} else if e.Overdue() {
			sb.WriteString(` should have alerted `)
		} else {
			sb.WriteString(` should alert `)
		}
		sb.WriteString(m.def.Exercise.MyCall)
		sb.WriteString(` by voice `)
		if !e.Occurred().IsZero() {
			sb.WriteString(`at `)
			m.renderTime(sb, e.Occurred())
		} else {
			sb.WriteString(`by `)
			m.renderTime(sb, e.Expected())
			m.renderExpectedReason(sb, eid)
		}
		sb.WriteString(` that message `)
		sb.WriteString(html.EscapeString(eid.Name))
		sb.WriteString(` was transmitted.`)
		if e.Overdue() {
			if !e.Occurred().IsZero() {
				sb.WriteString(`  This alert was expected by `)
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
				sb.WriteString(` and was `)
				m.renderDuration(sb, e.Occurred().Sub(e.Expected()))
				sb.WriteString(` late.`)
			} else {
				sb.WriteString(`  This alert is overdue.`)
			}
		}
		m.renderNotes(sb, e)
		if e.Occurred().IsZero() {
			m.renderManualTriggerButton(sb, eid.Type, stn.CallSign, eid.Name, "Record Alert")
		}
	case definition.EventBulletin:
		sb.WriteString(m.def.Exercise.MyCall)
		if e != nil && !e.Occurred().IsZero() {
			sb.WriteString(` posted`)
		} else {
			sb.WriteString(` will post`)
		}
		sb.WriteString(` the bulletin `)
		sb.WriteString(eid.Name)
		if e == nil {
			sb.WriteString(` on request.`)
		} else {
			sb.WriteString(` at `)
			if e.Occurred().IsZero() {
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
			} else {
				m.renderTime(sb, e.Occurred())
			}
			sb.WriteByte('.')
		}
		m.renderNotes(sb, e)
		if e != nil && e.LMI() != "" {
			m.renderViewButton(sb, "View Bulletin", e.LMI())
		} else {
			m.renderManualTriggerButton(sb, eid.Type, "", eid.Name, "Post Bulletin Now")
		}
	case definition.EventDeliver:
		m.renderStation(sb, stn)
		if !e.Occurred().IsZero() {
			sb.WriteString(` printed and delivered `)
		} else if e.Overdue() {
			sb.WriteString(` should have printed and delivered `)
		} else {
			sb.WriteString(` should print and deliver `)
		}
		sb.WriteString(e.LMI())
		sb.WriteString(` (`)
		sb.WriteString(html.EscapeString(eid.Name))
		sb.WriteString(`) to their principal `)
		if !e.Occurred().IsZero() {
			sb.WriteString(`at `)
			m.renderTime(sb, e.Occurred())
		} else {
			sb.WriteString(`by `)
			m.renderTime(sb, e.Expected())
			m.renderExpectedReason(sb, eid)
		}
		sb.WriteByte('.')
		if e.Overdue() {
			if !e.Occurred().IsZero() {
				sb.WriteString(`  This was expected by `)
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
				sb.WriteString(` and was `)
				m.renderDuration(sb, e.Occurred().Sub(e.Expected()))
				sb.WriteString(` late.`)
			} else {
				sb.WriteString(`  This is overdue.`)
			}
		}
		m.renderNotes(sb, e)
		if e.Occurred().IsZero() {
			m.renderManualTriggerButton(sb, eid.Type, stn.CallSign, eid.Name, "Record Delivery to Principal")
		}
	case definition.EventInject:
		sb.WriteString(`The exercise engine `)
		if e != nil && !e.Occurred().IsZero() {
			sb.WriteString(`injected `)
		} else {
			sb.WriteString(`will inject `)
		}
		sb.WriteString(` the message `)
		sb.WriteString(html.EscapeString(eid.Name))
		if e == nil {
			sb.WriteString(` on request`)
		} else {
			sb.WriteString(` at `)
			if e.Occurred().IsZero() {
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
			} else {
				m.renderTime(sb, e.Occurred())
			}
		}
		sb.WriteString(`, for `)
		m.renderStation(sb, stn)
		sb.WriteString(` to send to `)
		sb.WriteString(m.def.Exercise.MyCall)
		sb.WriteByte('.')
		m.renderNotes(sb, e)
		if e != nil && !e.Occurred().IsZero() {
			m.renderViewButton(sb, "View Message", fmt.Sprintf("INJ-%03dI", e.ID()))
		} else {
			m.renderManualTriggerButton(sb, eid.Type, stn.CallSign, eid.Name, "Inject Message Now")
		}
	case definition.EventReceipt:
		var se = m.st.GetEvent(e.Trigger())
		sb.WriteString(`A delivery receipt for `)
		sb.WriteString(se.LMI())
		sb.WriteString(` (`)
		sb.WriteString(html.EscapeString(e.Name()))
		sb.WriteString(`) `)
		if !e.Occurred().IsZero() {
			sb.WriteString(`was received `)
		} else {
			sb.WriteString(`is expected `)
		}
		sb.WriteString(`from `)
		m.renderStation(sb, stn)
		if !e.Occurred().IsZero() {
			sb.WriteString(`at `)
			m.renderTime(sb, e.Occurred())
		} else {
			sb.WriteString(`by `)
			m.renderTime(sb, e.Expected())
			m.renderExpectedReason(sb, eid)
		}
		sb.WriteByte('.')
		if e.Overdue() {
			if !e.Occurred().IsZero() {
				sb.WriteString(`  It was expected by `)
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
				sb.WriteString(` and was `)
				m.renderDuration(sb, e.Occurred().Sub(e.Expected()))
				sb.WriteString(` late.`)
			} else {
				sb.WriteString(`  It is overdue.`)
			}
		}
		if se.RMI() != "" {
			sb.WriteString(`  Their message number is `)
			sb.WriteString(se.RMI())
			sb.WriteByte('.')
		}
		m.renderNotes(sb, e)
	case definition.EventReceive:
		if e.Occurred().IsZero() {
			sb.WriteString(`Message `)
			sb.WriteString(html.EscapeString(eid.Name))
			sb.WriteString(` is expected from `)
			m.renderStation(sb, stn)
			sb.WriteString(` by `)
			m.renderTime(sb, e.Expected())
			m.renderExpectedReason(sb, eid)
			sb.WriteByte('.')
			if e.Overdue() {
				sb.WriteString(`  It is overdue.`)
			}
			m.renderNotes(sb, e)
			m.renderManualTriggerButton(sb, eid.Type, stn.CallSign, eid.Name, "Mark Message as Received")
			break
		}
		sb.WriteString(`Message `)
		sb.WriteString(e.RMI())
		sb.WriteString(` (`)
		sb.WriteString(html.EscapeString(eid.Name))
		sb.WriteString(`) was received from `)
		m.renderStation(sb, stn)
		sb.WriteString(` at `)
		m.renderTime(sb, e.Occurred())
		sb.WriteString(`, and given the local ID `)
		sb.WriteString(e.LMI())
		sb.WriteByte('.')
		if e.Overdue() {
			sb.WriteString(`  It was expected by `)
			m.renderTime(sb, e.Expected())
			m.renderExpectedReason(sb, eid)
			sb.WriteString(` and was `)
			m.renderDuration(sb, e.Occurred().Sub(e.Expected()))
			sb.WriteString(` late.`)
		}
		if e.Score() == 100 {
			sb.WriteString(`  The message was transcribed correctly.`)
		} else if e.Score() != 0 {
			fmt.Fprintf(sb, `  The message had a transcription score of %d%%.`, e.Score())
		}
		m.renderNotes(sb, e)
		m.renderViewButton(sb, "View Message", e.LMI())
	case definition.EventSend:
		sb.WriteString(`Message `)
		sb.WriteString(html.EscapeString(eid.Name))
		if e != nil && !e.Occurred().IsZero() {
			sb.WriteString(` was sent to `)
		} else {
			sb.WriteString(` will be sent to `)
		}
		m.renderStation(sb, stn)
		if e == nil {
			sb.WriteString(` on request`)
		} else {
			sb.WriteString(` at `)
			if !e.Occurred().IsZero() {
				m.renderTime(sb, e.Occurred())
			} else {
				m.renderTime(sb, e.Expected())
				m.renderExpectedReason(sb, eid)
			}
		}
		sb.WriteByte('.')
		if e != nil && e.RMI() != "" {
			sb.WriteString(`  It was received by them as `)
			sb.WriteString(e.RMI())
			sb.WriteByte('.')
		}
		m.renderNotes(sb, e)
		if e != nil && !e.Occurred().IsZero() {
			m.renderViewButton(sb, "View Message", e.LMI())
		} else {
			m.renderManualTriggerButton(sb, eid.Type, stn.CallSign, eid.Name, "Send Message Now")
		}
	}
	sb.WriteString(`</div>`)
}

// renderStation renders the description of a station in a popup dialog.
func (m *Monitor) renderStation(sb *strings.Builder, stn *definition.Station) {
	sb.WriteString(stn.CallSign)
	if stn.FCCCall != "" && stn.FCCCall != stn.CallSign {
		sb.WriteString(" (")
		sb.WriteString(stn.FCCCall)
		sb.WriteByte(')')
	}
}

// renderTime renders a time of day in a popup dialog.  If the exercise spans
// multiple days, it includes the date as well.
func (m *Monitor) renderTime(sb *strings.Builder, t time.Time) {
	if !m.def.Exercise.OpStart.IsZero() && !m.def.Exercise.OpEnd.IsZero() &&
		m.def.Exercise.OpStart.Format("2006-01-02") != m.def.Exercise.OpEnd.Format("2006-01-02") {
		sb.WriteString(t.Format("01-02 15:04"))
	} else {
		sb.WriteString(t.Format("15:04"))
	}
}

// renderDuration renders a duration in a popup dialog.
func (m *Monitor) renderDuration(sb *strings.Builder, d time.Duration) {
	hr, mi := d/time.Hour, (d%time.Hour)/time.Minute
	if hr != 0 && mi != 0 {
		fmt.Fprintf(sb, "%dh%dm", hr, mi)
	} else if hr != 0 {
		fmt.Fprintf(sb, "%dh", hr)
	} else {
		fmt.Fprintf(sb, "%dm", mi)
	}
}

// renderNotes renders an event's notes in a popup dialog.
func (m *Monitor) renderNotes(sb *strings.Builder, e *state.Event) {
	sb.WriteString(`</p>`)
	if e != nil {
		for _, note := range e.Notes() {
			fmt.Fprintf(sb, "<div>%s</div>", html.EscapeString(note))
		}
	}
}

// renderViewButton renders a button that opens a message in a separate window.
// label is the button label.  lmi is the LMI of the message to open.
func (m *Monitor) renderViewButton(sb *strings.Builder, label, lmi string) {
	fmt.Fprintf(sb, `<p><button onclick="javascript:window.open('/message/%s.pdf','%s')">%s</button></p>`, lmi, lmi, label)
}

// renderManualTriggerButton renders a button that triggers an event.
func (m *Monitor) renderManualTriggerButton(sb *strings.Builder, etype definition.EventType, station, name, label string) {
	fmt.Fprintf(sb, `<p><button onclick="javascript:manualTrigger('%s','%s','%s')">%s</button></p>`, etype, station, name, label)
}

// renderExpectedReason renders a description of the trigger of an event.  It
// generally appears immediately after the scheduled or expected time for the
// event.
func (m *Monitor) renderExpectedReason(sb *strings.Builder, eid eventID) {
	var e *definition.Event

	if e = m.def.Event(eid.Type, eid.Name); e == nil || e.TriggerType == 0 {
		return
	}
	if e.TriggerType == 0 {
		return
	}
	sb.WriteString(` (`)
	if e.Delay == 0 {
		sb.WriteString(`on `)
	} else {
		m.renderDuration(sb, e.Delay)
		sb.WriteString(` after `)
	}
	switch e.TriggerType {
	case definition.EventStart:
		sb.WriteString(`exercise start`)
	case definition.EventManual:
		sb.WriteString(`request`)
	case definition.EventAlert:
		sb.WriteString(`voice alert of transmission of `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventBulletin:
		sb.WriteString(`posting of bulletin `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventDeliver:
		sb.WriteString(`delivery to principal of `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventInject:
		sb.WriteString(`inject of `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventReceipt:
		sb.WriteString(`getting delivery receipt for `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventReceive:
		sb.WriteString(`receipt of `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	case definition.EventSend:
		sb.WriteString(`send of `)
		sb.WriteString(html.EscapeString(e.TriggerName))
	}
	sb.WriteByte(')')
}
