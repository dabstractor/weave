# PRP — P1.M4.T2.S1: `check.Check()` with all §9 rules + Report/ExtensionReport/Finding types

## Goal

**Feature Goal**: Implement `internal/check` — a FUNCTION that validates every
extension in the store against the PRD §9 rules and returns a structured
`Report`. It ports the `Report`/`Severity`/`Finding`/`ExtensionReport` TYPE
STRUCTURE from skilldozer's `internal/check/check.go` (read in full during
research), but REWRITES the validation logic because weave's §9 rules are
entirely different from skilldozer's Agent-Skills frontmatter rules. Per
`architecture_mapping.md §7`: "Port the Report/SkillReport(→ExtensionReport)/
Finding/Severity STRUCTURE from skilldozer. The validation RULES are different
(PRD §9)."

**Deliverable**: TWO new files in ONE new package —
- `internal/check/check.go` — package `check`: `Severity`, `Finding`,
  `ExtensionReport`, `Report`, `HasErrors()`, and `Check(dir, exts) Report`.
- `internal/check/check_test.go` — package `check` (white-box): ~10 test
  functions covering every §9 rule + the empty-input edge case.

PLUS a SMALL edit to the already-Complete discover package: EXPORT
`parsePackageJSON` → `ParsePackageJSON` and `packageJSON` → `PackageJSON` (rename
+ capitalize the 4 affected symbols) so check can re-parse per entry to
distinguish "package.json present but unparseable" from "no package.json"
(mirroring skilldozer's re-parse of frontmatter). NO behavior change in discover.

NO `main.go` changes (the `check` dispatch lands in P1.M4.T3.S1). NO `go.mod`
changes (stdlib `encoding/json`, `os`, `path/filepath`, `sort`, `fmt`, `strings`
+ `internal/discover`).

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `go test ./internal/check/... -v` — ALL tests pass.
- `go test -race ./...` passes (existing M1/M2/M3 + new check + discover export).
- A clean store returns `Report{Errors:0, Warnings:0}` with `HasErrors()==false`.
- Each of the 6 §9 rules fires exactly one finding of the correct level when its
  condition is met; the empty-folder rule requires the `dir` argument.
- `Check` does NOT print anything — it returns a structured `Report`; main
  (M4.T3.S1) renders it.

## User Persona (if applicable)

**Target User**: `main.go`'s `check` dispatch (P1.M4.T3.S1) — the sole consumer.
Indirectly: the end user running `weave check`, who wants to know which
extensions have problems (broken package.json, missing entry files, undeclared
deps, ambiguous tags) before relying on `pi -e "$(weave …)"`.

**Use Case**: `main.go` has already called `extdir.Find()` and
`discover.Index(dir)`. For `weave check` it calls `rep := check.Check(dir, exts)`,
renders the report (one line per finding + a summary line, PRD §9 format), and
exits 1 iff `rep.HasErrors()`. This task is the validator that call delegates to.

**User Journey**:
1. `weave check` → `main` calls `check.Check(dir, exts)`.
2. `Check` runs three passes: per-extension local checks (re-parse package.json,
   stat entryFile, deps/node_modules, description), global duplicate-relTag scan,
   and the empty-category-folder walk over `dir`. It tallies Errors/Warnings.
3. `main` renders the `Report` and maps `HasErrors()` to the exit code.

**Pain Points Addressed**:
- **Catalog ambiguity**: two entries with the same canonical relTag make
  `pi -e "$(weave tag)"` ambiguous; §9 surfaces it as an ERROR.
- **Silent breakage**: a package.json pointing at a missing entry file, or deps
  declared but not installed, breaks `pi -e` load silently; §9 flags them.
- **Hygiene**: empty category folders and missing descriptions are surfaced as
  informational WARNs.

## Why

- **PRD §9 is the authoritative rule list** (6 rules: 3 ERROR, 3 WARN). This task
  implements exactly that. The output-format/exit-code rendering is main's job
  (M4.T3.S1); this task is the pure validator returning a structured report.
- **Port the structure, rewrite the logic** (architecture_mapping §7). The TYPE
  structure (`Severity` iota + `String()`, `Finding{Level,Message}`,
  `ExtensionReport{Extension, Findings}`, `Report{ByExt, Errors, Warnings}` +
  `HasErrors()`) is PROVEN in skilldozer and ports verbatim with noun swaps
  (`Skill`→`Extension`, `SkillReport`→`ExtensionReport`, `BySkill`→`ByExt`). The
  validation LOGIC is rewritten because weave has no frontmatter/name rules.
- **Re-parse mirrors skilldozer's pattern**. discover.Index DROPS the
  parsePackageJSON error (lenient — a malformed package.json still yields a
  resolvable entry via dir classification). check re-parses each entry's package
  dir to recover the unparseable-vs-missing distinction, exactly as skilldozer's
  check re-parses each SKILL.md frontmatter. The item description sanctions
  exporting `parsePackageJSON` ("needs to be accessible — export it or re-parse
  in check"). Exporting is cleaner than re-implementing JSON parsing in check.
- **Scope boundary**: this task delivers ONLY the check package + the discover
  export. It does NOT touch `main.go` (M4.T3.S1 owns the `check` dispatch and
  the §9 output format), does NOT add exit-code logic, and does NOT change
  discover's behavior (only symbol visibility).

## What

A new package `internal/check` (package `check`) with the public surface below,
PLUS a visibility-only edit to `internal/discover/extension.go`.

### 1. Discover export edit (visibility-only, NO behavior change)

In `internal/discover/extension.go`:
- Rename `func parsePackageJSON` → `func ParsePackageJSON`.
- Rename `type packageJSON struct` → `type PackageJSON struct` and capitalize
  its field tags' Go names (`Name`, `Description`, `Keywords`, `Pi`, `Weave`,
  `Dependencies` — they are ALREADY capitalized; only the TYPE name changes).
- Update all internal callers (`classifyFile`, `classifyDir`, `BuildExtension`'s
  parameter type, the doc comments) to use the new names.

This is a mechanical rename; `go build` will catch every missed call site.

### 2. The check package

```go
package check

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// Severity ranks a finding. OK < WARN < ERROR. Exported so main (M4.T3.S1) can
// switch on it when rendering. OK is the implicit value for an entry with no
// findings (never carried by a Finding).
type Severity int

const (
	LevelOK Severity = iota
	LevelWarn
	LevelError
)

// String renders a Severity as the status word main left-pads ("OK", "WARN",
// "ERROR"). Ports verbatim from skilldozer.
func (s Severity) String() string {
	switch s {
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "OK"
	}
}

// Finding is one validation result line for a single extension. An entry with
// zero findings is OK; an entry with N findings emits N ERROR/WARN lines.
type Finding struct {
	Level   Severity
	Message string
}

// ExtensionReport binds an extension to its findings (ports skilldozer's
// SkillReport; field Skill → Extension). ByExt is in input order (Index sorts by
// RelTag), so the report is deterministic.
type ExtensionReport struct {
	Extension discover.Extension
	Findings  []Finding
}

// Report is the full check output. ByExt is in input order; Errors/Warnings are
// totals across all findings (drive the summary line + exit code).
type Report struct {
	ByExt    []ExtensionReport
	Errors   int
	Warnings int
}

// HasErrors reports whether any ERROR finding exists. main maps this to the exit
// code (PRD §9: exit 1 if any ERROR). WARNs never affect it.
func (r Report) HasErrors() bool { return r.Errors > 0 }
```

### 3. `Check(dir, exts) Report` — the three-pass algorithm

**Signature**: `func Check(dir string, exts []discover.Extension) Report`
(`dir` is the absolute extensions directory from `extdir.Find`; required for the
§9 empty-category-folder walk — see Known Gotchas).

**Pass 1 (per-extension local checks)**, for each `exts[i]`:
1. **Re-parse package.json** in the entry's package dir (see `packageDir(ext)`
   below): `pkg, hasPkg, perr := discover.ParsePackageJSON(packageDir(ext))`.
   - If `hasPkg && perr != nil` → **ERROR** `'package.json is not valid JSON'`
     (PRD §9; mirrors skilldozer's re-parse). The raw err is NOT surfaced (the
     message is fixed per the item description); check uses `perr != nil` only
     as the trigger.
2. **Stat the entry file**: `os.Stat(ext.EntryFile)`.
   - If it does not exist (`os.IsNotExist`, or any stat failure) → **ERROR**
     `'entry file does not exist: <ext.EntryFile>'` (PRD §9; defensive).
3. **deps without node_modules** (only when `ext.Kind == "package"` AND
   `hasPkg && perr == nil` AND `len(pkg.Dependencies) > 0`):
   - If `node_modules/` does NOT exist in `ext.Path` (the package dir) → **WARN**
     `'dependencies declared but node_modules/ not found'` (PRD §9).
4. **no description**: if `ext.Description == ""` (already folds package.json
   description OR the leading JSDoc from BuildExtension) → **WARN** `'no
   description (neither package.json description nor leading JSDoc)'` (PRD §9).

  `packageDir(ext)` returns the directory whose package.json discover parsed:
  `ext.Path` for `dir`/`package` kinds (Path is the dir), `filepath.Dir(ext.Path)`
  for `file` kind (Path is the file → package.json sits beside it).

**Pass 2 (global checks)**:
- **Duplicate relTag**: build `map[string][]string` (relTag → list of entry
  indices). For any relTag with ≥2 owners → **ERROR** per owner, naming the other
  tag(s). Message: `'duplicate relTag "<tag>" (also in: <other1>, <other2>)'`
  with `others` sorted for determinism. (PRD §9; note: in practice Index prunes
  duplicates, but two `foo.ts` and `foo/index.ts` can collide on
  case-insensitive filesystems — this catches it.)
- **Empty category folder** (the walk over `dir`): for each top-level child of
  `dir`:
  - If the child is a plain (NON-extension) directory, run a discover walk
    inside it; if it yields ZERO entries at any depth → **WARN**
    `'empty category folder: <child>'`. A child that IS a resolvable extension
    (has `index.ts`/`index.js` or a qualifying `package.json`) is NOT empty.

  Implementation: read `os.ReadDir(dir)`; for each child that `os.Stat` says is a
  directory AND is NOT itself a resolvable extension (test via a local
  `isResolvable(d)` predicate mirroring discover's classifyDir cases a/b/c), run
  `discover.Index(child)`; if the returned slice is empty → WARN. (Reusing
  `discover.Index` keeps the "discoverable entries at any depth" semantics
  identical to Index's classify-then-descend rule.)

**Pass 3 (tally)**: count Errors and Warnings across all `ByExt[i].Findings` plus
the empty-folder findings. (Empty-folder findings are appended to a synthetic
`ExtensionReport` whose `Extension` is zero-valued with `RelTag` set to the
folder name, OR stored in a separate slice on the Report — see Implementation
Tasks. Prefer appending to `ByExt` with a synthetic extension so main renders
them uniformly.)

**Empty store is clean**: `Check(dir, nil)` returns `Report{ByExt:nil,
Errors:0, Warnings:0}` and `HasErrors()==false`. (An empty store has no entries
and no non-empty top-level dirs that are non-extension → 0/0/0, exit 0.)

### Success Criteria

- [ ] `Severity`/`String()`/`Finding`/`ExtensionReport`/`Report`/`HasErrors()`
      exist with the exact signatures above (structure ported from skilldozer
      with `Skill`→`Extension`, `SkillReport`→`ExtensionReport`, `BySkill`→`ByExt`).
- [ ] `Check(dir, exts) Report` exists with the `dir` first parameter (required
      for the empty-folder walk).
- [ ] Pass 1 emits exactly the 4 local findings (unparseable pkg, missing
      entryFile, deps-without-node_modules WARN, no-description WARN).
- [ ] Pass 2 emits the duplicate-relTag ERROR per owner (with sorted others) and
      the empty-category-folder WARN.
- [ ] Pass 3 tallies Errors/Warnings correctly; `HasErrors()` reflects Errors>0.
- [ ] `Check(dir, nil)` is a clean empty report (0/0/0, no panic).
- [ ] `discover.ParsePackageJSON` and `discover.PackageJSON` are exported (rename
      only; behavior unchanged).
- [ ] No `main.go` changes; no `go.mod` changes.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the type structure to port is at
`/home/dustin/projects/skilldozer/internal/check/check.go` (read in full), the
§9 rules and exact finding messages are in the PRD (§9) and this PRP's "What"
section, the consumed type (`discover.Extension`, `discover.Index`) is fully
documented in `internal/discover/extension.go` and `index.go`, the 3-valued
`ParsePackageJSON` return is documented in `extension.go`'s doc comment, the
consumer (`main.go` check dispatch) is identified as a LATER task (M4.T3.S1),
and the design decisions (export ParsePackageJSON; add `dir` param for the
empty-folder walk) are spelled out in `research/design_notes.md` and the
Implementation Tasks. No guessing required.

### Documentation & References

```yaml
# MUST READ — the type structure to port
- file: /home/dustin/projects/skilldozer/internal/check/check.go
  why: contains Severity/String/Finding/SkillReport/Report/HasErrors — the type
       structure to port VERBATIM with noun swaps. Read in FULL.
  pattern: Severity iota + String switch; Finding{Level,Message}; Report struct
           with Errors/Warnings tally driven by a Pass 3 loop; HasErrors() one-liner.
  gotcha: skilldozer's Check(skills) takes ONLY []Skill; weave's Check(dir, exts)
          takes the dir too (the empty-folder rule needs a FS walk). Do NOT copy
          skilldozer's signature verbatim. checkFields/appendDupFindings/validName
          are skilldozer-specific (frontmatter/name rules) and are NOT ported.

- file: /home/dustin/projects/skilldozer/internal/check/check_test.go
  why: test-helper pattern (mkSkill writes a temp file + builds a Skill) to adapt
       into an mkExt that writes a temp extension tree. White-box same-package tests.
  pattern: t.TempDir(); os.MkdirAll; os.WriteFile; build []discover.Extension via
           the real discover helpers so check sees realistic Extension values.
  gotcha: the test BODIES are rewritten (weave's rules differ); only the helper
          SKELETON and the table-driven assertion style port.

# The input type + the function to export
- file: internal/discover/extension.go
  why: defines discover.Extension (the validated record) AND parsePackageJSON
       (the 3-valued function to EXPORT). The doc comment on parsePackageJSON
       documents the 3 return cases (no-pkg / unparseable / success) — check's
       Pass 1 branches on exactly those.
  pattern: parsePackageJSON(dir) (packageJSON, hasPkg bool, err error); BuildExtension
           already folds package.json description OR JSDoc into Extension.Description.
  gotcha: RENAME parsePackageJSON→ParsePackageJSON and packageJSON→PackageJSON
          (capitalize the type + func). The struct FIELDS are already capitalized;
          only the type name + func name change. Update classifyFile/classifyDir/
          BuildExtension's parameter type (all in discover/) — go build catches misses.

# The Index walk + classify-then-descend (for the empty-folder rule + re-use)
- file: internal/discover/index.go
  why: Index(extensionsDir) ([]Extension, error) — reused by check's empty-folder
       walk to test "does this subdir contain zero entries at any depth". Also
       confirms Index returns (nil, nil) for an empty dir.
  pattern: Index makes the dir absolute, stat-guards, WalkDir with SkipDir pruning.
  gotcha: an empty dir yields (nil, nil) — check treats len(result)==0 as "empty".
          A dir that IS a resolvable extension yields 1 entry — NOT empty.

- file: internal/discover/discover.go
  why: classifyDir's resolvability cases (a: package.json+pi.extensions, b: index.ts,
       c: index.js, d: plain) — check needs a local isResolvable(d) predicate that
       mirrors cases a/b/c to decide "is this top-level child an extension or a
       plain category folder".
  pattern: fileExists helper; parsePackageJSON-based package detection.
  gotcha: isResolvable in check is a LOCAL predicate (check cannot import discover's
          unexported classifyDir). Re-implement the ~3 cases; OR simpler — just call
          discover.Index on EVERY top-level subdir and treat len==0 as "empty"
          (an extension dir yields ≥1 entry, so this is equivalent and avoids
          duplicating the resolvability logic). PREFER the Index-on-every-subdir
          approach — it is correct and DRY.

# Architecture mapping (the source-of-truth ADAPT directive)
- docfile: plan/001_19b4b465824d/architecture/architecture_mapping.md
  section: §7 internal/check — Validation (ADAPT)
  why: pins "Port the Report/SkillReport(→ExtensionReport)/Finding/Severity
       STRUCTURE. The validation RULES are different (PRD §9)."

# PRD spec (authoritative rule list)
- docfile: PRD.md
  section: §9 Validation — weave check (the 6 rules, ERROR vs WARN, output format, exit code)
  why: pins every rule, its level, and the message intent. The EXACT finding
       messages are given in this PRP's "What" section (item description §3).
- docfile: PRD.md
  section: §7.1 Discovery (relTag/entryFile/kind field defs; the classify-then-descend rule)
  why: defines what relTag/entryFile/kind mean and why an empty category folder
       is "a top-level non-entry dir with zero discoverable entries at any depth".

# The CONSUMING task (P1.M4.T3.S1 — later)
- note: main.go's check dispatch (M4.T3.S1) will call check.Check(dir, exts),
        render the report (OK lines + WARN/ERROR lines + summary), and exit 1 iff
        rep.HasErrors(). This task does NOT wire that — it only delivers the package.
        Design Check so the consumer is: rep := check.Check(dir, exts); ...; os.Exit(exit).
```

### Current Codebase tree (relevant subset)

```bash
go.mod                          # module github.com/dabstractor/weave, go 1.25
internal/
├── discover/
│   ├── extension.go            # Extension + parsePackageJSON (TO EXPORT) + BuildExtension
│   ├── jsdoc.go                # M2.T1.S2 (done) — folded into Description
│   ├── discover.go             # classifyFile/classifyDir (consumed by Index; pattern for isResolvable)
│   └── index.go                # Index() → []Extension sorted by RelTag — reused for empty-folder walk
├── resolve/                    # M3.T1.S1 (done) — sibling pure-matching package (parity ref)
├── config/, extdir/, ui/       # done — not used here
└── (no check/ yet)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/
├── discover/
│   └── extension.go            # MODIFIED — export ParsePackageJSON + PackageJSON (rename only)
└── check/
    ├── check.go                # NEW — package check: types + Check(dir, exts) Report
    └── check_test.go           # NEW — package check: ~10 white-box tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: do NOT copy skilldozer's Check(skills) signature. weave needs
// Check(dir string, exts []discover.Extension) Report because the §9 empty-
// category-folder rule cannot be derived from []Extension alone (Index prunes
// resolvable subtrees and emits only Extension records — an empty top-level
// folder contributes zero entries and is invisible to exts). The dir param is
// the ONLY way to detect it.

// CRITICAL: EXPORT parsePackageJSON (→ ParsePackageJSON) and packageJSON
// (→ PackageJSON) in discover/extension.go. check re-parses each entry's
// package dir to distinguish "present but unparseable" (hasPkg && err != nil)
// from "no package.json" (!hasPkg). This mirrors skilldozer's re-parse of
// frontmatter. The rename is visibility-only; go build catches every missed
// call site in discover.

// CRITICAL: packageDir(ext) must return the SAME dir discover parsed. For
// file-kind exts, package.json sits BESIDE the file → filepath.Dir(ext.Path).
// For dir/package exts, package.json sits IN ext.Path (Path is the dir).
// Do NOT use Dir(ext.EntryFile) for package exts — EntryFile is
// ext.Path/src/index.ts, so Dir(EntryFile) != ext.Path.

// CRITICAL: the deps-without-node_modules WARN fires ONLY for
// ext.Kind == "package" (PRD §9: "a PACKAGE extension's package.json declares
// non-empty dependencies"). dir-kind extensions with a package.json that has
// deps do NOT trigger this (their deps are not load-bearing for pi). And it
// fires only when hasPkg && perr == nil (a parseable package.json) AND
// len(pkg.Dependencies) > 0.

// CRITICAL: node_modules existence is checked via os.Stat on
// filepath.Join(ext.Path, "node_modules") AND IsDir (a stray file named
// node_modules must not satisfy the check). ext.Path for a package ext IS the
// package dir (BuildExtension sets Path=the dir for package/dir kinds).

// GOTCHA: empty-folder walk via discover.Index(child) for EVERY top-level
// subdir is correct AND DRY (an extension dir yields ≥1 entry; a plain empty
// category dir yields 0). Do NOT re-implement classifyDir's resolvability
// cases in check — just call Index and test len==0.

// GOTCHA: empty-folder findings need a home in the Report. Append them as
// synthetic ExtensionReport entries (Extension zero-value with RelTag set to
// the folder's basename or relative path) so main (M4.T3.S1) renders them
// uniformly with the per-extension findings. Document this in a doc comment.

// GOTCHA: HasPackageJSON on the Extension reflects whether discover READ a
// package.json — but discover may have read it AND failed to parse (hasPkg is
// still set true on the unparseable path in parsePackageJSON's 2nd return).
// Actually re-check: parsePackageJSON returns hasPkg=true, err!=nil on
// unparseable JSON. BuildExtension stores hasPkg into HasPackageJSON. So an
// unparseable package.json yields ext.HasPackageJSON==true but re-parsing
// gives err!=nil. check MUST re-parse (cannot rely on HasPackageJSON alone) —
// confirmed by the item description's "check re-parses package.json per entry".

// GOTCHA: import path is github.com/dabstractor/weave/internal/discover (NOT
// skilldozer). A stale skilldozer import in a copied test fails to compile.

// GOTCHA: check must NOT print anything. It returns a Report; main renders.
// skilldozer's check likewise returns (does not print). The §9 output format
// ("OK <relTag>", summary line) is main's concern (M4.T3.S1).
```

## Implementation Blueprint

### Data models and structure

The data models are the ported type structure (see the "What" section for full
bodies): `Severity` (iota + `String()`), `Finding{Level, Message}`,
`ExtensionReport{Extension, Findings}`, `Report{ByExt, Errors, Warnings}` +
`HasErrors()`. No database, no ORM. The only consumed model is
`discover.Extension` (read-only).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EXPORT discover.ParsePackageJSON + PackageJSON (visibility-only rename)
  - EDIT internal/discover/extension.go:
      - `func parsePackageJSON(dir string)` → `func ParsePackageJSON(dir string)`
      - `type packageJSON struct` → `type PackageJSON struct`
      - the return type annotation `(pkg packageJSON, ...)` → `(pkg PackageJSON, ...)`
      - BuildExtension's parameter `pkg packageJSON` → `pkg PackageJSON`
  - EDIT internal/discover/discover.go + index.go: update any local references
    (classifyFile/classifyDir call parsePackageJSON → ParsePackageJSON; the
    `pkg packageJSON` local var types). go build will list every missed site.
  - RUN: go build ./internal/discover/... ; go test ./internal/discover/...
  - EXPECT: green — this is a pure rename; existing discover tests still pass.
  - FILES TOUCHED: internal/discover/extension.go, discover.go (2-3 sites).
  - NO behavior change. The doc comment on ParsePackageJSON stays as-is.

Task 2: CREATE internal/check/check.go — the type structure
  - CREATE the directory internal/check/ and the file check.go.
  - WRITE: `package check` + imports ("fmt", "os", "path/filepath", "sort",
    "strings", "github.com/dabstractor/weave/internal/discover") + the Severity
    type + the three consts + String() + Finding + ExtensionReport + Report +
    HasErrors() (see the What section for the full bodies — port from skilldozer
    with Skill→Extension, SkillReport→ExtensionReport, BySkill→ByExt).
  - PORT FROM: /home/dustin/projects/skilldozer/internal/check/check.go (the type
    declarations ONLY; NOT checkFields/appendDupFindings/validName — those are
    skilldozer's frontmatter rules).
  - FILES TOUCHED: 1 (internal/check/check.go — NEW, part 1).

Task 3: IMPLEMENT Check(dir, exts) Report — the three passes
  - ADD to internal/check/check.go:
      - `func packageDir(ext discover.Extension) string` — returns ext.Path for
        dir/package kinds, filepath.Dir(ext.Path) for file kind.
      - `func Check(dir string, exts []discover.Extension) Report`:
          Pass 1: for each ext, re-parse via discover.ParsePackageJSON(packageDir(ext));
            append findings for (unparseable ERROR, missing-entryFile ERROR,
            deps-without-node_modules WARN [package kind only], no-description WARN).
          Pass 2: build map[relTag][]index; for dups, append an ERROR per owner
            with sorted others. Then read os.ReadDir(dir); for each top-level
            subdir, call discover.Index(child); if len==0 → append a WARN finding
            'empty category folder: <relpath>' to a synthetic ExtensionReport.
          Pass 3: tally Errors/Warnings across all findings.
      - The deps/node_modules check: only when ext.Kind=="package" && hasPkg &&
        perr==nil && len(pkg.Dependencies)>0 && no node_modules/ dir in ext.Path.
  - DO NOT print anything. Return the Report.
  - FILES TOUCHED: 1 (internal/check/check.go — part 2).

Task 4: VALIDATE check.go compiles + vets
  - RUN: go build ./internal/check/... ; go vet ./internal/check/...
  - EXPECT: clean. Most likely failure: a stale skilldozer import path, or a
    missed Skill→Extension/BySkill→ByExt noun swap, or referencing the OLD
    parsePackageJSON name.

Task 5: CREATE internal/check/check_test.go
  - CREATE the file internal/check/check_test.go.
  - WRITE: `package check` (white-box) + imports ("os", "path/filepath",
    "strings", "testing", "github.com/dabstractor/weave/internal/discover") +
    an mkExt helper that writes a temp extension tree (file/dir/package) and
    returns a discover.Extension built the way Index would (write the files,
    call discover.Index(root), return the matching entry — or build directly
    via discover.BuildExtension with a synthetic parse).
  - TEST FUNCTIONS (one per §9 rule + edges):
      - TestCheckCleanStore: a valid file ext + a valid package ext → 0/0, HasErrors false.
      - TestCheckDuplicateRelTag: two exts with the same RelTag → 2 ERRORs,
        each naming the other.
      - TestCheckMissingEntryFile: an ext whose EntryFile stat fails → 1 ERROR
        'entry file does not exist'.
      - TestCheckUnparseablePackageJSON: a package.json with broken JSON → 1 ERROR
        'package.json is not valid JSON'.
      - TestCheckDepsWithoutNodeModules: a package ext with package.json deps
        and no node_modules/ → 1 WARN.
      - TestCheckDepsWithNodeModulesOK: same but node_modules/ present → no WARN.
      - TestCheckNoDescription: a file ext with no pkg and no JSDoc → 1 WARN
        'no description'.
      - TestCheckEmptyCategoryFolder: a top-level plain subdir with no entries →
        1 WARN 'empty category folder'.
      - TestCheckEmptyInputClean: Check(dir, nil) → 0/0, HasErrors false, no panic.
  - Use t.TempDir() + real files (mirrors skilldozer's mkSkill; realistic
    Extension values flow into Check).
  - FILES TOUCHED: 1 (internal/check/check_test.go — NEW).

Task 6: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./internal/check/... -v ;
    go test -race ./...
  - EXPECT: all green (the new check tests pass; discover export rename keeps
    existing discover tests green; all M1/M2/M3 tests unaffected).
  - On failure, the cause is almost always: a stale skilldozer import, a missed
    parsePackageJSON→ParsePackageJSON rename in discover, or an off-by-one in
    the empty-folder walk (e.g. flagging an extension dir as "empty").
```

### Implementation Patterns & Key Details

```go
// packageDir returns the directory whose package.json discover parsed for ext.
// file-kind: package.json sits BESIDE the file. dir/package: it sits IN ext.Path.
func packageDir(ext discover.Extension) string {
	if ext.Kind == "file" {
		return filepath.Dir(ext.Path)
	}
	return ext.Path
}

// nodeModulesPresent reports whether <ext.Path>/node_modules is a directory.
func nodeModulesPresent(ext discover.Extension) bool {
	info, err := os.Stat(filepath.Join(ext.Path, "node_modules"))
	return err == nil && info.IsDir()
}

// The deps-without-node_modules WARN gate (PRD §9: PACKAGE exts only).
if ext.Kind == "package" && hasPkg && perr == nil && len(pkg.Dependencies) > 0 && !nodeModulesPresent(ext) {
	findings = append(findings, Finding{LevelWarn, "dependencies declared but node_modules/ not found"})
}

// The empty-folder walk (Pass 2). For EVERY top-level subdir of dir, call
// discover.Index; an empty result means "no discoverable entries at any depth".
// This is correct AND DRY: an extension dir yields ≥1 entry, so it is never
// flagged. Do NOT re-implement classifyDir's resolvability here.
entries, _ := os.ReadDir(dir)
for _, e := range entries {
	if !e.IsDir() {
		continue
	}
	child := filepath.Join(dir, e.Name())
	if sub, ierr := discover.Index(child); ierr == nil && len(sub) == 0 {
		// plain category folder with zero entries → WARN.
		rep.ByExt = append(rep.ByExt, ExtensionReport{
			Extension: discover.Extension{RelTag: e.Name()},
			Findings: []Finding{{LevelWarn,
				"empty category folder: " + e.Name()}},
		})
	}
}

// The duplicate-relTag scan (Pass 2). Build map[relTag][]int; for any tag with
// ≥2 owners, append an ERROR per owner naming the sorted others.
owners := map[string][]int{}
for i, e := range exts { owners[e.RelTag] = append(owners[e.RelTag], i) }
for tag, idxs := range owners {
	if len(idxs) < 2 { continue }
	allTags := make([]string, 0, len(idxs))
	for _, i := range idxs { allTags = append(allTags, exts[i].RelTag) }
	sort.Strings(allTags)
	for _, i := range idxs {
		others := make([]string, 0, len(idxs)-1)
		for _, t := range allTags { if t != exts[i].RelTag { others = append(others, t) } }
		rep.ByExt[i].Findings = append(rep.ByExt[i].Findings, Finding{
			LevelError,
			fmt.Sprintf("duplicate relTag %q (also in: %s)", exts[i].RelTag, strings.Join(others, ", ")),
		})
	}
}

// The mkExt test helper skeleton (adapt skilldozer's mkSkill). Writes a temp
// tree and returns the Extension the way discover.Index would.
func mkFileExt(t *testing.T, root, relTag, body string) discover.Extension {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relTag)+".ts")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(body), 0o644)
	exts, _ := discover.Index(root)
	for _, e := range exts { if e.RelTag == relTag { return e } }
	t.Fatalf("ext %q not discovered in %s", relTag, root)
	return discover.Extension{}
}
```

### Integration Points

```yaml
CONSUMES:
  - discover.Extension       # the validated record (Path/EntryFile/RelTag/Kind/Description/HasPackageJSON)
  - discover.Index           # reused for the empty-folder walk (subdir → entries count)
  - discover.ParsePackageJSON # EXPORTED in Task 1 — re-parse to distinguish unparseable vs missing

PRODUCES (for the later check dispatch):
  - check.Report / check.Severity / check.Finding / check.ExtensionReport
  - check.Check(dir, exts) Report   # M4.T3.S1 (main.go check) consumes this;
                                     #   renders the report; exit 1 iff HasErrors().

NO CHANGES TO:
  - go.mod / go.sum (stdlib + internal/discover only)
  - main.go (M4.T3.S1 owns the check dispatch + §9 output format)
  - discover BEHAVIOR (Task 1 is a visibility-only rename)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (discover export):
go build ./internal/discover/...
go vet ./internal/discover/...
go test ./internal/discover/...      # rename is behavior-neutral; existing tests pass

# After Task 2-3 (check.go):
go build ./internal/check/...
go vet ./internal/check/...

# Project-wide:
go build ./...
go vet ./...

# Expected: zero errors. Most likely failure: a missed parsePackageJSON→
# ParsePackageJSON rename in discover (go build lists every site), or a stale
# skilldozer import in check.go/check_test.go.
```

### Level 2: Unit Tests (Component Validation)

```bash
# After Task 5 (check_test.go):
go test ./internal/check/... -v
go test -race ./...

# Targeted re-run while debugging a single rule:
go test -run 'TestCheckUnparseable|TestCheckDeps' ./internal/check/... -v
go test -run 'TestCheckEmptyCategory' ./internal/check/... -v

# Expected: all check tests pass. On failure, the cause is almost always:
#   (a) packageDir returning the wrong dir for file vs package kind (the
#       re-parse then misses the package.json)
#   (b) the deps WARN firing for dir-kind (should be package-kind only)
#   (c) the empty-folder walk flagging an extension dir as empty (Index should
#       return ≥1 entry for a resolvable dir — check the test fixture)
#   (d) a stale skilldozer import path (compile error)
```

### Level 3: Integration Testing (System Validation)

```bash
# check is a library (no I/O of its own beyond the empty-folder walk); the
# end-to-end `weave check` behavior is validated in P1.M4.T3.S1 (main dispatch).
# Here, do a smoke-test that check.Check composes with the real discover.Index
# and extdir resolution, to confirm the contract across package boundaries:

cat > /tmp/check_smoke_test.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dabstractor/weave/internal/check"
	"github.com/dabstractor/weave/internal/discover"
)

func main() {
	root, _ := os.MkdirTemp("", "weave-check-smoke")
	defer os.RemoveAll(root)
	// a clean file extension
	os.MkdirAll(filepath.Join(root, "writing"), 0o755)
	os.WriteFile(filepath.Join(root, "writing", "ok.ts"),
		[]byte("/** a clean extension */\nexport function x() {}\n"), 0o644)
	// an empty category folder
	os.MkdirAll(filepath.Join(root, "empty"), 0o755)
	exts, _ := discover.Index(root)
	rep := check.Check(root, exts)
	fmt.Printf("errors=%d warnings=%d hasErrors=%v\n", rep.Errors, rep.Warnings, rep.HasErrors())
	for _, r := range rep.ByExt {
		for _, f := range r.Findings {
			fmt.Printf("  %s %s: %s\n", f.Level, r.Extension.RelTag, f.Message)
		}
	}
	// Expect: errors=0, warnings=1 (empty category folder: empty), hasErrors=false.
}
EOF
go run /tmp/check_smoke_test.go
rm -f /tmp/check_smoke_test.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The "unparseable vs missing" distinction — the subtle §9 rule. Verify a
# package extension with a BROKEN package.json yields the ERROR, while a file
# extension with NO package.json yields nothing:
cat > /tmp/check_parse_test.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dabstractor/weave/internal/check"
	"github.com/dabstractor/weave/internal/discover"
)

func main() {
	root, _ := os.MkdirTemp("", "weave-parse")
	defer os.RemoveAll(root)
	// package ext with broken JSON
	os.MkdirAll(filepath.Join(root, "broken"), 0o755)
	os.WriteFile(filepath.Join(root, "broken", "package.json"), []byte("{ not json"), 0o644)
	os.WriteFile(filepath.Join(root, "broken", "index.ts"), []byte("/** x */"), 0o644)
	exts, _ := discover.Index(root)
	rep := check.Check(root, exts)
	for _, r := range rep.ByExt {
		for _, f := range r.Findings {
			fmt.Printf("%s %s: %s\n", f.Level, r.Extension.RelTag, f.Message)
		}
	}
	if rep.HasErrors() {
		fmt.Println("OK: unparseable package.json flagged as ERROR")
	} else {
		fmt.Println("FAIL: unparseable package.json NOT flagged")
	}
}
EOF
go run /tmp/check_parse_test.go
rm -f /tmp/check_parse_test.go
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./internal/check/... -v` — all tests pass.
- [ ] `go test ./internal/discover/...` — still green (the export rename is neutral).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged).

### Feature Validation

- [ ] `Severity`/`String()`/`Finding`/`ExtensionReport`/`Report`/`HasErrors()` ported
      from skilldozer with the noun swaps.
- [ ] `Check(dir, exts) Report` has the `dir` parameter (empty-folder walk needs it).
- [ ] Pass 1 emits the 4 local findings (unparseable ERROR, missing-entryFile ERROR,
      deps-without-node_modules WARN, no-description WARN).
- [ ] Pass 2 emits duplicate-relTag ERRORs (sorted others) and empty-folder WARNs.
- [ ] Pass 3 tallies Errors/Warnings; `HasErrors()` reflects Errors>0.
- [ ] `Check(dir, nil)` is a clean empty report (0/0/0, no panic).
- [ ] `check.Check` does NOT print — it returns a `Report`.
- [ ] `discover.ParsePackageJSON` + `discover.PackageJSON` are exported (rename only).

### Code Quality Validation

- [ ] Type structure ports skilldozer's (logic is rewritten per architecture_mapping §7).
- [ ] Doc comments cite PRD §9 and explain the re-parse rationale.
- [ ] Package layout matches `resolve`/`search` (own package, own `_test.go`,
      white-box same-package tests).
- [ ] The empty-folder walk reuses `discover.Index` (no duplicated resolvability logic).
- [ ] No `main.go` changes (M4.T3.S1 owns the dispatch).

### Documentation & Deployment

- [ ] `Check` has a doc comment explaining the three passes and the `dir` parameter.
- [ ] `Severity`/`Finding`/`ExtensionReport`/`Report` have doc comments (ported from skilldozer).
- [ ] No README/docs changes (the `weave check` CLI usage is documented in README §4
      in the final M6.T4/M6.T5 doc sweep — this task's item desc says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't copy skilldozer's `Check(skills)` signature — weave needs `Check(dir, exts)`
  because the empty-category-folder rule (§9) requires a filesystem walk and
  cannot be derived from `[]Extension` alone.
- ❌ Don't port skilldozer's `checkFields`/`appendDupFindings`/`validName`/
  `nameLenMax`/`descLenMax` — those are Agent-Skills frontmatter/name rules
  weave does NOT have. Rewrite the validation logic per PRD §9.
- ❌ Don't leave `parsePackageJSON`/`packageJSON` unexported and re-implement JSON
  parsing in check — the item description sanctions exporting them, and the
  3-valued return is exactly what the unparseable-vs-missing distinction needs.
- ❌ Don't use `filepath.Dir(ext.EntryFile)` as the package dir for package exts —
  EntryFile is `ext.Path/src/index.ts`, so Dir(EntryFile) != ext.Path. Use
  `ext.Path` for dir/package kinds, `filepath.Dir(ext.Path)` for file kind.
- ❌ Don't fire the deps-without-node_modules WARN for dir-kind extensions —
  PRD §9 scopes it to PACKAGE extensions ("a package extension's package.json
  declares non-empty dependencies").
- ❌ Don't test node_modules existence without `IsDir` — a stray file named
  `node_modules` would falsely satisfy the check.
- ❌ Don't re-implement classifyDir's resolvability cases in check for the
  empty-folder walk — call `discover.Index(child)` and test `len==0`. An
  extension dir yields ≥1 entry; only a plain empty category dir yields 0.
- ❌ Don't print anything from `Check` — it returns a `Report`; main (M4.T3.S1)
  renders it per the §9 output format.
- ❌ Don't wire `main.go`'s `check` dispatch — that is P1.M4.T3.S1. This task
  delivers ONLY the package + the discover export.
- ❌ Don't use the `github.com/dabstractor/skilldozer/...` import path — weave's
  is `github.com/dabstractor/weave/internal/discover`.
- ❌ Don't change `discover`'s behavior in Task 1 — it is a visibility-only rename.
  Existing discover tests must still pass.

---

**Confidence Score: 9/10** for one-pass success. The type structure is a verbatim
port of skilldozer's proven `Severity`/`Finding`/`Report`/`HasErrors()` (with
documented noun swaps); the validation logic is a clean three-pass rewrite whose
rules are pinned by PRD §9 and whose exact finding messages are spelled out in
the item description. The consumed types (`discover.Extension`, `discover.Index`)
are already landed (P1.M2 Complete). The ONE design decision that diverges from
skilldozer — adding the `dir` parameter and exporting `ParsePackageJSON` — is
explicitly sanctioned by the item description ("export it or re-parse in check")
and necessary for the empty-folder rule and the unparseable-vs-missing
distinction. The empty-folder walk reuses `discover.Index` (no duplicated
resolvability logic). The residual risk is the small discover edit (visibility
rename) touching an already-Complete package — mitigated by `go build` catching
every missed call site and `go test ./internal/discover/...` confirming no
behavior change.
