# gdsh

Run [gdsh](https://github.com/brohd11/godot-gdsh) commands from a terminal, headless, in any Godot project
(no gdsh needed in it), and manage the command roots gdsh reads.

```sh
gdsh 'ls res://addons'
gdsh 'for f in $(ls res://) { echo "file: $f" }'
gdsh 'ls res://' | grep '/$'
gdsh -f script.gdsh
gdsh install terrain3d        # a reviewed command package, into ~/.gdsh/packages
gdsh new tools bake           # a command script, in ~/.gdsh/commands
```

The line is gdsh syntax, the same as in the Editor Console, not your shell's: quote it so your shell
passes it whole. A first word that is one of gdsh's own commands (`new`, `search`, `install`, `packages`,
`update`, `uninstall`, `dirs`, `register`, `self-update`) runs that command; `gdsh -c '<line>'` always
runs a line. See `gdsh --help`.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/brohd11/gdsh/main/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/brohd11/gdsh/main/install.ps1 | iex
```

The installer puts `gdsh` in `~/.local/bin` (Windows: `%LOCALAPPDATA%\bin`) and registers the Godot
binaries it finds. `gdsh register <path to godot>` adds others, and `gdsh self-update` updates it.
`$GDSH_HOME` replaces `~/.gdsh`.

## How it runs

- The project is the nearest folder above the working directory with a `project.godot`.
- Its Godot is `$GODOT`, else the binary in `~/.gdsh/executables` matching the version in
  `config/features`, preferring mono when the project uses C#. It never falls back to another version,
  which would re-import the project's `.godot/`.
- `gdsh register` records each binary under the version `godot --version` reports.
- The GDScript half ([godot-gdsh-cli](https://github.com/brohd11/godot-gdsh-cli)) is embedded in the
  binary and unpacked to `~/.gdsh/cli` whenever it differs. Godot runs `boot.gd` from there, using the
  project's own gdsh when it has one.

## Layout

| Package    | What                                                                                      |
|------------|-------------------------------------------------------------------------------------------|
| `.`        | the `gdsh` binary: line running, Godot lookup, `register`, `self-update`                   |
| `gdshcmd`  | the command-root verbs, shared with `mcp_sh_godot gdsh` through a `Host`                   |
| `payload`  | the embedded GDScript release; refresh it with `scripts/fetch_payload.sh` (`go generate`)  |

Package installs come from [gdsh-pkg](https://github.com/brohd11/gdsh-pkg).
