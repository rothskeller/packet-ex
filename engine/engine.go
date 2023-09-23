// Package engine contains the engine that drives the exercise.
package engine

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/variables"
	"github.com/rothskeller/wppsvr/config"
)

type Monitor interface {
	Update(*Engine)
}

func New(m *model.Model, mon Monitor) *Engine {
	return &Engine{
		model:    m,
		monitor:  mon,
		UserVars: make(variables.MapSource),
		sentIDs:  make(map[string]map[string]string),
	}
}

type Engine struct {
	model       *model.Model
	monitor     Monitor
	connlog     io.Writer
	enginelog   io.Writer
	PassTime    time.Time
	DelayedSets []*DelayedSet
	DelayedSend []*DelayedSend
	UserVars    variables.MapSource
	Sent        []*Sent
	Received    []*Received
	sentIDs     sentIDMap
}
type DelayedSet struct {
	Time  time.Time
	Name  string
	Value string
}
type DelayedSend struct {
	Time  time.Time
	MName string
	PName string
	To    string
	Reply string
}
type Sent struct {
	Sent      time.Time
	Delivered time.Time
	LMI       string
	RMI       string
	MName     string
	PName     string
	Subject   string
}
type Received struct {
	Sent     time.Time
	Received time.Time
	RMI      string
	LMI      string
	MName    string
	PName    string
	Subject  string
	Score    int
	Problems []string
}

func (e *Engine) Run() (err error) {
	var (
		logfile *os.File
	)
	config.Read()
	// Open the connection and engine log files.
	if logfile, err = os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	e.connlog = logfile
	if logfile, err = os.OpenFile("exercise.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	e.enginelog = logfile
	// Read the engine state from the file system.
	e.readState()
	e.log("ENGINE START")
	if !e.PassTime.IsZero() {
		e.monitor.Update(e) // initial paint, before the sleep below
	}
	for {
		// This code works in discrete minutes; we lop off the seconds
		// and milliseconds.
		now := minuteGranularity(time.Now())
		if e.PassTime.IsZero() {
			// If e.PassTime is zero, this is the first pass ever
			// for this exercise.
			e.PassTime = now
		} else {
			e.PassTime = addMinute(e.PassTime)
			time.Sleep(time.Until(e.PassTime))
		}
		// Run the timed rules for e.PassTime.
		e.runTimeBasedRules()
		// Normally, e.PassTime will equal now.  However, when we're
		// playing catchup, it doesn't.  We only want to connect to the
		// BBS after we're finished catching up.
		if !e.PassTime.Before(now) {
			e.runBBSConnection()
		}
		// Save the engine state in case we're interrupted and need to
		// restart.
		e.saveState()
		// Update the monitor.
		e.monitor.Update(e)
	}
}
func minuteGranularity(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
}
func addMinute(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()+1, 0, 0, t.Location())
}

func (e *Engine) log(f string, args ...any) {
	now := time.Now().Format("2006-01-02 15:04:05")
	if !e.PassTime.IsZero() {
		pass := e.PassTime.Format("2006-01-02 15:04")
		if now[:10] != pass[:10] {
			now += " [" + pass + "]"
		} else if now[:16] != pass {
			now += " [" + pass[11:] + "]"
		}
	}
	var msg string
	if len(args) == 0 {
		msg = fmt.Sprintf("%s %s", now, f)
	} else {
		msg = fmt.Sprintf("%s %s", now, fmt.Sprintf(f, args...))
	}
	fmt.Fprintln(e.enginelog, msg)
}

type nowSource time.Time

func (ns nowSource) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "now":
		return (time.Time)(ns).Format("01/02/2006 15:04"), true
	case "now.date":
		return (time.Time)(ns).Format("01/02/2006"), true
	case "now.time":
		return (time.Time)(ns).Format("15:04"), true
	}
	return "", false
}

type sentIDMap map[string]map[string]string

func (sm sentIDMap) Lookup(varname string) (value string, ok bool) {
	parts := strings.Split(varname, ".")
	if len(parts) != 3 || parts[0] != "sentid" {
		return "", false
	}
	var m1 map[string]string
	if m1, ok = sm[parts[1]]; !ok {
		return "", false
	}
	value, ok = m1[parts[2]]
	return
}
