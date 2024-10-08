package main

import (
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	var (
		fname string
		def   *definition.Definition
		st    *state.State
		err   error
	)
	// Read the command line for the exercise definition filename.
	switch len(os.Args) {
	case 1:
		fname = "exercise.def"
	case 2:
		fname = os.Args[1]
	default:
		fmt.Fprintln(os.Stderr, "usage: pxtex-repots [definition-file]")
		os.Exit(2)
	}
	// If the exercise definition file is in a different directory, make
	// that the current working directory so we can reach all incident files
	// saved there.
	if dir := filepath.Dir(fname); dir != "." {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
		fname = filepath.Base(fname)
	}
	// Read the exercise definition.
	xscmsg.Register()
	if def, err = definition.Read(fname); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// Create a state tracker.
	st = state.New(true)
	// Read the exercise state.
	fname = strings.TrimSuffix(fname, ".def") + ".log"
	if err = st.Open(fname); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// Generate a report for each station in the definition.
	for _, stn := range def.Stations {
		genReport(def, st, stn)
	}
}

func genReport(def *definition.Definition, st *state.State, stn *definition.Station) {
	var (
		fh     *os.File
		groups []string
		err    error
	)
	if fh, err = os.Create(fmt.Sprintf("%s-report.html", stn.CallSign)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer fh.Close()
	io.WriteString(fh, "<h1>Packet Exercise Report</h1>\n<table>\n")
	fmt.Fprintf(fh, "  <tr><td>Incident</td><td>%s %s</td></tr>\n",
		html.EscapeString(def.Exercise.Incident), html.EscapeString(def.Exercise.Activation))
	if stn.CallSign != stn.FCCCall {
		fmt.Fprintf(fh, "  <tr><td>Call Sign</td><td>%s (%s)</td></tr>\n", stn.CallSign, stn.FCCCall)
	} else {
		fmt.Fprintf(fh, "  <tr><td>Call Sign</td><td>%s</td></tr>\n", stn.CallSign)
	}
	if stn.Position != "" && stn.Location != "" {
		fmt.Fprintf(fh, "  <tr><td>Station</td><td>%s / %s</td></tr>\n",
			html.EscapeString(stn.Position), html.EscapeString(stn.Location))
	} else if stn.Position != "" {
		fmt.Fprintf(fh, "  <tr><td>Position</td><td>%s</td></tr>\n", html.EscapeString(stn.Position))
	} else if stn.Location != "" {
		fmt.Fprintf(fh, "  <tr><td>Location</td><td>%s</td></tr>\n", html.EscapeString(stn.Location))
	}
	io.WriteString(fh, "</table>\n")
	for _, event := range def.Events {
		if event.Group != "" && !slices.Contains(groups, event.Group) {
			groups = append(groups, event.Group)
		}
	}
	if slices.ContainsFunc(def.Events, func(ev *definition.Event) bool { return ev.Group == "" }) {
		groups = append(groups, "")
	}
	for _, group := range groups {
		genGroupReport(fh, def, st, stn, group)
	}
	genRejectedMessages(fh, def, st, stn)
}

func genGroupReport(fh io.Writer, def *definition.Definition, st *state.State, stn *definition.Station, group string) {
	var started bool

	for _, edef := range def.Events {
		if edef.Group != group {
			continue
		}
		ev := st.FindEvent(edef.Type, stn.CallSign, edef.Name)
		if ev == nil {
			continue
		}
		if !started {
			if group == "" {
				io.WriteString(fh, "\n<h2>Other Events</h2>\n")
			} else {
				fmt.Fprintf(fh, "\n<h2>%s Events</h2>\n", html.EscapeString(group))
			}
			started = true
		}
		switch edef.Type {
		case definition.EventAlert:
			genAlertReport(fh, def, edef, ev, stn)
		case definition.EventBulletin:
			genBulletinReport(fh, def, edef, ev, stn)
		case definition.EventDeliver:
			genDeliverReport(fh, def, edef, ev, stn)
		case definition.EventInject:
			genInjectReport(fh, def, edef, ev, stn)
		case definition.EventReceipt:
			genReceiptReport(fh, def, edef, ev, stn)
		case definition.EventReceive:
			genReceiveReport(fh, def, edef, ev, stn)
		case definition.EventReject:
			genRejectReport(fh, def, edef, ev, stn)
		case definition.EventSend:
			genSendReport(fh, def, edef, ev, stn)
		}
	}
}

func genAlertReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>%s was expected to notify %s by voice that the IMMEDIATE message %s had been sent.  This notification was not recorded.</p>\n",
			stn.CallSign, def.Exercise.MyCall, html.EscapeString(edef.Name))
	} else if !ev.Expected().IsZero() && ev.Occurred().After(ev.Expected()) {
		fmt.Fprintf(fh, "<p>At %s, %s notified %s by voice that the IMMEDIATE message %s had been sent.  This was %s later than expected.</p>\n",
			formatDateTime(def, ev.Occurred()), stn.CallSign, def.Exercise.MyCall, html.EscapeString(edef.Name), formatDuration(ev.Occurred().Sub(ev.Expected())))
	} else {
		fmt.Fprintf(fh, "<p>At %s, %s notified %s by voice that the IMMEDIATE message %s had been sent.</p>\n",
			formatDateTime(def, ev.Occurred()), stn.CallSign, def.Exercise.MyCall, html.EscapeString(edef.Name))
	}
}

func genBulletinReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if !ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>At %s, bulletin %s became available for %s to retrieve.</p>\n",
			formatDateTime(def, ev.Occurred()), html.EscapeString(edef.Name), stn.CallSign)
	}
}

func genDeliverReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>%s was expected to deliver to their principal a printed copy of received message %s.  This delivery was not recorded.</p>\n",
			stn.CallSign, html.EscapeString(edef.Name))
	} else if !ev.Expected().IsZero() && ev.Occurred().After(ev.Expected()) {
		fmt.Fprintf(fh, "<p>At %s, %s delivered to their principal a printed copy of received message %s.  This was %s later than expected.</p>\n",
			formatDateTime(def, ev.Occurred()), stn.CallSign, html.EscapeString(edef.Name), formatDuration(ev.Occurred().Sub(ev.Expected())))
	} else {
		fmt.Fprintf(fh, "<p>At %s, %s delivered to their principal a printed copy of received message %s message.</p>\n",
			formatDateTime(def, ev.Occurred()), stn.CallSign, html.EscapeString(edef.Name))
	}
}

func genInjectReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if !ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>At %s, the principal handed %s the message %s to be sent to %s.</p>\n",
			formatDateTime(def, ev.Occurred()), stn.CallSign, html.EscapeString(edef.Name), def.Exercise.MyCall)
	}
}

func genReceiptReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	// TODO
}

func genReceiveReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>%s expected to receive the message %s from %s.  This message was not received.</p>\n",
			def.Exercise.MyCall, html.EscapeString(edef.Name), stn.CallSign)
		return
	}
	fmt.Fprintf(fh, "<p>At %s, %s received the message %s from %s.",
		formatDateTime(def, ev.Occurred()), def.Exercise.MyCall, html.EscapeString(edef.Name), stn.CallSign)
	if !ev.Expected().IsZero() && ev.Occurred().After(ev.Expected()) {
		fmt.Fprintf(fh, "  This was %s later than expected.", formatDuration(ev.Occurred().Sub(ev.Expected())))
	}
	if ev.Score() != 0 {
		fmt.Fprintf(fh, "  The message had a transcription score of %d%%.", ev.Score())
	}
	io.WriteString(fh, "</p>\n")
}

func genRejectReport(fh io.Writer, def *definition.Definition, _ *definition.Event, ev *state.Event, stn *definition.Station) {
	fmt.Fprintf(fh, "<p>At %s, a message from %s was rejected by the automation.  It could not be matched to any expected message.</p>\n",
		formatDateTime(def, ev.Occurred()), stn.CallSign)
}

func genSendReport(fh io.Writer, def *definition.Definition, edef *definition.Event, ev *state.Event, stn *definition.Station) {
	if !ev.Occurred().IsZero() {
		fmt.Fprintf(fh, "<p>At %s, %s sent the message %s to %s.</p>\n",
			formatDateTime(def, ev.Occurred()), def.Exercise.MyCall, html.EscapeString(edef.Name), stn.CallSign)
	}
}

// formatDuration renders a duration as a string.
func formatDuration(d time.Duration) (s string) {
	hr, mi := d/time.Hour, (d%time.Hour)/time.Minute
	switch hr {
	case 0:
		// nothing
	case 1:
		s = "1 hour"
	default:
		s = fmt.Sprintf("%d hours", hr)
	}
	if s != "" && mi == 0 {
		return s
	}
	if s != "" {
		s += " "
	}
	if mi == 1 {
		s += "1 minute"
	} else {
		s += fmt.Sprintf("%d minutes", mi)
	}
	return s
}

func formatDateTime(def *definition.Definition, dt time.Time) string {
	if def.Exercise.OpStart.Format("2006-01-02") != def.Exercise.OpEnd.Format("2006-01-02") {
		return dt.Format("2006-01-02 15:04")
	} else {
		return dt.Format("15:04")
	}
}

func genRejectedMessages(fh io.Writer, def *definition.Definition, st *state.State, stn *definition.Station) {
	var started bool

	for _, ev := range st.AllEvents() {
		if ev.Station() != stn.CallSign || ev.Type() != definition.EventReject {
			continue
		}
		if !started {
			io.WriteString(fh, "\n<h2>Unrecognized Messages</h2>\n")
			started = true
		}
		genRejectReport(fh, def, nil, ev, stn)
	}
}
