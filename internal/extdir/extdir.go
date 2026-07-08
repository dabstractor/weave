// Package extdir locates the on-disk extensions/ directory for weave.
//
// It implements the PRD §8.3 priority order (first hit wins):
//
//  1. weave_EXTENSIONS_DIR env var — override; if set and an existing dir, use it as-is.
//  2. Config file `store` (PRD §8.1) — the primary, set by `weave init`.   [S2]
//  3. Sibling of the running binary (symlink-aware via os.Executable + EvalSymlinks). [S2]
//  4. Walk up from the current working directory.                          [S3]
//  5. None ⇒ unconfigured: Find returns ErrNotFound.                       [S3]
//
// THIS FILE (P1.M1.T3.S1) implements rule 1 (findEnv) and the HasExtensionEntry
// predicate. Find() and rules 2-4 are added by S2/S3.
//
// findEnv reads weave_EXTENSIONS_DIR; if it names an existing directory the path is
// returned as-is, only made absolute/clean via filepath.Abs — NEVER through
// filepath.EvalSymlinks (the user points exactly where they want; a symlink is
// preserved verbatim). The sibling rule (S2) DOES use EvalSymlinks, but that is a
// different rule with a different contract.
//
// HasExtensionEntry reports whether a directory contains at least one extension
// entry (PRD §7.1) at any depth. It doubles as: (a) the §8.2 cwd-auto-detect
// predicate used by `weave init` ("if the current working directory already looks
// like a store … contains at least one extension entry at any depth"), and (b) the
// qualifier for the §8.3 rule 4 walk-up (an ancestor only wins if its extensions/
// subdir actually contains entries). It short-circuits on the first entry found.
package extdir

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source identifies which §8.3 rule located the extensions directory. It is
// reported by `weave --path` so users can tell how the dir was found. The iota
// order is pinned so S2/S3 constants slot in unchanged from skilldozer's layout.
type Source int

const (
	// SourceEnv means weave_EXTENSIONS_DIR was set and pointed at an existing dir.
	SourceEnv Source = iota
	// SourceConfig means the extensions dir was read from the config file's `store` key (PRD §8.1). [S2]
	SourceConfig
	// SourceSibling means the extensions dir was found next to the running binary. [S2]
	SourceSibling
	// SourceWalkUp means the extensions dir was found by walking up from cwd. [S3]
	SourceWalkUp
)

// String returns a human-readable label for the rule that won, used by
// `weave --path` stderr reporting (PRD §8.3 labels are exact). Satisfies
// fmt.Stringer.
func (s Source) String() string {
	switch s {
	case SourceEnv:
		return "weave_EXTENSIONS_DIR"
	case SourceConfig:
		return "config file"
	case SourceSibling:
		return "sibling of binary"
	case SourceWalkUp:
		return "ancestor of cwd"
	default:
		return "unknown"
	}
}

// envVar is the environment variable consulted by rule 1. It is a package
// constant (not a parameter): the contract is "mock/replace nothing" — tests
// drive it via t.Setenv / os.Unsetenv, never via injection. LOWERCASE per PRD §8.3
// + the item contract (skilldozer uses uppercase SKILLDOZER_SKILLS_DIR; weave does NOT).
const envVar = "weave_EXTENSIONS_DIR"

// findEnv implements PRD §8.3 rule 1.
//
// It reads weave_EXTENSIONS_DIR; if the value names an existing directory it returns
// that directory as an absolute path with src=SourceEnv and found=true. The env
// path is NOT passed through filepath.EvalSymlinks: the user points exactly
// where they want (a symlink is preserved verbatim, only made absolute/clean
// via filepath.Abs). If the variable is unset, empty, or does not name an
// existing directory, it returns found=false with src's zero value so Find()
// (S3) can fall through to rule 2 — a bad env value never hard-errors.
func findEnv() (dir string, src Source, found bool) {
	val, ok := os.LookupEnv(envVar)
	if !ok || val == "" {
		return "", 0, false
	}
	info, err := os.Stat(val)
	if err != nil || !info.IsDir() {
		return "", 0, false // not an existing dir -> let the next rule try
	}
	abs, err := filepath.Abs(val)
	if err != nil {
		return "", 0, false // cwd unresolvable -> let the next rule try
	}
	return abs, SourceEnv, true
}

// errExtensionFound is a sentinel error used to short-circuit filepath.WalkDir
// as soon as the first extension entry is found, so HasExtensionEntry does not
// walk the entire tree. Returning any non-nil error from a WalkDir callback
// stops the walk. Unexported: the sentinel is an implementation detail.
var errExtensionFound = errors.New("extension entry found")

// HasExtensionEntry reports whether dir contains at least one extension entry
// (PRD §7.1) at any depth. It walks the tree under dir but returns true as soon
// as it finds one entry (early exit via the errExtensionFound sentinel).
//
// The three PRD §7.1 entry kinds are recognized:
//   - a single-file extension: a *.ts/*.js file whose base name is NOT index.ts/index.js;
//   - a dir/package extension: a directory directly containing index.ts or index.js;
//   - a package extension: a directory whose package.json has a pi.extensions
//     array naming at least one EXISTING entry.
//
// It is exported because it doubles as the §8.2 cwd-auto-detect predicate
// (`weave init` uses it to decide whether cwd already looks like a store) and
// the §8.3 rule 4 walk-up qualifier (an ancestor only wins if its extensions/
// subdir contains at least one entry).
//
// NOTE: filepath.Glob with a "**" pattern is intentionally NOT used: Go's
// path/filepath does not support "**" (it behaves like single-level "*").
// WalkDir is the correct stdlib tool and recurses to arbitrary depth.
func HasExtensionEntry(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entry, keep walking (do NOT propagate)
		}
		if d.IsDir() {
			if isResolvableDir(path) {
				found = true
				return errExtensionFound // this dir IS an entry -> short-circuit
			}
			return nil // plain category dir -> WalkDir descends naturally
		}
		// file entry:
		if isExtensionFile(d.Name()) {
			found = true
			return errExtensionFound // single-file extension -> short-circuit
		}
		return nil
	})
	return found
}

// isExtensionFile reports whether name is a single-file extension per PRD §7.1:
// a *.ts or *.js file whose base name is NOT index.ts/index.js (those are
// directory-entry markers, handled by isResolvableDir).
func isExtensionFile(name string) bool {
	if name == "index.ts" || name == "index.js" {
		return false
	}
	return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")
}

// isResolvableDir reports whether dir is a dir/package extension per PRD §7.1:
// it directly contains an index.ts or index.js (case b), OR its package.json has
// a pi.extensions array naming at least one existing entry (case c).
func isResolvableDir(dir string) bool {
	if fileExists(filepath.Join(dir, "index.ts")) || fileExists(filepath.Join(dir, "index.js")) {
		return true
	}
	return hasPiExtensions(dir)
}

// hasPiExtensions reports whether dir/package.json declares a pi.extensions
// array naming at least one EXISTING entry (PRD §7.1 case c). Mirrors pi's own
// resolver (pi_extension_facts.md §4: "only entries that actually EXIST on disk
// are included"). A typed []string field silently drops non-string elements and
// yields nil when pi.extensions is absent or not an array — that is the lenient
// "wrong-typed fields coerced or ignored" behavior PRD §7.3 wants, so no custom
// coercion is written.
func hasPiExtensions(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Pi struct {
			Extensions []string `json:"extensions"`
		} `json:"pi"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	for _, e := range pkg.Pi.Extensions {
		if fileExists(filepath.Join(dir, e)) {
			return true
		}
	}
	return false
}

// fileExists reports whether a non-directory entry exists at path. os.Stat
// follows symlinks (matching pi's fs.existsSync semantics), so a symlink to a
// file counts. Used to check for index.* markers and existing pi.extensions entries.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
