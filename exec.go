package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/brohd11/godot-gdsh-cli/gdshcmd"
	"github.com/brohd11/godot-gdsh-cli/payload"
)

func runFile(path string, engineVerbose bool, stdout, stderr io.Writer) int {
	abs, err := filepath.Abs(path)
	if err == nil && !gdshcmd.FileExists(abs) {
		err = fmt.Errorf("no such file: %s", path)
	}
	if err != nil {
		fmt.Fprintln(stderr, "gdsh:", err)
		return 1
	}
	return runGdsh([]string{"-f", abs}, engineVerbose, stdout, stderr)
}

// runGdsh runs boot.gd with args (-c LINE or -f FILE) in the enclosing project and returns
// Godot's exit code. The command's output comes back through --result files, so Godot's own
// logging (banner, exit leak reports, load errors) is kept apart: shown with engineVerbose,
// or when boot.gd never wrote a result.
func runGdsh(args []string, engineVerbose bool, stdout, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(stderr, "gdsh:", err)
		return 1
	}
	project, err := gdshcmd.FindProject("")
	if err != nil {
		return fail(err)
	}
	godot, err := findGodot(project)
	if err != nil {
		return fail(err)
	}
	home := gdshcmd.Home()
	if home == "" {
		return fail(errors.New("no home folder for ~/.gdsh: set GDSH_HOME"))
	}
	cli, err := payload.Ensure(home)
	if err != nil {
		return fail(err)
	}
	result, err := os.MkdirTemp("", "gdsh-result-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(result)

	var engine bytes.Buffer
	cmd := exec.Command(godot, append([]string{"--headless", "--path", project, "--script",
		filepath.Join(cli, "boot.gd"), "--", "--result", result}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, &engine, &engine
	if engineVerbose {
		// Godot's own output goes to stderr, so stdout stays the command's.
		cmd.Stdout, cmd.Stderr = stderr, stderr
	}

	// Ctrl-C reaches Godot too; outlive it so its exit code comes back. Notify, not Ignore:
	// an ignored signal stays ignored in the child.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	err = cmd.Run()
	code := 0
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		if code = exit.ExitCode(); code <= 0 {
			code = 1
		}
	case err != nil:
		return fail(fmt.Errorf("running %s: %w", godot, err))
	}

	out, outErr := os.ReadFile(filepath.Join(result, "stdout"))
	errText, errErr := os.ReadFile(filepath.Join(result, "stderr"))
	if outErr != nil || errErr != nil {
		// boot.gd never got to write: Godot's output is the only explanation.
		stderr.Write(engine.Bytes())
		if code == 0 {
			code = 1
		}
		return code
	}
	stdout.Write(out)
	stderr.Write(errText)
	if code != 0 && !engineVerbose && loggedErrors(engine.Bytes()) {
		fmt.Fprintln(stderr, "gdsh: Godot logged errors; rerun with --engine-verbose to see them")
	}
	return code
}

// loggedErrors reports whether Godot logged an error or warning other than its exit report of
// leaked objects and resources, which nearly every run prints.
func loggedErrors(log []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(log))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "ERROR:") && !strings.HasPrefix(line, "SCRIPT ERROR:") && !strings.HasPrefix(line, "WARNING:") {
			continue
		}
		if !strings.Contains(line, "leaked at exit") && !strings.Contains(line, "still in use at exit") {
			return true
		}
	}
	return false
}
