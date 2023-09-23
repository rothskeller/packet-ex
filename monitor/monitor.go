package monitor

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/rothskeller/packet-ex/engine"
	"github.com/rothskeller/packet-ex/model"
)

var application *tview.Application
var pages *tview.Pages

// New creates the monitor.
func New(m *model.Model) (mon *Monitor) {
	var tableHeight int

	mon = &Monitor{model: m}
	mon.table, tableHeight = mon.newTable()
	mon.log = mon.newLog()
	var flex = tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(mon.table, tableHeight, 0, true)
	flex.AddItem(mon.log, 0, 1, false)
	pages = tview.NewPages()
	pages.AddPage("main", flex, true, true)
	application = tview.NewApplication()
	application.EnableMouse(true)
	application.SetRoot(pages, true)
	return mon
}

// Run starts the monitor UI.  Unless it hits an error during initialization,
// this function does not return until the program is terminated with Ctrl-C.
func (mon *Monitor) Run() (err error) {
	return application.Run()
}

type Monitor struct {
	model *model.Model
	table *table
	log   *log
}

func (mon *Monitor) Update(e *engine.Engine) {
	application.QueueUpdateDraw(func() {
		mon.table.update(e)
		mon.log.update()
	})
}

func sentMessageSummaryLines(model *model.Model, msg *engine.Sent, count int) (l1, l2 string) {
	l1 = fmt.Sprintf("%s %-6.6s SENT %s", msg.Sent.Format("01/02/2006 15:04"), model.Identity.TacCall, msg.Subject)
	if msg.Delivered.IsZero() {
		if msg.PName == "" {
			return
		}
		l2 = "                 NO DELIVERY RECEIPT"
	} else {
		l2 = fmt.Sprintf("%s %-6.6s RECEIVED as %s", msg.Delivered.Format("01/02/2006 15:04"), msg.PName, msg.RMI)
	}
	if count > 1 {
		l2 += fmt.Sprintf("     [%d times]", count)
	}
	return
}

func receivedMessageSummaryLines(model *model.Model, msg *engine.Received, count int) (l1, l2 string) {
	l1 = fmt.Sprintf("%s %-6.6s SENT %s", msg.Sent.Format("01/02/2006 15:04"), msg.PName, msg.Subject)
	l2 = fmt.Sprintf("%s %-6.6s RECEIVED as %s", msg.Received.Format("01/02/2006 15:04"), model.Identity.TacCall, msg.RMI)
	if msg.Score != 0 {
		l2 += fmt.Sprintf(", score %d%%", msg.Score)
	}
	if count > 1 {
		l2 += fmt.Sprintf("     [%d times]", count)
	}
	return
}
