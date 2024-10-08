package engine

import (
	"fmt"
	"os"
	"time"
)

// StartTicker computes the time at which the ticker should start, creates it,
// and returns its output channel.  If the ticker won't start for a while, it
// emits a notice to that effect.
func (e *Engine) StartTicker() <-chan time.Time {
	var start time.Time

	// When should the ticker start?
	if start, _ = e.st.LastEntry(); start.IsZero() || start.Before(e.def.Exercise.OpStart) {
		// No previous state, or the OpStart has been changed to be
		// later than now.
		start = e.def.Exercise.OpStart
	} else {
		// Start the integral minute after the last state entry.
		start = start.Add(time.Nanosecond)
	}
	if start.IsZero() {
		// We set it to OpStart, but there is no OpStart.  Set it to the
		// current integral minute so we start right away.
		start = time.Now()
		start = time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute(), 0, 0, start.Location())
	}
	if time.Until(start) > time.Minute {
		// The engine isn't going to start right away.  Make that
		// obvious.
		fmt.Fprintf(os.Stderr, "NOTICE: engine won't start until OpStart: %s\n",
			e.def.Exercise.OpStart.Format("2006-01-02 15:04"))
	}
	return newTicker(start, 0)
}

// newTicker creates a new ticker channel and returns it.  The times sent on
// that channel will be all integral minutes beginning with start.  If start is
// in the future, the first tick will be delayed until that time.  Otherwise,
// the first tick happens immediately.  Subsequent ticks will happen at (or as
// soon as possible after) catchupDelay after the previous tick, but in no case
// before the actual time in the tick.  The channel is unbuffered, so no tick
// will be delivered until calling code is waiting for it.
func newTicker(start time.Time, catchupDelay time.Duration) <-chan time.Time {
	var ch = make(chan time.Time)

	if start.Second() != 0 || start.Nanosecond() != 0 {
		start = time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), start.Minute()+1, 0, 0, start.Location())
	}
	go func() {
		var nextTick = start
		for {
			var delay = time.Until(nextTick)
			if delay > 0 {
				time.Sleep(delay)
			}
			nextTick = time.Now().Add(catchupDelay)
			ch <- start
			start = start.Add(time.Minute)
			if nextTick.Before(start) {
				nextTick = start
			}
		}
	}()
	return ch
}
