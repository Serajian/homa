package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// conversation is one open line: a pane of what was said, and the line
// being typed. The two never touch. A message arriving lands in the pane,
// and the input keeps whatever was half-typed; that is what version 1
// could not do, and the reason this interface exists.
type conversation struct {
	l     *line
	nick  string // what they call themselves
	files string // where received files go, as written in the settings

	pane  viewport.Model
	in    textinput.Model
	hist  history
	lines []string

	// ended is the far side gone or the line broken: the pane says so, the
	// typed line stays, and Enter or /quit goes back to the menu.
	ended bool
}

// The rows the conversation needs besides the pane: the rule and the input.
const conversationChrome = 2

func newConversation(st *styles, width, height int, l *line, nick, files string) *conversation {
	in := textinput.New()
	in.Prompt = ""
	in.SetVirtualCursor(true)
	in.CharLimit = maxInputLen

	c := &conversation{l: l, nick: nick, files: files, in: in, pane: viewport.New()}
	c.pane.MouseWheelEnabled = true
	c.resize(width, height)
	_ = c.in.Focus()

	if l.known {
		c.say(st.dim.Render("talking to ") + st.peer(l.name) + st.dim.Render(st.sep()+"they call themselves ") + st.dim.Render(quote(nick)))
	} else {
		c.say(st.dim.Render("talking to ") + st.peer(l.name) + st.dim.Render(st.sep()+"the name is theirs; they are not in your contacts"))
	}
	c.say(st.dim.Render("/help commands" + st.sep() + "/quit leave" + st.sep() + "files go to " + files))
	c.say("")
	return c
}

func quote(s string) string { return "\"" + s + "\"" }

func (c *conversation) resize(width, height int) {
	paneHeight := max(height-statusHeight-keysHeight-conversationChrome, 1)
	c.pane.SetWidth(width)
	c.pane.SetHeight(paneHeight)
	c.in.SetWidth(max(width-len("[me] ")-1, 10))
	c.pane.SetContent(strings.Join(c.lines, "\n"))
	c.pane.GotoBottom()
}

// say appends a line to the pane. If the person had scrolled up to read,
// the pane stays where they left it; otherwise it follows the newest line.
func (c *conversation) say(s string) {
	wasAtBottom := c.pane.AtBottom()
	c.lines = append(c.lines, s)
	c.pane.SetContent(strings.Join(c.lines, "\n"))
	if wasAtBottom {
		c.pane.GotoBottom()
	}
}

// view is the conversation's three regions.
func (c *conversation) view(st *styles, width int) (status, body, keys string) {
	status = " " + st.dim.Render("talking to ") + st.peer(c.l.name)
	if c.ended {
		status += st.dim.Render(st.sep() + "the line is closed")
	}

	rule := strings.Repeat("┄", max(width, 1))
	if !st.unicode {
		rule = strings.Repeat("-", max(width, 1))
	}

	body = c.pane.View() + "\n" + st.rule.Render(rule) + "\n" +
		st.dim.Render("[") + st.you.Render(selfNick) + st.dim.Render("]") + " " + c.in.View()

	keys = " " + st.dim.Render("PgUp/PgDn scroll · ↑↓ history · /help · /quit")
	if !st.unicode {
		keys = " " + st.dim.Render("PgUp/PgDn scroll - up/down history - /help - /quit")
	}
	if c.ended {
		keys = " " + st.dim.Render("Enter to go back to the menu")
	}
	return status, body, keys
}

// update handles a key or a wheel. leave is the person going back to the
// menu; the returned command is what to do on the way (send a line, close
// the line).
func (c *conversation) update(st *styles, msg tea.Msg) (cmd tea.Cmd, leave bool) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		c.pane, cmd = c.pane.Update(msg)
		return cmd, false

	case tea.KeyPressMsg:
		switch msg.String() {
		case keyEnter:
			return c.enter(st)
		case keyUp:
			if s, ok := c.hist.up(c.in.Value()); ok {
				c.in.SetValue(s)
				c.in.CursorEnd()
			}
			return nil, false
		case keyDown:
			if s, ok := c.hist.down(); ok {
				c.in.SetValue(s)
				c.in.CursorEnd()
			}
			return nil, false
		case keyPgUp, keyPgDown:
			c.pane, cmd = c.pane.Update(msg)
			return cmd, false
		}
		c.in, cmd = c.in.Update(msg)
		return cmd, false
	}
	return nil, false
}

// enter sends the typed line, or runs it as a command. A bare Enter is
// not a message; on a closed line it is the way back to the menu.
func (c *conversation) enter(st *styles) (tea.Cmd, bool) {
	text := strings.TrimSpace(c.in.Value())
	if c.ended {
		return nil, true
	}
	if text == "" {
		return nil, false
	}
	c.in.Reset()
	c.hist.push(text)

	if strings.HasPrefix(text, "/") {
		return c.command(st, text)
	}

	c.say(st.dim.Render("[") + st.you.Render(selfNick) + st.dim.Render("]") + " " + text)
	l := c.l
	return func() tea.Msg {
		if err := l.s.SendText(text); err != nil {
			return sendFailed{err}
		}
		return nil
	}, false
}

// command runs a slash command typed in the conversation. Files come in the
// next step; here are the ones that need nothing but the line.
func (c *conversation) command(st *styles, text string) (tea.Cmd, bool) {
	cmd, _, _ := strings.Cut(text, " ")
	switch cmd {
	case "/quit":
		return closeLine(c.l), true
	case "/help", "/":
		c.say(st.dim.Render(conversationCommands))
		return nil, false
	case "/who":
		if c.l.known {
			c.say(markInfo + st.peer(c.l.name) + st.dim.Render(", calling themselves "+quote(c.nick)))
		} else {
			c.say(markInfo + st.peer(c.l.name) + st.dim.Render(", which is what they call themselves"))
		}
		return nil, false
	case "/clear":
		c.lines = nil
		c.pane.SetContent("")
		return nil, false
	}
	c.say(st.warn.Render(markWarn + "no such command: " + quote(cmd)))
	c.say(markInfo + st.dim.Render("these are the commands; a line that is not one is sent as a message"))
	c.say(st.dim.Render(conversationCommands))
	return nil, false
}

// conversationCommands is what /help shows. It is content, kept beside the
// code that shows it; whoever changes a command changes this too.
const conversationCommands = `  /files [dir]  list a directory, numbered
  /files <n>    list one from the last listing, .. included
  /send <path>  offer a file
  /send <n>     offer one from the last listing
  /accept       take the file being offered, or just y
  /reject       refuse it, or just n
  /who          who you are talking to
  /clear        wipe the screen
  /quit         leave the conversation, not homa`
