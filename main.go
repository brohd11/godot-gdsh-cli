// gdsh runs gdsh lines headless in the enclosing Godot project, and manages the command roots
// gdsh reads (new, install, ...). Godot runs the GDScript half (package payload), unpacked in
// ~/.gdsh/cli; this binary picks the project's Godot and starts it. The root verbs come from
// package gdshcmd, which mcp_sh_godot imports too.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brohd11/godot-gdsh-cli/gdshcmd"
	"github.com/brohd11/godot-gdsh-cli/payload"
	"github.com/brohd11/goutil/selfupdate"
)

// version is set at build time via -ldflags "-X main.version=...". See the makefile.
var version = "dev"

const updateRepo = "brohd11/godot-gdsh-cli"

const usage = `Usage: gdsh '<line>' | gdsh -c '<line>' | gdsh -f <file.gdsh> | gdsh <command> ... (gdsh --help)`

const helpHead = `Usage: gdsh '<line>'                run a gdsh line in the enclosing Godot project
       gdsh -c '<line>'             the same; needed when the line starts with a command below
       gdsh -f <file.gdsh>          run a gdsh script (starts with #!gdsh)
       gdsh register [<godot>...]   record Godot binaries in ~/.gdsh/executables
       gdsh update [--check]        install the latest gdsh release over this binary
       gdsh --version | --help

The line is gdsh syntax, as in the Editor Console, not your shell's: quote it so your shell
passes it whole. Blocks use braces:
  gdsh 'for f in $(ls res://) { echo "file: $f" }'
Output is plain text, so it pipes into your shell's tools:
  gdsh 'ls res://' | grep '/$'

The project's Godot is $GODOT, else the registered binary matching the version in
project.godot (mono when the project uses C#).

Commands for the roots gdsh reads:
`

const helpTail = `
gdsh reads its builtins, the utils library, enabled plugins' cmd/ folders, and two roots:
~/.gdsh ($GDSH_HOME) and <project>/.gdsh. Each root gives its commands/ folder and every
packages/<name>/cmd. The project's own gdsh runs when it has one (addons/_lib/gdsh), else the
copy bundled in ~/.gdsh/cli.
`

// cliHost is gdsh's own roots, which it reads with no config.
func cliHost() *gdshcmd.Host {
	return &gdshcmd.Host{
		Prog: "gdsh", Name: "gdsh",
		Root:    func(project string) (string, error) { return gdshcmd.GdshRoot(project), nil },
		Folders: gdshcmd.GdshFolders,
		Try:     func(line string) string { return "gdsh '" + line + "'" },
		Listed:  "gdsh lists them on its next run; try: gdsh '<command> --help'",
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	host := cliHost()
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, helpHead+host.VerbHelp()+helpTail)
		return 0
	case "--version", "version":
		fmt.Fprintf(stdout, "gdsh %s (gdsh-cli %s)\n", version, payload.Version())
		return 0
	case "register":
		return register(args[1:], stdout, stderr)
	case "update":
		cmd := selfupdate.NewUpdateCommand(updateRepo, "gdsh", version)
		cmd.SetArgs(args[1:])
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			return 1
		}
		return 0
	case "-c":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "gdsh: -c needs a line")
			return 1
		}
		return runGdsh([]string{"-c", strings.Join(args[1:], " ")}, stdout, stderr)
	case "-f":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "gdsh: -f takes one file")
			return 1
		}
		return runFile(args[1], stdout, stderr)
	}
	if _, ok := host.Find(args[0]); ok {
		return host.Run(args, stdout, stderr)
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "gdsh: unknown flag %s\n%s\n", args[0], usage)
		return 1
	}
	return runGdsh([]string{"-c", strings.Join(args, " ")}, stdout, stderr)
}
