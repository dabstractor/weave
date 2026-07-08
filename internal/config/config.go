// Package config reads and writes the weave settings file (PRD §8.1), the small
// sidecar that records only the store location (the absolute path to the
// extensions directory). The whole §8 config model funnels through this package:
// extdir.findConfig (P1.M1.T3) reads the store via Load, and main.runInit
// (P1.M4.T4) writes it via Save.
//
// This is a SETTINGS SIDECAR, not a catalog index. PRD §2 constraint #1 (and §17)
// forbid a catalog enumerating the extension set — extensions are discovered by
// walking the on-disk store. The only thing this file persists is a value the
// filesystem cannot express (where the store lives). Do not grow this struct
// into a catalog.
//
// Parsing is a HAND-ROLLED line scan (PRD §8.1: "~10-line hand-rolled reader …
// we are not pulling in yaml.v3 for one key"). The scanner looks for the first
// line matching `^store:\s*(.+)$` and takes the captured value (trailing
// whitespace trimmed). Unknown keys are ignored (room to grow). Because the
// parser is this simple, "unparseable" reduces to "no store: line found", which
// is indistinguishable from "not configured": Load returns File{}, nil in both
// cases (PRD §8.1 "A missing or unreadable config is treated as 'not yet
// configured' and falls through … never a hard error"). This differs from
// skilldozer, where broken YAML is a hard error via yaml.v3 — weave has no such
// case by construction.
//
// Path/DefaultStore are pure functions of the environment (PRD §8.1/§8.2): they
// read env vars and compute a path, they do NOT touch the filesystem (existence
// is an extdir/init concern).
package config

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// File is the parsed weave settings config (PRD §8.1). The single field records
// the absolute path to the extensions directory (the "store"). There is
// intentionally no catalog/index here — PRD §2/§17 forbid it.
type File struct {
	Store string
}

// Load reads and parses the config file at path. It implements PRD §8.1.
//
// Parsing is a hand-rolled line scan: the first line matching
// `^store:\s*(.+)$` (with trailing whitespace trimmed) sets Store; unknown keys
// are ignored; a file with no "store:" line yields File{}, nil.
//
// The os.ReadFile error is returned VERBATIM — it is NOT wrapped with
// fmt.Errorf — so callers can distinguish a missing file from a broken one via
// errors.Is(err, fs.ErrNotExist). extdir.findConfig (P1.M1.T3) relies on this
// to fall through to the next §8.3 discovery rule instead of aborting when the
// file does not exist yet. A present-but-no-"store:"-line file is also
// "not configured" and returns File{}, nil (NOT an error).
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Return the read error untouched so callers can errors.Is(err, fs.ErrNotExist).
		return File{}, err
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	for scan.Scan() {
		line := strings.TrimRight(scan.Text(), " \t\r") // tolerate CRLF from Windows editors
		rest := strings.TrimPrefix(line, "store:")
		if rest == line {
			// Line did not start with "store:" — unknown key, ignored.
			continue
		}
		val := strings.TrimLeft(rest, " \t")
		if val == "" {
			// "store:" with nothing after it — keep scanning (treat as not set).
			continue
		}
		return File{Store: val}, nil // first match wins
	}
	if err := scan.Err(); err != nil {
		return File{}, err // scanner I/O error (rare for an in-memory reader)
	}
	return File{}, nil // no store: line found -> "not configured" -> NOT an error
}

// Save writes f to path as deterministic bytes, creating any missing parent
// directories. It implements PRD §8.1.
//
// The on-disk format is exactly "store: <value>\n" — hand-built, not produced
// by a YAML marshaler — so it is byte-stable across versions (no struct-field
// sorting, no trailing "..." or BOM). The parent directory is created with
// os.MkdirAll (mode 0o755) before the write — config.yaml's directory (e.g.
// $XDG_CONFIG_HOME/weave/) will not exist on first run, and MkdirAll is an
// idempotent no-op when it already exists. The file itself is written with
// mode 0o644.
func Save(path string, f File) error {
	out := []byte("store: " + f.Store + "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// configEnv is the environment variable that overrides the config-file location
// (PRD §8.1). Note the LOWERCASE name: PRD §8.1 specifies `weave_CONFIG`, not
// `WEAVE_CONFIG`; os.Getenv is case-sensitive on Linux. Set to an absolute or
// relative path to redirect weave at a different config file (useful for tests
// / multiple profiles). It is read by Path; a non-empty value is taken as the
// literal config-file path (cleaned lexically, NOT joined to the config home).
// Package-internal: no consumer needs the symbol — Path encapsulates the read.
const configEnv = "weave_CONFIG"

// Path returns the path to the weave config file (PRD §8.1 — "the one fixed,
// well-known path the binary can bootstrap from"). It is a pure function of the
// environment and reads no filesystem state.
//
// Resolution order:
//
//  1. $weave_CONFIG, if non-empty, is the literal config-file path: returned
//     AS-IS after filepath.Clean (lexical .. / trailing-slash cleanup only; no
//     symlink evaluation). Absolute AND relative values both work — the
//     override is NOT joined to the config home, so a relative value is usable
//     for tests / multiple profiles. (Empty == unset: os.Getenv returns "" for
//     both, and the "" guard means an empty override falls through to the XDG
//     default rather than producing filepath.Clean("") == ".".)
//  2. Otherwise $XDG_CONFIG_HOME/weave/config.yaml, where the XDG config home
//     is os.UserConfigDir() (which honors $XDG_CONFIG_HOME, falls back to
//     ~/.config, and rejects a relative $XDG_CONFIG_HOME with a non-nil error).
//
// Any error from os.UserConfigDir (a relative $XDG_CONFIG_HOME, or neither
// $XDG_CONFIG_HOME nor $HOME defined) is returned VERBATIM, not wrapped:
// extdir treats any Path error as "config unavailable -> fall through to the
// next §8.3 rule" and never inspects the error type.
func Path() (string, error) {
	if v := os.Getenv(configEnv); v != "" {
		return filepath.Clean(v), nil
	}
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configHome, "weave", "config.yaml"), nil
}

// DefaultStore returns the default extensions store directory (PRD §8.2 / §8.3).
// It is a pure function of the environment and reads no filesystem state.
//
// Resolution order:
//
//  1. $XDG_DATA_HOME, if set AND absolute, is the base:
//     $XDG_DATA_HOME/weave/extensions. (A relative $XDG_DATA_HOME is INVALID per
//     the XDG spec and is ignored — guarded by filepath.IsAbs — so a
//     misconfigured value never produces a relative store path.)
//  2. Otherwise ~/.local/share/weave/extensions, where ~ is os.UserHomeDir().
//     There is no os.UserDataDir(), so the XDG data-home rule is computed by
//     hand, exactly as external_deps.md §2 prescribes.
//
// Any error from os.UserHomeDir ($HOME unset) is returned VERBATIM, not wrapped.
// This is the value main.runInit (P1.M4.T4) offers as the out-of-the-box store
// when no weave_STORE env var is set, so a go install user gets a sane default.
func DefaultStore() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "weave", "extensions"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "weave", "extensions"), nil
}
