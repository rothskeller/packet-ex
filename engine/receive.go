package engine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/delivrcpt"
	"github.com/rothskeller/packet/xscmsg/readrcpt"
)

// receiveMessage receives and processes the BBS message with the specified
// number.  It returns whether it was successful.
func (e *Engine) receiveMessage(conn BBSConnection, msgnum int) bool {
	var w incident.Warning

	// Read the message.
	raw, err := conn.Read(msgnum)
	if err != nil {
		e.st.LogError(fmt.Errorf("JNOS read message: %w", err))
		return false
	}
	if raw == "" {
		return false
	}
	// Record receipt of the message.
	lmi, env, msg, oenv, omsg, err := incident.ReceiveMessage(
		raw, e.def.Exercise.BBSName, "", e.def.Exercise.StartMsgID, e.def.Exercise.OpCall, e.def.Exercise.OpName)
	if err != nil && !errors.As(err, &w) {
		e.st.LogError(fmt.Errorf("record received message: %w", err))
		return false
	}
	// Received receipts are handled differently than other messages.
	switch msg := msg.(type) {
	case nil:
		// ignore message (e.g. autoresponse)
	case *readrcpt.ReadReceipt:
		// ignore read receipts
	case *delivrcpt.DeliveryReceipt:
		// Record the delivery of the message.
		if lmi != "" {
			if ev := e.st.ReceiveDeliveryReceipt(lmi, msg.LocalMessageID); ev != nil {
				if stn := e.def.Station(ev.Station()); stn != nil && stn.NoReceipts {
					e.st.Execute("    WARNING: unexpected delivery receipt")
				}
			}
		}
	default:
		// If we have oenv/omsg, it's a delivery receipt to be sent.
		if oenv != nil {
			if err = e.sendDeliveryReceipt(conn, lmi, oenv, omsg.(*delivrcpt.DeliveryReceipt)); err != nil {
				e.st.LogError(fmt.Errorf("send delivery receipt for %s: %w", lmi, err))
				return false
			}
		}
		e.processReceivedMessage(conn, raw, lmi, env, msg)
	}
	// Kill the received message from the BBS.
	if err = conn.Kill(msgnum); err != nil {
		e.st.LogError(fmt.Errorf("JNOS kill message: %w", err))
		return false
	}
	return true
}

func (e *Engine) sendDeliveryReceipt(conn BBSConnection, lmi string, env *envelope.Envelope, dr *delivrcpt.DeliveryReceipt) (err error) {
	env.From = (&envelope.Address{
		Name:    e.def.Exercise.MyName,
		Address: strings.ToLower(e.def.Exercise.MyCall + "@" + e.def.Exercise.BBSName + ".scc-ares-races.org"),
	}).String()
	env.Date = e.st.Now()
	dr.SetOperator(e.def.Exercise.OpCall, e.def.Exercise.OpName, false)
	body := dr.EncodeBody()
	var to []string
	if addrs, err := envelope.ParseAddressList(env.To); err != nil {
		return errors.New("invalid To: address list")
	} else if len(addrs) == 0 {
		return errors.New("no To: addresses")
	} else {
		to = make([]string, len(addrs))
		for i, a := range addrs {
			to[i] = a.Address
		}
	}
	if err = conn.Send(env.SubjectLine, env.RenderBody(body), to...); err != nil {
		return fmt.Errorf("JNOS send message: %w", err)
	}
	if err = incident.SaveReceipt(lmi, env, dr); err != nil {
		return fmt.Errorf("save receipt: %s", err)
	}
	return nil
}

func (e *Engine) processReceivedMessage(conn BBSConnection, raw, lmi string, env *envelope.Envelope, msg message.Message) {
	// Determine the return address.
	var from = env.From
	if addrs, err := envelope.ParseAddressList(from); err == nil && len(addrs) != 0 {
		from = addrs[0].Address
	}
	if strings.IndexByte(from, ' ') >= 0 {
		from = "" // can't record that
	}
	// Which station is it from?
	var station = e.stationFromAddress(env.From)
	if station.CallSign == "UNKNOWN" {
		e.st.RecordReject(station.CallSign, "-", lmi, from, env.SubjectLine)
		e.rejectUnknownSender(conn, env)
		return
	}
	// Which message template does it match?
	var msgname = e.matchMessage(env.SubjectLine, msg)
	if msgname == "UNKNOWN" {
		e.st.RecordReject(station.CallSign, msgname, lmi, from, env.SubjectLine)
		e.rejectUnknownMessage(conn, env)
		return
	}
	// Record the reception of the message.
	var ev = e.st.ReceiveMessage(station.CallSign, msgname, lmi, from, env.SubjectLine)
	// Analyze the message.
	var problems, score = e.analyze(station, msgname, raw, lmi, env, msg)
	// Record the analysis of the message.
	e.st.ScoreMessage(ev, problems, score)
	// Trigger any events based on this message.
	if !ev.Expected().IsZero() {
		e.runTriggers(ev)
	} else {
		e.st.Execute("    ERROR: unexpected/early message")
	}
}

// stationFromAddress returns the defined station corresponding to the call sign
// extracted from the supplied message address.  If there is no match, it will
// return an artificial "station" with the call sign "UNKNOWN".
func (e *Engine) stationFromAddress(addrs string) *definition.Station {
	if alist, err := envelope.ParseAddressList(addrs); err == nil && len(alist) != 0 {
		name, _, _ := strings.Cut(alist[0].Address, "@")
		name = strings.ToUpper(name)
		for _, stn := range e.def.Stations {
			if stn.CallSign == name {
				return stn
			}
		}
	}
	return &definition.Station{CallSign: "UNKNOWN"}
}

// matchMessage returns the message name of the supplied message, or "UNKNOWN"
// if the supplied message doesn't match any defined message.
func (e *Engine) matchMessage(subjectline string, msg message.Message) (name string) {
	var (
		subject = subjectline
		formtag = msg.Base().Type.Tag
	)
	if sf := msg.Base().FSubject; sf != nil {
		subject = *sf
	}
	for _, md := range e.def.MatchReceive {
		if md.Type != "" && md.Type != formtag {
			continue
		}
		if md.Subject != "" && !strings.EqualFold(md.Subject, subject) {
			continue
		}
		if md.SubjectRE != nil && !md.SubjectRE.MatchString(subject) {
			continue
		}
		return md.Name
	}
	return "UNKNOWN"
}

// rejectUnknownSender sends a message back to the sender saying that we don't
// know who they are.
func (e *Engine) rejectUnknownSender(conn BBSConnection, reject *envelope.Envelope) {
	var body = fmt.Sprintf(`%s received a message from you with
  Subject: %s
The mailbox you sent this message from does not correspond to any station
participating in the current exercise.  Please make sure you are sending from
the correct mailbox (e.g., your assigned tactical callsign, not your personal
FCC callsign).  If you cannot find the problem, ask for help from the exercise
manager.`, e.def.Exercise.MyName, reject.SubjectLine)
	if err := conn.Send("REJECT: "+reject.SubjectLine, body, reject.From); err != nil {
		e.st.LogError(fmt.Errorf("sending reject message: %w", err))
	}
}

// rejectUnknownMessage sends a message back to the sender saying that we
// couldn't recognize their message.
func (e *Engine) rejectUnknownMessage(conn BBSConnection, reject *envelope.Envelope) {
	var body = fmt.Sprintf(`%s received a message from you with
  Subject: %s
This subject line does not match any of the messages the exercise automation
was expecting to receive.  Please check the subject line and try again.  If you
cannot find the problem, ask for help from the exercise manager.`,
		e.def.Exercise.MyName, reject.SubjectLine)
	if err := conn.Send("REJECT: "+reject.SubjectLine, body, reject.From); err != nil {
		e.st.LogError(fmt.Errorf("sending reject message: %w", err))
	}
}
