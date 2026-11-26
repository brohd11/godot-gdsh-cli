# gdsh

Run [gdsh](https://github.com/brohd11/godot-gdsh) commands from a terminal, headless, in any Godot project
(no gdsh needed in it), and manage the command roots gdsh reads.

```sh
gdsh 'ls res://addons'
gdsh 'for f in $(ls res://) { echo "file: $f" }'
gdsh 'ls res://' | grep '/$'
gdsh -f script.gdsh
gdsh package install terrain3d  # a reviewed command package, into ~/.gdsh/packages
gdsh new tools bake             # a command script, in ~/.gdsh/commands
gdsh editor                     # open the project in its Godot editor
gdsh run res://main.tscn        # run the project (or a scene) in its Godot
```

The line is gdsh syntax, the same as in the Editor Console, not your shell's: quote it so your shell
passes it whole. A first word that is one of gdsh's own commands (`new`, `package`, `dirs`, `editor`, `run`,
`register`, `update`) runs that command; `gdsh -c '<line>'` always runs a line. See `gdsh --help`.

`gdsh package` groups the package commands: `search`, `install`, `list`, `update` and `uninstall`.
`gdsh update` updates gdsh itself.

`gdsh editor` and `gdsh run` start the project's Godot (picked as below) with `--path <project>`, from
any folder inside it. `editor` returns at once; `run` stays in the foreground with Godot's output, and
Ctrl-C stops it. A scene and other Godot flags pass through: `gdsh run res://level.tscn --debug-collisions`.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/brohd11/godot-gdsh-cli/main/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/brohd11/godot-gdsh-cli/main/install.ps1 | iex
```

The installer puts `gdsh` in `~/.local/bin` (Windows: `%LOCALAPPDATA%\bin`) and registers the Godot
binaries it finds. `gdsh register <path to godot>` adds others, and `gdsh update` updates it.
`$GDSH_HOME` replaces `~/.gdsh`.

## How it runs

- The project is the nearest folder above the working directory with a `project.godot`.
- Its Godot is `$GODOT`, else the binary in `~/.gdsh/executables` matching the version in
  `config/features`, preferring mono when the project uses C#. It never falls back to another version,
  which would re-import the project's `.godot/`.
- `gdsh register` records each binary under the version `godot --version` reports.
- The GDScript half ([godot-gdsh-lib-cli](https://github.com/brohd11/godot-gdsh-lib-cli)) is embedded in the
  binary and unpacked to `~/.gdsh/cli` whenever it differs. Godot runs `boot.gd` from there, using the
  project's own gdsh when it has one.
- Only the command's output shows. `boot.gd` writes it to result files, apart from Godot's own logging
  (banner, the leak report at exit, load errors). `gdsh --engine-verbose …` adds Godot's output on
  stderr. When a command fails and Godot logged errors, gdsh says so; when `boot.gd` writes no result
  (a crash, say), Godot's output is printed instead.

## Layout

| Package    | What                                                                                      |
|------------|-------------------------------------------------------------------------------------------|
| `.`        | the `gdsh` binary: line running, Godot lookup, `register`, `update`                        |
| `gdshcmd`  | the command-root verbs, shared with `mcp_sh_godot gdsh` through a `Host`                   |
| `payload`  | the embedded GDScript release; refresh it with `scripts/fetch_payload.sh` (`go generate`)  |

Package installs come from [gdsh-pkg](https://github.com/brohd11/gdsh-pkg).
