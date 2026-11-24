package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

func TestBannerWriter(t *testing.T) {
	for in, want := range map[string]string{
		"Godot Engine v4.4.1 - https://godotengine.org\n\nhello\nworld\n": "hello\nworld\n",
		"Godot Engine v4.4\r\n\r\nhi\r\n":                                 "hi\r\n",
		"Godot Engine v4.4\nnot blank\n":                                  "not blank\n",
		"plain\nGodot Engine v4.4\n":                                      "plain\nGodot Engine v4.4\n",
		"no newline":                                                      "no newline",
	} {
		var out bytes.Buffer
		b := &bannerWriter{w: &out}
		for i := range in { // byte by byte, as a pipe may split it
			b.Write([]byte{in[i]})
		}
		b.Flush()
		if out.String() != want {
			t.Errorf("%q: got %q, want %q", in, out.String(), want)
		}
	}
}

func TestDispatch(t *testing.T) {
	project(t, `"4.4"`)
	if out, _, code := gdsh(t, "--help"); code != 0 || !strings.Contains(out, "gdsh -c '<line>'") || !strings.Contains(out, "  gdsh install NAME...") {
		t.Errorf("help: %d %q", code, out)
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

// fakeGodot is a script that prints the banner, its arguments and stdin, and exits with $FAKE_EXIT.
func fakeGodot(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	path := filepath.Join(t.TempDir(), "godot")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo "4.4.1.stable.mono.official.abc"; exit 0; fi
printf 'Godot Engine v4.4.1 - https://godotengine.org\n\n'
for a in "$@"; do echo "arg: $a"; done
[ -f "$GDSH_HOME/cli/boot.gd" ] && echo "payload: ok"
echo "oops" >&2
exit ${FAKE_EXIT:-0}
`
	os.WriteFile(path, []byte(script), 0o755)
	return path
}

func TestRunLine(t *testing.T) {
	dir, home := project(t, `"4.4"`)
	godot := fakeGodot(t)
	t.Setenv("GODOT", godot)

	out, errOut, code := gdsh(t, "echo", "hi")
	if code != 0 || strings.Contains(out, "Godot Engine") || !strings.Contains(out, "payload: ok") || errOut != "oops\n" {
		t.Fatalf("run: %d %q %q", code, out, errOut)
	}
	for _, want := range []string{"arg: --headless", "arg: --path\narg: " + dir, "arg: " + filepath.Join(home, "cli", "boot.gd"), "arg: --\narg: -c\narg: echo hi\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
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
