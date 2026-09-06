// Package config stores the settings a person chooses: the name shown next
// to their messages and where received files land. It is separate from the
// identity in peer: this is preference, that is cryptography.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Serajian/homa/internal/logx"
	"github.com/Serajian/homa/internal/paths"
)

var logger = logx.For("config")

// ErrNotFound means homa has not been set up on this machine yet, and the
// caller should ask the first-run questions.
var ErrNotFound = errors.New("config: not set up yet")

// Config is the set of choices a person made. Nothing here affects whether
// two peers can talk to each other.
type Config struct {
	// Nick is the name shown beside this machine's messages. A peer sees
	// whatever is written here, so it proves nothing about identity.
	Nick string `json:"nick"`

	// DownloadDir is where accepted files are written.
	DownloadDir string `json:"download_dir"`

	// AutoListen starts accepting incoming connections as soon as homa
	// runs, rather than waiting for the person to ask.
	AutoListen bool `json:"auto_listen"`
}

// Default returns the settings a first run starts from. The caller is
// expected to ask the person about them before saving.
func Default() *Config {
	return &Config{
		Nick:        defaultNick(),
		DownloadDir: defaultDownloadDir(),
		AutoListen:  true,
	}
}

// Path reports where the settings are stored, for messages to the user.
func Path() string { return paths.Display(configFile) }

// Load reads the saved settings. It returns ErrNotFound, which callers
// should check with errors.Is, when homa has not been set up yet.
func Load() (*Config, error) {
	p, err := paths.File(configFile)
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("config: reading %s: %w", p, err)
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config: %s is not valid JSON (%w); "+
			"fix it by hand or delete it to be asked again", p, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("config: %s holds bad settings: %w", p, err)
	}

	logger.Info("settings loaded", "nick", c.Nick, "auto_listen", c.AutoListen)
	return &c, nil
}

// Save validates the settings and writes them to disk.
func (c *Config) Save() error {
	if err := c.Validate(); err != nil {
		return err
	}

	p, err := paths.File(configFile)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encoding settings: %w", err)
	}
	if err := paths.WriteAtomic(p, append(b, '\n')); err != nil {
		return err
	}

	logger.Info("settings saved", "path", p)
	return nil
}

// EnsureDownloadDir creates the download directory if it does not exist and
// returns its absolute path. Call it before a transfer rather than at
// startup, so a path on a drive that is not plugged in fails when it
// matters instead of stopping homa from running at all.
func (c *Config) EnsureDownloadDir() (string, error) {
	return paths.EnsureDir(c.DownloadDir)
}

func defaultNick() string {
	if h, err := os.Hostname(); err == nil && validateNick(h) == nil {
		return h
	}
	return "homa"
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "homa-files"
	}
	return filepath.Join(home, "homa-files")
}
