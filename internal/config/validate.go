package config

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Validate reports whether the settings can actually be used.
func (c *Config) Validate() error {
	if err := validateNick(c.Nick); err != nil {
		return err
	}
	if strings.TrimSpace(c.DownloadDir) == "" {
		return errors.New("download directory is empty")
	}
	return nil
}

// validateNick keeps a display name printable and bounded.
//
// This is a security check, not a matter of taste. A nick travels to the
// peer and is drawn in their terminal. A name containing an escape sequence
// could clear their screen, and one containing a carriage return could
// overwrite earlier lines and forge messages. Refuse the whole class.
func validateNick(nick string) error {
	nick = strings.TrimSpace(nick)
	switch {
	case nick == "":
		return errors.New("display name is empty")
	case !utf8.ValidString(nick):
		return errors.New("display name is not valid UTF-8")
	case utf8.RuneCountInString(nick) > MaxNickLen:
		return fmt.Errorf("display name is longer than %d characters", MaxNickLen)
	}

	for _, r := range nick {
		if unicode.IsControl(r) {
			return errors.New("display name contains a control character")
		}
	}
	return nil
}
