package engine

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/plaintext"
)

var msgnumRE = regexp.MustCompile(`^(?:[A-Z][A-Z][A-Z]|[A-Z][0-9][A-Z0-9]|[0-9][A-Z][A-Z])-\d\d\d+[AC-HJ-NPR-Y]$`)

func (e *Engine) analyze(station *definition.Station, msgname, raw, lmi string, env *envelope.Envelope, msg message.Message) (problems []string, score int) {
	var (
		rmi      string
		model    message.Message
		outOf    int
		err      error
		maxScore = 100
	)
	// Find the corresponding inject if any.
	if of := msg.Base().FOriginMsgID; of != nil {
		rmi = *of
	}
	if ev := e.st.MatchInject(station.CallSign, msgname, rmi); ev != nil {
		if _, model, err = incident.ReadMessage(fmt.Sprintf("INJ-%03dI", ev.ID())); err != nil {
			e.st.LogError(fmt.Errorf("can't read inject INJ-%03dI for analysis of %s: %w", ev.ID(), lmi, err))
		}
	}
	// Make sure the message is plain text.
	outOf++
	if env.NotPlainText {
		problems = append(problems, "not a plain text message")
	} else {
		score++
	}
	// Make sure the message has only ASCII characters.
	outOf++
	if strings.IndexFunc(raw, nonASCII) >= 0 {
		problems = append(problems, "message has non-ASCII characters")
	} else {
		score++
	}
	// Some checks only apply to form messages (of known form types).
	if msg.Base().FToICSPosition != nil {
		// Make sure the message subject matches the form.
		outOf++
		subject := msg.EncodeSubject()
		if env.SubjectLine != subject && env.SubjectLine != strings.TrimRight(subject, " ") {
			problems = append(problems, "message subject doesn't agree with form contents")
		} else {
			score++
		}
		// Make sure the message is valid according to PackItForms' rules.
		if pifoprobs := msg.Base().PIFOValid(); len(pifoprobs) != 0 {
			outOf += len(pifoprobs)
			problems = append(problems, pifoprobs...)
		}
		// Make sure the PIFO and form versions are up to date.
		if fv := e.def.FormValidation[definition.PackItForms]; fv != nil && fv.MinVer != "" {
			outOf++
			if message.OlderVersion(msg.Base().PIFOVersion, fv.MinVer) {
				problems = append(problems, "PackItForms version out of date")
			} else {
				score++
			}
		}
		if fv := e.def.FormValidation[msg.Base().Type.Tag]; fv != nil && fv.MinVer != "" {
			outOf++
			if message.OlderVersion(msg.Base().Type.Version, fv.MinVer) {
				problems = append(problems, "form version out of date")
			} else {
				score++
			}
		}
		if omi := *msg.Base().FOriginMsgID; omi != "" {
			outOf++
			if !msgnumRE.MatchString(omi) {
				problems = append(problems, "incorrect message number format")
			} else if station.Prefix != "" && omi[:3] != station.Prefix {
				problems = append(problems, "wrong message number prefix")
			} else {
				score++
			}
		}
		// Make sure the form didn't have any spurious fields.
		outOf++
		if len(msg.Base().UnknownFields) != 0 {
			problems = append(problems, "form has extra fields")
		} else {
			score++
		}
	} else { // checks for plain text messages (or forms of unknown type)
		// Check the message subject format.
		outOf += 1
		msgid, severity, handling, formtag, _ := message.DecodeSubject(env.SubjectLine)
		if msgid == "" {
			problems = append(problems, "incorrect subject line format")
		} else {
			omi := *msg.Base().FOriginMsgID
			if !msgnumRE.MatchString(omi) {
				problems = append(problems, "incorrect message number format")
			} else if station.Prefix != "" && omi[:3] != station.Prefix {
				problems = append(problems, "wrong message number prefix")
			} else {
				score++
			}
			outOf++
			if severity != "" {
				problems = append(problems, "severity on subject line")
			} else {
				score++
			}
			outOf++
			switch handling {
			case "R", "P", "I":
				score++
			case "":
				problems = append(problems, "missing handling order code")
			default:
				problems = append(problems, "unknown handling order code")
			}
		}
		// If this is actually a plain text message (and not an unknown)
		// form type), there are a couple more things to check.
		if m, ok := msg.(*plaintext.PlainText); ok {
			outOf++
			if strings.Contains(m.Body, "!SCCoPIFO!") || strings.Contains(m.Body, "!PACF!") || strings.Contains(m.Body, "!/ADDON!") {
				problems = append(problems, "incorrectly encoded form")
			} else if formtag != "" {
				problems = append(problems, "form name in subject of non-form message")
			} else {
				score++
			}
		}
	}
	// If we have no inject, create a model from the template.
	if model == nil {
		model = e.generateReceivedModel(station.CallSign, msgname)
		// Note that it may still be nil, if there was no template, the
		// station no longer exists, etc.
	}
	// If the inject/model is not the same message type as the received
	// message, flag that, and don't use a model for comparison.
	if model != nil && model.Base().Type.Tag != msg.Base().Type.Tag {
		problems = append(problems, "incorrect message type")
		maxScore /= 2
		model = nil
	}
	// If we have a model, compare the received message to it, field by
	// field.
	if model != nil {
		for _, f := range model.Base().Fields {
			var (
				actv string
				expv string
			)
			if f.Value == nil {
				continue
			}
			expv = *f.Value
			if presence, _ := f.Presence(); presence == message.PresenceRequired && expv == "" {
				// If the model has no value for a required
				// field (e.g., date), any value is accepted.
				continue
			}
			if idx := slices.IndexFunc(msg.Base().Fields, func(mf *message.Field) bool {
				return mf.Label == f.Label
			}); idx >= 0 && msg.Base().Fields[idx].Value != nil {
				actv = *msg.Base().Fields[idx].Value
			}
			if comp := f.Compare(f.Label, expv, actv); comp != nil {
				outOf += comp.OutOf
				score += comp.Score
				if comp.OutOf != comp.Score {
					problems = append(problems, fmt.Sprintf("transcription error in %s: %s", f.Label, renderCompare(comp)))
				}
			}
		}
	} else if fv := e.def.FormValidation[msg.Base().Type.Tag]; fv != nil {
		var exp = fv.Handling
		if exp == "computed" && msg.Base().Type.Tag == "ICS213" {
			exp = ""
			for _, f := range msg.Base().Fields {
				if f.Label == "Severity" {
					switch *f.Value {
					case "EMERGENCY":
						exp = "IMMEDIATE"
					case "URGENT":
						exp = "PRIORITY"
					case "OTHER":
						exp = "ROUTINE"
					}
					break
				}
			}
		}
		if exp == "computed" && msg.Base().Type.Tag == "EOC213RR" {
			exp = ""
			for _, f := range msg.Base().Fields {
				if f.Label == "Priority" {
					switch *f.Value {
					case "Now", "High":
						exp = "IMMEDIATE"
					case "Medium":
						exp = "PRIORITY"
					case "Low":
						exp = "ROUTINE"
					}
					break
				}
			}
		}
		if hf := msg.Base().FHandling; hf != nil && *hf != "" && exp != "" {
			outOf++
			if *hf != exp {
				problems = append(problems, `"Handling" value is not recommended`)
			} else {
				score++
			}
		}
		if pf := msg.Base().FToICSPosition; pf != nil && *pf != "" && len(fv.ToPosition) != 0 {
			outOf++
			if slices.Contains(fv.ToPosition, *pf) {
				score++
			} else {
				problems = append(problems, `"To ICS Position" value is not recommended`)
			}
		}
		if lf := msg.Base().FToLocation; lf != nil && *lf != "" && len(fv.ToLocation) != 0 {
			outOf++
			if slices.Contains(fv.ToLocation, *lf) {
				score++
			} else {
				problems = append(problems, `"To Location" value is not recommended`)
			}
		}
	}
	return problems, score * maxScore / outOf
}

func nonASCII(r rune) bool {
	return r > 126 || (r < 32 && r != '\t' && r != '\n')
}

func renderCompare(comp *message.CompareField) string {
	act := renderCompareMask(comp.Actual, comp.ActualMask)
	exp := renderCompareMask(comp.Expected, comp.ExpectedMask)
	return fmt.Sprintf("%s s.b. %s", act, exp)
}
func renderCompareMask(s, mask string) string {
	if s == "" {
		return `""`
	}
	if len(mask) < len(s) {
		mask += strings.Repeat(string(mask[len(mask)-1]), len(s)-len(mask))
	}
	// Build a list of non-matching regions of the string.
	var regions []int
	var in bool
	for i, m := range mask {
		if m != ' ' && !in {
			regions = append(regions, i)
			in = true
		}
		if m == ' ' && in {
			regions = append(regions, i)
			in = false
		}
	}
	if in {
		regions = append(regions, len(mask))
	}
	if len(regions) == 0 {
		return `""`
	}
	// Render regions.
	var rendered = make([]string, len(regions)/2)
	for i := 0; i < len(regions); i += 2 {
		rendered[i/2] = strconv.Quote(s[regions[i]:regions[i+1]])
	}
	return strings.Join(rendered, ",")
}
