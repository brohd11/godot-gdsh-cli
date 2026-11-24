extends RefCounted
## Preloads every utils command, so exporters that follow preloads include them. GDSh.Load
## skips this file. Register the commands by adding this directory (cmd/) as a command folder:
## host.add_command_dir(Manifest.resource_path.get_base_dir()). Flat on purpose: the commands
## are top-level (non-discoverable) and utils/utils.gd only lists them.

const COMMANDS = [
	preload("./utils/utils.gd"),
	preload("./cat/cat.gd"),
	preload("./check/check.gd"),
	preload("./class/class.gd"),
	preload("./count/count.gd"),
	preload("./find/find.gd"),
	preload("./grep/grep.gd"),
	preload("./head/head.gd"),
	preload("./home/home.gd"),
	preload("./ls/ls.gd"),
	preload("./math/math.gd"),
	preload("./mkdir/mkdir.gd"),
	preload("./mv/mv.gd"),
	preload("./open/open.gd"),
	preload("./pwd/pwd.gd"),
	preload("./realpath/realpath.gd"),
	preload("./scan/scan.gd"),
	preload("./str/str.gd"),
	preload("./tail/tail.gd"),
	preload("./trash/trash.gd"),
	preload("./xargs/xargs.gd"),
]

## Subcommands of the `str` namespace. Kept apart from COMMANDS, which mirrors the
## top-level load; load_directory finds these through str.gd.
const STR_COMMANDS = [
	preload("./str/basedir/basedir.gd"),
	preload("./str/basename/basename.gd"),
	preload("./str/begins_with/begins_with.gd"),
	preload("./str/contains/contains.gd"),
	preload("./str/ends_with/ends_with.gd"),
	preload("./str/extension/extension.gd"),
	preload("./str/file/file.gd"),
	preload("./str/join/join.gd"),
	preload("./str/length/length.gd"),
	preload("./str/lower/lower.gd"),
	preload("./str/replace/replace.gd"),
	preload("./str/slice/slice.gd"),
	preload("./str/strip_edges/strip_edges.gd"),
	preload("./str/trim_prefix/trim_prefix.gd"),
	preload("./str/trim_suffix/trim_suffix.gd"),
	preload("./str/upper/upper.gd"),
]

