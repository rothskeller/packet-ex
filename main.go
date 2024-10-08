package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/jnos/telnet"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	var (
		fname   string
		def     *definition.Definition
		st      *state.State
		e       *engine.Engine
		err     error
		offline = flag.Bool("offline", false, "offline mode: no BBS connections, etc.")
	)
	// Read the command line for the exercise definition filename.
	flag.Parse()
	switch len(flag.Args()) {
	case 0:
		fname = "exercise.def"
	case 1:
		fname = flag.Arg(0)
	default:
		fmt.Fprintln(os.Stderr, "usage: packet-ex [definition-file]")
		os.Exit(2)
	}
	// If the exercise definition file is in a different directory, make
	// that the current working directory so that all incident files are
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
	// Create the exercise engine.
	if e, err = engine.New(def, st); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// Read the exercise state.
	fname = strings.TrimSuffix(fname, ".def") + ".log"
	if err = st.Open(fname); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// If the last state log entry is over a week old, then this is an
	// offline invocation even without the -offline flag.
	if last, _ := st.LastEntry(); !last.IsZero() && time.Since(last) > 7*24*time.Hour {
		*offline = true
	}
	// If we're online, register the ticker and the BBS connector.
	if !*offline {
		var jnosLog *os.File

		if jnosLog, err = os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
		e.SetTicker(e.StartTicker())
		e.SetBBSConnector(func(ex *definition.Exercise) (engine.BBSConnection, error) {
			return telnet.Connect(ex.BBSAddress, ex.MyCall, ex.BBSPassword, jnosLog)
		})
	}
	// Run the engine.  No fatal errors should be possible past this point.
	// (Panics may occur for software assertion errors only.)
	e.Run()
}
