package engine

import (
	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
)

func (e *Engine) runTriggers(ev *state.Event) (err error) {
	var evs = []*state.Event{ev}
	for len(evs) != 0 {
		if add, err := e.runTriggersForOne(evs[0]); err != nil {
			return err
		} else {
			evs = append(evs, add...)
		}
		evs = evs[1:]
	}
	return nil
}
func (e *Engine) runTriggersForOne(trigger *state.Event) (cascade []*state.Event, err error) {
	// Expect a delivery receipt if the triggering event sent a message to a
	// station with a delivery receipt delay time.
	if trigger.Type() == definition.EventSend {
		if stn := e.def.Station(trigger.Station()); stn != nil && stn.ReceiptDelay != 0 {
			e.st.ExpectEvent(definition.EventReceipt, stn.CallSign, trigger.Name(), trigger.Occurred().Add(stn.ReceiptDelay), trigger.ID())
		}
	}
	// Look for events that should be triggered by this one.
	for _, edef := range e.def.Events {
		if add, err := e.maybeTriggerEvent(trigger, edef); err != nil {
			return nil, err
		} else if add != nil {
			cascade = append(cascade, add)
		}
	}
	return cascade, err
}
func (e *Engine) maybeTriggerEvent(trigger *state.Event, edef *definition.Event) (cascade *state.Event, err error) {
	// Is edef triggered by the event we're running triggers for?
	if edef.TriggerType != trigger.Type() || edef.TriggerName != trigger.Name() {
		return
	}
	// Is there a condition on the triggering of edef, and is it met?
	if !e.triggerConditionMet(edef, trigger) {
		return
	}
	// Bulletin events can only be triggered globally, and are the only
	// events that can be triggered globally.
	if (edef.Type == definition.EventBulletin) != (trigger.Station() == "") {
		return
	}
	// Schedule or expect the event, depending on its type.
	switch edef.Type {
	case definition.EventBulletin:
		// On trigger of a global bulletin, schedule it both globally
		// and for all defined stations.
		e.st.ScheduleEvent(edef.Type, "", edef.Name, trigger.Occurred().Add(edef.Delay), trigger.ID())
		for _, stn := range e.def.Stations {
			e.st.ScheduleEvent(edef.Type, stn.CallSign, edef.Name, trigger.Occurred().Add(edef.Delay), trigger.ID())
		}
	case definition.EventInject, definition.EventSend:
		// On trigger of an inject or send, schedule it.
		e.st.ScheduleEvent(edef.Type, trigger.Station(), edef.Name, trigger.Occurred().Add(edef.Delay), trigger.ID())
	case definition.EventAlert, definition.EventDeliver, definition.EventReceive:
		// On trigger of an alert, deliver, or receive, add the
		// expectation for it.
		target := e.st.ExpectEvent(edef.Type, trigger.Station(), edef.Name, trigger.Occurred().Add(edef.Delay), trigger.ID())
		if target.LMI() != "" && target.Occurred().IsZero() {
			// This is a received message that came in before it was
			// expected.  We'll treat it as received now, and then
			// trigger its events.
			e.st.ReceiveMessage(target.Station(), target.Name(), target.LMI(), "", "")
			e.runTriggers(target)
		}
	default:
		// Definition parser shouldn't let anything else
		// through, so this is a software bug.
		panic("unexpected event type in runTriggers")
	}
	return cascade, err
}

func (e *Engine) triggerConditionMet(te *definition.Event, ev *state.Event) bool {
	if te.ConditionVar == "" {
		return true
	}
	have, ok := e.Variable(te.ConditionVar, ev.Station())
	if !ok {
		return false
	}
	switch te.ConditionOp {
	case "=":
		return have == te.ConditionVal
	case "!=":
		return have != te.ConditionVal
	case "<":
		return have < te.ConditionVal
	case "<=":
		return have <= te.ConditionVal
	case ">":
		return have > te.ConditionVal
	case ">=":
		return have >= te.ConditionVal
	case "≈":
		return te.ConditionRE.MatchString(have)
	}
	// Definition parser shouldn't let anything else through, so this is a
	// software bug.
	panic("not reachable")
}
