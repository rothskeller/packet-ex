package state

import (
	"fmt"
	"time"

	"github.com/rothskeller/packet-ex/definition"
)

func (s *State) StartExercise() (e *Event) {
	if len(s.events) != 0 {
		panic("StartExercise: exercise already started")
	}
	s.events = []*Event{nil} // put a nil in [0] so that EIDs start with 1.
	return s.mustExecutef(
		"%s [%d] ALL start",
		s.logNow(), len(s.events))
}

func (s *State) StartStation(station string) (e *Event) {
	return s.mustExecutef(
		"%s [%d] %s start",
		s.logNow(), len(s.events), station)
}

func (s *State) SendMessage(etype definition.EventType, station, name, lmi, subject string, trigger int) (e *Event) {
	eid := len(s.events)
	if ev := s.FindEvent(etype, station, name); ev != nil {
		eid = ev.id
	}
	line := fmt.Sprintf("%s [%d] %s %s %s SENT LMI %s",
		s.logNow(), eid, safeStation(station), etype, name, lmi)
	if trigger != 0 {
		line = fmt.Sprintf("%s [%d]", line, trigger)
	}
	e = s.mustExecute(line)
	if subject != "" {
		s.mustExecute("    Subject: " + subject)
	}
	return e
}

func (s *State) CreateInject(station, name, rmi, method string, trigger int) (e *Event) {
	switch method {
	case "PRINTED", "EMAILED", "CREATED":
		break
	default:
		panic("invalid inject method")
	}
	eid := len(s.events)
	if ev := s.FindEvent(definition.EventInject, station, name); ev != nil {
		eid = ev.id
	}
	line := fmt.Sprintf("%s [%d] %s inject %s %s",
		s.logNow(), eid, station, name, method)
	if rmi != "" {
		line = fmt.Sprintf("%s RMI %s", line, rmi)
	}
	if trigger != 0 {
		line = fmt.Sprintf("%s [%d]", line, trigger)
	}
	return s.mustExecute(line)
}

func (s *State) MatchInject(station, name, rmi string) (e *Event) {
	ev := s.FindEvent(definition.EventInject, station, name)
	if ev == nil || (ev.rmi != "" && ev.rmi != rmi) {
		return nil
	}
	line := fmt.Sprintf("%s [%d] %s inject %s MATCHED RMI %s",
		s.logNow(), ev.id, station, name, rmi)
	return s.mustExecute(line)
}

func (s *State) RecordReject(station, name, lmi, from, subject string) (e *Event) {
	line := fmt.Sprintf("%s [%d] %s reject %s REJECTED LMI %s",
		s.logNow(), len(s.events), station, name, lmi)
	if from != "" && from != s.addrs[station] {
		line = fmt.Sprintf("%s FROM %s", line, from)
	}
	e = s.mustExecute(line)
	s.mustExecute("    Subject: " + subject)
	return e
}

func (s *State) ReceiveMessage(station, name, lmi, from, subject string) (e *Event) {
	eid := len(s.events)
	if ev := s.FindEvent(definition.EventReceive, station, name); ev != nil && ev.Occurred().IsZero() {
		eid = ev.id
	}
	line := fmt.Sprintf("%s [%d] %s receive %s RECEIVED LMI %s",
		s.logNow(), eid, station, name, lmi)
	if from != "" && from != s.addrs[station] {
		line = fmt.Sprintf("%s FROM %s", line, from)
	}
	e = s.mustExecute(line)
	if subject != "" {
		s.mustExecute("    Subject: " + subject)
	}
	return e
}

func (s *State) ScoreMessage(e *Event, problems []string, score int) *Event {
	line := fmt.Sprintf("%s [%d] %s receive %s SCORE %d",
		s.logNow(), e.id, e.station, e.name, score)
	e = s.mustExecute(line)
	for _, problem := range problems {
		s.mustExecute("    PROBLEM: " + problem)
	}
	return e
}

func (s *State) ReceiveDeliveryReceipt(lmi, rmi string) (e *Event) {
	for _, e := range s.events {
		if e == nil || e.lmi != lmi || e.etype != definition.EventSend {
			continue
		}
		return s.mustExecutef(
			"%s [%d] %s %s %s DELIVERED RMI %s",
			s.logNow(), e.id, e.station, e.etype, e.name, rmi)
	}
	s.LogError(fmt.Errorf("can't record delivery receipt: no send event for %s->%s", lmi, rmi))
	return nil
}

func (s *State) ScheduleEvent(etype definition.EventType, station, name string, at time.Time, trigger int) (e *Event) {
	switch etype {
	case definition.EventBulletin, definition.EventSend, definition.EventInject:
		break
	default:
		panic("invalid etype for scheduled event")
	}
	eid := len(s.events)
	if ev := s.FindEvent(etype, station, name); ev != nil {
		if !ev.occurred.IsZero() {
			return nil
		}
		eid = ev.id
	}
	line := fmt.Sprintf("%s [%d] %s %s %s SCHEDULED %s",
		s.logNow(), eid, safeStation(station), etype, name,
		at.Format(expectedFormat))
	if trigger != 0 {
		line = fmt.Sprintf("%s [%d]", line, trigger)
	}
	return s.mustExecute(line)
}

func (s *State) ExpectEvent(etype definition.EventType, station, name string, by time.Time, trigger int) (e *Event) {
	var eid = len(s.events)

	switch etype {
	case definition.EventAlert, definition.EventDeliver, definition.EventReceipt:
		break
	case definition.EventReceive:
		// It's possible the message was already received before it was
		// expected.
		if e = s.FindEvent(etype, station, name); e != nil && e.expected.IsZero() {
			eid = e.id
		}
	default:
		panic("invalid etype for expected event")
	}
	return s.mustExecutef(
		"%s [%d] %s %s %s EXPECTED %s [%d]",
		s.logNow(), eid, safeStation(station), etype, name,
		by.Format(expectedFormat), trigger)
}

func (s *State) RecordEvent(etype definition.EventType, station, name string) (e *Event) {
	switch etype {
	case definition.EventAlert, definition.EventDeliver, definition.EventReceive:
		break
	default:
		panic("invalid etype for recorded event")
	}
	eid := len(s.events)
	if ev := s.FindEvent(etype, station, name); ev != nil {
		if !ev.occurred.IsZero() {
			return nil
		}
		eid = ev.id
	}
	return s.mustExecutef(
		"%s [%d] %s %s %s RECORDED",
		s.logNow(), eid, safeStation(station), etype, name)
}

func (s *State) MarkOverdueEvents(asof time.Time) {
	for _, e := range s.events {
		if e == nil {
			continue
		}
		switch e.etype {
		case definition.EventAlert, definition.EventDeliver, definition.EventReceive:
			// nothing
		default:
			continue
		}
		if !e.occurred.IsZero() || e.expected.IsZero() || e.overdue {
			continue
		}
		if e.expected.Before(asof) {
			s.mustExecutef(
				"%s [%d] %s %s %s OVERDUE",
				s.logNow(), e.id, safeStation(e.station), e.etype, e.name,
			)
		}
	}
}

func (s *State) DropEvent(e *Event) {
	s.mustExecutef(
		"%s [%d] %s %s %s DROPPED",
		s.logNow(), e.id, safeStation(e.station), e.etype, e.name)
}

func (s *State) logNow() string { return s.now().Format(occurredFormat) }
func safeStation(stn string) string {
	if stn == "" {
		return "ALL"
	}
	return stn
}
