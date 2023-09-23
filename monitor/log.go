package monitor

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	styleInfoLine  = tcell.StyleDefault.Foreground(tcell.NewHexColor(0x00FFFF))
	styleErrorLine = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xFF0000))
)

func (mon *Monitor) newLog() (l *log) {
	var (
		fh *os.File
	)
	fh, _ = os.OpenFile("exercise.log", os.O_RDONLY, 0666)
	l = &log{Box: tview.NewBox(), fh: fh, atBottom: true}
	l.update()
	return l
}

type log struct {
	*tview.Box
	fh       *os.File
	offset   int64
	lines    []string
	maxlen   int
	scrollX  int
	scrollY  int
	atBottom bool
}

func (l *log) update() {
	var (
		offset int64
		scan   *bufio.Scanner
		err    error
	)
	if offset, err = l.fh.Seek(0, io.SeekEnd); err != nil || offset == l.offset {
		return
	}
	if _, err = l.fh.Seek(l.offset, io.SeekStart); err != nil {
		return
	}
	scan = bufio.NewScanner(l.fh)
	for scan.Scan() {
		line := scan.Text()
		l.lines = append(l.lines, line)
		l.maxlen = max(l.maxlen, len(line))
	}
	if offset, err = l.fh.Seek(0, io.SeekCurrent); err == nil {
		l.offset = offset
	}
	if l.atBottom {
		var _, _, _, h = l.GetInnerRect()
		l.scrollY = max(len(l.lines)-h+1, 0)
	}
}

func (l *log) Draw(screen tcell.Screen) {
	l.DrawForSubclass(screen, l)
	x, y, w, h := l.GetInnerRect()
	y, h = y+1, h-1 // gap
	r, b := x+w, y+h
	l.scrollY = max(min(l.scrollY, len(l.lines)-h), 0)
	l.scrollX = max(min(l.scrollX, l.maxlen-w), 0)
	lnum := l.scrollY
	for ; y < b && lnum < len(l.lines); y, lnum = y+1, lnum+1 {
		line := l.lines[lnum]
		var style = styleInfoLine
		if strings.Contains(line, "ERROR:") {
			style = styleErrorLine
		}
		cnum := l.scrollX
		x := x
		for ; x < r && cnum < len(line); x, cnum = x+1, cnum+1 {
			screen.SetContent(x, y, rune(line[cnum]), nil, style)
		}
	}
	l.atBottom = l.scrollY == max(len(l.lines)-h, 0)
}

func (l *log) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return l.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			l.scrollY--
		case tcell.KeyDown:
			l.scrollY++
		case tcell.KeyLeft:
			l.scrollX--
		case tcell.KeyRight:
			l.scrollX++
		}
	})
}

func (l *log) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
	return l.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		mx, my := event.Position()
		x, y, w, h := l.GetInnerRect()
		if (mx >= x && mx < x+w) && (my >= y && my < y+h) {
			switch action {
			case tview.MouseScrollUp:
				l.scrollY--
				return true, nil
			case tview.MouseScrollDown:
				l.scrollY++
				return true, nil
			case tview.MouseScrollLeft:
				l.scrollX--
				return true, nil
			case tview.MouseScrollRight:
				l.scrollX++
				return true, nil
			}
		}
		return false, nil
	})
}
