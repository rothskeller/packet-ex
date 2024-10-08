package server

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	keepAliveTime = time.Minute
	debounceTime  = 200 * time.Millisecond
)

//go:embed log.html
var logHTML []byte

var highlightReplacer = strings.NewReplacer(
	"ERROR:", "<span class=error>ERROR:</span>",
	"PROBLEM:", "<span class=problem>PROBLEM:</span>",
	"WARNING:", "<span class=warning>WARNING:</span>",
)
var dateTrimRE = regexp.MustCompile(`\b20\d\d-(\d\d-\d\d)T(\d\d:\d\d)(?::\d\d(?:\.\d+)?)?\b`)

type LogServer struct {
	withDate bool
	log      bytes.Buffer
	idle     map[*time.Timer]struct{}
	mutex    sync.Mutex
}

func NewLogServer(withDate bool) (ls *LogServer) {
	ls = &LogServer{withDate: withDate, idle: make(map[*time.Timer]struct{})}
	http.Handle("/log", http.HandlerFunc(ls.ServeHTTP))
	http.Handle("/ws/log", http.HandlerFunc(ls.ServeLogWS))
	return ls
}

func (ls *LogServer) OnLogLine(s string) {
	s = html.EscapeString(s)
	s = highlightReplacer.Replace(s)
	if ls.withDate {
		s = dateTrimRE.ReplaceAllString(s, "$1 $2")
	} else {
		s = dateTrimRE.ReplaceAllString(s, "$2")
	}
	s += "\n"
	ls.mutex.Lock()
	ls.log.WriteString(s)
	for timer := range ls.idle {
		timer.Reset(debounceTime)
		delete(ls.idle, timer)
	}
	ls.mutex.Unlock()
}

func (ls *LogServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "nostore")
	w.Write(logHTML)
}

func (ls *LogServer) ServeLogWS(w http.ResponseWriter, r *http.Request) {
	have, err := strconv.Atoi(r.FormValue("have"))
	if err != nil || have < 0 {
		http.Error(w, "invalid have parameter", http.StatusBadRequest)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"log"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: websocket accept: %s\n", err)
		return
	}
	go ls.followLog(conn, have)
}

func (ls *LogServer) followLog(conn *websocket.Conn, have int) {
	var timer = time.NewTimer(time.Millisecond)
	for range timer.C {
		ls.mutex.Lock()
		buf := ls.log.Bytes()
		have = min(have, len(buf))
		ls.mutex.Unlock()
		err := conn.Write(context.Background(), websocket.MessageText, buf[have:])
		ls.mutex.Lock()
		if err != nil {
			delete(ls.idle, timer)
			ls.mutex.Unlock()
			fmt.Fprintf(os.Stderr, "ERROR: websocket write: %s\n", err)
			return
		}
		have = len(buf)
		if have < ls.log.Len() {
			timer.Reset(debounceTime)
			delete(ls.idle, timer)
		} else {
			timer.Reset(keepAliveTime)
			ls.idle[timer] = struct{}{}
		}
		ls.mutex.Unlock()
	}
}
