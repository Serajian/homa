package session

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Text arriving from a peer is drawn straight into a terminal, and a
// terminal obeys what it is given. An escape sequence can clear the screen,
// move the cursor, or repaint earlier lines to forge messages that were
// never sent. A carriage return alone is enough to overwrite the line above.
//
// So nothing from the network is printed as it arrived. It is sanitized
// first, here, once, rather than trusted at every print site.

// sanitizeText makes a received message safe to print. Tabs and newlines
// survive because they carry meaning in a message; everything else in the
// control range is dropped, as are invalid UTF-8 bytes.
func sanitizeText(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}

	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		}
		if unicode.IsControl(r) {
			return -1 // drop
		}
		return r
	}, s)

	return truncateRunes(s, MaxTextLen)
}

// sanitizeNick makes a peer's announced name safe to print. It is stricter
// than sanitizeText: a name sits inside a line of chat, so not even a
// newline or a tab belongs in it.
func sanitizeNick(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}

	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)

	s = strings.TrimSpace(s)
	if s == "" {
		// Never show an empty name: the line would read as if it came
		// from nobody.
		return "unknown"
	}
	return truncateRunes(s, MaxNickLen)
}

// truncateRunes cuts s to at most n runes. Counting runes rather than bytes
// keeps multi-byte characters whole; cutting mid-character would produce
// invalid UTF-8 out of valid input.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}

	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}
