extends "../test/test.gd"
## `test` under a name shell builtins don't shadow (e.g. the MCP sandbox shell).

static func get_command_name():
	return "gdtest"

