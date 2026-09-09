package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Serajian/homa/internal/contacts"
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
	c.note(st, "[alice] salam")

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
	c.note(st, "alice left the conversation.")

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
	plain := stripANSI(c.render())
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
	if plain := stripANSI(c.render()); !strings.Contains(
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
	plain := stripANSI(c.render())
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
	plain = stripANSI(c.render())
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
	plain := stripANSI(c.render())
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
	if !strings.Contains(stripANSI(c.render()), "tcpTESTADDR") {
		t.Error("/me copy did not print the address too")
	}

	typeInto(c, st, "/me now")
	_, _ = c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(stripANSI(c.render()), "or the word copy") {
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
		stripANSI(m.conv.render()),
		"through the terminal and pbcopy",
	) {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}
}

// words makes a message of n numbered words, so the last of them proves the
// whole thing survived.
func words(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("w%d", i+1)
	}
	return strings.Join(parts, " ")
}

// The fault this fixed: a message wider than the terminal was cut and the
// rest of it could not be reached at all.
func TestALongMessageIsWrappedAndWhollyReadable(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	for _, width := range []int{60, 200} {
		c := newConversation(st, width, 20, testLine("alice", true), "alice", "")
		c.msg(st, "alice", words(600), false)

		rows := strings.Split(stripANSI(c.render()), "\n")
		if !strings.Contains(rows[len(rows)-1], "w600") {
			t.Errorf(
				"%d columns: the message does not end where it should:\n%s",
				width,
				rows[len(rows)-1],
			)
		}
		for i, row := range rows {
			if got := lipgloss.Width(row); got > width {
				t.Errorf("%d columns: row %d is %d wide: %q", width, i, got, row)
			}
		}
		// The name is on the first row and the rest sit under the words.
		if !strings.HasPrefix(rows[0], "    alice │ w1 ") {
			t.Errorf("%d columns: first row %q", width, rows[0])
		}
		if !strings.HasPrefix(rows[1], strings.Repeat(" ", nameColumn)+" │ ") {
			t.Errorf("%d columns: second row %q", width, rows[1])
		}
	}
}

// A resize wraps what is already in the pane again, rather than leaving it
// at the width it was drawn at.
func TestAResizeWrapsWhatIsAlreadyThere(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 60, 20, testLine("alice", true), "alice", "")
	c.msg(st, "alice", words(600), false)
	narrow := len(strings.Split(c.render(), "\n"))

	c.resize(200, 20)
	wide := len(strings.Split(c.render(), "\n"))
	if wide >= narrow {
		t.Errorf("60 columns took %d rows, 200 took %d", narrow, wide)
	}
	if !strings.Contains(stripANSI(c.render()), "w600") {
		t.Error("the end of the message did not survive the resize")
	}

	c.resize(60, 20)
	if back := len(strings.Split(c.render(), "\n")); back != narrow {
		t.Errorf("back at 60 columns it took %d rows, not %d", back, narrow)
	}
}

// Color must survive the wrapping: every row of a wrapped note carries it,
// and stripping it leaves exactly the plain pane.
func TestAWrappedLineKeepsItsColorOnEveryRow(t *testing.T) {
	t.Parallel()

	colored := newConversation(newStyles(true), 60, 20, testLine("alice", true), "alice", "")
	plain := newConversation(plainStyles(true), 60, 20, testLine("alice", true), "alice", "")
	for _, c := range []*conversation{colored, plain} {
		st := newStyles(true)
		if c == plain {
			st = plainStyles(true)
		}
		c.note(st, st.dim.Render(words(80)))
		c.alert(st, words(40))
	}
	if got, want := stripANSI(colored.render()), plain.render(); got != want {
		t.Errorf("stripped:\n%s\nplain:\n%s", got, want)
	}
	rows := strings.Split(colored.render(), "\n")
	for i, row := range rows {
		if !strings.Contains(row, "\x1b[") {
			t.Errorf("row %d lost its color: %q", i, row)
		}
	}
}

// The fault this closes: the side that answered a call held ten bytes of a
// key and nothing to dial. An address handed over can be kept with /add,
// and nothing is written until it is.
func TestAnAddressHandedOverIsKeptOnlyWhenAsked(t *testing.T) {
	sandboxHome(t)

	deps := testDeps(t)
	m := sized(newModel(t.Context(), deps, newStyles(true)))
	m.screen = screenConversation
	m.conv = newConversation(m.st, 100, 24, testLine("~bob", false), "bob", "")

	// It arrives, and says how to keep it. Nothing is in the book yet.
	next, _ := m.Update(addressGiven{addr: realAddr})
	m = next.(model)
	pane := stripANSI(m.conv.render())
	if !strings.Contains(pane, "sent you their address") ||
		!strings.Contains(pane, `/add keeps them as bob`) {
		t.Errorf("pane:\n%s", pane)
	}
	if deps.Book.Len() != 0 {
		t.Fatal("an address was saved before anybody asked")
	}

	// /add with no name keeps them under the name they announced.
	typeInto(m.conv, m.st, "/add")
	cmd, _ := m.conv.update(m.st, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("/add did nothing")
	}
	next, _ = m.Update(cmd())
	m = next.(model)

	c, err := deps.Book.ByName("bob")
	if err != nil || c.Addr != realAddr {
		t.Fatalf("book has %+v, %v", c, err)
	}
	pane = stripANSI(m.conv.render())
	if !strings.Contains(pane, "saved as bob") {
		t.Errorf("pane:\n%s", pane)
	}
	// The header calls them by the name from now on.
	if !m.conv.l.known || m.conv.l.name != "bob" {
		t.Errorf("line is %q known=%v", m.conv.l.name, m.conv.l.known)
	}
	// Asking again is answered with the truth: you already have it.
	typeInto(m.conv, m.st, "/add")
	cmd, _ = m.conv.update(m.st, tea.KeyPressMsg{Code: tea.KeyEnter})
	next, _ = m.Update(cmd())
	m = next.(model)
	if !strings.Contains(stripANSI(m.conv.render()), "that is the address you already have") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}
}

// A contact whose address has changed can take the new one, but only when
// they are named: it replaces what is on disk.
func TestANewAddressReplacesTheSavedOneOnlyWhenNamed(t *testing.T) {
	sandboxHome(t)

	deps := testDeps(t, "bob")
	old, err := deps.Book.ByName("bob")
	if err != nil {
		t.Fatal(err)
	}

	m := sized(newModel(t.Context(), deps, newStyles(true)))
	m.screen = screenConversation
	m.conv = newConversation(m.st, 100, 24, testLine("bob", true), "bob", "")

	next, _ := m.Update(addressGiven{addr: realAddr})
	m = next.(model)
	if !strings.Contains(stripANSI(m.conv.render()), "already in your address book as bob") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}

	// /add alone will not replace an address that is already there.
	typeInto(m.conv, m.st, "/add")
	cmd, _ := m.conv.update(m.st, tea.KeyPressMsg{Code: tea.KeyEnter})
	next, _ = m.Update(cmd())
	m = next.(model)
	if got, _ := deps.Book.ByName("bob"); got.Addr != old.Addr {
		t.Fatal("the address was replaced without being asked for")
	}
	if !strings.Contains(stripANSI(m.conv.render()), "/add bob replaces the one you have") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}

	// Naming them is the yes.
	typeInto(m.conv, m.st, "/add bob")
	cmd, _ = m.conv.update(m.st, tea.KeyPressMsg{Code: tea.KeyEnter})
	next, _ = m.Update(cmd())
	m = next.(model)
	got, err := deps.Book.ByName("bob")
	if err != nil || got.Addr != realAddr {
		t.Fatalf("book has %+v, %v", got, err)
	}
	if !strings.Contains(stripANSI(m.conv.render()), "address is replaced") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}
	// And it is on disk, not only in memory.
	reloaded, err := contacts.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c, err := reloaded.ByName("bob"); err != nil || c.Addr != realAddr {
		t.Errorf("on disk: %+v, %v", c, err)
	}
}

func TestAddWithNothingSentAndABadAddressAreRefused(t *testing.T) {
	sandboxHome(t)

	deps := testDeps(t)
	m := sized(newModel(t.Context(), deps, newStyles(true)))
	m.screen = screenConversation
	m.conv = newConversation(m.st, 100, 24, testLine("~bob", false), "bob", "")

	typeInto(m.conv, m.st, "/add")
	if cmd, _ := m.conv.update(m.st, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Error("/add kept something nobody sent")
	}
	if !strings.Contains(stripANSI(m.conv.render()), "nobody has sent you an address") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}

	// Something that is not an address never reaches the book.
	next, _ := m.Update(addressGiven{addr: "tcpNOTANADDRESS"})
	m = next.(model)
	next, _ = m.Update(keepAddress{name: "bob", addr: "tcpNOTANADDRESS"})
	m = next.(model)
	if deps.Book.Len() != 0 {
		t.Error("a bad address was saved")
	}
	if !strings.Contains(stripANSI(m.conv.render()), "does not look like a homa address") {
		t.Errorf("pane:\n%s", stripANSI(m.conv.render()))
	}
}

// /me send needs a peer new enough to understand it, and says so when the
// peer is not, rather than letting a silent drop pass for a delivery.
func TestMeSendTellsYouWhenThePeerIsTooOld(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	c := newConversation(st, 100, 24, testLine("alice", true), "alice", "")
	c.me = func(int) (string, string) { return "ADDRESS\ntcpX", "tcpX" }

	// testLine has no session at all, which is the same as having nothing
	// to send through: the command must not pretend it went.
	typeInto(c, st, "/me send")
	if cmd, _ := c.update(st, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Error("an address was sent with no session")
	}
	if strings.Contains(stripANSI(c.render()), "went to") {
		t.Errorf("pane:\n%s", stripANSI(c.render()))
	}
}
