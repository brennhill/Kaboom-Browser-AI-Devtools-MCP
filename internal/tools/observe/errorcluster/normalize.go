// Purpose: Normalizes error messages for stable cluster fingerprinting by replacing dynamic tokens.
// Why: Two errors that differ only by an embedded id, uuid, url or timestamp are the same bug, and
// must land in the same cluster. Written as a single byte scan rather than the four regex passes it
// replaces: at the 10,000-entry cap the regex version cost 28ms per analyze call, this costs 2.5ms.
// Docs: docs/features/feature/error-clustering/index.md

package errorcluster

import "strings"

// normalizeErrorMessage replaces uuids, urls, ISO-8601 timestamps and numeric ids
// with fixed placeholders so that recurring instances of one error share a key.
//
// Behaviour is defined by referenceNormalize in errorcluster_normalize_test.go — four
// sequential regex passes — and TestNormalizeErrorMessage_MatchesReference pins the two
// to 200,000 randomized inputs. Prefer editing the reference first: it is the readable
// statement of intent, and this function is only an optimization of it.
//
// Messages needing no rewrite are returned as-is with zero allocations, which is the
// common case for browser errors ("Uncaught ReferenceError: analytics is not defined").
func normalizeErrorMessage(s string) string {
	var b strings.Builder
	last := 0
	prevRepl := false
	for i := 0; i < len(s); {
		var end int
		var repl string
		switch {
		case uuidAt(s, i):
			end, repl = i+uuidLen, "{uuid}"
		case s[i] == 'h' && urlAt(s, i) > 0:
			end, repl = urlAt(s, i), "{url}"
		case isASCIIDigit(s[i]) && timestampAt(s, i) > 0:
			end, repl = timestampAt(s, i), "{timestamp}"
		case isASCIIDigit(s[i]) && numericIDAt(s, i, prevRepl) > 0:
			end, repl = numericIDAt(s, i, prevRepl), "{id}"
		default:
			prevRepl = false
			i++
			continue
		}
		prevRepl = true
		if b.Cap() == 0 {
			b.Grow(len(s) + 16)
		}
		b.WriteString(s[last:i])
		b.WriteString(repl)
		last, i = end, end
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// uuidLen is the length of the canonical 8-4-4-4-12 hyphenated form.
const uuidLen = 36

// timestampPattern spells the fixed prefix of an ISO-8601 timestamp: 'd' means any
// digit, every other byte must match literally.
const timestampPattern = "dddd-dd-ddTdd:dd:dd"

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isASCIIDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isWordByte mirrors the \w class used by the reference's \b assertions.
func isWordByte(c byte) bool {
	return isASCIIDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// uuidAt reports whether a canonical hyphenated uuid starts at s[i].
func uuidAt(s string, i int) bool {
	if len(s)-i < uuidLen {
		return false
	}
	for group, width := range [5]int{8, 4, 4, 4, 12} {
		for j := 0; j < width; j++ {
			if !isHexDigit(s[i]) {
				return false
			}
			i++
		}
		if group < 4 {
			if s[i] != '-' {
				return false
			}
			i++
		}
	}
	return true
}

// urlAt returns the end index of an http(s) url starting at i, or -1.
// The run ends at whitespace or a quote, matching the reference's [^\s"']+.
func urlAt(s string, i int) int {
	var scheme int
	switch {
	case strings.HasPrefix(s[i:], "https://"):
		scheme = len("https://")
	case strings.HasPrefix(s[i:], "http://"):
		scheme = len("http://")
	default:
		return -1
	}
	j := i + scheme
	for j < len(s) && !isSpaceByte(s[j]) && s[j] != '"' && s[j] != '\'' {
		j++
	}
	return j
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

// timestampAt returns the end index of an ISO-8601 timestamp starting at i, or -1.
// The reference's trailing [^\s]* swallows any zone or fractional suffix.
func timestampAt(s string, i int) int {
	if len(s)-i < len(timestampPattern) {
		return -1
	}
	for k := 0; k < len(timestampPattern); k++ {
		c := s[i+k]
		if timestampPattern[k] == 'd' {
			if !isASCIIDigit(c) {
				return -1
			}
			continue
		}
		if c != timestampPattern[k] {
			return -1
		}
	}
	j := i + len(timestampPattern)
	for j < len(s) && !isSpaceByte(s[j]) {
		j++
	}
	return j
}

// numericIDAt returns the end index of a \b\d{3,}\b run starting at i, or -1.
//
// prevRepl says the span immediately before i was rewritten. That matters because the
// reference runs its passes sequentially over the whole string, so by the time the
// numeric pass runs the earlier spans already read "{uuid}"/"{url}"/"{timestamp}" —
// and those braces manufacture a word boundary the original text does not have.
func numericIDAt(s string, i int, prevRepl bool) int {
	if i > 0 && isWordByte(s[i-1]) && !prevRepl {
		return -1
	}
	j := i
	for j < len(s) && isASCIIDigit(s[j]) {
		j++
	}
	// A uuid or timestamp can begin partway inside this digit run ("999" + "2026-07-.."),
	// and because those passes run first, leftmost-match semantics give them the tail.
	// This run only owns the prefix up to where they start.
	for k := i + 1; k < j; k++ {
		if uuidAt(s, k) || timestampAt(s, k) > 0 {
			j = k
			break
		}
	}
	if j-i < 3 {
		return -1
	}
	if j < len(s) && isWordByte(s[j]) && !earlierPatternStartsAt(s, j) {
		return -1
	}
	return j
}

// earlierPatternStartsAt reports whether a higher-priority pattern begins at i, meaning
// that position will be rewritten and therefore ends a word for boundary purposes.
func earlierPatternStartsAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	return uuidAt(s, i) || urlAt(s, i) > 0 || timestampAt(s, i) > 0
}
