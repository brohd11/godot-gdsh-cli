package payload

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure(t *testing.T) {
	home := t.TempDir()
	dir, err := Ensure(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"boot.gd", "src/run.gd", "package.cfg", stampFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
	if Version() == "unknown" {
		t.Error("no embedded version")
	}

	// A current copy is left alone; a stale one is replaced whole.
	marker := filepath.Join(dir, "mine")
	os.WriteFile(marker, nil, 0o644)
	if _, err := Ensure(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("a current copy was re-unpacked")
	}
	os.WriteFile(filepath.Join(dir, stampFile), []byte("old\n"), 0o644)
	if _, err := Ensure(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a stale copy was kept")
	}
	if entries, _ := os.ReadDir(home); len(entries) != 1 {
		t.Errorf("staging left behind: %v", entries)
	}
}
