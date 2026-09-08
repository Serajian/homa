package ui

import (
	"errors"
	"os"

	"golang.org/x/term"
)

// ErrNeedsTerminal is returned when homa is started with something other
// than a terminal on either end. A full-screen program has nowhere to draw
// in a pipe, and nobody chats through one. Version 1 printed plain text
// there; version 2 says so and stops, because keeping a line-mode interface
// alongside would mean every later feature twice. See
// docs/design/2026-09-08-full-screen-interface.md, decision 3.
var ErrNeedsTerminal = errors.New("needs a terminal")

// needsTerminal reports whether in and out are both terminals. Both are
// checked: a terminal on stdout with a file on stdin is a script feeding
// homa, and it would hang waiting for keys that never come.
func needsTerminal(in, out *os.File) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return ErrNeedsTerminal
	}
	return nil
}
