package contacts

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// validate checks one contact.
func (c Contact) validate() error {
	if err := validateName(c.Name); err != nil {
		return err
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("contact %q has no address", c.Name)
	}
	return nil
}

// validate checks the whole book, including that no two contacts share a
// name. A duplicate would make ByName ambiguous and a menu misleading.
func (b *Book) validate() error {
	seen := make(map[string]struct{}, len(b.list))

	for _, c := range b.list {
		if err := c.validate(); err != nil {
			return err
		}

		key := strings.ToLower(strings.TrimSpace(c.Name))
		if _, dup := seen[key]; dup {
			return fmt.Errorf("two contacts are both named %q", c.Name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// validateName keeps a nickname printable and bounded. Unlike a display
// name, this one never leaves the machine, but it is drawn in menus, so a
// control character in it would still scramble the screen.
func validateName(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return errors.New("contact name is empty")
	case !utf8.ValidString(name):
		return errors.New("contact name is not valid UTF-8")
	case utf8.RuneCountInString(name) > MaxNameLen:
		return fmt.Errorf("contact name is longer than %d characters", MaxNameLen)
	}

	for _, r := range name {
		if unicode.IsControl(r) {
			return errors.New("contact name contains a control character")
		}
	}
	return nil
}
