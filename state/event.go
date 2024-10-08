package state

import (
	"time"

	"github.com/rothskeller/packet-ex/definition"
)

// An Event is a single event in the exercise, either expected or occurred.
type Event struct {
	id       int
	station  string
	etype    definition.EventType
	name     string
	trigger  int
	expected time.Time
	occurred time.Time
	overdue  bool
	lmi      string
	rmi      string
	score    int
	notes    []string
}

// ID is the unique identifier of the event.
func (e *Event) ID() int {
	return e.id
}

// Station is the call sign of the station for the event.  It can be "UNKNOWN"
// for a message received from an unknown station.  It will be "" for events not
// associated with a station, such as the start event or the sending of a
// bulletin.
func (e *Event) Station() string {
	return e.station
}

// Type is the type of event.
func (e *Event) Type() definition.EventType {
	return e.etype
}

// Name is the message name for the event.  It can be "UNKNOWN" for a received
// message that wasn't recognized.  It will be "" for events of type "start".
func (e *Event) Name() string {
	return e.name
}

// Trigger is the unique identifier of the event that triggered this one.  It is
// zero if there is no such event.
func (e *Event) Trigger() int {
	return e.trigger
}

// Expected is the time at which this event is scheduled or by which this event
// is expected.  It is zero if Trigger is zero.
func (e *Event) Expected() time.Time {
	return e.expected
}

// Occurred is the time at which this event occurred.  It is zero if the event
// is anticipated but hasn't yet occurred.
func (e *Event) Occurred() time.Time {
	return e.occurred
}

// Overdue returns whether the event is (or was) overdue.
func (e *Event) Overdue() bool {
	return e.overdue
}

// LMI is the local message ID of the message for a "send" or "receive" event
// that has occurred.  It is empty for other events.
func (e *Event) LMI() string {
	return e.lmi
}

// RMI is the remote message ID for a sent or received message.  For a sent
// message, it comes from the delivery receipt; for a received message, it comes
// from the subject line.  It is empty for all other events.
func (e *Event) RMI() string {
	return e.rmi
}

// Score is the percentage score (between 0 and 100) for a received message.  It
// is zero for all other events.
func (e *Event) Score() int {
	return e.score
}

// Notes are the notes associated with the event, if any.  The returned slice
// should not be changed by the caller.
func (e *Event) Notes() []string {
	if len(e.notes) == 0 {
		return nil
	}
	return e.notes
}
