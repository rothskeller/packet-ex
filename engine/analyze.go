package engine

import (
	"regexp"
	"slices"
	"strings"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/plaintext"
	"github.com/rothskeller/wppsvr/config"
)

// An analysis contains the analysis of a received message.
type analysis struct {
	env      *envelope.Envelope
	subject  string
	body     string
	msg      message.Message
	mb       *message.BaseMessage
	score    int
	outOf    int
	problems []string
}

// analyze analyzes a single received message, and returns its analysis.
func (e *Engine) analyze(env *envelope.Envelope, msg message.Message, expected, prefix string) (int, []string) {
	var a analysis

	// Store the basic message information in the analysis.
	a.env, a.msg, a.mb = env, msg, msg.Base()
	a.subject = a.env.SubjectLine
	a.score, a.outOf = 1, 1
	// Make sure the message is plain text.
	a.outOf++
	if a.env.NotPlainText {
		a.setSummary("not a plain text message")
	} else {
		a.score++
	}
	// Make sure the message has only ASCII characters.
	a.outOf++
	if strings.IndexFunc(a.body, nonASCII) >= 0 {
		a.setSummary("message has non-ASCII characters")
	} else {
		a.score++
	}
	// Some checks only apply to form messages (of known form types).
	if a.mb.FToICSPosition != nil {
		// Make sure the message subject matches the form.
		a.outOf++
		subject := a.msg.EncodeSubject()
		if a.subject != subject && a.subject != strings.TrimRight(subject, " ") {
			a.setSummary("message subject doesn't agree with form contents")
		} else {
			a.score++
		}
		// Make sure the message is valid according to PackItForms' rules.
		if problems := a.mb.PIFOValid(); len(problems) != 0 {
			a.outOf += len(problems)
			a.problems = append(a.problems, problems...)
		}
		// Make sure the PIFO and form versions are up to date.
		a.outOf += 2
		var minPIFO = config.Get().MinPIFOVersion
		var minForm = config.Get().MessageTypes[a.mb.Type.Tag].MinimumVersion
		if message.OlderVersion(a.mb.PIFOVersion, minPIFO) {
			a.setSummary("PackItForms version out of date")
		} else {
			a.score++
		}
		if message.OlderVersion(a.mb.Form.Version, minForm) {
			a.setSummary("form version out of date")
		} else {
			a.score++
		}
		a.checkMessageNumber(prefix)
	} else { // checks for plain text messages (or forms of unknown type)
		// Check the message subject format.
		a.outOf += 3
		msgid, severity, handling, formtag, _ := message.DecodeSubject(a.subject)
		if msgid == "" {
			a.setSummary("incorrect subject line format")
		} else {
			a.checkMessageNumber(prefix)
			a.score++
			if severity != "" {
				a.setSummary("severity on subject line")
			} else {
				a.score++
			}
			switch handling {
			case "R", "P", "I":
				a.score++
			case "":
				a.setSummary("missing handling order code")
			default:
				a.setSummary("unknown handling order code")
			}
		}
		// If this is actually a plain text message (and not an unknown)
		// form type), there are a couple more things. to check.
		if m, ok := a.msg.(*plaintext.PlainText); ok {
			a.outOf++
			if strings.Contains(m.Body, "!SCCoPIFO!") || strings.Contains(m.Body, "!PACF!") || strings.Contains(m.Body, "!/ADDON!") {
				a.setSummary("incorrectly encoded form")
			} else if formtag != "" {
				a.setSummary("form name in subject of non-form message")
			} else {
				a.score++
			}
		}
	}
	// Make sure the message is of a type allowed for the session.
	a.outOf++
	var allowed = []string{expected}
	if m, ok := a.msg.(*plaintext.PlainText); ok &&
		(strings.Contains(m.Body, "!SCCoPIFO!") || strings.Contains(m.Body, "!PACF!") || strings.Contains(m.Body, "!/ADDON!")) {
		// Allow a "plain text" message containing a corrupt form; that
		// problem gets reported elsewhere.
		allowed = append(allowed, plaintext.Type.Tag)
	}
	if !slices.Contains(allowed, a.mb.Type.Tag) {
		a.setSummary("incorrect message type")
	} else {
		a.score++
	}
	return a.score * 100 / a.outOf, a.problems
}
func nonASCII(r rune) bool {
	return r > 126 || (r < 32 && r != '\t' && r != '\n')
}

func (a *analysis) setSummary(s string) {
	a.problems = append(a.problems, s)
}

var msgnumRE = regexp.MustCompile(`^(?:[A-Z][A-Z][A-Z]|[A-Z][0-9][A-Z0-9]|[0-9][A-Z][A-Z])-\d\d\d+[PMR]$`)

func (a *analysis) checkMessageNumber(prefix string) {
	msgid := *a.msg.Base().FOriginMsgID
	if msgid != "" {
		a.outOf++
		if !msgnumRE.MatchString(msgid) {
			a.setSummary("incorrect message number format")
		} else {
			a.score++
		}
		if prefix != "" {
			a.outOf++
			if !strings.HasPrefix(msgid, prefix) {
				a.setSummary("incorrect message number prefix")
			} else {
				a.score++
			}
		}
	}
}
