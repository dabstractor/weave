# PRP — P1.M2.T1.S2: JSDoc comment extractor (ExtractJSDoc)

## Goal

**Feature Goal**: Create `internal/discover/jsdoc.go` — the weave replacement for
skilldozer's `ParseFrontmatter`. Implement `func ExtractJSDoc(path string) string` that
reads a `.ts`/`.js` entry file and extracts the leading JSDoc block (`/** ... */`) as a
single-line description string, per PRD §7.3 item 2. This is the metadata fallback source
for single-file extensions (the dominant case) — the closest standard analog to a
`SKILL.md` description. It populates `Extension.Description` (via S1's `BuildExtension`)
when `package.json` has no description, and feeds the `check` "no description" WARN
(indirectly, through `Description == ""`).

**Deliverable**: TWO new files in package `discover`:
- `internal/discover/jsdoc.go` — `func ExtractJSDoc(path string) string`, the
  `utf8BOM` constant (ported from skilldozer), and any package-private line-cleaning
  helper (e.g. `stripStarPrefix`). NO new structs, NO new exported symbols beyond
  `ExtractJSDoc`.
- `internal/discover/jsdoc_test.go` — table-driven / sub-test coverage for every case in
  the item contract (8 cases) plus `/***` rejection and CRLF robustness.

No other files change. go.mod gains NO `require` (stdlib `os`, `bytes`, `strings` only).
The package compiles and `go test ./internal/discover/...` passes. S1's `Extension`,
`packageJSON`, `parsePackageJSON`, `BuildExtension`, `toStringSlice` are ALREADY in
`extension.go` (S1 contract) — this subtask ADDS `ExtractJSDoc` to the same package and is
the producer of the `jsdocDesc` argument that S1's `BuildExtension` already accepts.

**Success Definition**:
- `go build ./...` exits 0 (discover package now has both extension.go AND jsdoc.go).
- `go vet ./...` exits 0 (clean — the new file included).
- `go test ./internal/discover/... -v` passes ALL tests (S1's + S2's).
- `go test -race ./...` passes.
- `ExtractJSDoc` returns `""` on ANY read error (`os.ReadFile` failure) — no panic, no
  error return (signature is `string`, not `(string, error)`).
- A file whose first non-blank line starts with EXACTLY `/**` (and not `/***`) and that
  contains a closing `*/` yields the cleaned, space-joined, trimmed description text.
- A file with `/*` (single star), `//`, code, or `/***` (3+ stars) as its first non-blank
  content returns `""`.
- A file with a leading UTF-8 BOM, leading blank lines, or CRLF endings is handled
  correctly (BOM stripped; blanks skipped; `\r` trimmed).
- `ExtractJSDoc` imports ONLY stdlib (`bytes`, `os`, `strings`); no `gopkg.in/yaml.v3`,
  no internal-package imports.
- go.mod still has no `require` block; no `go.sum`.

## User Persona (if applicable)

**Target User**: weave developers (later subtasks) and, transitively, CLI users. This
subtask is an internal package function with no user-facing surface yet.

**Use Case**: `Index()` (P1.M2.T3) walks the extensions dir; for each entry it calls
`parsePackageJSON(dir)` (S1), reads the JSDoc via `ExtractJSDoc(entryFile)` (THIS subtask),
and calls `BuildExtension(..., pkg, hasPkg, jsdocDesc)` (S1). `BuildExtension` applies the
PRD §7.3 priority: `pkg.Description` wins, else `jsdocDesc`, else `""`. `check` (M4.T2)
then reads `Extension.Description` for the "no description" WARN — it does NOT call
`ExtractJSDoc` directly.

**Pain Points Addressed**: Single-file extensions (the dominant case per PRD §10) have no
`package.json`, so there is no `description` field to read. Without `ExtractJSDoc`,
single-file extensions would always show `(none)` in `--list`. The leading JSDoc block is
the standard, natural metadata home for these files (PRD §7.3.2) and the closest analog to
a `SKILL.md` description. This subtask makes that source available to `BuildExtension`.

## Why

- **Closes the metadata-fallback chain (PRD §7.3)**: S1 shipped `BuildExtension` with a
  `jsdocDesc` PARAMETER but did NOT compute it (see S1 PRP GOTCHA: "S2 owns ExtractJSDoc.
  This subtask takes jsdocDesc as a BuildExtension PARAMETER only."). This subtask produces
  the value that flows into that parameter. Without it, the description fallback chain is
  untestable end-to-end and `Index()` cannot populate `Description` for file extensions.
- **Replaces skilldozer's `ParseFrontmatter` (YAML) with stdlib JSDoc parsing**: skilldozer
  reads `SKILL.md` YAML frontmatter via `gopkg.in/yaml.v3`. Extensions have NO frontmatter
  format (PRD §7.3) — the leading JSDoc `/** ... */` IS the metadata home. weave extracts it
  with pure stdlib (`os`, `bytes`, `strings`), honoring PRD §2 (stdlib-only hard constraint).
  This is the single biggest structural difference from skilldozer's metadata-fallback
  source and the reason NO third-party dep is added.
- **`ExtractJSDoc` returns `string`, NOT `(string, error)`**: the item contract pins this.
  A read failure (file vanished between classify and read, permission, etc.) returns `""`,
  which `BuildExtension` treats as "no JSDoc description" → falls through to `""` total. This
  keeps `Index()`'s walk simple (no error branching for a best-effort metadata source) and
  matches skilldozer's `ParseFrontmatter` only in spirit — `ParseFrontmatter` DOES return an
  error (for malformed YAML), but JSDoc extraction has no "malformed" state worth surfacing:
  either the leading block is a well-formed `/** ... */` (extract) or it isn't (return `""`).
  The §9 check's "unparseable package.json" ERROR applies to JSON, NOT to JSDoc.
- **Bounded, stdlib-only, no new types**: the function is ~40 lines of pure string
  processing. It adds NO structs, NO exported types, and exactly ONE exported function. The
  blast radius is 2 files (jsdoc.go + jsdoc_test.go). It does NOT touch extension.go (S1),
  does NOT walk, does NOT classify, does NOT call `BuildExtension`.

## What

A NEW `internal/discover/jsdoc.go` (`package discover`, NO package-level doc comment — S1's
extension.go already carries the package doc; Go uses the first package comment encountered
and IGNORES subsequent ones, but adding a second is poor form) containing:

1. **`var utf8BOM = []byte{0xEF, 0xBB, 0xBF}`** — ported VERBATIM from skilldozer's
   `discover.go`. The 3-byte UTF-8 BOM. `bytes.TrimPrefix` is a no-op when absent, so it is
   always safe to strip.
2. **`func ExtractJSDoc(path string) string`** — the sole exported symbol. Implements the
   PRD §7.3.2 extraction rule (8 steps; see Implementation Blueprint).
3. **`func stripStarPrefix(line string) string`** (package-private helper, RECOMMENDED but
   optional — may inline) — strips an optional leading run of whitespace, then a single
   `*`, then one optional space, from a JSDoc content line. Extracted for testability and
   readability.

A NEW `internal/discover/jsdoc_test.go` (`package discover`) with sub-test coverage for the
8 contract cases plus 2 robustness cases (`/***` rejection, CRLF).

### Success Criteria

- [ ] `internal/discover/jsdoc.go` exists with `package discover` and EXACTLY the symbols
      above (`utf8BOM`, `ExtractJSDoc`, and optionally `stripStarPrefix`).
- [ ] `ExtractJSDoc(path string) string` — signature is `string` (NOT `(string, error)`).
      Returns `""` on ANY `os.ReadFile` error. Never panics.
- [ ] Step 5 guard is CORRECT: the first non-blank line must start with `/**` AND must NOT
      start with `/***` (jsdoc.fyi: `/***` is not parsed by JSDoc). A `/*` (single star)
      or `//` or code first line → `""`.
- [ ] Step 6: if no closing `*/` is found → `""`.
- [ ] Step 7 per-line cleaning: optional leading whitespace + single `*` + one optional
      space stripped; empty lines dropped; survivors joined with ONE space; final Trim.
- [ ] Step 8 single-line `/** desc */` works via the SAME algorithm (degenerate one-line
      case — no inner `*` lines).
- [ ] BOM stripping via `bytes.TrimPrefix(data, utf8BOM)` (always called; no-op when absent).
- [ ] CRLF robustness: after `strings.Split(data, "\n")`, each line's trailing `\r` is
      trimmed (mirrors skilldozer's `ParseFrontmatter` CRLF handling) so a `\r` never leaks
      into the description.
- [ ] `jsdoc.go` imports ONLY stdlib (`bytes`, `os`, `strings`); NO `gopkg.in/yaml.v3`;
      NO internal-package imports.
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/discover/...`, and
      `go test -race ./...` all pass.
- [ ] go.mod has no `require` block; no go.sum exists.
- [ ] No DUPLICATION of S1 symbols: `utf8BOM` is NOT in extension.go (S1 does not define
      it), so defining it in jsdoc.go is correct. If S1 DID define it (it should not per
      its PRP), DO NOT redefine — reference the existing one. (Verify with grep before
      adding; see Validation Level 1.)

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP names the exact source-of-truth spec (PRD
§7.3.2, quoted in full below), the exact algorithm (8 numbered steps from the item
contract, expanded with the `/***` guard and CRLF handling verified in research), the exact
function to model (skilldozer's `ParseFrontmatter` — same read→BOM-strip→split→find-fences
shape, different fence markers and no YAML), the exact consumer contract (S1's
`BuildExtension` `jsdocDesc` parameter), and the exact test cases (8 from the contract +
2 robustness). The JSDoc `/**`-vs-`/*` distinction and the per-line single-`*` convention
are verified against jsdoc.fyi, jsdoc.app, Biome, and tslint (see research/jsdoc_extraction.md).
The function is pure stdlib string processing — no library docs needed beyond the PRD rule.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec. §7.3 (esp. §7.3.2) defines the JSDoc extraction rule verbatim;
       §7.1 field list shows `description` priority (package.json → JSDoc → ""); §9 lists
       the "no description" WARN that this function indirectly feeds (via Description=="").
  critical: §7.3.2 is the EXACT algorithm. Quoted here for in-PRP reference:
    "Read entryFile, strip a leading UTF-8 BOM if present. Skip leading whitespace and
    blank lines. If the next characters are exactly `/**` (a JSDoc block open), find the
    closing `*/`. Take the lines between. For each line, strip an optional leading run of
    whitespace followed by a single `*` (and one optional space). Drop empty lines. Join
    the survivors with a single space. Trim. That concatenation is the description. If
    there is no closing `*/`, or the first non-whitespace token is not `/**` (e.g. it's
    `//`, a `/*` plain block, or code), no description is extracted. Single-line
    `/** ... */` blocks work (no inner `*` lines → the content between `/**` and `*/`,
    trimmed)."

- file: /home/dustin/projects/skilldozer/internal/discover/discover.go
  why: PRIMARY structural pattern to MODEL (NOT port — the fence markers and the YAML
       unmarshal differ). `ParseFrontmatter` is the skilldozer function this REPLACES.
       Model its shape: os.ReadFile → bytes.TrimPrefix(utf8BOM) → strings.Split("\n") →
       find fences → extract between → return zero-value on missing. Model its CRLF
       handling (strings.TrimRight(line, "\r") before comparison). Model its lenient
       "opening fence but no closing fence → treat as none" behavior (maps to step 6's
       "no `*/` → return empty").
  pattern: |
    var utf8BOM = []byte{0xEF, 0xBB, 0xBF}  // PORT VERBATIM into jsdoc.go

    func ParseFrontmatter(path string) (Frontmatter, string, error) {
        data, err := os.ReadFile(path)
        if err != nil { return Frontmatter{}, "", err }  // weave: return "" (no error)
        data = bytes.TrimPrefix(data, utf8BOM)            // PORT VERBATIM
        lines := strings.Split(string(data), "\n")
        if strings.TrimRight(lines[0], "\r") != "---" {   // CRLF-aware line compare
            return Frontmatter{}, string(data), nil        // no fence → none
        }
        closeIdx := -1
        for i := 1; i < len(lines); i++ {
            if strings.TrimRight(lines[i], "\r") == "---" { closeIdx = i; break }
        }
        if closeIdx == -1 { return Frontmatter{}, string(data), nil } // no closer → none
        // ... extract between, unmarshal ...
    }
  gotcha: ParseFrontmatter returns (Frontmatter, body, error) — THREE values, and surfaces
          a YAML parse error. ExtractJSDoc returns ONE value (string) and NEVER errors. Do
          NOT copy the error-returning shape. The "no fence / no closer → return zero"
          LENIENCY carries over; the "malformed content → error" leniency does NOT (JSDoc
          has no malformed state worth surfacing — see Why section). ParseFrontmatter reads
          SKILL.md (always present for a skill); ExtractJSDoc reads entryFile (a .ts/.js
          file that classify already confirmed exists — but ExtractJSDoc must still handle
          a read error gracefully by returning "").

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M2T1S1/PRP.md
  why: The CONTRACT this subtask feeds. S1's `BuildExtension` signature is
       `BuildExtension(path, entryFile, relTag, kind string, pkg packageJSON, hasPkg bool,
       jsdocDesc string) Extension` — the `jsdocDesc` parameter is THIS subtask's output.
       S1's Description fallback chain (verified in its TestBuildExtensionDescriptionFallback):
       `pkg.Description` (if non-empty) ELSE `jsdocDesc` ELSE `""`. This subtask must NOT
       re-implement that fallback — it returns the raw JSDoc string and lets BuildExtension
       decide. S1 also confirms extension.go does NOT define `utf8BOM` (grep its imports:
       encoding/json, errors, io/fs, os, path/filepath — no `bytes`), so jsdoc.go owning
       `utf8BOM` is correct and non-conflicting.
  critical: The integration point is ONE line in T3's Index(): `jsdocDesc := ExtractJSDoc(
            entryFile)`. Do NOT call BuildExtension from jsdoc.go (that's T3's job). Do NOT
            import extension.go's symbols from jsdoc.go — they're in the SAME package
            (discover), so no import is needed, but jsdoc.go should not REFERENCE them
            either (keep the function standalone: path in → string out).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §3b is the JSDoc extractor spec. It restates the 5-step algorithm and confirms
       "replaces ParseFrontmatter" + "stdlib-only". This PRP refines §3b with the `/***`
       guard (jsdoc.fyi finding) and CRLF handling (skilldozer precedent) that the
       architecture mapping does not mention explicitly.
  critical: §3b steps 1-5 map 1:1 to the item contract's steps 2-8 (architecture_mapping
            collapses BOM+skip into "step 1"). Use the ITEM CONTRACT's 8-step numbering as
            the implementation authority; §3b is corroborating.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M2T1S2/research/jsdoc_extraction.md
  why: The empirical verification of JSDoc semantics. Documents: (A) `/**` is the exact
       marker (jsdoc.fyi: `/*` and `/***` NOT parsed); (B) the per-line single-`*` rule
       (Biome useSingleJsDocAsterisk, tslint jsdoc-format); (C) the 8 edge cases + correct
       behavior; (D) Go pitfalls (bytes.TrimPrefix no-op safety, CRLF, whole-string `*/`
       scan safety given the `/**` precondition); (E) reference impls (skilldozer
       ParseFrontmatter is the closest; Go's go/doc is for `//` comments, not applicable).
  critical: The `/***` guard is the non-obvious finding. `strings.HasPrefix(line, "/**")`
            alone WRONGLY accepts `/***` (since `/***` starts with `/**`). Must add `&&
            !strings.HasPrefix(line, "/***")`. See Implementation Blueprint step 5.

- file: /home/dustin/projects/weave/go.mod  (P1.M1.T1.S1)
  why: Confirms module `github.com/dabstractor/weave`, `go 1.25`. jsdoc.go imports are ALL
       stdlib (bytes, os, strings) — go.mod gains NO require block.
  critical: Adding `gopkg.in/yaml.v3` would violate PRD §2 (stdlib-only) and is WRONG —
            JSDoc is hand-parsed, not YAML. grep must find nothing.
```

### Current Codebase tree

```bash
# After P1.M1 (Complete) + S1 (implementing in parallel) + this subtask (S2).
# S1 ADDS internal/discover/extension.go + extension_test.go (per its PRP — treat as present).
# THIS subtask (S2) ADDS internal/discover/jsdoc.go + jsdoc_test.go to the SAME directory.
$ cd /home/dustin/projects/weave && find . -name '*.go' -not -path './.git/*' | sort
./internal/config/config.go            # P1.M1.T2.S1 (Complete)
./internal/config/config_test.go
./internal/extdir/extdir.go            # P1.M1.T3.S1+S2+S3 (Complete)
./internal/extdir/extdir_test.go
./internal/discover/extension.go       # P1.M2.T1.S1 (parallel — treat as PRESENT per contract)
./internal/discover/extension_test.go  # P1.M2.T1.S1 (parallel — treat as PRESENT per contract)
./main.go                              # P1.M1.T4.S1 (Complete)
./main_test.go
# THIS subtask ADDS:
#   ./internal/discover/jsdoc.go
#   ./internal/discover/jsdoc_test.go
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                    # UNCHANGED
│   ├── extdir/                    # UNCHANGED
│   └── discover/                  # S1 created the directory; this subtask ADDS to it
│       ├── extension.go           # S1 (PRESENT) — Extension, packageJSON, parsePackageJSON,
│       │                          #       BuildExtension, toStringSlice (defines utf8BOM? NO)
│       ├── extension_test.go      # S1 (PRESENT)
│       ├── jsdoc.go               # NEW (this subtask) — utf8BOM, ExtractJSDoc, stripStarPrefix
│       └── jsdoc_test.go          # NEW (this subtask) — ExtractJSDoc table/sub-tests
├── main.go                        # UNCHANGED
├── go.mod                         # UNCHANGED (no new require; all imports stdlib)
└── ...                            # everything else unchanged
# Later subtasks ADD to this directory: T2 adds discover.go (classifyEntry);
# T3 adds index.go (Index — the SOLE caller of ExtractJSDoc outside tests). NOT this subtask.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the /*** guard — the non-obvious finding): strings.HasPrefix(line, "/**") is
// TRUE for "/*** foo */" because "/***" starts with "/**". jsdoc.fyi explicitly states
// "/*** (three or more stars) are not parsed by JSDoc." So step 5 must be:
//     strings.HasPrefix(line, "/**") && !strings.HasPrefix(line, "/***")
// A plain HasPrefix would WRONGLY extract "* foo " from "/*** foo */" (content between
// "/**" and "*/" is "* foo "). The guard rejects the whole block. Verified in research.

// CRITICAL (ExtractJSDoc returns string, NOT (string, error)): the item contract pins the
// signature `ExtractJSDoc(path string) string`. A read error (file missing/perm) → return "".
// Do NOT add an error return "for safety" — T3's Index() calls it inline as
// `jsdocDesc := ExtractJSDoc(entryFile)` and relies on the no-error shape. This DIFFERS from
// skilldozer's ParseFrontmatter (which returns (Frontmatter, body, error)) because JSDoc has
// no "malformed content" state worth surfacing: either the leading block is a well-formed
// /** ... */ (extract) or it isn't (return ""). The §9 "unparseable" ERROR is for JSON, not JSDoc.

// CRITICAL (do NOT call BuildExtension or reference Extension from jsdoc.go): ExtractJSDoc is
// a LEAF function — path in, string out. It does NOT import or reference extension.go's
// symbols (they're in the same package, so no import needed, but don't call them). The
// fallback chain (pkg.Description → jsdocDesc → "") lives in BuildExtension (S1), tested by
// S1's TestBuildExtensionDescriptionFallback. Re-implementing it here duplicates logic and
// blurs the boundary. T3 (Index) is the sole wiring point.

// CRITICAL (BOM constant ownership): utf8BOM is defined in jsdoc.go (THIS subtask), NOT in
// extension.go (S1). S1's imports are encoding/json, errors, io/fs, os, path/filepath — NO
// `bytes`, so it cannot define utf8BOM (it would be an unused import). jsdoc.go imports
// `bytes` for bytes.TrimPrefix, so it owns the constant. If a future subtask needs utf8BOM,
// it references discover.utf8BOM (same package) — do NOT redefine. Verify with grep before
// adding: `grep -n utf8BOM internal/discover/*.go` should find it ONLY in jsdoc.go.

// GOTCHA (CRLF line endings): strings.Split(data, "\n") leaves a trailing "\r" on each line
// of a CRLF file. The per-line "* "-strip rule strips LEADING whitespace then "*" then one
// space — a trailing "\r" is NOT leading, so it would survive into the joined description
// (e.g. "line one\r line two"). FIX: after splitting, trim "\r" from every line via
// strings.TrimRight(line, "\r") (mirrors skilldozer's ParseFrontmatter CRLF handling). This
// is a minor robustness add consistent with skilldozer precedent and the PRD §7.3.2 "trim"
// intent. Alternatively, strings.TrimSpace the FINAL result handles trailing "\r" but not
// interior ones — prefer per-line TrimRight("\r") for correctness.

// GOTCHA (whole-string vs line-based */ scan): the PRD says "scan lines (or scan the string)
// for */". For a LEADING JSDoc (first non-blank line is /**), scanning the whole string with
// strings.Index(s, "*/") finds the FIRST "*/", which IS the JSDoc closer (no code has started
// yet, so no string-literal "*/" false positives before it). This is SAFE given the step-5
// precondition. EITHER approach (whole-string Index, or scan lines for the first line
// containing "*/") is correct. The line-based approach is more explicit and handles the
// multi-line case naturally; the whole-string approach is shorter. Choose ONE and be
// consistent. RECOMMENDED: line-based (find the first line index containing "*/"; if the
// opener line itself contains "*/" it's a single-line block).

// GOTCHA (single-line /** desc */ is a degenerate multi-line): the UNIFIED algorithm
// (split between-content into lines, strip "* " prefix from each, drop empties, join with
// space, trim) handles single-line blocks automatically — the between-content is " desc ",
// split into one line " desc ", strip "* " (no leading "*", so nothing stripped beyond the
// leading space which the rule does NOT strip — CAREFUL), ... SEE Implementation Blueprint
// for the exact single-line handling. The PRD explicitly allows single-line to be handled
// separately ("Single-line ... take content between /** and */, trim") OR by the general
// algorithm. The cleanest implementation handles both uniformly; verify with the
// TestExtractJSDocSingleLine case.

// GOTCHA (do NOT walk, classify, or Index here): T2 owns classifyEntry; T3 owns Index (the
// sole caller of ExtractJSDoc outside tests). jsdoc.go is the LEAF extractor only. Do NOT
// add a WalkDir, do NOT add classify, do NOT add Index. Those are separate files in separate
// subtasks. This keeps the blast radius to 2 files.

// GOTCHA (import minimization): jsdoc.go imports ONLY bytes, os, strings — all stdlib. Do
// NOT import gopkg.in/yaml.v3 (PRD §2 violation). Do NOT import encoding/json (this function
// parses no JSON). Do NOT import any internal/ package (this subtask has no cross-package
// dependency — same package as extension.go, no import needed). An unused import fails
// `go build` ("imported and not used").

// GOTCHA (no package-level doc comment on jsdoc.go): S1's extension.go carries the package
// doc comment (the comment starting "Package discover scans the on-disk extensions/ tree...").
// Go uses the FIRST package-level comment encountered across all files; adding a second
// package comment on jsdoc.go is ignored by godoc but is poor form and may confuse readers.
// jsdoc.go should start with an ORDINARY doc comment on its symbols (ExtractJSDoc), not a
// package comment. (If extension.go is somehow NOT present at merge time — it should be per
// S1 — do not add a package comment here either; flag the conflict instead.)
```

## Implementation Blueprint

### Data models and structure

NO new data models. This subtask adds ONE exported function, ONE package-private helper,
and ONE package-private variable (the BOM constant). All in `package discover` in a NEW
file `internal/discover/jsdoc.go`.

```go
// utf8BOM — ported VERBATIM from skilldozer discover.go. See godoc on ExtractJSDoc.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ExtractJSDoc — see full algorithm below. Returns "" on any read error or non-JSDoc content.
func ExtractJSDoc(path string) string { ... }

// stripStarPrefix — strips optional leading whitespace + single "*" + one optional space.
// Package-private; extracted for readability and direct table-testing.
func stripStarPrefix(line string) string { ... }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY no conflict + CREATE jsdoc.go skeleton
  - RUN: cd /home/dustin/projects/weave && grep -n "utf8BOM\|ExtractJSDoc\|stripStarPrefix" \
        internal/discover/*.go || echo "no conflicts (correct)"
    EXPECTED: "no conflicts" IF S1 landed without these symbols (it should — S1's PRP imports
    no `bytes`). If grep finds utf8BOM in extension.go, DO NOT redefine — reference the existing
    var instead (but this should not happen).
  - CREATE /home/dustin/projects/weave/internal/discover/jsdoc.go with `package discover`.
  - IMPORTS (exact set, alphabetical, ALL stdlib): bytes, os, strings.
  - NO package-level doc comment (S1's extension.go owns it — see GOTCHA).
  - ADD `var utf8BOM = []byte{0xEF, 0xBB, 0xBF}` with a doc comment adapted from skilldozer:
    "utf8BOM is the 3-byte UTF-8 byte-order mark (U+FEFF). Some editors prepend it to .ts/.js
    files; it must be stripped before '/**' detection, otherwise the opener reads as
    '\ufeff/**' and the JSDoc is silently missed. bytes.TrimPrefix is a no-op when the BOM is
    absent, so this is safe for BOM-free files."

Task 2: CREATE jsdoc.go — stripStarPrefix helper
  - ADD `func stripStarPrefix(line string) string`.
  - LOGIC (exact, per PRD §7.3.2 "strip an optional leading run of whitespace followed by a
    single `*` (and one optional space)"):
      // 1. Strip leading whitespace (spaces/tabs).
      s := strings.TrimLeft(line, " \t")
      // 2. Strip a single leading "*" if present.
      if strings.HasPrefix(s, "*") {
          s = s[1:]
      }
      // 3. Strip ONE optional space after the "*".
      if strings.HasPrefix(s, " ") {
          s = s[1:]
      }
      return s
  - DOC COMMENT: explain this is the JSDoc continuation-line cleaner (PRD §7.3.2): strips the
    leading whitespace + single "*" + one optional space that prefixes each line of a
    multi-line JSDoc block (the convention enforced by Biome useSingleJsDocAsterisk and tslint
    jsdoc-format). Note it does NOT strip a trailing "\r" (callers trim CRLF before calling, or
    the final Trim handles it). Note a line with no leading "*" (e.g. the first content line
    after "/**" on the same line) is returned with only leading whitespace + optional space
    stripped.
  - NAMING: stripStarPrefix (package-internal, lowercase).
  - PLACEMENT: after utf8BOM, before ExtractJSDoc.

Task 3: CREATE jsdoc.go — ExtractJSDoc (the core algorithm)
  - ADD `func ExtractJSDoc(path string) string`.
  - DOC COMMENT (comprehensive): document the PRD §7.3.2 algorithm step-by-step; document the
    return-"" cases (read error; first non-blank line not "/**" or is "/***"; no closing "*/");
    document the BOM strip; document CRLF handling; document that the result is the cleaned,
    space-joined, trimmed description (single line, even if the source spanned multiple lines);
    document that this is the metadata FALLBACK source (BuildExtension prefers pkg.Description);
    note the consumer is T3 Index (`jsdocDesc := ExtractJSDoc(entryFile)`).
  - LOGIC (exact, 8 steps from the item contract + the /*** guard + CRLF):
      // Step 1: Read file. On error, return "".
      data, err := os.ReadFile(path)
      if err != nil {
          return ""
      }
      // Step 2: Strip leading UTF-8 BOM (no-op when absent).
      data = bytes.TrimPrefix(data, utf8BOM)
      // Step 3: Convert to string, split into lines.
      lines := strings.Split(string(data), "\n")
      // (CRLF robustness — GOTCHA): trim trailing "\r" from every line.
      for i, l := range lines {
          lines[i] = strings.TrimRight(l, "\r")
      }
      // Step 4: Skip leading whitespace and blank lines. Find the first non-blank line.
      startIdx := -1
      for i, l := range lines {
          if strings.TrimSpace(l) != "" {
              startIdx = i
              break
          }
      }
      if startIdx == -1 {
          return ""  // entirely blank file
      }
      firstLine := lines[startIdx]
      // Step 5: If the first non-blank line does NOT start with exactly "/**" → return "".
      //   (/*** guard: jsdoc.fyi says 3+ stars are NOT JSDoc — reject.)
      if !strings.HasPrefix(firstLine, "/**") || strings.HasPrefix(firstLine, "/***") {
          return ""
      }
      // Step 6: Find the closing "*/". Scan lines from startIdx for the first line
      // containing "*/". If none → return "".
      //   NOTE: the opener line itself may contain "*/" (single-line block) — check it too.
      closeIdx := -1
      for i := startIdx; i < len(lines); i++ {
          if idx := strings.Index(lines[i], "*/"); idx >= 0 {
              closeIdx = i
              break
          }
      }
      if closeIdx == -1 {
          return ""  // unclosed JSDoc
      }
      // Step 7 & 8: Take content between "/**" and "*/". Clean each line. Join. Trim.
      //   Build the raw between-content:
      //     - On the opener line (startIdx): drop the "/**" prefix (and any chars up to "*/"
      //       if the closer is on the same line).
      //     - On the closer line (closeIdx): drop from "*/" onward.
      //     - Middle lines (if any): full line.
      var contentLines []string
      for i := startIdx; i <= closeIdx; i++ {
          line := lines[i]
          if i == startIdx {
              // Strip the "/**" opener. After HasPrefix check, line[3:] is the rest.
              // If the closer is on THIS line (single-line block), also cut at "*/".
              rest := line[3:]  // skip "/**"
              if closeIdx == startIdx {
                  // Single-line: cut at the first "*/" in rest.
                  if c := strings.Index(rest, "*/"); c >= 0 {
                      rest = rest[:c]
                  }
              }
              line = rest
          } else if i == closeIdx {
              // Closer line: cut at the first "*/".
              if c := strings.Index(line, "*/"); c >= 0 {
                  line = line[:c]
              }
          }
          // (For middle lines, line is unchanged — full line.)
          // Clean the line: strip optional leading ws + single "*" + one optional space.
          cleaned := stripStarPrefix(line)
          // Trim surrounding whitespace from the cleaned content for robustness, then drop
          // empty lines.
          cleaned = strings.TrimSpace(cleaned)
          if cleaned != "" {
              contentLines = append(contentLines, cleaned)
          }
      }
      // Join survivors with a single space. Trim the final result.
      return strings.TrimSpace(strings.Join(contentLines, " "))
  - NAMING: ExtractJSDoc (exported — T3 Index calls it; check/ui do not call it directly).
  - PLACEMENT: after stripStarPrefix, last function in jsdoc.go.
  - NOTE on the single-line degenerate case: for "/** desc */" on one line, startIdx==closeIdx,
    the opener branch strips "/**" → " desc */", then cuts at "*/" → " desc ", stripStarPrefix
    finds no leading "*", strips one optional space → "desc ", TrimSpace → "desc". Correct.

Task 4: CREATE jsdoc_test.go — imports + helper
  - CREATE /home/dustin/projects/weave/internal/discover/jsdoc_test.go with `package discover`.
  - IMPORTS: os, path/filepath, testing (all stdlib). (No bytes/strings needed in the test
    file unless a test asserts on them directly.)
  - ADD `func writeFile(t *testing.T, dir, name, content string) string` helper — MkdirAll(dir,
    0o755), WriteFile filepath.Join(dir,name) with content at 0o644, return the full path.
    t.Helper(). This is the weave analog of S1's writePackageJSON and skilldozer's writeSkill;
    define it HERE (do NOT depend on extension_test.go's writePackageJSON — different helper).
  - ADD a BOM-prefixed writer if useful: `func writeFileBOM(t, dir, name, content) string`
    that prepends []byte{0xEF,0xBB,0xBF} to content. OPTIONAL — may inline in the test.

Task 5: CREATE jsdoc_test.go — TestStripStarPrefix (table-driven, OPTIONAL but recommended)
  - ADD `TestStripStarPrefix` with cases:
      {"* foo", "foo"}          // star + space
      {"*foo", "foo"}           // star, no space
      {"*", ""}                 // star only
      {"   * foo", "foo"}       // leading ws + star + space
      {"\t*foo", "foo"}         // tab + star
      {"foo", "foo"}            // no star (passthrough after TrimLeft)
      {"  foo", "foo"}          // leading ws, no star (TrimLeft then no star)
      {"", ""}                  // empty
  - This pins the helper's contract in isolation, making ExtractJSDoc failures easier to
    localize. If you inline stripStarPrefix into ExtractJSDoc, SKIP this test (but the helper
    is recommended).

Task 6: CREATE jsdoc_test.go — TestExtractJSDoc (the 8 contract cases + 2 robustness)
  - ADD `TestExtractJSDoc` as a table-driven test (or t.Run sub-tests). Each case writes a temp
    file and asserts ExtractJSDoc(path) == want. Cases (from the item contract TEST list):
      1. "multiline": "/**\n * Line one.\n * Line two.\n */\ncode();\n" → "Line one. Line two."
      2. "single-line": "/** desc */\ncode();\n" → "desc"
      3. "no-jsdoc-code": "export function f() {}\n" → ""
      4. "line-comment": "// a comment\ncode();\n" → ""
      5. "plain-block": "/* plain block */\ncode();\n" → ""
      6. "bom-prefixed": BOM + "/**\n * desc\n */\n" → "desc"
      7. "leading-blanks": "\n\n  \n/**\n * desc\n */\n" → "desc"
      8. "unclosed": "/**\n * desc\n" (no "*/") → ""
      9. "triple-star": "/***\n * desc\n */\n" → "" (the /*** guard — GOTCHA)
     10. "crlf": "/**\r\n * desc\r\n */\r\n" → "desc" (CRLF robustness — GOTCHA)
  - ADD edge cases for robustness:
      11. "single-line-no-space": "/**desc*/\n" → "desc" (no spaces inside)
      12. "missing-file": call ExtractJSDoc on a path that does not exist → "" (no panic)
      13. "empty-file": "" → ""
      14. "jsdoc-with-blank-inner-line": "/**\n * a\n *\n * b\n */\n" → "a b" (blank star-only
          line dropped)
  - Use a temp dir (t.TempDir()) per sub-test or per case; clean up is automatic.
  - ASSERT with a small `strEq`-style helper OR direct == compare (these are string==string).

Task 7: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...                    # expect exit 0
  - RUN: go vet ./...                                                        # expect exit 0, clean
  - RUN: go test ./internal/discover/... -v                                  # expect ALL PASS (S1+S2)
  - RUN: go test ./... -v                                                    # expect ALL PASS
  - RUN: go test -race ./...                                                 # expect no data races
  - RUN: grep -rn "yaml.v3\|gopkg.in" --include=*.go ./internal/discover/    # expect nothing
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK                   # expect OK
  - RUN: test ! -f go.sum && echo OK || echo FAIL                            # expect OK
  - RUN: grep -c "utf8BOM" internal/discover/jsdoc.go                        # expect >=1
  - RUN: grep -rn "utf8BOM" internal/discover/*.go | wc -l                   # expect defines ONCE
  - RUN: ! grep -qE 'ParseFrontmatter|HasFM|Frontmatter|yaml' \
        ./internal/discover/jsdoc.go && echo "no skilldozer leftovers" || echo "FAIL"
    (allow nothing skilldozer-flavored in jsdoc.go)
  - EXPECT: build clean, vet clean, all tests pass, no yaml import, no require, no go.sum,
    utf8BOM defined exactly once, ExtractJSDoc present and correct.
```

### Implementation Patterns & Key Details

```go
// utf8BOM — PORT VERBATIM from skilldozer discover.go.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripStarPrefix — the JSDoc continuation-line cleaner (PRD §7.3.2).
func stripStarPrefix(line string) string {
	s := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(s, "*") {
		s = s[1:]
	}
	if strings.HasPrefix(s, " ") {
		s = s[1:]
	}
	return s
}

// ExtractJSDoc reads the .ts/.js file at path and extracts the leading JSDoc block
// ("/** ... */") as a single-line description (PRD §7.3.2). Returns "" when:
//   - the file cannot be read (any os.ReadFile error);
//   - the first non-blank line is not a "/**" JSDoc opener (e.g. "//", "/*" plain block,
//     "/***" 3+ stars, or code);
//   - the opener has no closing "*/".
//
// The BOM is stripped, CRLF "\r" is trimmed per line, and each content line is cleaned
// (optional leading ws + single "*" + one optional space). Survivors are joined with one
// space and the result is trimmed. Single-line "/** desc */" blocks work (degenerate case).
//
// This is the metadata FALLBACK source: BuildExtension (extension.go, S1) prefers a
// non-empty package.json description; only when that is absent does jsdocDesc win. T3 Index
// is the sole caller: `jsdocDesc := ExtractJSDoc(entryFile)`.
func ExtractJSDoc(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	data = bytes.TrimPrefix(data, utf8BOM)

	lines := strings.Split(string(data), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r") // CRLF robustness
	}

	// Step 4: find first non-blank line.
	startIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return ""
	}

	// Step 5: must start with "/**" and NOT "/***" (jsdoc.fyi: 3+ stars not parsed).
	first := lines[startIdx]
	if !strings.HasPrefix(first, "/**") || strings.HasPrefix(first, "/***") {
		return ""
	}

	// Step 6: find closing "*/" (scan from the opener line; single-line blocks close inline).
	closeIdx := -1
	for i := startIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "*/") {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return ""
	}

	// Steps 7-8: collect cleaned content lines between "/**" and "*/".
	var parts []string
	for i := startIdx; i <= closeIdx; i++ {
		line := lines[i]
		switch {
		case i == startIdx && i == closeIdx:
			// Single-line block: strip "/**" prefix, then cut at first "*/".
			rest := strings.TrimPrefix(line, "/**")
			if c := strings.Index(rest, "*/"); c >= 0 {
				rest = rest[:c]
			}
			line = rest
		case i == startIdx:
			line = strings.TrimPrefix(line, "/**")
		case i == closeIdx:
			if c := strings.Index(line, "*/"); c >= 0 {
				line = line[:c]
			}
		}
		if cleaned := strings.TrimSpace(stripStarPrefix(line)); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// GOTCHA (/*** guard): HasPrefix(first, "/**") is TRUE for "/***". The extra
// !HasPrefix(first, "/***") rejects 3+ star openers per jsdoc.fyi. Without it, "/*** x */"
// would extract "* x". Verified in research/jsdoc_extraction.md.

// GOTCHA (CRLF): per-line TrimRight("\r") before processing prevents a stray "\r" from
// leaking into the joined description. Mirrors skilldozer's ParseFrontmatter CRLF handling.

// GOTCHA (single-line is the i==startIdx && i==closeIdx case): handled explicitly to avoid
// double-stripping. The general middle-line and closer-line branches cover multi-line.

// GOTCHA (whole-string */ scan is safe here): because step 5 guarantees the first non-blank
// line is "/**", the first "*/" found by Contains must be the JSDoc closer — no code precedes
// it, so no string-literal "*/" false positives. A line-based Contains scan (above) is the
// explicit form; strings.Index on the whole string would also work.
```

### Integration Points

```yaml
DATABASE:
  - none. jsdoc.go reads the filesystem (os.ReadFile) only; no DB.

CONFIG:
  - none. jsdoc.go does NOT read the weave config or env vars. The entry FILE path is
    passed to ExtractJSDoc by T3 (Index), which gets it from the classify step (T2).

ROUTES / API:
  - none. discover is an internal package. ExtractJSDoc is consumed by:
      * T3 Index() (P1.M2.T3) — the SOLE non-test caller:
          jsdocDesc := ExtractJSDoc(entryFile)
          ext := BuildExtension(path, entryFile, relTag, kind, pkg, hasPkg, jsdocDesc)
      * M4.T2 check — INDIRECTLY only, via Extension.Description (already computed by
        BuildExtension). check does NOT call ExtractJSDoc.
    This subtask's symbol is consumed by exactly ONE caller (T3) outside tests.

MODULE:
  - jsdoc.go imports ONLY stdlib (bytes, os, strings). go.mod gains NO require block; no
    go.sum. The discover package has NO internal-package dependency (jsdoc.go and
    extension.go are in the SAME package, so no import is needed to reference each other's
    symbols — though jsdoc.go intentionally does NOT reference extension.go's symbols).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave

# Format check.
gofmt -l internal/discover/jsdoc.go internal/discover/jsdoc_test.go
# EXPECTED: no output. If a file is listed, run `gofmt -w` on it and re-check.

# go vet (whole repo — the new discover file included).
go vet ./...
# EXPECTED: exit 0, no output.

# No yaml.v3 leaked in (PRD §2 stdlib-only hard constraint).
grep -rn "yaml.v3\|gopkg.in" --include=*.go ./internal/discover/ || echo "no yaml import (correct)"
# EXPECTED: "no yaml import (correct)".

# No require block / no go.sum (all imports stdlib).
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum appeared"
# EXPECTED: both "correct".

# utf8BOM defined exactly ONCE in the discover package (not duplicated in extension.go).
test "$(grep -rn 'utf8BOM = ' internal/discover/*.go | wc -l)" = "1" && echo "utf8BOM defined once" \
  || echo "FAIL: utf8BOM count != 1"
# EXPECTED: "utf8BOM defined once".

# No skilldozer leftovers (ParseFrontmatter/HasFM/Frontmatter/yaml must not appear in jsdoc.go).
! grep -qE 'ParseFrontmatter|HasFM|Frontmatter|yaml\.v3|skilldozer' ./internal/discover/jsdoc.go \
  && echo "no skilldozer leftovers (correct)" || echo "FAIL: skilldozer leftover in jsdoc.go"
# EXPECTED: "no skilldozer leftovers (correct)".

# The exported symbol is present.
grep -q 'func ExtractJSDoc(path string) string' internal/discover/jsdoc.go && echo "ExtractJSDoc present" \
  || echo "FAIL: ExtractJSDoc signature wrong/missing"
# EXPECTED: "ExtractJSDoc present".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the discover package tests verbosely (S1's + S2's).
go test ./internal/discover/... -v
# EXPECTED: ALL tests pass. Pay special attention to S2's:
#   - TestStripStarPrefix (8 sub-cases — pins the helper contract)
#   - TestExtractJSDoc:
#       1. multiline            → "Line one. Line two."
#       2. single-line          → "desc"
#       3. no-jsdoc-code        → ""
#       4. line-comment         → ""
#       5. plain-block (/*)     → ""  ← the single-star rejection
#       6. bom-prefixed         → "desc"
#       7. leading-blanks       → "desc"
#       8. unclosed (no */)     → ""
#       9. triple-star (***)    → ""  ← the /*** guard (GOTCHA)
#      10. crlf                 → "desc"
#      11. single-line-no-space → "desc"
#      12. missing-file         → ""  (no panic)
#      13. empty-file           → ""
#      14. jsdoc-blank-inner    → "a b"
# If any fail, read the failure and fix jsdoc.go (NOT the test — tests encode the contract).

# Whole-repo test suite (config + extdir + main + discover).
go test ./... -v
# EXPECTED: ALL tests pass.

# Race detector sanity.
go test -race ./...
# EXPECTED: passes, no data races.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (config + extdir + main + discover must all compile — S1's extension.go
# AND this subtask's jsdoc.go in the same package).
go build ./...
# EXPECTED: exit 0.

# Integration: prove ExtractJSDoc's output flows correctly through BuildExtension's fallback
# chain (the S1 contract this subtask feeds). This is an in-package test (DiscoverTest) so it
# can see both unexported parsePackageJSON AND exported ExtractJSDoc/BuildExtension.
# Add it to jsdoc_test.go OR a separate integration_test.go in package discover:
cat > /tmp/jsdoc_integration_check.go <<'EOF'
// (illustrative — actual test lives in internal/discover/)
// Verifies: a file extension (no package.json) with a leading JSDoc gets Description from JSDoc.
func TestExtractJSDoc_FeedsBuildExtensionFallback(t *testing.T) {
    dir := t.TempDir()
    entry := filepath.Join(dir, "gate.ts")
    os.WriteFile(entry, []byte("/**\n * Gate function.\n * Second line.\n */\nexport function gate() {}\n"), 0o644)
    jsdoc := ExtractJSDoc(entry)                       // S2 (this subtask)
    ext := BuildExtension(entry, entry, "gate", "file", packageJSON{}, false, jsdoc)  // S1
    // ext.Description == "Gate function. Second line." (JSDoc, since no package.json desc)
    // ext.HasPackageJSON == false
}
EOF
# Run the real test (add it to jsdoc_test.go as TestExtractJSDocFeedsBuildExtensionFallback):
go test ./internal/discover/ -run TestExtractJSDocFeedsBuildExtensionFallback -v
# EXPECTED: PASS. This proves the S1+S2 contract: ExtractJSDoc's string feeds BuildExtension's
# jsdocDesc parameter, and the fallback (no pkg.Description → jsdocDesc wins) works end-to-end.
rm -f /tmp/jsdoc_integration_check.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove the /*** guard and the /* single-star rejection hold against the
# REALISTIC single-file extension shapes from PRD §10/§11. This mirrors real-world .ts files.
go test ./internal/discover/ -run 'TestExtractJSDoc' -v -count=1
# EXPECTED: PASS. The triple-star (***) and plain-block (/*) cases in particular prove the
# jsdoc.fyi-verified boundary: only "/**" (exactly two stars) is a JSDoc block.

# Verify the exported API surface is exactly ExtractJSDoc (no accidental extra exports).
go doc ./internal/discover
# EXPECTED: the exported symbols from THIS subtask are just ExtractJSDoc (plus S1's Extension
# and BuildExtension). utf8BOM and stripStarPrefix are NOT listed (unexported). This is the
# intended API surface.

# Verify no DUPLICATE of the package doc comment (S1's extension.go owns it; jsdoc.go must not
# repeat it). godoc would show the extension.go comment; jsdoc.go should have only symbol docs.
grep -c '^// Package discover' internal/discover/jsdoc.go
# EXPECTED: 0 (jsdoc.go has NO package-level doc comment — S1's extension.go owns it).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./... -v` (config + extdir + main + discover — S1 + S2).
- [ ] No linting/vet errors: `go vet ./...` exits 0.
- [ ] No formatting issues: `gofmt -l internal/discover/` lists nothing.
- [ ] No yaml.v3 dependency: `grep -rn yaml.v3 ./internal/discover/` finds nothing.
- [ ] No require block in go.mod; no go.sum.

### Feature Validation

- [ ] All success criteria from "What" section met (ExtractJSDoc signature `string`; 8-step
      algorithm; `/***` guard; CRLF handling; BOM strip; single-line degenerate case).
- [ ] Manual/ad-hoc testing successful: TestExtractJSDocFeedsBuildExtensionFallback passes
      (Level 3 — proves the S1+S2 contract end-to-end).
- [ ] Error cases handled per contract: read error → ""; not-`/**` → ""; `/***` → ""; no `*/`
      → "".
- [ ] Integration point ready: ExtractJSDoc is the exact symbol T3 (Index) will call
      (`jsdocDesc := ExtractJSDoc(entryFile)`); its output feeds S1's BuildExtension
      `jsdocDesc` parameter, matching the fallback chain S1 already tests.
- [ ] PRD §7.3.2 extraction rule honored verbatim (all 8 steps + the jsdoc.fyi-verified `/***`
      boundary).

### Code Quality Validation

- [ ] Follows existing codebase patterns: mirrors skilldozer ParseFrontmatter's shape
      (read→BOM-strip→split→find-fences→extract→return "" on missing); ports utf8BOM verbatim;
      handles CRLF like skilldozer does.
- [ ] File placement matches desired codebase tree (internal/discover/jsdoc.go + jsdoc_test.go).
- [ ] Anti-patterns avoided (no yaml.v3; no error return on ExtractJSDoc; no BuildExtension
      call from jsdoc.go; no walk/classify/Index; no duplicate package doc comment; no
      duplicate utf8BOM).
- [ ] Dependencies properly managed: stdlib only (bytes, os, strings); no internal-package
      imports.
- [ ] Doc comments on every exported symbol (ExtractJSDoc) + the non-obvious unexported ones
      (utf8BOM's no-op-when-absent note; stripStarPrefix's convention citation).

### Documentation & Deployment

- [ ] Code is self-documenting: ExtractJSDoc's doc comment explains the 8-step algorithm, the
      return-"" cases, the BOM/CRLF handling, and the fallback-source role (BuildExtension
      prefers pkg.Description).
- [ ] No new environment variables (this subtask reads none).
- [ ] The PRP's research note (jsdoc_extraction.md) is referenced from ExtractJSDoc's doc
      comment (or the stripStarPrefix comment) so future maintainers understand the `/***`
      guard and CRLF decisions.

---

## Anti-Patterns to Avoid

- ❌ Don't return `(string, error)` from `ExtractJSDoc` — the item contract pins the signature
  as `ExtractJSDoc(path string) string`. A read error → `""`. T3 calls it inline and relies on
  the no-error shape. This DIFFERS from skilldozer's ParseFrontmatter (which errors on
  malformed YAML) because JSDoc has no "malformed" state worth surfacing.
- ❌ Don't use `strings.HasPrefix(line, "/**")` ALONE as the opener check — it WRONGLY accepts
  `/***` (3+ stars), which jsdoc.fyi explicitly says is NOT a JSDoc block. Add `&&
  !strings.HasPrefix(line, "/***")`. Verified in research.
- ❌ Don't import `gopkg.in/yaml.v3` — JSDoc is hand-parsed with stdlib (`bytes`, `os`,
  `strings`). PRD §2 forbids third-party deps for this; skilldozer's yaml import does NOT
  carry over. JSDoc is NOT YAML.
- ❌ Don't call `BuildExtension` (or reference `Extension`/`packageJSON`) from `jsdoc.go` —
  ExtractJSDoc is a LEAF function (path in, string out). The fallback chain lives in S1's
  BuildExtension; T3 (Index) is the sole wiring point. Re-implementing the fallback here
  duplicates logic and blurs the subtask boundary.
- ❌ Don't redefine `utf8BOM` if S1's extension.go already defines it (it should NOT — S1's
  imports have no `bytes`). Verify with grep before adding. If somehow present, reference the
  existing var; do not create a duplicate (Go forbids redeclaration in the same package).
- ❌ Don't add a package-level doc comment to jsdoc.go — S1's extension.go owns the package
  doc. A second package comment is ignored by godoc but is poor form.
- ❌ Don't add walk/classify/Index logic here — T2 (classifyEntry) and T3 (Index) are separate
  subtasks with separate files. This subtask is the LEAF extractor only.
- ❌ Don't forget CRLF — `strings.Split(data, "\n")` leaves trailing `\r` on each line. Trim
  `\r` per line (mirrors skilldozer's ParseFrontmatter) or the description gets stray `\r`s.
- ❌ Don't scan for `*/` BEFORE confirming the first non-blank line is `/**` — the
  "first `*/` is the JSDoc closer" safety DEPENDS on the step-5 precondition (no code before
  the opener). Scanning a code-first file for `*/` could find a string-literal closer and
  produce garbage. Always run step 5 (the `/**` guard) BEFORE step 6 (the `*/` scan).
- ❌ Don't catch all exceptions / over-engineer — `ExtractJSDoc` returns `""` for EVERYTHING
  that isn't a clean leading `/** ... */` block. Do NOT add special cases for nested JSDoc,
  trailing JSDoc, or mid-file JSDoc (PRD §7.3.2 is explicit: LEADING only).
