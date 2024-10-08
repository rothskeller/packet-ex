package state

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
)

var stateLineRE = regexp.MustCompile(`^(20\d\d-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]\.[0-9]{3}) \[(\d+)\] ([A-Z][A-Z0-9]{0,5}) (\S+)`)
var errWarnLineRE = regexp.MustCompile(`^(20\d\d-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]\.[0-9]{3}) (?:ERROR|WARNING): `)
var triggerRE = regexp.MustCompile(`^\[(\d+)\]$`)

// Execute parses and executes a single state change line.
func (s *State) Execute(line string) (e *Event, err error) {
	var (
		tstamp time.Time
		fields []string
		e2     *Event
	)
	// Notify listeners of new log line.
	for _, l := range s.listeners {
		if ll, ok := l.(LogListener); ok {
			ll.OnLogLine(line)
		}
	}
	if s.debug {
		fmt.Println(line)
	}
	// Ignore blank lines, errors, and warnings.
	if strings.TrimSpace(line) == "" || errWarnLineRE.MatchString(line) {
		s.lastEID = 0
		return nil, nil
	}
	// Indented lines get added as notes on the event of the preceding entry.
	if line[0] == ' ' || line[0] == '\t' {
		if s.lastEID != 0 {
			e = s.events[s.lastEID]
			e.notes = append(e.notes, strings.TrimSpace(line))
			goto DONE
		}
		return nil, nil
	}
	// Everything else should match the state line regular expression.
	if match := stateLineRE.FindStringSubmatch(line); match == nil {
		return nil, errors.New("syntax error")
	} else {
		if tstamp, err = time.ParseInLocation(occurredFormat, match[1], time.Local); err != nil || tstamp.Format(occurredFormat) != match[1] {
			return nil, errors.New("syntax error: bad timestamp format")
		}
		e = new(Event)
		e.id, _ = strconv.Atoi(match[2])
		if e.station = match[3]; e.station == "ALL" {
			e.station = ""
		}
		if e.etype, err = definition.ParseEventType(match[4]); err != nil {
			return nil, errors.New("syntax error: bad event type")
		}
		line = line[len(match[0]):]
		line = strings.TrimLeft(line, " ")
	}
	// For all event types except "start", the next thing should be a
	// message name.
	if e.etype != definition.EventStart {
		if idx := strings.Index(line, " "); idx < 0 {
			e.name, line = line, ""
		} else {
			e.name, line = line[:idx], line[idx+1:]
		}
	}
	// Find the event in the list.
	switch {
	case e.id > 0 && e.id < len(s.events):
		// Make sure it matches the existing entry, then point to it.
		if s.events[e.id].etype != e.etype || s.events[e.id].station != e.station || s.events[e.id].name != e.name {
			return nil, errors.New("event data mismatch on type, station, or message name")
		}
		e = s.events[e.id]
	case e.id == len(s.events):
		// New event.  Make sure there are no existing events with the
		// same characteristics.  (EventReceive and EventReject are
		// exceptions.)
		if e.etype != definition.EventReject && e.etype != definition.EventReceive && slices.ContainsFunc(s.events, func(ee *Event) bool {
			return ee != nil && ee.etype == e.etype && ee.station == e.station && ee.name == e.name
		}) {
			return nil, errors.New("creating redundant event")
		}
		s.events = append(s.events, e)
	case e.id == 1 && len(s.events) == 0:
		// New initial event.
		s.events = []*Event{nil, e}
	default:
		return nil, errors.New("invalid event ID")
	}
	// Update the last-entry data.
	s.lastTime, s.lastEID = tstamp, e.id
	// Now look for the various other things that can appear on the line.
	fields = strings.Fields(line)
	// A start event has no arguments; if it's seen, it occurred.
	if e.etype == definition.EventStart && len(fields) == 0 {
		e.occurred = tstamp
		goto DONE
	}
	// If the last one is a number in brackets, it's the trigger event ID.
	if match := triggerRE.FindStringSubmatch(fields[len(fields)-1]); match != nil {
		if e.trigger, _ = strconv.Atoi(match[1]); e.trigger < 1 || e.trigger >= len(s.events) {
			return nil, errors.New("invalid trigger ID")
		}
		fields = fields[:len(fields)-1]
	}
	// "DROPPED" means an expected event was dropped because its definition
	// or message template was removed from the exercise before its
	// expected time.
	if len(fields) == 1 && fields[0] == "DROPPED" {
		if !e.occurred.IsZero() {
			return nil, errors.New("can't drop occurred event")
		}
		e.expected = time.Time{}
		goto DONE
	}
	// "PRINTED", "EMAILED", and "CREATED" all indicate occurrence of an
	// inject, and may all be followed by an RMI.
	if e.etype == definition.EventInject && (len(fields) == 1 || (len(fields) == 3 && fields[1] == "RMI")) && (fields[0] == "PRINTED" || fields[0] == "EMAILED" || fields[0] == "CREATED") {
		if !e.occurred.IsZero() {
			return nil, errors.New("inject re-created")
		}
		if len(fields) == 3 {
			e.rmi = fields[2]
		}
		e.occurred = tstamp
		goto DONE
	}
	// "MATCHED" indicates an inject that has been matched, and stores the
	// RMI of the received message that matched it.
	if e.etype == definition.EventInject && len(fields) == 3 && fields[0] == "MATCHED" && fields[1] == "RMI" {
		if e.occurred.IsZero() {
			return nil, errors.New("match of non-created inject")
		}
		if e.rmi != "" && e.rmi != fields[2] {
			return nil, errors.New("inject RMI mismatch")
		}
		e.rmi = fields[2]
		goto DONE
	}
	// If a bulletin or send is followed by SENT and an LMI, that means it
	// occurred.
	if (e.etype == definition.EventBulletin || e.etype == definition.EventSend) && len(fields) == 3 && fields[0] == "SENT" && fields[1] == "LMI" {
		if !e.occurred.IsZero() {
			return nil, errors.New("message re-sent")
		}
		e.lmi = fields[2]
		e.occurred = tstamp
		goto DONE
	}
	// If a reject is followed by REJECTED, an LMI, and possibly a FROM,
	// it has occurred.
	if e.etype == definition.EventReject && (len(fields) == 3 || len(fields) == 5) && fields[0] == "REJECTED" && fields[1] == "LMI" && (len(fields) == 3 || fields[3] == "FROM") {
		e.lmi = fields[2]
		e.occurred = tstamp
		goto DONE
	}
	// If a receive is followed by RECEIVED, an LMI, and possibly a FROM,
	// we record its details.  If it was expected, we also mark it as having
	// occurred.
	if e.etype == definition.EventReceive && (len(fields) == 3 || len(fields) == 5) && fields[0] == "RECEIVED" && fields[1] == "LMI" && (len(fields) == 3 || fields[3] == "FROM") {
		if !e.occurred.IsZero() {
			return nil, errors.New("message re-received")
		}
		e.lmi = fields[2]
		if len(fields) == 5 {
			s.addrs[e.station] = fields[4]
		}
		if !e.expected.IsZero() {
			e.occurred = tstamp
		}
		goto DONE
	}
	// If a receive is followed by a SCORE, that means it was analyzed.
	if e.etype == definition.EventReceive && len(fields) == 2 && fields[0] == "SCORE" {
		if e.lmi == "" {
			return nil, errors.New("score on unreceived message")
		}
		if e.score, err = strconv.Atoi(fields[1]); err != nil || e.score < 0 || e.score > 100 {
			return nil, errors.New("invalid score")
		}
		goto DONE
	}
	// If a send is followed by "DELIVERED" and an RMI, we add the RMI to
	// the event, and we mark any matching receipt event as occurred.
	if e.etype == definition.EventSend && len(fields) == 3 && fields[0] == "DELIVERED" && fields[1] == "RMI" {
		if e.occurred.IsZero() {
			return nil, errors.New("delivered on unsent message")
		}
		e.rmi = fields[2]
		if idx := slices.IndexFunc(s.events, func(re *Event) bool {
			return re != nil && re.etype == definition.EventReceipt && re.station == e.station && re.name == e.name && re.occurred.IsZero()
		}); idx >= 0 {
			e2 = s.events[idx]
			e2.occurred = tstamp
		}
		goto DONE
	}
	// Handle the various expect cases.
	switch e.etype {
	case definition.EventBulletin, definition.EventSend, definition.EventInject:
		if len(fields) == 2 && fields[0] == "SCHEDULED" {
			if !e.occurred.IsZero() {
				return nil, errors.New("rescheduling completed event")
			}
			if e.expected, err = time.ParseInLocation(expectedFormat, fields[1], time.Local); err != nil || e.expected.Format(expectedFormat) != fields[1] {
				return nil, errors.New("invalid scheduled time")
			}
			goto DONE
		}
	case definition.EventAlert, definition.EventReceive, definition.EventDeliver, definition.EventReceipt:
		if len(fields) == 2 && fields[0] == "EXPECTED" {
			if !e.occurred.IsZero() {
				return nil, errors.New("re-expecting completed event")
			}
			if e.expected, err = time.ParseInLocation(expectedFormat, fields[1], time.Local); err != nil || e.expected.Format(expectedFormat) != fields[1] {
				return nil, errors.New("invalid expected time")
			}
			goto DONE
		}
		if len(fields) == 1 && fields[0] == "OVERDUE" {
			if !e.occurred.IsZero() {
				return nil, errors.New("marking completed event overdue")
			}
			e.overdue = true
			goto DONE
		}
	}
	// Handle the recording cases.
	switch e.etype {
	case definition.EventAlert, definition.EventDeliver, definition.EventReceive:
		if len(fields) == 1 && fields[0] == "RECORDED" {
			if !e.occurred.IsZero() {
				return nil, errors.New("recording completion of completed event")
			}
			e.occurred = tstamp
			goto DONE
		}
	}
	// Those should be the only possibilities.
	return nil, errors.New("syntax error: unknown entry format")
DONE:
	// Notify listeners of event change.
	for _, l := range s.listeners {
		if el, ok := l.(EventListener); ok {
			el.OnEventChange(e)
			if e2 != nil {
				el.OnEventChange(e2)
			}
		}
	}
	return e, nil
}
