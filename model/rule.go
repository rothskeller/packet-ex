package model

import (
	"strings"
)

func (r *Rule) UsesReceived() bool {
	return strings.Contains(r.When, "received") // TODO tighten this
}
func (r *Rule) UsesParticipant() bool {
	return strings.Contains(r.When, "participant") // TODO tighten this
}
func (r *Rule) UsesMessage() bool {
	return strings.Contains(r.When, "message") // TODO tighten this
}
