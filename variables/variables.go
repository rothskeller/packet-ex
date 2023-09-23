// Package variables handles variable sets and variable interpolation.
package variables

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A Source is a source of variable values.
type Source interface {
	// Lookup looks up the variable with the specified name.  If it is
	// defined, Lookup returns the variable's value and true.  If it is not
	// defined, Lookup returns "", false.
	Lookup(string) (string, bool)
}

// Single is a variable source with a single variable setting.
func Single(varname, value string) Source {
	return singleSource{varname, value}
}

type singleSource struct{ varname, value string }

func (ss singleSource) Lookup(varname string) (value string, ok bool) {
	if varname == ss.varname {
		return ss.value, true
	}
	return "", false
}

// MapSource is a source driven by a map.
type MapSource map[string]string

func (ms MapSource) Lookup(varname string) (value string, ok bool) {
	value, ok = ms[varname]
	return
}

// Prefix adds a prefix to the variables from a Source.
func Prefix(p string, s Source) Source {
	return prefixedSource{p + ".", s}
}

type prefixedSource struct {
	p string
	s Source
}

func (ps prefixedSource) Lookup(varname string) (value string, ok bool) {
	if strings.HasPrefix(varname, ps.p) {
		return ps.s.Lookup(varname[len(ps.p):])
	}
	return "", false
}

type Merged []Source

func (ms Merged) Lookup(varname string) (value string, ok bool) {
	for _, src := range ms {
		if src != nil {
			if value, ok = src.Lookup(varname); ok {
				return
			}
		}
	}
	return
}

// Interpolate interpolates variables from the provided Source into the provided
// string.  If escape is not nil, the interpolated values are passed through the
// escape function before being interpolated.  Interpolate returns the resulting
// string.  The returned Boolean is true if all referenced variables were
// defined.
func Interpolate(src Source, s string, escape func(string) string) (ret string, ok bool) {
	ok = true
	for {
		var (
			dollarsign  int
			closebrace  int
			varname     string
			interpvalue string
			found       bool
		)
		// Fast path for strings with no interpolations.
		if !strings.Contains(s, "${") {
			return ret + s, ok
		}
		dollarsign = strings.IndexByte(s, '$')
		switch s[dollarsign+1] {
		case '$': // $$ is an escaped dollarsign sign.
			ret, s = ret+s[:dollarsign+1], s[dollarsign+2:]
			continue
		case '{': // ${ is a variable interpolation.
			break
		default: // Anything else is taken literally.
			ret, s = ret+s[:dollarsign+2], s[dollarsign+2:]
			continue
		}
		var cbcount = 1
		for i := dollarsign + 2; i < len(s); i++ {
			if s[i] == '{' {
				cbcount++
			}
			if s[i] == '}' {
				cbcount--
			}
			if cbcount == 0 {
				closebrace = i
				break
			}
		}
		if closebrace == 0 { // No close brace.
			return ret + s, ok
		}
		ret += s[:dollarsign]
		varname = s[dollarsign+2 : closebrace]
		s = s[closebrace+1:]
		if interpvalue, found = interpolateOne(src, varname); !found {
			ok = false
		}
		if escape != nil {
			interpvalue = escape(interpvalue)
		}
		ret += interpvalue
	}
}

var substr1RE = regexp.MustCompile(`^(.*):(-?\d+)$`)
var substr2RE = regexp.MustCompile(`^(.*):(-?\d+):(-?\d+)$`)
var addRE = regexp.MustCompile(`^(.*)([-+])([0-9hm]+)$`)
var defaultRE = regexp.MustCompile(`^(.*)\|(.*)$`)

// interpolateOne returns the value of the named variable, while handling any
// modifications to it requested in the interpolation.
func interpolateOne(src Source, varname string) (value string, ok bool) {
	var (
		defval  string
		havedef bool
	)
	if match := substr2RE.FindStringSubmatch(varname); match != nil {
		return substrInterpolate(src, match[1], match[2], match[3])
	}
	if match := substr1RE.FindStringSubmatch(varname); match != nil {
		return substrInterpolate(src, match[1], match[2], "")
	}
	if match := addRE.FindStringSubmatch(varname); match != nil {
		return addInterpolate(src, match[1], match[3], match[2] == "-")
	}
	if match := defaultRE.FindStringSubmatch(varname); match != nil {
		defval, havedef = match[2], true
	}
	varname, _ = Interpolate(src, varname, nil)
	if value, ok = src.Lookup(varname); !ok {
		if havedef {
			value, ok = defval, true
		} else {
			value = "UNDEFINED<" + varname + ">"
		}
	}
	return value, ok
}

func addInterpolate(src Source, varname, constant string, negate bool) (value string, ok bool) {
	if value, ok = interpolateOne(src, varname); !ok {
		return value, ok
	}
	if v, err := strconv.Atoi(value); err == nil || value == "" {
		if c, err := strconv.Atoi(constant); err == nil {
			if negate {
				c = -c
			}
			return strconv.Itoa(v + c), ok
		}
	}
	if v, err := time.ParseInLocation("01/02/2006", value, time.Local); err == nil {
		if c, err := strconv.Atoi(constant); err == nil {
			if negate {
				c = -c
			}
			v = v.AddDate(0, 0, c)
			return v.Format("01/02/2006"), ok
		}
	}
	if v, err := time.ParseInLocation("01/02/2006 15:04", value, time.Local); err == nil {
		if c, err := time.ParseDuration(constant); err == nil {
			if negate {
				c = -c
			}
			v = v.Add(c)
			return v.Format("01/02/2006 15:04"), ok
		}
	}
	if v, err := time.ParseInLocation("15:04", value, time.Local); err == nil {
		if c, err := time.ParseDuration(constant); err == nil {
			if negate {
				c = -c
			}
			v = v.Add(c)
			return v.Format("15:04"), ok
		}
	}
	return value, false
}

func substrInterpolate(src Source, varname, startstr, endstr string) (value string, ok bool) {
	if value, ok = interpolateOne(src, varname); !ok {
		return value, ok
	}
	var start, end int
	var err error
	if start, err = strconv.Atoi(startstr); err != nil {
		return value, false
	}
	if start < 0 {
		start += len(value)
	}
	if start < 0 || start > len(value) {
		return value, false
	}
	if endstr == "" {
		return value[start:], true
	}
	if end, err = strconv.Atoi(endstr); err != nil {
		return value, false
	}
	if end < 0 {
		end += len(value)
	}
	if end < start || end > len(value) {
		return value, false
	}
	return value[start:end], true
}
