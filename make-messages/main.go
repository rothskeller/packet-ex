// make-messages creates a set of similar messages based on a YAML file
// containing scenarios and template messages.
package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/variables"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg"
)

type inputData struct {
	Scenarios []map[string]string `yaml:"scenarios"`
	Messages  []map[string]string `yaml:"messages"`
}

var seenIDs = make(map[string]bool)

func main() {
	var (
		modelfname string
		modelfh    *os.File
		m          *model.Model
		err        error
	)
	xscmsg.Register()
	switch len(os.Args) {
	case 1:
		modelfname = "exercise.yaml"
	case 2:
		modelfname = os.Args[1]
	default:
		fmt.Fprintf(os.Stderr, "usage: make-messages [model-file]\n")
		os.Exit(2)
	}
	if modelfh, err = os.Open(modelfname); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	if m, err = model.Read(modelfh); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s\n", modelfname, err)
		os.Exit(1)
	}
	for seq, msg := range findReceivedMessages(m) {
		for _, part := range m.Participants {
			if part != nil {
				makeMessage(m, part, msg, seq)
			}
		}
	}
}

var receivedExprRE = regexp.MustCompile(`\$\{received\.name\}\s*==?\s*"?([^" ]+)`)

func findReceivedMessages(m *model.Model) (list []*model.Message) {
	var mnames = make(map[string]bool)
	for _, rule := range m.Rules {
		if match := receivedExprRE.FindStringSubmatch(rule.When); match != nil {
			mnames[match[1]] = true
		}
	}
	for _, msg := range m.Messages {
		if msg != nil && mnames[msg.Name] && len(msg.Fields) != 0 {
			list = append(list, msg)
		}
	}
	return list
}

var varSubstRE = regexp.MustCompile(`\$\{(\w+)\}`)

func makeMessage(m *model.Model, part *model.Participant, mm *model.Message, seq int) {
	var varsrc = variables.Merged{m, variables.Prefix("participant", part), noUndefinedErrors{}}

	var msg = model.CreateMessage(mm, varsrc)
	if err := msg.RenderPDF(fmt.Sprintf("%s-%02d-%s.pdf", part.TacCall, seq, mm.Name)); err != nil && err != message.ErrNotSupported {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

type noUndefinedErrors struct{}

func (noUndefinedErrors) Lookup(varname string) (value string, ok bool) {
	return "", true
}
