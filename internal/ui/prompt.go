package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Ask asks a question and returns the answer. An empty answer takes def,
// which is shown in brackets so the person knows what pressing Enter does.
// Pass an empty def to require an answer.
func (u *UI) Ask(ctx context.Context, question, def string) (string, error) {
	for {
		if def != "" {
			u.Printf("%s%s %s: ", u.promptMark(), question, u.dim("["+def+"]"))
		} else {
			u.Printf("%s%s: ", u.promptMark(), question)
		}

		answer, err := u.ReadLine(ctx)
		if err != nil {
			return "", err
		}

		switch {
		case answer != "":
			return answer, nil
		case def != "":
			return def, nil
		}

		u.Warn("an answer is needed")
	}
}

// Choose shows a numbered list and returns the index of what was picked.
//
// Options are numbered from one because that is how people count, while the
// returned index starts at zero because that is how Go indexes. The
// translation happens here, once, rather than at every call site.
func (u *UI) Choose(ctx context.Context, question string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("ui: nothing to choose from")
	}

	for {
		u.Blank()
		u.Printf("%s", question)
		for i, opt := range options {
			u.Printf("  %d) %s", i+1, opt)
		}

		u.Printf("%schoice: ", u.promptMark())
		answer, err := u.ReadLine(ctx)
		if err != nil {
			return 0, err
		}

		n, convErr := strconv.Atoi(answer)
		if convErr != nil || n < 1 || n > len(options) {
			u.Warn("enter a number between 1 and %d", len(options))
			continue
		}
		return n - 1, nil
	}
}

// menuItem is one line of a menu: the key to press and what it does.
type menuItem struct {
	key  string
	text string // what the key does; holds one %s when name is set

	// name is a peer's name to be painted into text as one, so the
	// people on a menu look like people everywhere else.
	name string

	// quiet makes the line recede: leaving, and anything that cannot be
	// undone, should not weigh the same as calling somebody.
	quiet bool
}

// ShowMenu prints a keyed list and stops there. It does not read the
// answer, which is the difference between it and Choose.
//
// The groups are separated by a blank line and nothing else: what belongs
// together sits together, and space says so better than a rule would. Keys
// are padded to the widest so the labels line up. The whole menu is one
// write, so a message arriving from a conversation cannot land inside it.
//
// The main menu has to wait on the keyboard and on an arriving call at the
// same time, and only a select can do that, so App.menuLoop does its own
// reading from Lines. Printing still lives here, with the other things a
// person is shown.
func (u *UI) ShowMenu(question string, groups ...[]menuItem) error {
	width, count := 0, 0
	for _, g := range groups {
		for _, it := range g {
			count++
			width = max(width, len(it.key))
		}
	}
	if count == 0 {
		return fmt.Errorf("ui: an empty menu")
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(u.you(question))
	b.WriteString("\n")

	first := true
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false

		for _, it := range g {
			key := fmt.Sprintf("%-*s", width, it.key)
			text := it.text
			if it.name != "" {
				text = fmt.Sprintf(it.text, u.peer(it.name))
			}

			if it.quiet {
				b.WriteString(markInfo + u.dim(key+"  "+text))
			} else {
				b.WriteString(markInfo + u.you(key) + "  " + text)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(u.promptMark())
	u.Printf("%s", b.String())
	return nil
}

// ConfirmBy asks a yes or no question that runs out, showing the time left and
// redrawing it every second.
//
// Unlike Confirm it draws with Prompt rather than Printf, so the countdown
// replaces itself and the answer is typed on the same line as the question.
// The cost is that anything already typed leaves the screen on each redraw: it
// is still in the terminal's buffer and will still be sent, but a person who
// typed "y" and waited a second no longer sees it. Nothing in Go can read those
// characters back to redraw them; a full-screen interface owns the input line
// and is what fixes it.
//
// When the deadline passes the caller's ctx is what ends the read, and the
// question is left on the screen rather than erased, so it is clear what ran
// out.
func (u *UI) ConfirmBy(
	ctx context.Context,
	question string,
	def bool,
	deadline time.Time,
) (bool, error) {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}

	for {
		stop := u.countdown(deadline, func(left time.Duration) string {
			return fmt.Sprintf("%s%s %s %s: ", u.promptMark(), question,
				u.dim("["+left.String()+"]"), u.dim("["+hint+"]"))
		})

		answer, err := u.ReadLine(ctx)
		stop()

		if err != nil {
			u.EndPrompt()
			return false, err
		}

		switch strings.ToLower(answer) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}

		u.Warn("answer y or n")
	}
}
