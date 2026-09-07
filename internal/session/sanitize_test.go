package session

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// This file tests the boundary the rest of homa trusts. Everything arriving
// from a peer passes through these three functions, and nothing downstream
// checks again, so a hole here is a hole everywhere.
//
// The cases are written as what an attacker would send, not as what a
// well-behaved peer would.

func TestSanitizeTextStripsWhatATerminalWouldObey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "an escape sequence that would clear the screen",
			in:   "before\x1b[2Jafter",
			want: "before[2Jafter",
		},
		{
			name: "a carriage return that would repaint the line above",
			in:   "real message\rforged message",
			want: "real messageforged message",
		},
		{
			name: "a bell",
			in:   "wake\x07up",
			want: "wakeup",
		},
		{
			name: "a newline survives, because a message can have one",
			in:   "line one\nline two",
			want: "line one\nline two",
		},
		{
			name: "a tab survives for the same reason",
			in:   "a\tb",
			want: "a\tb",
		},
		{
			name: "ordinary text is untouched",
			in:   "salam donya",
			want: "salam donya",
		},
		{
			name: "non-latin text is untouched",
			in:   "سلام دنیا",
			want: "سلام دنیا",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeText(tc.in); got != tc.want {
				t.Errorf("sanitizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The escape byte is dropped and the letters after it are not, which is
// deliberate: they are ordinary text, and guessing at where a sequence ends
// would mean deleting things somebody meant to send. What matters is that no
// escape byte survives, because that is the byte a terminal acts on.
func TestSanitizeTextLeavesNoControlBytes(t *testing.T) {
	t.Parallel()

	in := "a\x00b\x1b[31mc\x07d\x1bXe\x9bf"

	for _, r := range sanitizeText(in) {
		switch r {
		case '\n', '\t':
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			t.Errorf("sanitizeText(%q) kept control rune %#x", in, r)
		}
	}
}

func TestSanitizeTextReplacesInvalidUTF8(t *testing.T) {
	t.Parallel()

	got := sanitizeText("good\xff\xfebytes")

	if !utf8.ValidString(got) {
		t.Errorf("sanitizeText produced invalid UTF-8: %q", got)
	}
	if !strings.Contains(got, "good") || !strings.Contains(got, "bytes") {
		t.Errorf("sanitizeText(%q) = %q, want the valid parts kept", "good\xff\xfebytes", got)
	}
}

// Truncation counts runes rather than bytes. Cutting mid-character would turn
// valid input into invalid UTF-8, which is the one thing sanitizing must never
// produce.
func TestTruncationKeepsCharactersWhole(t *testing.T) {
	t.Parallel()

	// Persian letters are two bytes each, so a byte-based cut lands inside
	// one of them.
	long := strings.Repeat("س", MaxTextLen+50)
	got := sanitizeText(long)

	if n := utf8.RuneCountInString(got); n != MaxTextLen {
		t.Errorf("kept %d runes, want %d", n, MaxTextLen)
	}
	if !utf8.ValidString(got) {
		t.Error("truncation produced invalid UTF-8")
	}
}

func TestSanitizeNickIsStricterThanText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a newline in a name would break the line it sits in",
			in:   "bob\nnot bob",
			want: "bobnot bob",
		},
		{
			name: "a tab too",
			in:   "bob\tbob",
			want: "bobbob",
		},
		{
			name: "surrounding space is trimmed",
			in:   "   bob   ",
			want: "bob",
		},
		{
			name: "an empty name is never shown as nothing",
			in:   "",
			want: "unknown",
		},
		{
			name: "nor is a name that was only control characters",
			in:   "\x00\x01\x02",
			want: "unknown",
		},
		{
			name: "nor one that was only spaces",
			in:   "     ",
			want: "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeNick(tc.in); got != tc.want {
				t.Errorf("sanitizeNick(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeNickIsBounded(t *testing.T) {
	t.Parallel()

	got := sanitizeNick(strings.Repeat("ب", MaxNickLen+20))

	if n := utf8.RuneCountInString(got); n != MaxNickLen {
		t.Errorf("kept %d runes, want %d", n, MaxNickLen)
	}
}

// This is the sharpest edge in the program: a name from a peer becomes a path
// on this machine. Every case here is a way out of the download directory or
// a way to leave a file somebody did not mean to have.
func TestSafeFileNameCannotEscapeTheDownloadDirectory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{"the classic", "../../.ssh/authorized_keys"},
		{"an absolute path", "/etc/passwd"},
		{"a parent on its own", ".."},
		{"backslashes, for a peer that thinks it is on windows", `..\..\windows\system32\drivers\etc\hosts`},
		{"a trailing separator", "evil/"},
		{"a name that is only separators", "///"},
		{"a null byte, which some filesystems truncate on", "safe\x00/../escape"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := safeFileName(tc.in)

			if got == "" {
				t.Fatal("produced an empty name")
			}
			if strings.ContainsRune(got, filepath.Separator) || strings.ContainsRune(got, '/') {
				t.Errorf("safeFileName(%q) = %q, which still has a separator in it", tc.in, got)
			}
			if strings.HasPrefix(got, ".") {
				t.Errorf("safeFileName(%q) = %q, which is hidden or a parent reference", tc.in, got)
			}

			// The real assertion: joined onto a directory, it stays
			// inside it.
			const dir = "/tmp/downloads"
			full := filepath.Join(dir, got)
			if !strings.HasPrefix(filepath.Clean(full), dir+"/") {
				t.Errorf("safeFileName(%q) = %q, which lands at %q", tc.in, got, full)
			}
		})
	}
}

func TestSafeFileNameKeepsAnOrdinaryName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"poster.png",
		"gozaresh nahayi.pdf",
		"عکس.jpg",
		"archive.tar.gz",
	} {
		if got := safeFileName(name); got != name {
			t.Errorf("safeFileName(%q) = %q, want it unchanged", name, got)
		}
	}
}

func TestSafeFileNameNeverReturnsNothing(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "   ", "...", "\x00\x01", "."} {
		if got := safeFileName(in); got == "" {
			t.Errorf("safeFileName(%q) returned an empty name", in)
		}
	}
}

func TestSafeFileNameIsBounded(t *testing.T) {
	t.Parallel()

	got := safeFileName(strings.Repeat("a", MaxFileNameLen+100))

	if n := utf8.RuneCountInString(got); n != MaxFileNameLen {
		t.Errorf("kept %d runes, want %d", n, MaxFileNameLen)
	}
}
