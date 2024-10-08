package engine

import (
	"strings"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
)

var (
	// Cache of the last message looked up in a variable reference.  This
	// makes the use of multiple variables from that message more efficient.
	lastLMI string
	lastEnv *envelope.Envelope
	lastMsg message.Message
)

func (e *Engine) Variable(name, station string) (value string, ok bool) {
	group, item, _ := strings.Cut(name, ".")
	switch group {
	case "exercise":
		value, ok = e.def.Exercise.Variables[item]
		return value, ok
	case "station":
		if stn := e.def.Station(station); stn == nil {
			return "", false
		} else {
			value, ok = stn.Variables[item]
			return value, ok
		}
	case "now":
		switch item {
		case "date":
			return e.st.Now().Format("01/02/2006"), true
		case "time":
			return e.st.Now().Format("15:04"), true
		case "datetime":
			return e.st.Now().Format("01/02/2006 15:04"), true
		default:
			return "", false
		}
	}
	ev := e.st.GetSendReceiveEventByStationName(station, group)
	if ev == nil || ev.LMI() == "" {
		return "", false
	}
	var env *envelope.Envelope
	var msg message.Message
	if ev.LMI() == lastLMI {
		env, msg = lastEnv, lastMsg
	} else {
		var err error
		if env, msg, err = incident.ReadMessage(ev.LMI()); err != nil {
			return "", false
		}
		lastLMI, lastEnv, lastMsg = ev.LMI(), env, msg
	}
	switch item {
	case "msgid":
		if on := msg.Base().FOriginMsgID; on != nil && *on != "" {
			return *on, true
		}
	case "subjectline":
		return env.SubjectLine, true
	case "time":
		if !ev.Occurred().IsZero() {
			return ev.Occurred().Format("01/02/2006 15:04"), true
		}
	}
	return "", false
}
