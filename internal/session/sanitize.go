package session

import (
	"path/filepath"
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

// sanitizeAddr makes an address arriving from a peer safe to hold. It is
// the strictest of the three: an address is base64url with a prefix, so
// nothing that is not one of those characters can belong to it, and what is
// left is bounded. Whether the result is really an address is decided above
// this package, by whoever knows what one looks like.
func sanitizeAddr(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		}
		return -1 // drop
	}, s)

	return truncateRunes(s, MaxAddrLen)
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

// safeFileName turns a peer-supplied name into something safe to create on
// this machine.
//
// The name arrives from the network, so it is an attack surface before it
// is a convenience. filepath.Base strips any directory part, which is what
// stops "../../.ssh/authorized_keys" from escaping the download directory.
// Everything else here is about not creating a file whose name is a trap:
// hidden, empty, or full of control characters.
func safeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))

	name = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r):
			return -1
		case r == filepath.Separator || r == '/' || r == '\\':
			return '_'
		}
		return r
	}, name)

	name = strings.TrimSpace(name)
	name = strings.TrimLeft(name, ".")

	if name == "" {
		return "received-file"
	}
	return truncateRunes(name, MaxFileNameLen)
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
