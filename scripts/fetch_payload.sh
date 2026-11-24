#!/bin/sh
# Vendor the gdsh-cli release export into payload/cli, which go:embed carries and gdsh unpacks
# into ~/.gdsh/cli. Commit the result.
#
#   scripts/fetch_payload.sh             # latest release of brohd11/godot-gdsh-lib-cli
#   scripts/fetch_payload.sh v0.2.0      # that tag
#   scripts/fetch_payload.sh --zip FILE  # a local export zip (development only)
#   scripts/fetch_payload.sh --check [TAG]  # only compare versions: exit 1 if the vendored
#                                           # payload is not the latest release (or TAG)
#
# The fetch rewrites every .uid file; restore them by hand before committing. --check
# downloads nothing and writes nothing, so CI can run it.
#
# Needs curl, jq and unzip. GITHUB_TOKEN (or GH_TOKEN) raises the API rate limit.
set -eu

REPO="brohd11/godot-gdsh-lib-cli"
PKG="addons/_lib/gdsh_lib/cli"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
DEST="$ROOT/payload/cli"
TMP=$(mktemp -d "${TMPDIR:-/tmp}/gdsh-payload.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

check=0
if [ "${1:-}" = "--check" ]; then
	check=1
	shift
	[ "${1:-}" != "--zip" ] || { echo "usage: $0 --check [TAG]" >&2; exit 2; }
fi

if [ "${1:-}" = "--zip" ]; then
	[ -n "${2:-}" ] || { echo "usage: $0 --zip FILE" >&2; exit 2; }
	zip=$2
	source="$zip"
else
	if [ -n "${1:-}" ]; then
		api="https://api.github.com/repos/$REPO/releases/tags/$1"
	else
		api="https://api.github.com/repos/$REPO/releases/latest"
	fi
	token=${GITHUB_TOKEN:-${GH_TOKEN:-}}
	if [ -n "$token" ]; then
		curl -fsSL -H "Authorization: Bearer $token" "$api" > "$TMP/release.json"
	else
		curl -fsSL "$api" > "$TMP/release.json"
	fi
	tag=$(jq -r '.tag_name' "$TMP/release.json")
	if [ "$check" = 1 ]; then
		have=$(sed -n 's/^version="\(.*\)"/\1/p' "$DEST/package.cfg")
		if [ "v$have" = "$tag" ]; then
			echo "payload/cli is up to date ($tag)"
			exit 0
		fi
		echo "payload/cli is v$have but $REPO is at $tag -- run scripts/fetch_payload.sh, restore the .uid files, commit" >&2
		exit 1
	fi
	url=$(jq -r '[.assets[] | select(.name | endswith(".zip"))][0].browser_download_url // empty' "$TMP/release.json")
	[ -n "$url" ] || { echo "error: release $tag of $REPO has no zip asset" >&2; exit 1; }
	zip="$TMP/payload.zip"
	curl -fsSL -o "$zip" "$url"
	source="$REPO $tag"
fi

unzip -q "$zip" -d "$TMP/x"
src="$TMP/x/$PKG"
[ -f "$src/boot.gd" ] && [ -f "$src/package.cfg" ] || { echo "error: no $PKG/boot.gd in $zip" >&2; exit 1; }
# Leftovers of the shell-wrapper releases; the Go binary replaces them.
rm -rf "$src/bin" "$src/install.sh" "$src/install.ps1" "$src/register.gd" "$src/register.gd.uid"

rm -rf "$DEST"
mkdir -p "$(dirname "$DEST")"
cp -R "$src" "$DEST"
version=$(sed -n 's/^version="\(.*\)"/\1/p' "$DEST/package.cfg")
echo "vendored gdsh-cli $version from $source into payload/cli"
