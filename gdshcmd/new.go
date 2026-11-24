package gdshcmd

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*.gd.tmpl
var templates embed.FS

var commandName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// cmdNew writes NAME [SUB...]: missing parents get the parent template, the last name the
// command template.
func (h *Host) cmdNew(args []string, stdout io.Writer) error {
	var names []string
	var dir string
	project, force := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--project":
			project = true
		case a == "--force":
			force = true
		case a == "--dir":
			if i+1 >= len(args) {
				return UsageError("--dir needs a folder")
			}
			i++
			dir = args[i]
		case strings.HasPrefix(a, "--dir="):
			dir = strings.TrimPrefix(a, "--dir=")
		case strings.HasPrefix(a, "-"):
			return UsageError(fmt.Sprintf("unknown flag %q", a))
		default:
			if !commandName.MatchString(a) {
				return fmt.Errorf("invalid command name %q: use letters, digits, _ and -", a)
			}
			names = append(names, a)
		}
	}
	if len(names) == 0 {
		return UsageError("new needs a command name")
	}
	if project && dir != "" {
		return UsageError("use --project or --dir, not both")
	}

	// --dir names the commands folder itself; otherwise it is the root's commands/.
	folder := ResolveDir(dir, ".")
	if dir == "" {
		root, err := h.root("", project)
		if err != nil {
			return err
		}
		folder = filepath.Join(root, CommandsDir)
	}

	// Plan every file before writing any, so a refusal leaves nothing half made.
	type file struct{ path, tmpl, name, usage string }
	var files []file
	parent := folder
	for i, name := range names {
		usage := strings.Join(names[:i+1], " ")
		if loose := filepath.Join(parent, name+".gd"); FileExists(loose) {
			return fmt.Errorf("%s is a single-file command; move it to %s to give it subcommands or replace it",
				loose, filepath.Join(parent, name, name+".gd"))
		}
		path := filepath.Join(parent, name, name+".gd")
		last := i == len(names)-1
		switch {
		case last && FileExists(path) && !force:
			return fmt.Errorf("%s exists; use --force to replace it", path)
		case last:
			files = append(files, file{path, "command.gd.tmpl", name, usage})
		case !FileExists(path):
			files = append(files, file{path, "parent.gd.tmpl", name, usage})
		}
		parent = filepath.Join(parent, name)
	}
	for _, f := range files {
		if err := writeTemplate(f.path, f.tmpl, f.name, f.usage); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created %s\n", f.path)
	}

	if !h.reads(folder) {
		fmt.Fprintln(stdout, h.notRead(folder, "it", folder))
	}
	fmt.Fprintf(stdout, "Fill in its help and _execute, then try %s\n", h.Try(strings.Join(names, " ")+" --help"))
	if h.Note != "" {
		fmt.Fprintln(stdout, h.Note)
	}
	return nil
}

func writeTemplate(path, name, command, usage string) error {
	t, err := template.ParseFS(templates, "templates/"+name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = t.Execute(f, map[string]string{"Name": command, "Usage": usage})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
