package model

import (
	"strconv"
	"strings"

	"github.com/rothskeller/packet-ex/variables"
)

func (i *Identity) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "taccall":
		return i.TacCall, true
	case "tacname":
		return i.TacName, true
	case "fcccall":
		return i.FCCCall, true
	case "fccname":
		return i.FCCName, true
	}
	return "", false
}

func (b *BBS) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "name":
		return b.Name, true
	case "address":
		return b.Address, true
	}
	return "", false
}

func (i *Incident) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "name":
		return i.Name, true
	case "actnum":
		return i.ActNum, true
	case "startdate":
		return i.StartDate, true
	case "starttime":
		return i.StartTime, true
	case "enddate":
		return i.EndDate, true
	case "endtime":
		return i.EndTime, true
	}
	return "", false
}

func (p Participants) Lookup(varname string) (value string, ok bool) {
	if dot := strings.IndexByte(varname, '.'); dot > 0 {
		if pdef, ok := p[varname[:dot]]; ok {
			return pdef.Lookup(varname[dot+1:])
		}
	}
	return "", false
}

func (p *Participant) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "taccall":
		return p.TacCall, true
	case "prefix":
		return p.Prefix, true
	}
	value, ok = p.Vars[varname]
	return
}

func (r Rules) Lookup(varname string) (value string, ok bool) {
	if dot := strings.IndexByte(varname, '.'); dot > 0 {
		if index, err := strconv.Atoi(varname[:dot]); err == nil && index >= 0 && index < len(r) {
			return r[index].Lookup(varname[dot+1:])
		}
	}
	return "", false
}

func (r *Rule) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "when":
		return r.When, true
	case "wait":
		return r.Wait, r.Wait != ""
	case "then":
		return r.Then, r.Then != ""
	case "msg":
		return r.Message, r.Message != ""
	case "to":
		return r.To, r.To != ""
	}
	value, ok = r.Vars[varname]
	return
}

func (m Messages) Lookup(varname string) (value string, ok bool) {
	if dot := strings.IndexByte(varname, '.'); dot > 0 {
		if mdef, ok := m[varname[:dot]]; ok {
			return mdef.Lookup(varname[dot+1:])
		}
	}
	return "", false
}

func (m *Message) Lookup(varname string) (value string, ok bool) {
	switch varname {
	case "name":
		return m.Name, true
	case "type":
		return m.Type, m.Type != ""
	case "match":
		return m.Match, m.Match != ""
	}
	for fname, fvalue := range m.Fields {
		vname := strings.Map(KeepVarNameChars, fname)
		if vname == varname {
			return fvalue, true
		}
	}
	return "", false
}
func KeepVarNameChars(r rune) rune {
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
		return r
	}
	return -1
}

// Lookup looks up a variable in the model.
func (m *Model) Lookup(varname string) (value string, ok bool) {
	if varname == "startmsgid" {
		return m.StartMsgID, true
	}
	if m.varsource == nil {
		m.varsource = variables.Merged{
			variables.Prefix("identity", &m.Identity),
			variables.Prefix("bbs", &m.BBS),
			variables.Prefix("incident", &m.Incident),
			variables.Prefix("participants", m.ParticipantMap),
			variables.Prefix("rules", m.Rules),
			variables.Prefix("messages", m.MessageMap),
		}
	}
	return m.varsource.Lookup(varname)
}
