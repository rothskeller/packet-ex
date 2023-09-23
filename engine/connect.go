package engine

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/variables"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/jnos"
	"github.com/rothskeller/packet/jnos/telnet"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/delivrcpt"
	"github.com/rothskeller/packet/xscmsg/readrcpt"
)

func (e *Engine) runBBSConnection() (err error) {
	var (
		mailbox string
		conn    *jnos.Conn
	)
	// Connect to the BBS.
	if e.model.Identity.TacCall != "" {
		mailbox = e.model.Identity.TacCall
	} else {
		mailbox = e.model.Identity.FCCCall
	}
	if conn, err = telnet.Connect(e.model.BBS.Address, mailbox, e.model.BBS.Password, e.connlog); err != nil {
		e.log("ERROR: BBS connect: %s", err)
		return fmt.Errorf("connecting to %s as %s: %w", e.model.BBS.Name, mailbox, err)
	}
	defer func() {
		if err2 := conn.Close(); err == nil && err2 != nil {
			e.log("ERROR: BBS close: %s", err2)
			err = fmt.Errorf("closing connection: %w", err2)
		}
	}()
	// Receive messages.
	if err = e.receiveMessages(conn); err != nil {
		return fmt.Errorf("receiving messages: %w", err)
	}
	// Send messages.
	if err = e.sendMessages(conn); err != nil {
		return fmt.Errorf("sending messages: %w", err)
	}
	return nil
}

func (e *Engine) receiveMessages(conn *jnos.Conn) (err error) {
	var msgnum = 1
	for {
		var done bool
		if done, err = e.receiveMessage(conn, msgnum); err != nil {
			return fmt.Errorf("receiving message %d: %w", msgnum, err)
		}
		if done {
			break
		}
		msgnum++
	}
	return nil
}

func (e *Engine) receiveMessage(conn *jnos.Conn, msgnum int) (done bool, err error) {
	// Read the message.
	raw, err := conn.Read(msgnum)
	if err != nil {
		e.log("ERROR: JNOS receive: %s", err)
		return false, fmt.Errorf("JNOS read %d: %s", msgnum, err)
	}
	if raw == "" {
		return true, nil
	}
	// Record receipt of the message.
	lmi, env, msg, oenv, omsg, err := incident.ReceiveMessage(
		raw, e.model.BBS.Name, "", e.model.StartMsgID, e.model.Identity.FCCCall, e.model.Identity.FCCName)
	if err == incident.ErrDuplicateReceipt {
		goto KILL
	}
	if err != nil {
		e.log("ERROR: incident receive: %s", err)
		return false, err
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
			for _, sent := range e.Sent {
				if sent.LMI == lmi {
					sent.Delivered = e.PassTime
					sent.RMI = msg.LocalMessageID
					break
				}
			}
		}
	default:
		// If we have oenv/omsg, it's a delivery receipt to be sent.
		if oenv != nil {
			if err = e.sendMessage(conn, lmi+".DR", oenv, omsg); err != nil {
				return false, fmt.Errorf("sending delivery receipt for %s: %w", lmi, err)
			}
		}
		e.runReceivedRules(lmi, env, msg)
	}
KILL:
	if err = conn.Kill(msgnum); err != nil {
		e.log("ERROR: JNOS kill: %s", err)
		return false, fmt.Errorf("JNOS kill %d: %s", msgnum, err)
	}
	return false, nil
}

func (e *Engine) sendMessages(conn *jnos.Conn) (err error) {
	j := 0
	for _, ds := range e.DelayedSend {
		if !ds.Time.After(e.PassTime) {
			var lmi = incident.UniqueMessageID(e.model.StartMsgID)
			var varsrc = variables.Merged{
				variables.Single("nextid", lmi),
				nowSource(e.PassTime),
				e.UserVars, e.sentIDs, e.model,
			}
			if ds.Reply != "" {
				env, msg, err := incident.ReadMessage(ds.Reply)
				if err != nil {
					e.log("ERROR: reply to %s failed: original not readable: %s", ds.Reply, err)
					continue
				}
				pdef := e.participantForMessage(env)
				if pdef == nil {
					e.log("ERROR: reply to %s failed: participant no longer defined", ds.Reply)
					continue
				}
				mvars := model.VariablesForMessage(lmi, env, msg)
				mdef := e.messageDefForMessage(mvars, pdef)
				varsrc = append(varsrc, e.receivedMessageVariables(pdef, mdef, mvars))
			}
			var msg = model.CreateMessage(e.model.MessageMap[ds.MName], varsrc)
			var env = &envelope.Envelope{
				To:          ds.To,
				SubjectLine: msg.EncodeSubject(),
			}
			if err = e.sendMessage(conn, lmi, env, msg); err != nil {
				return fmt.Errorf("sending %s: %w", lmi, err)
			}
			if ds.PName != "" {
				e.log("Sent to %s: %s", ds.PName, env.SubjectLine)
			} else {
				e.log("Sent to %s: %s", ds.To, env.SubjectLine)
			}
			e.Sent = append(e.Sent, &Sent{
				LMI:     lmi,
				PName:   ds.PName,
				MName:   ds.MName,
				Sent:    e.PassTime,
				Subject: env.SubjectLine,
			})
			if ds.PName != "" {
				if e.sentIDs[ds.PName] == nil {
					e.sentIDs[ds.PName] = make(map[string]string)
				}
				e.sentIDs[ds.PName][ds.MName] = lmi
			}
			continue
		}
		e.DelayedSend[j] = ds
		j++
	}
	e.DelayedSend = e.DelayedSend[:j]
	return nil
}

// sendMessage sends a single message.  It is used for both outgoing human
// messages and delivery receipts.
func (e *Engine) sendMessage(conn *jnos.Conn, filename string, env *envelope.Envelope, msg message.Message) (err error) {
	if e.model.Identity.TacCall != "" {
		env.From = (&envelope.Address{Name: e.model.Identity.TacName, Address: strings.ToLower(e.model.Identity.TacCall + "@" + e.model.BBS.Name + ".ampr.org")}).String()
	} else {
		env.From = (&envelope.Address{Name: e.model.Identity.FCCName, Address: strings.ToLower(e.model.Identity.FCCCall + "@" + e.model.BBS.Name + ".ampr.org")}).String()
	}
	env.Date = time.Now()
	msg.SetOperator(e.model.Identity.FCCCall, e.model.Identity.FCCName, false)
	body := msg.EncodeBody()
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
		e.log("ERROR: JNOS send: %s", err)
		return fmt.Errorf("JNOS send: %s", err)
	}
	if strings.HasSuffix(filename, ".DR") {
		if err = incident.SaveReceipt(filename[:len(filename)-3], env, msg); err != nil {
			e.log("ERROR: incident save: %s", err)
			return fmt.Errorf("save receipt %s: %s", filename, err)
		}
	} else {
		if err = incident.SaveMessage(filename, "", env, msg, false); err != nil {
			e.log("ERROR: incident save: %s", err)
			return fmt.Errorf("save message %s: %s", filename, err)
		}
	}
	return nil
}
