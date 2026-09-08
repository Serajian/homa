package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// screen is which of homa's screens fills the body. One at a time; the
// call bar is not a screen but a line under whichever one is up.
type screen int

const (
	screenMenu screen = iota
	screenConversation
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

	menu menuModel
	bar  callBar
	conv *conversation

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

// Init starts greeting callers, for the life of the program.
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
			return m, tick()
		}
		return m, nil

	case callArrived:
		return m.callArrived(msg.l)
	case callGone:
		if m.bar.incoming == msg.l {
			m.bar.clear()
			m.say(sayCall(m.st, "the call from %s ran out of time while you were busy.", msg.l.name), false)
		}
		return m, nil
	case callRefused:
		m.bar.clear()
		m.say(sayCall(m.st, msg.format, msg.name), false)
		return m, nil
	case callFailed:
		m.bar.clear()
		m.say(sayCall(m.st, "could not reach %s: ", msg.name)+reason(msg.err), true)
		return m, nil
	case callAnswered:
		return m.startConversation(msg.l)

	case peerSaid:
		if m.conv != nil {
			m.conv.say(m.st.dim.Render("[") + m.st.peer(m.conv.l.name) + m.st.dim.Render("]") + " " + msg.text)
		}
		return m, nil
	case peerLeft:
		return m.peerLeft(msg.err)
	case sendFailed:
		if m.conv != nil && !m.conv.ended {
			m.conv.say(m.st.warn.Render(markWarn + "could not send: " + reason(msg.err)))
			m.conv.ended = true
		}
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
			return m, tea.Quit
		}
		switch m.screen {
		case screenConversation:
			return m.updateConversation(msg)
		default:
			return m.updateMenu(msg)
		}
	}
	return m, nil
}

func (m *model) say(text string, warn bool) { m.notice, m.warn = text, warn }

// callArrived parks a caller on the bar. A second caller while one waits
// is turned away as busy; a caller during a conversation waits unseen
// until it ends, and the bar's own timer hangs up if that is too long.
func (m model) callArrived(l *line) (tea.Model, tea.Cmd) {
	if m.bar.showing() {
		return m, turnAway(l)
	}
	m.bar = callBar{incoming: l, deadline: l.deadline}
	if m.screen == screenConversation {
		return m, nil
	}
	return m, tick()
}

// updateMenu is a key on the menu, or on the call bar over it. While a
// caller waits, the bar has the keyboard: y lets them in, n does not, and
// nothing else does anything, as the question in version 1 did.
func (m model) updateMenu(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if l := m.bar.incoming; l != nil {
		switch k {
		case "y":
			if !l.claim() {
				m.bar.clear()
				return m, nil
			}
			return m, takeCall(l)
		case "n", keyEnter:
			if !l.claim() {
				m.bar.clear()
				return m, nil
			}
			return m, declineCall(l, "they are not taking calls right now", "the call from %s was not taken.")
		default:
			m.say("y takes the call, n does not", false)
			return m, nil
		}
	}

	if m.bar.outgoing != "" {
		if k == keyEnter {
			m.bar.cancel() // the dial reports back that we stopped calling
		}
		return m, nil
	}

	act, ok := m.menu.key(k)
	if !ok {
		m.say("that is not one of the choices"+m.st.sep()+"press one of the keys on the left, or h for help", false)
		return m, nil
	}

	switch act {
	case actQuit:
		return m, tea.Quit
	case actCall:
		return m.placeCall()
	case actNone, actClear:
		// The frame is redrawn whole on every update; there is nothing a
		// clear could clear that the next draw does not.
		return m, nil
	default:
		// Replaced screen by screen as the plan's later tasks land.
		m.say("not on this screen yet", false)
		return m, nil
	}
}

// placeCall dials the contact under the cursor and puts the wait on the bar.
func (m model) placeCall() (tea.Model, tea.Cmd) {
	c := m.menu.chosen()
	ctx, cancel := context.WithCancel(m.ctx)
	m.bar = callBar{outgoing: c.Name, deadline: time.Now().Add(callAnswerTimeout), cancel: cancel}
	return m, tea.Batch(dial(ctx, m.deps, c, m.send), tick())
}

// startConversation is a line both sides agreed to.
func (m model) startConversation(l *line) (tea.Model, tea.Cmd) {
	m.bar.clear()
	m.screen = screenConversation
	m.conv = newConversation(m.st, m.width, m.height, l, l.s.Peer().Nick, m.deps.Cfg.DownloadDir)
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
	if err != nil {
		m.conv.say(m.st.warn.Render(markWarn + "the conversation ended: " + reason(err)))
	} else {
		m.conv.say(markInfo + m.st.peer(m.conv.l.name) + m.st.dim.Render(" left the conversation."))
	}
	return m, nil
}

func (m model) View() tea.View {
	var status, body, keys string
	switch m.screen {
	case screenConversation:
		status, body, keys = m.conv.view(m.st, m.width)
	default:
		status, body, keys = m.statusLine(), m.menuBody(), m.keyLine()
	}

	v := tea.NewView(frame(m.width, m.height, status, body, keys))
	// What was on the screen stays in the terminal's scrollback when homa
	// exits, the way version 1 left it; the alternate screen would wipe it.
	v.AltScreen = false
	return v
}

// statusLine is the top line of the menu: who you are, how your address
// starts, and that homa is listening. Below frameMinWidth only the name fits.
func (m model) statusLine() string {
	parts := []string{m.st.you.Render("homa"), "you are " + m.st.you.Render(m.deps.Cfg.Nick)}
	if m.width >= frameMinWidth && m.deps.Listener != nil {
		parts = append(parts, preview(m.deps.Listener.Addr()), "listening")
	}
	return " " + m.st.dim.Render(strings.Join(parts, m.st.sep()))
}

func (m model) menuBody() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.st.you.Render("What now?"))
	b.WriteString("\n\n")
	b.WriteString(m.menu.view(m.st))
	if bar := m.bar.view(m.st, time.Now()); bar != "" {
		b.WriteString("\n")
		b.WriteString(bar)
		b.WriteString("\n")
	}
	if m.notice != "" {
		b.WriteString("\n")
		if m.warn {
			b.WriteString(m.st.warn.Render(markWarn + m.notice))
		} else {
			b.WriteString(markInfo + m.st.dim.Render(m.notice))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) keyLine() string {
	keys := "↑↓ choose · Enter call · or press a key"
	if !m.st.unicode {
		keys = "up/down choose - Enter call - or press a key"
	}
	if m.bar.incoming != nil {
		keys = fmt.Sprintf("y take the call · n not now%s", "")
	}
	return " " + m.st.dim.Render(keys)
}
