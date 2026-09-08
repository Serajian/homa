# Full-Screen Interface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task, inline in this session. Steps use checkbox (`- [ ]`) syntax for tracking. **This project's hard rules override the skill's habits:** nothing is committed except on the user's exact phrase `commit kon`, nothing pushed except on `push kon`; every task ends with a report in `docs/changes/` and a checkpoint, not a commit; one file at a time, each described before it is written.

**Goal:** Replace homa's line-based terminal front end with a full-screen one on bubbletea v2, so a message can never land on a half-typed line and a line never disappears unsent, while nothing below `internal/ui` changes.

**Architecture:** One `tea.Model` in `internal/ui` with a `screen` value and a sub-model per screen; every event from the network arrives as a `tea.Msg` through `Program.Send` from adapters that wrap the interfaces `session`, `peer` and `contacts` already expose; `View` composes a frame (status line, body, key line) with lipgloss styles held as data. The line machinery of version 1 (pump, prompts, erasing, countdown, hand-rolled color) is deleted once the last screen has moved.

**Tech Stack:** Go 1.27; `charm.land/bubbletea/v2` v2.0.9, `charm.land/bubbles/v2` v2.2.1 (`textinput`, `viewport`), `charm.land/lipgloss/v2` v2.0.6, `github.com/charmbracelet/colorprofile`; `github.com/charmbracelet/x/exp/teatest/v2` for flow tests; `golang.org/x/term` (already direct) for the terminal check.

**Spec:** `docs/design/2026-09-08-full-screen-interface.md`

## Global Constraints

- Module import paths are the vanity ones: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/...`, `charm.land/lipgloss/v2` (the GitHub paths refuse to resolve for v2).
- Only `internal/ui` changes, plus `cmd/homa/main.go`/`boot.go` (the call into `ui`) and `go.mod`/`go.sum`. Anything needed from below `ui` is a leak: stop, report, fix below on its own first.
- One conversation at a time. Output that is not a terminal: print `homa: needs a terminal` to stderr, exit 1. No line-mode fallback. No alternate screen (`View.AltScreen` stays false).
- Color meanings are fixed by `docs/decisions.md`: you = bold in the terminal's foreground, them = green `#22E6A7` (ANSI `2` when the profile has no truecolor — lipgloss/bubbletea downsample automatically), homa = grey `#9AA3AD`, warning = the terminal's yellow (ANSI `3`). Color is never the only signal; every screen's colored output stripped of escapes must equal its plain output. `-no-color`, `NO_COLOR` and `TERM=dumb` turn color off.
- Network text (nicks, messages, file names) is passed to `Style.Render` as content, never formatted into a style or escape.
- Every wording from version 1 is kept verbatim unless the spec's mockups show otherwise. Constants live in `internal/ui/const.go`. Comments say why. `make lint` must pass. Tests live in `internal/ui`; hermetic; `make test` stays fast.
- Commit messages, when the user asks for a commit, are the user's own; Claude never appears in git.

---

## File Structure

New files in `internal/ui` (the old ones stay compiling until Task 5 deletes them):

| File | Responsibility |
| --- | --- |
| `run.go` | `Deps`, `Run(ctx, deps, opts)`: the terminal check, the program, the accept loop command, wiring the adapters |
| `model.go` | the root `model`: `screen`, sub-models, `Init`/`Update`/`View` dispatch, window size, quit |
| `msgs.go` | every `tea.Msg` type that crosses from below or from timers |
| `styles.go` | `styles` (lipgloss styles as data), `newStyles(unicode bool)`, `sep` |
| `frame.go` | `frame(st, width, height, status, body, keys) string`, the status line and key line builders |
| `screen_menu.go` | the menu sub-model: groups (reusing `menuItem`), cursor, key dispatch |
| `callbar.go` | the incoming/outgoing call bar and its `tea.Tick` countdown |
| `screen_conversation.go` | the conversation sub-model: `viewport`, `textinput` with history, commands |
| `adapter.go` | `sessionAdapter` implementing `session.Handler` and `session.FileHandler` by calling `Send` |
| `calls.go` | dialing, greeting, describing and parking calls, as commands returning messages (moved from `menu.go`/`chat.go`) |
| `screen_contacts.go`, `screen_settings.go`, `screen_setup.go`, `screen_help.go`, `screen_reset.go` | the small screens |
| `history.go` | the input history ring (`textinput` has none) |

Tests mirror files: `model_test.go`, `styles_test.go`, `frame_test.go`, `screen_menu_test.go`, `callbar_test.go`, `screen_conversation_test.go`, `history_test.go`, `flow_test.go` (teatest).

Deleted in Task 5: `ui.go`, `prompt.go`, `countdown.go`, `style.go`, `theme.go`, `welcome.go`, `menu.go`, `chat.go`, `handler.go`, `contacts.go`, `setup.go`, `help.go` and their tests, once every behavior has a home in the new files. `files.go` (the listing and `resolveSend`) and `format.go` stay as they are. `const.go` keeps the non-terminal constants (`bannerArt`, `wordBack`, timeouts, limits); the escape constants go.

---

### Task 0: Dependencies and the terminal check

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/ui/run.go`
- Test: `internal/ui/run_test.go`

**Interfaces:**
- Produces: `func needsTerminal(in, out *os.File) error` — nil when both are terminals, otherwise `errors.New("needs a terminal")`. `var ErrNeedsTerminal`.

- [ ] **Step 1: Add the modules**

```bash
go get charm.land/bubbletea/v2@v2.0.9 charm.land/bubbles/v2@v2.2.1 charm.land/lipgloss/v2@v2.0.6 github.com/charmbracelet/x/exp/teatest/v2@v2.0.0-20260906004030-3986e9119cf9
go mod tidy
```

Expected: `go.mod` gains the four requires; `go build ./...` still clean (nothing imports them yet).

- [ ] **Step 2: Write the failing test**

```go
// run_test.go
package ui

import (
	"errors"
	"os"
	"testing"
)

// A pipe is not a terminal, and homa says so rather than drawing into it.
func TestAPipeIsRefused(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if err := needsTerminal(r, w); !errors.Is(err, ErrNeedsTerminal) {
		t.Errorf("a pipe was accepted: %v", err)
	}
}
```

- [ ] **Step 3: Run it, expect a compile failure** — `go test ./internal/ui/ -run TestAPipeIsRefused` → `undefined: needsTerminal`.

- [ ] **Step 4: Implement**

```go
// run.go
package ui

import (
	"errors"
	"os"

	"golang.org/x/term"
)

// ErrNeedsTerminal is returned when homa is started with something other
// than a terminal on either end: a full-screen program has nowhere to draw
// in a pipe, and nobody chats through one. Version 1 printed plain text
// there; version 2 says so and stops, because two interfaces would mean
// every later feature twice (see docs/design/2026-09-08-full-screen-interface.md).
var ErrNeedsTerminal = errors.New("needs a terminal")

func needsTerminal(in, out *os.File) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return ErrNeedsTerminal
	}
	return nil
}
```

- [ ] **Step 5: Run it, expect PASS.** Then `make lint`.

- [ ] **Step 6: Checkpoint** — report `docs/changes/<date>-full-screen-0-deps.md`; say it is ready for `commit kon`.

---

### Task 1: Styles and the frame

**Files:**
- Create: `internal/ui/styles.go`, `internal/ui/frame.go`
- Modify: `internal/ui/const.go` (add `sepUnicode`/`sepASCII` are already there; add `frameMinWidth = 50`, `statusHeight = 1`, `keysHeight = 1`)
- Test: `internal/ui/styles_test.go`, `internal/ui/frame_test.go`

**Interfaces:**
- Produces:
  ```go
  type styles struct {
      you, them, dim, warn lipgloss.Style
      rule                 lipgloss.Style // the dashed line above the input
      unicode              bool
  }
  func newStyles(unicode bool) *styles   // by pointer: five lipgloss styles are kilobytes, and there is one
  func (s *styles) sep() string                      // "  ·  " or "  -  "
  func (s *styles) peer(name string) string          // green name, grey unknownMark
  func frame(width, height int, status, body, keys string) string
  ```
  `frame` returns exactly `height` lines of at most `width` cells: status on line 0, body lines padded or cut to `height-2`, keys on the last line.

- [ ] **Step 1: Failing tests**

```go
// styles_test.go
func TestPeerPaintsTheMarkGreyAndTheNameGreen(t *testing.T) {
	t.Parallel()
	s := newStyles(true)
	got := s.peer("~bob")
	want := s.dim.Render(unknownMark) + s.them.Render("bob")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if s.peer("alice") != s.them.Render("alice") {
		t.Error("a saved name is not green")
	}
}

func TestSepNeedsUnicode(t *testing.T) {
	t.Parallel()
	if newStyles(false).sep() != sepASCII || newStyles(true).sep() != sepUnicode {
		t.Error("separator does not follow the locale")
	}
}
```

```go
// frame_test.go
func TestFrameIsExactlyTheTerminalTall(t *testing.T) {
	t.Parallel()
	s := newStyles(true)
	got := frame(s, 40, 6, "status", "a\nb", "keys")
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("%d lines, want 6:\n%s", len(lines), got)
	}
	if lines[0] != "status" || lines[1] != "a" || lines[2] != "b" || lines[5] != "keys" {
		t.Errorf("wrong placement:\n%s", got)
	}
	for i, l := range lines {
		if lipgloss.Width(l) > 40 {
			t.Errorf("line %d is %d wide", i, lipgloss.Width(l))
		}
	}
}

func TestFrameCutsABodyTooTall(t *testing.T) {
	t.Parallel()
	got := frame(newStyles(true), 20, 4, "s", "1\n2\n3\n4\n5", "k")
	if strings.Split(got, "\n")[2] != "5" { // the last body lines win: a conversation shows its newest
		t.Errorf("body not cut from the top:\n%s", got)
	}
}
```

- [ ] **Step 2: Run, expect compile failures.**

- [ ] **Step 3: Implement**

```go
// styles.go
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// styles is the look, held as data: what color means is decided in
// docs/decisions.md, and this is the only place those meanings become
// escapes. Changing the look is changing this value, not the screens.
type styles struct {
	you, them, dim, warn lipgloss.Style
	rule                 lipgloss.Style
	unicode              bool
}

func newStyles(unicode bool) styles {
	return styles{
		you:     lipgloss.NewStyle().Bold(true),
		them:    lipgloss.NewStyle().Foreground(lipgloss.Color("#22E6A7")),
		dim:     lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA3AD")),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		rule:    lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA3AD")).Faint(true),
		unicode: unicode,
	}
}

func (s styles) sep() string {
	if s.unicode {
		return sepUnicode
	}
	return sepASCII
}

// peer paints a name the way the far side is always shown. The name is
// content passed to Render, never part of a style.
func (s styles) peer(name string) string {
	if strings.HasPrefix(name, unknownMark) {
		return s.dim.Render(unknownMark) + s.them.Render(strings.TrimPrefix(name, unknownMark))
	}
	return s.them.Render(name)
}
```

```go
// frame.go
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// frame lays one screen out: the status line, the body, the key line. It
// always returns exactly height lines, so the renderer never scrolls the
// terminal, and it cuts the body from the top because the newest lines of
// a conversation are the ones that matter.
func frame(s styles, width, height int, status, body, keys string) string {
	if height < 3 {
		height = 3
	}
	bodyH := height - statusHeight - keysHeight
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) > bodyH {
		lines = lines[len(lines)-bodyH:]
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	cut := lipgloss.NewStyle().MaxWidth(width)
	out := make([]string, 0, height)
	out = append(out, cut.Render(status))
	for _, l := range lines {
		out = append(out, cut.Render(l))
	}
	out = append(out, cut.Render(keys))
	return strings.Join(out, "\n")
}
```

- [ ] **Step 4: Run tests, expect PASS; `make lint`.**
- [ ] **Step 5: Checkpoint** — report `…-full-screen-1-styles-frame.md`; ready for `commit kon`.

---

### Task 2: Messages, the root model, the menu, and the switch-over

This is the step after which `homa` runs on the new interface with a working menu and quit; every other key prints a grey "not yet" line. The old screens are unreachable but still compile.

**Files:**
- Create: `internal/ui/msgs.go`, `internal/ui/model.go`, `internal/ui/screen_menu.go`
- Modify: `internal/ui/run.go` (add `Deps`, `Run`), `cmd/homa/main.go`, `cmd/homa/boot.go`
- Test: `internal/ui/model_test.go`, `internal/ui/screen_menu_test.go`

**Interfaces:**
- Consumes: `styles`, `frame` (Task 1); `contacts.Book.All()`, `peer.Listener.Addr()`, `config.Config.Nick`.
- Produces:
  ```go
  // run.go
  type Deps struct {
      Cfg      *config.Config
      Book     *contacts.Book
      ID       *peer.Identity
      Listener *peer.Listener
      NoColor  bool
  }
  func CheckTerminal() error                       // for cmd/homa, before bootstrap: a pipe is refused before anything is created
  func Run(ctx context.Context, deps Deps) error   // runs the program, returns when it quits

  // model.go
  type screen int
  const (screenMenu screen = iota; screenConversation; screenContacts; screenContact; screenAddContact; screenSettings; screenSetup; screenReset; screenHelp)
  type model struct {
      deps   Deps
      st     *styles
      width, height int
      screen screen
      menu   menuModel
      notice string          // one grey line under the body, cleared on the next key
  }
  func newModel(deps Deps, st styles) model
  func (m model) Init() tea.Cmd
  func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
  func (m model) View() tea.View

  // msgs.go (this task adds only these; later tasks append)
  type noticeMsg string       // something homa wants to say once, in grey

  // screen_menu.go
  type menuModel struct { groups [][]menuItem; contacts []contacts.Contact; cursor int }
  func newMenu(book *contacts.Book) menuModel
  func (mm menuModel) view(s styles) string
  func (mm menuModel) key(k string) (action menuAction, ok bool)   // "1".."n", "n","b","a","s","c","h","r","q", "enter", "up", "down"
  type menuAction int
  const (actNone menuAction = iota; actCall; actAdd; actContacts; actAddress; actSettings; actClear; actHelp; actReset; actQuit)
  ```
  `menuItem` and `menuEntries`' grouping move here from `prompt.go`/`menu.go` unchanged in wording.

- [ ] **Step 1: Failing tests**

```go
// screen_menu_test.go
func TestMenuKeysMapToActionsAndTheCursorCalls(t *testing.T) {
	t.Parallel()
	book := contacts.NewBook() // or however a test book is made in contacts_test.go today: copy that helper
	_ = book.Add(contacts.Contact{Name: "alice", Addr: testAddr})
	_ = book.Add(contacts.Contact{Name: "bob", Addr: testAddr})
	mm := newMenu(book)

	cases := map[string]menuAction{"1": actCall, "2": actCall, "n": actAdd, "b": actContacts, "a": actAddress,
		"s": actSettings, "c": actClear, "h": actHelp, "r": actReset, "q": actQuit, "enter": actCall}
	for k, want := range cases {
		if got, ok := mm.key(k); !ok || got != want {
			t.Errorf("%q → %v,%v want %v", k, got, ok, want)
		}
	}
	if _, ok := mm.key("x"); ok {
		t.Error("x is not a key")
	}
	mm.cursor = 1
	if _, ok := mm.key("down"); !ok || mm.cursor != 1 { // down at the end stays
		t.Error("cursor ran off the end")
	}
}

func TestMenuViewIsGroupedAndMarksTheCursor(t *testing.T) {
	t.Parallel()
	mm := newMenu(bookWith(t, "alice", "~bob"))
	got := stripANSI(mm.view(newStyles(true)))
	want := "  ▸ call alice\n    call ~bob\n\n    n  add a contact\n    b  contacts: rename, forget, call\n    a  show my address\n\n    s  settings\n    c  clear the screen\n    h  help\n\n    r  start over: forget everything\n    q  quit homa\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
```

```go
// model_test.go
func TestQuitFromTheMenu(t *testing.T) {
	t.Parallel()
	m := newModel(testDeps(t), newStyles(true))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil || cmd() != (tea.QuitMsg{}) {
		t.Error("q did not quit")
	}
}

func TestViewIsTheTerminalsSize(t *testing.T) {
	t.Parallel()
	m := newModel(testDeps(t), newStyles(true))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	v := mm.(model).View()
	if n := strings.Count(v.Content, "\n") + 1; n != 12 {
		t.Errorf("%d lines, want 12", n)
	}
	if v.AltScreen {
		t.Error("the alternate screen must stay off: what was on screen stays in scrollback")
	}
}
```

`stripANSI`, `bookWith`, `testDeps` are helpers in `model_test.go`: `stripANSI` is the regexp from `theme_test.go` (`\x1b\[[0-9;]*m`); `testDeps` builds `Deps` with a default `config.Default()`, an empty book, and nil `ID`/`Listener` (the menu does not touch them; `Run` is not called in unit tests).

- [ ] **Step 2: Run, expect compile failures.**

- [ ] **Step 3: Implement `msgs.go`, `screen_menu.go`, `model.go`**

`screen_menu.go` builds the same four groups `menuEntries` builds today; `view` renders each line as `markInfo + cursorMark + key + "  " + text` where `cursorMark` is `"▸ "` on the cursor row of the contacts group and `"  "` elsewhere (ASCII `"> "` when `!s.unicode`), keys through `s.you`, quiet rows through `s.dim`, names through `s.peer`. `key` maps as in the test; `"up"`/`"down"` move the cursor within the contacts and return `actNone, true`.

`model.go`:

```go
func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case noticeMsg:
		m.notice = string(msg)
		return m, nil
	case tea.KeyPressMsg:
		m.notice = ""
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		}
	}
	return m, nil
}

func (m model) updateMenu(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	act, ok := m.menu.key(msg.String())
	if !ok {
		m.notice = "that is not one of the choices" + m.st.sep() + "press one of the keys on the left, or h for help"
		return m, nil
	}
	switch act {
	case actQuit:
		return m, tea.Quit
	case actClear:
		return m, nil // the frame is redrawn whole; nothing to clear
	case actNone:
		return m, nil
	default:
		m.notice = "not yet" // replaced screen by screen in later tasks
		return m, nil
	}
}

func (m model) View() tea.View {
	status := m.st.dim.Render("homa"+m.st.sep()+"you are ") + m.st.you.Render(m.deps.Cfg.Nick) +
		m.st.dim.Render(m.st.sep()+preview(m.deps.Listener.Addr())+m.st.sep()+"listening")
	body := m.menu.view(m.st)
	if m.notice != "" {
		body += "\n" + markInfo + m.st.dim.Render(m.notice)
	}
	keys := m.st.dim.Render("↑↓ choose · Enter call · or press a key")
	v := tea.NewView(frame(m.st, m.width, m.height, status, body, keys))
	v.AltScreen = false
	return v
}
```

(`preview` exists in `menu.go` today; move it to `screen_menu.go`. When `Listener` is nil in tests, `status` must not dereference it: `addrPreview := "…"; if m.deps.Listener != nil { … }`.)

`run.go`:

```go
func Run(ctx context.Context, deps Deps) error {
	if err := needsTerminal(os.Stdin, os.Stdout); err != nil {
		return err
	}
	st := newStyles(styleFor(0, os.Getenv).unicode) // styleFor stays until Task 5; then a one-line locale check replaces it
	opts := []tea.ProgramOption{tea.WithContext(ctx)}
	if deps.NoColor {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}
	p := tea.NewProgram(newModel(deps, st), opts...)
	_, err := p.Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, context.Canceled) {
		return context.Canceled // main treats Ctrl+C as a quiet exit
	}
	return err
}
```

- [ ] **Step 4: Switch `cmd/homa` over**

`main.go`: replace `out := ui.New(os.Stdin, os.Stdout)` … `app.Run(ctx)` … `out.Info("bye.")` with:

```go
	deps, cleanup, err := bootstrap(ctx, opts.noColor)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := ui.Run(ctx, deps); err != nil {
		if errors.Is(err, ui.ErrNeedsTerminal) {
			return errors.New("needs a terminal")
		}
		return err
	}
	fmt.Println("bye.")
	return nil
```

`boot.go`: `bootstrap(ctx, noColor bool) (ui.Deps, func(), error)`; the first-run setup (`ui.Setup`) needs the old `*UI` until Task 4 moves it — for this task, when `config.Load` reports `ErrNotFound`, return `errors.New("first run: settings are not yet asked on the new interface")` so the build stays honest; `make run` on a machine with settings works. (Task 4 replaces this with the setup screen.)

- [ ] **Step 5: Run tests, `make lint`, `make build`, then start the binary in a real terminal: the banner is not yet drawn (Task 4), the menu shows, `q` quits, `x` shows the grey hint. Then run `./build/homa | cat` and expect `homa: needs a terminal`, exit 1.**
- [ ] **Step 6: Checkpoint** — report `…-full-screen-2-menu.md`; ready for `commit kon`.

---

### Task 3: Calls and the conversation

After this task the two faults of version 1 are gone: a call is taken with `y`, the conversation has a scrolling pane and an input line, and a message arriving while typing goes to the pane.

**Files:**
- Create: `internal/ui/adapter.go`, `internal/ui/calls.go`, `internal/ui/callbar.go`, `internal/ui/history.go`, `internal/ui/screen_conversation.go`
- Modify: `internal/ui/msgs.go`, `internal/ui/model.go`, `internal/ui/run.go`
- Test: `internal/ui/history_test.go`, `internal/ui/callbar_test.go`, `internal/ui/screen_conversation_test.go`, `internal/ui/flow_test.go`

**Interfaces:**
- Consumes: `peer.Listener.Accept(ctx) (net.Conn, error)`, `peer.Dial(ctx, id, addr)`, `session.Start(conn, nick, h) (*Session, error)`, `Session.SignalsAcceptance/SendAccept/WaitAccepted/SendText/Run/Close/Peer`, `peer.RemoteKey/RemoteKeyPrefix`, `contacts.Book.ByPubKey/ByPubKeyPrefix/SetPubKey/Save`.
- Produces (`msgs.go`):
  ```go
  type callArrived struct{ c *call }              // c: the parked call struct moved from menu.go (name, known, conn, session, deadline, claim)
  type callAnswered struct{ s *session.Session; conn net.Conn; name string; known bool }
  type callRefused struct{ reason string }        // "they are not taking calls right now", "no answer", "busy"
  type callFailed struct{ name string; err error }
  type peerSaid struct{ text string }
  type peerLeft struct{ err error }               // nil: they hung up cleanly
  type tickMsg time.Time
  type lineSent struct{}                          // SendText succeeded; nothing to show
  type sendFailed struct{ err error }
  ```
  `adapter.go`:
  ```go
  type sessionAdapter struct{ send func(tea.Msg); name string; offer chan<- fileOffered } // fileOffered defined in Task 4
  func (a *sessionAdapter) OnText(text string) { a.send(peerSaid{text}) }
  ```
  `calls.go` (all return `tea.Cmd`):
  ```go
  func acceptLoop(ctx context.Context, deps Deps, send func(tea.Msg)) tea.Cmd   // runs for the life of the program; greets like today's greet, parks like today's incoming channel, sends callArrived
  func dial(ctx context.Context, deps Deps, c contacts.Contact, send func(tea.Msg)) tea.Cmd  // today's dial+startChat+awaitAccept, returning callAnswered / callRefused / callFailed
  func takeCall(c *call, send func(tea.Msg)) tea.Cmd    // SendAccept, then callAnswered
  func declineCall(c *call, reason string) tea.Cmd       // today's decline: SendText(reason), Close; returns noticeMsg(format)
  func runSession(ctx, s *session.Session, send) tea.Cmd // s.Run in a goroutine → peerLeft
  ```
  `history.go`:
  ```go
  type history struct{ lines []string; pos int; draft string }
  func (h *history) push(line string)        // appends, resets pos to the end; ignores an empty or repeated line
  func (h *history) up(current string) (string, bool)
  func (h *history) down() (string, bool)   // past the newest returns the draft saved by the first up
  ```
  `callbar.go`:
  ```go
  type callBar struct{ incoming *call; outgoing string; deadline time.Time; giveUp bool }
  func (b callBar) view(s styles, now time.Time) string // "" when nothing is showing
  func tick() tea.Cmd                                   // tea.Tick(countdownStep, func(t time.Time) tea.Msg { return tickMsg(t) })
  ```
  `screen_conversation.go`:
  ```go
  type conversation struct {
      s *session.Session; name string; known bool; nick string
      pane viewport.Model; in textinput.Model; hist history
      lines []string           // rendered lines; the pane's content is their join
      leaving bool
  }
  func newConversation(st styles, width, height int, s *session.Session, name string, known bool, files string) conversation
  func (c *conversation) say(line string)            // append + SetContent + GotoBottom when it was at the bottom
  func (c conversation) view(st styles, width, height int) (status, body, keys string)
  func (c conversation) update(msg tea.Msg) (conversation, tea.Cmd)   // keys: enter → send or command; up/down → history; pgup/pgdn/wheel → pane; else textinput
  ```

- [ ] **Step 1: Failing tests**

```go
// history_test.go
func TestHistoryWalksBackAndKeepsTheDraft(t *testing.T) {
	t.Parallel()
	var h history
	h.push("one"); h.push("two"); h.push("two") // a repeat is not stored twice
	got, ok := h.up("draft")
	if !ok || got != "two" { t.Fatalf("up = %q,%v", got, ok) }
	got, _ = h.up(got)
	if got != "one" { t.Fatalf("up up = %q", got) }
	if _, ok := h.up(got); ok { t.Error("walked past the oldest") }
	h.down(); got, ok = h.down()
	if !ok || got != "draft" { t.Errorf("down past the newest = %q,%v want the draft", got, ok) }
}
```

```go
// callbar_test.go
func TestTheBarCountsDownAndSaysWhatToPress(t *testing.T) {
	t.Parallel()
	s := newStyles(true)
	deadline := time.Now().Add(47 * time.Second)
	b := callBar{incoming: &call{name: "~bob"}, deadline: deadline}
	got := stripANSI(b.view(s, time.Now()))
	if !strings.Contains(got, "~bob is calling") || !strings.Contains(got, "47s") || !strings.Contains(got, "y take it") {
		t.Errorf("bar = %q", got)
	}
	if callBar{}.view(s, time.Now()) != "" {
		t.Error("an empty bar draws something")
	}
}
```

```go
// screen_conversation_test.go — the fault that started version 2
func TestAMessageArrivingWhileTypingDoesNotTouchTheInput(t *testing.T) {
	t.Parallel()
	st := newStyles(true)
	c := newConversation(st, 60, 12, nil, "alice", true, "~/homa-files")
	c, _ = c.update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	c, _ = c.update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	c.say("[alice] salam")
	if c.in.Value() != "hi" {
		t.Errorf("input = %q, the arriving message changed it", c.in.Value())
	}
	_, body, _ := c.view(st, 60, 12)
	if !strings.Contains(stripANSI(body), "[alice] salam") || !strings.Contains(stripANSI(body), "[me] hi") {
		t.Errorf("body:\n%s", body)
	}
}

func TestALineNotYetSentStaysWhenThePeerLeaves(t *testing.T) {
	t.Parallel()
	c := newConversation(newStyles(true), 60, 12, nil, "alice", true, "")
	c, _ = c.update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	c = c.peerLeft(nil, newStyles(true))
	if c.in.Value() != "x" {
		t.Error("the typed line was dropped")
	}
}
```

```go
// flow_test.go — the whole path, with a fake session. session.Start needs a
// net.Conn: use the loopback pair from session's tests (copy connPair here),
// start a real session on each end, and drive one side through the model.
func TestACallIsTakenAndAMessageGoesEachWay(t *testing.T) {
	// 1. connPair; sessA := session.Start(a, "alice", adapterA); sessB started with a
	//    handler that records OnText into a channel.
	// 2. m := newModel(deps, st); tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	// 3. tm.Send(callArrived{&call{name: "~bob", session: sessA, conn: a, deadline: time.Now().Add(time.Minute)}})
	//    teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("~bob is calling")) })
	// 4. tm.Type("y"); WaitFor "talking to ~bob"
	// 5. tm.Type("salam"); tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}); expect B's handler to receive "salam" within a second
	// 6. sessB.SendText("khoobam"); WaitFor "[~bob] khoobam"
	// 7. tm.Type("/quit"); Enter; WaitFor "What now?"; tm.Quit()
}
```

(Write step 1–7 as real code; the comments are the shape, not placeholders — every call named above exists.)

- [ ] **Step 2: Run, expect compile failures.**

- [ ] **Step 3: Implement**, in this order, one file at a time, each described to the user first:
  1. `history.go` (make `history_test.go` pass).
  2. `msgs.go` additions.
  3. `adapter.go`.
  4. `calls.go`: move `greet`, `expire`, `turnAway`, `describe`, `rememberKey`, `dial`, `startChat`, `awaitAccept`, `decline` out of `menu.go`/`chat.go`, keeping their comments and wording, turning each print into a `send(...)`. The accept loop is one long-running `tea.Cmd`; it must `send(callArrived{c})` and keep looping, so it never returns a `Msg` itself — return `func() tea.Msg { for { … } }` and have the loop end only when `ctx` is done (return nil).
  5. `callbar.go` (make `callbar_test.go` pass). The countdown: `tick()` is issued when a bar appears and re-issued on each `tickMsg` while it is showing.
  6. `screen_conversation.go` (make the two conversation tests pass). Input: `textinput.New()`, `Prompt = ""`, `SetVirtualCursor(true)`, `Focus()`; the `[me] ` label is drawn by `view`, not by the prompt, so it is painted like the old `meLabel`. Pane: `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h-4))`, `MouseWheelEnabled = true`. Enter: empty line ignored; `/`-line → command (Task 4 fills the file commands; this task does `/quit`, `/help`, `/who`, `/clear` — `/clear` empties `lines`); otherwise `hist.push`, `SendText` in a `tea.Cmd` → `lineSent`/`sendFailed`, and the line echoed to the pane as `[me] …` immediately. `up`/`down` → `hist.up/down` into `in.SetValue`. Everything else → `in.Update`, and `pgup/pgdown` plus wheel → `pane.Update`.
  7. `model.go`: `screenConversation` dispatch; `callArrived` → `callBar.incoming` + `tick()`; `y`/`n`/`enter` on the bar → `takeCall`/`declineCall`; `callAnswered` → `newConversation` + `runSession`; `peerSaid` → `say`; `peerLeft` → `say(name left / the conversation ended: reason)`, `leaving = true`, and on the next Enter or `/quit` back to the menu with the typed line still in the input; `actCall` → `dial(...)` and `callBar.outgoing = name` with `Enter to give up` handled by cancelling the dial's context.
  8. `run.go`: `Init` returns `acceptLoop(...)`; `Send` is `p.Send` captured after `NewProgram` — build the model with a `send` that closes over a pointer set once the program exists.

- [ ] **Step 4: Run all tests with `-race`, `make lint`, then two real processes in two terminals (`make run-a`, `make run-b`): call, answer, type on one side while the other sends — the input line must not move.**
- [ ] **Step 5: Checkpoint** — report `…-full-screen-3-conversation.md`; ready for `commit kon`.

---

### Task 4: Files, and the remaining screens

**Files:**
- Modify: `internal/ui/adapter.go` (implement `session.FileHandler`), `internal/ui/msgs.go`, `internal/ui/screen_conversation.go` (`/files`, `/send`, `/accept`, `/reject`, `y`/`n` shorthand), `internal/ui/model.go` (dispatch)
- Create: `internal/ui/screen_contacts.go`, `internal/ui/screen_settings.go`, `internal/ui/screen_setup.go`, `internal/ui/screen_help.go`, `internal/ui/screen_reset.go`
- Modify: `cmd/homa/boot.go` (first run through the setup screen: `ui.RunSetup(ctx) (*config.Config, error)` — a program of its own that shows the banner and the two questions and returns when saved; the banner rows are `bannerArt` from `const.go`, drawn through `styles`)
- Test: `internal/ui/adapter_test.go`, `internal/ui/screen_contacts_test.go`, `internal/ui/screen_settings_test.go`, `internal/ui/screen_setup_test.go`, and additions to `screen_conversation_test.go`

**Interfaces:**
- Consumes: `session.FileHandler` (`OnFileOffer(name, size) (dir string, accept bool, reason string)`, `OnFileProgress(name, received, total)`, `OnFileDone(name, path)`, `OnFileError(name, err)`), `Session.SendFile(...)` (signature in `session/files.go:90`), `listing`/`readDir`/`resolveDir`/`resolveSend` from `files.go` (move `resolveSend`/`resolveDir` off `*App` onto plain functions taking the listing), `config.Config.Validate/Save/EnsureDownloadDir`, `contacts.Book.Add/Rename/Remove`, `peer.RemoveIdentity`, `config.Remove`, `contacts.Remove`.
- Produces (`msgs.go`):
  ```go
  type fileOffered struct{ name string; size int64; reply chan<- bool }
  type fileProgress struct{ name string; pct int }
  type fileDone struct{ name, path string }
  type fileFailed struct{ name string; err error }
  type sending struct{ name string; pct int }     // outgoing progress, from the SendFile cmd
  type sent struct{ name string }
  ```
  The adapter's `OnFileOffer` sends `fileOffered` with a buffered channel and blocks on it, exactly as today's handler blocks on `pendingOffer.reply`; `Update` answers on `y`/`n`/`/accept`/`/reject`; the timeout `offerAnswerTimeout` stays inside `OnFileOffer`. Progress is reported every `progressStep` percent, the same dedup as today's `lastStep`, kept in the adapter.
- Each small screen is a sub-model with `view(st) string` and `update(msg) (sub, tea.Cmd, done bool)`; the settings and setup screens hold two `textinput`s walked with Enter and validated with `config.Validate` exactly as `askUntilValid` does today, the reset screen holds one input that must read `reset`, the contacts screen is a list with a cursor plus `b`/`q`, the contact screen the four actions. Every wording is the version-1 text.

- [ ] **Step 1: Failing tests** — for each screen, a `view` test comparing stripped text against the version-1 wording, and an `update` test walking the happy path (settings: two Enters save; reset: typing `reset` runs the three removals — behind an interface so the test does not delete the developer's identity: `type wiper interface{ removeIdentity, removeBook, removeConfig func() error }` on `Deps`, default wired to the real functions in `bootstrap`); for the adapter, `TestAnOfferIsAnsweredFromUpdate` mirroring today's `handler_test.go` but asserting on the `fileOffered` message.
- [ ] **Step 2: Run, expect failures.**
- [ ] **Step 3: Implement**, one file at a time, screens in the order contacts → contact → add → settings → setup → reset → help, then the file commands, then `boot.go`.
- [ ] **Step 4: Tests, lint, and a real two-process run: `/files`, `/send 3`, `y` on the other side, the progress line, `saved to`.**
- [ ] **Step 5: Checkpoint** — report `…-full-screen-4-screens-files.md`; ready for `commit kon`.

---

### Task 5: Delete the line machinery; `test/live` reads a screen

**Files:**
- Delete: `internal/ui/ui.go`, `prompt.go`, `countdown.go`, `style.go`, `theme.go`, `welcome.go`, `menu.go`, `chat.go`, `handler.go`, `contacts.go`, `setup.go`, `help.go`, `ui_test.go`, `theme_test.go`, `welcome_test.go`, `menu_test.go`, `handler_test.go`, `contacts_test.go` (their surviving assertions have been rewritten in the new tests by Tasks 2–4; check each test name against the new files before deleting)
- Modify: `internal/ui/const.go` (remove `clearLine`, `clearScreen`, `esc`, `bel`, the four `color*` escapes, `bannerSplit`/`bannerIndent` if unused; keep `bannerArt`, `bannerPlain`, timeouts, limits, `wordBack`/`wordQuit`, `sep*`), `internal/ui/run.go` (replace the `styleFor` call with `unicodeLocale(os.Getenv)` — the three-variable check from `styleFor`, kept as a function with its test moved from `welcome_test.go`)
- Create: `test/live/screen.go` — a minimal VT interpreter: a grid of `rows×cols` runes; handles `\r`, `\n`, `\x1b[K`, `\x1b[2K`, `\x1b[<n>A/B/C/D`, `\x1b[<row>;<col>H`, `\x1b[2J`, SGR ignored; `func (s *screen) text() string`, `func (s *screen) contains(needle string) bool`
- Modify: `test/live/live_test.go` — `await` feeds the pty bytes into the screen and looks for the needle in `screen.text()`; the 37 needles are re-checked against the new wording (`What now?` stays; `> choice` is gone; `Enter to give up` stays in the bar)
- Test: `test/live/screen_test.go` (feed a scripted byte string, assert the grid), and `make test-live` on a machine with network

- [ ] **Step 1: Write `screen_test.go` first**, with a byte string that moves the cursor home, writes two lines, erases one, and asserts `text()`.
- [ ] **Step 2: Implement `screen.go`; run its test.**
- [ ] **Step 3: Rewrite `await`; run `make test-live`** (needs a relay: this is the only step that touches the network, and it is the user's machine that runs it).
- [ ] **Step 4: Delete the old files; `go build ./...`, `make test`, `make lint`. `grep -rn "clearLine\|Prompt(\|ErasePrompt" internal/` must be empty.**
- [ ] **Step 5: Checkpoint** — report `…-full-screen-5-cleanup-live.md`; ready for `commit kon`.

---

### Task 6: Documentation and the transcripts

**Files:**
- Modify: `README.md` (both session blocks recaptured from the finished interface through the pty script used for v0.1.0, rendered through `test/live`'s screen interpreter; the `-no-color` line stays), `docs/status.md` (the two warts removed; "a full-screen interface" under working today), `docs/architecture.md` (the `ui` package paragraph: model, messages, adapters), `docs/todo.md` (the item removed; the "not to be started" line removed), `docs/roadmap.md` (the full-screen section becomes history: what was built and the `x/term` road not taken, kept for the record), `docs/decisions.md` (one entry: why bubbletea over `x/term`, why no alternate screen, why a pipe is an error)

- [ ] **Step 1: Recapture; Step 2: edit each doc; Step 3: `make lint`, `make test`; Step 4: Checkpoint** — report `…-full-screen-6-docs.md`; ready for `commit kon`. Then the user decides on a tag (`v0.2.0`), which is a push.

---

## Self-review against the spec

- **Decisions 1–5:** Task 0 (v2 modules, terminal check), Task 3 (one conversation: the model holds exactly one), Task 2 (`AltScreen=false`, pipe refused in `Run`), every task's file list (`ui` + `cmd/homa` + `go.mod`).
- **Screens:** menu (2), incoming bar (3), conversation (3), calling bar (3), files (4), contacts/contact/add/settings/setup/reset/help (4), banner (4, in setup and the menu status).
- **Model and messages table:** `msgs.go` across Tasks 2–4 defines every row of the spec's table; `tick` is `tickMsg`.
- **What goes:** Task 5. **What stays:** Tasks 3–4 name each moved function.
- **Testing:** pure `view`/`update` tests per screen, `flow_test.go` on teatest (3), `test/live` on a grid (5), README recaptured (6). The color-equals-plain invariant: add `TestColorChangesNothingButColor` back in Task 2's `model_test.go`, rendering the menu with `newStyles(true)` twice through a program with `colorprofile.TrueColor` and `colorprofile.Ascii` — lipgloss renders identically apart from escapes.
- **Order of work:** the spec's six steps are Tasks 1–6 with Task 0 in front.
- **Placeholders:** none; `flow_test.go`'s numbered shape names every real call. **Type consistency:** `call`, `Deps`, `styles`, `menuAction`, `conversation`, `history`, `callBar` are defined once and used by name.
