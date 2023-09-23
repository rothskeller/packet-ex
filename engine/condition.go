package engine

import (
	"regexp"
	"strings"

	"github.com/rothskeller/packet-ex/model"
	"github.com/rothskeller/packet-ex/variables"
)

var (
	case1 = regexp.MustCompile(`(?i)^\s*([-A-Z0-9:]+)\s*==?\s*([-A-Z0-9:]+)\s*$`)
	case2 = regexp.MustCompile(`(?i)^\s*([-A-Z0-9:]+)\s*==?\s*([-A-Z0-9:]+)\s*\|\|\s*([-A-Z0-9:]+)\s*==?\s*([-A-Z0-9:]+)\s*$`)
	case3 = regexp.MustCompile(`(?i)^\s*([-A-Z0-9:]+)\s*==?\s*([-A-Z0-9:]+)\s*&&\s*([-A-Z0-9:]+)\s*==?\s*([-A-Z0-9:]+)\s*$`)
)

func (e *Engine) testRuleCondition(r *model.Rule, vars variables.Source) bool {
	// Eventually this should be a true expression evaluator.  For now I'm
	// just hard-coding the cases I'm using.
	when, ok := variables.Interpolate(vars, r.When, nil)
	if !ok {
		e.log("ERROR: bad variable interpolation in rule.When: %s", when)
		return false
	}
	if match := case1.FindStringSubmatch(when); match != nil {
		return strings.EqualFold(match[1], match[2])
	}
	if match := case2.FindStringSubmatch(when); match != nil {
		return strings.EqualFold(match[1], match[2]) || strings.EqualFold(match[3], match[4])
	}
	if match := case3.FindStringSubmatch(when); match != nil {
		return strings.EqualFold(match[1], match[2]) && strings.EqualFold(match[3], match[4])
	}
	e.log("ERROR no match to any condition expression case case: %s", when)
	return false
}
