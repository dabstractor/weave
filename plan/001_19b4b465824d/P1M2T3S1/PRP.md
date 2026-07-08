# PRP — P1.M2.T3.S1: Index() function with WalkDir + SkipDir + stat-guard + sort

## Goal

**Feature Goal**: Create `internal/discover/index.go` implementing `func Index(extensionsDir string) ([]Extension, error)` — the public catalog builder that walks the extensions dir ONCE and returns the fully-populated, sorted `[]Extension` consumed by every downstream mode (`main.go` --list/--search/--all/tag/check/resolve). It adapts skilldozer's `Index()` walk (WalkDir + stat-guard + sort by RelTag) to weave's classify-then-descend rule: where skilldozer's callback matches the FILE `SKILL.md`, weave's callback dispatches to `classifyFile`/`classifyDir` (from P1.M2.T2.S1) and returns `filepath.SkipDir` when a directory is a recognized extension (the load-bearing recursion rule preventing double-counting).

**Deliverable**: ONE new file `internal/discover/index.go` (`package discover`) containing `func Index(extensionsDir string) ([]Extension, error)` and ONE new test file `internal/discover/index_test.go` (`package discover`). No other files change. go.mod gains NO `require` (stdlib only: `errors`, `io/fs`, `os`, `path/filepath`, `sort`).

The function body is the parallel `walkClassified` test helper (created by P1.M2.T2.S1 in `discover_test.go:24-46` — the proven WalkDir skeleton) PLUS exactly four additions: (1) `filepath.Abs` first, (2) stat-guard the root before WalkDir, (3) return `error`, (4) explicit nil-slice-on-empty contract.

ONE new test file `internal/discover/index_test.go` (`package discover`, white-box — reuses `writeFile` from `jsdoc_test.go` and `writePackageJSON` from `extension_test.go`) with:
- `TestIndexWorkedExample` — the PRD §7.1 worked example driven through the PUBLIC `Index()` (proving Index == walkClassified + guard; 5 entries, correct relTags/kinds, no double-counting of `git-checkpoint/utils.ts` or `summarizer/src/index.ts`).
- `TestIndexMissingRoot` — non-existent root → error (not `(nil,nil)` — the stat-guard's whole purpose).
- `TestIndexRootIsFile` — root is a regular file → error ("not a directory").
- `TestIndexEmptyDir` — existing empty dir → `(nil/len-0 slice, nil error)`.
- `TestIndexSortedByRelTag` — results sorted by RelTag ascending, regardless of walk-visit order.
- `TestIndexRelativeInputStillAbsolute` — relative `extensionsDir` → every `Extension.Path`/`EntryFile` absolute (PRD §6.1/§13 contract).
- `TestIndexIgnoresStrayFiles` — `README.md`, `.bak`, non-entry subdirs ignored.
- `TestIndexNilOnEmpty` — pin the nil-vs-empty contract: `Index(emptyDir)` returns a slice with len 0.

No other files change. go.mod gains NO `require` (stdlib only). The package compiles and `go test ./internal/discover/...` passes (S1's + S2's + T2's + this subtask's tests). This subtask is CONSUMED by `main.go` (M2.T5 --list, M3.T2 resolve, M4.T3 search/check) which call `discover.Index(dir)`.

**Success Definition**:
- `go build ./...` exits 0 (discover package now has extension.go + jsdoc.go + discover.go + index.go).
- `go vet ./...` exits 0 (clean).
- `go test ./internal/discover/... -v` passes ALL tests (S1's + S2's + T2's + this subtask's).
- `go test -race ./...` passes.
- The PRD §7.1 worked example produces EXACTLY 5 entries with the correct relTags and kinds (`gate`/file, `git-checkpoint`/dir, `platform/linux`/dir, `summarizer`/package, `writing/reddit-poster`/file) when driven through the public `Index()`.
- `Index(nonexistentDir)` returns a non-nil error (the stat-guard prevents the `(nil, nil)` bug).
- `Index(regularFilePath)` returns a non-nil error.
- `Index(emptyDir)` returns a zero-length slice and nil error.
- Every `Extension.Path` and `Extension.EntryFile` is absolute even when `extensionsDir` is relative.
- `index.go` imports ONLY stdlib (`errors`, `io/fs`, `os`, `path/filepath`, `sort`); NO `encoding/json`; NO internal-package imports; NO call to `parsePackageJSON`/`BuildExtension`/`ExtractJSDoc` directly (those live inside `classifyFile`/`classifyDir` — Index delegates to the classifiers, it does not re-parse).
- go.mod still has no `require` block; no `go.sum`.

## User Persona (if applicable)

**Target User**: weave developers (the public `Index()` is the catalog builder) and, transitively, CLI users. This subtask is an internal package function with no user-facing surface yet (no main.go wiring — that is M2.T5).

**Use Case**: `Index(dir)` is called once per invocation by `main.go`'s `run()` (future milestones) after `extdir.Find()` resolves the extensions dir. It returns the `[]Extension` that `ui.PrintList` renders (--list), `resolve.Resolve` searches (tag/`-f`/`--all`), `search.Search` filters (--search), and `check.Check` validates (check). Because PRD §2.1 forbids a catalog index file, this rebuilds the catalog from disk on every call — fast (a directory walk of a small tree plus per-entry single-file reads, all already done by the classify functions).

**User Journey**: N/A (internal). The end-user journey (`weave <tag>`, `weave --list`) is assembled across M2-M5; this subtask is the catalog gear that feeds them.

**Pain Points Addressed**: Without the stat-guard, a missing root dir would be SWALLOWED — WalkDir feeds the root's lstat error to the callback, and the per-entry `if err != nil { return nil }` would hide it, yielding `(nil, nil)`. The caller (`main.go`) would then exit 0 with empty output instead of erroring to stderr and exiting 1 (violating PRD §6.4 error semantics critical for `$(...)` use). The stat-guard is the skilldozer pattern, explicitly called out as load-bearing in the item contract and architecture_mapping §3d.

## Why

- **Ties T2's classifiers into the public catalog builder**: T2 (P1.M2.T2.S1) owns the WHAT (is this an entry? what kind? what relTag?) via `classifyFile`/`classifyDir`; T3 owns the HOW (WalkDir, SkipDir, stat-guard, Abs, sort, error return). This split mirrors architecture_mapping §3c vs §3d exactly. T3's Index is a thin ~50-line dispatcher that drives the classifiers over a real walk.
- **Encodes the load-bearing recursion rule at the walk layer**: PRD §7.1 item 3 ("when a directory is recognized as a dir/package extension, do not descend into it") is enforced by T3's callback returning `filepath.SkipDir` when `classifyDir` returns `shouldDescend==false`. Without this, a naive recursive walk would emit `git-checkpoint/utils.ts` as a SEPARATE entry (double-counting) and descend into `summarizer/node_modules/` (catastrophic — potentially thousands of spurious entries). pi's own loader treats a dir extension as one unit (pi_extension_facts.md §4-5); weave must match.
- **Provides the absolute-output contract**: PRD §6.1 ("absolute path") and §13 (`case "$(./weave example)" in /*)`) require every emitted path to be absolute. `Index` calls `filepath.Abs(extensionsDir)` FIRST, so every classifier-derived `Path`/`EntryFile` is absolute regardless of whether the caller passed a relative dir. This protects the contract even when a future caller passes a cwd-relative path.
- **Surfaces root errors**: skilldozer's verified bug (research/walkdir_skipdir_semantics.md + skilldozer index.go comment) — without a stat-guard, a missing root returns `(nil, nil)`. The guard makes `Index(missing)` return a real error so `main.go` can exit 1 with a stderr message (PRD §6.4).

## What

A NEW `internal/discover/index.go` (`package discover`, NO package-level doc comment — S1's `extension.go` carries the package doc; a second is ignored by godoc but is poor form) containing ONE exported function:

### `func Index(extensionsDir string) ([]Extension, error)`
```go
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
//     ignore parsePackageJSON's err — lenient; check M4.T2 surfaces it).
//
// filepath.WalkDir does NOT follow symlinked directories (stdlib default); a
// symlink to an extension dir is therefore not discovered. PRD §7.1 does not
// require following symlinks, and the default avoids cycles.
//
// An empty extensions dir (no entries anywhere) yields a nil slice and a nil
// error; callers test with len() (e.g. --list exits 1 "if no extensions found").
func Index(extensionsDir string) ([]Extension, error)
```

Logic (EXACT — this is `walkClassified` from discover_test.go:24-46 plus the four additions):
1. `root, err := filepath.Abs(extensionsDir)` — make absolute first. On error (unreachable on Unix; possible on Windows for invalid paths), return `(nil, err)`.
2. **Stat-guard**: `info, err := os.Stat(root)`; if `err != nil` (missing/unreadable), return `(nil, err)`. If `!info.IsDir()`, return `(nil, errors.New(root + ": not a directory"))`.
3. `var result []Extension` — nil-init (empty store yields nil, matching the contract).
4. `walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error { ... })` with callback body:
   - `if err != nil { return nil }` — per-entry unreadable (e.g. chmod-000 subdir): skip, keep walking.
   - `if d.IsDir() { ... }`:
     - `ext, isExt, descend := classifyDir(root, path)`.
     - `if isExt { result = append(result, *ext) }`.
     - `if !descend { return filepath.SkipDir }` — prune subtree (load-bearing recursion rule).
     - `return nil` — plain category dir, descend naturally.
   - else (file): `ext, ok := classifyFile(root, path)`; `if ok { result = append(result, *ext) }`; `return nil` (NOT SkipDir — GOTCHA: SkipDir on a file skips siblings).
5. `if walkErr != nil { return result, walkErr }` (return partial + error; in practice WalkDir's only non-nil err is `filepath.SkipDir` from the callback, which is NOT an error to surface — but be defensive: SkipDir is the only thing the callback ever returns that's non-nil, and WalkDir treats SkipDir as a signal not an error, so walkErr is effectively always nil here. Still, surface it if present.)
6. `sort.Slice(result, func(i, j int) bool { return result[i].RelTag < result[j].RelTag })` — deterministic output (PRD §6.1 --all/--list "sorted by tag"). NOTE: WalkDir traversal is lexical BY FILENAME, not by relTag, so this sort is STILL REQUIRED for nested entries (e.g. `gate.ts` vs `writing/reddit-poster.ts` visit order ≠ relTag order).
7. `return result, nil`.

A NEW `internal/discover/index_test.go` (`package discover`, white-box — shares scope with discover_test.go/extension_test.go/jsdoc_test.go so it reuses `writeFile`, `writePackageJSON`, `strEq` without redefining them).

### Success Criteria

- [ ] `internal/discover/index.go` exists with `package discover` and the single
      exported function `Index(extensionsDir string) ([]Extension, error)`.
- [ ] `Index` calls `filepath.Abs(extensionsDir)` FIRST, then `os.Stat(root)`,
      THEN `filepath.WalkDir`. The stat-guard is BEFORE the walk (not inside the
      callback's err handling — that would swallow a missing root).
- [ ] `Index` returns `(nil, err)` for a missing/unreadable root (err non-nil).
- [ ] `Index` returns `(nil, err)` for a root that is a regular file (err non-nil,
      message contains "not a directory" or equivalent).
- [ ] `Index` returns a zero-length slice and nil error for an existing empty dir.
- [ ] The WalkDir callback branches on `d.IsDir()` and dispatches to
      `classifyDir` (dirs) / `classifyFile` (files). It returns `filepath.SkipDir`
      only when `classifyDir` returns `shouldDescend==false` (recognized extension
      dir). It returns `nil` for file entries (NOT SkipDir).
- [ ] Every emitted `Extension.Path` and `Extension.EntryFile` is absolute
      (`filepath.IsAbs`) even when `extensionsDir` is passed relative.
- [ ] The result is sorted by `RelTag` ascending (`sort.Slice`).
- [ ] The PRD §7.1 worked example produces EXACTLY 5 entries: `gate` (file),
      `git-checkpoint` (dir), `platform/linux` (dir), `summarizer` (package),
      `writing/reddit-poster` (file). `git-checkpoint/utils.ts`,
      `git-checkpoint/index.ts`, and `summarizer/src/index.ts` are NOT separate
      entries (the load-bearing recursion rule).
- [ ] `index.go` imports ONLY stdlib (`errors`, `io/fs`, `os`, `path/filepath`,
      `sort`). NO `encoding/json`. NO `internal/...` imports. NO direct calls to
      `parsePackageJSON`, `BuildExtension`, or `ExtractJSDoc` (those are inside
      the classify functions — Index delegates).
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/discover/...`,
      `go test -race ./...`, and `go test ./...` all pass.
- [ ] go.mod has no `require` block; no go.sum exists.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes.** This PRP names: the authoritative spec
(PRD §7.1, quoted in selected_prd_content); the EXACT port pattern (skilldozer's
`internal/discover/index.go`, read and quoted below); the EXACT proven reference
implementation (`walkClassified` in `discover_test.go:24-46`, created by the
parallel P1.M2.T2.S1, quoted in full); the contract for the two classify
functions Index delegates to (signatures + return-value semantics, from the
P1.M2.T2.S1 PRP); the verified WalkDir+SkipDir+stat-guard semantics
(research/walkdir_skipdir_semantics.md); the absolute-output contract (PRD §6.1,
§13); the exact worked-example test (5 entries, correct relTags/kinds); and the
test helpers already available (`writeFile`, `writePackageJSON`, `strEq`). The
function is pure stdlib + calls to same-package T2 functions — no external
library docs needed.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec. §7.1 (quoted in selected_prd_content) defines the
       classify-then-descend rule, the worked example, and the field semantics
       (path, entryFile, relTag, kind). §7.1 item 3 (the recursion rule) is
       load-bearing. §2 constraint 1 ("No catalog index — disk-discovered")
       means Index rebuilds on every call. §6.1 ("absolute path") + §13
       (`case "$(./weave example)" in /*)`) require absolute Path/EntryFile.
  critical: |
    PRD §7.1 item 3: "when a directory is recognized as a dir/package extension,
    do not descend into it." Index's callback returns filepath.SkipDir to enforce
    this. §2.1: "There is no extensions.json/index file enumerating the extension
    catalog; the set of extensions is always computed by walking the store on
    each call" — Index is that walk, called once per invocation.

- file: /home/dustin/projects/weave/internal/discover/discover_test.go  (P1.M2.T2.S1, PRESENT)
  why: THE CANONICAL REFERENCE. Contains `walkClassified` (lines 24-46) — a
       WORKING WalkDir skeleton that drives classifyFile/classifyDir and applies
       the SkipDir rule. Index() is literally this function PLUS filepath.Abs +
       the stat-guard + the error return. Copy the callback body VERBATIM.
  pattern: |
    func walkClassified(root string) []Extension {
        var result []Extension
        _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil }                       // skip unreadable entry
            if d.IsDir() {
                ext, isExt, descend := classifyDir(root, path)
                if isExt { result = append(result, *ext) }
                if !descend { return filepath.SkipDir }        // prune subtree
                return nil                                     // plain dir → descend
            }
            ext, ok := classifyFile(root, path)
            if ok { result = append(result, *ext) }
            return nil                                         // NOT SkipDir (skips siblings)
        })
        sort.Slice(result, func(i, j int) bool { return result[i].RelTag < result[j].RelTag })
        return result
    }
    // Index() = filepath.Abs(root) + os.Stat(root) guard + the above callback + error return.
  critical: |
    The callback body is PROVEN (TestWorkedExample in discover_test.go drives it
    and asserts exactly 5 entries with the correct relTags/kinds and no
    double-counting). Do NOT rewrite it differently — port it verbatim into
    Index's WalkDir call. The ONLY changes are the Abs preamble, the stat-guard,
    and returning (result, walkErr) instead of discarding the error.

- file: /home/dustin/projects/skilldozer/internal/discover/index.go  (READ-ONLY port pattern)
  why: skilldozer's Index is the direct ancestor. It has the EXACT shape weave
       needs: filepath.Abs → os.Stat guard → errors.New("not a directory") →
       WalkDir with err-skip → sort.Slice by RelTag → return. The ONLY difference
       is the callback body: skilldozer matches the FILE "SKILL.md"
       (`if d.IsDir() || d.Name() != "SKILL.md" { return nil }`); weave dispatches
       to classifyFile/classifyDir with SkipDir pruning.
  pattern: |
    // skilldozer index.go (the preamble + guard + sort to port VERBATIM):
    func Index(skillsDir string) ([]Skill, error) {
        root, err := filepath.Abs(skillsDir)
        if err != nil { return nil, err }
        info, err := os.Stat(root)
        if err != nil { return nil, err }
        if !info.IsDir() { return nil, errors.New(root + ": not a directory") }
        var skills []Skill
        walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil }
            // ... callback body (DIFFERS for weave — use walkClassified's body) ...
        })
        if walkErr != nil { return skills, walkErr }
        sort.Slice(skills, func(i, j int) bool { return skills[i].RelTag < skills[j].RelTag })
        return skills, nil
    }
  critical: |
    The stat-guard is load-bearing. skilldozer's own comment: "Stat-guard BEFORE
    WalkDir: a missing root is otherwise SWALLOWED. WalkDir feeds the root's
    lstat error to the callback, and the per-entry `if err != nil { return nil }`
    would hide it -> (nil, nil)." Port this guard EXACTLY (os.Stat + IsDir check
    + errors.New for not-a-dir). The walkErr != nil branch is defensive (the
    callback only ever returns nil or SkipDir; SkipDir is a signal not an error
    to WalkDir, so walkErr is effectively always nil — but keep the branch for
    safety, matching skilldozer).

- file: /home/dustin/projects/skilldozer/internal/discover/index_test.go  (READ-ONLY test pattern)
  why: The test PATTERNS to mirror in index_test.go: TestIndexMissingRoot,
       TestIndexRootIsFile, TestIndexEmptyDir, TestIndexSortedByRelTag,
       TestIndexRelativeInputStillAbsolute (uses t.Chdir — Go 1.24+, available in
       go 1.25), TestIndexIgnoresNonSkillMD (→ TestIndexIgnoresStrayFiles). Read
       these to copy the assertion style and the makeTree-style fixture approach.
  pattern: |
    // Missing root → error (the Stat guard):
    func TestIndexMissingRoot(t *testing.T) {
        _, err := Index(filepath.Join(t.TempDir(), "does-not-exist"))
        if err == nil { t.Fatal("err=nil; want an error (missing root must propagate)") }
    }
    // Root is a file → error:
    func TestIndexRootIsFile(t *testing.T) {
        f, _ := os.CreateTemp(t.TempDir(), "notadir"); f.Close()
        if _, err := Index(f.Name()); err == nil { t.Fatal("err=nil; want not-a-directory error") }
    }
    // Empty dir → (nil/len-0, nil):
    func TestIndexEmptyDir(t *testing.T) {
        got, err := Index(t.TempDir())
        if err != nil { t.Fatalf("err=%v; want nil", err) }
        if len(got) != 0 { t.Errorf("len=%d; want 0", len(got)) }
    }
    // Relative input → absolute output (t.Chdir scopes cwd; Go 1.24+):
    func TestIndexRelativeInputStillAbsolute(t *testing.T) {
        absRoot := <build tree>
        parent := filepath.Dir(absRoot)
        t.Chdir(parent)
        rel, _ := filepath.Rel(parent, absRoot)
        got, err := Index(rel)
        // assert filepath.IsAbs(got[0].Path)
    }

- file: /home/dustin/projects/weave/internal/discover/discover.go  (P1.M2.T2.S1, PRESENT — the CONTRACT Index delegates to)
  why: Index's callback calls these two functions. Their signatures + return-value
       semantics are the contract Index depends on:
       - func classifyFile(root, path string) (*Extension, bool)
         // (ext, true) = single-file extension; (nil, false) = index.ts/index.js or non-ts/js.
       - func classifyDir(root, path string) (ext *Extension, isExtension, shouldDescend bool)
         // (ext, true, false)  = recognized dir/package extension → emit + return SkipDir.
         // (nil, false, true)  = plain category dir → return nil, descend.
       Index does NOT call parsePackageJSON, BuildExtension, ExtractJSDoc, fileExists,
       isExtensionFile, or relTagForDir directly — those are INSIDE the classify
       functions. Index only calls classifyFile + classifyDir. This keeps T3 a thin
       dispatcher (architecture_mapping §3d).
  critical: |
    classifyDir's third return (shouldDescend) IS the recursion rule. When false,
    Index MUST return filepath.SkipDir (not nil — nil would descend and
    double-count). When true, Index MUST return nil (not SkipDir — SkipDir on a
    plain dir would skip its contents). classifyFile has no descend signal — file
    entries ALWAYS return nil from the callback (SkipDir on a file skips SIBLINGS,
    a GOTCHA — research/walkdir_skipdir_semantics.md point 3).

- file: /home/dustin/projects/weave/internal/discover/extension.go  (P1.M2.T1.S1, PRESENT)
  why: The Extension struct Index returns. Fields (confirmed read):
       type Extension struct {
           Path, EntryFile, RelTag, Kind, Name, Description string
           Keywords, Aliases []string
           Category string
           HasPackageJSON bool
       }
       Path/EntryFile MUST be absolute (Index's filepath.Abs guarantees this).
       RelTag is the sort key (already /-normalized and .ts/.js-stripped for files
       by the classify functions). Index does NOT touch these fields — it appends
       the *Extension the classifiers return.
  critical: Keywords/Aliases nil-vs-empty — callers test with len(), not nil check.
            Index passes them through unchanged.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §3d is THIS subtask's spec. Confirms: "Use filepath.WalkDir with custom
       descend logic"; "return filepath.SkipDir when an entry dir is recognized";
       "Stat-guard the root before walking (skilldozer pattern)"; "Sort by RelTag";
       "Empty store → nil slice, nil error." §3c is T2's spec (already done).
  critical: §3d's "Stat-guard the root before walking" is explicit and load-bearing.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M2T2S1/research/walkdir_skipdir_semantics.md
  why: The VERIFIED WalkDir+SkipDir+stat-guard semantics Index relies on. Confirms:
       (1) SkipDir on a DIR skips its subtree; (2) root is visited FIRST;
       (3) SkipDir on a FILE skips siblings (GOTCHA); (4) WalkDir does NOT follow
       symlinks; (5) IsDir() is reliable; (6) traversal is lexical by filename
       (so the final RelTag sort is STILL REQUIRED); (7) filepath.Rel + ToSlash.
  critical: |
    The stat-guard bug: without os.Stat(root) before WalkDir, a missing root's
    lstat error feeds the callback as (root, nil-dirEntry, err), and the callback's
    `if err != nil { return nil }` swallows it → (nil, nil). This is the bug the
    stat-guard exists to fix. Pin it with TestIndexMissingRoot.

- file: /home/dustin/projects/weave/go.mod
  why: Module `github.com/dabstractor/weave`, `go 1.25`. index.go imports are ALL
       stdlib (errors, io/fs, os, path/filepath, sort). go.mod gains NO require.
       t.Chdir (Go 1.24+) is available for TestIndexRelativeInputStillAbsolute.
  critical: Adding any third-party dep would violate PRD §2 (stdlib-only).
```

### Current Codebase tree

```bash
# After P1.M1 (Complete) + S1 (Complete) + S2 (Complete) + T2 (parallel, treated PRESENT).
# THIS subtask ADDS index.go + index_test.go.
$ cd /home/dustin/projects/weave && find . -name '*.go' -not -path './.git/*' | sort
./internal/config/config.go            # P1.M1.T2.S1 (Complete)
./internal/config/config_test.go
./internal/extdir/extdir.go            # P1.M1.T3.S1+S2+S3 (Complete)
./internal/extdir/extdir_test.go
./internal/discover/extension.go       # P1.M2.T1.S1 (Complete — PRESENT)
./internal/discover/extension_test.go  # P1.M2.T1.S1 (Complete — PRESENT)
./internal/discover/jsdoc.go           # P1.M2.T1.S2 (Complete — PRESENT)
./internal/discover/jsdoc_test.go      # P1.M2.T1.S2 (Complete — PRESENT)
./internal/discover/discover.go        # P1.M2.T2.S1 (parallel — treat as PRESENT)
./internal/discover/discover_test.go   # P1.M2.T2.S1 (parallel — PRESENT; has walkClassified + TestWorkedExample)
./main.go                              # P1.M1.T4.S1 (Complete — does NOT yet import discover)
./main_test.go
# THIS subtask ADDS:
#   ./internal/discover/index.go       (Index — WalkDir + stat-guard + sort)
#   ./internal/discover/index_test.go  (TestIndex* — error paths, abs, empty, sort, worked example)
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                    # UNCHANGED
│   ├── extdir/                    # UNCHANGED
│   └── discover/                  # S1 created; S2 added jsdoc.go; T2 added discover.go+discover_test.go
│       ├── extension.go           # S1 (PRESENT) — Extension, packageJSON, parsePackageJSON, BuildExtension, toStringSlice
│       ├── extension_test.go      # S1 (PRESENT) — writePackageJSON, strEq helpers
│       ├── jsdoc.go               # S2 (PRESENT) — ExtractJSDoc, utf8BOM, stripStarPrefix
│       ├── jsdoc_test.go          # S2 (PRESENT) — writeFile helper (REUSED in index_test.go)
│       ├── discover.go            # T2 (PRESENT) — classifyFile, classifyDir, isExtensionFile, fileExists, relTagForDir
│       ├── discover_test.go       # T2 (PRESENT) — walkClassified, TestWorkedExample, classify unit tests
│       ├── index.go               # NEW (this subtask) — Index(extensionsDir) ([]Extension, error)
│       └── index_test.go          # NEW (this subtask) — TestIndex* (error paths, abs, empty, sort, worked example)
├── main.go                        # UNCHANGED (M2.T5 wires --list → discover.Index later)
├── go.mod                         # UNCHANGED (no new require; all imports stdlib)
└── ...                            # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the stat-guard — skilldozer's verified bug): WITHOUT os.Stat(root)
// before filepath.WalkDir, a MISSING root is swallowed. WalkDir feeds the root's
// lstat error to the callback as (root, nil-dirEntry, err); the callback's
// `if err != nil { return nil }` hides it → Index returns (nil, nil). main.go
// would then exit 0 with empty output instead of erroring (violating PRD §6.4).
// The guard: os.Stat(root) BEFORE WalkDir; if err != nil return (nil, err); if
// !info.IsDir() return (nil, errors.New(root + ": not a directory")).
// Pin with TestIndexMissingRoot + TestIndexRootIsFile.

// CRITICAL (filepath.Abs FIRST): PRD §6.1/§13 require absolute Path/EntryFile.
// Call filepath.Abs(extensionsDir) BEFORE os.Stat and BEFORE WalkDir, and use the
// ABSOLUTE root as both the stat target and the WalkDir root AND the `root` arg
// passed to classifyFile/classifyDir (so relTag is computed against the absolute
// root). Do NOT pass the original (possibly relative) extensionsDir to WalkDir or
// the classifiers — that would yield relative Path/EntryFile and break the
// absolute contract. Pin with TestIndexRelativeInputStillAbsolute.

// CRITICAL (SkipDir only for recognized DIR extensions): return filepath.SkipDir
// ONLY when classifyDir returns shouldDescend==false. For FILE entries, ALWAYS
// return nil — SkipDir on a file skips the REMAINING files in that directory
// (siblings), which would silently drop sibling extensions in the same category
// folder (research/walkdir_skipdir_semantics.md point 3). walkClassified already
// encodes this correctly — copy its callback body verbatim.

// CRITICAL (sort is STILL REQUIRED even though WalkDir is lexical): WalkDir
// traverses lexical BY FILENAME within each dir, NOT by relTag. Nested entries
// (gate.ts vs writing/reddit-poster.ts) visit in filename order, but the sort key
// is the full relTag. So `sort.Slice(result, by RelTag)` is mandatory for
// deterministic output (PRD §6.1 --list/--all "sorted by tag"). walkClassified
// already sorts — copy that line.

// CRITICAL (Index does NOT call parsePackageJSON/BuildExtension/ExtractJSDoc
// directly): those live INSIDE classifyFile/classifyDir (T2). Index only calls
// classifyFile + classifyDir. Calling the metadata functions directly would
// duplicate T2's logic and risk drift (e.g. forgetting the lenient
// parsePackageJSON err-ignore policy, or the description fallback). This keeps
// T3 a thin ~50-line dispatcher (architecture_mapping §3d). grep index.go for
// parsePackageJSON/BuildExtension/ExtractJSDoc — must find ZERO matches.

// CRITICAL (root is visited FIRST by WalkDir, and classifyDir(root) returns
// plain/descend): WalkDir calls the callback for root (path==root, IsDir==true)
// before its children. classifyDir(root, root) will call parsePackageJSON(root)
// (the extensions container — usually no package.json → packageJSON{}, false, nil)
// and find no index.ts/index.js → returns (nil, false, true) → plain, descend →
// callback returns nil → WalkDir descends. This is CORRECT and harmless. Do NOT
// add a special-case `if path == root { return nil }` guard — it's unnecessary
// (classifyDir already returns descend for the root) and would be dead code.
// walkClassified handles root this way already (TestWorkedExample's root is the
// temp dir and it works).

// GOTCHA (imports: errors, io/fs, os, path/filepath, sort — ALL stdlib): do NOT
// import encoding/json (parsePackageJSON is in extension.go, same package — and
// Index doesn't even call it; the classifiers do). Do NOT import any internal/
// package (discover has no cross-package deps). Do NOT import strings (the
// .ts/.js stripping and ToSlash happen in the classifiers, not Index).

// GOTCHA (no package-level doc comment on index.go): S1's extension.go carries
// the package doc comment. Adding a second is ignored by godoc but is poor form.
// index.go starts with an ORDINARY doc comment on the Index function itself.

// GOTCHA (var result []Extension nil-init): an empty extensions dir yields ZERO
// appends, so `result` stays nil. Return nil (NOT an empty non-nil slice) — this
// matches the "empty store → nil slice" contract (§3d) and lets callers test
// len()==0. Do NOT pre-allocate `result := make([]Extension, 0)` — that would
// return a non-nil empty slice, which is semantically different (and breaks any
// caller doing `if exts == nil`).

// GOTCHA (walkErr is effectively always nil): the callback only ever returns nil
// or filepath.SkipDir. WalkDir treats SkipDir as a SIGNAL (skip subtree/siblings),
// not an error to propagate — so walkErr is nil unless something truly abnormal
// happens (e.g. a panic in the callback, which Go recovers into an error). Keep
// the `if walkErr != nil { return result, walkErr }` branch for defensive parity
// with skilldozer, but don't expect it to fire in tests.

// GOTCHA (reuse test helpers, don't redefine): index_test.go is `package
// discover` (white-box), so it shares scope with extension_test.go
// (writePackageJSON, strEq), jsdoc_test.go (writeFile), and discover_test.go
// (walkClassified, relTags). Reuse them — redeclaring any is a COMPILE ERROR.
// If you need a tree-builder like skilldozer's makeTree, either inline the
// writeFile/writePackageJSON calls (as TestWorkedExample in discover_test.go
// does) or add a NEW helper with a distinct name.
```

## Implementation Blueprint

### Data models and structure

NO new data models. This subtask uses S1's `Extension` struct (already in extension.go) and T2's classify functions (already in discover.go). It adds ONLY the `Index` function.

```go
// Index — see above. The sole symbol in index.go.
func Index(extensionsDir string) ([]Extension, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE index.go skeleton + imports + Index signature
  - CREATE /home/dustin/projects/weave/internal/discover/index.go with `package discover`.
  - IMPORTS (exact set, alphabetical, ALL stdlib): errors, io/fs, os, path/filepath, sort.
    (Do NOT import encoding/json, strings, or any internal/ package.)
  - NO package-level doc comment (S1's extension.go owns it — see GOTCHA).
  - ADD `func Index(extensionsDir string) ([]Extension, error)` with the
    comprehensive doc comment from the "What" section (absolute contract,
    classify-then-descend, SkipDir rule, error policy, empty→nil, no symlinks).
  - PLACEMENT: this is the ONLY function in index.go.

Task 2: IMPLEMENT Index — Abs + stat-guard
  - LOGIC (preamble — port skilldozer index.go:60-72 VERBATIM):
        root, err := filepath.Abs(extensionsDir)
        if err != nil {
            return nil, err
        }
        info, err := os.Stat(root)
        if err != nil {
            return nil, err  // missing/unreadable root
        }
        if !info.IsDir() {
            return nil, errors.New(root + ": not a directory")
        }
  - NOTE: filepath.Abs on Unix is effectively a no-op Clean for absolute inputs and
    a cwd-join for relative inputs; it errors only on Windows for paths that can't
    be made absolute. Return (nil, err) defensively.

Task 3: IMPLEMENT Index — WalkDir callback (port walkClassified's body VERBATIM)
  - LOGIC:
        var result []Extension  // nil-init: empty store yields nil (NOT make([]Extension, 0))
        walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                return nil  // per-entry unreadable (chmod-000 subdir) → skip, keep walking
            }
            if d.IsDir() {
                ext, isExt, descend := classifyDir(root, path)
                if isExt {
                    result = append(result, *ext)
                }
                if !descend {
                    return filepath.SkipDir  // prune subtree (load-bearing recursion rule)
                }
                return nil  // plain category dir → descend
            }
            ext, ok := classifyFile(root, path)
            if ok {
                result = append(result, *ext)
            }
            return nil  // NOT SkipDir (SkipDir on a file skips siblings — GOTCHA)
        })
  - NOTE: this callback body is IDENTICAL to walkClassified in discover_test.go:24-46.
    Copy it verbatim — it is PROVEN by TestWorkedExample. The ONLY difference is the
    enclosing function (Index returns error; walkClassified discards it via `_ =`).
  - NOTE: `root` passed to classifyDir/classifyFile is the ABSOLUTE root (from
    filepath.Abs), so the classifiers compute relTag against the absolute root and
    produce absolute Path/EntryFile (the classifiers use `path` directly from
    WalkDir, which is under the absolute root → absolute).

Task 4: IMPLEMENT Index — error branch + sort + return
  - LOGIC (port skilldozer index.go:90-96 VERBATIM):
        if walkErr != nil {
            return result, walkErr  // defensive; walkErr is effectively always nil
        }
        sort.Slice(result, func(i, j int) bool {
            return result[i].RelTag < result[j].RelTag
        })
        return result, nil
  - NOTE: sort is mandatory — WalkDir is lexical by FILENAME, not by relTag (nested
    entries like gate.ts vs writing/reddit-poster.ts need reordering).
  - NOTE: `result` is nil if nothing was appended (empty store) → returns (nil, nil),
    matching the §3d contract.

Task 5: CREATE index_test.go — TestIndexMissingRoot + TestIndexRootIsFile (stat-guard)
  - CREATE /home/dustin/projects/weave/internal/discover/index_test.go with `package discover`.
  - IMPORTS: os, path/filepath, testing. (strings only if needed for backslash check.)
  - ADD `func TestIndexMissingRoot(t *testing.T)`:
        _, err := Index(filepath.Join(t.TempDir(), "does-not-exist"))
        if err == nil {
            t.Fatal("err=nil; want an error (missing root must propagate after the Stat guard)")
        }
  - ADD `func TestIndexRootIsFile(t *testing.T)`:
        f, err := os.CreateTemp(t.TempDir(), "notadir")
        if err != nil { t.Fatal(err) }
        f.Close()
        _, err = Index(f.Name())
        if err == nil {
            t.Fatal("err=nil; want an error (root must be a directory)")
        }
  - NOTE: these pin the stat-guard. WITHOUT the guard, TestIndexMissingRoot would
    fail (Index would return (nil,nil)).

Task 6: CREATE index_test.go — TestIndexEmptyDir + TestIndexNilOnEmpty
  - ADD `func TestIndexEmptyDir(t *testing.T)`:
        got, err := Index(t.TempDir())  // exists, empty
        if err != nil {
            t.Fatalf("err=%v; want nil (empty dir is not an error)", err)
        }
        if len(got) != 0 {
            t.Errorf("len=%d; want 0 (empty tree → no extensions)", len(got))
        }
  - ADD `func TestIndexNilOnEmpty(t *testing.T)` (pin nil-vs-empty contract):
        got, _ := Index(t.TempDir())
        if got != nil {
            t.Errorf("got=%v; want nil (empty store → nil slice, §3d contract)", got)
        }
  - NOTE: the nil-vs-empty distinction matters for callers testing `if exts == nil`.
    `var result []Extension` with zero appends yields nil. Pre-allocating with
    make([]Extension, 0) would yield a non-nil empty slice and FAIL TestIndexNilOnEmpty.

Task 7: CREATE index_test.go — TestIndexSortedByRelTag
  - ADD `func TestIndexSortedByRelTag(t *testing.T)`:
        root := t.TempDir()
        // Create entries whose FILENAME order ≠ RELTAG order, to prove the sort
        // is by relTag (lexical), not by walk-visit order.
        writeFile(t, root, "zebra.ts", "/** z. */\n")              // tag "zebra"
        writeFile(t, root, "apple.ts", "/** a. */\n")              // tag "apple"
        writeFile(t, filepath.Join(root, "mango"), "fig.ts", "/** f. */\n")   // tag "mango/fig"
        writeFile(t, filepath.Join(root, "mango"), "beta.ts", "/** b. */\n")  // tag "mango/beta"
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        var tags []string
        for _, e := range got { tags = append(tags, e.RelTag) }
        want := []string{"apple", "mango/beta", "mango/fig", "zebra"}
        if !strEq(tags, want) {
            t.Errorf("order=%v; want %v (lexicographic by RelTag)", tags, want)
        }
  - NOTE: reuses writeFile (jsdoc_test.go) and strEq (extension_test.go).

Task 8: CREATE index_test.go — TestIndexRelativeInputStillAbsolute
  - ADD `func TestIndexRelativeInputStillAbsolute(t *testing.T)`:
        absRoot := t.TempDir()
        writeFile(t, absRoot, "example.ts", "/** ex. */\n")
        parent := filepath.Dir(absRoot)
        t.Chdir(parent)  // Go 1.24+; go.mod is 1.25 ✓
        rel, err := filepath.Rel(parent, absRoot)
        if err != nil { t.Fatal(err) }
        got, err := Index(rel)  // RELATIVE input
        if err != nil { t.Fatalf("err=%v", err) }
        if len(got) != 1 {
            t.Fatalf("len=%d; want 1", len(got))
        }
        if !filepath.IsAbs(got[0].Path) {
            t.Errorf("Path=%q is RELATIVE; want absolute (relative input must still abs-ify)", got[0].Path)
        }
        if !filepath.IsAbs(got[0].EntryFile) {
            t.Errorf("EntryFile=%q is RELATIVE; want absolute", got[0].EntryFile)
        }
        if got[0].RelTag != "example" {
            t.Errorf("RelTag=%q; want example", got[0].RelTag)
        }
  - NOTE: t.Chdir restores cwd on test completion (Go 1.24+). This pins the
    PRD §6.1/§13 absolute-output contract.

Task 9: CREATE index_test.go — TestIndexWorkedExample (the acceptance test, via PUBLIC Index)
  - ADD `func TestIndexWorkedExample(t *testing.T)` — build the EXACT PRD §7.1 tree:
        root := t.TempDir()
        writeFile(t, root, "gate.ts", "/** Gate. */\n")
        writeFile(t, filepath.Join(root, "git-checkpoint"), "index.ts", "/** Checkpoint. */\n")
        writeFile(t, filepath.Join(root, "git-checkpoint"), "utils.ts", "// internal helper\n")
        writePackageJSON(t, filepath.Join(root, "summarizer"), `{
          "name": "@o/summarizer",
          "description": "Summarize.",
          "pi": { "extensions": ["./src/index.ts"] }
        }`)
        writeFile(t, filepath.Join(root, "summarizer", "src"), "index.ts", "/** pkg entry. */\n")
        writeFile(t, filepath.Join(root, "writing"), "reddit-poster.ts", "/** Post to reddit. */\n")
        writeFile(t, filepath.Join(root, "platform", "linux"), "index.ts", "/** Linux platform. */\n")
    Call `got, err := Index(root)`; assert err==nil; assert len==5; assert the 5
    RelTags (sorted) are exactly:
        ["gate", "git-checkpoint", "platform/linux", "summarizer", "writing/reddit-poster"]
    Assert kinds: gate→file, git-checkpoint→dir, platform/linux→dir, summarizer→
    package, writing/reddit-poster→file. Assert utils.ts, git-checkpoint/index.ts,
    and summarizer/src/index.ts do NOT appear as separate entries (the load-bearing
    recursion rule). Build a map[relTag]Extension for lookup (mirror TestWorkedExample
    in discover_test.go). Assert every Path/EntryFile is absolute (filepath.IsAbs).
  - NOTE: this is the SAME tree as TestWorkedExample in discover_test.go, but driven
    through the PUBLIC Index() instead of walkClassified. It proves Index ==
    walkClassified + guard. (Optionally factor a shared assertWorkedExample helper
    that both call — lower priority; duplication is acceptable and clearer.)

Task 10: CREATE index_test.go — TestIndexIgnoresStrayFiles
  - ADD `func TestIndexIgnoresStrayFiles(t *testing.T)`:
        root := t.TempDir()
        writeFile(t, filepath.Join(root, "real"), "gate.ts", "/** real. */\n")
        // Distractions that must NOT be treated as extensions:
        writeFile(t, root, "README.md", "# hi\n")
        writeFile(t, filepath.Join(root, "notes"), "draft.txt", "draft\n")
        writeFile(t, root, "gate.ts.bak", "bak\n")
        got, err := Index(root)
        if err != nil { t.Fatalf("err=%v", err) }
        if len(got) != 1 || got[0].RelTag != "real/gate" {
            t.Fatalf("got=%v; want exactly one extension 'real/gate' (stray files/subdirs ignored)", got)
        }
  - NOTE: README.md (non-.ts/.js), draft.txt (in a plain subdir), and .bak
    (non-.ts/.js) are all ignored by classifyFile. "notes" is a plain dir with no
    entries (classifyDir returns descend, nothing appended).

Task 11: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...                    # expect exit 0
  - RUN: go vet ./...                                                        # expect exit 0, clean
  - RUN: go test ./internal/discover/... -v                                  # expect ALL PASS (S1+S2+T2+T3)
  - RUN: go test ./... -v                                                    # expect ALL PASS
  - RUN: go test -race ./...                                                 # expect no data races
  - RUN: gofmt -l internal/discover/index.go internal/discover/index_test.go # expect no output
  - RUN: grep -rn "encoding/json\|yaml.v3\|gopkg.in" --include=*.go ./internal/discover/index.go || echo "no forbidden imports (correct)"
  - RUN: grep -qE "parsePackageJSON|BuildExtension|ExtractJSDoc" ./internal/discover/index.go && echo "FAIL: Index must delegate to classify*, not call metadata funcs directly" || echo "OK"
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK                   # expect OK
  - RUN: test ! -f go.sum && echo OK || echo FAIL                            # expect OK
  - RUN: ! grep -qE 'internal/extdir|internal/config' ./internal/discover/index.go && echo "no internal imports (correct)" || echo "FAIL"
```

### Implementation Patterns & Key Details

```go
// The COMPLETE index.go (≈55 lines). Port walkClassified's callback body VERBATIM;
// add the skilldozer Abs+stat-guard preamble and the error-return tail.

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
// deterministic output (PRD §7.1). [full doc comment from the "What" section]
func Index(extensionsDir string) ([]Extension, error) {
	root, err := filepath.Abs(extensionsDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
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
```

### Integration Points

```yaml
PACKAGE BOUNDARY:
  - index.go is `package discover` — same package as extension.go, jsdoc.go, discover.go.
    It calls the UNEXPORTED classifyFile/classifyDir directly (no import needed; same package).
  - NO new exports beyond Index. The classify functions stay unexported (T2's choice);
    Index is the ONLY public entry point to the catalog.

MAIN.GO (FUTURE — NOT this subtask):
  - M2.T5.S1 (--list):   exts, err := discover.Index(dir); ui.PrintList(stdout, exts)
  - M3.T2.S1 (tag/-f):   exts, err := discover.Index(dir); resolve.Resolve(exts, tag)
  - M4.T3.S1 (search):   exts, err := discover.Index(dir); search.Search(exts, q)
  - M4.T3.S1 (check):    exts, err := discover.Index(dir); check.Check(exts)
  - This subtask does NOT touch main.go. It only provides the function those milestones call.

CONFIG:
  - none. Index takes a plain string path. The dir RESOLUTION (extdir.Find) is the
    caller's job (main.go), not Index's.

DATABASE:
  - none. PRD §2.1: no catalog index file. Index rebuilds from disk on every call.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating index.go + index_test.go — fix before proceeding.
cd /home/dustin/projects/weave
gofmt -l internal/discover/index.go internal/discover/index_test.go   # expect no output
go vet ./internal/discover/...                                          # expect clean

# Project-wide validation
go build ./...                                                          # expect exit 0
go vet ./...                                                            # expect exit 0, clean

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the discover package (S1 + S2 + T2 + T3 together — all must pass).
cd /home/dustin/projects/weave
go test ./internal/discover/... -v                                      # expect ALL PASS
go test ./internal/discover/... -run 'TestIndex' -v                     # just T3's tests

# Targeted: the stat-guard tests (the load-bearing additions over walkClassified)
go test ./internal/discover/... -run 'TestIndexMissingRoot|TestIndexRootIsFile' -v

# Targeted: the worked example via the PUBLIC Index()
go test ./internal/discover/... -run 'TestIndexWorkedExample' -v

# Race detector
go test -race ./...

# Full suite
go test ./... -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# Confirm the public function compiles and is callable from outside the package.
cd /home/dustin/projects/weave
# (discover.Index is not yet wired into main.go — M2.T5 does that. This subtask
# is verified via the white-box index_test.go in Level 2. The integration check
# here is just "the package builds and the function is reachable.")

# Smoke-test Index directly via a tiny throwaway program (optional, manual):
cat > /tmp/index_smoke_test.go <<'EOF'
package main
import (
    "fmt"
    "github.com/dabstractor/weave/internal/discover"
)
func main() {
    exts, err := discover.Index("/tmp/nonexistent-dir-xyz")
    fmt.Printf("missing-dir: exts=%v err=%v\n", exts, err)
}
EOF
go run /tmp/index_smoke_test.go
# Expected output: missing-dir: exts=[] err=stat /tmp/nonexistent-dir-xyz: no such file or directory
# (confirms the stat-guard surfaces the error — the load-bearing fix.)
rm -f /tmp/index_smoke_test.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Verify the classify-then-descend rule end-to-end via the public Index().
cd /home/dustin/projects/weave
# Build the PRD §7.1 worked example on disk and run Index through a one-liner:
TMP=$(mktemp -d)
mkdir -p "$TMP/git-checkpoint" "$TMP/summarizer/src" "$TMP/writing" "$TMP/platform/linux"
printf '/** Gate. */\n' > "$TMP/gate.ts"
printf '/** Checkpoint. */\n' > "$TMP/git-checkpoint/index.ts"
printf '// internal\n' > "$TMP/git-checkpoint/utils.ts"
printf '{"name":"@o/summarizer","pi":{"extensions":["./src/index.ts"]}}\n' > "$TMP/summarizer/package.json"
printf '/** pkg. */\n' > "$TMP/summarizer/src/index.ts"
printf '/** Reddit. */\n' > "$TMP/writing/reddit-poster.ts"
printf '/** Linux. */\n' > "$TMP/platform/linux/index.ts"
cat > /tmp/worked_example.go <<EOF
package main
import ("fmt"; "github.com/dabstractor/weave/internal/discover")
func main() {
    exts, _ := discover.Index("$TMP")
    for _, e := range exts { fmt.Printf("%-25s %s\n", e.RelTag, e.Kind) }
}
EOF
go run /tmp/worked_example.go
# Expected output (sorted, exactly 5, NO utils.ts/src/index.ts/index.ts entries):
#   gate                     file
#   git-checkpoint           dir
#   platform/linux           dir
#   summarizer               package
#   writing/reddit-poster    file
rm -rf "$TMP" /tmp/worked_example.go

# Expected: exactly 5 lines, correct kinds, NO double-counted internals.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `cd /home/dustin/projects/weave && go test ./... -v`
- [ ] `go test -race ./...` passes (no data races).
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/discover/index.go internal/discover/index_test.go` (no output)
- [ ] go.mod has no `require` block; no go.sum exists.

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] `Index(missingDir)` returns a non-nil error (stat-guard pinned by TestIndexMissingRoot).
- [ ] `Index(regularFile)` returns a non-nil error (pinned by TestIndexRootIsFile).
- [ ] `Index(emptyDir)` returns (nil, nil) (pinned by TestIndexEmptyDir + TestIndexNilOnEmpty).
- [ ] Relative input yields absolute Path/EntryFile (pinned by TestIndexRelativeInputStillAbsolute).
- [ ] Results sorted by RelTag (pinned by TestIndexSortedByRelTag).
- [ ] Worked example: exactly 5 entries, correct relTags + kinds, no double-counting (TestIndexWorkedExample).
- [ ] Stray files (README.md, .bak, .txt) ignored (TestIndexIgnoresStrayFiles).
- [ ] Error cases handled: missing root, not-a-directory root both surface errors.

### Code Quality Validation

- [ ] index.go imports ONLY stdlib (errors, io/fs, os, path/filepath, sort).
- [ ] Index delegates to classifyFile/classifyDir — does NOT call parsePackageJSON, BuildExtension, or ExtractJSDoc directly (grep confirms zero matches).
- [ ] No internal-package imports (discover has no cross-package deps).
- [ ] Callback body matches walkClassified verbatim (the proven reference).
- [ ] Stat-guard is BEFORE WalkDir (not inside the callback's err handler).
- [ ] `var result []Extension` nil-init (not make([]Extension, 0)) — preserves nil-on-empty contract.
- [ ] No package-level doc comment (extension.go owns it).

### Documentation & Deployment

- [ ] Index has a comprehensive doc comment (absolute contract, classify-then-descend, SkipDir rule, error policy, empty→nil, no symlinks).
- [ ] Code is self-documenting with clear variable names (root, info, result, walkErr).
- [ ] No environment variables introduced (Index takes a plain path).

---

## Anti-Patterns to Avoid

- ❌ Don't skip the stat-guard — a missing root would silently return (nil,nil), violating PRD §6.4 error semantics.
- ❌ Don't call parsePackageJSON/BuildExtension/ExtractJSDoc directly in Index — delegate to classifyFile/classifyDir (T2 owns metadata; T3 owns the walk).
- ❌ Don't return filepath.SkipDir for FILE entries — it skips siblings, silently dropping sibling extensions.
- ❌ Don't skip the sort — WalkDir is lexical by filename, not by relTag; nested entries need reordering.
- ❌ Don't pass the original (possibly relative) extensionsDir to WalkDir or the classifiers — Abs it FIRST so Path/EntryFile are absolute.
- ❌ Don't pre-allocate `result := make([]Extension, 0)` — it breaks the nil-on-empty contract.
- ❌ Don't add a special-case `if path == root { return nil }` guard — classifyDir(root) already returns plain/descend; it's dead code.
- ❌ Don't import encoding/json, strings, or any internal/ package — Index is pure stdlib + same-package classify calls.
- ❌ Don't rewrite the WalkDir callback differently from walkClassified — it's the PROVEN reference; port it verbatim.
- ❌ Don't catch all exceptions / ignore walkErr — surface it defensively (even though it's effectively always nil).
