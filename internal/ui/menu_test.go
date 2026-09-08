package ui

import (
	"strings"
	"testing"
)

func TestShowMenuGroupsBySpaceAndAlignsTheKeys(t *testing.T) {
	t.Parallel()

	u, _, out := newTerminalTest(t)
	err := u.ShowMenu(
		"What now?",
		[]menuItem{
			{key: "1", text: "call %s", name: "alice"},
			{key: "10", text: "call %s", name: "bob"},
		},
		nil, // an empty group draws nothing, not a stray blank line
		[]menuItem{{key: "n", text: "add a contact"}},
		[]menuItem{{key: "q", text: "quit homa", quiet: true}},
	)
	if err != nil {
		t.Fatalf("ShowMenu: %v", err)
	}

	want := "\nWhat now?\n" +
		"  1   call alice\n" +
		"  10  call bob\n" +
		"\n" +
		"  n   add a contact\n" +
		"\n" +
		"  q   quit homa\n" +
		"\n" +
		"> \n"
	if got := out.String(); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestShowMenuRefusesAnEmptyMenu(t *testing.T) {
	t.Parallel()

	u, _, out := newTerminalTest(t)
	if err := u.ShowMenu("What now?", nil, []menuItem{}); err == nil {
		t.Fatal("an empty menu was drawn")
	}
	if out.String() != "" {
		t.Errorf("an empty menu still wrote %q", out.String())
	}
}

// A quiet line is grey all through, key included; an ordinary line has a
// cream key and a green name and nothing else painted.
func TestShowMenuPaintsAQuietLineGreyAndAKeyCream(t *testing.T) {
	t.Parallel()

	u, _, out := newTerminalTest(t)
	u.st.color = true
	_ = u.ShowMenu("m",
		[]menuItem{{key: "1", text: "call %s", name: "alice"}},
		[]menuItem{{key: "q", text: "quit homa", quiet: true}},
	)

	got := out.String()
	you, green, grey := u.st.code(roleYou), u.st.code(roleThem), u.st.code(roleDim)
	if !strings.Contains(got, you+"1"+colorReset+"  call "+green+"alice"+colorReset) {
		t.Errorf("the ordinary line is not cream key + green name: %q", got)
	}
	if !strings.Contains(got, grey+"q  quit homa"+colorReset) {
		t.Errorf("the quiet line is not grey through: %q", got)
	}
}
