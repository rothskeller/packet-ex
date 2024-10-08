// Package engine is the workhorse of the exercise engine.  It receives incoming
// events and takes the appropriate actions on them.
package engine

import (
	"fmt"
	"net"
	"time"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/server"
	"github.com/rothskeller/packet-ex/state"
)

type EngineEvent struct {
	Tick          time.Time
	ManualTrigger *server.ManualTrigger
}
type Engine struct {
	def      *definition.Definition
	st       *state.State
	monitor  *server.Monitor
	listener net.Listener
	conn     BBSConnector
	noinject bool
	tickch   <-chan time.Time
	mtch     chan server.ManualTrigger
}
type BBSConnector func(*definition.Exercise) (BBSConnection, error)

func New(def *definition.Definition, st *state.State) (e *Engine, err error) {
	e = &Engine{def: def, st: st}
	// Start a log server.
	var ls = server.NewLogServer(def.Exercise.OpStart.Format("2006-01-02") != def.Exercise.OpEnd.Format("2006-01-02"))
	st.AddListener(ls)
	// Start an overview server.
	e.mtch = make(chan server.ManualTrigger)
	e.monitor = server.NewMonitor(def, st, e.mtch)
	st.AddListener(e.monitor)
	// Listen on the webserver port (but don't accept any connections yet).
	// This can fail particularly if the listen port is already bound (i.e.,
	// another copy of the exercise engine is already running).
	if e.listener, err = server.Listen(def.Exercise.ListenAddr); err != nil {
		return nil, fmt.Errorf("server listen: %w", err)
	}
	return e, nil
}

// SetNoInject sets the flag that inhibits printing and emailing injects.
func (e *Engine) SetNoInject() {
	e.noinject = true
}

// SetBBSConnector sets the BBS connector to use for connecting to the BBS.
// This is typically called before Run.
func (e *Engine) SetBBSConnector(conn BBSConnector) {
	e.conn = conn
}

// SetTicker sets the channel that supplies ticks to the engine.  If called at
// all, it must be called before Run.
func (e *Engine) SetTicker(tickch <-chan time.Time) {
	e.tickch = tickch
}

// SetManualTriggerChannel sets the channel that supplies manual event triggers
// to the engine.  If called at all, it must be called before Run.
func (e *Engine) SetManualTriggerChannel(mtch chan server.ManualTrigger) {
	e.mtch = mtch
}

func (e *Engine) Run() {
	// Start any stations added to the definition since the last run.  This
	// is a no-op if we're in offline mode or the exercise hasn't started
	// yet.
	e.startNewStations()
	// Start the webserver.
	server.Start(e.listener)
	// Loop waiting for events.
	for {
		select {
		case mt := <-e.mtch:
			e.ManualTrigger(mt)
		case tick := <-e.tickch:
			e.ClockTick(tick)
		}
	}
}

func (e *Engine) startExercise() {
	var ev *state.Event

	// Start the exercise itself.
	ev = e.st.StartExercise()
	e.runTriggers(ev)
	// Start each of the stations defined in the exercise.
	for _, stn := range e.def.Stations {
		ev = e.st.StartStation(stn.CallSign)
		e.runTriggers(ev)
	}
}

// startNewStations starts any stations in the (new) exercise definition that
// aren't already started.
func (e *Engine) startNewStations() {
	if e.st.GetEvent(1) == nil {
		// Exercise hasn't started yet, so don't start any stations.
		return
	}
	if e.conn == nil {
		// Running in offline mode, so don't start any stations.
		return
	}
	// Look for any newly defined stations.
	for _, stn := range e.def.Stations {
		var ev *state.Event

		// If the station has already been started, there's nothing to
		// do.
		if e.st.StationStarted(stn.CallSign) {
			continue
		}
		// Start the station, and trigger any events based on the start
		// of the station.
		ev = e.st.StartStation(stn.CallSign)
		e.runTriggers(ev)
		// If any bulletins have been sent, "send" the bulletins to that
		// station and trigger any events based on that.
		for _, sb := range e.st.SentBulletins() {
			ev = e.st.SendMessage(sb.Type(), stn.CallSign, sb.Name(), sb.LMI(), "", sb.Trigger())
			e.runTriggers(ev)
		}
	}
}
