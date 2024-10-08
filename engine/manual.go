package engine

import (
	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/server"
)

func (e *Engine) ManualTrigger(mt server.ManualTrigger) {
	switch mt.Type {
	case definition.EventBulletin:
		if mt.Station == "" {
			// (Re-)schedule the bulletin send for next tick.
			e.st.ScheduleEvent(definition.EventBulletin, "", mt.Name, e.st.Now(), 0)
		}
	case definition.EventInject, definition.EventSend:
		if mt.Station != "" {
			// (Re-)schedule the event for next tick.
			e.st.ScheduleEvent(mt.Type, mt.Station, mt.Name, e.st.Now(), 0)
		}
	case definition.EventAlert, definition.EventDeliver, definition.EventReceive:
		if mt.Station != "" {
			// Mark the event as having occurred (creating it if
			// need be) and run associated triggers.
			ev := e.st.RecordEvent(mt.Type, mt.Station, mt.Name)
			if ev != nil {
				e.runTriggers(ev)
			}
		}
	}
	// We may have created events that are already due.  Bulletin and send
	// events need to wait for the next BBS connection, but injects can
	// happen right away.
	e.generateInjects()
}
