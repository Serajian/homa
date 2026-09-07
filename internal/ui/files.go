package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Serajian/homa/internal/paths"
)

// entry is one thing in a listing: enough to show it and to send it.
type entry struct {
	name  string
	size  int64
	isDir bool
}

// listing is the last directory shown with /files, kept so /send can take a
// number out of it.
//
// It holds the directory it read rather than only the names, so picking a
// number sends the file that was listed even if the working directory has
// moved since.
type listing struct {
	dir     string
	entries []entry
}

// path returns the full path of the nth entry, counting from one the way the
// listing is numbered, and whether there is one.
func (l *listing) path(n int) (string, entry, bool) {
	if l == nil || n < 1 || n > len(l.entries) {
		return "", entry{}, false
	}
	e := l.entries[n-1]
	return filepath.Join(l.dir, e.name), e, true
}

// readDir lists one directory, returning what to show and how much was left
// out by the cap.
//
// It does not recurse. A listing is a thing a person reads before picking a
// number out of it, and a recursive one is neither.
//
// Hidden files are skipped. Nothing here needs them, and a home directory
// full of dotfiles would push the files somebody is looking for off the
// screen.
func readDir(dir string) (*listing, int, error) {
	full, err := paths.ExpandHome(dir)
	if err != nil {
		return nil, 0, err
	}

	des, err := os.ReadDir(full)
	if err != nil {
		return nil, 0, fmt.Errorf("ui: reading %s: %w", full, err)
	}

	var entries []entry
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}

		e := entry{name: de.Name(), isDir: de.IsDir()}
		if !e.isDir {
			// A file that cannot be stated is still worth listing; it
			// just has no size to show. Refusing the whole listing over
			// one unreadable entry would be worse.
			if info, err := de.Info(); err == nil {
				e.size = info.Size()
			}
		}
		entries = append(entries, e)
	}

	// Directories first, then files, both by name: walking into somewhere
	// is what the listing is for as often as sending out of it.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return entries[i].name < entries[j].name
	})

	hidden := 0
	if len(entries) > maxListing {
		hidden = len(entries) - maxListing
		entries = entries[:maxListing]
	}

	return &listing{dir: full, entries: entries}, hidden, nil
}

// showFiles lists a directory and remembers it, so /send can take a number
// from what was shown.
func (a *App) showFiles(h *chatHandler, dir string) {
	if dir == "" {
		dir = "."
	}

	l, hidden, err := readDir(dir)
	if err != nil {
		a.ui.Warn("%v", trimUIPrefix(err))
		return
	}

	h.setListing(l)

	a.ui.Blank()
	a.ui.Info("%s", l.dir)

	if len(l.entries) == 0 {
		a.ui.Info("  (empty)")
		return
	}

	// One width for every name, so the sizes line up and the eye can run
	// down them.
	width := 0
	for _, e := range l.entries {
		if n := len(e.name) + 1; n > width {
			width = n
		}
	}

	for i, e := range l.entries {
		name, size := e.name, humanBytes(e.size)
		if e.isDir {
			name, size = e.name+"/", "dir"
		}
		a.ui.Info("%3d) %-*s  %s", i+1, width, name, size)
	}

	if hidden > 0 {
		a.ui.Info("  ... and %d more, not shown", hidden)
	}
}

// trimUIPrefix drops this package's prefix from an error before showing it,
// the way trimSessionPrefix does for the layer below.
func trimUIPrefix(err error) string {
	const prefix = "ui: "

	s := err.Error()
	return strings.TrimPrefix(s, prefix)
}
