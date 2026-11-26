package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/brohd11/godot-gdsh-cli/gdshcmd"
)

const (
	editorUsage = "Usage: gdsh editor [<godot args>...]  open the enclosing project in its Godot editor"
	runUsage    = "Usage: gdsh run [<scene>] [<godot args>...]  run the enclosing project, or a scene, in its Godot"
)

// editorCmd opens the enclosing project in its Godot editor and returns at once.
func editorCmd(args []string, stdout, stderr io.Writer) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, editorUsage)
		return 0
	}
	return launchGodot(append([]string{"--editor"}, args...), true, stdout, stderr)
}

// runCmd runs the enclosing project (or the scene in args) in its Godot, in the foreground.
func runCmd(args []string, stdout, stderr io.Writer) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, runUsage)
		return 0
	}
	return launchGodot(args, false, stdout, stderr)
}

func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help")
}

// launchGodot starts the enclosing project's Godot with --path and args. Detached, it starts
// Godot in its own process group with no terminal and returns; otherwise Godot shares the
// terminal and its exit code comes back.
func launchGodot(args []string, detach bool, stdout, stderr io.Writer) int {
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
	if detach {
		godot = guiExe(godot)
	}
	cmd := exec.Command(godot, append([]string{"--path", project}, args...)...)

	if detach {
		setDetached(cmd)
		if err := cmd.Start(); err != nil {
			return fail(fmt.Errorf("starting %s: %w", godot, err))
		}
		cmd.Process.Release()
		fmt.Fprintf(stdout, "Opened %s in %s\n", project, godot)
		return 0
	}

	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	// Ctrl-C reaches Godot too; outlive it so its exit code comes back.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		if code := exit.ExitCode(); code > 0 {
			return code
		}
		return 1
	case err != nil:
		return fail(fmt.Errorf("running %s: %w", godot, err))
	}
	return 0
}

// guiExe swaps a registered Windows _console.exe for its GUI sibling, so a detached editor
// leaves no console window behind.
func guiExe(godot string) string {
	lower := strings.ToLower(godot)
	if !strings.HasSuffix(lower, "_console.exe") {
		return godot
	}
	gui := godot[:len(godot)-len("_console.exe")] + ".exe"
	if gdshcmd.FileExists(gui) {
		return gui
	}
	return godot
}
