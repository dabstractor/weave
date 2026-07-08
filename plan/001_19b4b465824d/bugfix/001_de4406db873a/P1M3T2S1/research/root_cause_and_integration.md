# Root-cause & integration map: Bug 5 (Issue 5) — all-missing pi.extensions

## The bug (PRD h2.3/h3.4 Issue 5)

A top-level dir whose `package.json` declares `pi.extensions` pointing at a
MISSING entry file is NOT classified as a package by discover (PRD §7.1: a
package requires ≥1 EXISTING entry). It has no `index.ts` either, so
`classifyDir` falls through all cases and treats it as a plain category folder,
descending into it. `discover.Index(child)` yields 0 entries.

`check.appendEmptyFolderFindings` then sees `len(sub) == 0` and emits
`WARN "empty category folder: <name>"` — exit 0. The real problem (a broken
`pi.extensions` reference) is hidden behind an unrelated-sounding WARN at the
wrong severity. PRD §9 explicitly wants an ERROR here ("ERROR: a dir/package
entry's `entryFile` does not exist on disk ... catches hand-edited `package.json`
`pi.extensions` pointing at a missing file").

## The cascade dependency (Bug 1 → Bug 5)

From `architecture/bug_cascade_map.md`:
```
Bug 1 (entries[0] only) ──► Bug 3/Issue5 (check all-missing ERROR)
  classifyDir case (a)        check.go
  NO dependencies             DEPENDS ON Bug 1
```

**Bug 1 (P1.M1.T1.S1) is COMPLETE.** Its fix made `classifyDir` case (a) iterate
ALL `pi.extensions` entries and select the first EXISTING one. The consequence
for Bug 5:

- A `package.json` with ≥1 EXISTING entry → classified as a package by case (a)
  → returned with SkipDir → **never reaches `appendEmptyFolderFindings`** (it is
  a real Extension in the catalog, checked via `localFindings`).
- A `package.json` with ALL entries missing → case (a) finds no existing entry →
  falls through to case b (JSON valid, no perr) → c/d (no index.ts/index.js) →
  e (plain dir, descend) → `discover.Index` returns 0 entries → reaches
  `appendEmptyFolderFindings` → **this is exactly the case Bug 5 must catch.**

So after Bug 1, the ONLY way a package.json-bearing dir reaches the empty-folder
WARN path is the all-missing case. Bug 5 intercepts it there.

## The fix location & shape

**File:** `internal/check/check.go`, function `appendEmptyFolderFindings`
(lines 287–312). Inside the `if len(sub) == 0 { ... }` block, BEFORE appending
the WARN, call a new helper `allMissingPiExtensions(child)`:

- If it returns a non-nil slice → append an `ExtensionReport` with
  `Finding{LevelError, "pi.extensions entry does not exist: " + strings.Join(missing, ", ")}` and `continue` (skip the WARN).
- Otherwise → the existing WARN path runs unchanged (no package.json, parse
  error, empty pi.extensions, or ≥1 entry exists — none of which are this bug).

**New helper `allMissingPiExtensions(dir string) []string`** (verbatim from
`architecture/fix_design.md` Bug 5 + the item CONTRACT):
```go
func allMissingPiExtensions(dir string) []string {
	pkg, hasPkg, perr := discover.ParsePackageJSON(dir)
	if !hasPkg || perr != nil {
		return nil // no package.json, or unparseable → not this check's concern
	}
	var missing []string
	hasAny := false
	for _, e := range pkg.Pi.Extensions {
		s, ok := e.(string)
		if !ok {
			continue // lenient: non-string entries dropped
		}
		hasAny = true
		if !fileExists(filepath.Join(dir, s)) {
			missing = append(missing, s)
		} else {
			return nil // at least one exists → not all-missing
		}
	}
	if !hasAny {
		return nil // pi.extensions empty/absent
	}
	return missing // all declared entries missing
}
```

**New helper `fileExists(path string) bool`** (the check package does NOT have one;
discover.fileExists and extdir.fileExists are unexported dups — add a local copy):
```go
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
```

## Why a local `fileExists` (not discover.fileExists)

`discover.fileExists` is UNEXPORTED (`internal/discover/discover.go:20`). The
`check` package cannot reach it. The codebase ALREADY accepts this duplication:
`discover.fileExists` and `extdir.fileExists` are byte-identical unexported copies
(`discover.go:16` comment: "Mirrors extdir.fileExists — KEEP IN SYNC ... a future
shared internal/rules package is out of scope"). A third local copy in `check`
follows the established pattern. The 2-line body (`os.Stat` + `!IsDir`) makes a
shared helper higher cost than the duplication.

Alternative considered: inline `os.Stat` in `allMissingPiExtensions`. Rejected —
`fileExists` is called once per entry in a loop, and a named helper reads better
+ matches the discover/extdir precedent. Use the helper.

## Type safety: ranging `pkg.Pi.Extensions` from the check package

`pkg.Pi.Extensions` has type `discover.lenientAnySlice` which is `[]any`
(`internal/discover/extension.go:38,209`). `lenientAnySlice` is UNEXPORTED, but
the UNDERLYING type `[]any` is a builtin — ranging it from the `check` package
works directly (`for _, e := range pkg.Pi.Extensions`). Each element is `any`;
the `s, ok := e.(string)` comma-ok narrows to the string entries and drops
non-strings (numbers, bools, objects) leniently — matching `discover.classifyDir`'s
`toStringSlice` behavior.

**VERIFIED:** the item CONTRACT explicitly states "the check package CAN range
over `pkg.Pi.Extensions` (type discover.lenientAnySlice = []any) from external
code — confirmed by compilation test." No export change needed.

## Imports: NOTHING new

`internal/check/check.go` already imports `os`, `path/filepath`, `strings`, and
`internal/discover` (lines 39–47). `fileExists` uses `os`; `allMissingPiExtensions`
uses `filepath.Join` + `discover.ParsePackageJSON`; the ERROR message uses
`strings.Join`. All present. NO new imports.

## What does NOT change (scope guard)

- The `WARN "empty category folder"` path stays for: no package.json, parse
  error, empty pi.extensions, or all-non-string pi.extensions (hasAny=false).
- `TestCheckEmptyCategoryFolder` (a dir with NO package.json) must still pass
  unchanged → still WARN, still exit 0.
- `TestCheckNestedExtensionFolderNotFlagged` (a dir extension with index.ts →
  ≥1 Index entry → never enters the `len(sub)==0` block) is untouched.
- `localFindings`, `appendDuplicateRelTagFindings`, `packageDir`,
  `nodeModulesPresent`, the `Check` signature, the `Report`/`Finding`/`Severity`
  types — all UNCHANGED.
- The parallel item P1.M3.T1.S1 edits `internal/discover/jsdoc.go` ONLY — no
  overlap with this `internal/check/check.go` edit.

## Edge cases the helper handles (return nil = "not this bug's concern")

| package.json state                         | hasPkg | perr  | Pi.Extensions | result  | WARN path? |
|--------------------------------------------|--------|-------|---------------|---------|------------|
| no package.json                            | false  | nil   | —             | nil     | YES (WARN) |
| unparseable JSON                           | true   | err   | —             | nil     | YES (WARN)*|
| valid, no `pi` key                         | true   | nil   | nil           | nil     | YES (WARN) |
| valid, `pi.extensions: []` (empty array)   | true   | nil   | []any{}       | nil     | YES (WARN) |
| valid, `pi.extensions: ["./missing.ts"]`   | true   | nil   | ["./missing"] | [path]  | NO → ERROR |
| valid, `pi.extensions: ["./exists.ts","x"]`| true   | nil   | [...]         | nil     | YES (WARN)**|
| valid, `pi.extensions: [123]` (non-string) | true   | nil   | [123]         | nil     | YES (WARN) |

*Unparseable JSON reaching the empty-folder path is theoretically possible but
rare (would need a package.json that fails ParsePackageJSON AND no index.ts AND
all pi.extensions missing — but a parse error means Pi.Extensions is nil anyway).
The `perr != nil → nil` guard keeps it out of this check's concern; the
unparseable ERROR is surfaced elsewhere via `localFindings` if the dir WAS
classified as an extension.
**If ≥1 entry exists, classifyDir case (a) classifies it as a package and it
never reaches appendEmptyFolderFindings — but the helper's `return nil` on first
existing entry is the correct, self-contained guard regardless.

## Test fixture pattern (mirror TestCheckUnparseablePackageJSON)

The new test must build the broken package dir DIRECTLY (not via mkPackageExt,
which writes a valid src/index.ts). Write files by hand under `t.TempDir()`:

```go
root := t.TempDir()
dir := filepath.Join(root, "badpkg")
os.MkdirAll(dir, 0o755)
os.WriteFile(filepath.Join(dir, "package.json"),
    []byte(`{"name":"badpkg","description":"d","pi":{"extensions":["./missing.ts"]}}`), 0o644)
// NOTE: no ./missing.ts file created — that's the bug condition.
```

Then call `Check(root, exts)` where `exts` is built how `main` would: it Indexes
the root, which yields ZERO entries for badpkg (all-missing → plain folder →
descend → nothing inside). So `exts` will be empty OR contain other extensions;
`appendEmptyFolderFindings` walks `root`'s children directly and finds badpkg.

The assertion: `rep.HasErrors() == true`, `rep.Errors >= 1`, and
`finding(rep, "pi.extensions entry does not exist")` returns a Finding with
`Level == LevelError`. Also assert `finding(rep, "empty category folder")` does
NOT find a message containing "badpkg" (the WARN is suppressed for badpkg).

To prove the WARN still fires for the no-package.json case in the SAME test run
(or in the existing TestCheckEmptyCategoryFolder which must still pass), include
a plain empty dir too — but the existing TestCheckEmptyCategoryFolder already
covers that regression, so the new test can focus solely on the badpkg ERROR.
