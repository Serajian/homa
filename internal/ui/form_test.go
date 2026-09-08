package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func press(f *form, st *styles, keys ...string) (done, cancel bool) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		default:
			for _, r := range k {
				done, cancel = f.update(st, tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		done, cancel = f.update(st, msg)
		if done || cancel {
			return done, cancel
		}
	}
	return done, cancel
}

// Enter on an empty field takes the default, shown in brackets, so
// changing one setting means pressing Enter past the rest.
func TestAFormWalksItsFieldsAndTakesDefaults(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	f := newForm("settings",
		field{label: "The name shown beside your messages", def: "homa"},
		field{label: "Where received files should go", def: "~/homa-files"},
	)
	if done, _ := press(f, st, "alice", "enter"); done {
		t.Fatal("done after the first field")
	}
	done, cancel := press(f, st, "enter")
	if !done || cancel {
		t.Fatalf("done=%v cancel=%v after the last field", done, cancel)
	}
	if got := f.answers(); got[0] != "alice" || got[1] != "~/homa-files" {
		t.Errorf("answers = %q", got)
	}
}

// A field that fails its check says why, under the field, and stays.
func TestAFormSaysWhatWasWrongAndStays(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	f := newForm("settings", field{label: "name", check: func(s string) error {
		if s == "" {
			return errors.New("config: display name is empty")
		}
		return nil
	}})
	if done, _ := press(f, st, "enter"); done {
		t.Fatal("an empty answer was accepted")
	}
	if got := stripANSI(f.view(st)); !strings.Contains(got, "display name is empty") {
		t.Errorf("no reason shown:\n%s", got)
	}
	if done, _ := press(f, st, "bob", "enter"); !done {
		t.Error("a valid answer was not accepted")
	}
}

// Backing out is a decision, not a failure: Esc leaves everything alone.
func TestEscCancelsAForm(t *testing.T) {
	t.Parallel()

	f := newForm("add", field{label: "A name for them"})
	if _, cancel := press(f, newStyles(true), "x", "esc"); !cancel {
		t.Error("Esc did not cancel")
	}
}

// A form that demands a word — reset, forget — is done only with that word;
// anything else, Enter included, cancels.
func TestAWordFormNeedsTheWord(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	f := newForm("reset", field{label: "type the word reset to confirm", def: "cancel", word: "reset"})
	done, cancel := press(f, st, "enter")
	if done || !cancel {
		t.Errorf("Enter alone: done=%v cancel=%v; want cancel", done, cancel)
	}
	f = newForm("reset", field{label: "type the word reset to confirm", def: "cancel", word: "reset"})
	done, cancel = press(f, st, "reset", "enter")
	if !done || cancel {
		t.Errorf("the word: done=%v cancel=%v; want done", done, cancel)
	}
}
