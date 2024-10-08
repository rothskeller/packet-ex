package definition

import (
	"fmt"
	"regexp"
	"time"
)

type Definition struct {
	Filename       string
	Exercise       *Exercise
	FormValidation map[string]*FormValidation
	Stations       []*Station
	Events         []*Event
	MatchReceive   []*MatchReceive
	Bulletin       map[string]*Bulletin
	Send           map[string]*Message
	Receive        map[string]*Message
}

// Station returns the station definition with the specified call sign.
func (d *Definition) Station(callSign string) *Station {
	for _, s := range d.Stations {
		if s.CallSign == callSign {
			return s
		}
	}
	return nil
}

func (d *Definition) Event(etype EventType, name string) *Event {
	for _, e := range d.Events {
		if e.Type == etype && e.Name == name {
			return e
		}
	}
	return nil
}

type Exercise struct {
	ListenAddr   string
	Incident     string
	Activation   string
	OpStart      time.Time
	OpEnd        time.Time
	MyCall       string
	MyName       string
	MyPosition   string
	MyLocation   string
	OpCall       string
	OpName       string
	BBSName      string
	BBSAddress   string
	BBSPassword  string
	EmailFrom    string
	SMTPAddress  string
	SMTPUser     string
	SMTPPassword string
	StartMsgID   string
	Variables    map[string]string
}

const PackItForms = "PackItForms"

type FormValidation struct {
	MinVer     string
	Handling   string
	ToPosition []string
	ToLocation []string
}

type Station struct {
	CallSign     string
	Prefix       string
	FCCCall      string
	Inject       string
	Position     string
	Location     string
	ReceiptDelay time.Duration
	NoReceipts   bool
	Variables    map[string]string
}

type EventType byte

const (
	_ EventType = iota
	EventInject
	EventReceive
	EventBulletin
	EventSend
	EventDeliver
	EventAlert
	// internal only:
	EventReceipt
	EventReject
	// triggers but not real events:
	EventStart
	EventManual
)

func (et EventType) String() string { return eventTypeNames[et] }

func ParseEventType(s string) (et EventType, err error) {
	for et, name := range eventTypeNames {
		if s == name {
			return et, nil
		}
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}

type Event struct {
	Group        string
	Type         EventType
	Name         string
	TriggerType  EventType
	TriggerName  string
	ConditionVar string
	ConditionOp  string
	ConditionVal string
	ConditionRE  *regexp.Regexp
	Delay        time.Duration
}

type MatchReceive struct {
	Name      string
	Type      string
	Subject   string
	SubjectRE *regexp.Regexp
}

func (mr *MatchReceive) hiddenBy(o *MatchReceive) bool {
	if mr.Type != o.Type {
		return false
	}
	if mr.Subject != "" && mr.Subject == o.Subject {
		return true
	}
	if mr.Subject != "" && o.SubjectRE != nil && o.SubjectRE.MatchString(mr.Subject) {
		return true
	}
	return false
}

type Bulletin struct {
	Area    string
	Subject string
	Message string
}

type Message struct {
	Type    string
	Version string
	Fields  map[string]StringWithInterps
}

// A StringWithInterps is a string that may contain interpolated variables.  It
// is modeled as a sequence of literal string, variable interpolation, literal
// string, etc.  The sequence always starts and ends with a literal string, even
// if it's empty, and there is always a literal string between each pair of
// variable interpolations, even if it's empty.  Therefore, the number of
// literal strings is always one greater than the number of variable
// interpolations.
//
// This construct is modeled in the StringWithInterps structure as a set of
// literal strings (Literals) and a set of variable interpolations (Variables,
// StartOffsets, EndOffsets, and Additions).  The resulting string is
// constructed by concatening literal #0, variable #0, literal #1, variable #1,
// etc.
type StringWithInterps struct {
	Literals     []string
	Variables    []string
	StartOffsets []int
	EndOffsets   []int
	Additions    []string
}

var eventTypeNames = map[EventType]string{
	EventInject:   "inject",
	EventReceive:  "receive",
	EventBulletin: "bulletin",
	EventSend:     "send",
	EventDeliver:  "deliver",
	EventAlert:    "alert",
	EventReceipt:  "receipt",
	EventReject:   "reject",
	EventStart:    "start",
	EventManual:   "manual",
}
