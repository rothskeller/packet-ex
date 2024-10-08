package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/server"
	"github.com/rothskeller/packet-ex/state"
	"github.com/rothskeller/packet/xscmsg"
)

var now time.Time
var tick time.Time
var e *engine.Engine
var tickch chan time.Time
var mtch chan server.ManualTrigger

func main() {
	var (
		def *definition.Definition
		st  *state.State
		err error
	)
	os.Chdir("/Users/stever/src/packet/packet-ex/t")
	// Read the exercise definition.  This also verifies we're in an
	// exercise directory.
	xscmsg.Register()
	if def, err = definition.Read("exercise.def"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// Clean out anything left over from previous attempts.
	os.Remove("exercise.log")
	saved, _ := filepath.Glob("X*")
	for _, f := range saved {
		os.Remove(f)
	}
	sent, _ := filepath.Glob("*/S*")
	for _, f := range sent {
		os.Remove(f)
	}
	injected, _ := filepath.Glob("*/I*")
	for _, f := range injected {
		os.Remove(f)
	}
	// Create a state tracker.
	st = state.New(true)
	// Set the state "now" to the opstart time.
	now, tick = def.Exercise.OpStart, def.Exercise.OpStart
	st.SetNowFunc(func() time.Time {
		now = now.Add(time.Millisecond)
		return now
	})
	// Create the exercise engine.
	if e, err = engine.New(def, st); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	// Create a fake ticker and give it to the engine.
	tickch = make(chan time.Time)
	e.SetTicker(tickch)
	// Create a manual event trigger channel and give it to the engine.
	mtch = make(chan server.ManualTrigger)
	e.SetManualTriggerChannel(mtch)
	// Create a fake connector.
	e.SetBBSConnector(func(*definition.Exercise) (engine.BBSConnection, error) { return new(connection), nil })
	e.SetNoInject()
	// Run the engine.
	go e.Run()
	// Run the simulation steps.
	for _, step := range steps {
		fmt.Print("> ")
		bufio.NewScanner(os.Stdin).Scan()
		step()
		// Give some time for the output from that step before printing
		// the prompt for the next step.
		time.Sleep(100 * time.Millisecond)
	}
}

var steps = []func(){}

func runTick(h, m int) {
	tick = time.Date(2023, 9, 23, h, m, 0, 0, time.Local)
	now = tick
	fmt.Printf("TICK = %s\n", now.Format("15:04"))
	tickch <- tick
}

type connection struct{}

func (c *connection) Read(msgnum int) (string, error) {
	fmt.Printf("CONN: read %d\n", msgnum)
	if c, err := os.ReadFile(fmt.Sprintf("%s/R%d", tick.Format("15:04"), msgnum)); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	} else {
		return string(c), nil
	}
}

func (c *connection) Kill(msgnum ...int) error {
	fmt.Printf("CONN: kill %d\n", msgnum[0])
	return nil
}

func (c *connection) Send(subject string, body string, to ...string) error {
	outnum := 1
	var fname string
	for {
		fname = fmt.Sprintf("%s/S%d", tick.Format("15:04"), outnum)
		if _, err := os.Stat(fname); os.IsNotExist(err) {
			break
		}
		outnum++
	}
	os.Mkdir(tick.Format("15:04"), 0777)
	fh, _ := os.Create(fname)
	fmt.Printf("CONN: send %s\n", strings.Join(to, " "))
	fmt.Fprintf(fh, "CONN: send %s\n", strings.Join(to, " "))
	// fmt.Printf("Subject: %s\n", subject)
	fmt.Fprintf(fh, "Subject: %s\n", subject)
	// fmt.Println(body)
	fmt.Fprint(fh, body)
	fh.Close()
	return nil
}

func (c *connection) Close() error {
	fmt.Println("CONN: close")
	return nil
}
