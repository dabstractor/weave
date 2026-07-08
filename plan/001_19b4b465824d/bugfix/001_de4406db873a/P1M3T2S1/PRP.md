# PRP — P1.M3.T2.S1: `allMissingPiExtensions` detection + ERROR in `appendEmptyFolderFindings`

## Goal

**Feature Goal**: Fix Bug 5 (Issue 5, Minor) — `weave check` currently reports a
top-level dir whose `package.json` declares `pi.extensions` pointing at ALL-missing
entry files as a benign `WARN "empty category folder: <name>"` (exit 0), hiding the
real problem (broken `pi.extensions` references) at the wrong severity. PRD §9
requires an ERROR ("ERROR: a dir/package entry's `entryFile` does not exist on
disk ... catches hand-edited `package.json` `pi.extensions` pointing at a missing
file"). After the fix, that case surfaces a clear, actionable ERROR naming the
missing entry paths (exit 1 via `main`), while every other empty-folder case (no
package.json, unparseable JSON, empty `pi.extensions`, ≥1 entry exists) keeps the
existing WARN unchanged.

**Deliverable**: TWO localized additions to ONE file + ONE test, plus a doc-comment
update:
- `internal/check/check.go` — add `func fileExists(path string) bool` (local 2-line
  `os.Stat` helper, matching the discover/extdir precedent); add
  `func allMissingPiExtensions(dir string) []string`; insert a 9-line
  all-missing-ERROR branch in `appendEmptyFolderFindings` BEFORE the existing WARN
  append (with `continue` to skip the WARN); update the
  `appendEmptyFolderFindings` doc comment to note the pi.extensions all-missing
  ERROR check ([Mode A] doc ride-along).
- `internal/check/check_test.go` — add `TestCheckAllMissingPiExtensionsIsError`
  (builds a top-level `badpkg/` with `pi.extensions:["./missing.ts"]` and no entry
  file; asserts `HasErrors()==true`, `Errors>=1`, an ERROR finding whose message
  contains "pi.extensions entry does not exist" with `Level==LevelError`, and that
  no WARN "empty category folder" names badpkg).

NO other files. NO new imports (`os`, `path/filepath`, `strings`, `discover` all
already imported in check.go). NO signature changes. NO new deps. The existing
`TestCheckEmptyCategoryFolder` regression guard is untouched and must still pass.

**Success Definition**:
- `weave check` on a store containing `badpkg/package.json` with
  `{"pi":{"extensions":["./missing.ts"]}}` and no `./missing.ts` → prints an
  `ERROR` line citing `pi.extensions entry does not exist: ./missing.ts`, and
  `Report.HasErrors()==true` (→ `main` exits 1).
- A dir with NO `package.json` still gets `WARN "empty category folder"` (exit 0)
  — `TestCheckEmptyCategoryFolder` passes unchanged.
- A dir with a parseable `package.json` but EMPTY `pi.extensions` still gets the
  WARN (empty pi.extensions is not a broken reference).
- A dir with `pi.extensions` naming ≥1 EXISTING entry never reaches the
  empty-folder path (Bug 1's classifyDir fix classifies it as a package; it is a
  real Extension in the catalog) — `TestCheckNestedExtensionFolderNotFlagged`
  passes unchanged.
- `go build ./...`, `go vet ./...`, `go test ./internal/check/... -v`, and
  `go test -race ./...` all clean.

## User Persona (if applicable)

**Target User**: A developer who hand-edits a package extension's `package.json`
`pi.extensions` array (or renames/moves the entry file without updating the JSON).
Indirectly: anyone running `weave check` as a CI gate (`if weave check; then ...`)
to catch a broken store before it breaks `pi -e "$(weave <tag>)"`.

**Use Case**: A user creates `extensions/summarizer/package.json` with
`"pi": {"extensions": ["./src/index.ts"]}` but forgets to create `src/index.ts`
(or typos it as `./scr/index.ts`). Today `weave check` says `WARN summarizer:
empty category folder: summarizer` and exits 0, so the CI gate passes and the
breakage is discovered later when `pi -e "$(weave summarizer)"` fails to load.
After the fix, `weave check` says `ERROR summarizer: pi.extensions entry does not
exist: ./src/index.ts` and exits 1 — the gate fails fast with an actionable message.

**User Journey**:
1. Store has `extensions/badpkg/package.json` = `{"name":"badpkg","pi":{"extensions":["./missing.ts"]}}`, no `./missing.ts`.
2. `weave check` → `main` → `check.Check(dir, exts)`.
3. `discover.Index(dir)` yields 0 entries for badpkg (all-missing → not a package → plain folder → descend → nothing).
4. `appendEmptyFolderFindings` walks dir's children, hits badpkg, `discover.Index(badpkg)` → 0 entries → `len(sub)==0`.
5. **After fix:** `allMissingPiExtensions(badpkg)` → `["./missing.ts"]` (non-nil) → append ERROR finding + `continue` (WARN skipped).
6. Pass 3 tallies `rep.Errors=1` → `HasErrors()==true` → `main` exits 1.

**Pain Points Addressed**:
- **Silent breakage**: a broken `pi.extensions` is invisible at WARN/exit-0; it surfaces only at runtime in `pi`.
- **Unactionable message**: "empty category folder" misdescribes the problem; "pi.extensions entry does not exist: ./missing.ts" names the exact bad path.
- **Wrong severity**: a broken entry reference is an ERROR (the extension will not load), not a WARN.

## Why

- **PRD §9 contract**: §9 explicitly lists "ERROR: a dir/package entry's
  `entryFile` does not exist on disk ... catches hand-edited `package.json`
  `pi.extensions` pointing at a missing file." The current WARN-at-exit-0 violates
  this for the all-missing case. (The PRD even flags the §7.1/§9 tension — a
  package.json with no existing entry is NOT a package per §7.1, so the §9 ERROR is
  "inherently hard to reach" — and asks for exactly this surfacing.)
- **Bug 1 (P1.M1.T1.S1, COMPLETE) made this fix precise**: before Bug 1,
  `classifyDir` checked only `entries[0]`; after Bug 1 it iterates all entries and
  picks the first existing. The consequence: a package.json with ≥1 existing entry
  is a real package (never reaches the empty-folder path); ONLY the ALL-missing
  case falls through to `appendEmptyFolderFindings`. So the new helper has exactly
  one trigger condition and cannot misfire on a valid package. (See
  `architecture/bug_cascade_map.md` Bug 3 dependency.)
- **Narrow, surgical fix**: the change is ~25 lines (2 helpers + a 9-line branch)
  inside one function of one file. No new types, no signature changes, no catalog
  semantics change — `discover.Index` and `classifyDir` are untouched. The fix
  lives entirely in `check`, where §9 validation belongs.
- **Local `fileExists` matches the codebase precedent**: `discover.fileExists` and
  `extdir.fileExists` are already byte-identical unexported copies (discover.go:16
  comment: "KEEP IN SYNC ... a future shared internal/rules package is out of
  scope"). A third local copy in `check` follows the established pattern; the 2-line
  body makes sharing higher cost than duplicating.
- **Doc ride-along [Mode A]**: the `appendEmptyFolderFindings` doc comment currently
  describes only the WARN path. It must note the all-missing-ERROR check that
  precedes the WARN, so a future maintainer understands why the branch exists.
- **No blast radius / no parallel conflict**: the parallel item P1.M3.T1.S1 edits
  `internal/discover/jsdoc.go` ONLY (a pure string function); this task edits
  `internal/check/check.go` ONLY. Zero file overlap.

## What

### New helper: `fileExists` (`internal/check/check.go`)

```go
// fileExists reports whether a non-directory entry exists at path. It mirrors
// discover.fileExists and extdir.fileExists (both unexported); the check package
// needs its own copy to inspect pi.extensions entries without an import cycle.
// os.Stat follows symlinks, so a symlink to a file counts.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
```

### New helper: `allMissingPiExtensions` (`internal/check/check.go`)

```go
// allMissingPiExtensions reports whether dir has a package.json with a non-empty
// pi.extensions array where NONE of the declared string entries exist on disk.
// It returns the list of missing entry paths (as declared in pi.extensions) when
// all are missing; it returns nil otherwise — including: no package.json, a
// parse error, an empty/absent pi.extensions, any non-string-only pi.extensions,
// or the case where AT LEAST ONE declared entry exists (the not-all-missing case,
// which classifyDir already turned into a real package extension that never
// reaches appendEmptyFolderFindings).
//
// pkg.Pi.Extensions has type discover.lenientAnySlice (an unexported []any); the
// underlying []any is rangeable from this package, and each element is an any
// narrowed to string via comma-ok (non-strings are dropped leniently, matching
// discover.classifyDir's toStringSlice).
func allMissingPiExtensions(dir string) []string {
	pkg, hasPkg, perr := discover.ParsePackageJSON(dir)
	if !hasPkg || perr != nil {
		return nil
	}
	var missing []string
	hasAny := false
	for _, e := range pkg.Pi.Extensions {
		s, ok := e.(string)
		if !ok {
			continue
		}
		hasAny = true
		if !fileExists(filepath.Join(dir, s)) {
			missing = append(missing, s)
		} else {
			return nil // at least one exists → not all-missing
		}
	}
	if !hasAny {
		return nil
	}
	return missing
}
```

### Modified `appendEmptyFolderFindings` — insert the ERROR branch before the WARN

The current `if len(sub) == 0 { ... }` block (check.go ~lines 303–312) appends
the WARN. Insert the all-missing check BEFORE the WARN append; if it fires,
append the ERROR and `continue` (skip the WARN):

```go
if len(sub) == 0 {
	// Before the WARN: if this empty folder has a package.json whose
	// pi.extensions are ALL missing, that is a §9 ERROR (a broken entry
	// reference), not a benign empty folder. Bug 1's classifyDir fix means
	// ONLY the all-missing case reaches here (a package.json with ≥1 existing
	// entry is a real extension in the catalog, never an empty folder).
	if missing := allMissingPiExtensions(child); len(missing) > 0 {
		rep.ByExt = append(rep.ByExt, ExtensionReport{
			Extension: discover.Extension{RelTag: e.Name()},
			Findings: []Finding{{
				Level:   LevelError,
				Message: "pi.extensions entry does not exist: " + strings.Join(missing, ", "),
			}},
		})
		continue // reported as ERROR; skip the empty-folder WARN
	}
	rep.ByExt = append(rep.ByExt, ExtensionReport{
		Extension: discover.Extension{RelTag: e.Name()},
		Findings: []Finding{{
			Level:   LevelWarn,
			Message: "empty category folder: " + e.Name(),
		}},
	})
}
```

### Doc comment update ([Mode A] ride-along)

The `appendEmptyFolderFindings` doc comment (~check.go line 275) currently says it
"appends a WARN for each plain category folder that contains ZERO discoverable
entries." Add a sentence noting the all-missing-ERROR check that precedes the WARN:

> Before emitting the WARN, the function checks whether the empty folder has a
> `package.json` whose `pi.extensions` are ALL missing (a §9 ERROR — a broken
> entry reference — rather than a benign empty folder); if so, it appends an ERROR
> naming the missing entries and skips the WARN.

### Truth table (what reaches which path after the fix)

| top-level child dir state                     | `len(sub)==0`? | `allMissingPiExtensions` | finding   | exit |
|-----------------------------------------------|----------------|--------------------------|-----------|------|
| no package.json, no entries                   | yes            | nil                      | WARN      | 0    |
| package.json unparseable, no entries          | yes            | nil (perr!=nil)          | WARN      | 0*   |
| package.json valid, no `pi` key               | yes            | nil (hasAny=false)       | WARN      | 0    |
| package.json valid, `pi.extensions: []`       | yes            | nil (hasAny=false)       | WARN      | 0    |
| package.json valid, `pi.extensions` all-missing| yes            | non-nil                  | **ERROR** | **1**|
| package.json valid, `pi.extensions` ≥1 exists  | no (classified as package) | n/a          | (localFindings) | per-entry |

*Unparseable JSON reaching the empty-folder path is rare (would need a parse
error AND no index.ts AND all pi.extensions missing); the `perr != nil → nil`
guard keeps it out of this check; if the dir WAS classified as a package, the
unparseable ERROR surfaces via `localFindings` instead.

### Success Criteria

- [ ] `fileExists(path string) bool` exists in check.go (local copy; 2-line body).
- [ ] `allMissingPiExtensions(dir string) []string` exists in check.go (the exact
      logic above: returns nil for no-pkg/parse-err/empty/≥1-exists, non-nil list
      only when ALL string entries are missing).
- [ ] `appendEmptyFolderFindings` calls `allMissingPiExtensions(child)` inside the
      `len(sub)==0` block BEFORE the WARN append, appends the ERROR + `continue`
      when it returns non-nil.
- [ ] The ERROR message is exactly
      `"pi.extensions entry does not exist: " + strings.Join(missing, ", ")`.
- [ ] The ERROR finding's `Level == LevelError` and its synthetic ExtensionReport's
      `RelTag == e.Name()` (the folder basename, matching the WARN path).
- [ ] The WARN path is UNCHANGED for all other cases (no package.json, parse error,
      empty pi.extensions).
- [ ] `appendEmptyFolderFindings` doc comment notes the all-missing-ERROR check.
- [ ] `TestCheckAllMissingPiExtensionsIsError` passes (ERROR, HasErrors, no WARN
      for badpkg).
- [ ] `TestCheckEmptyCategoryFolder` still passes (no-package.json → WARN).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the exact file, the exact function to
modify (with current line numbers), the exact insertion point (the
`if len(sub) == 0` block), the byte-for-byte helper code (from the item CONTRACT
and `architecture/fix_design.md` Bug 5), the type-safety note
(`lenientAnySlice`=`[]any` is rangeable cross-package), the import status
(nothing new needed), the test to add (with the exact fixture pattern from
`TestCheckUnparseablePackageJSON`), the regression guard
(`TestCheckEmptyCategoryFolder` must still pass), and the validation commands are
all specified. No guessing.

### Documentation & References

```yaml
# MUST READ — the file to edit
- file: internal/check/check.go
  why: Contains appendEmptyFolderFindings (lines 287-312) — the function to modify;
       the existing WARN path; the imports (os/filepath/strings/discover all present);
       and the doc comment (~line 275) to update.
  pattern: the WARN append is a synthetic ExtensionReport{Extension: {RelTag: e.Name()}, Findings: [{Level, Message}]}.
           Mirror that shape EXACTLY for the ERROR (same Extension, LevelError).
  gotcha: e.Name() is the folder BASENAME (os.DirEntry.Name), NOT the full path.
          RelTag is set to the basename so main renders it uniformly. Use e.Name()
          for BOTH the ERROR and WARN RelTag (the WARN already does).

# MUST READ — the test file to extend + the patterns to mirror
- file: internal/check/check_test.go
  why: Contains the fixture helpers (mkFileExt, mkPackageExt, findExt, finding),
       TestCheckEmptyCategoryFolder (the regression guard — MUST still pass), and
       TestCheckUnparseablePackageJSON (the DIRECT pattern to copy for the new test:
       build a broken package dir by hand via os.MkdirAll/WriteFile, NOT via mkPackageExt
       which writes a valid entry).
  pattern: finding(rep, substr) returns (Finding, bool); assert f.Level == LevelError;
           rep.HasErrors() bool; rep.Errors int. Tests use t.TempDir() + os.WriteFile.
  gotcha: mkPackageExt writes a VALID src/index.ts — do NOT use it for the all-missing
          test (the test needs the entry file ABSENT). Write package.json by hand.

# The consumed type — confirms pkg.Pi.Extensions is rangeable cross-package
- file: internal/discover/extension.go
  why: Defines lenientAnySlice (type []any, line 38), piBlock{Extensions lenientAnySlice}
       (line 208), PackageJSON{Pi piBlock} (line 199), and ParsePackageJSON (line 241).
       Confirms the check package can range pkg.Pi.Extensions and call discover.ParsePackageJSON.
  pattern: ParsePackageJSON(dir) returns (PackageJSON, hasPkg bool, err error). hasPkg=false
           for no/failed-read package.json; hasPkg=true && err!=nil for unparseable JSON;
           hasPkg=true && err==nil for success.
  gotcha: lenientAnySlice is UNEXPORTED but its underlying []any is a builtin — ranging
          it works. Each element is `any`; narrow with `s, ok := e.(string)`.

# The existing fileExists precedent (the dup to mirror)
- file: internal/discover/discover.go
  why: discover.fileExists (line 20) is the exact 2-line body to copy into check.go.
       Its doc comment (line 16) says it "Mirrors extdir.fileExists — KEEP IN SYNC";
       a third copy in check follows the same intentional-duplication pattern.
  pattern: `info, err := os.Stat(path); return err == nil && !info.IsDir()`.
  gotcha: discover.fileExists is UNEXPORTED — the check package CANNOT import it.
          A local copy is the established pattern (extdir also has its own copy).

# The bug report (authoritative expected/actual/repro)
- docfile: PRD.md   # the bugfix PRD, h2.3/h3.4
  section: "Issue 5: weave check reports a package with a missing pi.extensions entry as an 'empty category folder' WARN, not the §9 ERROR"
  why: confirms expected (ERROR citing the missing pi.extensions path, exit 1),
       actual (WARN "empty category folder", exit 0), the repro, and the §7.1/§9 tension.
  critical: the §9 quote "ERROR: a dir/package entry's entryFile does not exist on disk
            ... catches hand-edited package.json pi.extensions pointing at a missing file"
            is the authority for the ERROR severity.

# The settled fix design (byte-for-byte helper code)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md
  section: "Bug 5 (Minor): check surfaces ERROR for all-missing pi.extensions"
  why: gives the exact allMissingPiExtensions body, the exact appendEmptyFolderFindings
       branch, the fileExists note, and the Bug 1 interaction.
  gotcha: trust the item CONTRACT + this fix_design section; they agree verbatim.

# The cascade dependency (why Bug 1 must be complete first — it IS)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/bug_cascade_map.md
  section: "Bug 3 — check reports WARN for all-missing pi.extensions" (this is Bug 5's
           internal cascade name) + the dependency graph (Bug 1 → Bug 3/Issue5).
  why: confirms Bug 1 (P1.M1.T1.S1, COMPLETE) makes classifyDir iterate all entries,
       so ONLY the all-missing case reaches appendEmptyFolderFindings. This is what
       makes allMissingPiExtensions's single trigger condition correct.

# PRD spec (the §9 rule this enforces)
- docfile: PRD.md
  section: §9 (Validation — weave check) — the ERROR rule for a missing entryFile.
  why: the authority for ERROR severity (not WARN) and exit 1.

# Parallel item (NO CONFLICT — different file)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M3T1S1/PRP.md
  why: confirms the parallel item edits internal/discover/jsdoc.go ONLY (ExtractJSDoc
       single-line branch). ZERO overlap with this internal/check/check.go edit.
  critical: do NOT touch jsdoc.go; do NOT touch discover.go/index.go/extension.go.
```

### Current Codebase tree (relevant subset)

```bash
internal/check/
├── check.go          # MODIFY: +fileExists, +allMissingPiExtensions, +ERROR branch, +doc comment
└── check_test.go     # MODIFY: +TestCheckAllMissingPiExtensionsIsError
internal/discover/
├── extension.go      # PackageJSON, piBlock, lenientAnySlice, ParsePackageJSON — CONSUMED (read-only)
├── discover.go       # discover.fileExists (the dup to mirror) — NOT modified
├── jsdoc.go          # touched by parallel P1.M3.T1.S1 ONLY — NOT this task
└── index.go          # NOT modified
```

### Desired Codebase tree with files to be added/changed

```bash
internal/check/
├── check.go          # MODIFIED — +2 helpers, +ERROR branch in appendEmptyFolderFindings, +doc comment
└── check_test.go     # MODIFIED — +1 test (TestCheckAllMissingPiExtensionsIsError)
# NO new files. NO other packages touched.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: e.Name() is the folder BASENAME (os.DirEntry.Name), NOT filepath.Base(child)
// and NOT the full path. The existing WARN uses e.Name() for RelTag; the ERROR must too,
// so main renders them uniformly. Do NOT use filepath.Base(child) or child itself.

// CRITICAL: the ERROR branch must come BEFORE the WARN append AND end with `continue`.
// Without `continue`, the WARN append runs too and you get BOTH an ERROR and a WARN for
// the same dir (double-counting, wrong tallies). The `continue` skips the rest of the
// loop body for this child.

// CRITICAL: allMissingPiExtensions returns nil for "≥1 entry exists". This is the
// self-contained guard: even though Bug 1 means such a dir never reaches the empty-folder
// path (it is a real package in the catalog), the helper's early `return nil` on the first
// existing entry makes it correct REGARDLESS of how it is called. Do NOT remove it.

// CRITICAL: the `hasAny` flag distinguishes "empty pi.extensions" (nil → WARN) from
// "all entries missing" (non-nil → ERROR). An empty pi.extensions is NOT a broken
// reference (nothing was declared), so it must fall through to the WARN. Dropping hasAny
// would make `{"pi":{"extensions":[]}}` return a non-nil empty `missing` slice with len 0
// — which the `len(missing) > 0` caller guard already handles, BUT keeping hasAny makes
// the intent explicit and the return value clean (nil for empty/absent).

// CRITICAL: lenientAnySlice is UNEXPORTED, but `pkg.Pi.Extensions` is still rangeable
// because its underlying type is []any (a builtin). The item CONTRACT confirms this via
// a compilation test. Do NOT try to export lenientAnySlice or add a discover helper.

// GOTCHA: discover.fileExists and extdir.fileExists are UNEXPORTED duplicates. The check
// package needs its OWN local fileExists (the established pattern). Do NOT try to import
// one of them or create a shared internal/rules package (out of scope per discover.go:16).

// GOTCHA: os.Stat follows symlinks. A symlink to a real .ts file counts as "exists"
// (matching pi's fs.existsSync and discover.classifyDir's fileExists). This is correct
// for check — a symlinked entry that resolves is not broken.

// GOTCHA: ParsePackageJSON collapses ALL read errors (incl. fs.ErrNotExist AND perm
// errors) to hasPkg=false (extension.go parsePackageJSON doc). So `!hasPkg` covers both
// "no package.json" and "permission denied reading package.json" — both correctly fall
// through to the WARN (not this check's concern).

// GOTCHA: the new test must NOT use mkPackageExt (it writes a valid src/index.ts). Write
// the broken package dir by hand with os.MkdirAll + os.WriteFile, mirroring
// TestCheckUnparseablePackageJSON's direct-file pattern.
```

## Implementation Blueprint

### Data models and structure

No new data models. This task reuses `Report`, `ExtensionReport`, `Finding`,
`Severity`/`LevelError` (all already defined in check.go), and consumes
`discover.PackageJSON`/`ParsePackageJSON` (read-only). The two new helpers are
pure functions; the `appendEmptyFolderFindings` edit is a branch insertion.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the regression test FIRST (TDD)
  - FILE: internal/check/check_test.go
  - ADD func TestCheckAllMissingPiExtensionsIsError(t *testing.T) after
    TestCheckEmptyCategoryFolder (or after TestCheckNestedExtensionFolderNotFlagged,
    grouping it with the empty-folder tests).
  - FIXTURE (mirror TestCheckUnparseablePackageJSON's direct-write pattern):
      root := t.TempDir()
      // A real extension at the top level so the store is non-empty (matches
      // TestCheckEmptyCategoryFolder's shape; optional but realistic).
      ext := mkFileExt(t, root, "gate", "/** d */\nexport function g() {}\n")
      // The broken package dir: package.json with an all-missing pi.extensions.
      dir := filepath.Join(root, "badpkg")
      os.MkdirAll(dir, 0o755)
      os.WriteFile(filepath.Join(dir, "package.json"),
          []byte(`{"name":"badpkg","description":"d","pi":{"extensions":["./missing.ts"]}}`), 0o644)
      // NOTE: deliberately do NOT create ./missing.ts — that is the bug condition.
  - CALL: rep := Check(root, []discover.Extension{ext})
    (exts is the catalog main would build; badpkg is NOT in exts because discover.Index
    yields 0 entries for it — appendEmptyFolderFindings walks root's children directly.)
  - ASSERT:
      - rep.HasErrors() == true  (HasErrors means Errors>0 → main exits 1)
      - rep.Errors >= 1
      - f, ok := finding(rep, "pi.extensions entry does not exist"); ok && f.Level == LevelError
      - strings.Contains(f.Message, "./missing.ts")  (the missing path is named)
      - Now WARN regression: finding(rep, "empty category folder") that names badpkg
        must NOT exist. Use a helper or loop: no Finding whose Message contains BOTH
        "empty category folder" AND "badpkg". (A plain `finding(rep, "empty category folder")`
        is NOT specific enough if the store has other empty folders — but this fixture
        has only `gate` + `badpkg`, so it IS specific. For robustness, iterate and check
        no empty-folder finding mentions badpkg.)
  - RUN (expect FAIL until Task 2 lands — the bug currently emits WARN, not ERROR):
      go test ./internal/check/ -run TestCheckAllMissingPiExtensionsIsError -v

Task 2: ADD fileExists helper to check.go
  - FILE: internal/check/check.go
  - PLACEMENT: near the other small helpers (e.g. after nodeModulesPresent, ~line 245,
    or after packageDir). Group it with the fs helpers.
  - BODY: the 2-line os.Stat + !IsDir pattern (see the What section), with the doc
    comment noting it mirrors discover.fileExists/extdir.fileExists.
  - FILES TOUCHED: 1 (check.go).

Task 3: ADD allMissingPiExtensions helper to check.go
  - FILE: internal/check/check.go
  - PLACEMENT: immediately BEFORE appendEmptyFolderFindings (so the helper is defined
    next to its sole caller; matches the codebase's helper-near-caller convention).
  - BODY: the exact logic in the What section (ParsePackageJSON guard → range
    pkg.Pi.Extensions → comma-ok string narrow → hasAny → fileExists → return nil on
    first existing → return missing if hasAny).
  - FILES TOUCHED: 1 (check.go).

Task 4: MODIFY appendEmptyFolderFindings — insert the ERROR branch
  - FILE: internal/check/check.go
  - FIND: the `if len(sub) == 0 {` block (~line 303). Its current body is the single
    WARN append (~lines 304-311).
  - INSERT (as the FIRST statement inside the `if len(sub) == 0` block, before the
    WARN append): the allMissingPiExtensions(child) check → append ERROR → continue
    (see the What section for the exact code).
  - PRESERVE: the existing WARN append (it now runs only when allMissingPiExtensions
    returns nil — the `continue` skips it otherwise).
  - PRESERVE: everything else in appendEmptyFolderFindings (the os.ReadDir guard, the
    !e.IsDir() continue, the child/filepath.Join, the discover.Index(child), the ierr
    continue).
  - FILES TOUCHED: 1 (check.go).

Task 5: UPDATE the appendEmptyFolderFindings doc comment ([Mode A] ride-along)
  - FILE: internal/check/check.go
  - FIND: the appendEmptyFolderFindings doc comment (~line 275). It currently describes
    only the WARN path.
  - ADD: a sentence noting the all-missing-ERROR check that precedes the WARN (see the
    What section for the exact wording). Reference PRD §9 and Issue 5.
  - FILES TOUCHED: 1 (check.go).

Task 6: VALIDATE (whole-repo gate)
  - RUN: go build ./... ; go vet ./... ; go test ./internal/check/... -v ;
         go test -race ./...
  - EXPECT: all clean / all pass. The new test passes (ERROR, HasErrors); the existing
    TestCheckEmptyCategoryFolder and TestCheckNestedExtensionFolderNotFlagged still pass
    (WARN for no-package.json; not-flagged for resolvable dir).
  - NOTE: the parallel P1.M3.T1.S1 may have landed a jsdoc.go change; that is independent
    and must also pass. If a failure appears in discover/jsdoc, it is NOT this task's
    regression.
```

### Implementation Patterns & Key Details

```go
// The ERROR branch inside appendEmptyFolderFindings (the ONLY structural change to
// that function). It is the first statement in the `if len(sub) == 0 {` block:

if missing := allMissingPiExtensions(child); len(missing) > 0 {
	rep.ByExt = append(rep.ByExt, ExtensionReport{
		Extension: discover.Extension{RelTag: e.Name()}, // basename, matching the WARN
		Findings: []Finding{{
			Level:   LevelError,
			Message: "pi.extensions entry does not exist: " + strings.Join(missing, ", "),
		}},
	})
	continue // reported as ERROR; skip the empty-folder WARN (PRD §9 / Issue 5)
}
// ... existing WARN append follows, unchanged ...

// The helper's key guard: early return on the FIRST existing entry. This makes the
// helper self-contained — even if a future caller invokes it on a dir that classifyDir
// WOULD classify as a package, it correctly returns nil (not-all-missing).
for _, e := range pkg.Pi.Extensions {
	s, ok := e.(string)
	if !ok {
		continue // lenient: non-string entries dropped (matches toStringSlice)
	}
	hasAny = true
	if !fileExists(filepath.Join(dir, s)) {
		missing = append(missing, s)
	} else {
		return nil // at least one exists → NOT all-missing → not this check's concern
	}
}
```

### Integration Points

```yaml
# This fix is INTERNAL to the check package. The single integration is the
# Report.HasErrors() → main exit-code mapping, which ALREADY exists (M4.T3.S1).

CONSUMES (read-only, unchanged):
  - discover.ParsePackageJSON(dir) → (PackageJSON, hasPkg, err)  # the re-parse
  - discover.PackageJSON.Pi.Extensions (lenientAnySlice = []any) # rangeable cross-package
  - os.Stat (via the new local fileExists)                        # entry existence

PRODUCES (for main's check dispatch, unchanged contract):
  - Report.HasErrors()==true when an all-missing-pi.extensions dir is present
    → main prints "ERROR <tag> (<name>): pi.extensions entry does not exist: ..." and exits 1.
  - The synthetic ExtensionReport uses RelTag = folder basename (same shape as the
    WARN path), so main renders it identically to other findings.

NO CHANGES TO:
  - go.mod / go.sum (stdlib os/path/filepath/strings + internal/discover only)
  - discover/* (classifyDir, Index, ParsePackageJSON, PackageJSON — all read-only)
  - extdir/*, resolve/*, search/*, ui/*, main.go
  - the Check() signature, Report/Finding/Severity types, localFindings,
    appendDuplicateRelTagFindings, packageDir, nodeModulesPresent
  - jsdoc.go (parallel P1.M3.T1.S1 owns it)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Tasks 2-5 (check.go edits):
go build ./internal/check/...   # compiles the check package
go vet   ./internal/check/...   # clean

# Project-wide (should be a no-op for this change):
go build ./...
go vet ./...

# Expected: zero errors. The edit uses only os.Stat, filepath.Join, strings.Join,
# and discover.ParsePackageJSON — all already imported. No new symbols beyond the
# two helpers.
```

### Level 2: Unit Tests (Component Validation)

```bash
# TDD: run after Task 1 (expect FAIL — bug present), after Task 4 (expect PASS):
go test ./internal/check/ -run TestCheckAllMissingPiExtensionsIsError -v

# The regression guards (MUST still pass — no-package.json → WARN, resolvable dir → not flagged):
go test ./internal/check/ -run TestCheckEmptyCategoryFolder -v
go test ./internal/check/ -run TestCheckNestedExtensionFolderNotFlagged -v

# Full check package:
go test ./internal/check/... -v

# Expected: all pass. If TestCheckAllMissingPiExtensionsIsError still sees a WARN (not
# ERROR), the ERROR branch was not inserted in the right place — re-read Task 4's FIND
# step (it must be the FIRST statement inside `if len(sub) == 0`, before the WARN append,
# and must end with `continue`).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and run the Issue 5 repro (confirms the user-visible symptom:
# ERROR citing the missing entry, exit 1 — was WARN "empty category folder", exit 0):
go build -o /tmp/weave .
rm -rf /tmp/bug5 && mkdir -p /tmp/bug5/badpkg
printf '{"name":"badpkg","description":"d","pi":{"extensions":["./missing.ts"]}}\n' \
  > /tmp/bug5/badpkg/package.json
weave_EXTENSIONS_DIR=/tmp/bug5 /tmp/weave check
echo "exit=$?"
# EXPECTED: a line like "ERROR badpkg (badpkg): pi.extensions entry does not exist: ./missing.ts"
#           and exit=1 (was: "WARN badpkg (badpkg): empty category folder: badpkg", exit=0).

# Regression: a plain empty folder (NO package.json) still WARNs, exit 0:
rm -rf /tmp/bug5b && mkdir -p /tmp/bug5b/abandoned
printf '/** d */\nexport function g() {}\n' > /tmp/bug5b/gate.ts   # so the store is non-empty
weave_EXTENSIONS_DIR=/tmp/bug5b /tmp/weave check
echo "exit=$?"
# EXPECTED: "WARN abandoned ((none)): empty category folder: abandoned" and exit=0.

# Regression: a VALID package (≥1 existing entry) is clean, no empty-folder finding:
rm -rf /tmp/bug5c && mkdir -p /tmp/bug5c/summarizer/src
printf '{"name":"summarizer","description":"d","pi":{"extensions":["./src/index.ts"]}}\n' \
  > /tmp/bug5c/summarizer/package.json
printf 'export function s() {}\n' > /tmp/bug5c/summarizer/src/index.ts
weave_EXTENSIONS_DIR=/tmp/bug5c /tmp/weave check
echo "exit=$?"
# EXPECTED: "OK summarizer (summarizer)" and exit=0.

# Whole-repo sweep (catches any accidental regression):
go test ./... -v
go test -race ./...
rm -f /tmp/weave
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Multi-entry all-missing: pi.extensions lists TWO missing files → ERROR names both,
# comma-joined (exercises the strings.Join path):
go build -o /tmp/weave .
rm -rf /tmp/bug5d && mkdir -p /tmp/bug5d/multi
printf '{"name":"multi","pi":{"extensions":["./a.ts","./b.ts"]}}\n' > /tmp/bug5d/multi/package.json
weave_EXTENSIONS_DIR=/tmp/bug5d /tmp/weave check
# EXPECTED: "ERROR multi (multi): pi.extensions entry does not exist: ./a.ts, ./b.ts", exit 1.

# Mixed existing+missing: one exists → NOT all-missing → the dir IS a package (Bug 1),
# never reaches the empty-folder path; the existing entry makes it a real extension:
rm -rf /tmp/bug5e && mkdir -p /tmp/bug5e/mix
printf '{"name":"mix","pi":{"extensions":["./exists.ts","./missing.ts"]}}\n' > /tmp/bug5e/mix/package.json
printf '/** the real entry */\nexport function m() {}\n' > /tmp/bug5e/mix/exists.ts
weave_EXTENSIONS_DIR=/tmp/bug5e /tmp/weave check
# EXPECTED: "OK mix (mix)" (or a WARN only if description absent; here no description →
#           WARN "no description", but NO "pi.extensions entry does not exist" ERROR),
#           exit 0. The missing ./missing.ts is NOT flagged here because classifyDir
#           bound the dir to a package via ./exists.ts; check's localFindings then stats
#           the EntryFile (./exists.ts, which exists) → no missing-entryFile ERROR.
#           (If you want the missing SECOND entry flagged, that is a SEPARATE §9
#           enhancement, out of scope for Issue 5 which is the ALL-missing case only.)

rm -rf /tmp/bug5d /tmp/bug5e /tmp/weave
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./internal/check/... -v` — all tests pass (existing + new).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new deps; `go.mod`/`go.sum` unchanged (no new imports in check.go).

### Feature Validation

- [ ] `allMissingPiExtensions` returns `["./missing.ts"]` for a dir whose
      `pi.extensions:["./missing.ts"]` has no such file.
- [ ] `allMissingPiExtensions` returns nil for: no package.json, parse error,
      empty `pi.extensions`, ≥1 existing entry, all-non-string entries.
- [ ] `appendEmptyFolderFindings` appends an ERROR (not WARN) for the all-missing
      case, with `Level==LevelError` and message starting
      `"pi.extensions entry does not exist: "`.
- [ ] The ERROR `continue`s past the WARN append (no double finding).
- [ ] `Report.HasErrors()==true` when an all-missing dir is present.
- [ ] The no-package.json case still appends the WARN (`TestCheckEmptyCategoryFolder`).
- [ ] The resolvable-dir case is still not flagged
      (`TestCheckNestedExtensionFolderNotFlagged`).
- [ ] Level 3 repro: `weave check` on the Issue 5 fixture prints ERROR + exit 1.

### Code Quality Validation

- [ ] `fileExists` is a local 2-line helper mirroring discover.fileExists/extdir.fileExists.
- [ ] `allMissingPiExtensions` is placed immediately before `appendEmptyFolderFindings`.
- [ ] The ERROR branch is the FIRST statement in `if len(sub) == 0` and ends with `continue`.
- [ ] The ERROR uses `e.Name()` for RelTag (matching the WARN path exactly).
- [ ] The `appendEmptyFolderFindings` doc comment notes the all-missing-ERROR check.
- [ ] No signature changes; no new types; no new imports.
- [ ] Only `internal/check/check.go` and `internal/check/check_test.go` are modified.

### Documentation & Deployment

- [ ] `appendEmptyFolderFindings` doc comment updated ([Mode A] ride-along) to note
      the pi.extensions all-missing ERROR that precedes the WARN, citing PRD §9 / Issue 5.
- [ ] No README/user-doc change in this subtask (the M4.T1 doc sync owns user-facing
      prose; fix_design.md §"Mode B" explicitly says "No changes needed for ... Bug 5
      (internal fixes with no user-facing doc surface beyond what Mode A covers)").

---

## Anti-Patterns to Avoid

- ❌ Don't remove the `continue` after the ERROR append — without it the WARN append
  runs too, producing BOTH an ERROR and a WARN for the same dir (double-counting,
  wrong tallies, wrong user message).
- ❌ Don't use `filepath.Base(child)` or `child` for the ERROR's RelTag — use
  `e.Name()` (the os.DirEntry basename), exactly as the WARN path does, so main
  renders them uniformly.
- ❌ Don't drop the `hasAny` flag — it distinguishes "empty pi.extensions" (→ WARN,
  nothing declared) from "all entries missing" (→ ERROR). Without it, an empty
  `pi.extensions:[]` could yield a non-nil empty `missing` (len 0), which the
  caller's `len(missing) > 0` guard happens to handle, but the intent would be
  obscured. Keep `hasAny` for clarity.
- ❌ Don't drop the early `return nil` on the first existing entry — it is the
  self-contained "not all-missing" guard. Even though Bug 1 means such a dir never
  reaches this path, the helper must be correct on its own.
- ❌ Don't try to export `discover.lenientAnySlice` or add a discover helper —
  `pkg.Pi.Extensions` is rangeable as `[]any` cross-package (builtin underlying
  type). The item CONTRACT confirmed this via compilation test.
- ❌ Don't import `discover.fileExists` or `extdir.fileExists` — both are
  UNEXPORTED. Add a local `fileExists` (the established duplication pattern).
- ❌ Don't use `mkPackageExt` for the new test — it writes a VALID `src/index.ts`.
  Write the broken package dir by hand (mirror `TestCheckUnparseablePackageJSON`).
- ❌ Don't touch `discover.go`, `extension.go`, `index.go`, or `jsdoc.go` — this
  task is `internal/check/check.go` ONLY. The parallel P1.M3.T1.S1 owns jsdoc.go.
- ❌ Don't change the `Check` signature, `Report`/`Finding`/`Severity` types, or
  any other check function — the fix is two helpers + one branch.
- ❌ Don't flag the mixed existing+missing case (≥1 exists) as an ERROR — that is
  a real package (Bug 1 binds it); Issue 5 is the ALL-missing case only. Flagging
  a missing SECOND entry is a separate §9 enhancement, out of scope.

---

**Confidence Score: 10/10** for one-pass success. The fix is two small helpers
(a 2-line `fileExists` and a ~15-line `allMissingPiExtensions`) plus a 9-line
branch insertion, all byte-for-byte specified in the item CONTRACT and the
matching `architecture/fix_design.md` Bug 5 section (which agree verbatim). The
sole caller (`appendEmptyFolderFindings`) and its insertion point are identified
with line numbers; the regression guards (`TestCheckEmptyCategoryFolder`,
`TestCheckNestedExtensionFolderNotFlagged`) and the test pattern to mirror
(`TestCheckUnparseablePackageJSON`) are named; the type-safety concern
(`lenientAnySlice` cross-package ranging) is resolved by the item CONTRACT's
compilation-test note; no new imports are needed; and the Bug 1 dependency (which
makes the helper's single trigger condition correct) is COMPLETE. The only
residual risk — forgetting the `continue` after the ERROR append — is eliminated
by an explicit anti-pattern entry and the TDD test (which would catch a
double-finding via the WARN-absence assertion).
