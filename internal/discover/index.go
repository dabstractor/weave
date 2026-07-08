package discover

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
// a dir extension's internal .ts files (e.g. git-checkpoint/utils.ts) and
// avoiding descent into node_modules/. When a dir is plain (category folder),
// the callback returns nil so WalkDir descends naturally. File entries always
// return nil (NOT SkipDir — SkipDir on a file skips siblings).
//
// Error policy (skilldozer pattern, research/walkdir_skipdir_semantics.md):
//   - extensionsDir missing, unreadable, or not a directory -> returned as the
//     error. (Stat-guard BEFORE WalkDir: a missing root is otherwise swallowed
//     by the per-entry `if err != nil { return nil }` -> (nil,nil).)
//   - A per-entry error (an unreadable subtree) is SKIPPED; the walk continues.
//   - Malformed package.json does NOT abort the walk (classifyDir/classifyFile
//     ignore ParsePackageJSON's err — lenient; check M4.T2 surfaces it).
//
// filepath.WalkDir does NOT follow symlinked directories (stdlib default); a
// symlink to an extension dir is therefore not discovered. PRD §7.1 does not
// require following symlinks, and the default avoids cycles.
//
// An empty extensions dir (no entries anywhere) yields a nil slice and a nil
// error; callers test with len() (e.g. --list exits 1 "if no extensions found").
func Index(extensionsDir string) ([]Extension, error) {
	root, err := filepath.Abs(extensionsDir)
	if err != nil {
		return nil, err
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
			ext, isExt, descend := classifyDir(root, path)
			if isExt {
				result = append(result, *ext)
			}
			if !descend {
				return filepath.SkipDir // prune subtree (load-bearing recursion rule)
			}
			return nil // plain category dir → descend
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
