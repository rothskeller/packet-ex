package monitor

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
)

func (mon *Monitor) viewMessages(messages []any) {
	box := tview.NewBox()
	inner := tview.NewBox()
	inner.SetBorder(true).SetBorderColor(tcell.NewHexColor(0x0000FF))
	mv := &mviewer{Box: box, inner: inner}
	var lines []string
	for i, msg := range messages {
		if i != 0 {
			lines = append(lines, "", "--------------------------------------------------------------------------------", "")
		}
		if sm, ok := msg.(*engine.Sent); ok {
			lines = append(lines, linesForSentMessage(mon.model, sm)...)
		} else {
			lines = append(lines, linesForReceivedMessage(mon.model, msg.(*engine.Received))...)
		}
	}
	mv.tv = newTextViewer(lines, tcell.StyleDefault, func() {
		pages.RemovePage("message")
		_, front := pages.GetFrontPage()
		application.SetFocus(front)
	})
	pages.AddPage("message", mv, true, true)
	application.SetFocus(mv.tv)
	return
}

type mviewer struct {
	*tview.Box
	inner *tview.Box
	tv    *tviewer
}

func (mv *mviewer) Draw(screen tcell.Screen) {
	x, y, w, h := mv.GetInnerRect()
	mv.inner.SetRect(x+4, y+1, w-8, h-2)
	mv.inner.Draw(screen)
	mv.tv.SetRect(x+5, y+2, w-10, h-4)
	mv.tv.Draw(screen)
}

func (mv *mviewer) Focus(delegate func(tview.Primitive)) {
	delegate(mv.tv)
}

func (mv *mviewer) HasFocus() bool {
	return mv.tv.HasFocus()
}

func (mv *mviewer) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return mv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if mv.tv.HasFocus() {
			if handler := mv.tv.InputHandler(); handler != nil {
				handler(event, setFocus)
			}
		}
		return
	})
}

func (mv *mviewer) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return mv.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		consumed, capture = mv.tv.MouseHandler()(action, event, setFocus)
		return
	})
}

func linesForSentMessage(m *model.Model, sm *engine.Sent) (lines []string) {
	var l1, l2 = sentMessageSummaryLines(m, sm, 1)
	lines = append(lines, l1)
	if l2 != "" {
		lines = append(lines, l2)
	}
	var env, msg, err = incident.ReadMessage(sm.LMI)
	if err != nil {
		return
	}
	lines = append(lines, "")
	lines = append(lines, linesForBaseMessage(sm.LMI, env, msg)...)
	if env, msg, err = incident.ReadReceipt(sm.LMI, "DR"); err != nil {
		return
	}
	lines = append(lines, "", "--------------------------------------------------------------------------------", "")
	lines = append(lines, linesForBaseMessage("", env, msg)...)
	return
}

func linesForReceivedMessage(m *model.Model, rm *engine.Received) (lines []string) {
	var l1, l2 = receivedMessageSummaryLines(m, rm, 1)
	lines = append(lines, l1, l2)
	lines = append(lines, rm.Problems...)
	lines = append(lines, "")
	var env, msg, err = incident.ReadMessage(rm.LMI)
	if err != nil {
		return
	}
	lines = append(lines, linesForBaseMessage(rm.LMI, env, msg)...)
	if env, msg, err = incident.ReadReceipt(rm.LMI, "DR"); err != nil {
		return
	}
	lines = append(lines, "", "--------------------------------------------------------------------------------", "")
	lines = append(lines, linesForBaseMessage("", env, msg)...)
	return
}

func linesForBaseMessage(lmi string, env *envelope.Envelope, msg message.Message) (lines []string) {
	// Create artificial "fields" for the envelope data we want to
	// show.
	var fields = []*message.Field{makeArtificialField("Message Type", strings.ToUpper(msg.Base().Type.Name[:1])+msg.Base().Type.Name[1:])}
	if env.IsReceived() {
		fields = append(fields, makeArtificialField("From", env.From))
		fields = append(fields, makeArtificialField("Sent", env.Date.Format("01/02/2006 15:04")))
		fields = append(fields, makeArtificialField("To", env.To))
		if lmi != "" {
			fields = append(fields, makeArtificialField("Received", fmt.Sprintf("%s as %s", env.ReceivedDate.Format("01/02/2006 15:04"), lmi)))
		} else {
			fields = append(fields, makeArtificialField("Received", env.ReceivedDate.Format("01/02/2006 15:04")))
		}
	} else {
		if env.To != "" {
			fields = append(fields, makeArtificialField("To", env.To))
		}
		if !env.Date.IsZero() {
			fields = append(fields, makeArtificialField("Sent", env.Date.Format("01/02/2006 15:04")))
		}
	}
	// Add to them the actual message fields.
	fields = append(fields, msg.Base().Fields...)
	var labellen, j int
	for _, f := range fields {
		if f.TableValue(f) != "" {
			labellen = max(labellen, len(f.Label))
			fields[j], j = f, j+1
		}
	}
	fields = fields[:j]
	for _, f := range fields {
		var value = strings.TrimRight(f.TableValue(f), "\n")
		var vlines = strings.Split(value, "\n")
		if len(vlines) > 1 {
			lines = append(lines, f.Label)
			for _, vl := range vlines {
				lines = append(lines, "    "+vl)
			}
		} else {
			lines = append(lines, fmt.Sprintf("%-*.*s  %s", labellen, labellen, f.Label, value))
		}
	}
	return
}

func makeArtificialField(label, value string) (f *message.Field) {
	return message.AddFieldDefaults(&message.Field{Label: label, Value: &value})
}
