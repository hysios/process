package process

import "unicode"

func sep(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}

	switch r {
	case ' ', '\t', '\n':
		return true
	default:
		return false
	}
}

func MakeSplitQuota() func(rune) bool {
	var (
		inQuota bool
		escape  bool
		q       rune
	)

	return func(c rune) bool {
		if escape {
			escape = false
			return false
		}

		if c == '\\' {
			escape = true
			return false
		}

		if !inQuota && (c == '"' || c == '\'') {
			inQuota = true
			q = c
			return true
		} else if inQuota && c == q {
			inQuota = false
			return true
		} else if inQuota && sep(c) {
			return false
		} else if sep(c) {
			return true
		}
		return false
	}
}
