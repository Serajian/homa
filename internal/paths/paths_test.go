package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sandbox(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	return dir
}

func TestDirIsCreatedAndPrivate(t *testing.T) {
	sandbox(t)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}

	st, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("the directory was not created: %v", err)
	}
	if perm := st.Mode().Perm(); perm != DirPerm {
		t.Errorf(
			"mode = %o, want %o: everything in here is a secret or a private list",
			perm,
			DirPerm,
		)
	}
}

// File takes a bare name on purpose. Everything lives in one flat directory,
// so a name with a separator in it is a bug rather than a request for a
// subdirectory — and it is the shape a path traversal would arrive in.
func TestFileRefusesAnythingButAPlainName(t *testing.T) {
	sandbox(t)

	for _, name := range []string{"", ".", "..", "a/b", "../escape", "/etc/passwd"} {
		if _, err := File(name); err == nil {
			t.Errorf("File(%q) was allowed", name)
		}
	}
}

func TestFileIsUnderTheConfigDirectory(t *testing.T) {
	sandbox(t)

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	p, err := File("config.json")
	if err != nil {
		t.Fatal(err)
	}

	if got := filepath.Dir(p); got != dir {
		t.Errorf("File landed in %q, want %q", got, dir)
	}
}

// A crash must never leave a half-written file: a reader sees the old one or
// the new one, never something in between. The rename is what guarantees it,
// so what is checked here is that the target is only ever whole.
func TestWriteAtomicReplacesWithoutAHalfwayState(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")

	if err := WriteAtomic(p, []byte("first")); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if got := read(t, p); got != "first" {
		t.Errorf("content = %q, want %q", got, "first")
	}

	if err := WriteAtomic(p, []byte("second, and longer")); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	if got := read(t, p); got != "second, and longer" {
		t.Errorf("content = %q, want the replacement", got)
	}
}

func TestWriteAtomicLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()

	for range 5 {
		if err := WriteAtomic(filepath.Join(dir, "x.json"), []byte("data")); err != nil {
			t.Fatalf("writing: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only the target", names)
	}
}

func TestWriteAtomicSetsPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")

	if err := WriteAtomic(p, []byte("data")); err != nil {
		t.Fatalf("writing: %v", err)
	}

	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != FilePerm {
		t.Errorf("mode = %o, want %o", perm, FilePerm)
	}
}

// A file that fails to write must not take the old one with it. The
// temporary file is created in the target's directory, so a directory that
// cannot be written to is the failure to try.
func TestWriteAtomicIntoAnUnwritableDirectoryLeavesTheOldFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a directory with no write bit")
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")

	if err := WriteAtomic(p, []byte("original")); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := WriteAtomic(p, []byte("replacement")); err == nil {
		t.Error("writing into an unwritable directory reported success")
	}
	if got := read(t, p); got != "original" {
		t.Errorf("content = %q, want the original still there", got)
	}
}

func TestRemove(t *testing.T) {
	sandbox(t)

	p, err := File("gone.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(p, []byte("data")); err != nil {
		t.Fatal(err)
	}

	if err := Remove("gone.json"); err != nil {
		t.Fatalf("removing: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("the file is still there")
	}

	// A file that is not there is not an error: the caller wanted it gone.
	if err := Remove("gone.json"); err != nil {
		t.Errorf("removing what was already gone: %v", err)
	}
}

// Deleting is the one operation where being handed the wrong path cannot be
// undone, so the same refusal File makes has to hold here.
func TestRemoveRefusesAnythingButAPlainName(t *testing.T) {
	dir := sandbox(t)

	victim := filepath.Join(dir, "important")
	if err := os.WriteFile(victim, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"../important", "/etc/passwd", "..", ""} {
		if err := Remove(name); err == nil {
			t.Errorf("Remove(%q) was allowed", name)
		}
	}

	if _, err := os.Stat(victim); err != nil {
		t.Errorf("a file outside the config directory was removed: %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	home := sandbox(t)

	cases := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/homa-files", filepath.Join(home, "homa-files")},
		{"/tmp/absolute", "/tmp/absolute"},
	}

	for _, tc := range cases {
		got, err := ExpandHome(tc.in)
		if err != nil {
			t.Fatalf("ExpandHome(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// Only "~" and "~/" are expanded. "~other" is left exactly as typed
	// rather than resolved to somebody else's home: it becomes a literal
	// directory name, which is the safe reading of an ambiguous one.
	if got, err := ExpandHome("~other/files"); err != nil || got != "~other/files" {
		t.Errorf("ExpandHome(%q) = %q, %v; want it untouched", "~other/files", got, err)
	}
}

func TestDisplayNeverFails(t *testing.T) {
	sandbox(t)

	// It goes inside error messages, so it has to produce something even
	// for a name File would refuse.
	if got := Display("../nope"); got == "" || !strings.Contains(got, "nope") {
		t.Errorf("Display of a bad name = %q, want something showable", got)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
