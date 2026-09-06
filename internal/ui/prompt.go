package ui

import (
	"fmt"
	"strconv"
	"strings"
)

// Ask asks a question and returns the answer. An empty answer takes def,
// which is shown in brackets so the person knows what pressing Enter does.
// Pass an empty def to require an answer.
func (u *UI) Ask(question, def string) (string, error) {
	for {
		if def != "" {
			u.Printf("%s%s [%s]: ", markPrompt, question, def)
		} else {
			u.Printf("%s%s: ", markPrompt, question)
		}

		answer, err := u.ReadLine()
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

// Confirm asks a yes or no question. def is what Enter alone means, and is
// shown capitalized in the hint the way command line tools have done for
// decades: [Y/n] or [y/N].
func (u *UI) Confirm(question string, def bool) (bool, error) {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}

	for {
		u.Printf("%s%s [%s]: ", markPrompt, question, hint)

		answer, err := u.ReadLine()
		if err != nil {
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

// Choose shows a numbered list and returns the index of what was picked.
//
// Options are numbered from one because that is how people count, while the
// returned index starts at zero because that is how Go indexes. The
// translation happens here, once, rather than at every call site.
func (u *UI) Choose(question string, options []string) (int, error) {
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
		answer, err := u.ReadLine()
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

// ChooseKeyed shows a list where each entry has a letter or number to type,
// for menus whose entries are not simply numbered: "n) new contact".
//
// It returns the key that was typed, lowercased. Keys are compared
// case-insensitively, since nobody wants to be told they pressed Q instead
// of q.
func (u *UI) ChooseKeyed(question string, keys, labels []string) (string, error) {
	if len(keys) == 0 || len(keys) != len(labels) {
		return "", fmt.Errorf("ui: a menu needs one label per key")
	}

	valid := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		valid[strings.ToLower(k)] = struct{}{}
	}

	for {
		u.Blank()
		u.Printf("%s", question)
		for i, k := range keys {
			u.Printf("  %s) %s", k, labels[i])
		}

		u.Printf("%schoice: ", markPrompt)
		answer, err := u.ReadLine()
		if err != nil {
			return "", err
		}

		answer = strings.ToLower(answer)
		if _, ok := valid[answer]; ok {
			return answer, nil
		}

		u.Warn("that is not one of the choices")
	}
}
