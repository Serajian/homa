package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// view draws the bar for this moment, or nothing: a box, the one place the
// interface draws in the far side's color, because a call is the one
// moment that needs the whole screen's attention.
func (b *callBar) view(st *styles, width int, now time.Time) string {
	left := b.deadline.Sub(now).Round(time.Second)
	if left < 0 {
		left = 0 // "-1s" is a countdown nobody trusts again
	}
	inner := width - 4 - 2 - 1 // the box's inside, less its padding

	switch {
	case b.incoming != nil:
		who := " " + st.peer(b.incoming.name) + " is calling"
		who += strings.Repeat(
			" ",
			max(inner-lipgloss.Width(who)-lipgloss.Width(left.String())-1, 0),
		) + st.dim.Render(
			left.String(),
		)
		return "  " + strings.ReplaceAll(st.box(st.boxThem(), width-4, "incoming call",
			who, " "+st.keys("y", "take the call", "n", "not now")), "\n", "\n  ")

	case b.outgoing != "":
		who := " " + st.dim.Render(
			"calling ",
		) + st.peer(
			b.outgoing,
		) + st.dim.Render(
			st.sep()+"waiting for them to answer",
		)
		who += strings.Repeat(
			" ",
			max(inner-lipgloss.Width(who)-lipgloss.Width(left.String())-1, 0),
		) + st.dim.Render(
			left.String(),
		)
		return "  " + strings.ReplaceAll(st.box(st.boxDim(), width-4, "calling",
			who, " "+st.keys("Enter", "give up")), "\n", "\n  ")
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
