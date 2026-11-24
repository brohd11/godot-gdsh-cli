package gdshcmd

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gdshpkg "github.com/brohd11/gdsh-pkg"
)

// testHost is the gdsh CLI's host: gdsh's own roots, nothing to add folders with.
func testHost() *Host {
	return &Host{
		Prog: "gdsh", Name: "gdsh",
		Root:    func(project string) (string, error) { return GdshRoot(project), nil },
		Folders: GdshFolders,
		Try:     func(line string) string { return "gdsh '" + line + "'" },
		Listed:  "gdsh lists them on its next run.",
	}
}

// env points ~/.gdsh at a temp folder and runs from a temp working folder.
func env(t *testing.T) (home string) {
	t.Helper()
	home = filepath.Join(t.TempDir(), "gdsh")
	t.Setenv("GDSH_HOME", home)
	t.Chdir(t.TempDir())
	return home
}

func run(t *testing.T, h *Host, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := h.Run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.godot"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHelpAndDispatch(t *testing.T) {
	h := testHost()
	h.Extra = []Verb{{Names: []string{"add", "remove"}, Usage: []string{"add DIR", "remove DIR"}, Help: "      Edit {prog} folders.\n",
		Run: func(string, []string, io.Writer) error { return nil }}}
	out, _, code := run(t, h, "--help")
	for _, want := range []string{"  gdsh new NAME", "  gdsh dirs\n", "  gdsh add DIR\n  gdsh remove DIR\n", "Edit gdsh folders."} {
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
	if _, errOut, code := run(t, h, "bogus"); code != 2 || !strings.Contains(errOut, `unknown command "bogus"`) {
		t.Errorf("unknown: %d %q", code, errOut)
	}
	if _, _, code := run(t, h); code != 2 {
		t.Errorf("no args: %d", code)
	}
}

func TestNew(t *testing.T) {
	home := env(t)
	h := testHost()
	commands := filepath.Join(home, "commands")

	out, errOut, code := run(t, h, "new", "hello")
	if code != 0 {
		t.Fatalf("new: %d %s", code, errOut)
	}
	hello := filepath.Join(commands, "hello", "hello.gd")
	src := read(t, hello)
	for _, want := range []string{
		`extends "res://addons/_lib/gdsh/src/core/command_base.gd"`,
		`return "hello"`, "Usage: hello [ARG...]", `&"positional_count": "min:0"`, "func _execute(ctx:Context):",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("command missing %q:\n%s", want, src)
		}
	}
	// The user commands folder is read without config, so there is no hint.
	if !strings.Contains(out, "Created "+hello) || strings.Contains(out, "doesn't read") || !strings.Contains(out, "gdsh 'hello --help'") {
		t.Errorf("new output: %q", out)
	}

	if _, errOut, code = run(t, h, "new", "hello"); code != 1 || !strings.Contains(errOut, "--force") {
		t.Errorf("existing command: %d %q", code, errOut)
	}
	os.WriteFile(hello, []byte("edited"), 0o644)
	if _, _, code = run(t, h, "new", "hello", "--force"); code != 0 || read(t, hello) == "edited" {
		t.Errorf("--force: %d", code)
	}

	for _, args := range [][]string{{"new"}, {"new", "a", "--bogus"}, {"new", "a", "--dir"}, {"new", "a", "--project", "--dir", "x"}} {
		if _, _, code = run(t, h, args...); code != 2 {
			t.Errorf("%v: code %d, want usage 2", args, code)
		}
	}
	if _, errOut, code = run(t, h, "new", "bad/name"); code != 1 || !strings.Contains(errOut, "invalid command name") {
		t.Errorf("bad name: %d %q", code, errOut)
	}
}

func TestNewSubcommands(t *testing.T) {
	commands := filepath.Join(env(t), "commands")
	h := testHost()

	if _, errOut, code := run(t, h, "new", "tools", "bake"); code != 0 {
		t.Fatalf("new tools bake: %d %s", code, errOut)
	}
	parent := filepath.Join(commands, "tools", "tools.gd")
	if src := read(t, parent); !strings.Contains(src, `return "tools"`) || !strings.Contains(src, "Usage: tools <subcommand>") || strings.Contains(src, "_execute") {
		t.Errorf("parent:\n%s", src)
	}
	if src := read(t, filepath.Join(commands, "tools", "bake", "bake.gd")); !strings.Contains(src, `return "bake"`) || !strings.Contains(src, "Usage: tools bake [ARG...]") {
		t.Errorf("subcommand:\n%s", src)
	}

	// An existing parent is kept, even without --force.
	os.WriteFile(parent, []byte("mine"), 0o644)
	if out, errOut, code := run(t, h, "new", "tools", "paint"); code != 0 || read(t, parent) != "mine" || strings.Contains(out, parent) {
		t.Errorf("second subcommand: %d %q %q", code, out, errOut)
	}

	// A single-file command can't take subcommands until it has its own folder.
	os.WriteFile(filepath.Join(commands, "loose.gd"), nil, 0o644)
	if _, errOut, code := run(t, h, "new", "loose", "sub"); code != 1 || !strings.Contains(errOut, "single-file command") {
		t.Errorf("loose parent: %d %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(commands, "loose")); err == nil {
		t.Error("a refused new left files behind")
	}
}

func TestNewFolders(t *testing.T) {
	env(t)
	h := testHost()
	if _, errOut, code := run(t, h, "new", "x", "--project"); code != 1 || !strings.Contains(errOut, "project.godot") {
		t.Errorf("--project outside a project: %d %q", code, errOut)
	}

	project := newProject(t)
	t.Chdir(project)
	if _, errOut, code := run(t, h, "new", "x", "--project"); code != 0 {
		t.Fatalf("--project: %d %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(project, ".gdsh", "commands", "x", "x.gd")); err != nil {
		t.Error(err)
	}

	// Without AddCmd, a folder that isn't read points at `dirs`.
	out, errOut, code := run(t, h, "new", "y", "--dir", "lib")
	if code != 0 {
		t.Fatalf("--dir: %d %s", code, errOut)
	}
	lib, _ := filepath.Abs("lib")
	if _, err := os.Stat(filepath.Join(lib, "y", "y.gd")); err != nil {
		t.Error(err)
	}
	if !strings.Contains(out, "gdsh doesn't read "+lib+"; it reads the folders 'gdsh dirs' lists.") {
		t.Errorf("--dir output: %q", out)
	}

	// With AddCmd, the hint is the line that adds it; a folder Folders lists gets none.
	h.AddCmd = "prog add"
	if out, _, _ = run(t, h, "new", "z", "--dir=lib"); !strings.Contains(out, "Add it with: prog add "+lib) {
		t.Errorf("AddCmd hint: %q", out)
	}
	h.Folders = func() []Folder { return append(GdshFolders(), Folder{lib, "extra"}) }
	if out, _, _ = run(t, h, "new", "w", "--dir=lib"); strings.Contains(out, "doesn't read") {
		t.Errorf("read folder still hinted: %q", out)
	}
}

// Each root lists its packages/<name>/cmd after commands/, skipping folders without cmd/.
func TestDirs(t *testing.T) {
	home := env(t)
	for _, d := range []string{filepath.Join(home, "packages", "level", "cmd"), filepath.Join(home, "packages", "notes")} {
		os.MkdirAll(d, 0o755)
	}
	project := newProject(t)
	t.Chdir(project)
	out, _, _ := run(t, testHost(), "dirs")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[gdsh user commands folder]  (missing)", "[gdsh user package level]", "[gdsh project commands folder]  (missing)"}
	if len(lines) != len(want) {
		t.Fatalf("dirs:\n%s", out)
	}
	for i, w := range want {
		if !strings.HasSuffix(lines[i], w) {
			t.Errorf("line %d: want %s:\n%s", i, w, out)
		}
	}
	if _, _, code := run(t, testHost(), "dirs", "x"); code != 2 {
		t.Errorf("dirs x: %d", code)
	}
}

func pkgZip(t *testing.T, name, version string, require ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"package.cfg":                      "[package]\nversion=\"" + version + "\"\npath=\"addons/_lib/gdsh_lib/" + name + "\"\nrequire=[\"" + strings.Join(require, `","`) + "\"]\n",
		"cmd/manifest.gd":                  "extends RefCounted\n",
		"cmd/" + name + "/" + name + ".gd": "# command\n",
	}
	for p, body := range files {
		w, _ := zw.Create("repo-sha/" + p)
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

// fakeIndex serves level, terrain3d (needs level) and utils (project only) to h.
func fakeIndex(t *testing.T, h *Host) {
	t.Helper()
	index := `{"version": 1, "packages": [
  {"name": "level", "repo": "o/level", "summary": "Level building", "tags": ["3d"], "tag": "v0.1.0", "commit": "c1", "requires": []},
  {"name": "terrain3d", "repo": "o/terrain3d", "summary": "Terrain tools", "tags": ["3d"], "tag": "v0.1.0", "commit": "c2", "requires": ["o/level"]},
  {"name": "utils", "repo": "o/utils", "summary": "cat, ls", "tag": "v0.4.0", "commit": "c3", "requires": [], "project_only": ["o/util-r"]}
]}`
	zips := map[string][]byte{
		"/o/level/c1.zip":     pkgZip(t, "level", "0.1.0"),
		"/o/terrain3d/c2.zip": pkgZip(t, "terrain3d", "0.1.0", "o/level@v0.1.0"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manifest.json" {
			w.Write([]byte(index))
		} else if z, ok := zips[r.URL.Path]; ok {
			w.Write(z)
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	h.Client = &gdshpkg.Client{
		ManifestURL: srv.URL + "/manifest.json",
		ArchiveURL:  func(repo, commit string) string { return srv.URL + "/" + repo + "/" + commit + ".zip" },
		CacheDir:    t.TempDir(),
	}
}

func TestPackages(t *testing.T) {
	packages := filepath.Join(env(t), "packages")
	h := testHost()
	fakeIndex(t, h)

	out, errOut, code := run(t, h, "search", "3d")
	if code != 0 || !strings.Contains(out, "terrain3d") || !strings.Contains(out, "needs level") || strings.Contains(out, "utils") ||
		!strings.Contains(out, "Install with: gdsh install NAME") {
		t.Fatalf("search: %d %q %q", code, out, errOut)
	}
	if out, _, _ = run(t, h, "search"); !strings.Contains(out, "project only: gdaddon install o/utils") {
		t.Errorf("search all: %q", out)
	}

	out, errOut, code = run(t, h, "install", "terrain3d")
	if code != 0 || !strings.Contains(out, "level is needed by terrain3d") || !strings.Contains(out, "Installed terrain3d v0.1.0") ||
		!strings.Contains(out, "gdsh lists them on its next run.") {
		t.Fatalf("install: %d %q %q", code, out, errOut)
	}
	for _, p := range []string{"level/cmd/level/level.gd", "terrain3d/package.cfg"} {
		if _, err := os.Stat(filepath.Join(packages, p)); err != nil {
			t.Error(err)
		}
	}
	if out, _, _ = run(t, h, "install", "level"); !strings.Contains(out, "level 0.1.0 is already installed") {
		t.Errorf("reinstall: %q", out)
	}
	if out, _, _ = run(t, h, "packages"); !strings.Contains(out, "level") || !strings.Contains(out, "terrain3d") || strings.Contains(out, "update:") {
		t.Errorf("packages: %q", out)
	}
	if out, _, _ = run(t, h, "update"); !strings.Contains(out, "pinned version") {
		t.Errorf("update with nothing outdated: %q", out)
	}

	// An older level shows as outdated and updates.
	os.WriteFile(filepath.Join(packages, "level", "package.cfg"), []byte("[package]\nversion=\"0.0.9\"\npath=\"x/level\"\n"), 0o644)
	if out, _, _ = run(t, h, "packages"); !strings.Contains(out, "(update: v0.1.0)") || !strings.Contains(out, "Update with: gdsh update") {
		t.Errorf("outdated: %q", out)
	}
	if out, _, _ = run(t, h, "update"); !strings.Contains(out, "Updated level v0.1.0") {
		t.Errorf("update: %q", out)
	}
	if _, errOut, code = run(t, h, "update", "nope"); code != 1 || !strings.Contains(errOut, "install it with: gdsh install nope") {
		t.Errorf("update missing: %d %q", code, errOut)
	}

	if _, errOut, code = run(t, h, "uninstall", "level"); code != 1 || !strings.Contains(errOut, "needed by terrain3d") {
		t.Errorf("uninstall a dependency: %d %q", code, errOut)
	}
	if out, _, code = run(t, h, "uninstall", "terrain3d"); code != 0 || !strings.Contains(out, "Removed terrain3d") {
		t.Errorf("uninstall: %d %q", code, out)
	}
	if _, errOut, code = run(t, h, "install", "utils"); code != 1 || !strings.Contains(errOut, "gdaddon install o/utils") {
		t.Errorf("project-only: %d %q", code, errOut)
	}
	for _, args := range [][]string{{"install"}, {"uninstall"}, {"packages", "x"}, {"install", "level", "--bogus"}, {"install", "x", "--project", "--dir", "y"}} {
		if _, _, code = run(t, h, args...); code != 2 {
			t.Errorf("%v: code %d, want usage 2", args, code)
		}
	}
}

func TestPackagesRoots(t *testing.T) {
	home := env(t)
	h := testHost()
	fakeIndex(t, h)

	if _, errOut, code := run(t, h, "install", "level"); code != 0 {
		t.Fatalf("install into ~/.gdsh: %s", errOut)
	}
	if _, err := os.Stat(filepath.Join(home, "packages", "level", "package.cfg")); err != nil {
		t.Error(err)
	}

	project := newProject(t)
	t.Chdir(project)
	if _, errOut, code := run(t, h, "install", "level", "--project"); code != 0 {
		t.Fatalf("install --project: %s", errOut)
	}
	if _, err := os.Stat(filepath.Join(project, ".gdsh", "packages", "level")); err != nil {
		t.Error(err)
	}

	// A root that isn't read gets the hint instead.
	out, _, code := run(t, h, "install", "level", "--dir", "elsewhere")
	if code != 0 || !strings.Contains(out, "doesn't read") {
		t.Errorf("--dir: %d %q", code, out)
	}

	// A Root error stops before anything is written.
	h.Root = func(string) (string, error) { return "", os.ErrPermission }
	if _, errOut, code := run(t, h, "install", "level"); code != 1 || !strings.Contains(errOut, "permission") {
		t.Errorf("root error: %d %q", code, errOut)
	}
	h.Root = func(string) (string, error) { return "", nil }
	if _, errOut, code := run(t, h, "new", "x"); code != 1 || !strings.Contains(errOut, "GDSH_HOME") {
		t.Errorf("no home: %d %q", code, errOut)
	}
}
