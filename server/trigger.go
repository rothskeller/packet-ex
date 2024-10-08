package server

import (
	"net/http"

	"github.com/rothskeller/packet-ex/definition"
)

// serveManualTrigger is called for a POST /manualTrigger.  It puts the manual
// event trigger onto the trigger channel.
func (m *Monitor) serveManualTrigger(w http.ResponseWriter, r *http.Request) {
	var (
		mt  ManualTrigger
		err error
	)
	etypestr := r.FormValue("type")
	if mt.Type, err = definition.ParseEventType(etypestr); err != nil {
		http.Error(w, "invalid type", http.StatusBadRequest)
		return
	}
	mt.Station, mt.Name = r.FormValue("station"), r.FormValue("name")
	if m.mtch != nil {
		m.mtch <- mt
	}
	w.WriteHeader(http.StatusNoContent)
}
