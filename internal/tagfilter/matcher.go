package tagfilter

// Matcher is the compiled form of a tag filter.
// Zero value matches everything (so callers can use it unconditionally).
type Matcher struct {
	groups [][]string // AND of (OR of tokens)
}

// Matches reports whether tags satisfies all AND-groups.
func (m Matcher) Matches(tags []string) bool {
	if len(m.groups) == 0 {
		return true
	}
	have := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		have[t] = struct{}{}
	}
	for _, ors := range m.groups {
		ok := false
		for _, t := range ors {
			if _, found := have[t]; found {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// IsZero reports whether the matcher is the no-op (match-everything) zero value.
func (m Matcher) IsZero() bool { return len(m.groups) == 0 }
