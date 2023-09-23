package engine

import (
	"encoding/json"
	"errors"
	"os"
)

func (e *Engine) readState() {
	var (
		fh  *os.File
		err error
	)
	if fh, err = os.Open("exercise.state"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			e.log("ERROR: can't restore state: %s", err)
		}
		return
	}
	defer fh.Close()
	if err = json.NewDecoder(fh).Decode(e); err != nil {
		e.log("ERROR: can't restore state: %s", err)
	}
	for _, sm := range e.Sent {
		if sm.PName != "" && sm.MName != "" {
			if e.sentIDs[sm.PName] == nil {
				e.sentIDs[sm.PName] = make(map[string]string)
			}
			e.sentIDs[sm.PName][sm.MName] = sm.LMI
		}
	}
}

func (e *Engine) saveState() {
	var (
		fh  *os.File
		err error
	)
	if fh, err = os.Create("exercise.state"); err != nil {
		e.log("ERROR: can't save state: %s", err)
		return
	}
	defer fh.Close()
	if err = json.NewEncoder(fh).Encode(e); err != nil {
		e.log("ERROR: can't save state: %s", err)
		os.Remove("exercise.state")
	}
}
