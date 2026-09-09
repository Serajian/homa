package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Serajian/homa/internal/peer"
)

// conversation is one open line: a pane of what was said, and the line
// being typed. The two never touch. A message arriving lands in the pane,
// and the input keeps whatever was half-typed; that is what version 1
// could not do, and the reason this interface exists.
type conversation struct {
	l        *line
	nick     string // what they call themselves
	filesDir string // where received files go, as written in the settings

	pane  viewport.Model
	in    textinput.Model
	hist  history
	lines []paneLine

	// pick is which of the commands the hint row offers is marked, moved
	// with the left and right arrows and taken by Tab or Enter; it goes
	// back to the first whenever the command word changes.
	pick int

	// ended is the far side gone or the line broken: the pane says so, the
	// typed line stays, and Enter or /quit goes back to the menu.
	ended bool

	// started is when the line opened, for /who to say how long it has
	// been: a duration rather than a clock time, because homa shows no
	// times yet and /store in version 3 owns that question.
	started time.Time

	// offer is a file the far side is offering, waiting for y or n; the
	// session's read goroutine is blocked on its reply meanwhile.
	offer *fileOffered

	// files is the last directory shown with /files, so /send can take a
	// number out of it.
	files *listing

	// width is the last width the screen was drawn at: what the pane wraps
	// to, and what /me wraps an address to.
	width int

	// me answers /me: the three groups about this machine, and the address
	// to copy or hand over. nil in tests that do not need it.
	me func(width int) (body, addr string)

	// stopped is what this side asked to stop, so the failure that comes
	// back for it is not reported twice in different words.
	stopped map[string]bool

	// getting and pushing time the transfers running each way, so a rate
	// and an estimate can be worked out from the bytes reported.
	getting, pushing transfer

	// card is the address the peer handed over, held until the person
	// saves it or the conversation ends. Nothing is written without /save.
	card string

	// ctx and send are what /send needs to run a transfer in the
	// background and report on it; downloadDir is where an accepted file
	// goes, asked at the moment of accepting so a changed setting counts.
	ctx         context.Context
	send        func(tea.Msg)
	downloadDir func() (string, error)
}

func newConversation(st *styles, width, height int, l *line, nick, files string) *conversation {
	return newConversationWith(
		context.Background(),
		st,
		width,
		height,
		l,
		nick,
		files,
		func(tea.Msg) {},
		nil,
	)
}

// newConversationWith is newConversation with the pieces file transfer
// needs; tests of the pane and the input use the short form.
func newConversationWith(
	ctx context.Context, st *styles, width, height int, l *line, nick, files string,
	send func(tea.Msg), downloadDir func() (string, error),
) *conversation {
	in := textinput.New()
	in.Prompt = ""
	in.SetVirtualCursor(true)
	in.CharLimit = maxInputLen

	c := &conversation{
		l:           l,
		nick:        nick,
		started:     time.Now(),
		filesDir:    files,
		in:          in,
		pane:        viewport.New(),
		ctx:         ctx,
		send:        send,
		downloadDir: downloadDir,
	}
	c.pane.MouseWheelEnabled = true
	c.resize(width, height)
	_ = c.in.Focus()

	// Who this is lives in the header; the pane opens on what needs saying
	// once: that a name not in the book is theirs, not yours.
	if !l.known {
		c.note(st, st.dim.Render("the name is theirs; they are not in your contacts"))
	}
	return c
}

// transfer is a file on the move: which one, and when it started, which is
// all a rate needs beside the bytes each report carries.
type transfer struct {
	name    string
	started time.Time
}

// moving draws one line of a transfer's progress: the bar and the percent
// always, and how fast it is going and how much longer once there has been
// enough of it to say. A transfer is timed from the first report about it.
func (c *conversation) moving(
	st *styles,
	verb, name string,
	t *transfer,
	file string,
	received, total int64,
) {
	if t.name != file {
		*t = transfer{name: file, started: time.Now()}
	}
	pct := percent(received, total)

	line := st.dim.Render(verb) + name + "  " + progressBar(st, pct) +
		st.dim.Render(fmt.Sprintf("  %d%%", pct))
	since := time.Since(t.started)
	if rate := perSecond(received, since); rate != "" {
		line += st.dim.Render("  " + rate)
	}
	if left := leftText(total-received, received, since); left != "" {
		line += st.dim.Render("  " + left)
	}
	c.note(st, line)
}

// paneLine is one thing in the pane: who said it, the bar between the name
// column and the words, and the words. The three are kept apart and styled
// but never joined, because joining them is what fixed a line to the width
// it was drawn at: a message wider than the terminal was cut, and the rest
// of it could not be reached by scrolling or by making the window bigger.
// Rendering wraps the words instead, and a resize renders again.
//
// A zero paneLine is a blank line, which is how a file offer and /me set
// themselves apart from what was said around them.
type paneLine struct {
	name string // styled; only the first row of a wrapped message carries it
	bar  string // styled; empty for a blank line
	body string // styled
}

// wrapPoints are the characters a long word may be broken at, besides a
// space. A path and an address are the long words homa actually shows.
const wrapPoints = "-/_"

// msg is a line somebody said: the name right-aligned to the name column,
// a faint bar, the words. Every message's text starts at the same place,
// which is what makes a conversation readable at a glance.
func (c *conversation) msg(st *styles, who, text string, mine bool) {
	name := st.peer(who)
	if mine {
		name = st.you.Render(who)
	}
	c.add(paneLine{name: name, bar: st.dim.Render("│"), body: text})
}

// note is homa's own line in the pane, in the column the words use, with
// no name: what a file is doing, what a command said.
func (c *conversation) note(st *styles, styled string) {
	c.add(paneLine{bar: st.dim.Render("│"), body: styled})
}

// alert is note in the terminal's yellow, bar included.
func (c *conversation) alert(st *styles, text string) {
	c.add(paneLine{bar: st.warn.Render("│"), body: st.warn.Render(text)})
}

// blank is an empty row, for setting a question apart from the talk.
func (c *conversation) blank() { c.add(paneLine{}) }

func quote(s string) string { return "\"" + s + "\"" }

func (c *conversation) resize(width, height int) {
	c.width = width
	paneHeight := max(height-headerHeight-footerHeight-inputBoxHeight-1, 1)
	c.pane.SetWidth(width)
	c.pane.SetHeight(paneHeight)
	c.in.SetWidth(max(width-8, 10))
	c.pane.SetContent(c.render())
	c.pane.GotoBottom()
}

// add puts a line in the pane. If the person had scrolled up to read, the
// pane stays where they left it; otherwise it follows the newest line.
func (c *conversation) add(l paneLine) {
	wasAtBottom := c.pane.AtBottom()
	c.lines = append(c.lines, l)
	c.pane.SetContent(c.render())
	if wasAtBottom {
		c.pane.GotoBottom()
	}
}

// render lays the pane out at the width it is being drawn at: the words
// wrapped to the room left of the name column, and every row after the
// first under the words rather than under the name.
func (c *conversation) render() string {
	// the name column, a space, the bar, a space
	room := max(c.width-nameColumn-3, 12)
	indent := strings.Repeat(" ", nameColumn)

	out := make([]string, 0, len(c.lines))
	for _, l := range c.lines {
		if l.bar == "" && l.body == "" {
			out = append(out, "")
			continue
		}
		rows := strings.Split(lipgloss.Wrap(l.body, room, wrapPoints), "\n")
		for i, row := range rows {
			head := indent
			if i == 0 && l.name != "" {
				head = strings.Repeat(" ", max(nameColumn-lipgloss.Width(l.name), 0)) + l.name
			}
			out = append(out, head+" "+l.bar+" "+row)
		}
	}
	return strings.Join(out, "\n")
}

// view is the conversation's three regions: the header saying who, the
// pane, and the input in its box, with the keys under it.
func (c *conversation) view(st *styles, width int) (status, body, keys string) {
	who := st.dim.Render("talking to ") + st.peer(c.l.name)
	if c.l.known {
		who += st.dim.Render(st.sep() + "they call themselves " + quote(c.nick))
	}
	if c.ended {
		who += st.dim.Render(st.sep() + "the line is closed")
	}
	status = header(st, width, brand(st)+"   "+who, st.dim.Render("files → "+c.filesDir))

	// The row between the pane and the box is the hint row: what a line
	// starting with a slash can become, and blank otherwise, so the layout
	// is the same whether or not a command is being typed.
	body = c.pane.View() + "\n    " + hint(st, c.in.Value(), c.pick, width-4) + "\n  " +
		strings.ReplaceAll(st.box(st.boxDim(), width-4, "", c.in.View()), "\n", "\n  ")

	if c.ended {
		keys = footer(st, width, st.keys("Enter", "back to the menu"))
	} else {
		keys = footer(st, width, st.keys("PgUp PgDn", "scroll", "↑ ↓", "history", "/help", "commands", "/quit", "leave"))
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

	case tea.PasteMsg:
		before, _ := commandWord(c.in.Value())
		c.in, cmd = c.in.Update(msg)
		if after, _ := commandWord(c.in.Value()); after != before {
			c.pick = 0
		}
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
		case keyLeft, keyRight:
			// While a command word is being typed the arrows walk the
			// hint row rather than the cursor: there is nothing to edit
			// inside a word a few letters long, and the row is what the
			// eye is on.
			if n := len(c.candidates()); n > 1 {
				if msg.String() == keyLeft {
					c.pick = (c.pick + n - 1) % n
				} else {
					c.pick = (c.pick + 1) % n
				}
				return nil, false
			}
		case keyTab:
			c.take()
			return nil, false
		}
		before, _ := commandWord(c.in.Value())
		c.in, cmd = c.in.Update(msg)
		if after, _ := commandWord(c.in.Value()); after != before {
			c.pick = 0
		}
		return cmd, false
	}
	return nil, false
}

// candidates is what the hint row is offering for the typed line: the
// commands the word can still become, while no argument has begun.
func (c *conversation) candidates() []command {
	word, argBegun := commandWord(c.in.Value())
	if word == "" || argBegun {
		return nil
	}
	return matches(word)
}

// take puts the picked command in the line, with a space after it when it
// wants an argument, and reports whether the line changed.
func (c *conversation) take() bool {
	typed := c.in.Value()
	done := complete(typed, c.pick)
	if done == typed {
		return false
	}
	c.in.SetValue(done)
	c.in.CursorEnd()
	c.pick = 0
	return true
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

	// Enter on a command word still being typed takes what the hint row
	// offers, the way Tab does: a command that wants an argument goes in
	// the line to be finished, one that wants nothing runs at once. So an
	// arrow to /quit and Enter leaves, and a lone slash is /help.
	if ms := c.candidates(); len(ms) > 0 && ms[c.pick].needsArg() {
		c.take()
		return nil, false
	} else if len(ms) > 0 {
		text = ms[c.pick].name
	}
	c.in.Reset()

	// A yes or no while a file offer is waiting answers it. Checked before
	// anything else, so the answer to a question on the screen is never
	// sent to the peer as a message instead.
	if c.offer != nil {
		switch strings.ToLower(text) {
		case "y", "yes":
			c.answerOffer(st, true)
			return nil, false
		case "n", "no":
			c.answerOffer(st, false)
			return nil, false
		}
	}
	c.hist.push(text)

	if strings.HasPrefix(text, "/") {
		return c.command(st, text)
	}

	c.msg(st, selfNick, text, true)
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
	cmd, arg, _ := strings.Cut(text, " ")
	arg = strings.TrimSpace(arg)
	switch cmd {
	case "/quit":
		return closeLine(c.l), true
	case "/help":
		for _, l := range helpLines() {
			c.note(st, st.dim.Render(l))
		}
		return nil, false
	case "/who":
		c.who(st)
		if c.l.conn == nil {
			return nil, false
		}
		return probePath(c.ctx, c.l.conn), false
	case "/me":
		return c.showMe(st, arg), false
	case "/add":
		return c.keepAddress(st, arg), false
	case "/clear":
		c.lines = nil
		c.pane.SetContent("")
		return nil, false
	case "/accept":
		if c.offer == nil {
			c.alert(st, errNoOffer.Error())
			return nil, false
		}
		c.answerOffer(st, true)
		return nil, false
	case "/reject":
		if c.offer == nil {
			c.alert(st, errNoOffer.Error())
			return nil, false
		}
		c.answerOffer(st, false)
		return nil, false
	case "/files", "/ls":
		c.showFiles(st, arg)
		return nil, false
	case "/send":
		return c.sendFile(st, arg), false
	case "/cancel":
		c.cancelTransfers(st)
		return nil, false
	}
	c.alert(st, "no such command: "+quote(cmd))
	c.note(st, st.dim.Render("these are the commands; a line that is not one is sent as a message"))
	for _, l := range helpLines() {
		c.note(st, st.dim.Render(l))
	}
	return nil, false
}

// showMe prints this machine's own address, relay and key into the pane:
// the same three groups the page shows, so they can be read out or handed
// over without leaving the conversation. "copy" puts the address on the
// clipboard, since the page's c is an ordinary letter in here.
func (c *conversation) showMe(st *styles, arg string) tea.Cmd {
	if arg != "" && arg != "copy" && arg != "send" {
		c.alert(st, "/me takes nothing, or the word copy, or the word send")
		return nil
	}
	if c.me == nil {
		c.alert(st, "there is nothing to show: homa is not listening")
		return nil
	}

	body, addr := c.me(max(c.width-nameColumn-6, 20))
	c.blank()
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		c.note(st, line)
	}
	if arg == "" {
		// The nudge belongs here rather than in the groups: it is only
		// true inside a conversation, and only with a peer new enough.
		if c.l.s != nil && c.l.s.CanTakeAddress() {
			c.note(st, st.dim.Render("/me send gives them this address, so they can call you"))
		}
		return nil
	}
	if addr == "" {
		c.alert(st, "there is no address to "+arg)
		return nil
	}
	if arg == "copy" {
		return copyToClipboard(addr)
	}
	return c.sendAddress(st, addr)
}

// sendAddress hands this machine's address to the peer, which is the only
// way somebody who was called can ever call back. An older peer drops what
// it does not recognize, so it is told rather than left believing it went.
func (c *conversation) sendAddress(st *styles, addr string) tea.Cmd {
	if c.l.s == nil {
		return nil
	}
	if !c.l.s.CanTakeAddress() {
		c.alert(st, "they are running an older homa and cannot take an address")
		c.note(
			st,
			st.dim.Render("read it to them, or send it another way; it is a secret either way"),
		)
		return nil
	}
	l := c.l
	return func() tea.Msg { return addressSent{err: l.s.SendAddress(addr)} }
}

// gotAddress is the peer handing theirs over. It is held, not saved: a
// contact needs a name, and the name is the person's to choose.
func (c *conversation) gotAddress(st *styles, addr string) {
	c.card = addr
	c.blank()
	c.note(st, st.peer(c.l.name)+st.dim.Render(" sent you their address, so you can call them"))
	if c.l.known {
		c.note(st, st.dim.Render("they are already in your address book as "+c.l.name))
		return
	}
	c.note(st, st.dim.Render("/add keeps them as ")+st.you.Render(c.nick)+
		st.dim.Render(", or /add <name>"))
}

// keepAddress asks the model to keep what was sent: the book is the model's,
// not the conversation's.
func (c *conversation) keepAddress(st *styles, arg string) tea.Cmd {
	if c.card == "" {
		c.alert(st, "nobody has sent you an address")
		c.note(st, st.dim.Render("/me send gives them yours; theirs is theirs to send"))
		return nil
	}

	name := strings.TrimSpace(arg)
	named := name != ""
	if !named {
		name = c.nick
	}
	addr := c.card
	return func() tea.Msg { return keepAddress{name: name, addr: addr, named: named} }
}

// saved is the model reporting that the contact is on disk. The header and
// every later line use the name from now on, the way a saved caller's would.
func (c *conversation) saved(st *styles, name string) {
	// The address stays held: asking again is then answered with "you
	// already have that", which is the truth, rather than with a line
	// about nobody having sent anything.
	c.l.name, c.l.known = name, true
	c.note(st, st.dim.Render("saved as ")+st.peer(name)+
		st.dim.Render("; they are in the menu next time homa starts"))
}

// who is the first line /who says: the name and where it came from, and
// the far side's key fingerprint with whether it matched the book. The
// path follows when the probe answers, so the line is never held.
func (c *conversation) who(st *styles) {
	var line string
	if c.l.known {
		line = st.peer(c.l.name) + st.dim.Render(", calling themselves "+quote(c.nick))
	} else {
		line = st.peer(c.l.name) + st.dim.Render(", which is what they call themselves")
	}
	c.note(st, line)
	if c.l.conn == nil {
		return
	}
	// The key on a line of its own: with the name and the nick it is wider
	// than a terminal, and a fingerprint cut short is worse than none.
	if fp := peer.Fingerprint(c.l.conn); fp != "" {
		match := "not in your book"
		if c.l.known {
			match = "matches your book"
		}
		c.note(st, st.dim.Render("key: ")+st.you.Render(fp)+st.dim.Render("  "+match))
	}
}

// pathLine is /who's second line, once the probe has answered.
func (c *conversation) pathLine(st *styles, msg pathProbed) {
	since := "for " + sinceText(time.Since(c.started))
	if errors.Is(msg.err, peer.ErrPathUnknown) {
		// The side that answered has no view of the path: the transport's
		// status table stays empty there. The side that called can tell.
		c.note(
			st,
			st.dim.Render("path: not known on the side that answered; the caller's /who can tell"+
				st.sep()+since),
		)
		return
	}
	if msg.err != nil {
		c.note(st, st.dim.Render("path: "+reason(msg.err)))
		return
	}
	p := msg.path
	var parts []string
	switch {
	case p.Direct:
		parts = append(parts, st.you.Render("direct"))
	case p.Relay != "":
		parts = append(parts, "through the relay "+st.you.Render(p.Relay))
	default:
		parts = append(parts, "through a relay")
	}
	switch {
	case p.Latency >= time.Millisecond:
		parts = append(parts, fmt.Sprintf("%d ms", p.Latency.Milliseconds()))
	case p.Latency > 0:
		parts = append(parts, "<1 ms")
	}
	parts = append(parts, since)
	if p.Rx > 0 || p.Tx > 0 {
		parts = append(parts, "↑ "+humanBytes(p.Tx)+"  ↓ "+humanBytes(p.Rx))
	}
	c.note(st, st.dim.Render("path: ")+strings.Join(parts, st.dim.Render(st.sep())))
}

// sinceText is a duration as a person would say it: seconds under a
// minute, minutes under an hour, then hours and minutes.
func sinceText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}

// probePath asks peer how the line travels, off the update loop.
func probePath(ctx context.Context, conn net.Conn) tea.Cmd {
	return func() tea.Msg {
		p, err := peer.Probe(ctx, conn)
		return pathProbed{path: p, err: err}
	}
}

// cancelTransfers stops whatever files are moving, in either direction, and
// tells the far side so their end stops too.
func (c *conversation) cancelTransfers(st *styles) {
	if c.l.s == nil {
		return
	}

	stopped := c.l.s.CancelTransfers()
	if len(stopped) == 0 {
		c.alert(st, "nothing is being sent or taken right now")
		return
	}

	if c.stopped == nil {
		c.stopped = make(map[string]bool)
	}
	for _, name := range stopped {
		c.stopped[name] = true
		c.note(st, st.dim.Render("stopped ")+st.you.Render(name))
	}
	if !c.l.s.CanCancel() {
		c.note(st, st.dim.Render("they are running an older homa, so their end may not stop"))
	}
}

// wasStopped reports whether this side asked for that file to stop, and
// forgets it: the failure it causes is arriving now and has been said once
// already.
func (c *conversation) wasStopped(name string) bool {
	if !c.stopped[name] {
		return false
	}
	delete(c.stopped, name)
	return true
}

// offered is the far side offering a file: one line, who, what, how big,
// what to type, and the offer parked until the answer.
func (c *conversation) offered(st *styles, o fileOffered) {
	c.offer = &o
	c.blank()
	c.note(st, st.peer(c.l.name)+st.dim.Render(" offers ")+st.them.Render(o.name)+
		st.dim.Render(" ("+humanBytes(o.size)+")   ")+st.keys("y", "accept", "n", "reject"))
}

// answerOffer delivers the person's decision to the goroutine waiting on it.
// Accepting asks the settings where files go, now rather than earlier, so a
// setting changed since the offer counts.
func (c *conversation) answerOffer(st *styles, accept bool) {
	o := c.offer
	c.offer = nil

	ans := fileAnswer{accept: accept, reason: "declined"}
	if accept {
		dir, err := c.downloadDir()
		if err != nil {
			c.alert(st, err.Error())
			ans = fileAnswer{reason: "the receiver has nowhere to put it"}
		} else {
			ans = fileAnswer{accept: true, dir: dir}
		}
	}

	select {
	case o.reply <- ans:
	default: // already answered, or timed out
	}
}

// showFiles lists a directory in the pane and remembers it, so /send can
// take a number from what was shown.
func (c *conversation) showFiles(st *styles, arg string) {
	dir, err := dirFromArg(c.files, arg)
	if err != nil {
		c.alert(st, reason(err))
		return
	}

	l, hidden, err := readDir(dir)
	if err != nil {
		c.alert(st, reason(err))
		return
	}
	c.files = l

	c.blank()
	c.note(st, st.dim.Render(l.dir))
	if len(l.entries) == 0 {
		c.note(st, st.dim.Render("(empty)"))
		return
	}

	// One width for every name, so the sizes line up and the eye can run
	// down them.
	width := 0
	for _, e := range l.entries {
		width = max(width, len(e.name)+1)
	}
	for i, e := range l.entries {
		name, size := e.name, humanBytes(e.size)
		if e.isDir {
			name, size = e.name+"/", "dir"
		}
		c.note(
			st,
			st.chip(
				fmt.Sprintf("%2d", i+1),
			)+fmt.Sprintf(
				" %-*s  ",
				width,
				name,
			)+st.dim.Render(
				size,
			),
		)
	}
	if hidden > 0 {
		c.note(st, st.dim.Render(fmt.Sprintf("... and %d more, not shown", hidden)))
	}
}

// sendFile offers a file in the background, so the conversation carries on
// while it transfers. Progress arrives as messages every progressStep percent.
func (c *conversation) sendFile(st *styles, arg string) tea.Cmd {
	if arg == "" {
		c.alert(st, "which file? /send <path>, or /send <number> after /files")
		return nil
	}

	full, err := fileFromArg(c.files, arg)
	if err != nil {
		c.alert(st, reason(err))
		return nil
	}

	c.note(st, st.dim.Render("offering "+full+", waiting for them to accept..."))

	l, ctx, send := c.l, c.ctx, c.send
	return func() tea.Msg {
		last := -1
		progress := func(sentBytes, total int64) {
			step := percent(sentBytes, total) / progressStep
			if step <= last {
				return
			}
			last = step
			send(sending{name: full, received: sentBytes, total: total})
		}
		if err := l.s.SendFile(ctx, full, progress); err != nil {
			return sendFileFailed{name: full, err: err}
		}
		return sent{name: full}
	}
}

// dirFromArg turns what was typed after /files into a directory: nothing
// means the last listing or where homa is; a number picks a directory out
// of the last listing, the same way /send picks a file; anything else is a
// name resolved under the last listing, so a line that was just shown can
// simply be typed.
func dirFromArg(last *listing, arg string) (string, error) {
	lastDir := ""
	if last != nil {
		lastDir = last.dir
	}

	if arg == "" {
		if lastDir != "" {
			return lastDir, nil
		}
		return ".", nil
	}

	if n, err := strconv.Atoi(arg); err == nil {
		path, e, ok := last.path(n)
		if !ok {
			return "", fmt.Errorf("ui: there is no %d in the last listing", n)
		}
		if !e.isDir {
			return "", fmt.Errorf("ui: %s is a file; /send %d sends it", e.name, n)
		}
		return path, nil
	}

	full, err := underListing(lastDir, arg)
	if err != nil {
		return "", err
	}
	// Naming a file here is somebody who has just read a listing and is
	// reaching for one of its lines; the command they wanted is one word
	// away and worth saying.
	if st, err := os.Stat(full); err == nil && !st.IsDir() {
		return "", fmt.Errorf("ui: %s is a file; /send %s sends it", arg, arg)
	}
	return full, nil
}

// fileFromArg turns what was typed after /send into a path. A number means
// a line from the last listing; anything else is a name under it. So a file
// actually named "2" cannot be sent as "/send 2" — "/send ./2" is how, and
// that is the price of not needing a flag to tell the two apart.
func fileFromArg(last *listing, arg string) (string, error) {
	n, err := strconv.Atoi(arg)
	if err != nil {
		lastDir := ""
		if last != nil {
			lastDir = last.dir
		}
		full, err := underListing(lastDir, arg)
		if err != nil {
			return "", err
		}
		// A directory opens and stats like a file and fails only when it
		// is read, which is after the offer has gone and the far side is
		// waiting on it. Refuse it here, naming the command that was
		// wanted, the way the opposite mistake is refused in dirFromArg.
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			return "", fmt.Errorf(
				"ui: %s is a directory; /files %s lists it, an archive sends it",
				arg,
				arg,
			)
		}
		return full, nil
	}

	path, e, ok := last.path(n)
	if !ok {
		return "", fmt.Errorf("ui: there is no %d in the last listing; /files to make one", n)
	}
	if e.isDir {
		return "", fmt.Errorf(
			"ui: %s is a directory; /files %d lists it, an archive sends it",
			e.name,
			n,
		)
	}
	return path, nil
}

// errNoOffer is returned when /accept or /reject is typed with nothing
// waiting to be answered.
var errNoOffer = errors.New("no file is waiting for an answer")
