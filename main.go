// Main program for the exercise automation.
//
// usage: packet-ex [model-file]
//
// The command line argument is the filename of the exercise automation file.
// It defaults to "exercise.yaml".
package main

import (
	"fmt"
	"os"

	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/monitor"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	var (
		modelfname string
		modelfh    *os.File
		m          *model.Model
		mon        *monitor.Monitor
		e          *engine.Engine
		err        error
	)
	// Parse arguments and read model file.
	xscmsg.Register()
	switch len(os.Args) {
	case 1:
		modelfname = "exercise.yaml"
	case 2:
		modelfname = os.Args[1]
	default:
		fmt.Fprintf(os.Stderr, "usage: packet-ex [model-file]\n")
		os.Exit(2)
	}
	if modelfh, err = os.Open(modelfname); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	if m, err = model.Read(modelfh); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s\n", modelfname, err)
		os.Exit(1)
	}
	mon = monitor.New(m)
	e = engine.New(m, mon)
	go e.Run()
	// Start the engine.
	if err = mon.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
