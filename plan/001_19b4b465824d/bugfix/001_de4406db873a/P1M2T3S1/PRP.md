# PRP — P1.M2.T3.S1: Add EvalSymlinks resolution for walk root in Index

(Bugfix 001_de4406db873a — Issue 4, Minor. Discovery subsystem,
`internal/discover/index.go`.)

## Goal

**Feature Goal**: Fix `discover.Index` (in `internal/discover/index.go`) so a
symlinked `weave_EXTENSIONS_DIR` is fully resolved before the `filepath.WalkDir`
call, producing a correct catalog instead of an empty one. Today `Index` calls
`filepath.Abs` (preserves the symlink) then `os.Stat` (follows the symlink and
passes the `IsDir` guard), then `filepath.WalkDir(root, ...)`. But `WalkDir`
internally `Lstat`s the root, sees `ModeSymlink` (not a dir), and walks NOTHING —
returning `(nil, nil)` (an empty catalog with NO error). Result: `--path` prints
the dir (exit 0, it uses `extdir.Find` directly), but `--list` says "no
extensions found" (exit 1) and every tag resolves to unknown — a confusing
`--path`-vs-reality mismatch.

The fix adds a single `filepath.EvalSymlinks(root)` call between the existing
`filepath.Abs` error check and the `os.Stat` call, reassigning `root` to the
resolved target on success. It is a no-op for non-symlinked paths (EvalSymlinks
returns the same path Clean'd), and it composes with the sibling fixes already
landed in the same file (root-skip guard P1.M2.T1.S1, skipDirs/hidden
P1.M2.T2.S1).

**Deliverable**: ONE localized code edit + ONE regression test:
- `internal/discover/index.go`: insert the EvalSymlinks block (6 lines + a
  comment) between line 65 (the `}` closing the `filepath.Abs` error check) and
  line 66 (`info, err := os.Stat(root)`). NO import change (`path/filepath` is
  already imported). NO signature change. Update the `Index` doc comment to note
  the symlink resolution (Mode A).
- `internal/discover/index_test.go`: add `TestIndexSymlinkedRoot` — create a real
  dir with a `foo.ts` extension, symlink the dir, call `Index` on the symlink
  path, assert the extension is discovered. Guard with `t.Skipf` for platforms
  without symlink support.

No other files change.

**Success Definition**:
- A symlinked extensions dir containing `foo.ts` is discovered by `Index` as
  exactly ONE entry with `RelTag == "foo"` (NOT an empty catalog).
- A NON-symlinked extensions dir behaves IDENTICALLY before and after the fix
  (EvalSymlinks is a no-op for real paths) — every existing `TestIndex*` test
  still passes unchanged.
- `--path` still shows the ORIGINAL (possibly symlinked) path — the fix is in
  `discover.Index`, NOT `extdir.findEnv`, so `TestFindEnvDoesNotResolveSymlinks`
  (extdir_test.go:201) stays valid.
- `Extension.Path` and `Extension.EntryFile` for a symlinked root point to the
  RESOLVED target (the real filesystem location) — correct for `pi -e`
  consumption (pi loads the real file).
- A broken symlink chain (EvalSymlinks fails) falls through to `os.Stat`, which
  also fails and propagates the familiar Stat error — `TestIndexMissingRoot`
  still passes (it asserts `err != nil`, not the message).
- `go test ./internal/discover/ -count=1` passes; `go vet ./internal/discover/`
  clean; `go build ./...` exits 0.

## User Persona (if applicable)

**Target User**: weave CLI users whose `weave_EXTENSIONS_DIR` (env var) or
config `store` path passes through a symlink. The most common real-world trigger
(fix_design.md §Bug 4): on **macOS**, `/tmp` is a symlink to `/private/tmp`, so
`weave_EXTENSIONS_DIR=/tmp/my-extensions` silently breaks discovery. Also:
users who symlink their store from another volume, a dotfiles repo, or a shared
network mount.

**Use Case**: `weave --list`, `weave <tag>`, `weave check` against a store
reached via a symlink. Today `--path` cheerfully reports the dir (exit 0) while
`--list` says "no extensions found" (exit 1) — the user sees a contradiction and
has no clue the symlink is the cause.

**Pain Points Addressed**: The `--path`-vs-reality mismatch. A user runs
`weave --path`, sees `/tmp/my-extensions` (exit 0, looks configured correctly),
then runs `weave --list` and gets "no extensions found" (exit 1). They waste
time debugging permissions, file extensions, or package.json — never suspecting
the symlink. After the fix, both commands agree: the catalog is non-empty.

## Why

- **Closes a confusing --path-vs-reality gap (the bug PRD Issue 4)**: PRD §8.3
  rule 1 says "if [the env var] is set and an existing dir, use it." The intent
  is functional (use that store), not literal-path-preservation. `findEnv`
  validates the path with `os.Stat` (follows the symlink → passes) but returns
  it verbatim (deliberately, per its doc comment). The validation says "this is
  a usable dir" but the walk then can't see it. The fix makes the walk honor the
  validation's finding.
- **Preserves the findEnv contract (fix-in-Index, not fix-in-findEnv)**:
  `extdir.findEnv` DELIBERATELY does NOT EvalSymlinks — its doc comment
  (extdir.go:11-14) says "the path is returned as-is ... NEVER through
  filepath.EvalSymlinks (the user points exactly where they want; a symlink is
  preserved verbatim)." The existing test `TestFindEnvDoesNotResolveSymlinks`
  (extdir_test.go:201) PINS this: it asserts the returned path equals the
  symlink path (not the resolved target). Fixing in `discover.Index` keeps that
  test valid AND covers ALL `extdir.Find()` callers (env, config, sibling,
  walk-up) uniformly — they all funnel through `Index`.
- **Composes with the sibling index.go fixes**: Per `bug_cascade_map.md`, Bug 4
  (root-skip, P1.M2.T1.S1 — LANDED), Issue-6/skipDirs (P1.M2.T2.S1 — LANDED),
  and this fix (Bug 6/symlink) all live in the SAME `index.go`. This fix edits
  the `root` variable ABOVE the WalkDir callback, so it does not interact with
  the root-skip guard (`if path == root`) or the skipDirs/hidden checks
  (`d.Name()` is per-entry, independent of whether root was a symlink). The
  three fixes coexist without conflict.
- **No downstream contract change**: `Index` still returns `[]Extension` sorted
  by `RelTag`; for a symlinked root it returns the CORRECT entries (vs. empty
  before). `Extension.Path`/`EntryFile` point to the resolved target — what
  `pi -e` needs (pi loads the real file). No consumer code changes.

## What

### The change to `internal/discover/index.go`

**1. Insert the EvalSymlinks block** between line 65 (the `}` closing the
`filepath.Abs` error check) and line 66 (`info, err := os.Stat(root)`). Currently
there are ZERO lines between them; this is the single insertion point.

```go
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
	// ... rest UNCHANGED ...
```

**2. Update the `Index` doc comment** (Mode A, lines 11-58): the comment at
lines 56-58 currently says "filepath.WalkDir does NOT follow symlinked
directories (stdlib default); a symlink to an extension dir is therefore not
discovered. PRD §7.1 does not require following symlinks, and the default avoids
cycles." Replace those lines with a note that the WALK ROOT is symlink-resolved
(so a symlinked store IS discovered), while WalkDir still does not follow
symlinked DIRECTORIES within the tree (the stdlib cycle-avoidance default is
preserved for nested symlinks).

### Success Criteria

- [ ] `filepath.EvalSymlinks(root)` is called in `Index` BETWEEN the
      `filepath.Abs` error check (line 65) and `os.Stat(root)` (line 66).
- [ ] The call uses the `if resolved, rerr := filepath.EvalSymlinks(root);
      rerr == nil { root = resolved }` form — EvalSymlinks errors do NOT mask
      the subsequent Stat error (fall through on failure).
- [ ] NO new import is added (`path/filepath` is already imported at line 6).
- [ ] NO signature change to `Index`.
- [ ] The `Index` doc comment is updated (Mode A) to note the walk-root symlink
      resolution and that nested symlinked directories are still NOT followed.
- [ ] `TestIndexSymlinkedRoot` exists in `index_test.go` and passes: a symlinked
      dir with `foo.ts` is discovered as exactly ONE entry, `RelTag == "foo"`.
- [ ] `TestIndexSymlinkedRoot` guards `os.Symlink` with `t.Skipf` for platforms
      without symlink support.
- [ ] ALL pre-existing `TestIndex*` tests pass unchanged (EvalSymlinks is a
      no-op for the real dirs they use — `t.TempDir()` returns real paths).
- [ ] `TestFindEnvDoesNotResolveSymlinks` (extdir_test.go:201) STILL passes
      (findEnv is NOT modified).
- [ ] `go test ./internal/discover/ -count=1` → ok; `go vet ./internal/discover/`
      clean; `go build ./...` exits 0.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes.** This PRP quotes the exact current
`index.go` preamble (lines 61-72, with the verified zero-line gap at the
insertion point), the exact block to insert (with the `rerr == nil` fall-through
form and a full explanatory comment), the exact test to add (mirroring the
existing `os.Symlink` + `t.Skipf` idiom from extdir_test.go:201), the
findEnv-does-not-resolve contract (so the fixer knows NOT to touch findEnv), and
the EvalSymlinks semantics (no-op for real paths, error on broken chains). The
fix is a 7-line insertion into an existing function — no restructuring.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/internal/discover/index.go
  why: CONTAINS THE CODE TO EDIT. The Index preamble (lines 61-72) is where the
       EvalSymlinks block goes. Lines 62-65 are filepath.Abs + err check; line 66
       is os.Stat. The insertion point is BETWEEN line 65 and line 66 (currently
       zero lines apart). The Index doc comment (lines 11-58) gets the Mode A update.
  pattern: |
    # Current preamble (verified lines 61-72) — the insertion point is line 66:
    func Index(extensionsDir string) ([]Extension, error) {
        root, err := filepath.Abs(extensionsDir)
        if err != nil {
            return nil, err
        }
        # ← INSERT EvalSymlinks block HERE (between this } and os.Stat)
        info, err := os.Stat(root)
        if err != nil {
            return nil, err
        }
        if !info.IsDir() {
            return nil, errors.New(root + ": not a directory")
        }
  gotcha: |
    1. path/filepath is ALREADY imported (line 6). Do NOT add it again (unused/dup
       import fails go build). EvalSymlinks needs no new import.
    2. Use `if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil { root =
       resolved }` — do NOT propagate rerr. On a broken symlink chain, EvalSymlinks
       fails AND os.Stat also fails; letting Stat run produces the familiar
       "no such file" error that TestIndexMissingRoot expects (it asserts err != nil,
       not the message, but the Stat path is the cleaner contract).
    3. Do NOT reorder — the EvalSymlinks MUST come AFTER filepath.Abs (so the root
       is absolute first — EvalSymlinks on a relative path resolves against cwd,
       which is correct but Abs-first is the established pattern) and BEFORE os.Stat
       (so Stat validates the RESOLVED path). The WalkDir call (line 75) then
       receives the resolved root.

- file: /home/dustin/projects/weave/internal/extdir/extdir.go  (READ-ONLY — do NOT edit)
  why: findEnv (lines 102-126) is the ALTERNATIVE fix location that we are
       DELIBERATELY NOT using. It returns filepath.Abs(val) without EvalSymlinks.
       The package doc (lines 11-14) and findEnv doc (~89-101) state this is
       intentional: "the user points exactly where they want; a symlink is
       preserved verbatim." The existing test TestFindEnvDoesNotResolveSymlinks
       PINS this. Fixing in Index (not findEnv) keeps that test valid and covers
       all Find() callers uniformly.
  critical: Do NOT modify extdir.go. The fix is entirely in discover/index.go.
            findEnv's verbatim-symlink contract is a feature (--path shows the
            user's literal path), not the bug.

- file: /home/dustin/projects/weave/internal/extdir/extdir_test.go  (READ-ONLY — pattern source)
  why: TestFindEnvDoesNotResolveSymlinks (line 201) is the os.Symlink test pattern
       to MIRROR for TestIndexSymlinkedRoot. It uses realDir + parent temp dirs,
       os.Symlink(realDir, link), and the t.Skipf guard. READ it to copy the idiom.
  pattern: |
    realDir := t.TempDir()
    parent := t.TempDir()
    link := filepath.Join(parent, "link-to-ext")
    if err := os.Symlink(realDir, link); err != nil {
        t.Skipf("symlinks not supported on this platform: %v", err)
    }
    # then call Index(link) and assert entries
  critical: The t.Skipf guard is LOAD-BEARING — os.Symlink fails on restricted
            filesystems and some CI sandboxes. Both existing os.Symlink tests use it.

- file: /home/dustin/projects/weave/internal/discover/index_test.go
  why: WHERE THE TEST GOES. White-box (`package discover`), already imports os,
       path/filepath, strings, testing (verified lines 3-8 — NO import change
       needed). Reuses writeFile (jsdoc_test.go) and relTags (discover_test.go)
       WITHOUT redeclaring. The LAST function is TestIndexSkipsGitDir (line 350,
       ends at line 375 / EOF) — append TestIndexSymlinkedRoot after it.
  pattern: |
    # Idiom (mirrors TestIndexMissingRoot + the extdir os.Symlink pattern):
    func TestIndexSymlinkedRoot(t *testing.T) {
        realDir := t.TempDir()
        writeFile(t, realDir, "foo.ts", "/** foo */\n")
        parent := t.TempDir()
        link := filepath.Join(parent, "link-to-ext")
        if err := os.Symlink(realDir, link); err != nil {
            t.Skipf("symlinks not supported on this platform: %v", err)
        }
        got, err := Index(link)
        if err != nil { t.Fatalf("err=%v; want nil", err) }
        if len(got) != 1 || got[0].RelTag != "foo" {
            t.Fatalf("got=%v; want one extension 'foo' (symlinked root must resolve)", relTags(got))
        }
    }
  critical: Use os.Symlink(realDir, link) — the FIRST arg is the target (realDir),
            the SECOND is the link being created. Swapping them makes link point at
            nothing. Assert RelTag=="foo" (the .ts suffix is stripped by classifyFile).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md
  why: §Bug 4 gives the EXACT fix (the EvalSymlinks block with the rerr==nil guard)
       and the regression test. This PRP quotes it verbatim with verified line numbers.
  critical: fix_design confirms the macOS /tmp → /private/tmp real-world trigger
            (cross-platform relevance) and the "no mocking needed" / "fall through
            to os.Stat on failure" design.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/bug_cascade_map.md
  why: §Bug 6 (symlink root) confirms the location (index.go, before the walk) and
       the dependency graph: Bug 4 (root-skip M2.T1 — LANDED), Issue6 (skipDirs
       M2.T2 — LANDED), and this fix (M2.T3) are all in index.go, landed
       sequentially. The cross-bug graph shows they compose.
  critical: This fix (M2.T3) edits the `root` variable ABOVE the WalkDir callback.
            It does NOT touch the root-skip guard (M2.T1) or the skipDirs/hidden
            checks (M2.T2) — those operate on d.Name() per-entry, independent of
            whether root was a symlink. No conflict.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/test_patterns.md
  why: Confirms the helpers (writeFile, relTags) and the os.Symlink + t.Skipf idiom.
       Lists "Symlinked root" under "Existing related tests (gaps to fill)" with the
       note: "findEnv only; no Index test" — confirming TestIndexSymlinkedRoot fills
       a real gap (no existing discover-package symlink test).
  critical: The gap table confirms there is NO existing Index-level symlink coverage.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T2S1/PRP.md  (parallel, LANDED)
  why: The skipDirs PRP, ALREADY IMPLEMENTED. It added the "strings" import, the
       skipDirs var, and the node_modules/.git/hidden guards to the SAME WalkDir
       callback. Its "SAME-FILE SIBLING FIXES" section anticipated THIS task:
       "P1.M2.T3.S1 (symlink EvalSymlinks) edits the `root` variable ABOVE the
       callback (filepath.EvalSymlinks). Does NOT affect d.Name()." The two compose.
  critical: Do NOT re-add the "strings" import or skipDirs (they are already there).
            Do NOT touch the skipDirs/hidden/root-skip guards. Insert ONLY the
            EvalSymlinks block in the preamble.

- url: https://pkg.go.dev/path/filepath#EvalSymlinks
  why: Authoritative EvalSymlinks semantics. "EvalSymlinks returns the path name
       after the evaluation of any symbolic links. If path is relative the result
       will be relative to the current directory, unless one of the components is
       an absolute symbolic link." It returns the SAME path (Clean'd) for a
       non-symlinked path, and an error if any link in the chain can't be read.
  critical: The no-op property for real paths is why every existing test passes
            unchanged. The error-on-broken-chain property is why we use
            `rerr == nil` (fall through to os.Stat).

- url: https://pkg.go.dev/path/filepath#WalkDir
  why: "WalkDir does not follow symbolic links." This is the ROOT CAUSE — WalkDir
       Lstats the root; a symlinked root reports IsDir()==false; WalkDir walks
       nothing. EvalSymlinks resolves the root to a real dir before WalkDir sees it.
  critical: WalkDir STILL won't follow symlinked DIRECTORIES *within* the tree
            (nested symlinks). The fix resolves ONLY the walk root. This is the
            intended scope (cycle avoidance for nested symlinks is preserved).
```

### Current Codebase tree (the files this subtask touches)

```bash
internal/discover/
├── discover.go          # (read-only) — classifyDir/classifyFile
├── discover_test.go     # (read-only) — walkClassified, relTags helper
├── extension.go         # (read-only) — BuildExtension, Extension, parsePackageJSON
├── extension_test.go    # (read-only) — strEq helper
├── index.go             # ← EDIT: insert EvalSymlinks block (lines 65-66 gap) + doc comment
├── index_test.go        # ← EDIT: + TestIndexSymlinkedRoot (append after line 375)
├── jsdoc.go             # (read-only) — ExtractJSDoc
└── jsdoc_test.go        # (read-only) — writeFile helper
internal/extdir/
├── extdir.go            # (read-only — do NOT edit) — findEnv (deliberately no EvalSymlinks)
└── extdir_test.go       # (read-only) — TestFindEnvDoesNotResolveSymlinks (os.Symlink pattern source)
```

### Desired Codebase tree (no NEW files; edits only)

```bash
# No files added/deleted. Two existing files edited:
internal/discover/index.go        # + EvalSymlinks block (7 lines incl. comment) between Abs-check and os.Stat; + doc comment touch-up
internal/discover/index_test.go   # + TestIndexSymlinkedRoot (append at EOF, after TestIndexSkipsGitDir)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (fix in Index, NOT findEnv): extdir.findEnv DELIBERATELY returns the
// verbatim symlink path (filepath.Abs only, no EvalSymlinks). Its doc comment
// (extdir.go:11-14) and the existing test TestFindEnvDoesNotResolveSymlinks
// (extdir_test.go:201) PIN this. The fix MUST go in discover.Index so that test
// stays valid AND all Find() callers (env/config/sibling/walk-up) are covered
// uniformly. Editing findEnv would break its contract and the pinned test.

// CRITICAL (use `rerr == nil`, do NOT propagate EvalSymlinks error): on a broken
// symlink chain, EvalSymlinks fails. If we returned that error, a missing root
// would report EvalSymlinks' wording instead of the familiar os.Stat "no such
// file" message. Instead, fall through: `if resolved, rerr := ...; rerr == nil
// { root = resolved }`. The subsequent os.Stat will ALSO fail (the path is
// broken) and propagate ITS error — the one TestIndexMissingRoot and users
// expect. EvalSymlinks succeeding but Stat failing is impossible (EvalSymlinks
// resolves to a real path; Stat then succeeds).

// CRITICAL (no-op for real paths — all existing tests pass unchanged):
// filepath.EvalSymlinks on a non-symlinked path returns the same path (Clean'd).
// Every existing TestIndex* test uses t.TempDir() (a real path), so the fix is
// invisible to them. Do NOT add special-casing for "is this a symlink?" — just
// call EvalSymlinks unconditionally (guarded by rerr==nil).

// CRITICAL (no new import): path/filepath is ALREADY imported at index.go line 6.
// Adding it again is a duplicate-import compile error. EvalSymlinks is in the
// already-imported package.

// CRITICAL (insertion order: AFTER Abs, BEFORE Stat, BEFORE WalkDir): the
// EvalSymlinks block goes between line 65 (Abs err check close) and line 66
// (os.Stat). Abs first (root is absolute), then EvalSymlinks (resolves the
// absolute symlink chain), then Stat (validates the resolved real path), then
// WalkDir (walks the resolved real dir). Reordering breaks the contract: if
// EvalSymlinks ran before Abs, a relative symlink target would resolve against
// cwd (correct but inconsistent with the established Abs-first pattern); if it
// ran after Stat, Stat would validate the symlink (passes) but WalkDir would
// still see ModeSymlink.

// GOTCHA (WalkDir STILL does not follow nested symlinked directories): the fix
// resolves ONLY the walk root. A symlinked DIRECTORY *inside* the tree (e.g.
// extensions/category/link-to-other-dir/) is still NOT followed (WalkDir's
// stdlib default, which avoids cycles). This is the intended scope — the bug is
// about the STORE root being a symlink, not nested dirs. The doc comment update
// (Mode A) must preserve this distinction: root IS resolved; nested symlinked
// dirs are NOT.

// GOTCHA (Extension.Path/EntryFile point to the RESOLVED target): after the fix,
// root is the resolved real path. classifyFile/classifyDir receive `path`
// arguments that are under the resolved root, so Extension.Path and EntryFile
// are absolute paths to the REAL files (not through the symlink). This is
// CORRECT for `pi -e` consumption (pi loads the real file) and for the PRD §6.1
// absolute-path contract. --path still shows the symlink path (it uses
// extdir.Find, not Index) — the two commands intentionally show different
// things (--path = where the user pointed; --list/resolve = the real files).

// GOTCHA (os.Symlink arg order: target, then link): `os.Symlink(target, link)`
// creates `link` pointing at `target`. In the test: os.Symlink(realDir, link) —
// realDir is the target (the real extensions dir), link is the symlink being
// created. Swapping them creates a symlink to a non-existent path. Both existing
// os.Symlink tests (extdir_test.go:205, :465) use the correct order — mirror them.

// GOTCHA (t.Skipf guard is load-bearing for the test): os.Symlink fails on
// restricted filesystems (Windows without Developer Mode / admin, some CI
// sandboxes, read-only mounts). Both existing os.Symlink tests guard with
// `t.Skipf("symlinks not supported on this platform: %v", err)`. The new test
// MUST do the same or it will FAIL (not skip) on those platforms. t.Skipf marks
// the test as skipped, not failed.

// GOTCHA (white-box test, reuse helpers): index_test.go is `package discover`.
// It reuses writeFile (jsdoc_test.go) and relTags (discover_test.go) — do NOT
// redeclare them (Go redefinition error). It already imports os, path/filepath,
// strings, testing — NO import change needed for TestIndexSymlinkedRoot.

// GOTCHA (append at EOF, after TestIndexSkipsGitDir): index_test.go's last
// function is TestIndexSkipsGitDir (line 350, ending at line 375). Append
// TestIndexSymlinkedRoot after it to keep walk-edge-case tests grouped at the
// end. Do NOT insert in the middle (would shift line numbers for no benefit).
```

## Implementation Blueprint

### Data models and structure

None. No types, no fields, no vars, no helpers. The change is a 7-line insertion
(6 code lines + the comment is part of the block) into an existing function plus
a doc-comment touch-up, and one test function.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/discover/index.go — insert EvalSymlinks block
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: the Index function preamble. Line 62: `root, err := filepath.Abs(
          extensionsDir)`. Line 65: `}` closing the Abs error check. Line 66:
          `info, err := os.Stat(root)`. The insertion point is BETWEEN line 65
          and line 66 (currently adjacent — zero lines between them).
  INSERT (between the Abs-err-check `}` and the `os.Stat` call):
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
  PRESERVE: the filepath.Abs call + its err check (lines 62-65) UNCHANGED; the
            os.Stat call + its err check + the IsDir guard (lines 66-72) UNCHANGED;
            the WalkDir callback, the sort, the return — ALL UNCHANGED.
  CRITICAL: use `rerr == nil` (fall through on EvalSymlinks failure), NOT
            propagating rerr. Do NOT add a path/filepath import (already present).

Task 2: EDIT internal/discover/index.go — update Index doc comment (Mode A)
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: the Index doc comment paragraph at lines 56-58 (approx), which currently
          reads:
            "filepath.WalkDir does NOT follow symlinked directories (stdlib
             default); a symlink to an extension dir is therefore not discovered.
             PRD §7.1 does not require following symlinks, and the default avoids
             cycles."
  REPLACE with:
            "The walk ROOT is symlink-resolved via filepath.EvalSymlinks before
             walking (Bug 4: a symlinked weave_EXTENSIONS_DIR is otherwise
             Lstat'd by WalkDir as ModeSymlink and walks nothing). WalkDir still
             does NOT follow symlinked DIRECTORIES within the tree (the stdlib
             default, which avoids cycles); only the store root itself is resolved.
             --path shows the original (possibly symlinked) path because it uses
             extdir.Find() directly, not Index()."
  PRESERVE: the rest of the doc comment (filepath.Abs, classify-then-descend,
            skipDirs/hidden, error policy, nil-on-empty) UNCHANGED.
  WHY: Mode A doc ride-along (item contract point 5, DOCS). The old comment
       documented the buggy behavior as if it were intended; the new comment
       documents the fix and the root-vs-nested distinction.

Task 3: EDIT internal/discover/index_test.go — add TestIndexSymlinkedRoot
  FILE: /home/dustin/projects/weave/internal/discover/index_test.go
  PLACEMENT: APPEND at EOF (after TestIndexSkipsGitDir, which ends at line 375 —
             the last function in the file). Keeps walk-edge-case tests grouped.
  ADD (mirror the os.Symlink idiom from extdir_test.go:201 + TestIndexMissingRoot):

    // TestIndexSymlinkedRoot reproduces Bug 4: a symlinked weave_EXTENSIONS_DIR is
    // validated by os.Stat (follows the symlink, passes the IsDir guard) but
    // filepath.WalkDir Lstats the root, sees ModeSymlink, and walks nothing —
    // yielding an empty catalog. The EvalSymlinks resolution in Index fixes this.
    // On macOS /tmp is a symlink to /private/tmp, so this is a real-world trigger.
    func TestIndexSymlinkedRoot(t *testing.T) {
        realDir := t.TempDir()
        writeFile(t, realDir, "foo.ts", "/** foo extension. */\nexport default function(){}\n")

        // Create a symlink to the real extensions dir in a separate temp dir.
        parent := t.TempDir()
        link := filepath.Join(parent, "link-to-ext")
        if err := os.Symlink(realDir, link); err != nil {
            t.Skipf("symlinks not supported on this platform: %v", err)
        }

        got, err := Index(link)
        if err != nil {
            t.Fatalf("Index(symlink): err=%v; want nil (symlinked root must resolve and walk)", err)
        }
        if len(got) != 1 {
            t.Fatalf("len(got)=%d; want 1 (symlinked root must discover foo.ts, not walk nothing). Entries: %v",
                len(got), relTags(got))
        }
        if got[0].RelTag != "foo" {
            t.Errorf("RelTag=%q; want foo (the .ts suffix is stripped by classifyFile)", got[0].RelTag)
        }
        // The resolved Path/EntryFile point at the REAL dir (the symlink target),
        // not through the symlink — correct for pi -e consumption.
        if !strings.HasPrefix(got[0].Path, realDir) {
            t.Errorf("Path=%q; want it under the resolved real dir %q (pi -e loads the real file)", got[0].Path, realDir)
        }
    }
  NOTE: index_test.go ALREADY imports os, path/filepath, strings, testing (verified
        lines 3-8) — NO import change needed. writeFile (jsdoc_test.go) and relTags
        (discover_test.go) already exist — do NOT redeclare them. strings.HasPrefix
        is used for the Path-prefix assertion (realDir is a temp dir; the resolved
        Path is realDir/foo.ts).
  ASSERTS: err==nil (symlink resolved, walk ran); len(got)==1 (not empty);
           RelTag=="foo" (correct classification, suffix stripped); Path under realDir
           (resolved target, not the symlink).

Task 4: VALIDATE
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -run 'TestIndexSymlinkedRoot' -v -count=1
    EXPECT: PASS (or SKIP on a platform without symlink support — never FAIL).
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -run 'TestIndex' -v -count=1
    EXPECT: all PASS (new + existing — confirms EvalSymlinks is a no-op for real paths).
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -count=1
    EXPECT: ok github.com/dabstractor/weave/internal/discover.
  - RUN: cd /home/dustin/projects/weave && go test ./internal/extdir/ -run 'TestFindEnvDoesNotResolveSymlinks' -v -count=1
    EXPECT: PASS (findEnv is NOT modified — its contract is intact).
  - RUN: cd /home/dustin/projects/weave && go vet ./internal/discover/
    EXPECT: clean.
  - RUN: cd /home/dustin/projects/weave && go build ./...
    EXPECT: exit 0.
  - RUN: cd /home/dustin/projects/weave && gofmt -l internal/discover/index.go internal/discover/index_test.go
    EXPECT: no output.
  - RUN: cd /home/dustin/projects/weave && grep -c "EvalSymlinks" internal/discover/index.go
    EXPECT: >= 1 (the call exists). (The doc comment + the call may make it 2.)
  - RUN: cd /home/dustin/projects/weave && grep -q '"path/filepath"' internal/discover/index.go && echo "filepath imported (no dup needed)" || echo "FAIL"
  - OPTIONAL manual repro (the bug PRD's Issue 4):
        cd /home/dustin/projects/weave && go build -o /tmp/weave .
        rm -rf /tmp/s1 /tmp/s1link && mkdir -p /tmp/s1
        printf '/** ext */\nexport default function(){}\n' > /tmp/s1/foo.ts
        ln -sfn /tmp/s1 /tmp/s1link
        weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --list   # EXPECT: foo (rc=0)
        weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --path   # EXPECT: /tmp/s1link (rc=0, original symlink path)
        rm -f /tmp/weave; rm -rf /tmp/s1 /tmp/s1link
```

### Implementation Patterns & Key Details

```go
// PATTERN: the EvalSymlinks block with fall-through-on-error. The guard form
// `if resolved, rerr := ...; rerr == nil { root = resolved }` ensures a broken
// symlink chain falls through to os.Stat (which produces the familiar error),
// rather than masking it with EvalSymlinks' wording.
func Index(extensionsDir string) ([]Extension, error) {
	root, err := filepath.Abs(extensionsDir)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks on the walk root (Bug 4). [full comment from Task 1]
	if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = resolved
	}
	info, err := os.Stat(root)
	// ... rest unchanged
}

// CRITICAL: do NOT propagate rerr. On a broken chain, EvalSymlinks fails AND
// os.Stat fails; Stat's error is the one users/tests expect. EvalSymlinks
// succeeding + Stat failing is impossible (EvalSymlinks resolved to a real path).

// PATTERN: the symlink test mirrors extdir_test.go:201 (os.Symlink + t.Skipf).
realDir := t.TempDir()
writeFile(t, realDir, "foo.ts", "/** foo */\n")
parent := t.TempDir()
link := filepath.Join(parent, "link-to-ext")
if err := os.Symlink(realDir, link); err != nil {  // target=realDir, link=link
	t.Skipf("symlinks not supported on this platform: %v", err)
}
got, err := Index(link)
// len(got)==1, RelTag=="foo", Path under realDir (resolved target)
```

### Integration Points

```yaml
DATABASE:
  - none. Pure stdlib file code.

CONFIG:
  - none. No config, env vars, or settings touched. (The bug is triggered BY an
    env var value being a symlink, but the fix does not read or change env vars.)

ROUTES / API:
  - none. weave is a CLI; this is an internal discovery function.

DISCOVERY SUBSYSTEM (the integration surface):
  - Index is the public catalog builder consumed by main.go (all modes), resolve,
    search, check, and ui. The fix changes Index's OUTPUT for symlinked roots:
    it returns the CORRECT entries (vs. empty before). Every consumer already
    assumes "one Extension per real entry, sorted by RelTag" — the fix makes the
    symlinked case match the non-symlinked case. No consumer code changes.

EXTDIR SUBSYSTEM (NOT modified — the load-bearing constraint):
  - extdir.findEnv DELIBERATELY does NOT EvalSymlinks. Its doc comment and the
    existing test TestFindEnvDoesNotResolveSymlinks PIN this. --path shows the
    user's literal (possibly symlinked) path. The fix is in Index, so findEnv's
    contract is untouched and its test stays green. This is the key design
    decision: resolve at the WALK boundary, not the LOCATION boundary.

SAME-FILE SIBLING FIXES (cross-task dependencies, NOT this task):
  - P1.M2.T1.S1 (Issue 3: root-skip guard) — LANDED at index.go lines 81-85
    (the `if path == root` guard inside the WalkDir callback). This fix edits the
    `root` variable ABOVE the callback. No conflict: after EvalSymlinks, `root`
    is the resolved real path; `path == root` still correctly identifies the root
    visit (WalkDir visits the resolved root first).
  - P1.M2.T2.S1 (Issue 6: skipDirs/hidden) — LANDED. The skipDirs var + guards
    are in the WalkDir callback (lines 94, 110 approx). This fix does not touch
    them. d.Name() is per-entry and independent of whether root was a symlink.

PARALLEL WORK (no conflict):
  - P1.M1.T1.S1 (Bug 1: classifyDir multi-entry) edited discover.go, a DIFFERENT
    file. No overlap.
  - P1.M3.T1.S1 (Bug 2: JSDoc /**/) edits jsdoc.go, a DIFFERENT file. No overlap.
  - P1.M3.T2.S1 (Bug 5: check all-missing ERROR) edits check.go, a DIFFERENT
    file. No overlap.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave

# gofmt check (Go repo — NOT ruff/mypy).
gofmt -l internal/discover/index.go internal/discover/index_test.go
# EXPECT: no output. If listed, run: gofmt -w on the file(s).

go vet ./internal/discover/
# EXPECT: clean (no output, exit 0).

# Confirm EvalSymlinks is now called in Index:
grep -c "filepath.EvalSymlinks" internal/discover/index.go
# EXPECT: >= 1.

# Confirm NO duplicate path/filepath import (it was already there):
grep -c '"path/filepath"' internal/discover/index.go
# EXPECT: exactly 1.

# Confirm the fall-through guard form (rerr == nil, NOT returning rerr):
grep -A1 'filepath.EvalSymlinks(root)' internal/discover/index.go | grep -q 'rerr == nil' \
  && echo "fall-through form (correct)" || echo "FAIL: must use rerr==nil, not propagate"
# EXPECT: "fall-through form (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# The new regression test in isolation (PASS on Unix/macOS; SKIP on no-symlink platforms):
go test ./internal/discover/ -run 'TestIndexSymlinkedRoot' -v -count=1
# EXPECT: --- PASS: TestIndexSymlinkedRoot (or --- SKIP on restricted platforms; never FAIL).

# All Index tests (new + existing — confirms EvalSymlinks is a no-op for real paths):
go test ./internal/discover/ -run 'TestIndex' -v -count=1
# EXPECT: all PASS. Watch especially TestIndexMissingRoot (must still return err for a
# missing root — EvalSymlinks fails, falls through, Stat fails, error propagated) and
# TestIndexWorkedExample (the PRD §7.1 tree must still produce 5 entries).

# The whole discover package:
go test ./internal/discover/ -count=1
# EXPECT: ok github.com/dabstractor/weave/internal/discover.

# CRITICAL: findEnv's contract is UNCHANGED (the fix is in Index, not findEnv):
go test ./internal/extdir/ -run 'TestFindEnvDoesNotResolveSymlinks' -v -count=1
# EXPECT: PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole module build (resolve/search/check/ui/main all depend on discover.Index):
go build ./...
# EXPECT: exit 0.

# End-to-end repro (the bug PRD's Issue 4):
go build -o /tmp/weave .
rm -rf /tmp/s1 /tmp/s1link && mkdir -p /tmp/s1
printf '/** ext */\nexport default function(){}\n' > /tmp/s1/foo.ts
ln -sfn /tmp/s1 /tmp/s1link

weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --list   # EXPECT: foo (rc=0) — was "no extensions found" (rc=1)
weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave foo      # EXPECT: the resolved real path (rc=0) — was "unknown tag" (rc=1)
weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --path   # EXPECT: /tmp/s1link (rc=0) — UNCHANGED, shows the symlink path
# EXPECT: --list shows foo; <tag> resolves; --path still shows the symlink. All three agree the store is usable.
rm -f /tmp/weave; rm -rf /tmp/s1 /tmp/s1link

# Whole repo test suite (catches any cross-package regression):
go test ./... -count=1
# EXPECT: all packages ok.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None beyond Level 3. No MCP/Docker/DB/web/perf. The domain-specific validation
# IS TestIndexSymlinkedRoot, which proves the fix through the real WalkDir path.
# The key creative check is the macOS /tmp → /private/tmp scenario (reproduced by
# any symlink-to-real-dir in Level 3). The Path-prefix assertion in the test
# (got[0].Path under realDir) confirms pi -e would load the REAL file, not a
# symlink path — the correctness contract for downstream consumption.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l` lists no files; `go vet ./internal/discover/` clean;
      grep confirms `filepath.EvalSymlinks` present (>= 1), no duplicate
      `path/filepath` import (exactly 1), and the `rerr == nil` fall-through form.
- [ ] Level 2: `TestIndexSymlinkedRoot` PASS (or SKIP on no-symlink platforms).
- [ ] Level 2: all existing `TestIndex*` tests PASS unchanged (EvalSymlinks is a
      no-op for real paths).
- [ ] Level 2: `TestFindEnvDoesNotResolveSymlinks` STILL PASSES (findEnv untouched).
- [ ] Level 2: `go test ./internal/discover/ -count=1` → ok.
- [ ] Level 3: `go build ./...` exits 0.
- [ ] (Optional) Level 3 manual repro: `weave --list` on a symlinked dir shows the
      extension (was "no extensions found").
- [ ] (Optional) Level 4: `go test ./... -count=1` → all packages ok.

### Feature Validation

- [ ] A symlinked extensions dir with `foo.ts` is discovered as exactly 1 entry,
      `RelTag == "foo"` (NOT an empty catalog).
- [ ] A non-symlinked dir behaves identically before/after (all existing tests pass).
- [ ] `Extension.Path`/`EntryFile` for a symlinked root point to the RESOLVED real
      target (under realDir), correct for `pi -e`.
- [ ] `--path` still shows the original symlink path (findEnv untouched).
- [ ] A broken symlink chain falls through to `os.Stat` (TestIndexMissingRoot still
      passes — err != nil for a missing root).
- [ ] The EvalSymlinks block is AFTER `filepath.Abs` and BEFORE `os.Stat`.

### Code Quality Validation

- [ ] Follows existing conventions: mirrors the TestIndex* idiom + the
      extdir os.Symlink pattern; reuses writeFile/relTags unchanged.
- [ ] Test style matches neighbors: t.TempDir(), t.Fatal for the count guard,
      t.Errorf for field assertions, t.Skipf for symlink-unsupported platforms.
- [ ] Mode A doc-comment update names the walk-root symlink resolution and the
      root-vs-nested distinction.
- [ ] Anti-patterns avoided: no propagated EvalSymlinks error; no findEnv edit;
      no duplicate import; no special-casing for "is this a symlink?".
- [ ] Scope respected: only index.go (EvalSymlinks block + doc) and index_test.go
      (+1 test) change.

### Documentation & Deployment

- [ ] Mode A doc ride-along: Index doc comment names the walk-root EvalSymlinks
      resolution and that nested symlinked directories are still NOT followed.
- [ ] No new env vars, no config change, no README change (that is P1.M4.T1.S1,
      the final changeset-level docs sweep — Issue 4 is an internal fix with no
      user-facing doc surface beyond Mode A).
- [ ] Code is self-documenting: the EvalSymlinks block comment explains the
      WalkDir-Lstats-root root cause and the fix-in-Index (not findEnv) decision.

---

## Anti-Patterns to Avoid

- ❌ Don't fix this in `extdir.findEnv`. It DELIBERATELY preserves the symlink
  verbatim (doc comment extdir.go:11-14), and `TestFindEnvDoesNotResolveSymlinks`
  (extdir_test.go:201) PINS that. The fix MUST be in `discover.Index` so that test
  stays valid and all Find() callers are covered uniformly.
- ❌ Don't propagate the `EvalSymlinks` error (`return nil, rerr`). On a broken
  symlink chain, that masks the familiar `os.Stat` "no such file" error with
  EvalSymlinks' wording. Use `if resolved, rerr := ...; rerr == nil { root =
  resolved }` so a broken chain falls through to Stat, which fails with the
  expected message.
- ❌ Don't add a `path/filepath` import. It is ALREADY imported at index.go line 6.
  Adding it again is a duplicate-import compile error.
- ❌ Don't reorder the preamble. EvalSymlinks MUST run AFTER `filepath.Abs` (root
  is absolute first) and BEFORE `os.Stat` (Stat validates the resolved path) and
  BEFORE `WalkDir` (WalkDir walks the resolved real dir).
- ❌ Don't special-case "is root a symlink?" before calling EvalSymlinks. Just
  call it unconditionally (guarded by `rerr == nil`). It's a no-op for real paths,
  and special-casing adds branching for zero benefit.
- ❌ Don't resolve symlinks for NESTED directories inside the tree. WalkDir's
  stdlib default (don't follow symlinked dirs) avoids cycles and is preserved.
  The fix resolves ONLY the walk root. The doc comment must keep this distinction.
- ❌ Don't forget the `t.Skipf` guard in `TestIndexSymlinkedRoot`. `os.Symlink`
  fails on restricted filesystems / some CI sandboxes. Without `t.Skipf`, the test
  FAILS (not skips) on those platforms. Both existing os.Symlink tests use it.
- ❌ Don't swap the `os.Symlink(target, link)` args. The FIRST arg is the target
  (the real dir), the SECOND is the link being created. Swapping creates a
  symlink to nothing.
- ❌ Don't touch the root-skip guard (P1.M2.T1.S1, lines 81-85) or the
  skipDirs/hidden guards (P1.M2.T2.S1, lines 94/110). This fix edits the `root`
  variable ABOVE the callback; those operate on `d.Name()` per-entry and are
  independent. Insert ONLY the EvalSymlinks block in the preamble.
- ❌ Don't edit discover.go / classifyDir / classifyFile / extdir.go. The fix is
  entirely in discover/index.go (preamble + doc) and index_test.go (+1 test).
- ❌ Don't add a new test package or redeclare writeFile/relTags/strEq. Tests are
  white-box (`package discover`) and reuse existing helpers. index_test.go already
  imports os, path/filepath, strings, testing — no import change needed.
- ❌ Don't commit anything or edit tasks.json / PRD.md — those are orchestrator-owned.
