package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// callBar is the line under the body while a call is on the way in or
// out. It is not a screen: the menu stays behind it, and it goes when the
// call is answered, refused, given up or runs out.
//
// At most one call is on the bar. A second caller while one is waiting is
// turned away as busy, as in version 1; a caller who arrives during a
// conversation waits on the bar, unseen, until the conversation ends.
type callBar struct {
	incoming *line     // a caller waiting for y or n
	outgoing string    // the name being called, or ""
	deadline time.Time // when whichever it is runs out

	// cancel gives up an outgoing call: it ends the wait for the far side
	// to answer, which reports back as the call being stopped by us.
	cancel context.CancelFunc
}

func (b *callBar) showing() bool { return b.incoming != nil || b.outgoing != "" }

func (b *callBar) clear() {
	if b.cancel != nil {
		b.cancel()
	}
	*b = callBar{}
}

// view draws the bar for this moment, or nothing.
func (b *callBar) view(st *styles, now time.Time) string {
	left := b.deadline.Sub(now).Round(time.Second)
	if left < 0 {
		left = 0 // "-1s" is a countdown nobody trusts again
	}

	switch {
	case b.incoming != nil:
		return markInfo + st.peer(b.incoming.name) + " is calling" + st.sep() +
			st.dim.Render(left.String()) + "        " +
			st.you.Render("y") + st.dim.Render(" take it") + st.dim.Render(" · ") +
			st.you.Render("n") + st.dim.Render(" not now")

	case b.outgoing != "":
		return markInfo + st.dim.Render("calling ") + st.peer(b.outgoing) +
			st.dim.Render(st.sep()+"waiting for them to answer"+st.sep()+left.String()+st.sep()) +
			st.you.Render("Enter") + st.dim.Render(" to give up")
	}
	return ""
}

// tick redraws the bar once a second while it is showing. It is issued
// when the bar appears and again on each tick until it goes.
func tick() tea.Cmd {
	return tea.Tick(countdownStep, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// sayCall formats a notice about a call, with the name painted as theirs.
func sayCall(st *styles, format, name string) string {
	return fmt.Sprintf(format, st.peer(name))
}
