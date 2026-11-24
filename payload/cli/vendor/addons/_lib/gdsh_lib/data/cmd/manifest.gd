extends RefCounted
## Preloads the data commands, so exporters that follow preloads include them. GDSh.Load skips
## this file. Register by adding this directory (cmd/) as a command folder:
## host.add_command_dir(Manifest.resource_path.get_base_dir()).

const DATA = preload("./data/data.gd")

const COMMANDS = [
	preload("./data/to_bin/to_bin.gd"),
	preload("./data/to_json/to_json.gd"),
	preload("./data/to_yaml/to_yaml.gd"),
]

