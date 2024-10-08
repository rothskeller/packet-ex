package engine

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"os/exec"

	"github.com/rothskeller/packet-ex/definition"
	"github.com/rothskeller/packet-ex/state"
)

func (e *Engine) doInject(ev *state.Event, lmi string) string {
	var (
		stn *definition.Station
	)
	if stn = e.def.Station(ev.Station()); stn == nil || stn.Inject == "" || e.noinject {
		return "CREATED"
	}
	if _, err := os.Stat(lmi + ".pdf"); err != nil {
		return "CREATED" // no PDF for inject message
	}
	if stn.Inject == "print" {
		return e.printInject(lmi)
	}
	return e.emailInject(stn.Inject, lmi)
}

func (e *Engine) printInject(lmi string) string {
	var (
		cmdpath string
		cmd     *exec.Cmd
		err     error
	)
	if cmdpath, err = exec.LookPath("lpr"); err != nil {
		if cmdpath, err = exec.LookPath("lp"); err != nil {
			return "CREATED"
		}
	}
	cmd = exec.Command(cmdpath, lmi+".pdf")
	go func() {
		if err = cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: printing message: %s\n", err)
		}
	}()
	return "PRINTED"
}

func (e *Engine) emailInject(addr, lmi string) string {
	var (
		auth smtp.Auth
		buf  bytes.Buffer
		pdf  *os.File
		b64  io.WriteCloser
		err  error
	)
	if e.def.Exercise.SMTPAddress == "" {
		return "CREATED"
	}
	if e.def.Exercise.SMTPUser != "" && e.def.Exercise.SMTPPassword != "" {
		if host, _, err := net.SplitHostPort(e.def.Exercise.SMTPAddress); err != nil {
			return "CREATED"
		} else {
			auth = smtp.PlainAuth("", e.def.Exercise.SMTPUser, e.def.Exercise.SMTPPassword, host)
		}
	}
	if _, err := mail.ParseAddress(addr); err != nil {
		return "CREATED"
	}
	if pdf, err = os.Open(lmi + ".pdf"); err != nil {
		return "CREATED"
	}
	defer pdf.Close()
	fmt.Fprintf(&buf, "From: %s\r\n", e.def.Exercise.EmailFrom)
	fmt.Fprintf(&buf, "To: %s\r\n", addr)
	buf.WriteString("Subject: Message to Send\r\nContent-Type: multipart/mixed; boundary=PKTEX-BOUNDARY\r\n\r\n")
	buf.WriteString("\r\n--PKTEX-BOUNDARY\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString("Greetings,\r\nAs part of the current packet exercise, please send the attached message.  Treat it as if your principal handed it to you to send, and then walked away (i.e., is unavailable for questions).\r\nThank you for participating in this exercise.\r\n")
	buf.WriteString("\r\n--PKTEX-BOUNDARY\r\nContent-Type: application/pdf\r\nContent-Transfer-Encoding: base64\r\nContent-Disposition: attachment; filename=message.pdf\r\n\r\n")
	b64 = base64.NewEncoder(base64.StdEncoding, &buf)
	io.Copy(b64, pdf)
	b64.Close()
	buf.WriteString("\r\n--PKTEX-BOUNDARY--\r\n")
	go func() {
		if err = smtp.SendMail(e.def.Exercise.SMTPAddress, auth, e.def.Exercise.EmailFrom, []string{addr}, buf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: sending email: %s\n", err)
		}
	}()
	return "EMAILED"
}
