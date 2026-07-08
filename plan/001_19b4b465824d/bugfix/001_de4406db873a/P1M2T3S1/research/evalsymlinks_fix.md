# Research: EvalSymlinks fix for symlinked walk root (Bug 4)

## The bug (verified)

`discover.Index` (index.go:61-72) does:
1. `root, err := filepath.Abs(extensionsDir)` — preserves the symlink path
2. `info, err := os.Stat(root)` — FOLLOWS the symlink, sees the real dir, passes IsDir guard
3. `filepath.WalkDir(root, ...)` — internally `Lstat`s the root, sees `ModeSymlink`
   (NOT a dir), and walks NOTHING → returns `(nil, nil)` → empty catalog

Result: `--path` prints the dir (exit 0, uses extdir.Find directly), but `--list`
says "no extensions found" (exit 1), `--search` finds nothing, tag resolution fails.

**Real-world trigger (fix_design.md):** on macOS `/tmp` is a symlink to
`/private/tmp`, so `weave_EXTENSIONS_DIR=/tmp/...` silently breaks discovery.

## Why the fix is in Index, NOT findEnv

`extdir.findEnv` (extdir.go:102-126) DELIBERATELY returns `filepath.Abs(val)`
WITHOUT EvalSymlinks. Two artifacts pin this:
- The package doc comment (extdir.go:11-14): "the path is returned as-is, only
  made absolute/clean via filepath.Abs — NEVER through filepath.EvalSymlinks
  (the user points exactly where they want; a symlink is preserved verbatim)."
- The existing test `TestFindEnvDoesNotResolveSymlinks` (extdir_test.go:201):
  creates a symlink, calls findEnv, asserts the returned path == the symlink path
  (NOT the resolved target). Asserts `got == realDir` would FAIL.

Fixing in Index (not findEnv) keeps `TestFindEnvDoesNotResolveSymlinks` valid AND
covers ALL extdir.Find() callers (env, config, sibling, walk-up) uniformly — they
all funnel through discover.Index. `--path` still shows the original symlink path
(it uses extdir.Find directly, not Index).

## The fix (verified insertion point)

Current index.go preamble (lines 61-72):
```go
func Index(extensionsDir string) ([]Extension, error) {
	root, err := filepath.Abs(extensionsDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)   // ← line 66; insertion point is BEFORE this
	if err != nil {
		return nil, err
	}
	...
```

**There are ZERO lines between line 65 (`}` closing Abs-err-check) and line 66
(`info, err := os.Stat(root)`).** Insert the EvalSymlinks block between them:

```go
	// Resolve symlinks on the walk root. filepath.WalkDir Lstats the root and
	// will not descend a symlinked directory (stdlib default). os.Stat below
	// follows the symlink and passes the IsDir guard, but WalkDir then sees
	// ModeSymlink and walks nothing — producing an empty catalog with no error
	// (Bug 4). EvalSymlinks resolves the chain so WalkDir sees a real directory.
	// --path still shows the original symlink path because it uses extdir.Find()
	// directly, not Index(). The fix is here (not in findEnv) so the existing
	// TestFindEnvDoesNotResolveSymlinks stays valid and ALL Find() callers are
	// covered uniformly.
	if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = resolved
	}
	info, err := os.Stat(root)
```

### Why `if rerr == nil` (not propagating the EvalSymlinks error)

If EvalSymlinks FAILS (e.g. a broken symlink chain, or a path that doesn't
exist), we fall through to `os.Stat(root)` which will ALSO fail and propagate
THE STAT error (the more accurate "does not exist" message). If we propagated
EvalSymlinks' error directly, a missing root would report EvalSymlinks' wording
instead of the familiar os.Stat wording that existing tests pin
(`TestIndexMissingRoot` asserts `err != nil` but not the message — but keeping
the Stat path is the cleaner contract).

**No-op for non-symlinks:** EvalSymlinks on a real path returns the same path
(Clean'd). So the fix is invisible for the normal case — every existing test
passes unchanged.

## No import changes needed

`path/filepath` is ALREADY imported in index.go (line 6). `filepath.EvalSymlinks`
needs no new import. index_test.go already imports `os`, `path/filepath`,
`strings`, `testing` — all needed for the new test are present.

## The test (mirrors existing os.Symlink idiom)

Pattern from extdir_test.go:201 (TestFindEnvDoesNotResolveSymlinks):
```go
realDir := t.TempDir()
writeFile(t, realDir, "foo.ts", "/** foo */\n")   // a real extension
parent := t.TempDir()
link := filepath.Join(parent, "link-to-ext")
if err := os.Symlink(realDir, link); err != nil {
	t.Skipf("symlinks not supported on this platform: %v", err)
}
got, err := Index(link)
// assert err==nil, len(got)==1, got[0].RelTag=="foo"
```

The `t.Skipf` guard is load-bearing: symlink creation fails on restricted
filesystems / some CI sandboxes (Windows without admin, certain containers).
Both existing os.Symlink tests use it.

**Placement:** after `TestIndexSkipsGitDir` (index_test.go:350-375, the LAST
function in the file — EOF). Append `TestIndexSymlinkedRoot` there.

## Sources
- /home/dustin/projects/weave/internal/discover/index.go (lines 61-72 — the preamble; line 66 insertion point)
- /home/dustin/projects/weave/internal/extdir/extdir.go (lines 11-14, 102-126 — findEnv contract)
- /home/dustin/projects/weave/internal/extdir/extdir_test.go (lines 201-226 — TestFindEnvDoesNotResolveSymlinks, the os.Symlink pattern)
- /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md §Bug 4
- /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/bug_cascade_map.md §Bug 6 (symlink)
- /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/test_patterns.md (writeFile, relTags helpers; os.Symlink + t.Skipf idiom)
- https://pkg.go.dev/path/filepath#EvalSymlinks (resolves the chain; error if any link can't be read)
- https://pkg.go.dev/path/filepath#WalkDir (Lstats the root; does not follow symlinked dirs)
