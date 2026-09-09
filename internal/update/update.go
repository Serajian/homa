// Package update asks GitHub for the latest release and says whether it
// is newer than the running one. It never downloads anything: how homa
// was installed decides how it is upgraded, and this package only says
// which command that is.
//
// It is asked, never asks by itself. homa talks to nothing but the relay
// on its own; a request to GitHub is one the person makes by pressing a
// key, and it says so on the screen.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultURL is the GitHub API endpoint that describes the latest release.
const DefaultURL = "https://api.github.com/repos/Serajian/homa/releases/latest"

// timeout bounds one check: the API answers in well under a second, and a
// person is waiting on the screen.
const timeout = 10 * time.Second

// Result is what a check found.
type Result struct {
	Current string // the running version, as stamped at build time
	Latest  string // the latest release's tag, "v0.2.1"
	URL     string // the release page
	Newer   bool   // Latest is newer than Current
	Known   bool   // Current parsed as a release version; false for a build from source
}

// Checker asks one URL. The zero value asks GitHub with the default
// client; tests point URL at a server of their own.
type Checker struct {
	URL    string
	Client *http.Client
}

// Check compares current with the latest release.
func (c Checker) Check(ctx context.Context, current string) (Result, error) {
	url, client := c.URL, c.Client
	if url == "" {
		url = DefaultURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return Result{}, fmt.Errorf("update: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "homa/"+current)

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("update: reaching GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("update: GitHub answered %s", resp.Status)
	}

	var rel struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Result{}, fmt.Errorf("update: reading GitHub's answer: %w", err)
	}
	if rel.Tag == "" {
		return Result{}, errors.New("update: GitHub's answer names no release")
	}

	r := Result{Current: current, Latest: rel.Tag, URL: rel.URL}
	latest, ok := parse(rel.Tag)
	if !ok {
		return Result{}, fmt.Errorf("update: %q is not a release version", rel.Tag)
	}
	cur, known := parse(current)
	r.Known = known
	r.Newer = known && less(cur, latest)
	return r, nil
}

// semver is the three numbers of a release tag.
type semver [3]int

// parse reads the leading vX.Y.Z of a version. A build from source is
// stamped with what git describes, "v0.2.0-3-gfb441be-dirty", whose
// leading part still parses; "dev" does not.
func parse(v string) (semver, bool) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var s semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		s[i] = n
	}
	return s, true
}

func less(a, b semver) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// Advice is the command that upgrades, guessed from where the binary
// lives: Homebrew keeps casks under Caskroom, apt puts the binary in
// /usr/bin, and anything else came from an archive or go install.
func Advice(exe string) string {
	switch {
	case strings.Contains(exe, "/Caskroom/"):
		return "brew update && brew upgrade --cask homa"
	case runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/bin/"):
		return "sudo apt update && sudo apt install homa"
	case strings.Contains(exe, "/go/bin/"):
		return "go install github.com/Serajian/homa/cmd/homa@latest"
	}
	return "download the archive for your system from the release page"
}
