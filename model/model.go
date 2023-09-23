package model

import (
	"fmt"
	"io"

	"github.com/rothskeller/packet-ex/variables"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Identity     Identity
	BBS          BBS
	StartMsgID   string
	Incident     Incident
	Participants []*Participant
	Rules        Rules
	Messages     []*Message

	ParticipantMap Participants `yaml:"-"`
	MessageMap     Messages     `yaml:"-"`
	varsource      variables.Source
}

type Identity struct {
	TacCall string
	TacName string
	FCCCall string
	FCCName string
}

type BBS struct {
	Name     string
	Address  string
	Password string
}

type Incident struct {
	Name      string
	ActNum    string
	StartDate string
	StartTime string
	EndDate   string
	EndTime   string
}

type Participants map[string]*Participant
type Participant struct {
	TacCall string
	Prefix  string
	Vars    map[string]string `yaml:",inline"`
}

type Rules []*Rule
type Rule struct {
	When    string
	Wait    string
	Then    string
	Message string `yaml:"msg"`
	To      string
	Vars    map[string]string `yaml:",inline"`
}

type Messages map[string]*Message
type Message struct {
	Name   string
	Type   string
	Match  string
	Fields map[string]string `yaml:",inline"`
}

// Read reads the model definition from a stream.
func Read(r io.Reader) (m *Model, err error) {
	var dec *yaml.Decoder

	m = new(Model)
	dec = yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err = dec.Decode(m); err != nil {
		return nil, fmt.Errorf("reading exercise model: %w", err)
	}
	if err = m.validateIdentity(); err != nil {
		return nil, err
	}
	if err = m.validateBBS(); err != nil {
		return nil, err
	}
	if err = m.validateStartMsgID(); err != nil {
		return nil, err
	}
	if err = m.validateIncident(); err != nil {
		return nil, err
	}
	if err = m.validateParticipants(); err != nil {
		return nil, err
	}
	if err = m.validateRules(); err != nil {
		return nil, err
	}
	if err = m.validateMessages(); err != nil {
		return nil, err
	}
	m.ParticipantMap = make(map[string]*Participant)
	for _, p := range m.Participants {
		if p != nil {
			m.ParticipantMap[p.TacCall] = p
		}
	}
	m.MessageMap = make(map[string]*Message)
	for _, mdef := range m.Messages {
		if mdef != nil {
			m.MessageMap[mdef.Name] = mdef
		}
	}
	return m, nil
}

func (m *Model) validateIdentity() (err error) { // TODO
	return nil
}

func (m *Model) validateBBS() (err error) { // TODO
	return nil
}

func (m *Model) validateStartMsgID() (err error) { // TODO
	return nil
}

func (m *Model) validateIncident() (err error) { // TODO
	return nil
}

func (m *Model) validateParticipants() (err error) { // TODO
	return nil
}

func (m *Model) validateRules() (err error) { // TODO
	return nil
}

func (m *Model) validateMessages() (err error) { // TODO
	return nil
}
