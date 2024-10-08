package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	var fname = "exercise.def"
	if len(os.Args) > 1 {
		fname = os.Args[1]
	}
	xscmsg.Register()
	def, err := definition.Read(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
	spew.Dump(def)
}
