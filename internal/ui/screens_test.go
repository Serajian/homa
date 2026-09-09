package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/Serajian/homa/internal/config"
	"github.com/Serajian/homa/internal/update"
)

func key(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	if k == "" {
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	r, _ := utf8.DecodeRuneInString(k)
	return tea.KeyPressMsg{Code: r, Text: k}
}

// steer presses keys on a model; a multi-character string is typed.
func steer(m model, keys ...string) model {
	for _, k := range keys {
		if len([]rune(k)) > 1 && k != "enter" && k != "esc" && k != "up" && k != "down" &&
			k != "backspace" {
			for _, r := range k {
				next, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				m = next.(model)
			}
			continue
		}
		next, _ := m.Update(key(k))
		m = next.(model)
	}
	return m
}

// realAddr is a real homa address (from a captured test session), for
// the one form that checks its input against peer.ValidAddr.
const realAddr = "tcpGFwWCADq1uyijFwTpeVLHaUsFHGkkAj9iKYaoF1kNB1zTiTHmFrWCDCSWppTYNmKR1MMx6J06tTANJ0r9i54825yVAk1KsIaWFxWCDVqyaa8w_4o9G7Ejhg4gjXFcDssK4aiXE6EgnbybaKqmFygaFhToGjYWhudGMzMDNhLmlwbi5kZXZhNG8xODUuMTc4LjIwMi4xOTdhNnEyYTAwOmRkODA6MjA6OjIwNw"

// sized is a model that has been told the terminal's size, as the program
// tells it before the first draw; without it the frame is three lines.
func sized(m model) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	return next.(model)
}

// sandboxHome points the config and contacts files at a temp dir, so a
// screen that saves does not touch the developer's.
func sandboxHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
}

func TestTheContactsScreenOpensOneAndComesBack(t *testing.T) {
	m := sized(newModel(t.Context(), testDeps(t, "alice", "bob"), newStyles(true)))
	m = steer(m, "b")
	if m.screen != screenContacts {
		t.Fatalf("b did not open the contacts screen: %v", m.screen)
	}
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "CONTACTS") || !strings.Contains(view, "1  alice") ||
		!strings.Contains(view, "2  bob") {
		t.Errorf("contacts screen:\n%s", view)
	}

	m = steer(m, "down", "enter")
	if m.screen != screenContact || m.contact.Name != "bob" {
		t.Fatalf("Enter on the second row opened %q on screen %v", m.contact.Name, m.screen)
	}
	view = stripANSI(m.View().Content)
	for _, want := range []string{"c  call bob", "r  rename", "a  show their address", "f  forget", "b  back"} {
		if !strings.Contains(view, want) {
			t.Errorf("contact screen lacks %q:\n%s", want, view)
		}
	}

	m = steer(m, "b")
	if m.screen != screenContacts {
		t.Error("b did not go back to the contacts")
	}
	m = steer(m, "b")
	if m.screen != screenMenu {
		t.Error("b did not go back to the menu")
	}
}

func TestAddingAContactThroughTheForm(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "n")
	if m.screen != screenForm {
		t.Fatal("n did not open the form")
	}
	m = steer(m, "carol", "enter", "nonsense", "enter")
	if m.screen != screenForm ||
		!strings.Contains(stripANSI(m.View().Content), "does not look like a homa address") {
		t.Fatalf("a bad address was not refused on the form:\n%s", stripANSI(m.View().Content))
	}
	addr := realAddr
	// clear the field, then the real address
	for range len("nonsense") {
		m = steer(m, "backspace")
	}
	m = steer(m, addr, "enter")
	if m.screen != screenMenu {
		t.Fatalf("the form did not finish; screen %v", m.screen)
	}
	if _, err := m.deps.Book.ByName("carol"); err != nil {
		t.Errorf("carol was not added: %v", err)
	}
	if !strings.Contains(stripANSI(m.View().Content), "carol added.") {
		t.Error("no confirmation")
	}
}

func TestRenamingAndForgettingAContact(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t, "alice"), newStyles(true)))
	m = steer(m, "b", "enter", "r")
	if m.screen != screenForm {
		t.Fatal("r did not open the rename form")
	}
	m = steer(m, "ali", "enter")
	if m.screen != screenContact || m.contact.Name != "ali" {
		t.Fatalf("after rename: screen %v, contact %q", m.screen, m.contact.Name)
	}
	if _, err := m.deps.Book.ByName("ali"); err != nil {
		t.Errorf("the book was not renamed: %v", err)
	}

	m = steer(m, "f", "enter") // Enter alone is "cancel"
	if m.screen != screenContact ||
		!strings.Contains(stripANSI(m.View().Content), "ali is still there.") {
		t.Fatalf("Enter did not cancel the forget:\n%s", stripANSI(m.View().Content))
	}
	m = steer(m, "f", "forget", "enter")
	if m.screen != screenMenu {
		t.Fatalf("after forgetting the only contact the screen is %v, want the menu", m.screen)
	}
	if _, err := m.deps.Book.ByName("ali"); err == nil {
		t.Error("ali was not forgotten")
	}
}

func TestSettingsAreSavedAndReplaced(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "s", "zed", "enter", "enter", "backspace", "n", "enter")
	if m.screen != screenMenu {
		t.Fatalf("screen %v after the settings", m.screen)
	}
	if m.deps.Cfg.Nick != "zed" || m.deps.Cfg.Bell {
		t.Errorf("settings after the form: %+v", m.deps.Cfg)
	}
	saved, err := config.Load()
	if err != nil || saved.Nick != "zed" || saved.Bell {
		t.Errorf("saved settings: %+v, %v", saved, err)
	}
}

// The third question takes y or n and nothing else; the bell is on by
// default, so Enter past it leaves it on.
func TestTheBellQuestionTakesOnlyYesOrNo(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "s", "enter", "enter", "backspace", "maybe", "enter")
	if m.screen != screenForm {
		t.Fatalf("\"maybe\" was taken as an answer; screen %v", m.screen)
	}
	m = steer(m, "backspace", "backspace", "backspace", "backspace", "backspace", "yes", "enter")
	if m.screen != screenMenu || !m.deps.Cfg.Bell {
		t.Errorf("screen %v, settings %+v", m.screen, m.deps.Cfg)
	}
}

// ringing collects what a command sends, and reports whether the bell is
// among it.
func ringing(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case tea.RawMsg:
		return msg.Msg == bell
	case tea.BatchMsg:
		for _, c := range msg {
			if ringing(c) {
				return true
			}
		}
	}
	return false
}

// callAnswered starts the conversation on both sides; the bell is for the
// side that dialed and waited, not the one that just pressed y.
func TestATakenCallRingsForTheCallerOnly(t *testing.T) {
	t.Parallel()

	caller := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	caller.bar = callBar{outgoing: "alice"}
	if !ringing(caller.ringIfWaiting()) {
		t.Error("the caller was not rung")
	}
	answerer := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	answerer.bar = callBar{incoming: testLine("alice", true)}
	if ringing(answerer.ringIfWaiting()) {
		t.Error("the answerer was rung for their own y")
	}
}

// A call that waits rings again every callRingEvery, and not between.
func TestAWaitingCallKeepsRinging(t *testing.T) {
	t.Parallel()

	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	next, _ := m.Update(callArrived{l: testLine("~bob", false)})
	m = next.(model)
	arrived := m.bar.lastRing

	next, cmd := m.Update(tickMsg(arrived.Add(time.Second)))
	m = next.(model)
	if ringing(cmd) {
		t.Error("rang a second after arriving")
	}
	next, cmd = m.Update(tickMsg(arrived.Add(callRingEvery)))
	m = next.(model)
	if !ringing(cmd) {
		t.Error("did not ring again when the time came")
	}
	if _, cmd = m.Update(tickMsg(arrived.Add(callRingEvery + time.Second))); ringing(cmd) {
		t.Error("rang twice for one interval")
	}

	// Answered or gone, the ringing stops with the box.
	m.bar.clear()
	if _, cmd = m.Update(tickMsg(arrived.Add(3 * callRingEvery))); ringing(cmd) {
		t.Error("rang for a call no longer on the screen")
	}
}

// The outcome of your own call rings, because you may have looked away
// while it waited; stopping it yourself does not.
func TestTheOutcomeOfYourCallRings(t *testing.T) {
	t.Parallel()

	waiting := func() model {
		m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
		m.bar = callBar{outgoing: "alice"}
		return m
	}
	if _, cmd := waiting().Update(callRefused{name: "alice", format: "%s is not taking calls"}); !ringing(
		cmd,
	) {
		t.Error("a refusal did not ring")
	}
	if _, cmd := waiting().Update(callFailed{name: "alice", err: errNoOffer}); !ringing(cmd) {
		t.Error("a failure did not ring")
	}
	if _, cmd := waiting().Update(callRefused{name: "alice", format: "you stopped calling %s.", quiet: true}); ringing(
		cmd,
	) {
		t.Error("stopping your own call rang")
	}
}

func TestACallRingsTheBellWhenTheSettingSaysSo(t *testing.T) {
	t.Parallel()

	for _, on := range []bool{true, false} {
		deps := testDeps(t)
		deps.Cfg.Bell = on
		m := sized(newModel(t.Context(), deps, newStyles(true)))
		_, cmd := m.Update(callArrived{l: testLine("~bob", false)})
		if ringing(cmd) != on {
			t.Errorf("bell %v: a call rang %v", on, ringing(cmd))
		}
		if _, cmd = m.Update(fileProgress{name: "x", pct: 10}); ringing(cmd) {
			t.Error("progress rang")
		}
		if _, cmd = m.Update(sent{}); ringing(cmd) != on {
			t.Errorf("bell %v: a file sent rang %v", on, ringing(cmd))
		}
		if _, cmd = m.Update(fileFailed{name: "x", err: errNoOffer}); ringing(cmd) != on {
			t.Errorf("bell %v: a failed file rang %v", on, ringing(cmd))
		}
	}
}

func TestResetNeedsTheWordAndThenQuits(t *testing.T) {
	deps := testDeps(t)
	wiped := false
	deps.Reset = func() []string { wiped = true; return nil }
	m := sized(newModel(t.Context(), deps, newStyles(true)))

	m = steer(m, "r", "enter")
	if wiped || m.screen != screenMenu ||
		!strings.Contains(stripANSI(m.View().Content), "nothing was deleted.") {
		t.Fatalf("Enter alone reset, or did not say so; wiped=%v screen=%v", wiped, m.screen)
	}

	m = steer(m, "r", "reset")
	next, cmd := m.Update(key("enter"))
	if !wiped {
		t.Error("the word did not reset")
	}
	if cmd == nil {
		t.Fatal("no quit after the reset")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("the command after a reset is not quit")
	}
	if !strings.Contains(stripANSI(next.(model).View().Content), "are gone") {
		t.Error("the reset did not say what happened")
	}
}

func TestHelpAndAddressArePagesAnyKeyLeaves(t *testing.T) {
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "h")
	if m.screen != screenPage ||
		!strings.Contains(stripANSI(m.View().Content), "homa connects two people directly") {
		t.Fatalf("h did not show the help:\n%s", stripANSI(m.View().Content))
	}
	m = steer(m, "x")
	if m.screen != screenMenu {
		t.Error("a key did not leave the page")
	}
}

func TestTheFirstRunAsksTwoQuestionsUnderTheBanner(t *testing.T) {
	sandboxHome(t)
	sm := setupModel{st: newStyles(true), form: settingsForm("Welcome", config.Default(), true)}
	next, _ := sm.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	view := stripANSI(next.(setupModel).View().Content)
	for _, want := range []string{"█", "H O M A", "This is the first run, so two questions.", "The name shown beside your messages"} {
		if !strings.Contains(view, want) {
			t.Errorf("first run lacks %q:\n%s", want, view)
		}
	}
	sm = next.(setupModel)
	for _, r := range "dana" {
		n, _ := sm.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		sm = n.(setupModel)
	}
	n, _ := sm.Update(key("enter"))
	n, cmd := n.(setupModel).Update(key("enter"))
	sm = n.(setupModel)
	if cmd == nil || sm.cfg == nil || sm.cfg.Nick != "dana" {
		t.Fatalf("the first run did not finish: cfg=%+v err=%v", sm.cfg, sm.err)
	}
	if _, err := os.Stat(config.Path()); err != nil && !strings.Contains(config.Path(), "~") {
		t.Errorf("settings not saved at %s: %v", config.Path(), err)
	}
}

// An address is shown in rows of one width, never broken at a hyphen the
// way prose wraps, so it copies as a block.
func TestAnAddressIsShownInRowsOfOneWidth(t *testing.T) {
	t.Parallel()

	addr := strings.Repeat("abcdefghi-", 23) + "xy"
	got := strings.Split(blockRows(addr, 96), "\n")
	if len(got) != 3 || len(got[0]) != 96 || len(got[1]) != 96 || len(got[2]) != 40 {
		t.Fatalf("rows of %v", func() []int {
			var n []int
			for _, r := range got {
				n = append(n, len(r))
			}
			return n
		}())
	}
	if strings.Join(got, "") != addr {
		t.Error("the rows do not join back into the address")
	}
}

// The bug a person found first: the address pasted into "add a contact"
// went nowhere. A paste must reach the form through the model.
func TestAPastedAddressIsTakenByTheContactForm(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "n", "vaio", "enter")
	next, _ := m.Update(tea.PasteMsg{Content: realAddr})
	m = next.(model)
	m = steer(m, "enter")
	if m.screen != screenMenu {
		t.Fatalf(
			"screen %v after pasting the address; form says %q",
			m.screen,
			m.form.fields[1].err,
		)
	}
	if _, err := m.deps.Book.ByName("vaio"); err != nil {
		t.Error("vaio was not added")
	}
}

func TestAPasteReachesTheFirstRun(t *testing.T) {
	sandboxHome(t)
	sm := setupModel{st: newStyles(true), form: settingsForm("Welcome", config.Default(), true)}
	next, _ := sm.Update(tea.PasteMsg{Content: "dana"})
	sm = next.(setupModel)
	if sm.form.answer(0) != "dana" {
		t.Errorf("first field after a paste: %q", sm.form.answer(0))
	}
}

// u asks GitHub, off the update loop, and shows a page; an unreachable
// GitHub is a notice, not a page. Pointed at a server of the test's own.
func TestCheckingForUpdatesShowsAPageOrSaysWhyNot(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://example.invalid/r"}`))
	}))
	t.Cleanup(srv.Close)

	deps := testDeps(t)
	deps.Version = "v0.2.1"
	deps.Update = update.Checker{URL: srv.URL}
	m := sized(newModel(t.Context(), deps, newStyles(true)))
	next, cmd := m.Update(key("u"))
	m = next.(model)
	if cmd == nil || !strings.Contains(m.notice, "asking GitHub") {
		t.Fatalf("u did not start a check; notice %q", m.notice)
	}
	next, _ = m.Update(cmd())
	m = next.(model)
	body := stripANSI(m.View().Content)
	if m.screen != screenPage || !strings.Contains(body, "v9.9.9 is out; you have v0.2.1") ||
		!strings.Contains(body, "to upgrade:") {
		t.Errorf("screen %v:\n%s", m.screen, body)
	}

	srv.Close()
	m = sized(newModel(t.Context(), deps, newStyles(true)))
	next, cmd = m.Update(key("u"))
	next, _ = next.(model).Update(cmd())
	m = next.(model)
	if m.screen != screenMenu || !m.warn || !strings.Contains(m.notice, "could not check") {
		t.Errorf("an unreachable server: screen %v, notice %q", m.screen, m.notice)
	}
}

// The address as a person copies it off the a page: rows of equal width,
// each indented, joined by line breaks. It must be taken as it is.
func TestAWrappedAddressPastedIsTaken(t *testing.T) {
	sandboxHome(t)
	m := sized(newModel(t.Context(), testDeps(t), newStyles(true)))
	m = steer(m, "n", "vaio", "enter")
	copied := "  " + strings.ReplaceAll(blockRows(realAddr, 60), "\n", "\n  ")
	if !strings.Contains(copied, "\n") {
		t.Fatal("the test address did not wrap")
	}
	next, _ := m.Update(tea.PasteMsg{Content: copied})
	m = steer(next.(model), "enter")
	if m.screen != screenMenu {
		t.Fatalf("screen %v; the form says %q", m.screen, m.form.fields[1].err)
	}
	c, err := m.deps.Book.ByName("vaio")
	if err != nil || c.Addr != realAddr {
		t.Errorf("saved %q, %v", c.Addr, err)
	}
}
