// Package state maintains the state of the exercise engine.
package state

import (
	"fmt"
	"time"
)

const expectedFormat = "2006-01-02T15:04"
const occurredFormat = "2006-01-02T15:04:05.000"

// A LogListener is an object that listens for new lines added to the state log.
type LogListener interface {
	// OnLogLine is called whenever there is a new line added to the state
	// log.  To receive all lines starting from the beginning of the log,
	// the listener must be added with AddListener before Open is called.
	OnLogLine(line string)
}

// An EventListener is an object that listens for new and updated events.
type EventListener interface {
	// OnEventChange is called whenever an event is created or updated.
	// To receive all event changes starting from the beginning of the
	// exercise, the listener must be added with AddListener before Open is
	// called.
	OnEventChange(e *Event)
}

// State represents the state of the exercise engine.  Note that the State is
// not thread-safe.  All calls to its methods must be synchronized by the
// caller.
type State struct {
	events    []*Event
	addrs     map[string]string
	listeners []any
	now       func() time.Time
	lastTime  time.Time
	lastEID   int
	debug     bool
}

// New creates a new State tracker.
func New(debug bool) *State {
	return &State{now: time.Now, debug: debug, addrs: make(map[string]string)}
}

// SetNowFunc sets the function used by the state engine to determine the
// current time of day.  It is used for replaying old exercises and for testing.
func (s *State) SetNowFunc(now func() time.Time) {
	s.now = now
}

// Now returns the current time of day.
func (s *State) Now() time.Time {
	return s.now()
}

// AddListener adds a listener to the state.  It should implement LogListener or
// EventListener or both.  Any number of listeners can be added.  To receive
// notifications from the exercise beginning, AddListener must be called before
// Open.
func (s *State) AddListener(sl any) {
	s.listeners = append(s.listeners, sl)
}

// LastEntry returns the timestamp and event ID of the last entry in the state
// log.  It returns zeros if there have been no entries.
func (s *State) LastEntry() (tstamp time.Time, eid int) {
	return s.lastTime, s.lastEID
}

// LogError adds an error to the log file.
func (s *State) LogError(err error) {
	s.mustExecutef("%s ERROR: %s\n", s.logNow(), err)
}

// mustExecute records a state change entry in the state log, and then
// executes it as if reading it from the log.
func (s *State) mustExecutef(f string, args ...any) *Event {
	return s.mustExecute(fmt.Sprintf(f, args...))
}
func (s *State) mustExecute(line string) *Event {
	if e, err := s.Execute(line); err != nil {
		panic(fmt.Sprintf("recording line %q: %s", line, err))
	} else {
		return e
	}
}
