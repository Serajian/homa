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
			u.Printf("%s%s [%s]: ", markPrompt, question, def)
		} else {
			u.Printf("%s%s: ", markPrompt, question)
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

		u.Printf("%schoice: ", markPrompt)
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

// ShowMenu prints a keyed list and stops there. It does not read the
// answer, which is the difference between it and Choose.
//
// The main menu has to wait on the keyboard and on an arriving call at the
// same time, and only a select can do that, so App.menuLoop does its own
// reading from Lines. Printing still lives here, with the other things a
// person is shown.
func (u *UI) ShowMenu(question string, keys, labels []string) error {
	if len(keys) == 0 || len(keys) != len(labels) {
		return fmt.Errorf("ui: a menu needs one label per key")
	}

	u.Blank()
	u.Printf("%s", question)
	for i, k := range keys {
		u.Printf("  %s) %s", k, labels[i])
	}

	u.Printf("%schoice: ", markPrompt)
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
			return fmt.Sprintf("%s%s [%s] [%s]: ", markPrompt, question, left, hint)
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
