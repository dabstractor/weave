// (No package-level doc comment here: extension.go, P1.M2.T1.S1, owns the
// "Package discover" doc. Go uses the first package comment encountered across
// files; adding a second is ignored by godoc but is poor form.)

package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// fileExists reports whether a non-directory entry exists at path. os.Stat
// follows symlinks (matching pi's fs.existsSync semantics, and extdir.
// fileExists), so a symlink to a file counts. Used to check for index.*
// markers and existing pi.extensions entries. Mirrors extdir.fileExists —
// KEEP IN SYNC with that copy (intentional duplication across two packages
// per architecture_mapping §3c; a future shared internal/rules package is
// out of scope and would touch the Complete extdir package).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// relTagForDir computes the canonical relTag for a DIR/package extension
// (PRD §7.1 item 4): the dir path relative to root, OS separators normalized
// to '/', with NO .ts/.js stripping (dir extensions have no file suffix to
// strip — "git-checkpoint/" → "git-checkpoint", NOT "git-checkpoint/index").
// Returns "" if filepath.Rel errors (defensive; unreachable for a walk entry
// under root). PRD §7.1 item 4 strips the trailing .ts/.js ONLY for
// single-file entries; relTagForDir is the dir counterpart (no stripping).
func relTagForDir(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "" // defensive; unreachable for a walk entry under root
	}
	return filepath.ToSlash(rel)
}

// isExtensionFile reports whether name is a single-file extension per
// PRD §7.1: a *.ts or *.js file whose base name is NOT index.ts/index.js
// (those are directory-entry markers, handled by classifyDir on the parent
// dir). Mirrors extdir.isExtensionFile — KEEP IN SYNC with that copy
// (intentional duplication; the two packages re-implement the same ~5-line
// predicate because Go forbids importing unexported symbols across packages).
func isExtensionFile(name string) bool {
	if name == "index.ts" || name == "index.js" {
		return false
	}
	return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")
}

// classifyFile classifies a FILE entry discovered during the walk. Returns:
//   - (ext, true): path is a single-file extension (.ts/.js, not index.*).
//     ext is fully populated: Path==EntryFile==path (PRD §7.1: "the entry
//     path is the file"), Kind="file", and Name/Description/Keywords/Category/
//     Aliases/HasPackageJSON drawn from ParsePackageJSON of path's DIR plus
//     ExtractJSDoc(path) as the description fallback.
//   - (nil, false): path is index.ts/index.js (a dir extension's entry —
//     handled by classifyDir on the parent dir, NOT here, to avoid
//     double-counting), OR path is not a .ts/.js file.
//
// relTag is the file path relative to root, OS separators normalized to '/',
// with the trailing .ts (then .js, defensively) stripped. gate.ts → "gate";
// writing/reddit-poster.ts → "writing/reddit-poster".
//
// classifyFile COMPOSES S1 (ParsePackageJSON, BuildExtension) and S2
// (ExtractJSDoc); it does NOT reimplement JSON parsing or JSDoc extraction.
// ParsePackageJSON's error is IGNORED (lenient discovery: a malformed
// package.json yields PackageJSON{} metadata and the file still resolves;
// check M4.T2 re-parses and surfaces the error).
func classifyFile(root, path string) (*Extension, bool) {
	name := filepath.Base(path)
	if !isExtensionFile(name) {
		return nil, false // index.ts/index.js (dir's entry) OR non-ts/js
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, false // path not under root (unreachable for a walk entry)
	}
	relTag := filepath.ToSlash(rel)
	relTag = strings.TrimSuffix(relTag, ".ts")
	relTag = strings.TrimSuffix(relTag, ".js")
	pkg, hasPkg, _ := ParsePackageJSON(filepath.Dir(path)) // S1 — lenient (ignore err)
	jsdoc := ExtractJSDoc(path)                            // S2
	ext := BuildExtension(path, path, relTag, "file", pkg, hasPkg, jsdoc)
	return &ext, true
}

// classifyDir classifies a DIRECTORY entry discovered during the walk. Returns
// (ext, isExtension, shouldDescend) where:
//   - isExtension==true && shouldDescend==false: path is a recognized
//     dir/package extension (PRD §7.1 cases a/b/c). ext is fully populated.
//     The caller MUST return the skip-directory sentinel from its walk
//     callback to prune the subtree — this is the LOAD-BEARING recursion rule
//     (PRD §7.1 item 3): it prevents double-counting a dir extension's internal .ts
//     helper files (e.g. git-checkpoint/utils.ts) as separate entries,
//     mirroring pi's own loader which treats a dir as one unit.
//   - isExtension==false && shouldDescend==true: path is a PLAIN category
//     directory. The caller returns nil so the walk descends naturally.
//
// Precedence (load-bearing, matches pi's loader per pi_extension_facts.md §4:
// "package.json with pi field checked FIRST (before index.ts)"):
//
//	a) package extension: package.json with a pi.extensions array naming
//	   ≥1 EXISTING entry. EntryFile = first existing pi.extensions entry
//	   (resolved relative to path). Kind="package".
//	b) dir extension with index.ts. Kind="dir".
//	c) dir extension with index.js. Kind="dir".
//	d) plain directory — descend.
//
// A dir with BOTH a qualifying package.json (case a) AND index.ts (case b) is
// a PACKAGE (case a wins). The fileExists guard on case a enforces PRD §7.1's
// "≥1 existing entry" — a package.json whose pi.extensions names a non-existent
// file does NOT qualify and falls through to b/c/d.
//
// relTag for a/b/c has NO .ts/.js strip (relTagForDir) — git-checkpoint/ →
// "git-checkpoint". classifyDir COMPOSES S1 (ParsePackageJSON, BuildExtension)
// and S2 (ExtractJSDoc); ParsePackageJSON's error is IGNORED (lenient).
func classifyDir(root, path string) (ext *Extension, isExtension, shouldDescend bool) {
	pkg, hasPkg, _ := ParsePackageJSON(path) // S1 — lenient (ignore err)

	// Case (a): package extension — package.json with pi.extensions → existing entry.
	// Precedence: package.json is checked FIRST (pi_extension_facts.md §4).
	if hasPkg {
		entries := toStringSlice(pkg.Pi.Extensions) // S1 — []any → []string, drops non-strings
		if len(entries) > 0 {
			entryFile := filepath.Join(path, entries[0])
			if fileExists(entryFile) { // PRD §7.1: "≥1 existing entry"
				relTag := relTagForDir(root, path)
				jsdoc := ExtractJSDoc(entryFile) // S2
				e := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
				return &e, true, false // shouldDescend=false (load-bearing)
			}
			// pi.extensions names a non-existent file → NOT a package; fall through.
		}
	}

	// Case (b): dir extension with index.ts.
	if entryFile := filepath.Join(path, "index.ts"); fileExists(entryFile) {
		relTag := relTagForDir(root, path)
		jsdoc := ExtractJSDoc(entryFile)
		e := BuildExtension(path, entryFile, relTag, "dir", pkg, hasPkg, jsdoc)
		return &e, true, false // shouldDescend=false (load-bearing)
	}

	// Case (c): dir extension with index.js.
	if entryFile := filepath.Join(path, "index.js"); fileExists(entryFile) {
		relTag := relTagForDir(root, path)
		jsdoc := ExtractJSDoc(entryFile)
		e := BuildExtension(path, entryFile, relTag, "dir", pkg, hasPkg, jsdoc)
		return &e, true, false // shouldDescend=false (load-bearing)
	}

	// Case (d): plain category directory — descend.
	return nil, false, true
}
