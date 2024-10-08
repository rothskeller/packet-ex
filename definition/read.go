package definition

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

type section struct {
	name      string
	startline int
	endline   int
	table     [][]string
}

func Read(filename string) (def *Definition, err error) {
	var contents []byte
	var lines []string
	var sections []section

	// Read the file.
	if contents, err = os.ReadFile(filename); err != nil {
		return nil, err
	}
	// Split into lines.
	lines = strings.Split(strings.NewReplacer("\r\n", "\n", "\r", "").Replace(string(contents)), "\n")
	// Split into sections.
	if sections, err = splitSections(lines); err != nil {
		return nil, fmt.Errorf("%s:%s", filename, err)
	}
	// Parse the table in each section.
	for i, s := range sections {
		keyvalue := s.name == "EXERCISE" || strings.HasPrefix(s.name, "SEND ") || strings.HasPrefix(s.name, "RECEIVE ")
		if sections[i].table, err = parseTable(lines[s.startline:s.endline], s.startline+1, keyvalue); err != nil {
			return nil, fmt.Errorf("%s:%s", filename, err)
		}
	}
	// Parse the contents of each section.
	def = &Definition{
		Filename: filename,
		Bulletin: make(map[string]*Bulletin),
		Send:     make(map[string]*Message),
		Receive:  make(map[string]*Message),
	}
	for _, s := range sections {
		switch s.name {
		case "EXERCISE":
			err = def.parseExercise(s.table, s.startline+1)
		case "FORM VALIDATION":
			err = def.parseFormValidation(s.table, s.startline+1)
		case "STATIONS":
			err = def.parseStations(s.table, s.startline+1)
		case "EVENTS":
			err = def.parseEvents(s.table, s.startline+1)
		case "MATCH RECEIVE":
			err = def.parseMatchReceive(s.table, s.startline+1)
		default:
			if strings.HasPrefix(s.name, "BULLETIN ") {
				err = def.parseBulletin(s.name[9:], s.table, s.startline+1)
			} else if strings.HasPrefix(s.name, "SEND ") {
				err = def.parseSend(s.name[5:], s.table, s.startline+1)
			} else if strings.HasPrefix(s.name, "RECEIVE ") {
				err = def.parseReceive(s.name[8:], s.table, s.startline+1)
			} else {
				return nil, fmt.Errorf("%s:%d: unknown section [%s]", filename, s.startline, s.name)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%s", filename, err)
		}
	}
	if def.Exercise == nil {
		return nil, fmt.Errorf("%s: [EXERCISE] section is required", filename)
	}
	if def.Stations == nil {
		return nil, fmt.Errorf("%s: [STATIONS] section is required", filename)
	}
	if def.Events == nil {
		return nil, fmt.Errorf("%s: [EVENTS] section is required", filename)
	}
	if def.MatchReceive == nil && len(def.Receive) != 0 {
		return nil, fmt.Errorf("%s: [MATCH RECEIVE] section is required", filename)
	}
	if err = def.verifyCrossReferences(); err != nil {
		return nil, fmt.Errorf("%s: %s", filename, err)
	}
	return def, nil
}

var sectNameRE = regexp.MustCompile(`^\[([^]]+)\]\s*(?:#.*)?$`)

// splitSections breaks the file up into sections delimited by [SECTION] lines.
func splitSections(lines []string) (sections []section, err error) {
	for lnum, line := range lines {
		// If the line starts with "[", it must be a [SECTION] line.
		// Start a new section with the given section name, initially
		// of zero length starting on the line after [SECTION].
		if strings.HasPrefix(line, "[") {
			if match := sectNameRE.FindStringSubmatch(line); match == nil {
				return nil, fmt.Errorf("%d: syntax error on [SECTION] line", lnum+1)
			} else {
				sections = append(sections, section{match[1], lnum + 1, lnum + 1, nil})
				continue
			}
		}
		// If the line starts with a "#" or contains only whitespace, we
		// skip it.
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		// If we encounter a regular text line, advance the end of the
		// current section to contain it.
		if len(sections) == 0 {
			return nil, fmt.Errorf("%d: text before first [SECTION] line", lnum+1)
		}
		sections[len(sections)-1].endline = lnum + 1
	}
	return sections, nil
}

var multispaceRE = regexp.MustCompile(` [ \t]+|\t[ \t]*`)

// parseTable parses the table in a section.
func parseTable(lines []string, start int, keyvalue bool) (table [][]string, err error) {
	var indent int

	// Certain sections always implicitly have two columns, so give them
	// headings.
	if keyvalue {
		table = [][]string{{"key", "value"}}
	}
	for lnum, line := range lines {
		// Handle indented lines (and blank lines).  These should occur
		// only after a non-indented line whose last column is "¶".
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			if indent == 0 {
				return nil, fmt.Errorf("%d: indented text without ¶ mark", lnum+start)
			}
			line = expandInitialTabs(line)
			if nonblank := strings.IndexFunc(line, func(r rune) bool { return r != ' ' }); indent < 0 {
				// indent < 0 means this is the first line after
				// the ¶; we want to detect its indentation and
				// use that for subsequent lines.
				if nonblank < 0 {
					return nil, fmt.Errorf("%d: line after ¶ must contain indented text", lnum+start)
				}
				indent = nonblank
				line = line[nonblank:]
			} else if nonblank >= 0 && nonblank < indent {
				return nil, fmt.Errorf("%d: line is indented less than first line in ¶ section", lnum+start)
			} else if nonblank >= 0 {
				line = line[indent:]
			} else {
				line = ""
			}
			// Add the indented line (after removing the indent) to
			// the last value of the last entry in the table.
			table[len(table)-1][len(table[0])-1] += line + "\n"
			continue
		}
		// We have a non-indented line.  Split it into columns.
		indent = 0
		var columns = multispaceRE.Split(line, -1)
		// If we have an empty column or one starting with "#", chop the
		// list of columns at that point.  It's trailing whitespace or a
		// trailing comment.  Also, columns containing only a bullet are
		// changed to be empty.
		for i, col := range columns {
			if col == "" || col[0] == '#' {
				columns = columns[:i]
				break
			}
			if col == "•" {
				columns[i] = ""
			}
		}
		// If the final column is "¶", set up for indented text on
		// subsequent lines.
		if columns[len(columns)-1] == "¶" {
			indent = -1
			columns[len(columns)-1] = ""
		}
		// Any other column containing "¶" is an error.
		for _, col := range columns {
			if col == "¶" {
				return nil, fmt.Errorf("%d: indented text with ¶ can only be used in the rightmost column", lnum+start)
			}
		}
		// The line must have the correct number of columns.
		if len(table) != 0 && len(columns) != len(table[0]) {
			return nil, fmt.Errorf("%d: line has %d columns; expected %d", lnum+start, len(columns), len(table[0]))
		} else {
			table = append(table, columns)
		}
	}
	// If we added headings for a keyvalue section, remove them.
	if keyvalue {
		table = table[1:]
	}
	return table, nil
}

// expandInitialTabs expands any tabs in the initial whitespace of s, using an
// 8-column tab size.  It does not touch tabs that occur after the first
// non-whitespace character.
func expandInitialTabs(s string) (expanded string) {
	var col int

	for i, r := range s {
		if r == ' ' {
			expanded += " "
			col++
		} else if r == '\t' {
			width := (col+8)&^7 - col
			expanded += "        "[:width]
			col += width
		} else {
			expanded += s[i:]
			return expanded
		}
	}
	return expanded
}

func (def *Definition) verifyCrossReferences() (err error) {
	var names = make(map[string]string)
	for _, e := range def.Events {
		if e.Type == EventReceive {
			if !slices.ContainsFunc(def.MatchReceive, func(mr *MatchReceive) bool { return mr.Name == e.Name }) {
				return fmt.Errorf("no entry in [MATCH RECEIVE] for message %s", e.Name)
			}
		}
		if e.Type == EventInject {
			if _, ok := def.Receive[e.Name]; !ok {
				return fmt.Errorf("no [RECEIVE %s] entry for inject event", e.Name)
			}
		}
		if e.Type == EventSend {
			if _, ok := def.Send[e.Name]; !ok {
				return fmt.Errorf("no [SEND %s] entry for %s event", e.Name, eventTypeNames[e.Type])
			}
		}
		if e.ConditionVar != "" && !def.variableExists(e.ConditionVar) {
			return fmt.Errorf("[EVENT] %s %s: no such variable %q", eventTypeNames[e.Type], e.Name, e.ConditionVar)
		}
	}
	for i, mr := range def.MatchReceive {
		if !slices.ContainsFunc(def.Events, func(e *Event) bool {
			return e.Name == mr.Name
		}) {
			return fmt.Errorf("no receive event for message %s referenced in [MATCH RECEIVE]", mr.Name)
		}
		for j := 0; j < i; j++ {
			if mr.hiddenBy(def.MatchReceive[j]) {
				return fmt.Errorf("[MATCH RECEIVE] for message %s is not reachable after %s", mr.Name, def.MatchReceive[j].Name)
			}
		}
	}
	for name := range def.Bulletin {
		if !slices.ContainsFunc(def.Events, func(e *Event) bool {
			return e.Name == name && (e.Type == EventBulletin)
		}) {
			return fmt.Errorf("no bulletin event for [BULLETIN %s]", name)
		}
		names[name] = "BULLETIN"
	}
	for name, m := range def.Send {
		if names[name] != "" {
			return fmt.Errorf("message %s cannot be both BULLETIN and SEND", name)
		}
		names[name] = "SEND"
		if !slices.ContainsFunc(def.Events, func(e *Event) bool {
			return e.Name == name && e.Type == EventSend
		}) {
			return fmt.Errorf("no send event for [SEND %s]", name)
		}
		for fname, swi := range m.Fields {
			for _, vname := range swi.Variables {
				if !def.variableExists(vname) {
					return fmt.Errorf("[SEND %s] value for %q refers to nonexistent variable %s", name, fname, vname)
				}
			}
		}
	}
	for name, m := range def.Receive {
		var haveInject, haveReceive bool

		if names[name] != "" {
			return fmt.Errorf("message %s cannot be both %s and RECEIVE", name, names[name])
		}
		for _, e := range def.Events {
			if e.Name == name {
				if e.Type == EventInject {
					haveInject = true
				} else if e.Type == EventReceive {
					haveReceive = true
				}
			}
		}
		if !haveInject && !haveReceive {
			return fmt.Errorf("no inject or receive event for [RECEIVE %s]", name)
		}
		for fname, swi := range m.Fields {
			for _, vname := range swi.Variables {
				if !def.variableExists(vname) {
					return fmt.Errorf("[RECEIVE %s] value for %q refers to nonexistent variable %s", name, fname, vname)
				}
			}
		}
	}
	return nil
}

func (def *Definition) variableExists(vname string) bool {
	group, item, _ := strings.Cut(vname, ".")
	switch group {
	case "exercise":
		_, ok := def.Exercise.Variables[item]
		return ok
	case "station":
		_, ok := def.Stations[0].Variables[item]
		return ok
	case "now":
		return item == "date" || item == "time" || item == "datetime"
	default:
		if group == "UNKNOWN" || slices.ContainsFunc(def.Events, func(e *Event) bool { return e.Name == group }) {
			return item == "msgid" || item == "subjectline"
		}
		return false
	}
}
