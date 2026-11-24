extends RefCounted
## The builtin command scripts. Kept apart from load.gd so a plugin that only implements commands
## (CommandBase -> Utils -> Load) does not ship every builtin; GDSh.Host preloads this instead.

const Types = preload("./types.gd")

const SCRIPTS = [
	preload("./builtins/builtins.gd"),
	preload("./builtins/hidden/hidden.gd"),
	preload("./builtins/clear/clear.gd"),
	preload("./builtins/new_ctx/new_ctx.gd"),
	preload("./builtins/break/break.gd"),
	preload("./builtins/continue/continue.gd"),
	preload("./builtins/return/return.gd"),
	preload("./builtins/exit/exit.gd"),
	preload("./builtins/shift/shift.gd"),
	preload("./builtins/true/true.gd"),
	preload("./builtins/false/false.gd"),
	preload("./builtins/comparison/comparison.gd"),
	preload("./builtins/expr/expr.gd"),
	preload("./builtins/echo/echo.gd"),
	preload("./builtins/source/source.gd"),
	preload("./builtins/cd/cd.gd"),
	preload("./builtins/cn/cn.gd"),
	preload("./builtins/pwn/pwn.gd"),
	preload("./builtins/node/node.gd"),
	preload("./builtins/script/script.gd"),
	preload("./builtins/ambiguous/ambiguous.gd"),
	preload("./builtins/help/help.gd"),
	preload("./builtins/function/function.gd"),
	preload("./builtins/gdsh/gdsh.gd"),
	preload("./builtins/undoredo/undoredo.gd"),
	preload("./builtins/wait/wait.gd"),
]


static func load_builtins() -> Dictionary:
	var scopes = {}
	for script in SCRIPTS:
		scopes[script.get_command_name()] = {Types.ScopeDataKeys.SCRIPT: script}
	return scopes

