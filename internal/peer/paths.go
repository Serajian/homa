package peer

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir returns homa's configuration directory, creating it if needed.
// It honours XDG_CONFIG_HOME and falls back to the platform default:
// ~/.config/homa on Linux, ~/Library/Application Support/homa on macOS.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("peer: locating config directory: %w", err)
	}
	dir := filepath.Join(base, "homa")
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("peer: creating %s: %w", dir, err)
	}
	return dir, nil
}

// KeyPath reports where the identity is stored, for messages to the user.
// It returns a placeholder rather than an error, since it is only ever shown.
func KeyPath() string {
	p, err := keyPath()
	if err != nil {
		return "<unknown>"
	}
	return p
}

func keyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, keyFile), nil
}
