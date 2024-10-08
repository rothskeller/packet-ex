package engine

import (
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/message"
)

// ClockTick handles a tick of the clock, performing all time-based actions.
func (e *Engine) ClockTick(tick time.Time) {
	// If the engine has not been started, do it now.
	if e.st.GetEvent(1) == nil {
		e.startExercise()
	}
	e.runBbsSession()
	e.generateInjects()
	e.st.MarkOverdueEvents(tick)
}

type BBSConnection interface {
	Read(msgnum int) (string, error)
	Kill(msgnums ...int) error
	Send(subject, body string, to ...string) error
	Close() error
}

func (e *Engine) runBbsSession() {
	var (
		conn BBSConnection
		ev   *state.Event
		lmi  string
		env  *envelope.Envelope
		msg  message.Message
		err  error
	)
	// Connect to the BBS.
	if conn, err = e.conn(e.def.Exercise); err != nil {
		e.st.LogError(err)
		return
	}
	defer func() {
		if err = conn.Close(); err != nil {
			e.st.LogError(err)
		}
	}()
	// Receive messages from the BBS.
	for msgnum := 1; true; msgnum++ {
		if !e.receiveMessage(conn, msgnum) {
			break
		}
	}
	// Post any bulletins that are due.
	for {
		if ev = e.st.PendingEvent(definition.EventBulletin); ev == nil {
			break
		}
		if lmi, env, msg = e.generateBulletin(ev); msg == nil {
			e.st.DropEvent(ev)
		}
		if err = e.sendMessage(conn, ev, lmi, env, msg); err != nil {
			return // transient error, retry next tick
		}
		e.st.SendMessage(definition.EventBulletin, "", ev.Name(), lmi, env.SubjectLine, ev.Trigger())
		if err = e.runTriggers(ev); err != nil {
			e.st.LogError(err)
		}
		for _, s := range e.def.Stations {
			sev := e.st.SendMessage(definition.EventBulletin, s.CallSign, ev.Name(), lmi, env.SubjectLine, ev.Trigger())
			if err = e.runTriggers(sev); err != nil {
				e.st.LogError(err)
			}
		}
	}
	// Send messages to the BBS.
	for {
		if ev = e.st.PendingEvent(definition.EventSend); ev == nil {
			break
		}
		if lmi, env, msg = e.generateSendMessage(ev); msg == nil {
			e.st.DropEvent(ev)
		}
		if err = e.sendMessage(conn, ev, lmi, env, msg); err != nil {
			return // transient error, retry next tick
		}
		e.st.SendMessage(ev.Type(), ev.Station(), ev.Name(), lmi, env.SubjectLine, ev.Trigger())
		if err = e.runTriggers(ev); err != nil {
			e.st.LogError(err)
		}
	}
}

func (e *Engine) generateInjects() {
	for {
		var (
			ev     *state.Event
			lmi    string
			msg    message.Message
			rmi    string
			method string
			err    error
		)
		if ev = e.st.PendingEvent(definition.EventInject); ev == nil {
			break
		}
		if lmi, _, msg = e.generateInject(ev); msg == nil {
			e.st.DropEvent(ev)
		}
		if of := msg.Base().FOriginMsgID; of != nil {
			rmi = *of
		}
		method = e.doInject(ev, lmi)
		e.st.CreateInject(ev.Station(), ev.Name(), rmi, method, ev.Trigger())
		if err = e.runTriggers(ev); err != nil {
			e.st.LogError(err)
		}
	}
}
