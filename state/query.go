package state

import (
	"strings"

	"github.com/rothskeller/packet-ex/definition"
)

// GetEvent returns the event with the specified ID, or nil if there is none.
func (s *State) GetEvent(eid int) *Event {
	if eid > 0 && eid < len(s.events) {
		return s.events[eid]
	}
	return nil
}

// AllEvents returns a slice of all events.  It must not be modified.
func (s *State) AllEvents() []*Event {
	if len(s.events) == 0 {
		return s.events
	} else {
		return s.events[1:]
	}
}

// FindEvent returns the event with the specified type, station, and message
// name, if it exists.  If there is more than one such event, it returns the
// latest one.
func (s *State) FindEvent(etype definition.EventType, station, name string) *Event {
	for i := len(s.events) - 1; i >= 0; i-- {
		ev := s.events[i]
		if ev != nil && ev.etype == etype && ev.station == station && ev.name == name {
			return ev
		}
	}
	return nil
}

// GetEventByTrigger returns any existing event with the specified type,
// station, message name, and trigger.
func (s *State) GetEventByTrigger(etype definition.EventType, station, name string, trigger int) *Event {
	for _, e := range s.events {
		if e != nil && e.etype == etype && e.station == station && e.name == name && e.trigger == trigger {
			return e
		}
	}
	return nil
}

// GetSendReceiveEventByStationName returns the Send or Receive event with the specified
// station and message name.
func (s *State) GetSendReceiveEventByStationName(station, name string) *Event {
	for i := len(s.events) - 1; i > 0; i-- {
		e := s.events[i]
		if (e.etype == definition.EventReceive || e.etype == definition.EventSend) &&
			e.station == station && e.name == name {
			return e
		}
	}
	return nil
}

// StationStarted returns whether the specified station has started the
// exercise.
func (s *State) StationStarted(stn string) bool {
	for _, e := range s.events {
		if e != nil && e.station == stn {
			return true
		}
	}
	return false
}

// AddressForStation returns the last recorded address from which the named
// station sent a message.  It returns the station name itself (lowercased) if
// no messages have been received from the station.
func (s *State) AddressForStation(stn string) string {
	if addr, ok := s.addrs[stn]; ok {
		return addr
	}
	return strings.ToLower(stn)
}

// SentBulletins returns a list of events for sent bulletins.  Only those sent
// to station "-" are included, and only those that have occurred.
func (s *State) SentBulletins() (sbs []*Event) {
	for _, e := range s.events {
		if e != nil && e.etype == definition.EventBulletin && e.station == "" && !e.occurred.IsZero() {
			sbs = append(sbs, e)
		}
	}
	return sbs
}

// PendingEvent returns the past-scheduled but not completed event of the
// specified type with the lowest scheduled time.  If there is no such, it
// return nil.
func (s *State) PendingEvent(etype definition.EventType) (event *Event) {
	now := s.now()
	for _, e := range s.events {
		switch {
		case e == nil, !e.occurred.IsZero(), e.expected.IsZero(), !e.expected.Before(now):
			// nope, completed or not ready
		case e.etype != etype:
			// nope, wrong type
		case event != nil && !e.expected.Before(event.expected):
			// nope, the one we already found is expected sooner
		default:
			// this one might do
			event = e
		}
	}
	return event
}

// IsMessageExpected returns whether a received message with the specified
// station and message name is expected.
func (s *State) IsMessageExpected(station, name string) bool {
	if ev := s.FindEvent(definition.EventReceive, station, name); ev == nil {
		return false
	} else {
		return ev.occurred.IsZero() && !ev.expected.IsZero()
	}
}
