package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Serajian/homa/internal/contacts"
)

// contactsScreen lists the address book and acts on one entry at a time.
//
// It exists because the address book could be added to and called from and
// nothing else: a name typed in a hurry was a name forever, and a contact who
// had gone could not be removed.
//
// It reports whether the person is leaving homa, so quitting from in here
// works the way it does anywhere else.
func (a *App) contactsScreen(ctx context.Context) (quit bool) {
	for {
		list := a.book.All()
		if len(list) == 0 {
			a.ui.Blank()
			a.ui.Info("no contacts yet. n at the menu adds one.")
			return false
		}

		keys, labels := contactEntries(list)
		if err := a.ui.ShowMenu("contacts", keys, labels); err != nil {
			return false
		}

		line, err := a.ui.ReadLine(ctx)
		if err != nil {
			return errors.Is(err, ErrCanceled) && ctx.Err() != nil
		}

		switch line {
		case "":
			continue // a bare Enter is somebody looking again
		case "b", "back":
			return false
		case "q", "quit":
			return true
		}

		n, convErr := strconv.Atoi(line)
		if convErr != nil || n < 1 || n > len(list) {
			a.ui.Warn("that is not one of the choices")
			continue
		}

		if quit := a.contactActions(ctx, list[n-1]); quit {
			return true
		}
	}
}

// contactEntries numbers the address book and adds the two ways out.
func contactEntries(list []contacts.Contact) (keys, labels []string) {
	for i, c := range list {
		keys = append(keys, strconv.Itoa(i+1))
		labels = append(labels, fmt.Sprintf("%-16s %s", c.Name, preview(c.Addr)))
	}
	return append(keys, "b", "q"), append(labels, "back", "quit homa")
}

// contactActions offers what can be done with one contact.
//
// The contact is passed by value and looked up again by name where it
// matters, because a rename here changes the name this was found under.
func (a *App) contactActions(ctx context.Context, c contacts.Contact) (quit bool) {
	for {
		keys := []string{"c", "r", "a", "f", "b", "q"}
		labels := []string{
			"call " + c.Name,
			"rename",
			"show their address",
			"forget",
			"back",
			"quit homa",
		}

		if err := a.ui.ShowMenu(c.Name, keys, labels); err != nil {
			return false
		}

		line, err := a.ui.ReadLine(ctx)
		if err != nil {
			return errors.Is(err, ErrCanceled) && ctx.Err() != nil
		}

		switch line {
		case "":
			continue
		case "b", "back":
			return false
		case "q", "quit":
			return true

		case "c", "call":
			a.dial(ctx, c)
			return false

		case "a", "address":
			a.ui.Blank()
			a.ui.Info("%s", c.Name)
			a.ui.Printf("%s", c.Addr)
			a.ui.Blank()

		case "r", "rename":
			renamed, ok := a.renameContact(ctx, c)
			if !ok {
				return false // the input ended
			}
			c = renamed

		case "f", "forget":
			done, ok := a.forgetContact(ctx, c)
			if !ok {
				return false
			}
			if done {
				return false // there is nothing left to act on
			}

		default:
			a.ui.Warn("that is not one of the choices")
		}
	}
}

// renameContact changes the local name, returning the contact as it now is
// and whether the screen may carry on.
//
// The name is this machine's, not the peer's. Somebody calling themselves
// "babak" is a good reason to rename "BB", but it stays a decision the person
// makes: a peer able to rename their own entry in an address book could
// rename it to anything.
func (a *App) renameContact(ctx context.Context, c contacts.Contact) (contacts.Contact, bool) {
	name, err := a.ui.Ask(ctx, "a new name for them", c.Name)
	if err != nil {
		return c, false
	}

	if name == c.Name {
		return c, true
	}

	if err := a.book.Rename(c.Name, name); err != nil {
		a.ui.Warn("%v", reason(err))
		return c, true
	}
	if err := a.book.Save(); err != nil {
		a.ui.Warn("could not save the address book: %v", err)
	}

	a.ui.Info("%s is now %s.", c.Name, name)

	c.Name = name
	return c, true
}

// forgetContact removes a contact after asking, reporting whether it is gone
// and whether the screen may carry on.
func (a *App) forgetContact(ctx context.Context, c contacts.Contact) (done, ok bool) {
	a.ui.Blank()
	a.ui.Warn("Forgetting %s takes their address and their key with them.", c.Name)
	a.ui.Warn("Their next call arrives under the name they choose for themselves,")
	a.ui.Warn("and reaching them again means pasting their address in again.")

	answer, err := a.ui.Ask(ctx, "type the word forget to confirm", "cancel")
	if err != nil {
		return false, false
	}
	if answer != "forget" {
		a.ui.Info("%s is still there.", c.Name)
		return false, true
	}

	if err := a.book.Remove(c.Name); err != nil {
		a.ui.Warn("%v", reason(err))
		return false, true
	}
	if err := a.book.Save(); err != nil {
		a.ui.Warn("could not save the address book: %v", err)
	}

	a.ui.Info("%s is forgotten.", c.Name)
	return true, true
}
