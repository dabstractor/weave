// Package extdir locates the on-disk extensions/ directory for weave.
//
// It implements the PRD §8.3 priority order (first hit wins):
//
//  1. weave_EXTENSIONS_DIR env var — override; if set and an existing dir, use it as-is.
//  2. Config file `store` (PRD §8.1) — the primary, set by `weave init`.  (findConfig)
//  3. Sibling of the running binary (symlink-aware via os.Executable + EvalSymlinks). (findSibling)
//  4. Walk up from the current working directory.                          (findWalkUp)
//  5. None ⇒ unconfigured: Find returns ErrNotFound.
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

	"github.com/dabstractor/weave/internal/config"
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

// findConfig implements PRD §8.3 rule 2 — the config file's `store` key (PRD §8.1).
//
// It is the primary discovery rule, set by `weave init` (P1.M4.T4). config.Path()
// gives the one fixed, well-known bootstrap path ($weave_CONFIG or
// $XDG_CONFIG_HOME/weave/config.yaml); config.Load() reads it. findConfig treats ANY
// error from either as "not yet configured -> fall through" — PRD §8.1: a
// missing/unreadable config NEVER hard-errors.
//
// A relative `store` is resolved against the config file's own directory (PRD §8.1:
// store may be relative to the config file), NOT against cwd. The resolved store
// must name an existing directory or the rule misses.
//
// weave-vs-skilldozer note: weave's config.Load is a HAND-ROLLED line scanner, NOT
// yaml.v3, so it NEVER returns a "malformed YAML" hard error — a present-but-garbage
// file with no "store:" line returns File{Store: ""}, nil, and the f.Store == ""
// branch below handles it. skilldozer's findConfig had to convert a yaml.v3 hard
// error into a fall-through; weave does not (the case is impossible by construction).
//
// Returns (absStore, SourceConfig, true) on a hit; ("", 0, false) otherwise so
// Find() (S3) can fall through to the sibling rule. Never errors (locked
// per-rule shape).
func findConfig() (dir string, src Source, found bool) {
	p, err := config.Path()
	if err != nil {
		return "", 0, false // no bootstrap path (e.g. relative $XDG_CONFIG_HOME) -> fall through
	}
	f, err := config.Load(p)
	if err != nil {
		return "", 0, false // missing/unreadable -> "not yet configured" -> fall through
	}
	if f.Store == "" {
		return "", 0, false // no `store` key (or empty value) -> fall through
	}
	var store string
	if filepath.IsAbs(f.Store) {
		store = filepath.Clean(f.Store)
	} else {
		store = filepath.Join(filepath.Dir(p), f.Store) // relative to config file's dir (PRD §8.1)
	}
	info, err := os.Stat(store)
	if err != nil || !info.IsDir() {
		return "", 0, false // store path is not an existing dir -> fall through
	}
	return store, SourceConfig, true
}

// findSibling implements PRD §8.3 rule 3 — locate <repoDir>/extensions next to the
// running binary, symlink-aware. This is the rule that makes the §12.1 symlink
// install work: ~/.local/bin/weave -> ~/projects/weave/weave resolves back to the
// repo's own extensions/, and `git pull && go build` updates the linked binary in
// place.
//
// It is a thin entry that asks the OS for the running binary (os.Executable) and
// delegates the symlink/dir logic to resolveSiblingFromExe. os.Executable cannot
// be controlled in a test (it returns the test binary's own path in a temp
// go-build dir), so the testable core lives in resolveSiblingFromExe.
//
// Returns (candidate, SourceSibling, true) on a hit; ("", 0, false) otherwise so
// Find() (S3) can fall through to rule 4. Never errors (locked per-rule shape).
func findSibling() (dir string, src Source, found bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", 0, false // no binary path at all -> skip rule
	}
	d, ok := resolveSiblingFromExe(exe)
	if !ok {
		return "", 0, false
	}
	return d, SourceSibling, true
}

// resolveSiblingFromExe is the symlink-aware sibling-resolution core for rule 3,
// factored out so it can be unit-tested with arbitrary exe paths.
//
// PRD §8.3 sequence:
//
//	real, err := filepath.EvalSymlinks(exe)  // REQUIRED on macOS (redundant but
//	                                         //   harmless on Linux via /proc/self/exe)
//	if err != nil { real = exe }             // fall back to raw exe on EvalSymlinks error
//	repoDir := filepath.Dir(real)
//	candidate := filepath.Join(repoDir, "extensions")
//	win iff os.Stat(candidate) reports an existing directory
//
// CRITICAL: EvalSymlinks MUST stay. On Linux os.Executable() resolves the symlink
// via /proc/self/exe (so EvalSymlinks is redundant-but-harmless), but on macOS
// os.Executable() may return the symlink path and rule 3 SILENTLY misses without
// EvalSymlinks. See architecture/verified_symlink_resolution.md. Linux-only test
// runs pass with OR without EvalSymlinks — do NOT use that as justification to
// remove it.
func resolveSiblingFromExe(exe string) (dir string, found bool) {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		real = exe // EvalSymlinks could not resolve -> use exe verbatim (REQUIRED fallback)
	}
	repoDir := filepath.Dir(real)
	candidate := filepath.Join(repoDir, "extensions") // weave: the skills dir is named extensions
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", false // no existing extensions/ sibling -> rule misses
	}
	return candidate, true
}

// findWalkUpAncestor implements PRD §8.3 rule 4 ascent core. It is factored out of
// findWalkUp so it can be tested with an arbitrary start dir — os.Getwd is not
// controllable in a test without t.Chdir, and this core takes start as a parameter.
//
// Starting at filepath.Clean(start), for each ancestor cur it computes
// candidate := cur/extensions and checks whether that is an existing directory
// that HasExtensionEntry qualifies. PRD §8.3 qualifies the match with "at least
// one extension entry": an extensions/ dir that exists but has NO entries is
// SKIPPED and ascent continues. The START dir is checked FIRST (the loop body
// runs before any ascent). The loop terminates via parent == cur, which is how
// filepath.Dir signals the filesystem root (filepath.Dir("/") == "/"); a depth
// counter or HasPrefix root check is NOT used.
//
// weave-vs-skilldozer: ports skilldozer's findWalkUpAncestor body VERBATIM with
// two renames — the sibling dir literal changes from skills to extensions, and
// the predicate HasSkillMD → HasExtensionEntry (the S1 predicate that recognizes the
// three PRD §7.1 entry kinds at any depth).
func findWalkUpAncestor(start string) (dir string, found bool) {
	cur := filepath.Clean(start)
	for {
		candidate := filepath.Join(cur, "extensions") // weave: extensions dir, not skills
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if HasExtensionEntry(candidate) { // weave: HasExtensionEntry not HasSkillMD
				return candidate, true
			}
			// extensions/ exists here but has no entries -> keep ascending.
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false // reached filesystem root, no match
		}
		cur = parent
	}
}

// findWalkUp implements PRD §8.3 rule 4 — the catch-all for `go run` / dev
// workflows where the binary is in a temp build dir and rules 1-3 all miss. It
// is a thin entry that asks the OS for the current working directory
// (os.Getwd) and delegates the ascent to findWalkUpAncestor. The testable core
// lives there because os.Getwd is not controllable in tests (use t.Chdir).
//
// Returns (dir, SourceWalkUp, true) on a hit; ("", 0, false) on a miss or when
// os.Getwd is unresolvable. Never errors (locked per-rule shape — a miss is a
// fall-through, not a hard error).
func findWalkUp() (dir string, src Source, found bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, false // cwd unresolvable -> rule misses
	}
	d, ok := findWalkUpAncestor(cwd)
	if !ok {
		return "", 0, false
	}
	return d, SourceWalkUp, true
}

// ErrNotFound is returned by Find when every PRD §8.3 rule misses (the system
// is unconfigured). Its message is the user-facing one-line fix, pinned verbatim
// by PRD §8.2 / §6.4: main prints err.Error() to stderr with NO wrapping or
// prefix and exits 1, so `pi -e "$(weave x)"` fails loudly inside command
// substitution instead of hanging. The literal backticks around `weave init`
// are part of the message (shell users copy-paste the command).
//
// PRD §8.2 prompt safety: the bare `weave <tag>` path NEVER prompts. Find
// returns ErrNotFound and lets the caller (main) decide; no isatty / auto-init
// logic lives in this package. errors.Is(err, ErrNotFound) works because this
// is a sentinel created by errors.New.
var ErrNotFound = errors.New("weave is not configured; run `weave init`")

// Find is the single public entrypoint that locates the extensions directory.
// It implements the PRD §8.3 first-hit-wins priority order, trying each rule in
// turn and returning the first hit:
//
//  1. weave_EXTENSIONS_DIR env var (findEnv)
//  2. config file `store` key (findConfig)
//  3. sibling of the running binary (findSibling)
//  4. walk up from cwd (findWalkUp)
//  5. None ⇒ unconfigured: returns ("", 0, ErrNotFound)
//
// Each rule helper already returns an absolute path on a hit, so the returned
// dir is always absolute (the absDir invariant). Consumed by main.run
// (P1.M1.T4) and discover.Index (P1.M2.T3).
//
// PRD §8.2 prompt safety: Find NEVER prompts and NEVER auto-initializes. The
// unconfigured case returns ErrNotFound and the caller decides what to do —
// main prints the message to stderr and exits 1. The ONLY place weave prompts
// is `weave init` (P1.M4.T4).
func Find() (dir string, src Source, err error) {
	if d, s, ok := findEnv(); ok {
		return d, s, nil
	}
	if d, s, ok := findConfig(); ok { // PRD §8.3 priority #2 (S2)
		return d, s, nil
	}
	if d, s, ok := findSibling(); ok { // PRD §8.3 priority #3 (S2)
		return d, s, nil
	}
	if d, s, ok := findWalkUp(); ok {
		return d, s, nil
	}
	return "", 0, ErrNotFound
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
