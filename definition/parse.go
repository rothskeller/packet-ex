package definition

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/message"
)

var (
	fcccallRE = regexp.MustCompile(`^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})$`)
	msgidRE   = regexp.MustCompile(`^(?:[A-Z][A-Z0-9]{2}|[0-9][A-Z]{2})-[0-9]{3,}[AC-HJ-NPR-Y]$`)
	prefixRE  = regexp.MustCompile(`^(?:[A-Z][A-Z0-9]{2}|[0-9][A-Z]{2})$`)
	taccallRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{3,}$`)
	msgnameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

func (def *Definition) parseExercise(table [][]string, start int) (err error) {
	if def.Exercise != nil {
		return fmt.Errorf("%d: already have an [EXERCISE] section", start-1)
	}
	def.Exercise = &Exercise{Variables: make(map[string]string)}
	for lnum, line := range table {
		if line == nil {
			continue
		}
		if !ascii(line[1]) {
			return fmt.Errorf("%d: value is not ASCII", lnum+start)
		}
		switch line[0] {
		case "listenaddr":
			if _, _, err := net.SplitHostPort(line[1]); err != nil {
				return fmt.Errorf("%d: listenaddr must be in host:port format", lnum+start)
			}
			def.Exercise.ListenAddr = line[1]
		case "incident":
			def.Exercise.Incident = line[1]
		case "activation":
			def.Exercise.Activation = line[1]
		case "opstart":
			if t, err := time.ParseInLocation("1/2/2006 15:04", line[1], time.Local); err != nil {
				return fmt.Errorf("%d: opstart must have format MM/DD/YYYY HH:MM", lnum+start)
			} else {
				def.Exercise.OpStart = t
			}
		case "opend":
			if t, err := time.ParseInLocation("1/2/2006 15:04", line[1], time.Local); err != nil {
				return fmt.Errorf("%d: opend must have format MM/DD/YYYY HH:MM", lnum+start)
			} else {
				def.Exercise.OpEnd = t
			}
		case "mycall":
			if !taccallRE.MatchString(line[1]) {
				return fmt.Errorf("%d: mycall is not a valid FCC or tactical call sign", lnum+start)
			}
			def.Exercise.MyCall = line[1]
		case "myname":
			def.Exercise.MyName = line[1]
		case "myposition":
			def.Exercise.MyPosition = line[1]
		case "mylocation":
			def.Exercise.MyLocation = line[1]
		case "opcall":
			if !fcccallRE.MatchString(line[1]) {
				return fmt.Errorf("%d: opcall is not a valid FCC call sign", lnum+start)
			}
			def.Exercise.OpCall = line[1]
		case "opname":
			def.Exercise.OpName = line[1]
		case "bbsname":
			if !fcccallRE.MatchString(line[1]) {
				return fmt.Errorf("%d: bbsname is not a valid FCC call sign", lnum+start)
			}
			def.Exercise.BBSName = line[1]
		case "bbsaddress":
			if _, _, err := net.SplitHostPort(line[1]); err != nil {
				return fmt.Errorf("%d: bbsaddress is not a valid hostname:portnum or ipaddress:portnum", lnum+start)
			}
			def.Exercise.BBSAddress = line[1]
		case "bbspassword":
			def.Exercise.BBSPassword = line[1]
			continue // do not make available as variable
		case "emailfrom":
			if _, err := mail.ParseAddress(line[1]); err != nil {
				return fmt.Errorf("%d: emailfrom is not a valid email address", lnum+start)
			}
			def.Exercise.EmailFrom = line[1]
		case "smtpaddress":
			if _, _, err := net.SplitHostPort(line[1]); err != nil {
				return fmt.Errorf("%d: smtpaddress is not a valid hostname:portnum or ipaddress:portnum", lnum+start)
			}
			def.Exercise.SMTPAddress = line[1]
		case "smtpuser":
			def.Exercise.SMTPUser = line[1]
		case "smtppassword":
			def.Exercise.SMTPPassword = line[1]
			continue // do not make available as variable
		case "startmsgid":
			if !msgidRE.MatchString(line[1]) {
				return fmt.Errorf("%d: startmsgid is not a valid XXX-###P message ID", lnum+start)
			}
			def.Exercise.StartMsgID = line[1]
		}
		def.Exercise.Variables[line[0]] = line[1]
	}
	if !def.Exercise.OpStart.IsZero() && !def.Exercise.OpEnd.IsZero() && def.Exercise.OpEnd.Before(def.Exercise.OpStart) {
		return fmt.Errorf("%d: opend must be after opstart", start-1)
	}
	if def.Exercise.MyCall == "" || def.Exercise.MyName == "" {
		return fmt.Errorf("%d: mycall and myname are required", start-1)
	}
	if def.Exercise.OpCall == "" || def.Exercise.OpName == "" {
		return fmt.Errorf("%d: opcall and opname are required", start-1)
	}
	if def.Exercise.BBSName == "" || def.Exercise.BBSAddress == "" || def.Exercise.BBSPassword == "" {
		return fmt.Errorf("%d: bbsname, bbsaddress, and bbspassword are required", start-1)
	}
	if (def.Exercise.SMTPAddress != "" || def.Exercise.SMTPUser != "" || def.Exercise.SMTPPassword != "") &&
		(def.Exercise.SMTPAddress == "" || def.Exercise.SMTPUser == "" || def.Exercise.SMTPPassword == "") {
		return fmt.Errorf("%d: specify all or none of smtpaddress, smtpuser, and smtppassword", start-1)
	}
	if def.Exercise.StartMsgID == "" {
		return fmt.Errorf("%d: startmsgid is required", start-1)
	}
	return nil
}

func (def *Definition) parseFormValidation(table [][]string, start int) (err error) {
	if def.FormValidation != nil {
		return fmt.Errorf("%d: already have a [FORM VALIDATION] section", start-1)
	}
	def.FormValidation = make(map[string]*FormValidation)
	if len(table) == 0 || table[0] == nil {
		return fmt.Errorf("%d: table must begin with column headings", start)
	}
	var tagcol, minvercol, handlingcol, positioncol, locationcol = -1, -1, -1, -1, -1
	for i, col := range table[0] {
		switch col {
		case "tag":
			tagcol = i
		case "minver":
			minvercol = i
		case "handling":
			handlingcol = i
		case "toposition":
			positioncol = i
		case "tolocation":
			locationcol = i
		default:
			return fmt.Errorf("%d: unknown column %q", start, col)
		}
	}
	if tagcol == -1 {
		return fmt.Errorf("%d: table must contain column \"tag\"", start)
	}
	for lnum, line := range table[1:] {
		if line == nil {
			continue
		}
		for i, col := range line {
			if !ascii(col) {
				return fmt.Errorf("%d: %s value is not ASCII", lnum+start+1, table[0][i])
			}
		}
		if _, ok := def.FormValidation[line[tagcol]]; ok {
			return fmt.Errorf("%d: multiple lines with tag %q", lnum+start+1, line[tagcol])
		}
		if _, ok := message.RegisteredTypes[line[tagcol]]; !ok && line[tagcol] != PackItForms {
			return fmt.Errorf("%d: unknown form tag %q", lnum+start+1, line[tagcol])
		}
		var fv FormValidation
		if minvercol != -1 {
			fv.MinVer = line[minvercol]
		}
		if handlingcol != -1 {
			switch line[handlingcol] {
			case "", "IMMEDIATE", "PRIORITY", "ROUTINE":
				break
			case "computed":
				if line[tagcol] != "EOC213RR" && line[tagcol] != "ICS213" {
					return fmt.Errorf("%d: handling cannot be computed for %s", lnum+start+1, line[tagcol])
				}
			default:
				return fmt.Errorf("%d: unknown handling order %q", lnum+start+1, line[handlingcol])
			}
			fv.Handling = line[handlingcol]
		}
		if positioncol != -1 {
			fv.ToPosition = commaSplit(line[positioncol])
		}
		if locationcol != -1 {
			fv.ToLocation = commaSplit(line[locationcol])
		}
		def.FormValidation[line[tagcol]] = &fv
	}
	return nil
}

func (def *Definition) parseStations(table [][]string, start int) (err error) {
	if def.Stations != nil {
		return fmt.Errorf("%d: already have a [STATIONS] section", start-1)
	}
	if len(table) == 0 || table[0] == nil {
		return fmt.Errorf("%d: table must begin with column headings", start)
	}
	var callsigncol, prefixcol, fcccallcol, injectcol, positioncol, locationcol, receiptcol = -1, -1, -1, -1, -1, -1, -1
	for i, col := range table[0] {
		switch col {
		case "callsign":
			callsigncol = i
		case "prefix":
			prefixcol = i
		case "fcccall":
			fcccallcol = i
		case "inject":
			injectcol = i
		case "position":
			positioncol = i
		case "location":
			locationcol = i
		case "receipt":
			receiptcol = i
		default:
			if !ascii(col) {
				return fmt.Errorf("%d: column %d name is not ASCII", start+1, i+1)
			}
		}
	}
	if callsigncol == -1 {
		return fmt.Errorf("%d: table must contain column \"callsign\"", start)
	}
	for lnum, line := range table[1:] {
		for i, col := range line {
			if !ascii(col) {
				return fmt.Errorf("%d: %s value is not ASCII", lnum+start+1, table[0][i])
			}
		}
		if slices.ContainsFunc(def.Stations, func(stn *Station) bool {
			return stn != nil && stn.CallSign == line[callsigncol]
		}) {
			return fmt.Errorf("%d: multiple lines with callsign %q", lnum+start+1, line[callsigncol])
		}
		var stn Station
		if !taccallRE.MatchString(line[callsigncol]) {
			return fmt.Errorf("%d: callsign column does not contain a valid tactical or FCC call sign", lnum+start+1)
		}
		stn.CallSign = line[callsigncol]
		if prefixcol != -1 {
			if line[prefixcol] != "" && !prefixRE.MatchString(line[prefixcol]) {
				return fmt.Errorf("%d: prefix column does not contain a valid message ID prefix", lnum+start+1)
			}
			stn.Prefix = line[prefixcol]
		}
		if fcccallcol != -1 {
			if line[fcccallcol] != "" && !fcccallRE.MatchString(line[fcccallcol]) {
				return fmt.Errorf("%d: fcccall column does not contain a valid FCC call sign", lnum+start+1)
			}
			stn.FCCCall = line[fcccallcol]
		}
		if injectcol != -1 {
			if _, err := mail.ParseAddress(line[injectcol]); err != nil && line[injectcol] != "" && line[injectcol] != "print" {
				return fmt.Errorf("%d: inject column does not contain \"print\" or a valid email address", lnum+start+1)
			}
			stn.Inject = line[injectcol]
		}
		if positioncol != -1 {
			stn.Position = line[positioncol]
		}
		if locationcol != -1 {
			stn.Location = line[locationcol]
		}
		if receiptcol != -1 && line[receiptcol] != "" {
			if strings.EqualFold(line[receiptcol], "NONE") {
				stn.NoReceipts = true
			} else if stn.ReceiptDelay, err = time.ParseDuration(line[receiptcol]); err != nil || stn.ReceiptDelay <= 0 {
				return fmt.Errorf("%d: receipt column does not contain a valid duration", lnum+start+1)
			}
		}
		stn.Variables = make(map[string]string)
		for i, col := range table[0] {
			stn.Variables[col] = line[i]
		}
		def.Stations = append(def.Stations, &stn)
	}
	return nil
}

var conditionRE = regexp.MustCompile(`^((?:exercise|station)\.[A-Za-z][-A-Za-z0-9_]*|[A-Za-z][A-Za-z0-9_]*\.(?:msgid|subjectline))\s*(=|!=|<|<=|>|>=|≈)\s*(\S.*)$`)

func (def *Definition) parseEvents(table [][]string, start int) (err error) {
	if def.Events != nil {
		return fmt.Errorf("%d: already have an [EVENTS] section", start-1)
	}
	if len(table) == 0 || table[0] == nil {
		return fmt.Errorf("%d: table must begin with column headings", start)
	}
	var groupcol, typecol, namecol, triggercol, delaycol, reactcol, conditioncol = -1, -1, -1, -1, -1, -1, -1
	for i, col := range table[0] {
		switch col {
		case "group":
			groupcol = i
		case "type":
			typecol = i
		case "name":
			namecol = i
		case "trigger":
			triggercol = i
		case "delay":
			delaycol = i
		case "react":
			reactcol = i
		case "condition":
			conditioncol = i
		default:
			return fmt.Errorf("%d: unknown column %q", start, col)
		}
	}
	if typecol == -1 || namecol == -1 || triggercol == -1 {
		return fmt.Errorf("%d: table must contain \"type\", \"name\", and \"trigger\" columns", start)
	}
	for lnum, line := range table[1:] {
		for i, col := range line {
			if !ascii(col) {
				return fmt.Errorf("%d: %s value is not ASCII", lnum+start+1, table[0][i])
			}
		}
		var event Event
		if groupcol != -1 {
			event.Group = line[groupcol]
		}
		switch line[typecol] {
		case "inject":
			event.Type = EventInject
		case "receive":
			event.Type = EventReceive
		case "bulletin":
			event.Type = EventBulletin
		case "send":
			event.Type = EventSend
		case "deliver":
			event.Type = EventDeliver
		case "alert":
			event.Type = EventAlert
		default:
			return fmt.Errorf("%d: invalid event type %q", lnum+start+1, line[typecol])
		}
		if event.Name = line[namecol]; event.Name == "" {
			return fmt.Errorf("%d: event name is required", lnum+start+1)
		} else if !msgnameRE.MatchString(event.Name) {
			return fmt.Errorf("%d: invalid message name", lnum+start+1)
		}
		if slices.ContainsFunc(def.Events, func(e *Event) bool {
			return e != nil && e.Type == event.Type && e.Name == event.Name
		}) {
			return fmt.Errorf("%d: multiple lines for %s %q", lnum+start+1, eventTypeNames[event.Type], event.Name)
		}
		switch line[triggercol] {
		case "":
			if len(def.Events) == 0 || def.Events[len(def.Events)-1] == nil {
				return fmt.Errorf("%d: trigger is required when there is no previous line", lnum+start+1)
			}
			event.TriggerType, event.TriggerName = def.Events[len(def.Events)-1].Type, def.Events[len(def.Events)-1].Name
		case "start":
			event.TriggerType = EventStart
		case "manual":
			event.TriggerType = EventManual
		default:
			if strings.HasPrefix(line[triggercol], "inject ") {
				event.TriggerType, event.TriggerName = EventInject, line[triggercol][7:]
			} else if strings.HasPrefix(line[triggercol], "receive ") {
				event.TriggerType, event.TriggerName = EventReceive, line[triggercol][8:]
			} else if strings.HasPrefix(line[triggercol], "send ") {
				event.TriggerType, event.TriggerName = EventSend, line[triggercol][5:]
			} else if strings.HasPrefix(line[triggercol], "bulletin ") {
				event.TriggerType, event.TriggerName = EventBulletin, line[triggercol][9:]
			} else if strings.HasPrefix(line[triggercol], "deliver ") {
				event.TriggerType, event.TriggerName = EventDeliver, line[triggercol][8:]
			} else if strings.HasPrefix(line[triggercol], "alert ") {
				event.TriggerType, event.TriggerName = EventAlert, line[triggercol][6:]
			} else {
				return fmt.Errorf("%d: invalid trigger %q", lnum+start+1, line[triggercol])
			}
			if !msgnameRE.MatchString(event.TriggerName) {
				return fmt.Errorf("%d: invalid trigger %q", lnum+start+1, line[triggercol])
			}
		}
		if event.Type == EventBulletin && event.TriggerType != EventStart && event.TriggerType != EventManual {
			return fmt.Errorf("%d: bulletins can only be triggered by start or manual", lnum+start+1)
		}
		if delaycol != -1 {
			if d, err := time.ParseDuration(line[delaycol]); err != nil && line[delaycol] != "" {
				return fmt.Errorf("%d: invalid delay %q", lnum+start+1, line[delaycol])
			} else {
				event.Delay = d
			}
		}
		if conditioncol != -1 && line[conditioncol] != "" {
			if match := conditionRE.FindStringSubmatch(line[conditioncol]); match == nil {
				return fmt.Errorf("%d: syntax error in condition", lnum+start+1)
			} else {
				event.ConditionVar, event.ConditionOp = match[1], match[2]
				if event.ConditionOp == "≈" {
					if re, err := regexp.Compile(match[3]); err != nil {
						return fmt.Errorf("%d: syntax error in condition regular expression", lnum+start+1)
					} else {
						event.ConditionRE = re
					}
				} else {
					event.ConditionVal = match[3]
				}
			}
		}
		def.Events = append(def.Events, &event)
		if reactcol != -1 {
			if line[reactcol] == "" {
				// nothing
			} else if d, err := time.ParseDuration(line[reactcol]); err != nil {
				return fmt.Errorf("%d: invalid react %q", lnum+start+1, line[reactcol])
			} else if event.Type == EventInject {
				var event2 = event
				event2.TriggerType, event2.TriggerName = event.Type, event.Name
				event2.Type, event2.Delay = EventReceive, d
				def.Events = append(def.Events, &event2)
			} else if event.Type == EventBulletin || event.Type == EventSend {
				var event2 = event
				event2.TriggerType, event2.TriggerName = event.Type, event.Name
				event2.Type, event2.Delay = EventDeliver, d
				def.Events = append(def.Events, &event2)
			} else if line[reactcol] != "" {
				return fmt.Errorf("%d: %s events do not support react values", lnum+start+1, eventTypeNames[event.Type])
			}
		}
	}
	for _, e := range def.Events {
		if e != nil && e.TriggerName != "" {
			if !slices.ContainsFunc(def.Events, func(e2 *Event) bool {
				return e2 != nil && e2.Type == e.TriggerType && e2.Name == e.TriggerName
			}) {
				return fmt.Errorf("%d: %s %s is triggered by nonexistent event %s %s", start-1, eventTypeNames[e.Type], e.Name, eventTypeNames[e.TriggerType], e.TriggerName)
			}
		}
	}
	return nil
}

func (def *Definition) parseMatchReceive(table [][]string, start int) (err error) {
	if def.MatchReceive != nil {
		return fmt.Errorf("%d: already have a [MATCH RECEIVE] section", start-1)
	}
	if len(table) == 0 || table[0] == nil {
		return fmt.Errorf("%d: table must begin with column headings", start)
	}
	var namecol, typecol, subjectcol, subjectrecol = -1, -1, -1, -1
	for i, col := range table[0] {
		switch col {
		case "name":
			namecol = i
		case "type":
			typecol = i
		case "subject":
			subjectcol = i
		case "subjectre", "subjectRE":
			subjectrecol = i
		default:
			return fmt.Errorf("%d: unknown column %q", start, col)
		}
	}
	if namecol == -1 {
		return fmt.Errorf("%d: table must contain \"name\" column", start)
	}
	if typecol+subjectcol+subjectrecol == -3 {
		return fmt.Errorf("%d: table must contain at least one of the \"type\", \"subject\", or \"subjectRE\" columns", start)
	}
	for lnum, line := range table[1:] {
		if line == nil {
			continue
		}
		var mr MatchReceive
		if mr.Name = line[namecol]; mr.Name == "" {
			return fmt.Errorf("%d: name column must have a value", lnum+start+1)
		} else if !msgnameRE.MatchString(mr.Name) {
			return fmt.Errorf("%d: invalid message name", lnum+start+1)
		}
		if slices.ContainsFunc(def.MatchReceive, func(i *MatchReceive) bool { return i.Name == mr.Name }) {
			return fmt.Errorf("%d: multiple lines for message %q", lnum+start+1, mr.Name)
		}
		if typecol != -1 {
			if mr.Type = line[typecol]; mr.Type != "" {
				if _, ok := message.RegisteredTypes[line[typecol]]; !ok {
					return fmt.Errorf("%d: %q is not a known message type", lnum+start+1, line[typecol])
				}
			}
		}
		if subjectcol != -1 {
			if mr.Subject = line[subjectcol]; !ascii(mr.Subject) {
				return fmt.Errorf("%d: subject value is not ASCII", lnum+start+1)
			}
		}
		if subjectrecol != -1 {
			if restr := line[subjectrecol]; restr != "" {
				if !ascii(restr) {
					return fmt.Errorf("%d: subjectRE value is not ASCII", lnum+start+1)
				}
				if mr.SubjectRE, err = regexp.Compile("^(?i:" + restr + ")$"); err != nil {
					return fmt.Errorf("%d: subjectRE value is not a valid regular expression", lnum+start+1)
				}
			}
		}
		if mr.Type == "" && mr.Subject == "" && mr.SubjectRE == nil {
			return fmt.Errorf("%d: line for %s must have a type, a subject, and/or a subjectRE", lnum+start+1, mr.Name)
		}
		def.MatchReceive = append(def.MatchReceive, &mr)
	}
	return nil
}

func (def *Definition) parseBulletin(name string, table [][]string, start int) (err error) {
	if !msgnameRE.MatchString(name) {
		return fmt.Errorf("%d: invalid message name", start-1)
	}
	if _, ok := def.Bulletin[name]; ok {
		return fmt.Errorf("%d: already have a [BULLETIN %s] section", start-1, name)
	}
	var b Bulletin
	for lnum, line := range table {
		if line == nil {
			continue
		}
		switch line[0] {
		case "Area":
			if !ascii(line[1]) {
				return fmt.Errorf("%d: \"Area\" value is not ASCII", lnum+start)
			}
			if _, err := envelope.ParseAddressList(line[1]); err != nil {
				return fmt.Errorf("%d: \"Area\" value is not a valid email or packet address", lnum+start)
			}
			b.Area = line[1]
		case "Subject":
			if !ascii(line[1]) {
				return fmt.Errorf("%d: \"Subject\" value is not ASCII", lnum+start+1)
			}
			b.Subject = line[1]
		case "Message":
			if !ascii(line[1]) {
				return fmt.Errorf("%d: \"Message\" value is not ASCII", lnum+start+1)
			}
			b.Message = line[1]
		default:
			return fmt.Errorf("%d: unknown key %q", lnum+start+1, line[0])
		}
	}
	if b.Area == "" || b.Subject == "" || b.Message == "" {
		return fmt.Errorf("%d: \"Area\", \"Subject\", and \"Message\" fields are required", start)
	}
	def.Bulletin[name] = &b
	return nil
}

func (def *Definition) parseSend(name string, table [][]string, start int) (err error) {
	if !msgnameRE.MatchString(name) {
		return fmt.Errorf("%d: invalid message name", start-1)
	}
	if _, ok := def.Send[name]; ok {
		return fmt.Errorf("%d: already have a [SEND %s] section", start-1, name)
	}
	var m = Message{Fields: make(map[string]StringWithInterps)}
	var blank message.Message
	for lnum, line := range table {
		if line == nil {
			continue
		}
		switch line[0] {
		case "type":
			var typ = line[1]
			if typ == "bulletin" {
				typ = "plain"
			}
			if blank = message.Create(typ, ""); blank == nil {
				return fmt.Errorf("%d: %q is not a known message type", lnum+start, line[1])
			}
			m.Type = typ
		case "version":
			if blank = message.Create(m.Type, line[1]); blank == nil {
				return fmt.Errorf("%d: cannot create version %s of %s", lnum+start, line[1], m.Type)
			}
			m.Version = line[1]
		default:
			if !ascii(line[0]) {
				return fmt.Errorf("%d: field name is not ASCII", lnum+start)
			} else if _, ok := m.Fields[line[0]]; ok {
				return fmt.Errorf("%d: multiple entries for %q", lnum+start, line[0])
			} else if swi, err := parseStringWithInterps(line[1], ascii); err != nil {
				return fmt.Errorf("%d: %q value: %s", lnum+start, line[0], err)
			} else {
				m.Fields[line[0]] = swi
			}
		}
	}
	if blank == nil {
		return fmt.Errorf("%d: a value for \"type\" is required", start)
	}
	for fname := range m.Fields {
		if !slices.ContainsFunc(blank.Base().Fields, func(f *message.Field) bool { return f.Label == fname }) {
			return fmt.Errorf("%d: %s messages do not have a %q field", start, m.Type, fname)
		}
	}
	def.Send[name] = &m
	return nil
}

func (def *Definition) parseReceive(name string, table [][]string, start int) (err error) {
	if !msgnameRE.MatchString(name) {
		return fmt.Errorf("%d: invalid message name", start-1)
	}
	if _, ok := def.Receive[name]; ok {
		return fmt.Errorf("%d: already have a [RECEIVE %s] section", start-1, name)
	}
	var m = Message{Fields: make(map[string]StringWithInterps)}
	var blank message.Message
	for lnum, line := range table {
		if line == nil {
			continue
		}
		switch line[0] {
		case "type":
			if blank = message.Create(line[1], ""); blank == nil {
				return fmt.Errorf("%d: %q is not a known message type", lnum+start, line[1])
			}
			m.Type = line[1]
		case "version":
			if blank = message.Create(m.Type, line[1]); blank == nil {
				return fmt.Errorf("%d: cannot create version %s of %s", lnum+start, line[1], m.Type)
			}
			m.Version = line[1]
		default:
			if !ascii(line[0]) {
				return fmt.Errorf("%d: field name is not ASCII", lnum+start)
			} else if _, ok := m.Fields[line[0]]; ok {
				return fmt.Errorf("%d: multiple entries for %q", lnum+start, line[0])
			} else if swi, err := parseStringWithInterps(line[1], asciiOrBang); err != nil {
				return fmt.Errorf("%d: %q value: %s", lnum+start, line[0], err)
			} else {
				m.Fields[line[0]] = swi
			}
		}
	}
	if blank == nil {
		return fmt.Errorf("%d: a value for \"type\" is required", start)
	}
	for fname := range m.Fields {
		if !slices.ContainsFunc(blank.Base().Fields, func(f *message.Field) bool { return f.Label == fname }) {
			return fmt.Errorf("%d: %s messages do not have a %q field", start, m.Type, fname)
		}
	}
	def.Receive[name] = &m
	return nil
}

var commasplitRE = regexp.MustCompile(`\s*,\s*`)

func commaSplit(s string) (list []string) {
	list = commasplitRE.Split(s, -1)
	list = slices.DeleteFunc(list, func(s string) bool { return s == "" })
	if len(list) == 0 {
		return nil
	}
	return list
}

var interpRE = regexp.MustCompile(`^((?:exercise|station)\.[A-Za-z][-a-zA-Z0-9_]*|[A-Za-z][A-Za-z0-9_]*\.(?:msgid|subjectline)|now\.(?:date|time|datetime))(?::(-?[0-9]+)(?::(-?[0-9]+))?)?([-+]\d[0-9dhm]+)?$`)

func parseStringWithInterps(s string, checkASCII func(string) bool) (swi StringWithInterps, err error) {
	for {
		idx := strings.IndexRune(s, '«')
		if idx < 0 {
			if !checkASCII(s) {
				return swi, errors.New("string is not ASCII")
			}
			swi.Literals = append(swi.Literals, s)
			return swi, nil
		}
		if !checkASCII(s[:idx]) {
			return swi, errors.New("string is not ASCII")
		}
		swi.Literals = append(swi.Literals, s[:idx])
		s = s[idx+2:]
		idx = strings.IndexRune(s, '»')
		if idx < 0 {
			return swi, errors.New("unmatched « in string")
		}
		if match := interpRE.FindStringSubmatch(s[:idx]); match == nil {
			return swi, errors.New("syntax error in variable interpolation")
		} else {
			swi.Variables = append(swi.Variables, match[1])
			val, _ := strconv.Atoi(match[2])
			swi.StartOffsets = append(swi.StartOffsets, val)
			val, _ = strconv.Atoi(match[3])
			swi.EndOffsets = append(swi.EndOffsets, val)
			if match[4] != "" {
				if _, err := strconv.Atoi(match[4][1:]); err != nil {
					if _, ok := ParseDuration(match[4]); !ok {
						return swi, errors.New("syntax error in variable interpolation")
					}
				}
			}
			swi.Additions = append(swi.Additions, match[4])
		}
		s = s[idx+2:]
	}
}

func ParseDuration(s string) (dur time.Duration, ok bool) {
	var (
		val      time.Duration
		mult     time.Duration
		lastMult time.Duration
		neg      time.Duration = 1
	)
	if strings.HasPrefix(s, "-") {
		neg = -1
	} else if !strings.HasPrefix(s, "+") {
		return 0, false
	}
	s = s[1:]
	if s == "" || s[0] < '0' || s[0] > '9' {
		return 0, false
	}
	for s != "" {
		for s != "" && s[0] >= '0' && s[0] <= '9' {
			val = val*10 + time.Duration(s[0]-'0')
			s = s[1:]
		}
		if s == "" {
			return 0, false
		}
		switch s[0] {
		case 'd':
			mult = 24 * time.Hour
		case 'h':
			mult = time.Hour
		case 'm':
			mult = time.Minute
		default:
			return 0, false
		}
		s = s[1:]
		if lastMult != 0 && mult >= lastMult {
			return 0, false
		}
		dur += val * mult
		lastMult = mult
	}
	return dur * neg, true
}

func ascii(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return (r < 32 || r > 127) && r != '\t' && r != '\n'
	}) < 0
}
func asciiOrBang(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return (r < 32 || r > 127) && r != '\t' && r != '\n' && r != '¡'
	}) < 0
}
