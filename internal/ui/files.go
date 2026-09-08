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

	// ".." goes on the front, after the cap, so a way back is never the
	// thing that got cut and never has to win a sort against a name
	// starting with punctuation. The root has no parent and gets none.
	if parent := filepath.Dir(full); parent != full {
		entries = append([]entry{{name: "..", isDir: true}}, entries...)
	}

	return &listing{dir: full, entries: entries}, hidden, nil
}

// underListing expands ~ and resolves a relative name against the directory
// last listed.
//
// Both /files and /send go through it, and they have to: a listing that shows
// "notes.md" is an invitation to type it, and it would be a poor one if one
// command took it from where you are looking and the other from wherever homa
// was started. They were briefly out of step, and homa suggested "/send
// notes.md" as the fix for a mistake while /send could not resolve it either.
func underListing(last, arg string) (string, error) {
	full, err := paths.ExpandHome(arg)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(full) && last != "" {
		full = filepath.Join(last, full)
	}
	return full, nil
}
