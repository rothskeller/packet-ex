package model

import (
	"strings"

	"github.com/rothskeller/packet-ex/variables"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/plaintext"
)

func CreateMessage(mdef *Message, vars variables.Source) (msg message.Message) {
	var mtype = mdef.Type
	if mtype == "" {
		mtype = plaintext.Type.Tag
	}
	if msg = message.Create(mtype); msg == nil {
		return nil
	}
	for _, f := range msg.Base().Fields {
		if f.Value != nil {
			*f.Value = "" // remove defaults
		}
		if value, ok := mdef.Fields[f.Label]; ok {
			value, _ = variables.Interpolate(vars, value, nil)
			value = strings.ReplaceAll(value, "¡", "")
			f.EditApply(f, value)
		}
	}
	return msg
}

func VariablesForMessage(lmi string, env *envelope.Envelope, msg message.Message) variables.Source {
	var vars = make(map[string]string)
	var msgid, _, hcode, tag, _ = message.DecodeSubject(env.SubjectLine)
	vars["msgid"] = msgid
	vars["h"] = hcode
	vars["form"] = tag
	vars["msgid_h"] = msgid + "_" + hcode
	if tag != "" {
		vars["msgid_h_form"] = vars["msgid_h"] + "_" + tag
	} else {
		vars["msgid_h_form"] = vars["msgid_h"]
	}
	vars["type"] = msg.Base().Type.Tag
	vars["from"] = env.From
	vars["to"] = env.To
	vars["subject"] = env.SubjectLine
	for _, f := range msg.Base().Fields {
		if f.Label != "" && f.Value != nil {
			vname := strings.Map(KeepVarNameChars, f.Label)
			vars[vname] = *f.Value
		}
	}
	if lmi != "" {
		vars["lmi"] = lmi
	}
	return variables.MapSource(vars)
}
