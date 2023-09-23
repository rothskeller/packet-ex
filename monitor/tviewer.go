package monitor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newTextViewer(lines []string, style tcell.Style, onClose func()) *tviewer {
	var maxlen int
	for _, line := range lines {
		maxlen = max(maxlen, len(line))
	}
	return &tviewer{Box: tview.NewBox(), lines: lines, maxlen: maxlen, style: style, onClose: onClose}
}

type tviewer struct {
	*tview.Box
	lines    []string
	maxlen   int
	onClose  func()
	scrollX  int
	scrollY  int
	style    tcell.Style
	atBottom bool
}

func (tv *tviewer) setLines(lines []string) {
	tv.lines = lines
	if tv.atBottom {
		var _, _, _, h = tv.GetInnerRect()
		tv.scrollY = max(len(tv.lines)-h, 0)
	}
}

func (tv *tviewer) Draw(screen tcell.Screen) {
	tv.DrawForSubclass(screen, tv)
	x, y, w, h := tv.GetInnerRect()
	r, b := x+w, y+h
	if tv.atBottom {
		tv.scrollY = max(len(tv.lines)-h, 0)
	} else {
		tv.scrollY = max(min(tv.scrollY, len(tv.lines)-h), 0)
	}
	tv.scrollX = max(min(tv.scrollX, tv.maxlen-w), 0)
	lnum := tv.scrollY
	for ; y < b && lnum < len(tv.lines); y, lnum = y+1, lnum+1 {
		line := tv.lines[lnum]
		cnum := tv.scrollX
		x := x
		for ; x < r && cnum < len(line); x, cnum = x+1, cnum+1 {
			screen.SetContent(x, y, rune(line[cnum]), nil, tv.style)
		}
	}
	tv.atBottom = tv.scrollY == max(len(tv.lines)-h, 0)
}

func (tv *tviewer) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return tv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			tv.scrollY--
		case tcell.KeyDown:
			tv.scrollY++
		case tcell.KeyLeft:
			tv.scrollX--
		case tcell.KeyRight:
			tv.scrollX++
		case tcell.KeyEsc:
			if tv.onClose != nil {
				tv.onClose()
			}
		}
	})
}

func (tv *tviewer) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
	return tv.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		mx, my := event.Position()
		x, y, w, h := tv.GetInnerRect()
		// A click outside of our area is a dismissal.
		if action == tview.MouseLeftClick && (mx < x || mx >= x+w || my < y || my >= y+h) && tv.onClose != nil {
			tv.onClose()
			return true, nil
		}
		// Scrolling within our area is supported.
		if (mx >= x && mx < x+w) && (my >= y && my < y+h) {
			switch action {
			case tview.MouseScrollUp:
				tv.scrollY--
				return true, nil
			case tview.MouseScrollDown:
				tv.scrollY++
				return true, nil
			case tview.MouseScrollLeft:
				tv.scrollX--
				return true, nil
			case tview.MouseScrollRight:
				tv.scrollX++
				return true, nil
			}
		}
		return false, nil
	})
}
