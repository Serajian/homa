package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path so that a crash can never leave a
// half-written file. It writes a temporary file in the same directory and
// renames it over the target: rename within a directory is atomic, so a
// reader sees either the old file or the new one, never a partial one.
//
// The file is created with FilePerm.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("paths: creating temporary file in %s: %w", dir, err)
	}
	tmp := f.Name()

	// Clean up on any path that does not reach the rename, panics included.
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp)
	}()

	if err = f.Chmod(FilePerm); err != nil {
		return fmt.Errorf("paths: setting permissions on %s: %w", tmp, err)
	}
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("paths: writing %s: %w", tmp, err)
	}
	// Flush to the disk before renaming, so a power cut cannot leave a
	// renamed but empty file.
	if err = f.Sync(); err != nil {
		return fmt.Errorf("paths: flushing %s: %w", tmp, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("paths: closing %s: %w", tmp, err)
	}
	if err = os.Rename(tmp, path); err != nil {
		return fmt.Errorf("paths: replacing %s: %w", path, err)
	}
	return nil
}
