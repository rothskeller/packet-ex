package engine

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/plaintext"
)

// generateSendMessage generates an outgoing private message based on a template
// in the exercise definition.
func (e *Engine) generateSendMessage(ev *state.Event) (lmi string, env *envelope.Envelope, msg message.Message) {
	lmi = incident.UniqueMessageID(e.def.Exercise.StartMsgID)
	env = &envelope.Envelope{From: e.myFrom(), To: e.st.AddressForStation(ev.Station())}
	msg = e.generateMessage(e.def.Send[ev.Name()], ev.Station())
	e.setMessageDefaults(msg, ev.Station(), false)
	if mn := msg.Base().FOriginMsgID; mn != nil {
		*mn = lmi
	}
	if err := incident.SaveMessage(lmi, "", env, msg, true, false); err != nil {
		e.st.LogError(fmt.Errorf("saving generated message: %w", err))
		return "", nil, nil
	}
	return
}

// generateBulletin generates an outgoing bulletin based on a template in the
// exercise definition.
func (e *Engine) generateBulletin(ev *state.Event) (lmi string, env *envelope.Envelope, _ message.Message) {
	var (
		tmpl *definition.Bulletin
		msg  *plaintext.PlainText
	)
	if tmpl = e.def.Bulletin[ev.Name()]; tmpl == nil {
		return "", nil, nil // bulletin no longer defined
	}
	lmi = incident.UniqueMessageID(e.def.Exercise.StartMsgID)
	env = &envelope.Envelope{
		From:        e.myFrom(),
		To:          tmpl.Area,
		SubjectLine: tmpl.Subject,
		Bulletin:    true,
	}
	msg = message.Create("plain", "").(*plaintext.PlainText)
	msg.Subject = tmpl.Subject
	msg.Body = tmpl.Message
	if err := incident.SaveMessage(lmi, "", env, msg, true, false); err != nil {
		e.st.LogError(fmt.Errorf("saving generated bulletin: %w", err))
		return "", nil, nil
	}
	return lmi, env, msg
}

// generateInject generates a message we expect to receive based on a template
// in the exercise definition.
func (e *Engine) generateInject(ev *state.Event) (lmi string, env *envelope.Envelope, msg message.Message) {
	lmi = fmt.Sprintf("INJ-%03dI", ev.ID())
	env = new(envelope.Envelope)
	msg = e.generateMessage(e.def.Receive[ev.Name()], ev.Station())
	e.setMessageDefaults(msg, ev.Station(), true)
	if err := incident.SaveMessage(lmi, "", env, msg, false, false); err != nil {
		e.st.LogError(fmt.Errorf("saving generated inject: %w", err))
		return "", nil, nil
	}
	return
}

// generateReceivedModel generates a model message to compare a received message
// against, based on a template in the exercise definition.
func (e *Engine) generateReceivedModel(station, msgname string) (msg message.Message) {
	return e.generateMessage(e.def.Receive[msgname], station)
}

// generateMessage generates a message based on a template in the exercise
// definition.
func (e *Engine) generateMessage(tmpl *definition.Message, station string) (msg message.Message) {
	// If the exercise definition no longer contains things referenced by
	// this message, don't generate the message.
	if tmpl == nil {
		return nil
	}
	if station != "" && e.def.Station(station) == nil {
		return nil
	}
	// Create a message of the type named in the template.
	msg = message.Create(tmpl.Type, tmpl.Version)
	// Clear out any defaults; we want to do our own.
	for _, f := range msg.Base().Fields {
		if f.Value != nil {
			*f.Value = ""
		}
	}
	// Apply the values from the template.
	for fname, ftmpl := range tmpl.Fields {
		value := e.generateValue(ftmpl, station)
		for _, mf := range msg.Base().Fields {
			if mf.Label == fname {
				mf.EditApply(mf, value)
				break
			}
		}
	}
	return msg
}

// setMessageDefaults sets the Date, Time, Handling, To ICS Position, To
// Location, From ICS Position, From Location, and Reference fields of the
// message.  If reverse is true, the To and From fields are swapped (i.e., the
// engine receiving rather than sending the message).
func (e *Engine) setMessageDefaults(msg message.Message, station string, reverse bool) {
	if md := msg.Base().FMessageDate; md != nil && *md == "" {
		*md = e.st.Now().Format("01/02/2006")
	}
	if mt := msg.Base().FMessageTime; mt != nil && *mt == "" {
		*mt = e.st.Now().Format("15:04")
	}
	if h := msg.Base().FHandling; h != nil && *h == "" {
		if fh := e.def.FormValidation[msg.Base().Type.Tag]; fh != nil && len(fh.Handling) != 0 {
			if fh.Handling != "computed" {
				*h = fh.Handling
			} else {
				switch msg.Base().Type.Tag {
				case "ICS213":
					// just ignore it; let it fail validation below
				case "EOC213RR":
					for _, f := range msg.Base().Fields {
						if f.Label == "Priority" {
							switch *f.Value {
							case "Now", "High":
								*h = "IMMEDIATE"
							case "Medium":
								*h = "PRIORITY"
							case "Low":
								*h = "ROUTINE"
							}
							break
						}
					}
				default:
					// Definition parser shouldn't let
					// anything else through, so this is a
					// software bug.
					panic("'computed' handling order not supported for " + msg.Base().Type.Tag)
				}
			}
		}
	}
	var topos, toloc string
	if s := e.def.Station(station); s != nil {
		topos, toloc = s.Position, s.Location
	}
	frompos, fromloc := e.def.Exercise.MyPosition, e.def.Exercise.MyLocation
	if fh := e.def.FormValidation[msg.Base().Type.Tag]; fh != nil {
		if topos == "" && len(fh.ToPosition) != 0 {
			topos = fh.ToPosition[0]
		}
		if toloc == "" && len(fh.ToLocation) != 0 {
			toloc = fh.ToLocation[0]
		}
	}
	if reverse {
		topos, toloc, frompos, fromloc = frompos, fromloc, topos, toloc
	}
	if fp := msg.Base().FFromICSPosition; fp != nil && *fp == "" {
		*fp = frompos
	}
	if fl := msg.Base().FFromLocation; fl != nil && *fl == "" {
		*fl = fromloc
	}
	if tp := msg.Base().FToICSPosition; tp != nil && *tp == "" {
		*tp = topos
	}
	if tl := msg.Base().FToLocation; tl != nil && *tl == "" {
		*tl = toloc
	}
}

// myFrom returns the envelope From address of the engine.
func (e *Engine) myFrom() string {
	return (&mail.Address{
		Name:    e.def.Exercise.MyName,
		Address: fmt.Sprintf("%s@%s.ampr.org", strings.ToLower(e.def.Exercise.MyCall), strings.ToLower(e.def.Exercise.BBSName)),
	}).String()
}

// generateValue builds a message field value from a template that may have
// interpolated variables.
func (e *Engine) generateValue(tmpl definition.StringWithInterps, station string) (val string) {
	var sb strings.Builder

	for i := range len(tmpl.Variables) {
		sb.WriteString(tmpl.Literals[i])
		if vval, ok := e.Variable(tmpl.Variables[i], station); !ok {
			e.st.LogError(fmt.Errorf("no such variable %q", tmpl.Variables[i]))
		} else {
			sidx, eidx := tmpl.StartOffsets[i], tmpl.EndOffsets[i]
			if sidx < 0 {
				sidx += len(vval)
			}
			if eidx <= 0 {
				eidx += len(vval)
			}
			sidx = max(min(sidx, len(vval)-1), 0)
			eidx = max(min(eidx, len(vval)), 0)
			vval = vval[sidx:eidx]
			if tmpl.Additions[i] != "" {
				vval = e.applyAddition(tmpl.Variables[i], vval, tmpl.Additions[i])
			}
			sb.WriteString(vval)
		}
	}
	sb.WriteString(tmpl.Literals[len(tmpl.Variables)])
	return sb.String()
}

// applyAddition performs the addition described in a variable interpolation.
func (e *Engine) applyAddition(vname, val, add string) string {
	if v, err := strconv.Atoi(add); err == nil {
		if v2, err := strconv.Atoi(val); err == nil {
			return strconv.Itoa(v + v2)
		} else {
			e.st.LogError(fmt.Errorf("variable interpolation: can't add integer to non-integer %s", vname))
			return val
		}
	}
	dur, _ := definition.ParseDuration(add)
	for _, fmt := range []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"01/02/2006 15:04",
		"2006-01-02",
		"01/02/2006",
		"15:04",
	} {
		if t, err := time.ParseInLocation(fmt, val, time.Local); err == nil {
			return t.Add(dur).Format(fmt)
		}
	}
	e.st.LogError(fmt.Errorf("variable interpolation: can't add duration to non-date/time value %s", vname))
	return val
}
