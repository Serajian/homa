package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testLine(name string, known bool) *line {
	return &line{name: name, known: known}
}

func typeInto(c *conversation, st *styles, s string) {
	for _, r := range s {
		_, _ = c.update(st, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// The fault that started version 2: a message arriving while typing used to
// land over the half-typed line. Now it lands in the pane and the input is
// untouched.
func TestAMessageArrivingWhileTypingDoesNotTouchTheInput(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 60, 12, testLine("alice", true), "alice", "~/homa-files")
	typeInto(c, st, "hi")
	c.say("[alice] salam")

	if got := c.in.Value(); got != "hi" {
		t.Errorf("input = %q; the arriving message changed it", got)
	}
	_, body, _ := c.view(st, 60)
	plain := stripANSI(body)
	if !strings.Contains(plain, "[alice] salam") {
		t.Errorf("the message is not in the pane:\n%s", plain)
	}
	if !strings.Contains(plain, "hi") {
		t.Errorf("the input line is not drawn with what was typed:\n%s", plain)
	}
}

// The second fault: a line not yet sent when the peer left was dropped.
func TestALineNotYetSentStaysWhenThePeerLeaves(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 60, 12, testLine("alice", true), "alice", "")
	typeInto(c, st, "x")
	c.ended = true
	c.say("alice left the conversation.")

	if c.in.Value() != "x" {
		t.Error("the typed line was dropped")
	}
	_, leave := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !leave {
		t.Error("Enter on a closed line did not go back to the menu")
	}
}

func TestTheHeaderSaysWhoAndWhereFilesGo(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 80, 12, testLine("~bob", false), "bob", "~/homa-files")
	status, body, keys := c.view(st, 80)
	plain := stripANSI(status + body + keys)
	for _, want := range []string{"talking to ~bob", "the name is theirs; they are not in your contacts", "files → ~/homa-files", "/help", "/quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("conversation lacks %q:\n%s", want, plain)
		}
	}
}

func TestUpAndDownWalkTheHistory(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 60, 12, testLine("alice", true), "alice", "")
	c.hist.push("first")
	c.hist.push("second")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyUp})
	if c.in.Value() != "second" {
		t.Errorf("up = %q", c.in.Value())
	}
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyUp})
	if c.in.Value() != "first" {
		t.Errorf("up up = %q", c.in.Value())
	}
}

func TestAnUnknownCommandIsSaidAndTheListShown(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 60, 20, testLine("alice", true), "alice", "")
	typeInto(c, st, "/nope")
	_, leave := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if leave {
		t.Fatal("/nope left the conversation")
	}
	plain := stripANSI(strings.Join(c.lines, "\n"))
	if !strings.Contains(plain, `no such command: "/nope"`) ||
		!strings.Contains(plain, "/quit         leave the conversation") {
		t.Errorf("pane:\n%s", plain)
	}
	if c.in.Value() != "" {
		t.Error("the command stayed in the input")
	}
}
