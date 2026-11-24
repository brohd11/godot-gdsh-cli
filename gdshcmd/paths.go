package gdshcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	CommandsDir = "commands"
	PackagesDir = "packages"
)

// Folder is a command folder and why it is read, for `dirs`.
type Folder struct {
	Path, Source string
}

// Home is gdsh's shared user root: $GDSH_HOME, else ~/.gdsh ("" without a home folder), as
// GDSh.Load.home_dir.
func Home() string {
	if dir := os.Getenv("GDSH_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gdsh")
}

// GdshRoot is gdsh's own root: Home, or <project>/.gdsh when project is set.
func GdshRoot(project string) string {
	if project != "" {
		return filepath.Join(project, ".gdsh")
	}
	return Home()
}

// GdshFolders are the folders of gdsh's own roots: Home, then this project's .gdsh when run
// inside one.
func GdshFolders() []Folder {
	var out []Folder
	if home := Home(); home != "" {
		out = append(out, RootFolders(home, "gdsh user")...)
	}
	if project, err := FindProject(""); err == nil {
		out = append(out, RootFolders(GdshRoot(project), "gdsh project")...)
	}
	return out
}

// RootFolders is GDSh.Load.root_dirs: commands/ (listed even when missing, as where new
// commands go), then each packages/<name>/cmd that exists, sorted.
func RootFolders(root, label string) []Folder {
	out := []Folder{{filepath.Join(root, CommandsDir), label + " commands folder"}}
	entries, err := os.ReadDir(filepath.Join(root, PackagesDir))
	if err != nil {
		return out
	}
	for _, e := range entries { // ReadDir sorts by name
		cmd := filepath.Join(root, PackagesDir, e.Name(), "cmd")
		if e.IsDir() && DirExists(cmd) {
			out = append(out, Folder{cmd, label + " package " + e.Name()})
		}
	}
	return out
}

// FindProject returns the folder holding project.godot: dir (default: the working
// directory) or the nearest parent.
func FindProject(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for d := abs; ; d = filepath.Dir(d) {
		if FileExists(filepath.Join(d, "project.godot")) {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("no project.godot in %s or its parents", abs)
		}
	}
}

// ResolveDir expands $VARS and a leading ~, and makes dir absolute against base.
func ResolveDir(dir, base string) string {
	dir = os.ExpandEnv(dir)
	if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[1:])
		}
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(base, dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// SameDir compares by path, then by identity (symlinks, case-insensitive file systems).
func SameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

var errNoHome = errors.New("no home folder for ~/.gdsh: set GDSH_HOME or use --dir")

// root is the root a verb works on: dir when given, else Host.Root for the user or this project.
func (h *Host) root(dir string, project bool) (string, error) {
	if dir != "" {
		return ResolveDir(dir, "."), nil
	}
	p := ""
	if project {
		var err error
		if p, err = FindProject(""); err != nil {
			return "", err
		}
	}
	root, err := h.Root(p)
	if err == nil && root == "" {
		err = errNoHome
	}
	return root, err
}
