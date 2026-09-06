package ui

import (
	"github.com/Serajian/homa/internal/config"
)

// Setup asks the first-run questions and saves the answers.
//
// It asks only what homa cannot reasonably guess. Everything else keeps its
// default, because a setup that asks eight questions is a setup people
// answer without reading.
func Setup(u *UI) (*config.Config, error) {
	cfg := config.Default()

	u.Blank()
	u.Info("This is the first run, so two questions.")
	u.Blank()

	nick, err := u.askUntilValid(
		"The name shown beside your messages",
		cfg.Nick,
		func(s string) error {
			candidate := *cfg
			candidate.Nick = s
			return candidate.Validate()
		},
	)
	if err != nil {
		return nil, err
	}
	cfg.Nick = nick

	dir, err := u.Ask("Where received files should go", cfg.DownloadDir)
	if err != nil {
		return nil, err
	}
	cfg.DownloadDir = dir

	if err := cfg.Save(); err != nil {
		return nil, err
	}

	u.Blank()
	u.Info("Saved to %s", config.Path())
	return cfg, nil
}

// EditSettings walks the same questions again with the current values as
// the defaults, so changing one setting means pressing Enter past the rest.
//
// It returns the saved settings rather than editing the ones passed in: the
// caller may be sharing them with other goroutines, and swapping a pointer
// under a lock is safe while writing through one is not.
func EditSettings(u *UI, current *config.Config) (*config.Config, error) {
	edited := *current

	nick, err := u.askUntilValid(
		"The name shown beside your messages",
		edited.Nick,
		func(s string) error {
			candidate := edited
			candidate.Nick = s
			return candidate.Validate()
		},
	)
	if err != nil {
		return nil, err
	}
	edited.Nick = nick

	dir, err := u.Ask("Where received files should go", edited.DownloadDir)
	if err != nil {
		return nil, err
	}
	edited.DownloadDir = dir

	if err := edited.Save(); err != nil {
		return nil, err
	}

	u.Info("Saved.")
	return &edited, nil
}

// askUntilValid keeps asking until the answer passes check. The person sees
// what was wrong with what they typed, rather than the question repeating
// for no visible reason.
func (u *UI) askUntilValid(question, def string, check func(string) error) (string, error) {
	for {
		answer, err := u.Ask(question, def)
		if err != nil {
			return "", err
		}

		if err := check(answer); err != nil {
			u.Warn("%v", trimPrefix(err))
			continue
		}
		return answer, nil
	}
}

// trimPrefix drops the package prefix from an error before showing it. A
// person reading "display name is empty" does not need to know which Go
// package noticed.
func trimPrefix(err error) string {
	const prefix = "config: "

	s := err.Error()
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
