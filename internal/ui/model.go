package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// screen is which of homa's screens fills the body. One at a time; the
// incoming-call bar is not a screen but a line under whichever one is up.
type screen int

const (
	screenMenu screen = iota
)

// model is the whole interface: what is on the screen, and enough of what
// homa knows to draw it. Every Update runs on one goroutine, so nothing in
// here is guarded; what arrives from elsewhere arrives as a message.
type model struct {
	deps Deps
	st   *styles

	width, height int
	screen        screen

	menu menuModel

	// notice is one grey line under the body: a hint after a wrong key,
	// why a call did not go through. The next key clears it.
	notice string
}

func newModel(deps Deps, st *styles) model {
	return model{deps: deps, st: st, menu: newMenu(deps.Book), screen: screenMenu}
}

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
		m.notice = "that is not one of the choices" + m.st.sep() +
			"press one of the keys on the left, or h for help"
		return m, nil
	}

	switch act {
	case actQuit:
		return m, tea.Quit
	case actNone, actClear:
		// The frame is redrawn whole on every update; there is nothing a
		// clear could clear that the next draw does not.
		return m, nil
	default:
		// Replaced screen by screen as the plan's later tasks land.
		m.notice = "not on this screen yet"
		return m, nil
	}
}

func (m model) View() tea.View {
	v := tea.NewView(frame(m.width, m.height, m.statusLine(), m.body(), m.keyLine()))
	// What was on the screen stays in the terminal's scrollback when homa
	// exits, the way version 1 left it; the alternate screen would wipe it.
	v.AltScreen = false
	return v
}

// statusLine is the top line: who you are, how your address starts, and
// that homa is listening. Below frameMinWidth only the name fits.
func (m model) statusLine() string {
	parts := []string{m.st.you.Render("homa"), "you are " + m.st.you.Render(m.deps.Cfg.Nick)}
	if m.width >= frameMinWidth && m.deps.Listener != nil {
		parts = append(parts, preview(m.deps.Listener.Addr()), "listening")
	}
	return " " + m.st.dim.Render(strings.Join(parts, m.st.sep()))
}

func (m model) body() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.st.you.Render("What now?"))
	b.WriteString("\n\n")
	b.WriteString(m.menu.view(m.st))
	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(markInfo)
		b.WriteString(m.st.dim.Render(m.notice))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) keyLine() string {
	keys := "↑↓ choose · Enter call · or press a key"
	if !m.st.unicode {
		keys = "up/down choose - Enter call - or press a key"
	}
	return " " + m.st.dim.Render(keys)
}
