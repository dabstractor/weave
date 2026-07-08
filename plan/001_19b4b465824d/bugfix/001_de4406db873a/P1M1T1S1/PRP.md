# PRP — P1.M1.T1.S1: Iterate `pi.extensions` entries in `classifyDir`, select first existing

(Bugfix 001_de4406db873a — Issue 1, Major. Discovery subsystem.)

## Goal

**Feature Goal**: Fix `discover.classifyDir` case (a) so a package whose
`package.json` declares `pi.extensions: ["./missing.ts", "./src/real.ts"]` (first entry
absent, a later entry present) is correctly classified as ONE package extension with
`entryFile` = the first EXISTING entry — instead of being misclassified as a plain
category folder, descended into, and fragmented into stray single-file entries.

**Deliverable**: A localized change to ONE code block in
`internal/discover/discover.go` (the body of `if len(entries) > 0 { ... }` inside
`classifyDir` case (a)), two inline comment updates (the fall-through comment + the
case-(a) doc line), and three new regression tests in
`internal/discover/discover_test.go`. No other files change.

**Success Definition**:
- `classifyDir` returns `(ext, true, false)` for a package with ≥1 existing
  `pi.extensions` entry regardless of that entry's position in the array.
- `ext.EntryFile` is the FIRST existing entry path; `ext.RelTag` is the dir relative to
  root (no `.ts`/`.js` strip — `relTagForDir`); `ext.Kind == "package"`.
- When ALL declared entries are missing, `classifyDir` falls through to cases b/c/d/e
  unchanged (`isExt=false, descend=true`).
- The three new tests pass; all existing tests still pass; `go vet ./internal/discover/`
  is clean; `go build ./...` succeeds.
- End-to-end: `weave <pkg>` / `weave -f <pkg>` / `weave --list` on the bug's repro store
  behave per PRD §6/§7.1 (manual confirmation, not a unit test).

## Why

- **Correctness / PRD conformance**: PRD §7.1 explicitly defines a package extension as
  a `package.json` with `pi.extensions` "naming **≥1 existing entry**" and `entryFile`
  as "**the first existing** `pi.extensions` entry". The current code only checks
  `entries[0]`, violating both clauses. This is a direct spec implementation.
- **Cascading harm**: The misclassification causes `weave <pkg>` and `weave -f <pkg>` to
  return "unknown extension tag" (exit 1), `--list` to omit the package and show its
  entry file as a stray single-file extension (losing name/description/keywords), and
  `check` to emit a spurious **duplicate relTag ERROR** when two real siblings share a
  stripped stem (e.g. `real.js` + `real.ts`). One localized fix dissolves all of these.
- **Internal consistency**: `extdir.hasPiExtensions` (used by `weave init` cwd
  auto-detect and the §8.3 walk-up qualifier) ALREADY iterates all entries. Today, `weave
  init` run inside such a package dir will "Adopt existing store", but `weave --list` on
  that same store then mis-discovers it — two code paths disagreeing on what a package
  extension is. This fix makes `discover.classifyDir` agree with `extdir.hasPiExtensions`.
- **Unblocks P1.M3.T2.S1** (Bug 3 / Issue 5): the `check` all-missing-ERROR fix depends
  on this fix landing first (see `architecture/bug_cascade_map.md` dependency graph).

## What

### The change (internal/discover/discover.go, `classifyDir` case (a))

Replace the single-entry check:
```go
entryFile := filepath.Join(path, entries[0])
if fileExists(entryFile) {
    relTag := relTagForDir(root, path)
    jsdoc := ExtractJSDoc(entryFile)
    e := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
    return &e, true, false // shouldDescend=false (load-bearing)
}
// pi.extensions names a non-existent file → NOT a package; fall through.
```
with a loop over ALL entries that selects the first existing one:
```go
var entryFile string
for _, e := range entries {
    if f := filepath.Join(path, e); fileExists(f) {
        entryFile = f
        break
    }
}
if entryFile != "" {
    relTag := relTagForDir(root, path)
    jsdoc := ExtractJSDoc(entryFile)
    e := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
    return &e, true, false // shouldDescend=false (load-bearing)
}
// no existing entry among ANY declared pi.extensions → fall through to b/c/d/e.
```

> Note: the loop variable must not shadow the outer `e *Extension` return semantics —
> use a distinct name (`e` inside the loop shadows nothing problematic because the
> `Extension` is built and assigned to a separate `ext`/returned inline). The contract's
> sketch uses `e` for the loop var and then `e := BuildExtension(...)`; to avoid
> confusion, name the loop var `entry` (or keep `e` — both compile, since the inner
> `e :=` redeclares in a new scope). Pick one and be consistent; see Implementation
> Tasks Task 2 for the recommended naming.

### Doc-comment updates (ride with the code change — Mode A, no separate docs subtask)

1. The case-(a) block comment already says "EntryFile = first existing pi.extensions
   entry" — this is the CORRECT spec, so no change needed to that line. Verify it still
   reads accurately after the edit (it should).
2. The inline fall-through comment changes from
   `// pi.extensions names a non-existent file → NOT a package; fall through.`
   to
   `// no existing entry among ANY declared pi.extensions → fall through to b/c/d/e.`
   (reflects that the check is now "any", not "the first one").

### Success Criteria

- [ ] `classifyDir` returns `(ext, true, false)` when ≥1 declared `pi.extensions` entry
      exists, regardless of position; `ext.EntryFile` is the first existing entry.
- [ ] When ALL declared entries are missing, `classifyDir` returns
      `(nil, false, true)` (falls through to b/c/d/e).
- [ ] Cases (b), (c), (d), (e) and the `ParsePackageJSON` call are byte-for-byte
      unchanged.
- [ ] `TestClassifyDirPackageMultiEntryFirstMissing` passes.
- [ ] `TestClassifyDirPackageMultiEntryAllMissing` passes.
- [ ] An end-to-end test via `walkClassified` confirms the package resolves as ONE entry
      (no stray single-file fragment).
- [ ] All pre-existing tests in `internal/discover` still pass.
- [ ] `go vet ./internal/discover/` clean; `go build ./...` succeeds.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP quotes the exact buggy block, gives the
exact replacement code, cites the authoritative iteration pattern to mirror
(`extdir.hasPiExtensions`), lists exact signatures (`BuildExtension`, `Extension`,
`toStringSlice`, `fileExists`, `relTagForDir`), specifies the exact test helpers and
assertion idiom to reuse, and bounds the scope to a single block + 3 tests. No further
codebase spelunking is required.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for the fix. §7.1 defines a package extension as
       "package.json with pi.extensions array naming ≥1 existing entry" and entryFile
       as "the first existing pi.extensions entry (package)".
  critical: The fix is a direct implementation of these two clauses. The current
            doc comment already cites them; only the CODE was wrong.

- file: /home/dustin/projects/weave/internal/discover/discover.go
  why: CONTAINS THE BUG. classifyDir case (a), the `if len(entries) > 0 { ... }` block.
       This is the ONLY file whose non-test code changes.
  pattern: |
    // Current (BUGGY):
    entryFile := filepath.Join(path, entries[0])        // only entries[0]
    if fileExists(entryFile) { ... return &e, true, false }
    // pi.extensions names a non-existent file → NOT a package; fall through.
  gotcha: Keep the `if hasPkg && perr == nil {` guard, the `entries := toStringSlice(...)`
          line, and the `if len(entries) > 0 {` guard UNCHANGED. Only the body changes.

- file: /home/dustin/projects/weave/internal/extdir/extdir.go
  why: CONTAINS THE CORRECT ITERATION to mirror — hasPiExtensions (lines ~381-400).
       It loops over all entries and returns true if ANY exists.
  pattern: |
    for _, e := range pkg.Pi.Extensions {
        if fileExists(filepath.Join(dir, e)) { return true }
    }
  critical: classifyDir must do the same loop but CAPTURE the first existing path
            (return the path, not just a bool). This is the "aggravating factor" fix —
            it makes discover agree with extdir (which weave init already uses).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/bug_cascade_map.md
  why: Bug 1 entry gives the exact code path, the cascade (5 steps), and the minimal fix.
       Also documents that Bug 3 (check WARN→ERROR) DEPENDS on this fix.
  critical: Do NOT also fix Bug 3 here (it's P1.M3.T2.S1). Do NOT touch cases b/c/d/e.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/test_patterns.md
  why: Lists the exact test helpers (writeFile, writePackageJSON, walkClassified,
       relTags) and the check assertion idiom. Confirms tests are white-box
       (package discover).
  critical: Use walkClassified for the end-to-end regression test (it drives the real
            SkipDir rule). Use strings.HasSuffix for EntryFile assertions (abs path varies).

- file: /home/dustin/projects/weave/internal/discover/extension.go
  why: Defines the signatures the fix depends on — BuildExtension (line 289),
       Extension struct (line 156), toStringSlice (line 88).
  pattern: |
    func BuildExtension(path, entryFile, relTag, kind string, pkg PackageJSON, hasPkg bool, jsdocDesc string) Extension
  gotcha: pkg.Pi.Extensions is a lenientAnySlice (NOT []string); always go through
          toStringSlice to get []string. The buggy code already does this — keep it.
```

### Current Codebase tree (the files this subtask touches)

```bash
internal/discover/
├── discover.go          # ← EDIT: classifyDir case (a) body + 2 comments
├── discover_test.go     # ← EDIT: add 3 regression tests
├── extension.go         # (read-only reference: BuildExtension, Extension, toStringSlice)
├── extension_test.go    # (read-only reference: writePackageJSON helper)
├── index.go             # (read-only reference: Index, the real WalkDir consumer)
├── jsdoc.go             # (read-only reference: ExtractJSDoc)
├── jsdoc_test.go        # (read-only reference: writeFile helper)
└── index_test.go        # (unchanged)
```

### Desired Codebase tree (no NEW files; edits only)

```bash
# No files added. No files deleted. Two existing files edited:
internal/discover/discover.go       # classifyDir case (a): entries[0] → loop; 2 comment updates
internal/discover/discover_test.go  # + TestClassifyDirPackageMultiEntryFirstMissing
                                     # + TestClassifyDirPackageMultiEntryAllMissing
                                     # + TestWalkClassifiedPackageMultiEntryResolvesAsOne (e2e)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — the fix must preserve the load-bearing `shouldDescend=false` return.
// classifyDir's third return value drives the SkipDir recursion rule in Index/walkClassified.
// A package extension MUST return (ext, true, false) so its subtree is pruned — otherwise
// the entry file(s) inside src/ get double-counted as stray single-file extensions (the
// exact bug being fixed). Do NOT change the return shape.

// CRITICAL — keep the `if hasPkg && perr == nil {` and `if len(entries) > 0 {` guards.
// perr != nil means broken JSON → handled by case (b), NOT this loop. An empty entries
// slice → falls through to c/d/e (a package.json with no pi.extensions can still be a
// dir extension via index.ts). Both guards are load-bearing and unchanged.

// GOTCHA — loop variable shadowing. The contract sketch uses `e` for the loop var and
// then `e := BuildExtension(...)`. This compiles (inner `e :=` redeclares in a new scope)
// but is confusing. RECOMMENDED: name the loop var `entry` and keep `e :=` for the
// Extension, matching the style of cases c/d which use `entryFile :=` + `e :=`.

// GOTCHA — EntryFile assertion in tests. classifyDir returns ABSOLUTE paths (root comes
// from t.TempDir(), which is absolute and machine-specific). Assert with
// strings.HasSuffix(ext.EntryFile, "mypkg/src/real.ts"), NOT with ==. This mirrors
// TestClassifyDirIndexTS which asserts HasSuffix("git-checkpoint/index.ts").

// GOTCHA — relTag for a package is the DIR (no .ts/.js strip). For a package at
// root/mypkg, RelTag == "mypkg" (via relTagForDir). It is NOT "mypkg/src/real".
// The fragment-on-the-bare-filename is the BUG signature; the e2e test asserts its ABSENCE.

// CRITICAL — white-box tests. discover_test.go is `package discover` (not discover_test),
// so it can call the unexported classifyDir, walkClassified, writeFile, writePackageJSON,
// relTags directly. Do NOT add a new test package or rename the file.

// GOTCHA — no t.Parallel() needed here. These tests use t.TempDir() (per-test isolated),
// no process-global state (no env vars, no cwd). t.Parallel() is optional and fine but
// not required; match the style of the neighboring TestClassifyDir* tests (they omit it).
```

## Implementation Blueprint

### Data models and structure

None added. This subtask reuses `Extension` (extension.go:156), `PackageJSON`
(extension.go:166), and `BuildExtension` (extension.go:289) verbatim. No structs, no new
types, no field changes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/discover/discover.go — classifyDir case (a) body
  FILE: /home/dustin/projects/weave/internal/discover/discover.go
  LOCATE: inside `func classifyDir`, the block:
          `if hasPkg && perr == nil { entries := toStringSlice(pkg.Pi.Extensions);
           if len(entries) > 0 { ... } }`
  REPLACE the body of `if len(entries) > 0 { ... }` (the `entryFile := filepath.Join(path, entries[0])`
          through the fall-through comment) with the loop form:
            var entryFile string
            for _, entry := range entries {
                if f := filepath.Join(path, entry); fileExists(f) {
                    entryFile = f
                    break
                }
            }
            if entryFile != "" {
                relTag := relTagForDir(root, path)
                jsdoc := ExtractJSDoc(entryFile)
                e := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
                return &e, true, false // shouldDescend=false (load-bearing)
            }
            // no existing entry among ANY declared pi.extensions → fall through to b/c/d/e.
  PRESERVE: the `if hasPkg && perr == nil {` guard line, the `entries := toStringSlice(...)`
            line, the `if len(entries) > 0 {` line, and the BuildExtension arg order
            (path, entryFile, relTag, "package", pkg, hasPkg, jsdoc).
  NAMING: loop var `entry` (avoids shadowing confusion with the `e :=` Extension). Do NOT
          reuse `e` as the loop var.
  DO NOT TOUCH: cases (b), (c), (d), (e); the ParsePackageJSON call; classifyFile; relTagForDir;
            fileExists; brokenPackageEntryFile.

Task 2: EDIT internal/discover/discover.go — doc comment (Mode A, rides with Task 1)
  FILE: same file.
  - The case-(a) doc comment in the classifyDir doc block (~lines 96-100) already says
    "EntryFile = first existing pi.extensions entry" — this is CORRECT; verify it still
    reads accurately after the edit (no change expected).
  - The fall-through comment is updated as part of Task 1's replacement (now reads
    "no existing entry among ANY declared pi.extensions → fall through to b/c/d/e.").

Task 3: EDIT internal/discover/discover_test.go — add TestClassifyDirPackageMultiEntryFirstMissing
  FILE: /home/dustin/projects/weave/internal/discover/discover_test.go
  PLACEMENT: directly AFTER TestClassifyDirPackageNonExistentEntry (it extends that case
             from single-entry-missing to multi-entry-first-missing).
  BODY (follow the idiom of TestClassifyDirPackage + the HasSuffix style of TestClassifyDirIndexTS):
    func TestClassifyDirPackageMultiEntryFirstMissing(t *testing.T) {
        root := t.TempDir()
        dir := filepath.Join(root, "mypkg")
        writePackageJSON(t, dir, `{
          "name": "mypkg",
          "description": "My package",
          "pi": { "extensions": ["./src/MISSING.ts", "./src/real.ts"] }
        }`)
        writeFile(t, filepath.Join(dir, "src"), "real.ts", "/** real entry. */\n")
        // NOTE: ./src/MISSING.ts does NOT exist; ./src/real.ts DOES.

        ext, isExt, descend := classifyDir(root, dir)
        if !isExt {
            t.Fatal("isExt=false; want true (first entry missing but a later entry exists)")
        }
        if descend {
            t.Error("descend=true; want false (load-bearing: recognized package is NOT descended)")
        }
        if ext == nil {
            t.Fatal("ext=nil; want non-nil")
        }
        if ext.Kind != "package" {
            t.Errorf("Kind=%q; want package", ext.Kind)
        }
        if !strings.HasSuffix(ext.EntryFile, "mypkg/src/real.ts") {
            t.Errorf("EntryFile=%q; want suffix 'mypkg/src/real.ts' (FIRST EXISTING entry)", ext.EntryFile)
        }
        if ext.EntryFile == filepath.Join(dir, "src", "MISSING.ts") {
            t.Errorf("EntryFile selected the MISSING entry; want the first EXISTING one")
        }
        if ext.RelTag != "mypkg" {
            t.Errorf("RelTag=%q; want mypkg (dir, no .ts strip)", ext.RelTag)
        }
        if ext.Name != "mypkg" {
            t.Errorf("Name=%q; want mypkg (from package.json)", ext.Name)
        }
        if ext.Description != "My package" {
            t.Errorf("Description=%q; want 'My package' (package.json beats JSDoc)", ext.Description)
        }
        if !ext.HasPackageJSON {
            t.Error("HasPackageJSON=false; want true")
        }
    }
  ASSERTS: isExt=true, descend=false, Kind="package", EntryFile ends with src/real.ts
           (NOT the missing one), RelTag="mypkg" (not the fragmented filename),
           Name/Description from package.json, HasPackageJSON=true.

Task 4: EDIT internal/discover/discover_test.go — add TestClassifyDirPackageMultiEntryAllMissing
  FILE: same file.
  PLACEMENT: directly AFTER the test from Task 3.
  BODY (extends TestClassifyDirPackageNonExistentEntry to the multi-entry all-missing case):
    func TestClassifyDirPackageMultiEntryAllMissing(t *testing.T) {
        root := t.TempDir()
        dir := filepath.Join(root, "ghostpkg")
        writePackageJSON(t, dir, `{ "pi": { "extensions": ["./a.ts", "./b.ts"] } }`)
        // NOTE: neither ./a.ts nor ./b.ts exists; no index.ts/index.js.

        ext, isExt, descend := classifyDir(root, dir)
        if isExt {
            t.Error("isExt=true; want false (no declared entry exists → not a package)")
        }
        if ext != nil {
            t.Errorf("ext=%v; want nil (fell through, nothing recognized)", ext)
        }
        if !descend {
            t.Error("descend=false; want true (falls through to plain category dir)")
        }
    }
  ASSERTS: isExt=false, ext=nil, descend=true — same contract as the single-entry
           all-missing case, now proven for multi-entry. This is the "fall through to
           b/c/d/e" path that Bug 3 (P1.M3.T2.S1) will later turn into a check ERROR.

Task 5: EDIT internal/discover/discover_test.go — add end-to-end regression test via walkClassified
  FILE: same file.
  PLACEMENT: directly AFTER the test from Task 4 (or alongside the other walkClassified
             tests further down the file — either is fine; keep it near the classifyDir
             package tests for discoverability).
  BODY (the regression-of-the-regression: the package must resolve as ONE entry, NOT
        fragment into stray single-file extensions):
    func TestWalkClassifiedPackageMultiEntryResolvesAsOne(t *testing.T) {
        root := t.TempDir()
        dir := filepath.Join(root, "mypkg")
        writePackageJSON(t, dir, `{
          "name": "mypkg",
          "description": "My package",
          "pi": { "extensions": ["./src/MISSING.ts", "./src/real.ts"] }
        }`)
        writeFile(t, filepath.Join(dir, "src"), "real.ts", "/** real entry. */\n")

        exts := walkClassified(root)
        tags := relTags(exts)

        // Exactly ONE extension, tagged with the package dir name (not the entry filename).
        if len(exts) != 1 {
            t.Fatalf("len(exts)=%d; want 1 (package must resolve as ONE entry, not fragment). tags=%v",
                len(exts), tags)
        }
        if exts[0].RelTag != "mypkg" {
            t.Errorf("RelTag=%q; want 'mypkg' (the package dir, not the entry filename)", exts[0].RelTag)
        }
        // Guard against the bug's signature: a fragmented stray single-file entry.
        for _, bad := range []string{"mypkg/src/real", "src/real", "real"} {
            for _, tag := range tags {
                if tag == bad {
                    t.Errorf("found fragmented stray tag %q (bug signature); tags=%v", bad, tags)
                }
            }
        }
        if exts[0].Kind != "package" {
            t.Errorf("Kind=%q; want package", exts[0].Kind)
        }
    }
  ASSERTS: exactly 1 extension; RelTag="mypkg"; NO fragmented stray tag like
           "mypkg/src/real"; Kind="package". This is the test that would have caught the
           original bug — it exercises the real SkipDir recursion rule via walkClassified.
  NOTE: walkClassified and relTags already exist in discover_test.go (no new helpers).

Task 6: VALIDATE
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -run 'TestClassifyDir|TestWalkClassified' -v -count=1
  - EXPECT: all PASS (old + new).
  - RUN: cd /home/dustin/projects/weave && go test ./internal/discover/ -count=1
  - EXPECT: ok (no regressions across the whole discover package).
  - RUN: cd /home/dustin/projects/weave && go vet ./internal/discover/
  - EXPECT: clean (no output, exit 0).
  - RUN: cd /home/dustin/projects/weave && go build ./...
  - EXPECT: exit 0.
  - OPTIONAL post-implementation manual confirmation (the bug PRD's repro, NOT a unit test):
        go build -o /tmp/weave .
        rm -rf /tmp/bug1 && mkdir -p /tmp/bug1/mypkg/src
        printf '{"name":"mypkg","description":"My package","pi":{"extensions":["./src/MISSING.ts","./src/real.ts"]}}\n' > /tmp/bug1/mypkg/package.json
        printf '/** real entry */\nexport default function(){}\n' > /tmp/bug1/mypkg/src/real.ts
        weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave mypkg    # → prints /tmp/bug1/mypkg (rc=0)
        weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave -f mypkg # → /tmp/bug1/mypkg/src/real.ts
        weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave --list   # → shows package "mypkg"
```

### Implementation Patterns & Key Details

```go
// PATTERN: the loop-and-capture (mirrors extdir.hasPiExtensions, extdir.go:393-397).
// hasPiExtensions returns a bool; classifyDir must capture the first existing PATH.
var entryFile string
for _, entry := range entries {
    if f := filepath.Join(path, entry); fileExists(f) {
        entryFile = f
        break   // first existing wins (PRD §7.1 "the first existing pi.extensions entry")
    }
}
if entryFile != "" {
    // ... BuildExtension(..., "package", ...); return &e, true, false
}
// fall through to b/c/d/e ONLY when entryFile is still "" (no entry exists)

// CRITICAL: the return shape (ext, true, false) is load-bearing. `false` for
// shouldDescend is what makes Index/walkClassified return filepath.SkipDir and prune
// the subtree — preventing double-counting of the entry file as a stray extension.
// This is the exact mechanism the bug broke (it returned descend=true).

// PATTERN: test assertion on EntryFile uses HasSuffix (abs path is machine-specific),
// mirroring TestClassifyDirIndexTS:
//   if !strings.HasSuffix(ext.EntryFile, "mypkg/src/real.ts") { t.Errorf(...) }
```

### Integration Points

```yaml
DATABASE:
  - none. Pure stdlib file/JSON code.

CONFIG:
  - none. No config, env vars, or settings touched.

ROUTES / API:
  - none. weave is a CLI; this is an internal discovery function.

DISCOVERY SUBSYSTEM (the integration surface):
  - classifyDir is called by Index (index.go WalkDir callback) and walkClassified
    (discover_test.go). Its return tuple drives the SkipDir recursion rule.
  - Downstream consumers of the resulting Extension slice: main.go (all modes),
    resolve (P1.M3 / existing), search (existing), check (existing), ui (existing).
    None of their call sites change — the fix makes classifyDir CONFORM to the contract
    they already assume (one Extension per package, correct RelTag/EntryFile/Kind).

CHECK SUBSYSTEM (cross-task dependency, NOT this task):
  - Bug 3 / P1.M3.T2.S1 (check all-missing → ERROR) DEPENDS on this fix. Once this lands,
    an all-missing pi.extensions package falls through to (e) cleanly, and check's later
    fix can detect "package.json with pi.extensions but all entries missing" and emit the
    §9 ERROR. Do NOT implement that detection here.

EXTDIR SUBSYSTEM (consistency, no code change here):
  - extdir.hasPiExtensions is already correct (iterates all entries). After this fix,
    discover.classifyDir AGREES with it, resolving the init-vs-discovery inconsistency
    noted as an aggravating factor in the bug PRD. No extdir edit needed.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave
# Go uses gofmt + go vet (this is a Go repo, NOT Python — ruff/mypy do not apply).
gofmt -l internal/discover/discover.go internal/discover/discover_test.go
# EXPECT: no output (no files need formatting). If a file is listed, run:
#   gofmt -w internal/discover/discover.go internal/discover/discover_test.go

go vet ./internal/discover/
# EXPECT: clean (no output, exit 0).
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# The new tests in isolation:
go test ./internal/discover/ -run 'TestClassifyDirPackageMultiEntryFirstMissing' -v -count=1
go test ./internal/discover/ -run 'TestClassifyDirPackageMultiEntryAllMissing'  -v -count=1
go test ./internal/discover/ -run 'TestWalkClassifiedPackageMultiEntryResolvesAsOne' -v -count=1
# EXPECT: each PASS.

# The full classifyDir + walk suite (new + existing, to catch regressions):
go test ./internal/discover/ -run 'TestClassifyDir|TestWalkClassified' -v -count=1
# EXPECT: all PASS.

# The whole discover package (catches any indirect regression in Index/classifyFile/etc.):
go test ./internal/discover/ -count=1
# EXPECT: ok github.com/dabstractor/weave/internal/discover
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Build the whole module (the fix must not break compilation anywhere):
go build ./...
# EXPECT: exit 0 (no output, or only the pre-existing "matched no packages"-style note).

# Optionally build the CLI binary and run the bug PRD's repro end-to-end:
go build -o /tmp/weave .
rm -rf /tmp/bug1 && mkdir -p /tmp/bug1/mypkg/src
printf '{"name":"mypkg","description":"My package","pi":{"extensions":["./src/MISSING.ts","./src/real.ts"]}}\n' > /tmp/bug1/mypkg/package.json
printf '/** real entry */\nexport default function(){}\n' > /tmp/bug1/mypkg/src/real.ts

weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave mypkg     # EXPECT: prints /tmp/bug1/mypkg (rc=0)
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave -f mypkg  # EXPECT: /tmp/bug1/mypkg/src/real.ts
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave --list    # EXPECT: shows package "mypkg" with its metadata
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave check      # EXPECT: no spurious "duplicate relTag" ERROR

# Expected: all four behave per PRD §6/§7.1/§9. This is a manual confirmation; the
# deterministic regression coverage lives in the Level 2 unit/e2e tests above.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None beyond Level 3. No MCP, no Docker, no DB, no web UI, no performance test.
# The domain-specific validation IS the walkClassified end-to-end test (Task 5),
# which proves the package resolves as one entry through the real SkipDir walk —
# the exact behavior that was broken.

# Optional: run the full repo test suite to confirm no cross-package regression
# (resolve/search/check/ui consume the Extension slice this fix produces):
cd /home/dustin/projects/weave && go test ./... -count=1
# EXPECT: all packages ok.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l` lists no files; `go vet ./internal/discover/` clean.
- [ ] Level 2 passed: the 3 new tests pass; all existing `TestClassifyDir*`/`TestWalkClassified*` pass.
- [ ] Level 2 passed: `go test ./internal/discover/ -count=1` → ok.
- [ ] Level 3 passed: `go build ./...` exits 0.
- [ ] (Optional) Level 3 manual repro confirms `weave mypkg` / `-f` / `--list` / `check`.
- [ ] (Optional) Level 4: `go test ./... -count=1` → all packages ok.

### Feature Validation

- [ ] A package with `pi.extensions: ["./missing.ts","./real.ts"]` (first missing, later
      present) classifies as `(ext, true, false)` with `EntryFile` = `.../real.ts`.
- [ ] A package with ALL entries missing falls through to `(nil, false, true)`.
- [ ] `ext.RelTag` is the dir name (`"mypkg"`), NOT a fragmented filename.
- [ ] `ext.Kind == "package"`; metadata (Name/Description) drawn from package.json.
- [ ] The package resolves as exactly ONE entry via `walkClassified` (no stray fragment).
- [ ] Cases (b)/(c)/(d)/(e) of `classifyDir` are unchanged.

### Code Quality Validation

- [ ] Follows existing conventions: mirrors `extdir.hasPiExtensions` iteration; reuses
      `BuildExtension`/`relTagForDir`/`ExtractJSDoc`/`toStringSlice` unchanged.
- [ ] Test style matches neighbors: `t.TempDir()`, `writeFile`/`writePackageJSON`,
      `strings.HasSuffix` for abs paths, `t.Fatal` for guards / `t.Errorf` for fields.
- [ ] Loop variable named `entry` (not `e`) to avoid shadowing the `e :=` Extension.
- [ ] Anti-patterns avoided: no new patterns invented (the loop IS the extdir pattern);
      no over-catch of errors; the return shape is preserved.
- [ ] Scope respected: only `discover.go` (case a body + 2 comments) and
      `discover_test.go` (+3 tests) change. No extdir/check/index/jsdoc/extension edits.

### Documentation & Deployment

- [ ] Mode A doc ride-along: fall-through comment updated to "no existing entry among ANY
      declared pi.extensions → fall through to b/c/d/e." (inline in discover.go).
- [ ] The case-(a) doc block still accurately says "EntryFile = first existing
      pi.extensions entry" (verified, no change needed).
- [ ] No new env vars, no config change, no README change (that is P1.M4.T1.S1).
- [ ] Code is self-documenting: the loop + break clearly express "first existing wins".

---

## Anti-Patterns to Avoid

- ❌ Don't change cases (b), (c), (d), or (e) of `classifyDir`. Only the body of
  `if len(entries) > 0 { ... }` in case (a) changes.
- ❌ Don't change `classifyFile`, `Index`, `relTagForDir`, `fileExists`,
  `brokenPackageEntryFile`, `BuildExtension`, `toStringSlice`, or `ExtractJSDoc`.
- ❌ Don't edit `extdir.hasPiExtensions` — it's the REFERENCE (already correct).
- ❌ Don't fix Bug 3 (check WARN→ERROR) here — that's P1.M3.T2.S1 and depends on this fix.
- ❌ Don't change the return shape `(ext, true, false)`. `shouldDescend=false` is
  load-bearing — it drives `filepath.SkipDir` in Index/walkClassified.
- ❌ Don't drop the `if hasPkg && perr == nil {` or `if len(entries) > 0 {` guards. Broken
  JSON belongs to case (b); an empty entries slice falls through to c/d/e.
- ❌ Don't name the loop variable `e` and then `e := BuildExtension(...)` in the same
  block — it compiles but is confusing. Use `entry` for the loop var.
- ❌ Don't assert `ext.EntryFile == <abs path>` in tests — the path comes from
  `t.TempDir()` and varies. Use `strings.HasSuffix`.
- ❌ Don't add a new test package or rename `discover_test.go`. Tests are white-box
  (`package discover`) so they can call the unexported helpers directly.
- ❌ Don't invent a new helper for the e2e test — `walkClassified` and `relTags` already
  exist in `discover_test.go` and are the right tools.
- ❌ Don't commit anything or edit `tasks.json`/`PRD.md` — those are orchestrator-owned.
