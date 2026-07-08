package discover

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Index walks the extensions directory at extensionsDir and returns every
// extension it contains, as a []Extension sorted by canonical tag (RelTag) for
// deterministic output (PRD §7.1). It is the public catalog builder consumed by
// main.go (all modes), resolve, search, and check.
//
// extensionsDir is made absolute FIRST (filepath.Abs), so every Extension.Path
// and Extension.EntryFile is absolute — the contract behind PRD §6.1 ("absolute
// path") and the §13 acceptance gate (`case "$(./weave example)" in /*)`).
//
// classify-then-descend (PRD §7.1 item 3, load-bearing): the WalkDir callback
// dispatches each entry to classifyFile (files) or classifyDir (dirs). When a
// dir is recognized as an extension, the entry is emitted AND the callback
// returns filepath.SkipDir to prune its subtree — preventing double-counting of
// a dir extension's internal .ts files (e.g. git-checkpoint/utils.ts). When a
// dir is plain (category folder), the callback returns nil so WalkDir descends
// naturally. File entries always return nil (NOT SkipDir — SkipDir on a file
// skips siblings).
//
// Well-known non-extension directories (node_modules, .git) and hidden entries
// (any file or directory whose base name starts with '.') are skipped during
// the walk (PRD Issue 6): directories are pruned via filepath.SkipDir, hidden
// files are skipped individually (return nil, not SkipDir, so sibling
// extensions are still discovered).
//
// Error policy (skilldozer pattern, research/walkdir_skipdir_semantics.md):
//   - extensionsDir missing, unreadable, or not a directory -> returned as the
//     error. (Stat-guard BEFORE WalkDir: a missing root is otherwise swallowed
//     by the per-entry `if err != nil { return nil }` -> (nil,nil).)
//   - A per-entry error (an unreadable subtree) is SKIPPED; the walk continues.
//   - Malformed package.json does NOT abort the walk (classifyDir/classifyFile
//     ignore ParsePackageJSON's err — lenient; check M4.T2 surfaces it).
//
// The walk ROOT is symlink-resolved via filepath.EvalSymlinks before walking
// (Bug 4: a symlinked weave_EXTENSIONS_DIR is otherwise Lstat'd by WalkDir as
// ModeSymlink and walks nothing). WalkDir still does NOT follow symlinked
// DIRECTORIES within the tree (the stdlib default, which avoids cycles); only
// the store root itself is resolved. --path shows the original (possibly
// symlinked) path because it uses extdir.Find() directly, not Index().
//
// An empty extensions dir (no entries anywhere) yields a nil slice and a nil
// error; callers test with len() (e.g. --list exits 1 "if no extensions found").

// skipDirs are well-known directories that are never extension containers or
// category folders: node_modules holds npm dependencies (created by `npm
// install` at the store root to share deps across package extensions), and .git
// holds git internals. Their contents are never user-authored extensions, so
// the WalkDir callback prunes them via filepath.SkipDir (PRD Issue 6).
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
}

func Index(extensionsDir string) ([]Extension, error) {
	root, err := filepath.Abs(extensionsDir)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks on the walk root. filepath.WalkDir Lstats the root and
	// will not descend a symlinked directory (stdlib default). os.Stat below
	// follows the symlink and passes the IsDir guard, but WalkDir then sees
	// ModeSymlink and walks nothing — producing an empty catalog with no error
	// (Bug 4). EvalSymlinks resolves the chain so WalkDir sees a real directory.
	// This is a no-op for non-symlinked paths. --path still shows the original
	// symlink path because it uses extdir.Find() directly, not Index(). The fix
	// is here (not in findEnv) so TestFindEnvDoesNotResolveSymlinks stays valid
	// and ALL Find() callers are covered uniformly. If EvalSymlinks fails (e.g.
	// a broken symlink chain), fall through to os.Stat which will also fail and
	// propagate the error normally.
	if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = resolved
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err // missing/unreadable root
	}
	if !info.IsDir() {
		return nil, errors.New(root + ": not a directory")
	}

	var result []Extension // nil-init: empty store → nil slice (§3d)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // per-entry unreadable → skip, keep walking
		}
		if d.IsDir() {
			// The walk root is the store container, never an extension itself. Without
			// this guard, an index.ts at the root would classify the root as a dir
			// extension (relTag "."), then SkipDir would prune the ENTIRE store — every
			// real extension would become invisible. filepath.WalkDir visits the root
			// first as a directory entry, so this guard fires exactly once, for that
			// single visit, and always descends.
			if path == root {
				return nil // root is the store container, never an extension — always descend
			}
			// Skip well-known non-extension directories (node_modules, .git) and
			// hidden directories (base name starts with '.'). These never contain
			// user-authored extensions; descending would pollute the catalog (PRD
			// Issue 6). SkipDir prunes the entire subtree.
			name := d.Name()
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir // prune node_modules, .git, hidden dirs
			}
			ext, isExt, descend := classifyDir(root, path)
			if isExt {
				result = append(result, *ext)
			}
			if !descend {
				return filepath.SkipDir // prune subtree (load-bearing recursion rule)
			}
			return nil // plain category dir → descend
		}
		// Skip hidden files (.secret.ts, .DS_Store-as-.ts, editor dotfiles).
		// return nil (NOT SkipDir): SkipDir on a file prunes the REMAINING
		// siblings, and '.' sorts first lexically, so SkipDir would hide real
		// extensions like myext.ts that sort after the hidden file.
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext, ok := classifyFile(root, path)
		if ok {
			result = append(result, *ext)
		}
		return nil // NOT SkipDir (SkipDir on a file skips siblings — GOTCHA)
	})
	if walkErr != nil {
		return result, walkErr // defensive; walkErr is effectively always nil
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RelTag < result[j].RelTag
	})
	return result, nil
}
