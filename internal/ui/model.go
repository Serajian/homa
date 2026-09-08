package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Serajian/homa/internal/contacts"
	"github.com/Serajian/homa/internal/peer"
)

// screen is which of homa's screens fills the body. One at a time; the
// call bar is not a screen but a line under whichever one is up.
type screen int

const (
	screenMenu screen = iota
	screenConversation
	screenContacts
	screenContact
	screenForm
	screenPage
)

// formKind is which question a form on the screen is asking, so its
// answers go to the right place.
type formKind int

const (
	formAdd formKind = iota
	formRename
	formForget
	formSettings
	formReset
)

// model is the whole interface: what is on the screen, and enough of what
// homa knows to draw it. Every Update runs on one goroutine, so nothing in
// here is guarded; what arrives from elsewhere arrives as a message.
type model struct {
	deps Deps
	st   *styles

	// ctx is the program's; a call being placed gets a child of it so the
	// person can give up on that one call. send hands a message to the
	// program from any goroutine; the adapters and commands use it.
	ctx  context.Context
	send func(tea.Msg)

	width, height int
	screen        screen
	back          screen // where a form or a page returns to

	menu     menuModel
	bar      callBar
	conv     *conversation
	contacts contactsModel
	contact  contacts.Contact // the one open on screenContact
	form     *form
	kind     formKind
	page     *page

	// notice is one line under the body: a hint after a wrong key, why a
	// call did not go through. The next key clears it. warn draws it in
	// yellow, for something that went wrong.
	notice string
	warn   bool
}

func newModel(ctx context.Context, deps Deps, st *styles) model {
	return model{
		ctx:  ctx,
		deps: deps,
		st:   st,
		menu: newMenu(deps.Book),
		send: func(tea.Msg) {},
	}
}

// Init starts greeting callers, for the life of the program. The screen
// was cleared by Run before the program began; see clearScreen.
func (m model) Init() tea.Cmd {
	if m.deps.Listener == nil {
		return nil // tests, and nothing to listen on
	}
	return acceptLoop(m.ctx, m.deps, m.send)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.conv != nil {
			m.conv.resize(m.width, m.height)
		}
		return m, nil

	case noticeMsg:
		m.say(string(msg), false)
		return m, nil
	case warnMsg:
		m.say(string(msg), true)
		return m, nil

	case tickMsg:
		if m.bar.showing() && m.screen != screenConversation {
			return m, tea.Batch(tick(), m.ringAgain(time.Time(msg)))
		}
		return m, nil

	case callArrived:
		next, cmd := m.callArrived(msg.l)
		return next, tea.Batch(cmd, m.ring())
	case callGone:
		if m.bar.incoming == msg.l {
			m.bar.clear()
			m.say(sayCall(m.st, "the call from %s ran out of time while you were busy.", msg.l.name), false)
		}
		return m, nil
	case callRefused:
		var ring tea.Cmd
		if !msg.quiet {
			ring = m.ringIfWaiting()
		}
		m.bar.clear()
		m.say(sayCall(m.st, msg.format, msg.name), false)
		return m, ring
	case callFailed:
		ring := m.ringIfWaiting()
		m.bar.clear()
		m.say(sayCall(m.st, "could not reach %s: ", msg.name)+reason(msg.err), true)
		return m, ring
	case callAnswered:
		ring := m.ringIfWaiting()
		next, cmd := m.startConversation(msg.l)
		return next, tea.Batch(cmd, ring)

	case peerSaid:
		if m.conv != nil {
			m.conv.msg(m.st, m.conv.l.name, msg.text, false)
		}
		return m, m.ring()
	case peerLeft:
		next, cmd := m.peerLeft(msg.err)
		return next, tea.Batch(cmd, m.ring())
	case sendFailed:
		if m.conv != nil && !m.conv.ended {
			m.conv.alert(m.st, "could not send: "+reason(msg.err))
			m.conv.ended = true
		}
		return m, nil

	case fileOffered, fileDone, fileFailed, sent:
		// sent is the one thing of your own doing that rings: a large file
		// takes minutes, and nobody watches a progress bar for minutes.
		m.fileEvent(msg)
		return m, m.ring()
	case offerTimedOut, fileProgress, sending, sendFileFailed:
		m.fileEvent(msg)
		return m, nil

	case tea.MouseWheelMsg:
		if m.conv != nil {
			cmd, _ := m.conv.update(m.st, msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		m.notice, m.warn = "", false
		if msg.String() == keyQuit {
			if m.conv != nil && !m.conv.ended {
				// A goodbye the far side knows how to read, then out.
				return m, hangUpAndQuit(m.conv.l)
			}
			return m, tea.Quit
		}
		switch m.screen {
		case screenConversation:
			return m.updateConversation(msg)
		case screenContacts:
			return m.updateContacts(msg)
		case screenContact:
			return m.updateContact(msg)
		case screenForm:
			return m.updateForm(msg)
		case screenPage:
			m.screen = m.back
			return m, nil
		default:
			return m.updateMenu(msg)
		}
	}
	return m, nil
}

func (m *model) say(text string, warn bool) { m.notice, m.warn = text, warn }

// fileEvent is anything about a file, said in the conversation's pane.
func (m *model) fileEvent(msg tea.Msg) {
	c, st := m.conv, m.st
	if c == nil {
		return
	}
	switch msg := msg.(type) {
	case fileOffered:
		c.offered(st, msg)
	case offerTimedOut:
		c.offer = nil
		c.alert(st, "the offer of "+msg.name+" timed out")
		c.note(st, st.dim.Render(fmt.Sprintf("they can offer it again; y or n answers it within %s", offerAnswerTimeout)))
	case fileProgress:
		c.note(st, st.dim.Render("receiving ")+st.them.Render(msg.name)+"  "+progressBar(st, msg.pct)+st.dim.Render(fmt.Sprintf("  %d%%", msg.pct)))
	case fileDone:
		c.note(st, st.them.Render(msg.name)+st.dim.Render(" saved to "+msg.path))
	case fileFailed:
		c.alert(st, quote(msg.name)+": "+reason(msg.err))
	case sending:
		c.note(st, st.dim.Render("sending ")+progressBar(st, msg.pct)+st.dim.Render(fmt.Sprintf("  %d%%", msg.pct)))
	case sent:
		c.note(st, st.dim.Render("sent."))
	case sendFileFailed:
		c.alert(st, reason(msg.err))
	}
}

// callArrived parks a caller on the bar. A second caller while one waits
// is turned away as busy; a caller during a conversation waits unseen
// until it ends, and the bar's own timer hangs up if that is too long.
// ring is the bell, when the setting says so: for what arrives from the
// far side, never for what you did yourself. tea.Raw sends the byte down
// the program's own output, between two frames and never inside one.
func (m model) ring() tea.Cmd {
	if m.deps.Cfg == nil || !m.deps.Cfg.Bell {
		return nil
	}
	return tea.Raw(bell)
}

// ringAgain is the bell for a call still waiting on the screen: once every
// callRingEvery since it last rang, the way a phone keeps ringing until it
// is picked up. now comes from the tick so a test can move time.
func (m *model) ringAgain(now time.Time) tea.Cmd {
	if m.bar.incoming == nil || now.Sub(m.bar.lastRing) < callRingEvery {
		return nil
	}
	m.bar.lastRing = now
	return m.ring()
}

// ringIfWaiting is the bell for a call taken. The same message starts the
// conversation on both sides; only the side that dialed and waited is
// told, because the side that pressed y is looking at the screen.
func (m model) ringIfWaiting() tea.Cmd {
	if m.bar.outgoing == "" {
		return nil
	}
	return m.ring()
}

func (m model) callArrived(l *line) (tea.Model, tea.Cmd) {
	if m.bar.showing() {
		return m, turnAway(l)
	}
	m.bar = callBar{incoming: l, deadline: l.deadline, lastRing: time.Now()}
	if m.screen == screenConversation {
		return m, nil
	}
	return m, tick()
}

// barKey is a key while a call is on the bar. A waiting caller has the
// keyboard: y lets them in, n does not, and nothing else does anything, as
// the question in version 1 did. An outgoing call takes only Enter, to
// give up. handled is false when no call is on the bar.
func (m model) barKey(k string) (tea.Model, tea.Cmd, bool) {
	if l := m.bar.incoming; l != nil {
		switch k {
		case "y":
			if !l.claim() {
				m.bar.clear()
				return m, nil, true
			}
			return m, takeCall(l), true
		case "n", keyEnter:
			if !l.claim() {
				m.bar.clear()
				return m, nil, true
			}
			return m, declineCall(
				l,
				"they are not taking calls right now",
				"the call from %s was not taken.",
			), true
		default:
			m.say("y takes the call, n does not", false)
			return m, nil, true
		}
	}
	if m.bar.outgoing != "" {
		if k == keyEnter {
			m.bar.cancel() // the dial reports back that we stopped calling
		}
		return m, nil, true
	}
	return m, nil, false
}

// updateMenu is a key on the menu, or on the call bar over it.
func (m model) updateMenu(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if next, cmd, handled := m.barKey(k); handled {
		return next, cmd
	}

	act, ok := m.menu.key(k)
	if !ok {
		m.say(
			"that is not one of the choices"+m.st.sep()+"press one of the keys on the left, or h for help",
			false,
		)
		return m, nil
	}

	switch act {
	case actQuit:
		return m, tea.Quit
	case actCall:
		return m.placeCall(m.menu.chosen())
	case actAdd:
		return m.openForm(formAdd, newForm("add a contact",
			field{label: "A name for them"},
			field{label: "Their address", check: checkAddr},
		), screenMenu), nil
	case actContacts:
		m.contacts = newContacts(m.deps.Book)
		if len(m.contacts.list) == 0 {
			m.say("no contacts yet. n at the menu adds one.", false)
			return m, nil
		}
		m.screen = screenContacts
		return m, nil
	case actAddress:
		m.page = &page{
			title: "your address",
			body:  addressText(m.st, m.deps.Listener.Addr(), m.width-4),
			back:  screenMenu,
		}
		m.screen = screenPage
		return m, nil
	case actSettings:
		return m.openForm(
			formSettings,
			settingsForm("settings", m.deps.Cfg, false),
			screenMenu,
		), nil
	case actHelp:
		m.page = &page{title: "help", body: helpText, back: screenMenu}
		m.screen = screenPage
		return m, nil
	case actReset:
		f := newForm("start over",
			field{label: "type the word reset to confirm", def: "cancel", word: "reset"})
		f.warn = []string{
			"This deletes your identity, your address book and your settings.",
			"Your address changes, and everyone who saved the old one can no",
			"longer reach you.",
		}
		return m.openForm(formReset, f, screenMenu), nil
	default:
		// actNone, actClear: the frame is redrawn whole on every update;
		// there is nothing a clear could clear that the next draw does not.
		return m, nil
	}
}

// checkAddr is the address field's check: the same test the old screen
// made, with its hint in the same breath.
func checkAddr(s string) error {
	if !peer.ValidAddr(s) {
		return fmt.Errorf(
			"that does not look like a homa address; it is the long line a) shows on their side; paste all of it",
		)
	}
	return nil
}

func (m model) openForm(kind formKind, f *form, back screen) model {
	m.form, m.kind, m.back = f, kind, back
	m.screen = screenForm
	return m
}

// updateForm is a key on a form: the form takes it, and when it is done or
// backed out of, the answers go where the kind says.
func (m model) updateForm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	done, cancel := m.form.update(m.st, msg)
	switch {
	case cancel:
		m.screen = m.back
		switch m.kind {
		case formForget:
			m.say(sayCall(m.st, "%s is still there.", m.contact.Name), false)
		case formReset:
			m.say("nothing was deleted.", false)
		}
		return m, nil
	case !done:
		return m, nil
	}

	answers := m.form.answers()
	m.screen = m.back
	switch m.kind {
	case formAdd:
		return m.addContact(answers[0], answers[1])
	case formRename:
		return m.renameContact(answers[0])
	case formForget:
		return m.forgetContact()
	case formSettings:
		cfg, err := applySettings(m.deps.Cfg, answers)
		if err != nil {
			m.say(err.Error(), true)
			return m, nil
		}
		m.deps.Cfg = cfg
		m.say("Saved.", false)
		return m, nil
	case formReset:
		return m.reset()
	}
	return m, nil
}

func (m model) addContact(name, addr string) (tea.Model, tea.Cmd) {
	if err := m.deps.Book.Add(contacts.Contact{Name: name, Addr: addr}); err != nil {
		m.say(err.Error(), true)
		return m, nil
	}
	if err := m.deps.Book.Save(); err != nil {
		m.say("could not save the address book: "+err.Error(), true)
		return m, nil
	}
	m.menu = newMenu(m.deps.Book)
	m.say(sayCall(m.st, "%s added.", name), false)
	return m, nil
}

// renameContact changes the local name. The name is this machine's, not
// the peer's: somebody calling themselves "babak" is a good reason to
// rename "BB", but it stays a decision the person makes.
func (m model) renameContact(name string) (tea.Model, tea.Cmd) {
	if name == m.contact.Name {
		return m, nil
	}
	if err := m.deps.Book.Rename(m.contact.Name, name); err != nil {
		m.say(reason(err), true)
		return m, nil
	}
	if err := m.deps.Book.Save(); err != nil {
		m.say("could not save the address book: "+err.Error(), true)
	}
	m.say(
		m.st.peer(m.contact.Name)+m.st.dim.Render(" is now ")+m.st.peer(name)+m.st.dim.Render("."),
		false,
	)
	m.contact.Name = name
	m.menu = newMenu(m.deps.Book)
	return m, nil
}

func (m model) forgetContact() (tea.Model, tea.Cmd) {
	if err := m.deps.Book.Remove(m.contact.Name); err != nil {
		m.say(reason(err), true)
		return m, nil
	}
	if err := m.deps.Book.Save(); err != nil {
		m.say("could not save the address book: "+err.Error(), true)
	}
	m.say(sayCall(m.st, "%s is forgotten.", m.contact.Name), false)
	m.menu = newMenu(m.deps.Book)
	m.contacts = newContacts(m.deps.Book)
	if len(m.contacts.list) == 0 {
		m.screen = screenMenu
	} else {
		m.screen = screenContacts
	}
	return m, nil
}

// reset deletes everything and quits: the next start asks the first-run
// questions. What could not be removed is named, so a person told "reset
// failed" is not left wondering whether their key is still on the disk.
func (m model) reset() (tea.Model, tea.Cmd) {
	failed := m.deps.Reset()
	if len(failed) > 0 {
		m.say(
			"could not delete "+strings.Join(failed, ", ")+"; some of it is still on the disk.",
			true,
		)
	} else {
		m.say("your identity, your address book and your settings are gone. start homa again and it will ask the first-run questions.", false)
	}
	return m, tea.Quit
}

func (m model) updateContacts(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if next, cmd, handled := m.barKey(k); handled {
		return next, cmd
	}
	act, ok := m.contacts.key(k)
	if !ok {
		m.say(
			"that is not one of the choices"+m.st.sep()+"press one of the keys on the left",
			false,
		)
		return m, nil
	}
	switch act {
	case contactOpen:
		m.contact = m.contacts.chosen()
		m.screen = screenContact
	case contactBack:
		m.screen = screenMenu
	case contactQuit:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateContact(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if next, cmd, handled := m.barKey(k); handled {
		return next, cmd
	}
	act, ok := contactKey(k)
	if !ok {
		m.say(
			"that is not one of the choices"+m.st.sep()+"press one of the keys on the left",
			false,
		)
		return m, nil
	}
	switch act {
	case contactCall:
		m.screen = screenMenu
		return m.placeCall(m.contact)
	case contactRename:
		return m.openForm(formRename, newForm("rename "+m.contact.Name,
			field{label: "a new name for them", def: m.contact.Name}), screenContact), nil
	case contactAddress:
		m.page = &page{
			title: m.contact.Name,
			body:  blockRows(m.contact.Addr, m.width-4) + "\n",
			back:  screenContact,
		}
		m.screen = screenPage
	case contactForget:
		f := newForm("forget "+m.contact.Name,
			field{label: "type the word forget to confirm", def: "cancel", word: wordForget})
		f.warn = []string{
			"Forgetting " + m.contact.Name + " takes their address and their key with them.",
			"Their next call arrives under the name they choose for themselves,",
			"and reaching them again means pasting their address in again.",
		}
		return m.openForm(formForget, f, screenContact), nil
	case contactBack:
		m.contacts = newContacts(m.deps.Book)
		m.screen = screenContacts
	case contactQuit:
		return m, tea.Quit
	}
	return m, nil
}

// placeCall dials a contact and puts the wait on the bar.
func (m model) placeCall(c contacts.Contact) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.bar = callBar{outgoing: c.Name, deadline: time.Now().Add(callAnswerTimeout), cancel: cancel}
	return m, tea.Batch(dial(ctx, m.deps, c, m.send), tick())
}

// startConversation is a line both sides agreed to.
func (m model) startConversation(l *line) (tea.Model, tea.Cmd) {
	m.bar.clear()
	m.screen = screenConversation
	cfg := m.deps.Cfg
	m.conv = newConversationWith(
		m.ctx,
		m.st,
		m.width,
		m.height,
		l,
		l.s.Peer().Nick,
		cfg.DownloadDir,
		m.send,
		cfg.EnsureDownloadDir,
	)
	m.say("", false)
	return m, runSession(m.ctx, l, m.send)
}

func (m model) updateConversation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cmd, leave := m.conv.update(m.st, msg)
	if !leave {
		return m, cmd
	}
	return m.leaveConversation(cmd)
}

// leaveConversation goes back to the menu. A caller who arrived meanwhile
// is on the bar, and its countdown starts showing.
func (m model) leaveConversation(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.conv = nil
	m.screen = screenMenu
	m.menu = newMenu(m.deps.Book) // a key learned on this call may have changed a label
	if m.bar.showing() {
		return m, tea.Batch(cmd, tick())
	}
	return m, cmd
}

// peerLeft is the far side gone. The pane says so, the typed line stays,
// and Enter goes back to the menu; if we were the ones leaving, the
// session's return is just the door closing behind us.
func (m model) peerLeft(err error) (tea.Model, tea.Cmd) {
	if m.conv == nil || m.conv.ended {
		return m, nil
	}
	m.conv.ended = true
	m.conv.offer = nil
	if err != nil {
		m.conv.alert(m.st, "the conversation ended: "+reason(err))
	} else {
		m.conv.note(m.st, m.st.peer(m.conv.l.name)+m.st.dim.Render(" left the conversation."))
	}
	// The line is done either way; close our end now rather than when the
	// person presses Enter, so the far side's Close, which waits for ours,
	// returns at once.
	return m, closeLine(m.conv.l)
}

func (m model) View() tea.View {
	var status, body, keys string
	switch m.screen {
	case screenConversation:
		status, body, keys = m.conv.view(m.st, m.width)
	case screenContacts:
		status, body, keys = m.header(), m.withBarAndNotice(
			m.contacts.view(m.st),
		), m.footer(
			"↑↓",
			"choose",
			"Enter",
			"open",
			"b",
			"back",
			"q",
			"quit",
		)
	case screenContact:
		status, body, keys = m.header(), m.withBarAndNotice(
			"\n  "+m.st.you.Render(
				m.contact.Name,
			)+"\n\n"+renderGroups(
				m.st,
				contactGroups(m.contact),
				-1,
			),
		), m.footer(
			"b",
			"back",
		)
	case screenForm:
		status, body, keys = m.header(), m.withBarAndNotice(
			m.form.view(m.st),
		), m.footer(
			"Enter",
			"next",
			"Esc",
			"back",
		)
	case screenPage:
		status, body, keys = m.header(), m.withBarAndNotice(
			m.page.view(m.st, m.width),
		), m.footer(
			"any key",
			"back",
		)
	default:
		status, body, keys = m.header(), m.withBarAndNotice(m.menuBody()), m.menuFooter()
	}

	v := tea.NewView(frame(m.width, m.height, status, body, keys))
	// What was on the screen stays in the terminal's scrollback when homa
	// exits, the way version 1 left it; the alternate screen would wipe it.
	v.AltScreen = false
	return v
}

// header is the top of every screen but the conversation: the mark and
// the name on the left; who you are, how your address starts, and that
// homa is listening on the right. Below frameMinWidth only the name fits.
func (m model) header() string {
	parts := []string{m.st.you.Render(m.deps.Cfg.Nick)}
	if m.width >= frameMinWidth && m.deps.Listener != nil {
		parts = append(parts, preview(m.deps.Listener.Addr()), m.st.them.Render("●")+" listening")
	}
	return header(m.st, m.width, brand(m.st), m.st.dim.Render(strings.Join(parts, m.st.sep())))
}

func (m model) footer(pairs ...string) string { return footer(m.st, m.width, m.st.keys(pairs...)) }

func (m model) menuFooter() string {
	switch {
	case m.bar.incoming != nil:
		return m.footer("y", "take the call", "n", "not now")
	case m.bar.outgoing != "":
		return m.footer("Enter", "give up")
	case len(m.menu.contacts) > 0:
		return m.footer(
			"↑↓",
			"choose",
			"Enter",
			"call",
			"1-"+strconv.Itoa(len(m.menu.contacts)),
			"call by number",
		)
	}
	return m.footer("n", "add a contact", "a", "your address", "h", "help")
}

// menuBody is the people on the left and homa's own keys on the right when
// there is room, one under the other when there is not. The people come
// first either way, because calling somebody is what the screen is for.
func (m model) menuBody() string {
	groups := m.menu.groups
	people := renderGroups(m.st, [][]menuItem{groups[0]}, m.menu.cursor)
	if len(groups[0]) == 0 {
		people = markInfo + "  " + m.st.dim.Render("nobody yet") + "\n" +
			markInfo + "  " + m.st.dim.Render("n adds a contact, a shows your address") + "\n"
	}
	rest := renderGroups(m.st, groups[1:], -1)

	if m.width < menuTwoColumns {
		return "\n  " + m.st.label.Render(
			"PEOPLE",
		) + "\n\n" + people + "\n  " + m.st.label.Render(
			"HOMA",
		) + "\n\n" + rest
	}

	left := lipgloss.NewStyle().
		Width(menuLeftColumn).
		Render("  " + m.st.label.Render("PEOPLE") + "\n\n" + people)
	right := "  " + m.st.label.Render("HOMA") + "\n\n" + rest
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// withBarAndNotice puts the call bar and the notice under a body.
func (m model) withBarAndNotice(body string) string {
	var b strings.Builder
	b.WriteString(body)
	if bar := m.bar.view(m.st, m.width, time.Now()); bar != "" {
		b.WriteString("\n" + bar + "\n")
	}
	if m.notice != "" {
		b.WriteString("\n")
		if m.warn {
			b.WriteString("  " + m.st.warn.Render(m.notice))
		} else {
			b.WriteString("  " + m.st.dim.Render(m.notice))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// progressBar is ten cells of a file's progress, in the far side's color for
// what has arrived. Full and light blocks, which every terminal font has.
func progressBar(st *styles, pct int) string {
	done := min(max(pct/10, 0), 10)
	if !st.unicode {
		return st.them.Render(
			strings.Repeat("#", done),
		) + st.dim.Render(
			strings.Repeat("-", 10-done),
		)
	}
	return st.them.Render(strings.Repeat("█", done)) + st.dim.Render(strings.Repeat("░", 10-done))
}
