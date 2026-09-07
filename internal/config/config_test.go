package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Serajian/homa/internal/paths"
)

// sandbox points the config directory at a temporary one. Both variables are
// set because os.UserConfigDir reads XDG_CONFIG_HOME on Linux and HOME on
// macOS, and a test that only set one would pass on a single platform.
func sandbox(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	return dir
}

func TestSaveAndLoad(t *testing.T) {
	sandbox(t)

	want := &Config{Nick: "alice", DownloadDir: "/tmp/homa-files", AutoListen: true}
	if err := want.Save(); err != nil {
		t.Fatalf("saving: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if *got != *want {
		t.Errorf("loaded %+v, want %+v", *got, *want)
	}
}

// A first run has nothing saved, and the caller needs to be able to tell that
// from a real failure so it knows to ask the questions.
func TestLoadingBeforeSetupSaysSo(t *testing.T) {
	sandbox(t)

	_, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCorruptFileIsReportedWithSomethingToDo(t *testing.T) {
	sandbox(t)
	p := write(t, "{not json")

	_, err := Load()
	if err == nil {
		t.Fatal("loaded settings from broken JSON")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error does not name the file: %v", err)
	}
	if !strings.Contains(err.Error(), "delete it") {
		t.Errorf("error does not say what to do: %v", err)
	}
}

// Valid JSON can still hold settings homa will not run on, and the check has
// to happen on the way in as well as on the way out.
func TestValidJSONWithBadSettingsIsRejected(t *testing.T) {
	sandbox(t)
	write(t, `{"nick":"","download_dir":"/tmp/x"}`)

	if _, err := Load(); err == nil {
		t.Fatal("loaded settings with an empty display name")
	}
}

func TestSaveRefusesSettingsItCouldNotLoadBack(t *testing.T) {
	sandbox(t)

	cases := []struct {
		name string
		cfg  Config
	}{
		{"no name", Config{Nick: "", DownloadDir: "/tmp/x"}},
		{"a name of only spaces", Config{Nick: "   ", DownloadDir: "/tmp/x"}},
		{"a control character in the name", Config{Nick: "a\x1bb", DownloadDir: "/tmp/x"}},
		{
			"a name longer than the limit",
			Config{Nick: strings.Repeat("a", MaxNickLen+1), DownloadDir: "/tmp/x"},
		},
		{"no download directory", Config{Nick: "alice", DownloadDir: ""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Save(); err == nil {
				t.Error("saved settings that would not load back")
			}
		})
	}
}

// A newline in a nick would break the line it is printed on, and a nick is
// announced to a peer, so the limit is not cosmetic.
func TestNickLengthIsCountedInCharactersNotBytes(t *testing.T) {
	sandbox(t)

	// Persian letters are two bytes each: a byte-based limit would reject
	// half as many as it should.
	cfg := Config{Nick: strings.Repeat("ب", MaxNickLen), DownloadDir: "/tmp/x"}

	if err := cfg.Save(); err != nil {
		t.Errorf("rejected a name of exactly %d characters: %v", MaxNickLen, err)
	}
}

func TestDefaultsAreUsable(t *testing.T) {
	sandbox(t)

	if err := Default().Save(); err != nil {
		t.Errorf("the settings a first run starts from do not validate: %v", err)
	}
}

// The file holds no secrets, but it sits in a directory that does, and the
// permissions are the same everywhere so one wrong file cannot go unnoticed.
func TestSavedFileIsPrivate(t *testing.T) {
	sandbox(t)

	if err := Default().Save(); err != nil {
		t.Fatalf("saving: %v", err)
	}

	p, err := paths.File(configFile)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != paths.FilePerm {
		t.Errorf("mode = %o, want %o", perm, paths.FilePerm)
	}
}

func TestRemoveTakesTheFileAndNotTheDirectory(t *testing.T) {
	sandbox(t)

	if err := Default().Save(); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if err := Remove(); err != nil {
		t.Fatalf("removing: %v", err)
	}

	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Errorf("after removing, err = %v, want ErrNotFound", err)
	}

	// Removing again is not an error: the caller wanted it gone, and it is.
	if err := Remove(); err != nil {
		t.Errorf("removing what was already gone: %v", err)
	}

	dir, err := paths.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the config directory went with the file: %v", err)
	}
}

func write(t *testing.T, content string) string {
	t.Helper()

	p, err := paths.File(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
