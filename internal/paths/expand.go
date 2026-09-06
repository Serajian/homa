package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome turns a leading ~ into the home directory. A person typing a
// path at a prompt writes ~/Downloads and expects it to work; the shell does
// that expansion for a command line, but nothing does it for text read from
// a prompt.
func ExpandHome(path string) (string, error) {
	path = strings.TrimSpace(path)

	sep := string(filepath.Separator)
	if path != "~" && !strings.HasPrefix(path, "~"+sep) {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("paths: expanding %q: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// EnsureDir expands a leading ~ in path, creates the directory if it does
// not exist, and returns its absolute path. It is for directories a person
// names, which arrive as typed text and may not exist yet.
func EnsureDir(path string) (string, error) {
	dir, err := ExpandHome(path)
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("paths: empty directory")
	}

	// Resolve before creating, so the directory and the path we hand back
	// are the same one even if the process later changes directory.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("paths: resolving %q: %w", dir, err)
	}

	if err := os.MkdirAll(abs, DirPerm); err != nil {
		return "", fmt.Errorf("paths: creating %s: %w", abs, err)
	}
	return abs, nil
}
