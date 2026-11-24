// Package gdshcmd holds the command-root verbs (new, search, install, packages, update,
// uninstall, dirs) shared by the gdsh CLI and `mcp_sh_godot gdsh`, so the two can't drift.
// A Host says how its program is typed, where it writes and which folders it reads; a
// program adds its own verbs through Host.Extra.
package gdshcmd

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	gdshpkg "github.com/brohd11/gdsh-pkg"
)

// Host adapts the verbs to one program.
type Host struct {
	Prog string // how the verbs are typed: "gdsh", "mcp_sh_godot gdsh"
	Name string // subject of the hints: "gdsh", "The bridge"
	// AddCmd adds a folder to what Name reads ("mcp_sh_godot gdsh add"); "" when there is none.
	AddCmd string
	// Root is the command root new and install use: the user one, or project's when set.
	Root    func(project string) (string, error)
	Folders func() []Folder          // the command folders Name reads, in its order
	Try     func(line string) string // how to run line through Name, for hints
	Listed  string                   // printed after installing into a root Name reads
	Note    string                   // printed after new; optional
	// Extra verbs follow the shared ones in dispatch and help.
	Extra    []Verb
	HelpTail string          // closing help paragraphs
	Client   *gdshpkg.Client // nil: the published index
}

// Verb is one subcommand. Names share one help entry ("add", "remove").
type Verb struct {
	Names []string
	Usage []string // one line per form, typed after Prog
	Help  string   // indented description; "{prog}" is replaced
	Run   func(name string, args []string, stdout io.Writer) error
}

// UsageError makes Run print the help and exit 2.
type UsageError string

func (e UsageError) Error() string { return string(e) }

// Run dispatches args: 0 ok, 1 failed, 2 usage.
func (h *Host) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, h.Help())
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, h.Help())
		return 0
	}
	var err error = UsageError(fmt.Sprintf("unknown command %q", args[0]))
	if v, ok := h.Find(args[0]); ok {
		err = v.Run(args[0], args[1:], stdout)
	}
	var ue UsageError
	if errors.As(err, &ue) {
		fmt.Fprintf(stderr, "%s\n\n%s", ue, h.Help())
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// Find returns the verb named name, shared or extra.
func (h *Host) Find(name string) (Verb, bool) {
	for _, v := range h.Verbs() {
		if slices.Contains(v.Names, name) {
			return v, true
		}
	}
	return Verb{}, false
}

// Verbs is the shared verbs, then Extra.
func (h *Host) Verbs() []Verb {
	return append(h.shared(), h.Extra...)
}

// Help is "usage:", VerbHelp, then HelpTail.
func (h *Host) Help() string {
	help := "usage:\n" + h.VerbHelp()
	if h.HelpTail != "" {
		help += "\n" + h.HelpTail
	}
	return help
}

// VerbHelp lists every verb's forms and description, for a program that writes its own help.
func (h *Host) VerbHelp() string {
	var b strings.Builder
	for _, v := range h.Verbs() {
		for _, u := range v.Usage {
			fmt.Fprintf(&b, "  %s %s\n", h.Prog, u)
		}
		b.WriteString(strings.ReplaceAll(v.Help, "{prog}", h.Prog))
	}
	return b.String()
}

func (h *Host) client() *gdshpkg.Client {
	if h.Client == nil {
		h.Client = &gdshpkg.Client{}
	}
	return h.Client
}

// reads reports whether folder is one Name reads.
func (h *Host) reads(folder string) bool {
	for _, f := range h.Folders() {
		if SameDir(f.Path, folder) {
			return true
		}
	}
	return false
}

// notRead is the hint for a path Name doesn't read: what to add and AddCmd's argument.
func (h *Host) notRead(path, what, arg string) string {
	if h.AddCmd == "" {
		return fmt.Sprintf("%s doesn't read %s; it reads the folders '%s dirs' lists.", h.Name, path, h.Prog)
	}
	return fmt.Sprintf("%s doesn't read %s. Add %s with: %s %s", h.Name, path, what, h.AddCmd, arg)
}

func (h *Host) shared() []Verb {
	return []Verb{
		{Names: []string{"new"}, Usage: []string{"new NAME [SUB...] [--project | --dir DIR] [--force]"},
			Help: `      Write a command script. Name it as it is typed: 'new tools bake' writes
      tools/bake/bake.gd, the 'bake' subcommand of 'tools', and a tools/tools.gd parent if
      there is none. It goes in the user root's commands/ folder, or this project's
      (--project), or any folder (--dir). --force replaces the command.
`, Run: func(_ string, args []string, stdout io.Writer) error { return h.cmdNew(args, stdout) }},
		{Names: []string{"search"}, Usage: []string{"search [TERM] [--refresh]"},
			Help: `      List the reviewed command packages (godot-gdsh-lib-manifest), with what they need and
      whether they are installed.
`, Run: h.packages},
		{Names: []string{"install"}, Usage: []string{"install NAME... [--project | --dir ROOT] [--force] [--refresh]"},
			Help: `      Install packages and the packages they need into the root's packages/ folder, at the
      versions the index pins. --force reinstalls current ones.
`, Run: h.packages},
		{Names: []string{"packages"}, Usage: []string{"packages [--project | --dir ROOT]"},
			Help: `      List installed packages, and newer pinned versions.
`, Run: h.packages},
		{Names: []string{"update"}, Usage: []string{"update [NAME...] [--project | --dir ROOT]"},
			Help: `      Update the named packages, or every outdated one.
`, Run: h.packages},
		{Names: []string{"uninstall"}, Usage: []string{"uninstall NAME [--project | --dir ROOT] [--force]"},
			Help: `      Remove a package; --force even when an installed package needs it.
`, Run: h.packages},
		{Names: []string{"dirs"}, Usage: []string{"dirs"},
			Help: `      List the command folders read here, in order (later ones win on a name clash).
`, Run: func(_ string, args []string, stdout io.Writer) error {
				if len(args) > 0 {
					return UsageError("dirs takes no arguments")
				}
				for _, f := range h.Folders() {
					state := ""
					if !DirExists(f.Path) {
						state = "  (missing)"
					}
					fmt.Fprintf(stdout, "%s  [%s]%s\n", f.Path, f.Source, state)
				}
				return nil
			}},
	}
}
