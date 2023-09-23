package engine

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/variables"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/plaintext"
	"github.com/rothskeller/wppsvr/store"
)

func (e *Engine) runTimeBasedRules() {
	var vars = variables.Merged{nowSource(e.PassTime), e.UserVars, e.sentIDs, e.model}
	for _, rule := range e.model.Rules {
		if !rule.UsesReceived() && e.testRuleCondition(rule, vars) {
			e.runRule(rule, vars)
		}
	}
	j := 0
	for _, set := range e.DelayedSets {
		if !set.Time.After(e.PassTime) {
			e.log("Set set.%s to %q", set.Name, set.Value)
			e.UserVars[set.Name] = set.Value
			continue
		}
		e.DelayedSets[j] = set
		j++
	}
	e.DelayedSets = e.DelayedSets[:j]
}

func (e *Engine) runReceivedRules(lmi string, env *envelope.Envelope, msg message.Message) {
	var (
		pdef   *model.Participant
		mvars  variables.Source
		mdef   *model.Message
		varsrc = variables.Merged{nowSource(e.PassTime), e.UserVars, e.sentIDs, e.model}
	)
	e.log("Received %s", env.SubjectLine)
	if pdef = e.participantForMessage(env); pdef == nil {
		e.log("=> ignored, not from a known participant")
		return
	}
	mvars = model.VariablesForMessage(lmi, env, msg)
	mdef = e.messageDefForMessage(mvars, pdef)
	varsrc = append(varsrc, e.receivedMessageVariables(pdef, mdef, mvars))
	// Score the message.
	var score, problems = e.scoreReceivedMessage(env, msg, pdef, mdef, varsrc)
	// Record the received message for the monitor.
	var rcvd = &Received{LMI: lmi}
	rcvd.Received = env.ReceivedDate
	rcvd.Sent = env.Date
	rcvd.Subject = env.SubjectLine
	rcvd.RMI, _ = varsrc.Lookup("received.msgid")
	rcvd.MName, _ = varsrc.Lookup("received.name")
	rcvd.PName = pdef.TacCall
	rcvd.Score = score
	rcvd.Problems = problems
	e.Received = append(e.Received, rcvd)
	// Run the rules for the message.
	for _, rule := range e.model.Rules {
		if rule.UsesReceived() && e.testRuleCondition(rule, varsrc) {
			e.runRule(rule, varsrc)
		}
	}
}

func (e *Engine) participantForMessage(env *envelope.Envelope) *model.Participant {
	var pname string

	if flist, err := envelope.ParseAddressList(env.From); err == nil && len(flist) != 0 {
		pname = flist[0].Address
	}
	pname, _, _ = strings.Cut(pname, "@")
	pname = strings.ToUpper(pname)
	return e.model.ParticipantMap[pname]
}

func (e *Engine) messageDefForMessage(msgvars variables.Source, part *model.Participant) *model.Message {
	var varsrc = variables.Merged{
		variables.Prefix("received", msgvars),
		variables.Prefix("participant", part),
		nowSource(e.PassTime), e.UserVars, e.sentIDs, e.model,
	}
	var subject, _ = msgvars.Lookup("subject")
	// Try to match it against a known message.
	for _, mdef := range e.model.MessageMap {
		// Try a match against the SubjectRE if provided.
		if mdef.Match != "" {
			if matchREstr, ok := variables.Interpolate(varsrc, mdef.Match, regexp.QuoteMeta); ok {
				if matchRE, err := regexp.Compile("(?i)^" + matchREstr + "$"); err == nil {
					if matchRE.MatchString(subject) {
						return mdef
					}
				}
			}
			continue
		}
		// If not, interpolate into the mdef, compute a subject, and
		// compare against that.
		var mm = model.CreateMessage(mdef, varsrc)
		if strings.EqualFold(mm.EncodeSubject(), subject) {
			return mdef
		}
	}
	return nil
}

func (e *Engine) receivedMessageVariables(pdef *model.Participant, mdef *model.Message, mvars variables.Source) variables.Source {
	var mname = "UNKNOWN"
	if mdef != nil {
		mname = mdef.Name
	}
	return variables.Merged{
		variables.Single("received.name", mname),
		variables.Prefix("received", mvars),
		variables.Prefix("participant", pdef),
	}
}

func (e *Engine) scoreReceivedMessage(
	env *envelope.Envelope, msg message.Message, pdef *model.Participant, mdef *model.Message, varsrc variables.Source,
) (score int, problems []string) {
	// First, determine the type of message we're validating.
	var mtype string
	if mdef != nil {
		mtype = mdef.Type
	} else {
		return 0, []string{"not a recognized message"}
	}
	if mtype == "" {
		mtype = plaintext.Type.Tag
	}
	score, problems = e.analyze(env, msg, mtype, pdef.Prefix)
	if len(mdef.Fields) == 0 { // match only, no compare
		return
	}
	if msg.Base().Type.Tag != mtype {
		score /= 2 // wrong type, completely failed comparison
		problems = append(problems, fmt.Sprintf("message is %s; expected %s", msg.Base().Type.Tag, mtype))
		return
	}
	var cscore, coutof int
	for _, f := range msg.Base().Fields {
		if f.Label != "" && f.Value != nil {
			if restr := mdef.Fields[f.Label+"/RE"]; restr != "" {
				if re, err := regexp.Compile("(?i)^" + restr + "$"); err == nil {
					coutof++
					if re.MatchString(*f.Value) {
						cscore++
					} else {
						problems = append(problems, fmt.Sprintf("value of %q field does not match expected pattern", f.Label))
					}
				} else {
					e.log("ERROR: messages[%s].%s/RE is not a valid regular expression", mdef.Name, f.Label)
				}
				continue
			}
			var mval, _ = variables.Interpolate(varsrc, mdef.Fields[f.Label], nil)
			if cf := f.Compare(f.Label, mval, *f.Value); cf != nil {
				cscore, coutof = cscore+cf.Score, coutof+cf.OutOf
				if cf.Score < cf.OutOf {
					problems = append(problems, fmt.Sprintf("value of %q field (%q) does not match expected value (%q)", f.Label, *f.Value, mval))
				}
			}
		}
	}
	score = score/2 + cscore*50/coutof
	return
}

func (e *Engine) runRule(rule *model.Rule, varsrc variables.Source) {
	var rtime = e.PassTime
	if delay, err := time.ParseDuration(rule.Wait); err == nil {
		rtime = rtime.Add(delay)
	}
	switch rule.Then {
	case "nothing":
		break
	case "set":
		for name, value := range rule.Vars {
			e.DelayedSets = append(e.DelayedSets, &DelayedSet{
				Time:  rtime,
				Name:  name,
				Value: value,
			})
		}
	case "send":
		e.DelayedSend = append(e.DelayedSend, &DelayedSend{
			Time:  rtime,
			MName: rule.Message,
			To:    rule.To,
		})
	case "reply":
		toPName, _ := varsrc.Lookup("participant.taccall")
		to, _ := varsrc.Lookup("received.from")
		lmi, _ := varsrc.Lookup("received.lmi")
		e.DelayedSend = append(e.DelayedSend, &DelayedSend{
			Time:  rtime,
			MName: rule.Message,
			PName: toPName,
			To:    to,
			Reply: lmi,
		})
	}
}

type fakeAnalysisStore struct{}

// The fake store never already has the message being analyzed.
func (s fakeAnalysisStore) HasMessageHash(string) string { return "" }

// We never generate responses, so we never use a NextMessageID.
func (s fakeAnalysisStore) NextMessageID(string) string { return "" }

// We never save the analysis results.
func (s fakeAnalysisStore) SaveMessage(*store.Message) {}
