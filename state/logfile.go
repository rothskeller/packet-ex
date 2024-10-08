package state

import (
	"bufio"
	"fmt"
	"os"
)

// Open connects the state tracker to a log file.  It opens the log file,
// creating it if it doesn't already exist.  It executes each log entry already
// in the file.  Then it sets up a state listener that adds new log entries to
// the end of the file.  If fname is empty, the default name (exercise.log) is
// used.
func (s *State) Open(fname string) (err error) {
	var (
		logf    *os.File
		scan    *bufio.Scanner
		linenum int
	)
	if fname == "" {
		fname = "exercise.log"
	}
	if logf, err = os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0644); err != nil {
		return err
	}
	scan = bufio.NewScanner(logf)
	for scan.Scan() {
		linenum++
		if _, err = s.Execute(scan.Text()); err != nil {
			return fmt.Errorf("%s:%d: %s", fname, linenum, err)
		}
	}
	if err = scan.Err(); err != nil {
		return fmt.Errorf("%s:%d: %s", fname, linenum, err)
	}
	if _, err = logf.Seek(0, 2); err != nil {
		return fmt.Errorf("%s: %s", fname, err)
	}
	s.AddListener(logger{logf})
	return nil
}

type logger struct{ logf *os.File }

func (l logger) OnLogLine(line string) {
	fmt.Fprintln(l.logf, line)
}
