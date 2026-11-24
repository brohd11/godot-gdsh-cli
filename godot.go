package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/brohd11/godot-gdsh-cli/gdshcmd"
)

// Godot binaries are recorded one per line as `godot-<maj>.<min>.<patch>[-mono]: <exe>`.
const registryFile = "executables"

var (
	featureVersion = regexp.MustCompile(`(?m)^config/features=PackedStringArray\("([0-9.]+)"`)
	featureCSharp  = regexp.MustCompile(`(?m)^config/features=.*"C#"`)
)

func registryPath() (string, error) {
	home := gdshcmd.Home()
	if home == "" {
		return "", errors.New("no home folder for ~/.gdsh: set GDSH_HOME")
	}
	return filepath.Join(home, registryFile), nil
}

// findGodot is $GODOT, else the registered binary for project.godot's version, preferring
// the matching mono flavour. It never falls back to another version, which would re-import
// the project's .godot/.
func findGodot(project string) (string, error) {
	if g := os.Getenv("GODOT"); g != "" {
		return g, nil
	}
	cfg, err := os.ReadFile(filepath.Join(project, "project.godot"))
	if err != nil {
		return "", err
	}
	ver, want := "", ""
	if m := featureVersion.FindSubmatch(cfg); m != nil {
		ver = string(m[1])
	}
	if featureCSharp.Match(cfg) {
		want = "-mono"
	}
	registry, err := registryPath()
	if err != nil {
		return "", err
	}
	exact, other := "", ""
	if data, err := os.ReadFile(registry); err == nil && ver != "" {
		for _, line := range strings.Split(string(data), "\n") {
			key, path, ok := strings.Cut(strings.TrimRight(line, "\r"), ": ")
			if !ok || !strings.HasPrefix(key, "godot-"+ver+".") {
				continue
			}
			mono := ""
			if strings.HasSuffix(key, "-mono") {
				mono = "-mono"
			}
			if mono == want {
				exact = path
			} else {
				other = path
			}
		}
	}
	if exact != "" {
		return exact, nil
	}
	if other != "" {
		return other, nil
	}
	if ver == "" {
		ver = "(unknown version)"
	}
	return "", fmt.Errorf("no Godot %s registered in %s\n  register one: gdsh register <path to godot>", ver, registry)
}

// register records each bin under the version it reports, replacing that key's old line.
// Without bins it registers $GODOT, godot on PATH and, on macOS, /Applications/Godot*.app.
func register(bins []string, stdout, stderr io.Writer) int {
	if len(bins) == 1 && (bins[0] == "-h" || bins[0] == "--help") {
		fmt.Fprintln(stdout, "Usage: gdsh register [<godot>...]  record Godot binaries in ~/.gdsh/executables")
		return 0
	}
	if len(bins) == 0 {
		bins = discoverGodot()
	}
	if len(bins) == 0 {
		fmt.Fprintln(stderr, "gdsh: no Godot found; run: gdsh register <path to godot>")
		return 1
	}
	registry, err := registryPath()
	if err != nil {
		fmt.Fprintln(stderr, "gdsh:", err)
		return 1
	}
	status := 0
	for _, bin := range bins {
		key, exe, err := probe(bin)
		if err == nil {
			err = upsert(registry, key, exe)
		}
		if err != nil {
			fmt.Fprintf(stderr, "gdsh: %s: %v\n", bin, err)
			status = 1
			continue
		}
		fmt.Fprintf(stdout, "Registered %s: %s\n", key, exe)
	}
	return status
}

func discoverGodot() []string {
	var bins []string
	seen := map[string]bool{}
	add := func(p string) {
		if abs, err := filepath.Abs(p); err == nil && !seen[abs] {
			seen[abs] = true
			bins = append(bins, abs)
		}
	}
	if g := os.Getenv("GODOT"); g != "" {
		add(g)
	}
	if p, err := exec.LookPath("godot"); err == nil {
		add(p)
	}
	if runtime.GOOS == "darwin" {
		apps, _ := filepath.Glob("/Applications/Godot*.app/Contents/MacOS/Godot")
		for _, a := range apps {
			add(a)
		}
	}
	return bins
}

// probe runs `bin --version` and returns the registry key and the exe to record. On Windows
// the GUI exe doesn't hand a terminal its stdout, so its _console sibling is recorded.
func probe(bin string) (key, exe string, err error) {
	exe, err = filepath.Abs(bin)
	if err != nil {
		return "", "", err
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(exe), "_console.exe") {
		if console := strings.TrimSuffix(exe, filepath.Ext(exe)) + "_console.exe"; gdshcmd.FileExists(console) {
			exe = console
		}
	}
	// Godot loads a project.godot in its cwd, so it runs from an empty folder.
	empty, err := os.MkdirTemp("", "gdsh-register-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(empty)
	cmd := exec.Command(exe, "--version")
	cmd.Dir = empty
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("--version failed: %w", err)
	}
	key, err = versionKey(string(out))
	return key, exe, err
}

// versionKey turns `godot --version` output ("4.4.1.stable.mono.official.49a5bc7b6", or
// "4.5.stable…" with no patch) into "godot-4.4.1-mono".
func versionKey(out string) (string, error) {
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) == 0 {
		return "", errors.New("no version printed")
	}
	fields := strings.Split(lines[len(lines)-1], ".")
	nums := []int{}
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			break
		}
		nums = append(nums, n)
	}
	if len(nums) < 2 {
		return "", fmt.Errorf("can't read a version from %q", strings.TrimSpace(out))
	}
	if len(nums) == 2 {
		nums = append(nums, 0)
	}
	key := fmt.Sprintf("godot-%d.%d.%d", nums[0], nums[1], nums[2])
	for _, f := range fields[len(nums):] {
		if f == "mono" {
			key += "-mono"
			break
		}
	}
	return key, nil
}

// upsert writes key's line last, keeping every other line.
func upsert(registry, key, exe string) error {
	var lines []string
	if data, err := os.ReadFile(registry); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, key+": ") {
				lines = append(lines, line)
			}
		}
	}
	lines = append(lines, key+": "+exe)
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		return err
	}
	return os.WriteFile(registry, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
