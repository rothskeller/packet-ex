package engine

import (
	"fmt"

	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
)

// sendMessage sends a message through the BBS connection.  Errors are logged.
// Only errors that might be transient (i.e., JNOS errors) are returned.
func (e *Engine) sendMessage(conn BBSConnection, ev *state.Event, lmi string, env *envelope.Envelope, msg message.Message) (err error) {
	env.Date = e.st.Now()
	msg.SetOperator(e.def.Exercise.MyCall, e.def.Exercise.MyName, false)
	body := msg.EncodeBody()
	var to []string
	if addrs, err := envelope.ParseAddressList(env.To); err != nil {
		e.st.LogError(fmt.Errorf("can't send %s: invalid To: address list", lmi))
		e.st.DropEvent(ev)
		return nil
	} else if len(addrs) == 0 {
		e.st.LogError(fmt.Errorf("can't send %s: no To: addresses", lmi))
		e.st.DropEvent(ev)
		return nil
	} else {
		to = make([]string, len(addrs))
		for i, a := range addrs {
			to[i] = a.Address
		}
	}
	if err := conn.Send(env.SubjectLine, env.RenderBody(body), to...); err != nil {
		e.st.LogError(fmt.Errorf("can't send %s: JNOS send: %w", lmi, err))
		return err // possibly transient, so return it
	}
	if err := incident.SaveMessage(lmi, "", env, msg, false, false); err != nil {
		e.st.LogError(fmt.Errorf("can't save sent %s: %w", lmi, err))
		// not transient error, so don't return it
	}
	return nil
}
