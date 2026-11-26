package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func gdsh(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// project makes a project.godot with features and runs from it, with ~/.gdsh in a temp folder.
func project(t *testing.T, features string) (dir, home string) {
	t.Helper()
	dir, home = t.TempDir(), filepath.Join(t.TempDir(), "gdsh")
	t.Setenv("GDSH_HOME", home)
	t.Setenv("GODOT", "")
	cfg := "config_version=5\n[application]\nconfig/features=PackedStringArray(" + features + ")\n"
	os.WriteFile(filepath.Join(dir, "project.godot"), []byte(cfg), 0o644)
	t.Chdir(dir)
	return dir, home
}

func TestVersionKey(t *testing.T) {
	for in, want := range map[string]string{
		"4.4.1.stable.official.49a5bc7b6\n":     "godot-4.4.1",
		"4.5.stable.mono.official.abc\n":        "godot-4.5.0-mono",
		"Godot Engine\n4.6.2.rc1.mono.custom\n": "godot-4.6.2-mono",
	} {
		if got, err := versionKey(in); err != nil || got != want {
			t.Errorf("%q: %q %v, want %q", in, got, err, want)
		}
	}
	if _, err := versionKey("nope"); err == nil {
		t.Error("no version should fail")
	}
}

func TestFindGodot(t *testing.T) {
	dir, home := project(t, `"4.4", "C#", "Forward Plus"`)
	if _, err := findGodot(dir); err == nil || !strings.Contains(err.Error(), "no Godot 4.4 registered") {
		t.Errorf("empty registry: %v", err)
	}
	registry := filepath.Join(home, registryFile)
	upsert(registry, "godot-4.4.1", "/plain")
	upsert(registry, "godot-4.3.0-mono", "/old")
	if g, _ := findGodot(dir); g != "/plain" {
		t.Errorf("other flavour as fallback: %q", g)
	}
	upsert(registry, "godot-4.4.1-mono", "/mono")
	if g, _ := findGodot(dir); g != "/mono" {
		t.Errorf("matching mono: %q", g)
	}
	upsert(registry, "godot-4.4.1-mono", "/mono2")
	if data, _ := os.ReadFile(registry); strings.Count(string(data), "godot-4.4.1-mono") != 1 {
		t.Errorf("upsert duplicated a key:\n%s", data)
	}
	t.Setenv("GODOT", "/env")
	if g, _ := findGodot(dir); g != "/env" {
		t.Errorf("$GODOT: %q", g)
	}
}

func TestLoggedErrors(t *testing.T) {
	exitReport := "Godot Engine v4.7\n\nWARNING: 64 ObjectDB instances were leaked at exit (run with `--verbose` for details).\n" +
		"   at: cleanup (core/object/object.cpp:2535)\nERROR: 37 resources still in use at exit (run with --verbose for details).\n" +
		"ERROR: 2 RID allocations of type 'N5Image' were leaked at exit.\n"
	for log, want := range map[string]bool{
		"":         false,
		exitReport: false,
		exitReport + "SCRIPT ERROR: Parse Error: Preload file \"x.gd\" does not exist.\n": true,
		"ERROR: GDSh.Load: cannot load command x.gd\n":                                    true,
		"WARNING: something else\n":                                                       true,
		"plain text mentioning ERROR: midline\n":                                          false,
	} {
		if got := loggedErrors([]byte(log)); got != want {
			t.Errorf("%q: %v, want %v", log, got, want)
		}
	}
}

func TestDispatch(t *testing.T) {
	project(t, `"4.4"`)
	if out, _, code := gdsh(t, "--help"); code != 0 || !strings.Contains(out, "gdsh -c '<line>'") || !strings.Contains(out, "  gdsh package install NAME...") ||
		!strings.Contains(out, "gdsh update [--check]") {
		t.Errorf("help: %d %q", code, out)
	}
	// update is the binary's own, not the package verb.
	if out, _, code := gdsh(t, "update", "--help"); code != 0 || !strings.Contains(out, "latest gdsh release") {
		t.Errorf("update --help: %d %q", code, out)
	}
	if out, _, code := gdsh(t, "--version"); code != 0 || !strings.HasPrefix(out, "gdsh dev (gdsh-cli ") {
		t.Errorf("version: %d %q", code, out)
	}
	if _, errOut, code := gdsh(t); code != 1 || !strings.Contains(errOut, "Usage:") {
		t.Errorf("no args: %d %q", code, errOut)
	}
	// A root verb runs here, not in Godot.
	if out, _, code := gdsh(t, "new", "hi"); code != 0 || !strings.Contains(out, "gdsh 'hi --help'") {
		t.Errorf("new: %d %q", code, out)
	}
	if _, errOut, code := gdsh(t, "-f", "missing.gdsh"); code != 1 || !strings.Contains(errOut, "no such file") {
		t.Errorf("-f missing: %d %q", code, errOut)
	}
	if _, errOut, code := gdsh(t, "--bogus"); code != 1 || !strings.Contains(errOut, "unknown flag") {
		t.Errorf("bad flag: %d %q", code, errOut)
	}
	if _, errOut, code := gdsh(t, "ls res://"); code != 1 || !strings.Contains(errOut, "gdsh register") {
		t.Errorf("no Godot: %d %q", code, errOut)
	}
}

// fakeGodot is a script that logs like Godot (banner, exit leak report; a script error with
// $FAKE_ENGINE_ERROR) and, as boot.gd does, writes its arguments to the --result dir's stdout
// and "oops" to its stderr, unless $FAKE_NORESULT. It exits with $FAKE_EXIT.
func fakeGodot(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	path := filepath.Join(t.TempDir(), "godot")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo "4.4.1.stable.mono.official.abc"; exit 0; fi
printf 'Godot Engine v4.4.1 - https://godotengine.org\n\n'
[ -n "$FAKE_ENGINE_ERROR" ] && echo "SCRIPT ERROR: Parse Error: boom" >&2
echo "WARNING: 64 ObjectDB instances were leaked at exit (run with --verbose for details)." >&2
echo "   at: cleanup (core/object/object.cpp:2535)" >&2
result=""; prev=""
for a in "$@"; do [ "$prev" = "--result" ] && result=$a; prev=$a; done
if [ -z "$FAKE_NORESULT" ] && [ -n "$result" ]; then
	{ for a in "$@"; do echo "arg: $a"; done; [ -f "$GDSH_HOME/cli/boot.gd" ] && echo "payload: ok"; } > "$result/stdout"
	echo "oops" > "$result/stderr"
fi
exit ${FAKE_EXIT:-0}
`
	os.WriteFile(path, []byte(script), 0o755)
	return path
}

func TestRunLine(t *testing.T) {
	dir, home := project(t, `"4.4"`)
	godot := fakeGodot(t)
	t.Setenv("GODOT", godot)

	// Only the command's output shows; Godot's banner and exit report don't.
	out, errOut, code := gdsh(t, "echo", "hi")
	if code != 0 || strings.Contains(out, "Godot Engine") || !strings.Contains(out, "payload: ok") || errOut != "oops\n" {
		t.Fatalf("run: %d %q %q", code, out, errOut)
	}
	for _, want := range []string{"arg: --headless", "arg: --path\narg: " + dir, "arg: " + filepath.Join(home, "cli", "boot.gd"),
		"arg: --\narg: --result\n", "arg: -c\narg: echo hi\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// --engine-verbose adds Godot's output on stderr; stdout stays the command's.
	out, errOut, code = gdsh(t, "--engine-verbose", "echo", "hi")
	if code != 0 || strings.Contains(out, "WARNING") || !strings.Contains(errOut, "Godot Engine v4.4.1") ||
		!strings.Contains(errOut, "leaked at exit") || !strings.HasSuffix(errOut, "oops\n") {
		t.Errorf("--engine-verbose: %d %q %q", code, out, errOut)
	}

	// Without a result, Godot's output is the only explanation, and the run fails.
	t.Setenv("FAKE_NORESULT", "1")
	if _, errOut, code = gdsh(t, "echo", "hi"); code != 1 || !strings.Contains(errOut, "Godot Engine") || !strings.Contains(errOut, "leaked at exit") {
		t.Errorf("no result: %d %q", code, errOut)
	}
	t.Setenv("FAKE_NORESULT", "")

	// A failure gets the hint only when Godot logged more than its exit report.
	t.Setenv("FAKE_EXIT", "3")
	if _, errOut, code = gdsh(t, "false"); code != 3 || errOut != "oops\n" {
		t.Errorf("plain failure: %d %q", code, errOut)
	}
	t.Setenv("FAKE_ENGINE_ERROR", "1")
	if _, errOut, code = gdsh(t, "false"); code != 3 || !strings.Contains(errOut, "rerun with --engine-verbose") {
		t.Errorf("failure with engine errors: %d %q", code, errOut)
	}
	t.Setenv("FAKE_EXIT", "")
	if _, errOut, _ = gdsh(t, "true"); strings.Contains(errOut, "--engine-verbose") {
		t.Errorf("hint on success: %q", errOut)
	}
	t.Setenv("FAKE_ENGINE_ERROR", "")
	// -c forces a line, even one that starts with a root verb.
	if out, _, _ = gdsh(t, "-c", "new", "x"); !strings.Contains(out, "arg: -c\narg: new x\n") {
		t.Errorf("-c: %q", out)
	}
	os.WriteFile("s.gdsh", []byte("#!gdsh\n"), 0o644)
	if out, _, _ = gdsh(t, "-f", "s.gdsh"); !strings.Contains(out, "arg: -f\narg: "+filepath.Join(dir, "s.gdsh")) {
		t.Errorf("-f: %q", out)
	}
	t.Setenv("FAKE_EXIT", "3")
	if _, _, code = gdsh(t, "false"); code != 3 {
		t.Errorf("exit code: %d, want 3", code)
	}
}

func TestRegister(t *testing.T) {
	_, home := project(t, `"4.4"`)
	godot := fakeGodot(t)
	out, errOut, code := gdsh(t, "register", godot)
	if code != 0 || !strings.Contains(out, "Registered godot-4.4.1-mono: "+godot) {
		t.Fatalf("register: %d %q %q", code, out, errOut)
	}
	data, _ := os.ReadFile(filepath.Join(home, registryFile))
	if string(data) != "godot-4.4.1-mono: "+godot+"\n" {
		t.Errorf("registry: %q", data)
	}
	if _, errOut, code = gdsh(t, "register", filepath.Join(home, "missing")); code != 1 || !strings.Contains(errOut, "--version failed") {
		t.Errorf("missing bin: %d %q", code, errOut)
	}
}

// fakeLauncher is a script that writes its arguments to $FAKE_ARGS (after $FAKE_SLEEP seconds),
// prints "game output" and exits with $FAKE_EXIT.
func fakeLauncher(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	path := filepath.Join(t.TempDir(), "godot")
	script := `#!/bin/sh
[ -n "$FAKE_SLEEP" ] && sleep "$FAKE_SLEEP"
for a in "$@"; do echo "$a"; done > "$FAKE_ARGS.tmp" && mv "$FAKE_ARGS.tmp" "$FAKE_ARGS"
echo "game output"
exit ${FAKE_EXIT:-0}
`
	os.WriteFile(path, []byte(script), 0o755)
	return path
}

func TestEditorAndRun(t *testing.T) {
	dir, _ := project(t, `"4.4"`)
	for _, verb := range []string{"editor", "run"} {
		if out, _, code := gdsh(t, verb, "--help"); code != 0 || !strings.Contains(out, "gdsh "+verb) {
			t.Errorf("%s --help: %d %q", verb, code, out)
		}
		// No Godot registered for the project.
		if _, errOut, code := gdsh(t, verb); code != 1 || !strings.Contains(errOut, "gdsh register") {
			t.Errorf("%s without Godot: %d %q", verb, code, errOut)
		}
	}

	t.Setenv("GODOT", fakeLauncher(t))
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_ARGS", argsFile)
	os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755)
	t.Chdir(filepath.Join(dir, "sub", "deeper"))

	// run waits, shares the terminal and passes the scene and flags through.
	t.Setenv("FAKE_EXIT", "4")
	out, _, code := gdsh(t, "run", "res://a.tscn", "--x")
	if code != 4 || out != "game output\n" {
		t.Errorf("run: %d %q", code, out)
	}
	if data, _ := os.ReadFile(argsFile); string(data) != "--path\n"+dir+"\nres://a.tscn\n--x\n" {
		t.Errorf("run args: %q", data)
	}
	t.Setenv("FAKE_EXIT", "")
	os.Remove(argsFile)

	// editor returns before Godot finishes.
	t.Setenv("FAKE_SLEEP", "1")
	out, _, code = gdsh(t, "editor")
	if code != 0 || !strings.HasPrefix(out, "Opened "+dir) {
		t.Errorf("editor: %d %q", code, out)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("editor waited for Godot")
	}
	var data []byte
	for range 50 {
		if data, _ = os.ReadFile(argsFile); data != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if string(data) != "--path\n"+dir+"\n--editor\n" {
		t.Errorf("editor args: %q", data)
	}

	// Outside a project.
	t.Chdir(t.TempDir())
	if _, errOut, code := gdsh(t, "run"); code != 1 || !strings.Contains(errOut, "no project.godot") {
		t.Errorf("run outside a project: %d %q", code, errOut)
	}
}
