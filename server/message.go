package server

import (
	"net/http"
	"os"
	"strings"

	"github.com/rothskeller/packet/incident"
)

func ServeMessage(w http.ResponseWriter, r *http.Request) {
	var filename = strings.TrimPrefix(r.URL.Path, "/message/")
	if !strings.HasSuffix(filename, ".pdf") {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	lmi := filename[:len(filename)-4]
	if !incident.MsgIDRE.MatchString(lmi) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if _, err := os.Stat(lmi + ".txt"); os.IsNotExist(err) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if _, err := os.Stat(lmi + ".pdf"); err != nil {
		env, msg, err := incident.ReadMessage(lmi)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = msg.RenderPDF(env, lmi+".pdf"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
	http.ServeFile(w, r, lmi+".pdf")
}
