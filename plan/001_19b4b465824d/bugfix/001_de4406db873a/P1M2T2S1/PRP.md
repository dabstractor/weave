# PRP — P1.M2.T2.S1: Add skipDirs set and hidden-entry skip to WalkDir callback

(Bugfix 001_de4406db873a — Issue 6, Minor. Discovery subsystem, `internal/discover/index.go`.)

## Goal

**Feature Goal**: Fix `discover.Index` (in `internal/discover/index.go`) so the
WalkDir callback skips well-known non-extension directories (`node_modules`,
`.git`) and hidden entries (any file or directory whose base name starts with `.`)
during the discovery walk. Today these pollute the catalog: a top-level
`node_modules/` (from `npm install` at the store root) yields a spurious entry per
nested package; a `.secret.ts` becomes an extension tagged `.secret`; a `.git/`
dir contributes `.ts`-like artifacts. The fix adds a package-level `skipDirs` set
and two `strings.HasPrefix(name, ".")` guards to the existing WalkDir callback,
plus the `"strings"` import the file is currently missing.

**Deliverable**: ONE localized code edit (imports + skipDirs var + two guards in
the callback + a doc-comment touch-up) + THREE regression tests:
- `internal/discover/index.go`:
  - add `"strings"` to the import block;
  - add `var skipDirs = map[string]bool{"node_modules": true, ".git": true}` at
    package level;
  - in the `if d.IsDir()` branch, AFTER the root-skip guard (P1.M2.T1.S1, already
    landed) and BEFORE `classifyDir`, add `name := d.Name(); if skipDirs[name] ||
    strings.HasPrefix(name, ".") { return filepath.SkipDir }`;
  - in the file branch, BEFORE `classifyFile`, add `if strings.HasPrefix(d.Name(),
    ".") { return nil }`;
  - update the `Index` doc comment to name `node_modules`, `.git`, and hidden
    entries as skipped (Mode A).
- `internal/discover/index_test.go`: add `TestIndexSkipsNodeModules`,
  `TestIndexSkipsHiddenFile`, `TestIndexSkipsGitDir`.

No other files change. No new helpers, no signature changes beyond the import.

**Success Definition**:
- A store with `myext.ts` + `node_modules/somepkg/index.js` + `node_modules/
  somepkg/package.json` is discovered as exactly ONE entry (`myext`); the
  `node_modules/` subtree is pruned entirely (SkipDir), so no `node_modules/*`
  tag appears.
- A store with `myext.ts` + `.secret.ts` is discovered as exactly ONE entry
  (`myext`); `.secret.ts` is skipped (`return nil`), and `myext.ts` (a sibling)
  is STILL discovered (the hidden-file skip does NOT prune siblings).
- A store with `myext.ts` + `.git/hooks/some.ts` is discovered as exactly ONE
  entry (`myext`); `.git/` is pruned (SkipDir).
- The pre-existing `TestIndexWorkedExample` (PRD §7.1 acceptance tree) and every
  other `TestIndex*` test still pass — the skip logic only changes behavior for
  the previously-uncovered `node_modules`/`.git`/hidden cases.
- `Index` still descends into plain category directories and discovers non-hidden
  extensions nested under them (the skip does not over-prune).
- `go test ./internal/discover/ -count=1` passes; `go vet ./internal/discover/`
  is clean; `go build ./...` exits 0.

## User Persona (if applicable)

**Target User**: weave CLI users who run `npm install` at their extensions store
root (to share dependencies across package extensions), who version their store
with git (creating `.git/`), or who have editor/OS dotfiles (`.DS_Store`, config
scratch files) in the tree. Also: anyone whose store lives in a git repo.

**Use Case**: `weave --list`, `weave <tag>`, `weave check` on a store that
contains `node_modules/`, `.git/`, or hidden files. Today `--list` shows spurious
`node_modules/*` and `.secret` entries; after the fix, only real user-authored
extensions appear.

**Pain Points Addressed**: Catalog pollution. A single `npm install` at the root
can add HUNDREDS of dependency packages to `weave --list`, burying the real
extensions. Hidden files like `.secret.ts` leak secrets into the catalog (and
into `--search`). The fix mirrors how pi, npm, and git itself treat these
directories and dotfiles as non-content.

## Why

- **Practical hygiene (the bug PRD's "footgun")**: PRD §7.1's classify-then-descend
  rule descends into every plain directory and accepts every `*.ts`/`*.js` file.
  This is correct for user-authored category folders, but it naively trusts
  `node_modules/` and `.git/` as "category folders" and `.secret.ts` as an
  "extension." PRD §17 only forbids placing the STORE itself in a pi
  auto-discovery location; it does not address these nested cases. The fix is the
  product-judgment hygiene layer the bug PRD (Issue 6) explicitly recommends.
- **Mirrors ecosystem conventions**: `node_modules` is the npm convention for
  dependencies (never user content). `.git` is the git internals directory.
  Dotfiles (leading `.`) are the Unix convention for "hidden" / config / scratch
  files. Every major tool (git itself, npm, ripgrep, eslint) skips these by
  default. weave matching this convention means a store that "looks normal" to a
  developer is discovered cleanly without surprise entries.
- **Composes with the sibling index.go fixes**: Per `bug_cascade_map.md`, Bug 3
  (root-skip, P1.M2.T1.S1 — ALREADY LANDED), Bug 6 (symlinked root, P1.M2.T3.S1),
  and Issue-6 (this fix) all live in the SAME WalkDir callback in `index.go`.
  This fix inserts its checks AFTER the root-skip guard (line 70) and BEFORE
  `classifyDir` (line 71), and the file-branch check BEFORE `classifyFile` (line
  80). The placement is chosen so the three fixes coexist without conflict.
- **No downstream contract change**: `Index` still returns `[]Extension` sorted by
  `RelTag`; it simply returns FEWER entries (no `node_modules/*`, no `.*`). Every
  consumer (main.go, resolve, search, check, ui) already assumes "one Extension
  per real entry" — the fix removes spurious entries, it does not change the type
  or ordering.

## What

### The change to `internal/discover/index.go`

**1. Imports** (lines 3-9): add `"strings"` to the import block, keeping it
alphabetical:

```go
import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)
```

**2. Package-level `skipDirs`** (add immediately above `func Index`, after the
doc comment):

```go
// skipDirs are well-known directories that are never extension containers or
// category folders: node_modules holds npm dependencies (created by `npm
// install` at the store root to share deps across package extensions), and .git
// holds git internals. Their contents are never user-authored extensions, so
// the WalkDir callback prunes them via filepath.SkipDir. PRD Issue 6.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
}
```

**3. Dir branch** (inside `if d.IsDir()`, AFTER the root-skip guard at line 70,
BEFORE `classifyDir` at line 71):

```go
if d.IsDir() {
	// [root-skip guard — P1.M2.T1.S1, already present, unchanged]
	if path == root {
		return nil // root is the store container, never an extension — always descend
	}
	// Skip well-known non-extension directories (node_modules, .git) and hidden
	// directories (base name starts with '.'). These never contain user-authored
	// extensions; descending would pollute the catalog (PRD Issue 6). SkipDir
	// prunes the entire subtree.
	name := d.Name()
	if skipDirs[name] || strings.HasPrefix(name, ".") {
		return filepath.SkipDir // prune node_modules, .git, hidden dirs
	}
	ext, isExt, descend := classifyDir(root, path)   // ← existing, unchanged
	// ... rest unchanged
}
```

**4. File branch** (BEFORE `classifyFile` at line 80):

```go
// File entry:
if strings.HasPrefix(d.Name(), ".") {
	return nil // skip hidden files (.secret.ts, .DS_Store-as-.ts, etc.); NOT SkipDir
}
ext, ok := classifyFile(root, path)
// ... rest unchanged
```

**5. Doc comment** (Mode A, lines 11-50): update to name `node_modules`, `.git`,
and hidden entries. The existing comment already mentions "avoiding descent into
node_modules/" (line 28) aspirationally; make it precise and add hidden entries.

### Success Criteria

- [ ] `"strings"` is in the import block of `index.go` (alphabetical position).
- [ ] `var skipDirs = map[string]bool{"node_modules": true, ".git": true}` exists
      at package level (above `func Index`).
- [ ] In `if d.IsDir()`, the skipDirs+hidden check is AFTER the `if path == root`
      guard and BEFORE `classifyDir`.
- [ ] The dir-skip returns `filepath.SkipDir` (prunes subtree).
- [ ] In the file branch, the hidden-file check is BEFORE `classifyFile`.
- [ ] The file-skip returns `nil` (NOT SkipDir — SkipDir on a file prunes
      siblings, which would hide real extensions).
- [ ] `TestIndexSkipsNodeModules`, `TestIndexSkipsHiddenFile`, `TestIndexSkipsGitDir`
      all pass.
- [ ] All pre-existing `TestIndex*` / `TestClassifyDir*` / `TestWalkClassified*` /
      `TestWorkedExample*` tests still pass (no regression — the skip only changes
      the uncovered node_modules/.git/hidden cases).
- [ ] The `Index` doc comment names `node_modules`, `.git`, and hidden entries.
- [ ] `go test ./internal/discover/ -count=1` → ok; `go vet ./internal/discover/`
      clean; `go build ./...` exits 0.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP quotes the exact current `index.go`
(with verified line numbers), the exact insertion points (after the root guard at
line 70, before classifyDir at line 71, before classifyFile at line 80), the exact
code to add (imports, skipDirs var, two guards), the critical SkipDir-vs-nil
distinction (verified in research/skipdir_hidden_semantics.md with stdlib doc
citations), the exact three regression tests (reusing existing `writeFile` +
`relTags` helpers), and the Mode A doc-comment update. The fix is additive — it
inserts checks into an existing callback without restructuring it.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: §7.1 (classify-then-descend) descends into every plain dir and accepts every
       *.ts/*.js; §17 only forbids placing the STORE in a pi auto-discovery location.
       Issue 6 (the bug PRD) explicitly flags node_modules/.git/hidden as a footgun
       and recommends skipping them (product judgment, not a strict spec requirement).
  critical: This is a hygiene fix the PRD invited but did not mandate. The fix MUST
            NOT over-prune — plain category dirs and non-hidden nested extensions
            (e.g. platform/linux/, writing/reddit-poster.ts) MUST still be discovered.

- file: /home/dustin/projects/weave/internal/discover/index.go
  why: CONTAINS THE CODE TO EDIT. The WalkDir callback (lines 57-85) is where both
       guards go. The root-skip guard (P1.M2.T1.S1) is ALREADY present at lines 62-70.
       The import block (lines 3-9) is missing "strings".
  pattern: |
    # Current (BEFORE this fix) — verified at index.go lines 57-85:
    walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() {
            # [lines 62-70: root-skip guard — UNCHANGED]
            if path == root { return nil }
            ext, isExt, descend := classifyDir(root, path)   # line 71
            if isExt { result = append(result, *ext) }
            if !descend { return filepath.SkipDir }
            return nil
        }
        ext, ok := classifyFile(root, path)                  # line 80
        if ok { result = append(result, *ext) }
        return nil
    })
    # AFTER this fix: skipDirs check inserted between line 70 and 71;
    # hidden-file check inserted before line 80.
  gotcha: |
    1. The dir-skip returns filepath.SkipDir (prune subtree). The file-skip returns
       nil (skip ONE file, keep walking siblings). Swapping these is the #1 way to
       get this wrong: SkipDir on a file prunes siblings (and `.` sorts first
       lexically, so .secret.ts would prune myext.ts).
    2. The skipDirs check goes AFTER the root guard (line 70), not before. The root
       is never named node_modules/.git, so order is functionally irrelevant, but
       the item contract specifies "AFTER the root-skip guard and BEFORE classifyDir."
    3. "strings" MUST be added to imports — index.go currently imports errors, io/fs,
       os, path/filepath, sort (verified). Without it, `strings.HasPrefix` fails to
       compile.

- file: /home/dustin/projects/weave/internal/discover/index_test.go
  why: WHERE THE THREE REGRESSION TESTS GO. White-box (`package discover`), reuses
       writeFile (jsdoc_test.go) and relTags (discover_test.go) WITHOUT redeclaring.
       Match the TestIndex* naming + idiom.
  pattern: |
    # Idiom (from TestIndexIgnoresStrayFiles / TestIndexSortedByRelTag):
    func TestIndexX(t *testing.T) {
        root := t.TempDir()
        writeFile(t, root, "myext.ts", "/** my ext. */\n")
        writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "index.js", "...")
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        tags := relTags(got)
        # assert len / tags / absence of node_modules/*
    }
  critical: Place the three tests AFTER TestIndexIgnoresStrayFiles (the last existing
            TestIndex* test) so walk edge cases stay grouped. Use relTags(got) for the
            absence-of-spurious-entry assertions. Do NOT redeclare writeFile/relTags.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md
  why: §Bug 6 gives the EXACT fix (skipDirs map, shouldSkipDir helper, the two
       insertion points) and the three regression tests. This PRP inlines the check
       rather than extracting shouldSkipDir (simpler, matches the item contract
       verbatim); both are equivalent.
  critical: fix_design confirms "strings" is NOT currently imported and MUST be added.
            It also notes symlinked .ts files are a SEPARATE concern NOT addressed here.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/bug_cascade_map.md
  why: Bug Issue6 entry confirms the location (index.go WalkDir callback) and the
       dependency graph: Bug 4 (root, M2.T1 — LANDED), Issue6 (this, M2.T2),
       Bug 6/symlink (M2.T3) are all in the same callback, landed sequentially.
  critical: This fix (M2.T2) lands AFTER M2.T1 (root guard, already present) and
            BEFORE M2.T3 (symlink EvalSymlinks, which edits the `root` variable
            ABOVE the callback). The two compose: d.Name() is unaffected by whether
            root was a symlink.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/test_patterns.md
  why: Lists exact helpers: writeFile(t, dir, name, content), relTags(exts). Confirms
       white-box package discover, t.TempDir(), t.Fatal/t.Errorf idiom, no t.Parallel().
  critical: "node_modules/hidden skip" is listed under "Existing related tests (gaps
            to fill)" — confirms there is NO existing coverage (the three new tests
            fill the gap).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T1S1/PRP.md  (parallel, LANDED)
  why: The root-skip guard PRP. It is ALREADY IMPLEMENTED (verified: index.go lines
       62-70). This subtask inserts AFTER its guard. The P1M2T1S1 PRP's "SAME-FILE
       SIBLING FIXES" section explicitly anticipated THIS task: "P1.M2.T2.S1 (Issue 6)
       edits the SAME `if d.IsDir()` branch, inserting a shouldSkipDir check AFTER
       this root guard."
  critical: Do NOT re-add the root guard (it's there). Do NOT modify the root guard.
            Insert the skipDirs check between the root guard's closing `}` (line 70)
            and the `classifyDir` call (line 71).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T2S1/research/skipdir_hidden_semantics.md
  why: The EMPIRICAL verification of SkipDir-vs-nil semantics, lexical ordering,
       d.Name()/d.IsDir() behavior, and that `.`/`..` are NOT visited by WalkDir.
  critical: The load-bearing finding: SkipDir on a DIR prunes the subtree; SkipDir on
            a FILE prunes siblings. `.` sorts before letters, so a hidden file visited
            first would prune real siblings if the file-skip used SkipDir. Hence
            file-skip = nil, dir-skip = SkipDir.

- url: https://pkg.go.dev/io/fs#WalkDirFunc
  why: Authoritative SkipDir semantics. "If the function returns SkipDir when invoked
       on a directory, WalkDir skips the directory's contents entirely." vs "If the
       function returns SkipDir when invoked on a file, WalkDir skips the remaining
       files in the current directory."
  critical: This is why dir-skip and file-skip use DIFFERENT return values.

- url: https://pkg.go.dev/io/fs#DirEntry
  why: d.Name() returns the BASE name (final path element), and d.IsDir() reliably
       distinguishes dirs. So strings.HasPrefix(d.Name(), ".") catches hidden entries
       by base name; skipDirs[d.Name()] checks the exact base name.
```

### Current Codebase tree (the files this subtask touches)

```bash
internal/discover/
├── discover.go          # (parallel P1.M1.T1.S1 edits this; NOT this subtask) — classifyDir/classifyFile
├── discover_test.go     # (NOT this subtask) — walkClassified, relTags helper
├── extension.go         # (read-only: BuildExtension, Extension, parsePackageJSON)
├── extension_test.go    # (read-only: strEq helper)
├── index.go             # ← EDIT: imports + skipDirs var + 2 guards + doc comment
├── index_test.go        # ← EDIT: + TestIndexSkipsNodeModules, TestIndexSkipsHiddenFile, TestIndexSkipsGitDir
├── jsdoc.go             # (read-only: ExtractJSDoc)
└── jsdoc_test.go        # (read-only: writeFile, writeFileBytes helpers)
```

### Desired Codebase tree (no NEW files; edits only)

```bash
# No files added/deleted. Two existing files edited:
internal/discover/index.go        # + "strings" import; + skipDirs var; + dir-branch guard; + file-branch guard; + doc comment
internal/discover/index_test.go   # + 3 regression tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (dir-skip = SkipDir, file-skip = nil): the two branches MUST return
// different values. SkipDir on a DIRECTORY prunes its entire subtree (correct for
// node_modules/.git/hidden-dirs — we want to skip everything inside). SkipDir on a
// FILE skips the REMAINING siblings in the same directory (WRONG for .secret.ts —
// it would also skip myext.ts, which sorts after '.'). So hidden files return nil
// (skip just that file), hidden/skipDirs directories return SkipDir (prune subtree).
// Verified: https://pkg.go.dev/io/fs#WalkDirFunc.

// CRITICAL (lexical order: '.' sorts first): WalkDir visits entries in lexical
// filename order. '.' (0x2E) < letters/digits, so .secret.ts is visited BEFORE
// myext.ts. If the file-skip returned SkipDir, .secret.ts would prune myext.ts.
// This is WHY file-skip must be nil. Do not "optimize" by returning SkipDir in the
// file branch.

// CRITICAL (add "strings" to imports): index.go currently imports errors, io/fs,
// os, path/filepath, sort (verified lines 3-9). It does NOT import strings. The
// fix uses strings.HasPrefix, so "strings" MUST be added. Keep the import block
// alphabetical (strings comes after sort). An unused import fails `go build`; a
// missing import fails `go build` with "undefined: strings".

// CRITICAL (insert AFTER the root guard, not before): the root-skip guard
// (if path == root { return nil }) is at lines 68-70 (P1.M2.T1.S1, already landed).
// The skipDirs check goes AFTER it (between line 70 and the classifyDir call at
// line 71). The item contract specifies this order. Functionally the root is never
// named node_modules/.git, so order doesn't matter — but the contract is explicit,
// and placing it after keeps the "is this the root?" question first.

// GOTCHA (d.Name() is the BASE name, not the full path): fs.DirEntry.Name() returns
// the final path element. For /store/node_modules, d.Name() == "node_modules". For
// /store/.secret.ts, d.Name() == ".secret.ts" (INCLUDING the leading dot). So
// strings.HasPrefix(d.Name(), ".") and skipDirs[d.Name()] are correct. Do NOT use
// filepath.Base(path) — d.Name() is already the base name (and cheaper).

// GOTCHA (WalkDir does NOT visit '.' or '..'): os.ReadDir excludes the self/parent
// entries, so d.Name() is never literally "." or ".." (which would otherwise match
// HasPrefix(name, ".")). The hidden check only catches user-named dotfiles/dotdirs.
// Verified: https://pkg.go.dev/os#ReadDir.

// GOTCHA (hidden check uses HasPrefix, not a suffix/middle check): a file like
// "my.ext.ts" (dot in the middle) is NOT hidden — it's a normal extension file
// (tag "my.ext"). Only a LEADING '.' makes it hidden. strings.HasPrefix(d.Name(),
// ".") is the correct check. Do not use strings.Contains.

// GOTCHA (do NOT extract a shouldSkipDir helper unless you want to): fix_design.md
// §Bug 6 suggests a shouldSkipDir(name) bool helper. The item contract INLINES the
// check (if skipDirs[name] || strings.HasPrefix(name, ".")). Either is fine; the
// contract's inline form is simpler and is what the PRP specifies. If you do
// extract a helper, name it shouldSkipDir and put it next to skipDirs, but the
// inline form is preferred for a 1-line condition.

// GOTCHA (node_modules can appear at ANY depth, not just root): a category folder
// might itself contain a package that ran npm install (writing/subpkg/node_modules).
// The skipDirs check runs on EVERY dir entry, so nested node_modules are pruned
// too. This is correct — node_modules is never user content regardless of depth.

// GOTCHA (symlinked .ts files are OUT OF SCOPE): the bug PRD (Issue 6) lists three
// footguns: node_modules, hidden files, symlinked .ts files. This subtask (per its
// item contract) fixes ONLY the first two. Symlinked files are visited by WalkDir
// (it does not follow symlinked DIRS, but it visits symlinked FILES as entries)
// and classifyFile classifies them. Do NOT add symlink handling here — it's a
// separate decision needing its own test, and the contract does not mention it.

// GOTCHA (white-box test, reuse helpers): index_test.go is package discover. It
// reuses writeFile (jsdoc_test.go) and relTags (discover_test.go) — do NOT
// redeclare them (Go redefinition error). The tests are white-box so they call the
// PUBLIC Index AND the unexported helpers.

// GOTCHA (no t.Parallel): the tests use t.TempDir() (per-test isolated, no env/cwd
// mutation). Match neighboring TestIndex* tests, which omit t.Parallel().
```

## Implementation Blueprint

### Data models and structure

One package-level variable added (`skipDirs`). No types, no fields, no helpers
(unless you choose to extract `shouldSkipDir` — optional, not recommended). The
change is two import additions (`"strings"`), one var, two guards, and a doc
touch-up.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/discover/index.go — add "strings" import
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: the import block (lines 3-9).
  EDIT: add `"strings"` after `"sort"`, keeping alphabetical order:
      import (
          "errors"
          "io/fs"
          "os"
          "path/filepath"
          "sort"
          "strings"
      )
  WHY: the fix uses strings.HasPrefix; index.go does not currently import strings.

Task 2: EDIT internal/discover/index.go — add skipDirs var
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: after the Index doc comment (ends ~line 50), immediately before
          `func Index(extensionsDir string) ([]Extension, error) {`.
  INSERT:
      // skipDirs are well-known directories that are never extension containers or
      // category folders: node_modules holds npm dependencies (created by `npm
      // install` at the store root to share deps across package extensions), and
      // .git holds git internals. Their contents are never user-authored extensions,
      // so the WalkDir callback prunes them via filepath.SkipDir (PRD Issue 6).
      var skipDirs = map[string]bool{
          "node_modules": true,
          ".git":         true,
      }
  NAMING: skipDirs (package-private; only Index's callback uses it).

Task 3: EDIT internal/discover/index.go — dir-branch skip guard
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: inside `if d.IsDir() {`, AFTER the root-skip guard's closing brace
          (line 70: `}` closing `if path == root`), BEFORE the `ext, isExt,
          descend := classifyDir(root, path)` line (line 71).
  INSERT (between line 70 and 71):
          // Skip well-known non-extension directories (node_modules, .git) and
          // hidden directories (base name starts with '.'). These never contain
          // user-authored extensions; descending would pollute the catalog (PRD
          // Issue 6). SkipDir prunes the entire subtree.
          name := d.Name()
          if skipDirs[name] || strings.HasPrefix(name, ".") {
              return filepath.SkipDir // prune node_modules, .git, hidden dirs
          }
  PRESERVE: the root-skip guard (lines 62-70) UNCHANGED; the classifyDir call and
            everything after it in the dir branch UNCHANGED; the file branch; the
            stat-guard; filepath.Abs; the sort; the return.
  CRITICAL: return filepath.SkipDir (prune subtree), NOT nil.

Task 4: EDIT internal/discover/index.go — file-branch hidden skip
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: the file branch (after the `if d.IsDir() { ... }` block closes at line
          79), BEFORE `ext, ok := classifyFile(root, path)` (line 80).
  INSERT (before line 80):
          // Skip hidden files (.secret.ts, .DS_Store-as-.ts, editor dotfiles).
          // return nil (NOT SkipDir): SkipDir on a file prunes the REMAINING
          // siblings, and '.' sorts first lexically, so SkipDir would hide real
          // extensions like myext.ts that sort after the hidden file.
          if strings.HasPrefix(d.Name(), ".") {
              return nil
          }
  PRESERVE: the classifyFile call and everything after it UNCHANGED.
  CRITICAL: return nil (skip one file), NOT SkipDir.

Task 5: EDIT internal/discover/index.go — update Index doc comment (Mode A)
  FILE: /home/dustin/projects/weave/internal/discover/index.go
  LOCATE: the Index doc comment (lines 11-50).
  EDIT: the comment currently says (line 27-29, approx):
      "avoiding descent into node_modules/."
    Update the classify-then-descend paragraph to name node_modules, .git, AND
    hidden entries explicitly. Replace the relevant sentence with:
      "Well-known non-extension directories (node_modules, .git) and hidden
      entries (any file or directory whose base name starts with '.') are
      skipped during the walk (PRD Issue 6): directories are pruned via
      filepath.SkipDir, hidden files are skipped individually (return nil, not
      SkipDir, so sibling extensions are still discovered)."
    Leave the rest of the doc comment (filepath.Abs, error policy, symlink note,
    nil-on-empty) UNCHANGED.
  WHY: Mode A doc ride-along (the item contract point 5, DOCS).

Task 6: EDIT internal/discover/index_test.go — add 3 regression tests
  FILE: /home/dustin/projects/weave/internal/discover/index_test.go
  PLACEMENT: directly AFTER TestIndexIgnoresStrayFiles (the last existing
             TestIndex* test), so walk edge-case tests stay grouped.
  ADD three tests (mirror the TestIndexIgnoresStrayFiles idiom; reuse writeFile +
    relTags):

    // TestIndexSkipsNodeModules reproduces Issue 6: a top-level node_modules/ (from
    // `npm install` at the store root) must NOT contribute its nested packages as
    // extensions. The WalkDir callback prunes node_modules via filepath.SkipDir.
    func TestIndexSkipsNodeModules(t *testing.T) {
        root := t.TempDir()
        writeFile(t, root, "myext.ts", "/** my ext. */\n")
        // A nested npm package — must NOT become an extension.
        writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "index.js",
            "/** dep. */\nexport default function(){}\n")
        writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "package.json",
            `{"name":"somepkg"}`)
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        tags := relTags(got)
        if len(got) != 1 || got[0].RelTag != "myext" {
            t.Fatalf("got=%v; want exactly one extension 'myext' (node_modules pruned)", tags)
        }
        // Guard against the spurious entry by name.
        for _, tag := range tags {
            if strings.HasPrefix(tag, "node_modules/") {
                t.Errorf("node_modules leaked into catalog: %q (must be pruned)", tag)
            }
        }
    }

    // TestIndexSkipsHiddenFile reproduces Issue 6: a hidden file (.secret.ts) must
    // NOT become an extension. The file-skip returns nil (NOT SkipDir), so the
    // sibling myext.ts is STILL discovered.
    func TestIndexSkipsHiddenFile(t *testing.T) {
        root := t.TempDir()
        writeFile(t, root, "myext.ts", "/** my ext. */\n")
        writeFile(t, root, ".secret.ts", "/** secret. */\nexport default function(){}\n")
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        tags := relTags(got)
        if len(got) != 1 || got[0].RelTag != "myext" {
            t.Fatalf("got=%v; want exactly one extension 'myext' (.secret.ts skipped, myext.ts kept)", tags)
        }
        for _, tag := range tags {
            if strings.HasPrefix(tag, ".") {
                t.Errorf("hidden file leaked into catalog: %q", tag)
            }
        }
    }

    // TestIndexSkipsGitDir reproduces Issue 6: a .git/ directory (git internals)
    // must NOT contribute its artifacts as extensions. Pruned via SkipDir.
    func TestIndexSkipsGitDir(t *testing.T) {
        root := t.TempDir()
        writeFile(t, root, "myext.ts", "/** my ext. */\n")
        // A .ts-like artifact inside .git/ — must NOT become an extension.
        writeFile(t, filepath.Join(root, ".git", "hooks"), "some.ts", "// git hook\n")
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        tags := relTags(got)
        if len(got) != 1 || got[0].RelTag != "myext" {
            t.Fatalf("got=%v; want exactly one extension 'myext' (.git pruned)", tags)
        }
        for _, tag := range tags {
            if strings.HasPrefix(tag, ".git/") || strings.HasPrefix(tag, ".git") {
                t.Errorf(".git leaked into catalog: %q", tag)
            }
        }
    }
  NOTE: these tests use strings.HasPrefix, so index_test.go must import "strings".
        Check the existing import block (os, path/filepath, testing) and add
        "strings" if missing. writeFile (jsdoc_test.go) and relTags (discover_test.go)
        already exist — do NOT redeclare them.
  ASSERTS: each test — err==nil, len(got)==1, got[0].RelTag=="myext", no spurious
           node_modules/.git/. prefix in tags.

Task 7: VALIDATE
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -run 'TestIndexSkips' -v -count=1
    EXPECT: 3 PASS.
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -run 'TestIndex|TestClassifyDir|TestWalkClassified|TestWorkedExample' -v -count=1
    EXPECT: all PASS (new + existing — confirms no regression).
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -count=1
    EXPECT: ok github.com/dabstractor/weave/internal/discover.
  - RUN: cd /home/dustin/projects/weave && go vet ./internal/discover/
    EXPECT: clean.
  - RUN: cd /home/dustin/projects/weave && go build ./...
    EXPECT: exit 0.
  - RUN: cd /home/dustin/projects/weave && gofmt -l internal/discover/index.go internal/discover/index_test.go
    EXPECT: no output.
  - OPTIONAL manual repro (the bug PRD's Issue 6, NOT a unit test):
        go build -o /tmp/weave .
        rm -rf /tmp/bug6 && mkdir -p /tmp/bug6/node_modules/somepkg
        printf '/** my ext */\nexport default function(){}\n' > /tmp/bug6/myext.ts
        printf '/** dep */\nexport default function(){}\n' > /tmp/bug6/node_modules/somepkg/index.js
        printf '{"name":"somepkg"}\n' > /tmp/bug6/node_modules/somepkg/package.json
        weave_EXTENSIONS_DIR=/tmp/bug6 /tmp/weave --list   # EXPECT: only myext (no node_modules/somepkg)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the skipDirs set + two guards. Directories are pruned (SkipDir);
// hidden files are skipped individually (nil) so siblings survive.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
}

// Inside the WalkDir callback:
if d.IsDir() {
	if path == root {
		return nil // [P1.M2.T1.S1 root guard — already present]
	}
	name := d.Name()
	if skipDirs[name] || strings.HasPrefix(name, ".") {
		return filepath.SkipDir // prune node_modules, .git, hidden dirs (subtree)
	}
	// ... classifyDir as before
}
// File branch:
if strings.HasPrefix(d.Name(), ".") {
	return nil // skip hidden file; NOT SkipDir (would prune siblings)
}
// ... classifyFile as before

// CRITICAL: dir-skip = SkipDir (prune subtree); file-skip = nil (skip one file).
// Swapping these is the #1 bug: SkipDir on a file prunes siblings, and '.'
// sorts first, so .secret.ts would hide myext.ts.

// CRITICAL: "strings" must be in the import block (it currently is not).

// PATTERN: the regression tests mirror TestIndexIgnoresStrayFiles (count +
// surviving tag) PLUS a prefix-guard so the spurious entry is caught by name.
writeFile(t, root, "myext.ts", "/** my ext. */\n")
writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "index.js", "...")
got, _ := Index(root)
// len(got)==1, got[0].RelTag=="myext", no "node_modules/*" tag
```

### Integration Points

```yaml
DATABASE:
  - none. Pure stdlib file code.

CONFIG:
  - none. No config, env vars, or settings touched.

ROUTES / API:
  - none. weave is a CLI; this is an internal discovery function.

DISCOVERY SUBSYSTEM (the integration surface):
  - Index is the public catalog builder consumed by main.go (all modes), resolve,
    search, check, and ui. The fix changes Index's OUTPUT only for the previously-
    polluted node_modules/.git/hidden cases (it returns FEWER entries — no
    spurious ones). Every consumer already assumes "one Extension per real entry,
    sorted by RelTag" — the fix removes pollution, it does not change the type or
    ordering. No consumer code changes.

SAME-FILE SIBLING FIXES (cross-task dependencies, NOT this task):
  - P1.M2.T1.S1 (Issue 3: root-skip guard) — ALREADY LANDED at index.go lines
    62-70. This fix inserts AFTER it. No conflict.
  - P1.M2.T3.S1 (Issue 4: symlinked root) — PLANNED, edits the `root` variable
    ABOVE the callback (filepath.EvalSymlinks). Does NOT affect d.Name() (which
    is per-entry, independent of whether root was a symlink). The two compose.

PARALLEL WORK (no conflict):
  - P1.M1.T1.S1 (Bug 1: classifyDir multi-entry) edits discover.go, a DIFFERENT
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

# Confirm "strings" is now imported (it was missing before this fix):
grep -A8 '^import (' internal/discover/index.go | grep -q '"strings"' \
  && echo "strings imported (correct)" || echo "FAIL: strings import missing"
# EXPECT: "strings imported (correct)".

# Confirm skipDirs var exists:
grep -q 'var skipDirs = map\[string\]bool' internal/discover/index.go \
  && echo "skipDirs present (correct)" || echo "FAIL: skipDirs missing"
# EXPECT: "skipDirs present (correct)".

# Confirm dir-skip returns SkipDir, file-skip returns nil (the critical distinction):
# (dir branch) the skipDirs check's return:
grep -A3 'skipDirs\[name\]' internal/discover/index.go | grep -q 'filepath.SkipDir' \
  && echo "dir-skip returns SkipDir (correct)" || echo "FAIL: dir-skip must return SkipDir"
# (file branch) the hidden-file check's return:
grep -B1 -A2 'HasPrefix(d.Name(), "\.")' internal/discover/index.go | grep -q 'return nil' \
  && echo "file-skip returns nil (correct)" || echo "FAIL: file-skip must return nil"
# EXPECT: both correct.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# The 3 new regression tests in isolation:
go test ./internal/discover/ -run 'TestIndexSkips' -v -count=1
# EXPECT: 3 PASS:
#   TestIndexSkipsNodeModules — only myext; node_modules/somepkg NOT present
#   TestIndexSkipsHiddenFile  — only myext; .secret NOT present; myext kept
#   TestIndexSkipsGitDir      — only myext; .git/* NOT present

# The full walk + classify suite (new + existing — catches regressions):
go test ./internal/discover/ -run 'TestIndex|TestClassifyDir|TestWalkClassified|TestWorkedExample' -v -count=1
# EXPECT: all PASS. Watch especially TestIndexWorkedExample (PRD §7.1 tree —
# platform/linux/ and writing/reddit-poster.ts MUST still be discovered, proving
# the skip does not over-prune plain category dirs).

# The whole discover package:
go test ./internal/discover/ -count=1
# EXPECT: ok github.com/dabstractor/weave/internal/discover.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole module build (resolve/search/check/ui/main all depend on discover.Index):
go build ./...
# EXPECT: exit 0.

# Optional end-to-end repro (the bug PRD's Issue 6):
go build -o /tmp/weave .
rm -rf /tmp/bug6 && mkdir -p /tmp/bug6/node_modules/somepkg
printf '/** my ext */\nexport default function(){}\n' > /tmp/bug6/myext.ts
printf '/** dep */\nexport default function(){}\n' > /tmp/bug6/node_modules/somepkg/index.js
printf '{"name":"somepkg"}\n' > /tmp/bug6/node_modules/somepkg/package.json

weave_EXTENSIONS_DIR=/tmp/bug6 /tmp/weave --list   # EXPECT: only myext (rc=0)
# EXPECT: "myext" is the only row; node_modules/somepkg does NOT appear.
rm -f /tmp/weave

# Whole repo test suite (catches any cross-package regression):
go test ./... -count=1
# EXPECT: all packages ok.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None beyond Level 3. No MCP/Docker/DB/web/perf. The domain-specific validation
# IS the three TestIndexSkips* tests, which prove the skip through the real
# WalkDir path. The key creative check is TestIndexSkipsHiddenFile: it proves
# the file-skip does NOT over-prune (myext.ts survives despite .secret.ts
# sorting before it) — the load-bearing nil-vs-SkipDir distinction.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l` lists no files; `go vet ./internal/discover/` clean;
      grep confirms `"strings"` import, `skipDirs` var, dir-skip=SkipDir, file-skip=nil.
- [ ] Level 2: `TestIndexSkipsNodeModules`, `TestIndexSkipsHiddenFile`,
      `TestIndexSkipsGitDir` all pass.
- [ ] Level 2: all existing `TestIndex*`/`TestClassifyDir*`/`TestWalkClassified*`/
      `TestWorkedExample*` pass (no regression; TestIndexWorkedExample proves no
      over-pruning of plain category dirs).
- [ ] Level 2: `go test ./internal/discover/ -count=1` → ok.
- [ ] Level 3: `go build ./...` exits 0.
- [ ] (Optional) Level 3 manual repro confirms `weave --list` shows only myext.
- [ ] (Optional) Level 4: `go test ./... -count=1` → all packages ok.

### Feature Validation

- [ ] A store with myext.ts + node_modules/somepkg/{index.js,package.json} yields
      exactly 1 entry (myext); node_modules/* pruned.
- [ ] A store with myext.ts + .secret.ts yields exactly 1 entry (myext); .secret
      skipped AND myext kept (file-skip does not prune siblings).
- [ ] A store with myext.ts + .git/hooks/some.ts yields exactly 1 entry (myext);
      .git pruned.
- [ ] The dir-skip returns `filepath.SkipDir`; the file-skip returns `nil`.
- [ ] The skipDirs check is AFTER the root-skip guard, BEFORE classifyDir.
- [ ] The hidden-file check is BEFORE classifyFile.
- [ ] Plain category dirs and nested non-hidden extensions still discovered
      (TestIndexWorkedExample passes).

### Code Quality Validation

- [ ] Follows existing conventions: mirrors the TestIndex* idiom; reuses writeFile/
      relTags unchanged; no new helpers (skipDirs is a package var, not a function).
- [ ] Test style matches neighbors: t.TempDir(), t.Fatal for count guard, t.Errorf
      for field/tag assertions, no t.Parallel().
- [ ] Mode A doc-comment update names node_modules, .git, and hidden entries.
- [ ] Anti-patterns avoided: no SkipDir in the file branch; no Clean/EvalSymlinks
      on d.Name(); no over-pruning; no symlinked-file handling (out of scope).
- [ ] Scope respected: only index.go (imports + var + 2 guards + doc) and
      index_test.go (+3 tests) change.

### Documentation & Deployment

- [ ] Mode A doc ride-along: Index doc comment names node_modules, .git, hidden
      entries, and the SkipDir-vs-nil distinction.
- [ ] No new env vars, no config change, no README change (that is P1.M4.T1.S1,
      the final changeset-level docs sweep).
- [ ] Code is self-documenting: the skipDirs var doc + inline comments explain
      the prune-vs-skip logic.

---

## Anti-Patterns to Avoid

- ❌ Don't return `filepath.SkipDir` in the FILE branch for hidden files. SkipDir on
  a file prunes the REMAINING siblings; because `.` sorts first lexically,
  `.secret.ts` would hide `myext.ts`. File-skip MUST return `nil`.
- ❌ Don't return `nil` in the DIR branch for node_modules/.git/hidden-dirs. `nil`
  means "descend" — WalkDir would walk INTO node_modules and discover every nested
  package. Dir-skip MUST return `filepath.SkipDir` to prune the subtree.
- ❌ Don't forget to add `"strings"` to the import block. index.go does not
  currently import it; `strings.HasPrefix` will not compile without it.
- ❌ Don't place the skipDirs check BEFORE the root-skip guard. The item contract
  specifies "AFTER the root-skip guard and BEFORE classifyDir." (Functionally the
  root is never node_modules/.git, but follow the contract.)
- ❌ Don't use `filepath.Base(path)` instead of `d.Name()`. `d.Name()` is already
  the base name and is cheaper (no string allocation). Both work, but `d.Name()`
  is the idiomatic WalkDir pattern.
- ❌ Don't use `strings.Contains(d.Name(), ".")` for the hidden check. Only a
  LEADING `.` makes an entry hidden; a dot in the middle (`my.ext.ts`) is a normal
  extension file. Use `strings.HasPrefix(d.Name(), ".")`.
- ❌ Don't handle symlinked `.ts` files. The bug PRD lists them as a third footgun,
  but this subtask's item contract covers ONLY node_modules/.git/hidden. Symlinked
  files are out of scope (separate decision, separate test).
- ❌ Don't over-prune: plain category dirs (platform/, writing/) and non-hidden
  nested extensions MUST still be discovered. TestIndexWorkedExample is the guard.
- ❌ Don't touch the root-skip guard (P1.M2.T1.S1, already landed at lines 62-70).
  Insert AFTER it; do not modify or move it.
- ❌ Don't edit discover.go / classifyDir / classifyFile. The skip is a WalkDir-
  callback concern, entirely in index.go. (P1.M1.T1.S1 edits discover.go in
  parallel — different file, no conflict.)
- ❌ Don't add a new test package or redeclare writeFile/relTags/strEq. Tests are
  white-box (`package discover`) and reuse existing helpers from sibling _test.go
  files. If index_test.go needs `"strings"` for the test assertions, add it to
  index_test.go's imports (it currently imports os, path/filepath, testing).
- ❌ Don't commit anything or edit tasks.json / PRD.md — those are orchestrator-owned.
