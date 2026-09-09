package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Serajian/homa/internal/peer"
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

func pressKey(c *conversation, st *styles, code rune) {
	_, _ = c.update(st, tea.KeyPressMsg{Code: code})
}

// The hint row: blank for a message, the commands for a slash, one command
// as letters narrow it, and warning at once for a word that is nothing.
func TestTheHintRowOffersCommandsAsTheyAreTyped(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")
	view := func() string {
		_, body, _ := c.view(st, 100)
		return stripANSI(body)
	}

	typeInto(c, st, "hi")
	if strings.Contains(view(), "/help") {
		t.Errorf("a message got a hint:\n%s", view())
	}
	c.in.Reset()

	typeInto(c, st, "/")
	if !strings.Contains(view(), "▸ /help  ·  /files [dir]  ·  /send <path>") {
		t.Errorf("a slash did not offer the commands:\n%s", view())
	}
	typeInto(c, st, "s")
	if !strings.Contains(view(), "/send <path>   offer a file  ·  Tab completes") {
		t.Errorf("/s did not narrow to /send:\n%s", view())
	}
	typeInto(c, st, "x")
	if !strings.Contains(view(), `no such command: "/sx"`) {
		t.Errorf("/sx did not warn:\n%s", view())
	}
}

func TestTabAndTheArrowsTakeFromTheHintRow(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")

	typeInto(c, st, "/s")
	pressKey(c, st, tea.KeyTab)
	if c.in.Value() != "/send " {
		t.Errorf("Tab on /s gave %q", c.in.Value())
	}
	c.in.Reset()

	// Right twice from /help is /send; left from /help wraps to /quit.
	typeInto(c, st, "/")
	pressKey(c, st, tea.KeyRight)
	pressKey(c, st, tea.KeyRight)
	if _, body, _ := c.view(st, 100); !strings.Contains(stripANSI(body), "▸ /send <path>") {
		t.Errorf("two rights did not reach /send:\n%s", stripANSI(body))
	}
	pressKey(c, st, tea.KeyTab)
	if c.in.Value() != "/send " {
		t.Errorf("Tab on the pick gave %q", c.in.Value())
	}
	c.in.Reset()

	typeInto(c, st, "/")
	pressKey(c, st, tea.KeyLeft)
	if _, body, _ := c.view(st, 100); !strings.Contains(stripANSI(body), "▸ /quit") {
		t.Errorf("left did not wrap to /quit:\n%s", stripANSI(body))
	}

	// A letter typed resets the pick to the first candidate.
	typeInto(c, st, "c")
	if _, body, _ := c.view(st, 100); !strings.Contains(
		stripANSI(body),
		"/clear   wipe the screen",
	) {
		t.Errorf("/c:\n%s", stripANSI(body))
	}
}

func TestEnterTakesThePickRunningItWhenItWantsNothing(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")

	// /s wants a path: Enter finishes the word and waits.
	typeInto(c, st, "/s")
	if _, leave := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter}); leave {
		t.Fatal("left")
	}
	if c.in.Value() != "/send " {
		t.Errorf("Enter on /s gave %q", c.in.Value())
	}
	c.in.Reset()

	// A lone slash is /help, as in version 1.
	typeInto(c, st, "/")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if plain := stripANSI(strings.Join(c.lines, "\n")); !strings.Contains(
		plain,
		"/quit         leave the conversation",
	) {
		t.Errorf("a lone slash did not list the commands:\n%s", plain)
	}
	if c.in.Value() != "" {
		t.Errorf("the slash stayed: %q", c.in.Value())
	}

	// An arrow to /quit and Enter leaves.
	typeInto(c, st, "/")
	pressKey(c, st, tea.KeyLeft)
	if _, leave := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter}); !leave {
		t.Error("Enter on the picked /quit did not leave")
	}
}

func TestAPasteLandsInTheLineAndTheHintFollows(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")
	_, _ = c.update(st, tea.PasteMsg{Content: "salam az paste"})
	if c.in.Value() != "salam az paste" {
		t.Errorf("line after a paste: %q", c.in.Value())
	}
	c.in.Reset()
	_, _ = c.update(st, tea.PasteMsg{Content: "/se"})
	if _, body, _ := c.view(st, 100); !strings.Contains(stripANSI(body), "/send <path>") {
		t.Errorf("a pasted command word got no hint:\n%s", stripANSI(body))
	}
}

// /who without a connection (as tests have it) says the name and where it
// came from; a probe's answer is the second line.
func TestWhoAndThePathLine(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")
	typeInto(c, st, "/who")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	plain := stripANSI(strings.Join(c.lines, "\n"))
	if !strings.Contains(plain, `alice, calling themselves "alice"`) {
		t.Errorf("who:\n%s", plain)
	}

	c.pathLine(
		st,
		pathProbed{path: peer.Path{Direct: true, Latency: 38 * time.Millisecond, Rx: 1100, Tx: 42}},
	)
	c.pathLine(st, pathProbed{path: peer.Path{Relay: "fra"}})
	c.pathLine(st, pathProbed{err: peer.ErrPathUnknown})
	c.pathLine(st, pathProbed{path: peer.Path{Direct: true, Latency: 400 * time.Microsecond}})
	plain = stripANSI(strings.Join(c.lines, "\n"))
	for _, want := range []string{
		"path: direct  ·  38 ms  ·  for 0 s  ·  ↑ 42 B  ↓ 1.1 KB",
		"path: through the relay fra  ·  for 0 s", "path: direct  ·  <1 ms  ·  for 0 s", "path: not known on the side that answered; the caller's /who can tell  ·  for 0 s",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("lacks %q:\n%s", want, plain)
		}
	}
}

func TestSinceTextSpeaksLikeAPerson(t *testing.T) {
	t.Parallel()

	cases := map[time.Duration]string{
		12 * time.Second: "12 s", 90 * time.Second: "1 min", 61 * time.Minute: "1 h 1 min",
	}
	for d, want := range cases {
		if got := sinceText(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

// /me prints this machine's own three groups into the pane, so they can be
// read out without leaving the conversation; /me copy also copies.
func TestMeInAConversationPrintsAndCopies(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no clipboard tool: nothing of the developer's is touched

	st := newStyles(true)
	c := newConversation(st, 100, 20, testLine("alice", true), "alice", "")
	c.me = func(int) (string, string) { return "ADDRESS\ntcpTESTADDR\n\nRELAY\nfra · Frankfurt", "tcpTESTADDR" }

	typeInto(c, st, "/me")
	cmd, _ := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	plain := stripANSI(strings.Join(c.lines, "\n"))
	for _, want := range []string{"ADDRESS", "tcpTESTADDR", "RELAY", "fra · Frankfurt"} {
		if !strings.Contains(plain, want) {
			t.Errorf("/me lacks %q:\n%s", want, plain)
		}
	}
	if cmd != nil {
		t.Error("/me alone copied something")
	}

	typeInto(c, st, "/me copy")
	cmd, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("/me copy copied nothing")
	}
	if !strings.Contains(stripANSI(strings.Join(c.lines, "\n")), "tcpTESTADDR") {
		t.Error("/me copy did not print the address too")
	}

	typeInto(c, st, "/me now")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(stripANSI(strings.Join(c.lines, "\n")), "or the word copy") {
		t.Error("/me now was taken as an answer")
	}
}

// A conversation draws no notice line, so a copy says so in the pane.
func TestACopyInAConversationIsSaidInThePane(t *testing.T) {
	t.Parallel()

	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m.screen = screenConversation
	m.conv = newConversation(m.st, 100, 20, testLine("alice", true), "alice", "")
	next, _ := m.Update(copied{tool: "pbcopy"})
	m = next.(model)
	if m.notice != "" {
		t.Errorf("a notice nobody can see: %q", m.notice)
	}
	if !strings.Contains(
		stripANSI(strings.Join(m.conv.lines, "\n")),
		"through the terminal and pbcopy",
	) {
		t.Errorf("pane:\n%s", stripANSI(strings.Join(m.conv.lines, "\n")))
	}
}
