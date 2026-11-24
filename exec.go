package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/brohd11/gdsh/gdshcmd"
	"github.com/brohd11/gdsh/payload"
)

func runFile(path string, stdout, stderr io.Writer) int {
	abs, err := filepath.Abs(path)
	if err == nil && !gdshcmd.FileExists(abs) {
		err = fmt.Errorf("no such file: %s", path)
	}
	if err != nil {
		fmt.Fprintln(stderr, "gdsh:", err)
		return 1
	}
	return runGdsh([]string{"-f", abs}, stdout, stderr)
}

// runGdsh runs boot.gd with args (-c LINE or -f FILE) in the enclosing project and returns
// Godot's exit code.
func runGdsh(args []string, stdout, stderr io.Writer) int {
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

	out := &bannerWriter{w: stdout}
	cmd := exec.Command(godot, append([]string{"--headless", "--path", project, "--script",
		filepath.Join(cli, "boot.gd"), "--"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, stderr

	// Ctrl-C reaches Godot too; outlive it so its exit code comes back. Notify, not Ignore:
	// an ignored signal stays ignored in the child.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	err = cmd.Run()
	out.Flush()
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

// bannerWriter drops Godot's "Godot Engine v…" first line and the blank line after it.
type bannerWriter struct {
	w     io.Writer
	buf   []byte
	state int // 0: reading line 1, 1: dropping a blank line 2, 2: passing through
}

func (b *bannerWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 && b.state < 2 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			b.buf = append(b.buf, p...)
			return n, nil
		}
		line := append(b.buf, p[:i]...)
		b.buf, p = nil, p[i+1:]
		switch {
		case b.state == 0 && bytes.HasPrefix(line, []byte("Godot Engine v")):
			b.state = 1
		case b.state == 1 && len(bytes.TrimRight(line, "\r")) == 0:
			b.state = 2
		default:
			b.state = 2
			if _, err := b.w.Write(append(line, '\n')); err != nil {
				return n, err
			}
		}
	}
	if len(p) > 0 {
		if _, err := b.w.Write(p); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Flush writes a first line that never got its newline.
func (b *bannerWriter) Flush() {
	if len(b.buf) > 0 {
		b.w.Write(b.buf)
		b.buf = nil
	}
}
