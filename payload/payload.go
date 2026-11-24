// Package payload carries the GDScript half of the gdsh CLI (boot.gd, src/run.gd and the gdsh
// copy vendored beside them) and unpacks it to <gdsh home>/cli, where Godot runs it. It is the
// release export of github.com/brohd11/godot-gdsh-cli, vendored by scripts/fetch_payload.sh
// (`go generate`) and committed. Only the gdsh binary imports it, so mcp_sh_godot stays lean.
package payload

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

//go:generate sh ../scripts/fetch_payload.sh
//go:embed all:cli
var files embed.FS

const (
	srcDir    = "cli"
	stampFile = ".payload-stamp"
	Dir       = "cli" // under the gdsh home
)

var cfgVersion = regexp.MustCompile(`(?m)^version="([^"]*)"`)

// Version is the embedded package.cfg's version.
func Version() string {
	data, err := files.ReadFile(srcDir + "/package.cfg")
	if err != nil {
		return "unknown"
	}
	if m := cfgVersion.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return "unknown"
}

// Stamp hashes every embedded path and body, so a payload rebuilt at the same version still
// replaces the unpacked copy.
var Stamp = sync.OnceValue(func() string {
	h := sha256.New()
	fs.WalkDir(files, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(data))
		h.Write(data)
		return nil
	})
	return Version() + " " + hex.EncodeToString(h.Sum(nil))[:16]
})

// Ensure unpacks the payload into home/cli unless the copy there carries the same stamp, and
// returns that folder. It unpacks beside it and swaps, so a failure leaves the old copy whole.
func Ensure(home string) (string, error) {
	dest := filepath.Join(home, Dir)
	if current(dest) {
		return dest, nil
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(home, ".cli-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := extract(stage); err != nil {
		return "", fmt.Errorf("unpacking the gdsh payload: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stage, stampFile), []byte(Stamp()+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", fmt.Errorf("replacing %s (is Godot still running from it?): %w", dest, err)
	}
	if err := os.Rename(stage, dest); err != nil {
		// Another gdsh may have unpacked the same payload first.
		if current(dest) {
			return dest, nil
		}
		return "", err
	}
	return dest, nil
}

func current(dest string) bool {
	data, err := os.ReadFile(filepath.Join(dest, stampFile))
	return err == nil && string(data) == Stamp()+"\n"
}

func extract(stage string) error {
	return fs.WalkDir(files, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		out := filepath.Join(stage, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}
