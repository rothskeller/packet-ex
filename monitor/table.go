package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/model"
	"golang.org/x/exp/maps"
)

var (
	background            = tcell.NewHexColor(0)
	styleDelivered        = tcell.StyleDefault.Foreground(tcell.NewHexColor(0x0066FF)).Background(background)
	styleSentNotDelivered = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xED7D31)).Background(background)
	styleOK               = tcell.StyleDefault.Foreground(tcell.NewHexColor(0x00FF00)).Background(background)
	styleWarning          = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xED7D31)).Background(background)
	styleError            = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xff0000)).Background(background)
	styleLabel            = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xffffff)).Background(background)
	styleEmpty            = tcell.StyleDefault.Foreground(tcell.NewHexColor(0x888888)).Background(background)
	styleDetail           = tcell.StyleDefault.Foreground(tcell.NewHexColor(0xffffff)).Background(tcell.NewHexColor(0x444444))
)

func (mon *Monitor) newTable() (t *table, h int) {
	t = &table{
		Box:      tview.NewBox(),
		mon:      mon,
		model:    mon.model,
		partrows: make(map[string]int),
		msgcols:  make(map[string]int),
	}
	t.numrows = len(mon.model.Participants)
	t.numcols = len(mon.model.Messages)
	t.celltext = make([][]rune, t.numrows)
	t.cellstyle = make([][]tcell.Style, t.numrows)
	t.celldata = make([][][]any, t.numrows)
	// Assign participant rows.
	for i, p := range mon.model.Participants {
		t.celltext[i] = make([]rune, t.numcols)
		t.cellstyle[i] = make([]tcell.Style, t.numcols)
		t.celldata[i] = make([][]any, t.numcols)
		if p != nil {
			t.partrows[p.TacCall] = i
			t.partcalls = append(t.partcalls, p.TacCall)
		} else {
			t.partcalls = append(t.partcalls, "")
		}
	}
	// Assign message columns.
	for i, m := range mon.model.Messages {
		if m != nil {
			t.msgcols[m.Name] = i
			t.msgnames = append(t.msgnames, m.Name)
		} else {
			t.msgnames = append(t.msgnames, "")
		}
	}
	return t, t.numrows + 4
}

type table struct {
	*tview.Box
	mon       *Monitor
	model     *model.Model
	numrows   int
	numcols   int
	partrows  map[string]int
	msgcols   map[string]int
	partcalls []string
	msgnames  []string
	celldata  [][][]any
	celltext  [][]rune
	cellstyle [][]tcell.Style
	cursorX   int
	cursorY   int
}

func (t *table) update(e *engine.Engine) {
	// Clear old data.
	for row := range t.celldata {
		for col := range t.celldata[row] {
			t.celldata[row][col] = t.celldata[row][col][:0]
		}
	}
	// Add sent messages to data.
	for _, sm := range e.Sent {
		if col, ok := t.msgcols[sm.MName]; ok {
			if sm.PName != "" {
				if row, ok := t.partrows[sm.PName]; ok {
					t.celldata[row][col] = append(t.celldata[row][col], sm)
				}
			} else {
				for row := 0; row < t.numrows; row++ {
					if t.partcalls[row] != "" {
						t.celldata[row][col] = append(t.celldata[row][col], sm)
					}
				}
			}
		}
	}
	// Add received messages to data.
	for _, rm := range e.Received {
		if row, ok := t.partrows[rm.PName]; ok {
			if col, ok := t.msgcols[rm.MName]; ok {
				t.celldata[row][col] = append(t.celldata[row][col], rm)
			}
		}
	}
	// Compute text and styles.
	for row := range t.celldata {
		for col := range t.celldata[row] {
			t.fillCell(row, col)
		}
	}
}

func (t *table) fillCell(row, col int) {
	msgs := t.celldata[row][col]
	if len(msgs) == 0 {
		if t.partcalls[row] == "" || t.msgnames[col] == "" {
			t.celltext[row][col] = ' '
		} else {
			t.celltext[row][col] = '○'
		}
		t.cellstyle[row][col] = styleEmpty
		return
	}
	switch last := msgs[len(msgs)-1].(type) {
	case *engine.Sent:
		t.fillCellSent(row, col, last)
	case *engine.Received:
		t.fillCellReceived(row, col, len(msgs), last)
	}
}

func (t *table) fillCellSent(row, col int, sm *engine.Sent) {
	t.celltext[row][col] = '●'
	if sm.Delivered.IsZero() && sm.PName != "" {
		t.cellstyle[row][col] = styleSentNotDelivered
	} else {
		t.cellstyle[row][col] = styleDelivered
	}
}

func (t *table) fillCellReceived(row, col, count int, rm *engine.Received) {
	if count == 1 {
		t.celltext[row][col] = '●'
	} else if count < 10 {
		t.celltext[row][col] = '0' + rune(count)
	} else {
		t.celltext[row][col] = '+'
	}
	switch {
	case rm.Score == 0 || rm.Score == 100:
		t.cellstyle[row][col] = styleOK
	case rm.Score >= 90:
		t.cellstyle[row][col] = styleWarning
	default:
		t.cellstyle[row][col] = styleError
	}
	if rm.MName == "UNKNOWN" {
		t.cellstyle[row][col] = styleError
	}
}

func (t *table) Draw(screen tcell.Screen) {
	t.DrawForSubclass(screen, t)
	_, _, width, height := t.GetInnerRect()
	t.drawParticipantCalls(screen, height)
	t.drawMessageName(screen, width)
	t.drawTableCells(screen, width, height)
	t.drawDetailPane(screen, width, height)
	screen.HideCursor()
}

func (t *table) drawParticipantCalls(screen tcell.Screen, height int) {
	for row := 0; row < t.numrows && row < height-4; row++ {
		call := t.partcalls[row]
		for x := 0; x < len(call) && x < 6; x++ {
			var style = styleLabel
			if t.cursorX == -1 && t.cursorY == row {
				style = invertStyle(style)
			}
			screen.SetContent(x, row+1, rune(call[x]), nil, style)
		}
	}
}

func (t *table) drawMessageName(screen tcell.Screen, width int) {
	if t.cursorX < 0 {
		return
	}
	var msg = t.msgnames[t.cursorX]
	if msg == "" {
		return
	}
	var x int
	if t.cursorX+len(msg)+9 > width {
		screen.SetContent(t.cursorX+8, 0, '┐', nil, styleLabel)
		x = t.cursorX - len(msg) + 8
	} else {
		screen.SetContent(t.cursorX+8, 0, '┌', nil, styleLabel)
		x = t.cursorX + 9
	}
	for _, r := range msg {
		screen.SetContent(x, 0, r, nil, styleLabel)
		x++
	}
}

func (t *table) drawTableCells(screen tcell.Screen, width, height int) {
	for row := 0; row < t.numrows && row < height-4; row++ {
		for col := 0; col < t.numcols && col < width-8; col++ {
			var text = t.celltext[row][col]
			var style = t.cellstyle[row][col]
			if row == t.cursorY && col == t.cursorX {
				style = invertStyle(style)
			}
			screen.SetContent(col+8, row+1, text, nil, style)
		}
	}
}

func (t *table) drawDetailPane(screen tcell.Screen, width, height int) {
	if t.cursorX == -1 {
		t.drawParticipantDetail(screen, width, height)
		return
	}
	var msgs = t.celldata[t.cursorY][t.cursorX]
	if len(msgs) == 0 {
		return
	}
	var last = msgs[len(msgs)-1]
	if sm, ok := last.(*engine.Sent); ok {
		t.drawSentMessageDetail(screen, width, height, len(msgs), sm)
	} else {
		t.drawReceivedMessageDetail(screen, width, height, len(msgs), last.(*engine.Received))
	}
}

func (t *table) drawParticipantDetail(screen tcell.Screen, width, height int) {
	var part = t.model.Participants[t.cursorY]
	if part == nil {
		return
	}
	var list = []string{fmt.Sprintf("taccall: %s", part.TacCall)}
	var vnames = maps.Keys(part.Vars)
	sort.Strings(vnames)
	for _, vname := range vnames {
		var value = part.Vars[vname]
		if value != "" {
			list = append(list, fmt.Sprintf("%s: %s", vname, value))
		}
	}
	var line = strings.Join(list, "  ")
	for x := 0; x < width; x++ {
		var r = ' '
		if x < len(line) {
			r = rune(line[x])
		}
		screen.SetContent(x, height-3, r, nil, styleDetail)
	}
}

func (t *table) drawSentMessageDetail(screen tcell.Screen, width, height, count int, msg *engine.Sent) {
	var l1, l2 = sentMessageSummaryLines(t.model, msg, count)
	for x := 0; x < width; x++ {
		var r = ' '
		if x < len(l1) {
			r = rune(l1[x])
		}
		screen.SetContent(x, height-3, r, nil, styleDetail)
	}
	if l2 == "" {
		return
	}
	for x := 0; x < width; x++ {
		var r = ' '
		if x < len(l2) {
			r = rune(l2[x])
		}
		screen.SetContent(x, height-2, r, nil, styleDetail)
	}
}

func (t *table) drawReceivedMessageDetail(screen tcell.Screen, width, height, count int, msg *engine.Received) {
	var l1, l2 = receivedMessageSummaryLines(t.model, msg, count)
	for x := 0; x < width; x++ {
		var r = ' '
		if x < len(l1) {
			r = rune(l1[x])
		}
		screen.SetContent(x, height-3, r, nil, styleDetail)
	}
	for x := 0; x < width; x++ {
		var r = ' '
		if x < len(l2) {
			r = rune(l2[x])
		}
		screen.SetContent(x, height-2, r, nil, styleDetail)
	}
}

func (t *table) InputHandler() func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			if t.cursorY > 0 {
				t.cursorY--
			}
		case tcell.KeyDown:
			if t.cursorY < t.numrows-1 {
				t.cursorY++
			}
		case tcell.KeyLeft:
			if t.cursorX >= 0 {
				t.cursorX--
			}
		case tcell.KeyRight:
			if t.cursorX < t.numcols-1 {
				t.cursorX++
			}
		case tcell.KeyEnter:
			if msgs := t.celldata[t.cursorY][t.cursorX]; len(msgs) != 0 {
				t.mon.viewMessages(msgs)
			}
		}
	})
}

func (t *table) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
	return t.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		x, y = x-8, y-1
		if x < -8 || x >= t.numcols || y < 0 || y >= t.numrows {
			return false, nil
		}
		if x < 0 {
			x = -1
		}
		switch action {
		case tview.MouseLeftClick:
			t.cursorX, t.cursorY = x, y
			return true, nil
		case tview.MouseLeftDoubleClick:
			t.cursorX, t.cursorY = x, y
			if x >= 0 {
				if msgs := t.celldata[t.cursorY][t.cursorX]; len(msgs) != 0 {
					t.mon.viewMessages(msgs)
				}
			}
			return true, nil
		}
		return false, nil
	})
}

func invertStyle(s tcell.Style) tcell.Style {
	fg, bg, _ := s.Decompose()
	return s.Foreground(bg).Background(fg)
}
