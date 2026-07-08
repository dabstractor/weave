# PRP — P1.M3.T1.S1: Fix `ExtractJSDoc` single-line branch for degenerate `/**/` overlap

## Goal

**Feature Goal**: Fix Bug 2 (Issue 2, Minor) — `ExtractJSDoc`
(`internal/discover/jsdoc.go`) returns `"/"` as the description for a file whose
leading JSDoc block is the degenerate empty single-line form `/**/` (opener and
closer overlap at index 2). The correct result is `""` (PRD §7.3.2: an empty
JSDoc block yields no description, rendered as `(none)` in `--list`). Replace the
single-line branch's opener-then-closer extraction with a position-based
extraction on the ORIGINAL line that finds `*/` first, so the opener/closer
overlap is handled correctly.

**Deliverable**: ONE localized code edit + ONE comment rewrite + TWO test cases:
- `internal/discover/jsdoc.go` — replace the `case i == startIdx && i == closeIdx`
  branch body (~lines 135–141) with position-based extraction; rewrite the branch
  comment to explain the overlap rationale ([Mode A] doc ride-along).
- `internal/discover/jsdoc_test.go` — add `{"empty-block-degenerate", "/**/\n", ""}`
  (the regression test for this bug) and `{"empty-block-space", "/** */\n", ""}`
  (regression guard for the already-working space variant) to the
  `TestExtractJSDoc` cases slice.

NO other files. NO import change (`strings` already imported). NO signature
change. NO new deps.

**Success Definition**:
- `ExtractJSDoc` returns `""` for a file containing exactly `/**/\n` (was `"/"`).
- All 14 existing `TestExtractJSDoc` cases continue to pass UNCHANGED (including
  `single-line` → `"desc"` and `single-line-no-space` → `"desc"`).
- `go build ./...`, `go vet ./...`, `go test ./internal/discover/... -v`, and
  `go test -race ./...` all exit clean.
- The PRD §7.3.2 worked example holds: a degenerate empty single-line block is a
  valid empty JSDoc; its description is `""`, so `--list` renders `(none)` and
  `--search` does not index a stray `/`.

## User Persona (if applicable)

**Target User**: `weave` CLI users whose extension files happen to begin with a
degenerate empty `/**/` JSDoc (a common editor/templating artifact — e.g. a
`/**/` placeholder left by a snippet expander). Indirectly: anyone running
`weave --list` / `weave --search`, whose output would otherwise be polluted by a
bogus `/` description.

**Use Case**: A user has a `.ts` extension file starting with `/**/` (an empty
JSDoc stub). Today `weave --list` shows its DESCRIPTION column as `/` (a stray
slash) and `--search /` would match it. After the fix, it shows `(none)` —
indistinguishable from any other metadata-less extension, exactly as PRD §7.3.2
intends.

**User Journey**:
1. File `extensions/empty.ts` begins with exactly `/**/\n`.
2. `discover.Index` walks it; `classifyEntry` sees a `*.ts` single-file entry;
   `BuildExtension` calls `ExtractJSDoc(entryFile)` for the description fallback.
3. `ExtractJSDoc` reaches the single-line branch (`line == "/**/"`). **After the
   fix:** finds `*/` at index 2 on the original line; `2 <= 3` → overlap →
   `line = ""` → returns `""`.
4. `BuildExtension` sets `Description = ""` (no package.json, JSDoc fallback empty).
5. `weave --list` renders the row's DESCRIPTION as `(none)`.

**Pain Points Addressed**:
- **Catalog pollution**: a stray `/` in `--list` DESCRIPTION is confusing and
  looks like a bug to users.
- **Search noise**: `--search /` (or any substring search containing `/`) would
  spuriously match these degenerate-empty extensions.
- **Spec fidelity**: PRD §7.3.2 explicitly says an empty JSDoc yields no
  description; the current behavior violates that for this one degenerate form.

## Why

- **PRD §7.3.2 contract**: "If neither [package.json description nor JSDoc]
  yields a description, `description = ""` (rendered as `(none)` in `--list`)." A
  `/**/` block is empty; it must yield `""`. Returning `"/"` is a spec violation.
- **The bug is narrow and the fix is narrow**: only the single-line branch
  misbehaves, and only when `*/` starts at index ≤ 3 (opener/closer overlap). The
  only degenerate input is `/**/`. All other empty forms (`/** */`, `/**\n*/`,
  `/***/`) are already correct. This is a surgical fix, not a rewrite.
- **Position-based extraction is the correct shape**: the root cause is that
  `TrimPrefix(line, "/**")` destroys the closer's `*` when opener and closer
  overlap. Finding `*/` on the ORIGINAL line (before any prefix stripping) avoids
  the overlap entirely. The item CONTRACT specifies this exact approach; the
  `research/bug2_overlap.md` traces verify it against all four single-line forms.
- **Doc ride-along [Mode A]**: the current 1-line branch comment ("strip `/**`
  prefix, then cut at first `*/`") describes the BROKEN approach. Rewriting it to
  explain WHY we find `*/` first (opener/closer overlap in degenerate blocks)
  prevents a future maintainer from "simplifying" it back to the bug.
- **No blast radius**: `ExtractJSDoc` is a pure function (`path → string`); the
  fix changes its output for exactly one input shape. No caller (`BuildExtension`,
  `Index`) changes. P1.M2.T3.S1 (the parallel item) edits `index.go` only — no
  overlap with this `jsdoc.go` edit.

## What

### The current (broken) single-line branch — `internal/discover/jsdoc.go`, ~lines 135–141

```go
case i == startIdx && i == closeIdx:
    // Single-line block: strip "/**" prefix, then cut at first "*/".
    rest := strings.TrimPrefix(line, "/**")
    if c := strings.Index(rest, "*/"); c >= 0 {
        rest = rest[:c]
    }
    line = rest
```
For `line == "/**/"`: `TrimPrefix("/**/", "/**") == "/"`, `Index("/", "*/") ==
-1`, so `rest` stays `"/"` and `line = "/"`. Wrong.

### The fixed single-line branch (item CONTRACT, position-based on original line)

```go
case i == startIdx && i == closeIdx:
    // Single-line block: find "*/" on the ORIGINAL line BEFORE extracting
    // content, so the opener/closer overlap in degenerate blocks (e.g. "/**/"
    // has "*/" at index 2, within the opener's first 3 chars) is handled. For
    // "/**/" the closer sits inside the opener "/**" -> empty content.
    c := strings.Index(line, "*/")
    if c <= 3 {
        // "*/" starts at or before the opener "/**" ends (index 3 exclusive)
        // -> opener and closer overlap -> empty content.
        line = ""
    } else {
        line = line[3:c] // content strictly between "/**" and "*/"
    }
```

The result then flows through the UNCHANGED tail of the loop:
`stripStarPrefix(line)` → `strings.TrimSpace(...)` → dropped if empty.

### Trace verification (ASCII; byte indices match Go string indexing)

| input `line`  | `c`=`Index(line,"*/")` | branch        | `line` after | `stripStarPrefix` | `TrimSpace` | output |
|---------------|------------------------|---------------|--------------|-------------------|-------------|--------|
| `/**/`        | 2                      | `c<=3` → `""` | `""`         | `""`              | `""`        | `""`   |
| `/** desc */` | 9                      | `c>3` → `[3:9]` | `" desc "` | `" desc "`→`"desc "` | `"desc"` | `"desc"` |
| `/**desc*/`   | 7                      | `c>3` → `[3:7]` | `"desc"`   | `"desc"`          | `"desc"`    | `"desc"` |
| `/** */`      | 4                      | `c>3` → `[3:4]` | `" "`      | `""` (TrimLeft)   | `""`        | `""`   |

All four are correct. The two existing single-line test cases (`single-line` →
`"desc"`, `single-line-no-space` → `"desc"`) continue to pass.

### Bounds safety of `line[3:c]`
`c` comes from `strings.Index(line, "*/")`, so `c+2 <= len(line)`. The `else`
branch requires `c > 3`, i.e. `3 < c`, so `line[3:c]` is a valid slice with
`3 < c <= len(line)`. No panic possible.

### Success Criteria

- [ ] `ExtractJSDoc` returns `""` for `/**/\n` (regression test
      `empty-block-degenerate` passes; was `"/"`).
- [ ] `ExtractJSDoc` returns `""` for `/** */\n` (regression guard
      `empty-block-space`; already worked, must not regress).
- [ ] All 14 pre-existing `TestExtractJSDoc` cases pass unchanged.
- [ ] `TestExtractJSDocFeedsBuildExtensionFallback` still passes (the fix does
      not affect multi-line extraction).
- [ ] `go build ./...` && `go vet ./...` && `go test ./internal/discover/... -v`
      && `go test -race ./...` all clean.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the exact file, the exact branch
(with current line numbers), the exact broken code, the exact replacement code
(the item CONTRACT, byte-for-byte), the trace proof for all four single-line
forms, the exact test cases to add (with names and content), and the validation
commands are all specified. No guessing; no exploration required.

### Documentation & References

```yaml
# MUST READ — the file containing the bug
- file: internal/discover/jsdoc.go
  why: Contains ExtractJSDoc and the broken single-line branch (~lines 135-141).
       Read the full ExtractJSDoc doc comment (steps 1-8) to understand the
       algorithm and confirm the single-line branch is the ONLY change point.
  pattern: steps 1-8 algorithm; the loop at the bottom iterates startIdx..closeIdx
           with a switch on {single-line, opener-line, closer-line, middle}.
  gotcha: the SINGLE-LINE case is `i == startIdx && i == closeIdx`. The opener-line
          (`i == startIdx`) and closer-line (`i == closeIdx`) multi-line branches
          are NOT affected by this bug — do NOT change them. Step 5's `/***`
          triple-star guard is also correct — do NOT change it.

# MUST READ — the test file to extend
- file: internal/discover/jsdoc_test.go
  why: Contains TestExtractJSDoc (table-driven, ~14 cases) and the integration
       test TestExtractJSDocFeedsBuildExtensionFallback. Add the two new cases
       to the `cases` slice; do NOT renumber or remove existing cases.
  pattern: each case is {name, content, want}; the test writes content to a
           TempDir .ts file and asserts ExtractJSDoc(path) == want. BOM case is
           handled specially in the loop body — leave that logic alone.
  gotcha: the case NAME doubles as the temp filename (name+".ts"), so names must
          be filesystem-safe and unique within the table. "empty-block-degenerate"
          and "empty-block-space" satisfy both.

# The bug report (authoritative expected/actual + repro)
- docfile: PRD.md   # or the bugfix PRD h2.3/h3.1 section
  section: "Issue 2: Empty single-line JSDoc /**/ extracts / as the description"
  why: confirms expected="" (rendered "(none)"), actual="/" (shown in --list and
       indexed by --search), and the repro (weave_EXTENSIONS_DIR=/tmp/bug2 ...).
  critical: the repro shows the user-visible symptom (DESCRIPTION column = "/"
            instead of "(none)"); the fix must make it "(none)".

# The contract (the exact fix code + traces)
- item_description: (this work item's CONTRACT DEFINITION, points 3 & 6)
  why: gives the byte-for-byte replacement branch and the exact test row to add.
  critical: use `c <= 3` (NOT `c < 3`) and `line[3:c]` exactly as specified; the
            traces in research/bug2_overlap.md prove these constants are correct.

# Research notes (full trace table + scope boundaries)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M3T1S1/research/bug2_overlap.md
  why: the verified trace table for all four single-line forms, the root-cause
       analysis (index-2 overlap), and the explicit "do NOT touch" list.

# Architecture fix design (exploratory — read ONLY for root-cause confirmation)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md
  section: "Bug 2 (Minor): ExtractJSDoc degenerate empty /**/ block"
  why: confirms the root cause (TrimPrefix destroys the closer's * at the
       overlap). NOTE: its "Fixed code" attempts are EXPLORATORY and one is
       marked "STILL WRONG" by its own author — DO NOT copy them. Use the
       item-CONTRACT position-based fix instead (it is the settled solution).
  gotcha: the fix_design.md doc literally says "Wait - that doesn't work. Let me
          reconsider." mid-stream. Trust the item CONTRACT, not fix_design.md's
          intermediate attempts.

# PRD spec (the §7.3.2 rule this enforces)
- docfile: PRD.md
  section: §7.3.2 (Metadata extraction — leading JSDoc comment)
  why: "Single-line /** ... */ blocks work" and "If neither yields a description,
       description = "" (rendered as (none) in --list)". An empty /**/ yields "".
```

### Current Codebase tree (relevant subset)

```bash
internal/discover/
├── jsdoc.go           # ExtractJSDoc + stripStarPrefix — THE file to edit (single-line branch)
├── jsdoc_test.go      # TestExtractJSDoc + integration test — ADD two cases
├── extension.go       # BuildExtension (caller of ExtractJSDoc) — NOT changed
├── discover.go        # classifyEntry — NOT changed
└── index.go           # Index walk — touched by P1.M2.T3.S1 (parallel), NOT this task
```

### Desired Codebase tree with files to be added/changed

```bash
internal/discover/
├── jsdoc.go           # MODIFIED — single-line branch body + comment rewritten
└── jsdoc_test.go      # MODIFIED — +2 rows in TestExtractJSDoc cases slice
# NO new files. NO other files changed.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the bug is ONLY in the single-line branch (case i == startIdx && i == closeIdx).
// Do NOT touch the opener-line (case i == startIdx), closer-line (case i == closeIdx),
// or middle-line branches. Do NOT touch step 5's "/***" guard (it is correct).

// CRITICAL: use `c <= 3` (inclusive of 3), NOT `c < 3`. The opener "/**" spans
// indices 0,1,2 (length 3, exclusive end = index 3). A closer at index <= 3
// overlaps or abuts the opener -> empty content. (Index 3 is effectively
// unreachable for a valid block since "/**" + "*" at index 3 = "/***", rejected
// at step 5; so `c <= 3` collapses to `c == 2` in practice, but the inclusive
// bound is the faithful encoding of the contract.)

// CRITICAL: `line[3:c]` is safe ONLY because c > 3 in the else branch and c comes
// from Index(line, "*/") (so c+2 <= len(line)). Do NOT change the branch order
// or the comparison.

// CRITICAL: do NOT strip the "/**" prefix with TrimPrefix and then operate on the
// remainder — THAT IS THE BUG. The closer's "*" at index 2 is consumed by the
// opener's TrimPrefix, leaving "/". Find "*/" on the ORIGINAL line first.

// GOTCHA: the test case NAME is used as the temp filename (name+".ts"). Keep
// names filesystem-safe and unique. "empty-block-degenerate" / "empty-block-space" OK.

// GOTCHA: ExtractJSDoc is a pure string->string function with NO error return.
// Do not add error handling, logging, or a signature change.

// GOTCHA: the existing cases are NOT numbered sequentially in comments (the table
// skips "12" because missing-file is a separate t.Run). Just APPEND your two
// cases; do not renumber.
```

## Implementation Blueprint

### Data models and structure

No data models are involved. `ExtractJSDoc` operates on strings; `stripStarPrefix`
is unchanged. This is a pure logic fix in one branch of one switch.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the regression test cases FIRST (TDD)
  - FILE: internal/discover/jsdoc_test.go
  - FIND: the `cases := []struct{...}{ ... }` slice in TestExtractJSDoc.
  - APPEND two rows to the slice (after the last existing case, before the
    closing `}`):
      // 15. Empty single-line JSDoc, degenerate opener/closer overlap (Issue 2).
      {"empty-block-degenerate", "/**/\n", ""},
      // 16. Empty single-line JSDoc with a space (regression guard; already worked).
      {"empty-block-space", "/** */\n", ""},
  - DO NOT renumber or remove any existing case.
  - DO NOT touch the BOM special-casing in the t.Run loop body.
  - RUN (expect empty-block-degenerate to FAIL, proving the test catches the bug;
        empty-block-space should PASS already):
      go test ./internal/discover/ -run 'TestExtractJSDoc/empty-block' -v

Task 2: FIX the single-line branch in ExtractJSDoc
  - FILE: internal/discover/jsdoc.go
  - FIND: the `case i == startIdx && i == closeIdx:` branch (~line 135), whose
    current body is:
        rest := strings.TrimPrefix(line, "/**")
        if c := strings.Index(rest, "*/"); c >= 0 {
            rest = rest[:c]
        }
        line = rest
    with the 1-line comment `// Single-line block: strip "/**" prefix, then cut at first "*/".`
  - REPLACE the comment + body with the position-based extraction (item CONTRACT):
        // Single-line block: find "*/" on the ORIGINAL line BEFORE extracting
        // content, so the opener/closer overlap in degenerate blocks (e.g. "/**/"
        // has "*/" at index 2, within the opener's first 3 chars) is handled. For
        // "/**/" the closer sits inside the opener "/**" -> empty content.
        c := strings.Index(line, "*/")
        if c <= 3 {
            // "*/" starts at or before the opener "/**" ends (index 3 exclusive)
            // -> opener and closer overlap -> empty content.
            line = ""
        } else {
            line = line[3:c] // content strictly between "/**" and "*/"
        }
  - PRESERVE: the surrounding `switch { ... }`, the other three cases (opener-line,
    closer-line, middle), and the post-switch `stripStarPrefix` + `TrimSpace` +
    drop-empty tail. PRESERVE the `var parts []string` and the final
    `strings.TrimSpace(strings.Join(parts, " "))` return.
  - DO NOT touch step 5 (the `/***/` triple-star guard) or any multi-line branch.
  - RUN: go test ./internal/discover/ -run TestExtractJSDoc -v
    EXPECT: all cases pass (incl. the two new ones, incl. single-line and
    single-line-no-space unchanged).

Task 3: VALIDATE (whole-repo gate)
  - RUN: go build ./... ; go vet ./... ; go test ./internal/discover/... -v ;
         go test -race ./...
  - EXPECT: all clean / all pass. (P1.M2.T3.S1 may have landed an index.go change
    in parallel; that is independent of this jsdoc.go change and must also pass.
    If a race/compile failure appears in index.go from the parallel work, it is
    NOT this task's regression — but this task's jsdoc_test.go must still pass.)
```

### Implementation Patterns & Key Details

```go
// The fixed branch in context (the ONLY change inside the loop). Everything
// outside this `case` is unchanged. The tail (stripStarPrefix + TrimSpace +
// drop-empty) is what turns line="" into "no parts appended" and line=" " into
// "" (dropped), so the fix only needs to set `line` correctly.

case i == startIdx && i == closeIdx:
    // Single-line block: find "*/" on the ORIGINAL line BEFORE extracting
    // content, so the opener/closer overlap in degenerate blocks (e.g. "/**/"
    // has "*/" at index 2, within the opener's first 3 chars) is handled. For
    // "/**/" the closer sits inside the opener "/**" -> empty content.
    c := strings.Index(line, "*/")
    if c <= 3 {
        // "*/" starts at or before the opener "/**" ends (index 3 exclusive)
        // -> opener and closer overlap -> empty content.
        line = ""
    } else {
        line = line[3:c] // content strictly between "/**" and "*/"
    }
// (fall through to the shared: if cleaned := strings.TrimSpace(stripStarPrefix(line)); cleaned != "" { parts = append(parts, cleaned) })
```

### Integration Points

```yaml
# This fix has NO integration surface — it is a pure-function output change.
# The single consumer chain is unchanged:

CONSUMED BY (unchanged):
  - discover.BuildExtension (internal/discover/extension.go): calls
    ExtractJSDoc(entryFile) for the description fallback. With the fix, a /**/-
    leading file yields Description="" (was "/"), so --list renders "(none)".
  - discover.Index (internal/discover/index.go): calls BuildExtension per entry.
    No change to Index (P1.M2.T3.S1 edits Index for a SEPARATE symlink bug).

NO CHANGES TO:
  - go.mod / go.sum (stdlib `strings`/`bytes`/`os` already imported)
  - extension.go, discover.go, index.go, ui.go, main.go
  - any test other than the two added rows in jsdoc_test.go
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 2 (the jsdoc.go edit):
go build ./internal/discover/...   # compiles
go vet   ./internal/discover/...   # clean

# Project-wide (should be a no-op for this change):
go build ./...
go vet ./...

# Expected: zero errors. The edit uses only stdlib `strings.Index` (already used
# in the file) and string slicing — no new imports, no new symbols.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The targeted cases (TDD: run after Task 1 to see the bug, after Task 2 to see green):
go test ./internal/discover/ -run 'TestExtractJSDoc/empty-block' -v

# The full ExtractJSDoc table (all 16 cases after the addition):
go test ./internal/discover/ -run TestExtractJSDoc -v

# The integration test (JSDoc -> BuildExtension fallback; unaffected by this fix):
go test ./internal/discover/ -run TestExtractJSDocFeedsBuildExtensionFallback -v

# Expected: all pass. If `empty-block-degenerate` still returns "/", the edit did
# not land in the right branch — re-read Task 2's FIND step (it must be the
# `case i == startIdx && i == closeIdx` branch, not the opener/closer-line ones).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and run the issue's repro (confirms the user-visible symptom
# is gone: DESCRIPTION shows "(none)", not "/"):
go build -o /tmp/weave .
rm -rf /tmp/bug2 && mkdir -p /tmp/bug2
printf '/**/\n' > /tmp/bug2/empty.ts
weave_EXTENSIONS_DIR=/tmp/bug2 /tmp/weave --list
# EXPECTED: a table row whose DESCRIPTION column is "(none)" (was "/").

# Confirm a sibling single-line-with-content extension still extracts correctly:
printf '/** real desc */\nexport default function(){}\n' > /tmp/bug2/real.ts
weave_EXTENSIONS_DIR=/tmp/bug2 /tmp/weave --list
# EXPECTED: empty.ts -> (none) ; real.ts -> "real desc".

# Whole-repo sweep (catches any accidental regression in sibling packages):
go test ./... -v
go test -race ./...
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None beyond Level 3. This is a pure string-function fix with no I/O, no config,
# no network, no rendering of its own. The §7.3.2 contract (empty JSDoc -> "")
# is fully covered by the table-driven unit tests in Level 2 plus the Level 3
# --list repro.

# Optional sanity (not required): confirm "/***/" (triple-star) is STILL rejected
# at step 5 (must not now leak through as a description). It is covered by the
# existing {"triple-star", "/***\n * desc\n */\n", ""} case — verify it still
# passes in the Level 2 run.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./internal/discover/ -run TestExtractJSDoc -v` — all cases pass
      (14 existing + 2 new).
- [ ] `go test ./internal/discover/... -v` — full discover package green
      (incl. TestExtractJSDocFeedsBuildExtensionFallback).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new deps; `go.mod`/`go.sum` unchanged.

### Feature Validation

- [ ] `ExtractJSDoc` returns `""` for `/**/\n` (case `empty-block-degenerate`).
- [ ] `ExtractJSDoc` returns `""` for `/** */\n` (case `empty-block-space`; guard).
- [ ] `ExtractJSDoc` still returns `"desc"` for `/** desc */\n` (case `single-line`).
- [ ] `ExtractJSDoc` still returns `"desc"` for `/**desc*/\n` (case `single-line-no-space`).
- [ ] `ExtractJSDoc` still returns `""` for `/***...` (case `triple-star`; step-5 guard intact).
- [ ] Level 3 repro: `weave --list` on a `/**/`-leading file shows `(none)`, not `/`.

### Code Quality Validation

- [ ] ONLY the single-line branch (`case i == startIdx && i == closeIdx`) changed.
- [ ] The opener-line, closer-line, and middle-line branches are UNCHANGED.
- [ ] Step 5 (`/***/` triple-star guard) is UNCHANGED.
- [ ] The branch comment explains WHY `*/` is found on the original line first
      (opener/closer overlap in degenerate blocks) — [Mode A] doc ride-along.
- [ ] The two new test cases use filesystem-safe, unique names and append to the
      slice without renumbering existing cases.
- [ ] No signature change; `ExtractJSDoc(path string) string` preserved.

### Documentation & Deployment

- [ ] Branch comment rewritten to explain the overlap rationale (prevents a
      future "simplification" back to the bug).
- [ ] No README/user-doc change in this subtask (the M4.T1 doc sync owns
      user-facing prose; this is an internal-function fix).

---

## Anti-Patterns to Avoid

- ❌ Don't operate on `TrimPrefix(line, "/**")` and then look for `*/` — THAT IS
  THE BUG (the closer's `*` at index 2 is consumed by the opener). Find `*/` on
  the ORIGINAL line first.
- ❌ Don't change `c <= 3` to `c < 3` — use the inclusive bound exactly as the
  item CONTRACT specifies (it faithfully encodes "opener ends at index 3
  exclusive").
- ❌ Don't touch the multi-line branches or step 5 — the bug is single-line only;
  the other forms are already correct.
- ❌ Don't copy the "Fixed code" from `architecture/fix_design.md` Bug 2 — that
  doc's attempts are exploratory and one is self-marked "STILL WRONG". Use the
  item-CONTRACT position-based fix.
- ❌ Don't renumber or remove existing test cases — append only.
- ❌ Don't add error handling, logging, or a signature change to `ExtractJSDoc`.
- ❌ Don't edit `index.go`, `extension.go`, or any other file — P1.M2.T3.S1 owns
  the `index.go` symlink fix; this task owns `jsdoc.go` only.

---

**Confidence Score: 10/10** for one-pass success. The fix is a single localized
branch replacement with byte-for-byte code specified in the item CONTRACT,
verified by an exhaustive trace table over all four single-line forms
(`/**/`, `/** desc */`, `/**desc*/`, `/** */`), bounded by an explicit "do NOT
touch" list, validated by TDD (add the failing test first, then the fix), and
gated by the standard `go build/vet/test -race` suite plus the issue's
`--list` repro. The only residual risk — editing the wrong `case` branch — is
eliminated by naming the exact branch (`case i == startIdx && i == closeIdx`) and
showing the exact current broken body to find-and-replace.
