// Package tagfilter parses AND-of-ORs tag filter expressions.
//
// Syntax: space-separated AND-groups; each AND-group is comma-separated
// OR-list of plain tag tokens. Empty expression matches everything.
package tagfilter

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrInvalidFilter = errors.New("invalid tag filter")

var tagRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// Parse compiles a filter expression into a Matcher.
// Returns the zero Matcher (matches everything) for empty input.
func Parse(expr string) (Matcher, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Matcher{}, nil
	}
	// Remove spaces around commas to normalize "prod  ,  staging" to "prod,staging"
	expr = strings.ReplaceAll(expr, " , ", ",")
	expr = strings.ReplaceAll(expr, ", ", ",")
	expr = strings.ReplaceAll(expr, " ,", ",")

	var groups [][]string
	for _, andTok := range strings.Fields(expr) {
		var ors []string
		for _, orTok := range strings.Split(andTok, ",") {
			tok := strings.TrimSpace(orTok)
			if tok == "" {
				return Matcher{}, fmt.Errorf("%w: empty token in %q", ErrInvalidFilter, andTok)
			}
			if !tagRe.MatchString(tok) {
				return Matcher{}, fmt.Errorf("%w: %q (allowed: [a-zA-Z0-9._-]{1,64})", ErrInvalidFilter, tok)
			}
			ors = append(ors, tok)
		}
		groups = append(groups, ors)
	}
	return Matcher{groups: groups}, nil
}
