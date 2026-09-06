// Package paths knows where homa keeps its files and with what permissions.
// Every package that touches the config directory goes through it, so the
// layout is defined once rather than in each package that stores something.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns homa's configuration directory, creating it if needed. It
// honors XDG_CONFIG_HOME and falls back to the platform default:
// ~/.config/homa on Linux, ~/Library/Application Support/homa on macOS.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("paths: locating config directory: %w", err)
	}

	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return "", fmt.Errorf("paths: creating %s: %w", dir, err)
	}
	return dir, nil
}

// File returns the full path of name inside the config directory, creating
// the directory if needed. The name must be a bare file name: callers store
// files in one flat directory, and a name with a separator in it would be a
// bug rather than a request for a subdirectory.
func File(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, filepath.Separator) || name == "." || name == ".." {
		return "", fmt.Errorf("paths: %q is not a plain file name", name)
	}

	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Display returns the path of name for showing to a person, without touching
// the disk. It never fails, so it is safe inside an error message.
func Display(name string) string {
	p, err := File(name)
	if err != nil {
		return "<" + name + ">"
	}
	return p
}
