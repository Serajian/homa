package ui

import (
	"errors"
	"os"
	"testing"
)

// A pipe is not a terminal, and homa says so rather than drawing into it:
// a full-screen program has nowhere to put a cursor in a pipe, and nobody
// chats through one.
func TestAPipeIsRefused(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if err := needsTerminal(r, w); !errors.Is(err, ErrNeedsTerminal) {
		t.Errorf("a pipe was accepted: err = %v", err)
	}
}

// Block characters need a UTF-8 locale. LC_ALL overrides LC_CTYPE, which
// overrides LANG, so the first of those that is set is the one that counts.
func TestUnicodeLocale(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no locale at all", map[string]string{}, false},
		{"LANG UTF-8", map[string]string{"LANG": "en_US.UTF-8"}, true},
		{"LANG utf8, lower and without the dash", map[string]string{"LANG": "fa_IR.utf8"}, true},
		{"LANG=C", map[string]string{"LANG": "C"}, false},
		{
			"LC_ALL=C overrides a UTF-8 LANG",
			map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"},
			false,
		},
		{
			"LC_CTYPE UTF-8 overrides LANG=C",
			map[string]string{"LC_CTYPE": "en_US.UTF-8", "LANG": "C"},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := unicodeLocale(func(k string) string { return c.env[k] })
			if got != c.want {
				t.Errorf("unicode = %v, want %v", got, c.want)
			}
		})
	}
}
