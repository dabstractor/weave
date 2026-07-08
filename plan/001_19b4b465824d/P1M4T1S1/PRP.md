# PRP — P1.M4.T1.S1: `Search()` + `matches()` over 6 fields

## Goal

**Feature Goal**: Implement `internal/search` — a PURE function that filters a
`[]discover.Extension` by a case-insensitive substring query over the six
searchable fields defined by PRD §6.1 `--search` and §7.1/§7.3: the canonical
tag (`RelTag`), the `package.json` `name`, the description (from `package.json`
OR the leading JSDoc, already folded into `Extension.Description` by
`BuildExtension`), each keyword INDIVIDUALLY, each alias INDIVIDUALLY, and the
`weave.category`. This is a **near-verbatim port** of skilldozer's
`internal/search/search.go` (read in full during research); the ONLY change is
the element type `discover.Skill` → `discover.Extension` (and the noun swap
`skill` → `extension` / `skilldozer` → `weave` in doc comments and test names).

**Deliverable**: TWO new files in ONE new package —
- `internal/search/search.go` — package `search`: `Search()` and the unexported
  `matches()` helper.
- `internal/search/search_test.go` — package `search` (white-box): 16 test
  functions ported from skilldozer's `search_test.go` (all of them) with the
  type/noun/field swaps.

NO other files change. NO new deps (`go.mod` untouched — stdlib `strings` +
`internal/discover`, both present). `main.go` does NOT wire this yet (that is
P1.M4.T3.S1); this task delivers the package `main` will import.

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `go test ./internal/search/... -v` — ALL 16 tests pass.
- `go test -race ./...` passes (the function is pure; this is free insurance).
- `Search` is a pure function over its arguments: no filesystem, no globals,
  no I/O. `main` (M4.T3.S1) supplies `exts` from `discover.Index(dir)` and
  passes the filtered (still-sorted) slice to `ui.PrintList`.
- An empty query matches every extension; a no-match query returns an empty
  (possibly nil) slice; input order is preserved; keywords/aliases are matched
  INDIVIDUALLY (boundary-safe).

## User Persona (if applicable)

**Target User**: `main.go`'s `--search` dispatch (P1.M4.T3.S1) — the sole
consumer. Indirectly: the end user running `weave --search social` / `weave -s
gate`, who wants to find extensions by any catalog fragment without remembering
the exact tag.

**Use Case**: `main.go` has already called `extdir.Find()` and
`discover.Index(dir)` to get `exts []discover.Extension`. For `weave --search
<q>` it calls `search.Search(q, exts)` and passes the result to
`ui.PrintList`. If the result is empty → exit 1 (PRD §6.1: `--search` is `1` if
no matches); else → table, exit 0. This task is the filter engine that call
delegates to.

**User Journey**:
1. `weave --search reddit` → `main` calls `Search("reddit", exts)`.
2. `Search` lowercases `"reddit"` once, iterates `exts`, and for each extension
   calls `matches("reddit", ext)`.
3. An extension with `RelTag:"writing/reddit-poster"` matches (substring of
   `RelTag`); an extension with `Keywords:["social"]` does NOT. Input order is
   preserved.
4. `main` passes the filtered slice to `ui.PrintList`, exit 0.

**Pain Points Addressed**:
- **Find-by-fragment**: users do not remember exact tags; `--search social`
  should surface every extension whose tag/name/desc/keyword/alias/category
  contains "social".
- **Boundary safety**: a query spanning two keywords (`"wriocial"` against
  `["writing","social"]`) must NOT match — joining keywords would create false
  positives.

## Why

- **PRD §6.1 `weave --search <q>` / `-s <q>` is the contract**: "Substring
  (case-insensitive) search over tag, `package.json` `name`/`description`/
  `keywords`, the leading-JSDoc description, `weave.aliases`, and
  `weave.category`." This task implements exactly that 6-field scope. The
  leading-JSDoc description is folded into `Extension.Description` by
  `BuildExtension` (the fallback chain from P1.M2.T1.S1), so search sees a
  single `Description` field regardless of source.
- **Near-verbatim port minimizes risk**: skilldozer's `search.go` is PROVEN
  and heavily tested (16 test functions cover each of the 6 fields, case
  insensitivity, empty-query-matches-all, no-match-returns-empty,
  order-preservation, multi-field-distinct-result, boundary-safety on BOTH
  keywords and aliases, nil input, and the metadata-less case). Porting it
  with the documented type/noun swap inherits all that coverage. The
  boundary-safety invariant (keywords/aliases matched INDIVIDUALLY) is subtle
  enough that re-deriving it would re-litigate a settled decision.
- **Scope boundary**: this task delivers ONLY the search package. It does NOT
  touch `main.go` (M4.T3.S1), does NOT add the `--search` dispatch, does NOT
  decide exit codes (those live in `main`'s dispatch and the PRD §6.1
  `--search` row: `0`; `1` if no matches). Keeping it isolated lets the
  package be unit-tested in pure-table form with synthetic `[]Extension`
  inputs — no filesystem, no `extdir`, no `discover.Index` required.
- **Parity with `resolve`**: `internal/resolve` (P1.M3.T1.S1, landed) and
  `internal/search` are sibling packages — both are pure matching functions
  over `[]discover.Extension`, in their own packages with their own `_test.go`.
  This keeps the two matching concerns — precise tag resolution (`resolve`)
  and fuzzy catalog search (`search`) — isolated and independently testable,
  exactly as the architecture_mapping §4/§6 prescribes.

## What

A new package `internal/search` (package `search`) with the public surface
below. Full code structure mirrors skilldozer's `internal/search/search.go`
(read in full; see `research/port_mapping.md`).

### Public function

```go
package search

import (
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// Search returns every extension in exts for which query is a case-insensitive
// substring of ANY of six fields: RelTag (the canonical tag), Name (package.json
// name), Description (package.json description OR the leading JSDoc, already
// folded into Extension.Description by BuildExtension), any Keyword, any Alias,
// or Category (weave.category) (PRD §6.1, §7.1, §7.3). Input order is preserved:
// discover.Index sorts []Extension by RelTag, and ui.PrintList does NOT re-sort,
// so the filtered slice is displayed already-sorted.
//
// An empty query matches EVERY extension: strings.Contains(hay, "") is always
// true, so `weave --search ""` behaves like `weave --list` (exit 1 only if the
// store is empty). The PRD carves out no special case for an empty query.
//
// An extension with no package.json (HasPackageJSON==false) has Name=="" and
// Description=="" and nil Keywords, but its RelTag is always present — so it is
// still discoverable by searching its tag, matching how resolve lets a
// metadata-less extension resolve by basename (PRD §7.1). Only RelTag is
// searchable for such an extension.
func Search(query string, exts []discover.Extension) []discover.Extension {
	q := strings.ToLower(query) // lowercase the query ONCE, not per field
	var matched []discover.Extension
	for _, e := range exts {
		if matches(q, e) {
			matched = append(matched, e)
		}
	}
	return matched
}
```

### Unexported helper

```go
// matches reports whether the already-lowercased query q is a case-insensitive
// substring of any searchable field of e. q is lowercased once by the caller
// (Search); each field is lowercased lazily inside Contains.
//
// Field scope is SIX fields: RelTag, Name, Description, each Keyword, each
// Alias, and Category. PRD §6.1 states --search covers tag, package.json
// name/description/keywords, the leading-JSDoc description, weave.aliases, and
// weave.category — so aliases and category ARE searched. This makes --search
// consistent with resolve, which resolves by alias (§7.2 step 4).
//
// Keywords are tested INDIVIDUALLY (not strings.Join'd): a query spanning a
// boundary between two keywords must not match (joining would create false
// positives like "wri"+"ocial" => "wriocial" across "writing","social").
// Aliases are matched INDIVIDUALLY for the same boundary-safety reason.
func matches(q string, e discover.Extension) bool {
	if strings.Contains(strings.ToLower(e.RelTag), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), q) {
		return true
	}
	for _, kw := range e.Keywords {
		if strings.Contains(strings.ToLower(kw), q) {
			return true
		}
	}
	// Aliases (weave.aliases) — matched INDIVIDUALLY, same boundary-safety
	// as Keywords: a query spanning two aliases must not match. PRD §6.1/§7.3
	// say aliases feed --search; this also makes --search consistent with
	// resolve, which resolves by alias (§7.2 step 4).
	for _, a := range e.Aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	// Category (weave.category) — a single scalar field (PRD §6.1/§7.3).
	if strings.Contains(strings.ToLower(e.Category), q) {
		return true
	}
	return false
}
```

### Success Criteria

- [ ] `Search` and `matches` exist in `internal/search/search.go` with the exact
      signatures above (exported `Search`, unexported `matches`).
- [ ] `Search` lowercases the query ONCE (not per field) and iterates `exts` in
      order, appending matches. Input order is preserved (no re-sort).
- [ ] `matches` short-circuits across the six fields in this order: RelTag,
      Name, Description, each Keyword, each Alias, Category.
- [ ] Keywords AND aliases are tested INDIVIDUALLY (never joined). A
      boundary-spanning query (`"wriocial"` vs `["writing","social"]`) must NOT
      match.
- [ ] An empty query matches every extension (`strings.Contains(hay, "")`).
- [ ] A no-match query returns an empty (possibly nil) slice.
- [ ] A multi-field match returns the extension ONCE (not duplicated).
- [ ] `Search` is pure: no filesystem, no globals, no I/O.
- [ ] `internal/search/search.go` imports `"strings"` and
      `github.com/dabstractor/weave/internal/discover`; no unused imports.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the verbatim port source is at a
known absolute path (`/home/dustin/projects/skilldozer/internal/search/search.go`),
the exact type/noun/field swaps are spelled out in `research/port_mapping.md`,
the input type is fully documented (`discover.Extension` in
`internal/discover/extension.go`, all 6 searchable field names confirmed
identical to skilldozer's `Skill`), the 16 tests to port are available at
`/home/dustin/projects/skilldozer/internal/search/search_test.go`, the test
helper convention is documented in `research/test_conventions.md`, and the
validation commands are project-standard. No guessing required.

### Documentation & References

```yaml
# MUST READ — the verbatim port source (read in full)
- file: /home/dustin/projects/skilldozer/internal/search/search.go
  why: contains Search() and matches() — the two functions to port verbatim.
       Read it in FULL before editing.
  pattern: lowercase-query-once, append-in-iteration-order (preserve input order),
           6-field short-circuit cascade, keywords/aliases matched INDIVIDUALLY.
  gotcha: the ONLY change is discover.Skill → discover.Extension. The HasFM bool
          (skilldozer) becomes HasPackageJSON (weave), but NEITHER matches() nor
          Search() reads it — it only matters for tests that build
          "metadata-less" extensions. See research/port_mapping.md.

- file: /home/dustin/projects/skilldozer/internal/search/search_test.go
  why: the 16 test functions to port verbatim (TestSearchMatchByTag,
       TestSearchMatchByTagSubstring, TestSearchMatchByBasenameAsSubstring,
       TestSearchMatchByName, TestSearchMatchByDescription, TestSearchMatchByKeyword,
       TestSearchCaseInsensitive, TestSearchNoMatchReturnsEmpty,
       TestSearchEmptyQueryMatchesAll, TestSearchPreservesInputOrder,
       TestSearchMultipleMatchesAllReturned, TestSearchNoFrontmatterStillMatchesByTag
       (→ rename NoPackageJSON), TestSearchMatchesCategoryAndAliases,
       TestSearchKeywordSubstringNotJoinBoundary, TestSearchNilInput,
       TestSearchReturnsDistinctResults).
  pattern: sk() helper (3-positional + variadic-keywords) → mkExt() with HasPackageJSON:true;
           inline discover.Extension{...} literals for the alias/category tests.
  gotcha: swap HasFM → HasPackageJSON in the helper and the inline literals;
          rename TestSearchNoFrontmatterStillMatchesByTag →
          TestSearchNoPackageJSONStillMatchesByTag (the "frontmatter" noun is
          skilldozer's; weave's is "package.json").

# The input type (all 6 searchable fields)
- file: internal/discover/extension.go
  why: defines discover.Extension — RelTag, Name, Description, Keywords, Aliases,
       Category (the 6 searchable fields) + HasPackageJSON. Confirms field names
       are IDENTICAL to skilldozer's Skill (no renames), so the port is a pure
       type swap.
  pattern: Extension is BUILT by BuildExtension (never unmarshaled); Description
           already folds package.json description OR leading JSDoc (the fallback
           chain), so search sees a single Description field regardless of source.
  gotcha: Keywords/Aliases are []string (normalized from []any by toStringSlice);
          an ABSENT field is nil, a PRESENT-but-empty array is []string{} — both
          have len 0, so `for _, kw := range e.Keywords` over a nil slice is a
          safe no-op (NEVER panic). Do NOT add a nil guard.

# The sibling package (parity reference — already landed)
- file: internal/resolve/resolve.go
  why: resolve is the SIBLING pure-matching package (precise tag resolution vs
       fuzzy search). Confirms the package-layout convention (own package, own
       _test.go, white-box same-package tests, pure function over []Extension)
       and the doc-comment style (PRD section citations, preformatted examples).
  pattern: package resolve; func Resolve(tag string, exts []discover.Extension) (Result, error);
           imports fmt/sort/strings + internal/discover.
  gotcha: search does NOT need fmt or sort (no error types, no sorting — Index
          pre-sorts). Import ONLY strings + internal/discover.

# Architecture mapping (the source-of-truth port directive)
- docfile: plan/001_19b465824d/architecture/architecture_mapping.md
  section: §6 internal/search — Substring search (PORT NEARLY VERBATIM)
  why: pins the directive "Port Search() and matches() verbatim. Same 6
       searchable fields. Type changes: discover.Skill → discover.Extension."

# PRD spec (authoritative)
- docfile: PRD.md
  section: §6.1 Commands/flags (the weave --search <q> / -s <q> row)
  why: pins the 6-field scope ("tag, package.json name/description/keywords, the
       leading-JSDoc description, weave.aliases, and weave.category") and the
       output contract (same table format as --list; exit 0, 1 if no matches).
- docfile: PRD.md
  section: §7.1 Discovery (RelTag/Name/Description/Keywords/Category/Aliases field defs)
  why: defines what each searchable field means and that Description already
       folds the JSDoc fallback.
- docfile: PRD.md
  section: §7.3 Metadata extraction (the leniency rules that feed Keywords/Aliases)
  why: confirms a non-array keywords ⇒ [] (so Keywords is always iterable) and
       weave.aliases/weave.category are the search-fed fields.

# The CONSUMING task (P1.M4.T3.S1 — later)
- note: main.go's --search dispatch (P1.M4.T3.S1) will call search.Search(q, exts)
        and pass the result to ui.PrintList; exit 1 if the slice is empty, else 0.
        This task does NOT wire that — it only delivers the package. Design the
        signature so the consumer is a one-liner.
```

### Current Codebase tree (relevant subset)

```bash
go.mod                          # module github.com/dabstractor/weave, go 1.25
internal/
├── discover/
│   ├── extension.go            # Extension struct (6 searchable fields) — consumed
│   ├── jsdoc.go                # M2.T1.S2 (done) — folded into Description by BuildExtension
│   ├── discover.go             # classifyFile/classifyDir (M2.T2)
│   └── index.go                # Index() → []Extension sorted by RelTag — main supplies it
├── resolve/                    # M3.T1.S1 (done) — SIBLING pure-matching package (parity ref)
│   ├── resolve.go
│   └── resolve_test.go
├── config/                     # M1.T2 (done) — not used here
├── extdir/                     # M1.T3 (done) — not used here
└── ui/                         # M2.T4 (done) — consumes search output in M4.T3.S1
```

### Desired Codebase tree with files to be added

```bash
internal/
└── search/
    ├── search.go               # NEW — package search: Search() + matches()
    └── search_test.go          # NEW — package search: 16 ported tests + mkExt helper
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the ONLY production-code change is the element type. Port
// skilldozer's Search() and matches() VERBATIM, swapping discover.Skill →
// discover.Extension. Do NOT "improve" the logic, reorder the fields, or add
// early-return optimizations — the cascade order and the per-keyword/per-alias
// loops are load-bearing.

// CRITICAL: Keywords AND Aliases are tested INDIVIDUALLY (never joined). A
// boundary-spanning query ("wriocial" vs ["writing","social"]) must NOT match.
// This is pinned by skilldozer's TestSearchKeywordSubstringNotJoinBoundary.
// Port BOTH the `for _, kw := range e.Keywords` AND `for _, a := range e.Aliases`
// loops EXACTLY as-is. Do NOT replace them with strings.Join.

// CRITICAL: do NOT add a special case for an empty query. strings.Contains(hay, "")
// is always true, so an empty query naturally matches every extension. Adding
// `if query == "" { return exts }` is harmless but UNNECESSARY and diverges from
// the verbatim port (skilldozer does not have it). Port as-is.

// CRITICAL: do NOT sort the output. discover.Index already sorts []Extension by
// RelTag; Search iterates in order and appends → output stays sorted. ui.PrintList
// does not re-sort. Adding a sort diverges from the port and is wasted work.

// CRITICAL: do NOT add a nil guard for Keywords/Aliases. `for _, kw := range nil`
// is a safe no-op in Go (zero iterations). An absent package.json field yields
// nil Keywords (toStringSlice: nil → nil), which iterates fine. A nil guard is
// dead code.

// GOTCHA: HasPackageJSON (weave) is the semantic twin of HasFM (skilldozer), but
// NEITHER matches() nor Search() reads it. It only appears in tests (to build a
// "metadata-less" extension for TestSearchNoPackageJSONStillMatchesByTag). Do not
// branch on it in production code.

// GOTCHA: the helper in tests is mkExt (NOT sk) to avoid the skilldozer noun.
// It sets HasPackageJSON: true to document intent, but matches() is indifferent.

// GOTCHA: import path is github.com/dabstractor/weave/internal/discover (NOT
// skilldozer). A copied test that forgets the swap will fail to compile with a
// clear error — easy to catch, but it is the most common copy-port slip.

// GOTCHA: search does NOT need fmt or sort (no error types, no sorting). Import
// ONLY "strings" and "github.com/dabstractor/weave/internal/discover". Unused
// imports fail `go vet`.
```

## Implementation Blueprint

### Data models and structure

No new data models. This task consumes `discover.Extension` (the 6 searchable
fields: RelTag, Name, Description, Keywords, Aliases, Category) and produces a
filtered `[]discover.Extension`. `Search` is a pure function over its two
arguments.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/search/search.go
  - CREATE the directory internal/search/ and the file search.go.
  - WRITE: `package search` + imports ("strings", "github.com/dabstractor/weave/internal/discover")
    + the Search() function + the matches() helper (see the What section for the
    full bodies).
  - PORT FROM: /home/dustin/projects/skilldozer/internal/search/search.go (read in FULL).
  - EDITS: discover.Skill → discover.Extension (3 sites: the Search signature, the
    matches signature, the `var matched []discover.Skill` declaration); loop var
    `s` → `e` (or keep `s` — cosmetic; skilldozer uses `s` for both Skill and the
    loop var; weave convention in resolve_test.go uses `e` for Extension. Pick `e`
    for consistency with the rest of weave, but it is NOT load-bearing); param name
    `skills` → `exts` (cosmetic; matches weave's noun).
  - DOC COMMENT NOUN SWAPS: `discover.Skill` → `discover.Extension`; `skill` →
    `extension`; `skilldozer --search` → `weave --search`; `frontmatter name` →
    `package.json name`; `metadata.keyword` → `package.json keywords`;
    `metadata.alias`/`metadata.category` → `weave.aliases`/`weave.category`;
    `HasFM==false` → `HasPackageJSON==false`; skilldozer §10/§7.1 citations →
    weave §6.1/§7.1/§7.3.
  - PLACEMENT: a new file in a new directory. No existing code to preserve.
  - FILES TOUCHED: 1 (internal/search/search.go — NEW).

Task 2: VALIDATE search.go compiles + vets
  - RUN: go build ./internal/search/... ; go vet ./internal/search/...
  - EXPECT: clean. Most likely failure: a stale skilldozer import path
    (github.com/dabstractor/skilldozer/...) or a missed Skill→Extension swap
    (compile error — go build catches it immediately).

Task 3: CREATE internal/search/search_test.go
  - CREATE the file internal/search/search_test.go.
  - WRITE: `package search` (white-box, same package) + imports ("testing",
    "github.com/dabstractor/weave/internal/discover") + the mkExt helper + all
    16 test functions ported from skilldozer's search_test.go.
  - PORT FROM: /home/dustin/projects/skilldozer/internal/search/search_test.go
    (read in FULL — every test function).
  - EDITS: the sk() helper → mkExt() with HasFM:true → HasPackageJSON:true and
    discover.Skill → discover.Extension in the return type; every test function's
    local []discover.Skill → []discover.Extension; the inline literals in
    TestSearchMatchesCategoryAndAliases swap HasFM:true → HasPackageJSON:true and
    discover.Skill{...} → discover.Extension{...}.
  - RENAME: TestSearchNoFrontmatterStillMatchesByTag →
    TestSearchNoPackageJSONStillMatchesByTag (noun swap; the test body is identical
    except HasFM:false → HasPackageJSON:false).
  - KEEP: the "Issue 4 fix" explanatory comment on TestSearchMatchesCategoryAndAliases
    (it documents why aliases/category ARE searched) — but UPDATE its PRD citation
    from skilldozer's §10 to weave's §6.1/§7.3.
  - NAMING: all other test names port UNCHANGED (TestSearchMatchByTag, etc.) —
    they are noun-agnostic.
  - PLACEMENT: a new file alongside search.go.
  - FILES TOUCHED: 1 (internal/search/search_test.go — NEW).

Task 4: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./internal/search/... -v ;
    go test -race ./...
  - EXPECT: all green (the 16 search tests pass; all existing M1/M2/M3 tests
    still pass; no race). On failure, the cause is almost always a stale
    skilldozer import path, a missed Skill→Extension/HasFM→HasPackageJSON swap,
    or a forgotten test-name rename.
```

### Implementation Patterns & Key Details

```go
// The lowercasing-once pattern (do NOT lowercase per field — that re-allocates
// per Contains call). q is lowercased ONCE in Search; each field is lowercased
// lazily inside Contains.
func Search(query string, exts []discover.Extension) []discover.Extension {
	q := strings.ToLower(query) // lowercase the query ONCE, not per field
	var matched []discover.Extension
	for _, e := range exts {
		if matches(q, e) {
			matched = append(matched, e)
		}
	}
	return matched
}

// The 6-field short-circuit cascade. Order is cosmetic (any true returns true)
// but port skilldozer's order VERBATIM to keep the port auditable:
// RelTag → Name → Description → each Keyword → each Alias → Category.
func matches(q string, e discover.Extension) bool {
	if strings.Contains(strings.ToLower(e.RelTag), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), q) {
		return true
	}
	for _, kw := range e.Keywords { // INDIVIDUALLY — never Join
		if strings.Contains(strings.ToLower(kw), q) {
			return true
		}
	}
	for _, a := range e.Aliases { // INDIVIDUALLY — never Join
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(e.Category), q) {
		return true
	}
	return false
}

// The test helper (ports skilldozer's sk). HasPackageJSON:true documents intent;
// matches() does NOT read it.
func mkExt(tag, name, desc string, keywords ...string) discover.Extension {
	return discover.Extension{
		RelTag:         tag,
		Name:           name,
		Description:    desc,
		Keywords:       keywords,
		HasPackageJSON: true,
	}
}
```

### Integration Points

```yaml
# This task INTEGRATES the already-landed discover package into a new pure filter.
CONSUMES:
  - discover.Extension         # M2.T1.S1 — the 6 searchable fields (RelTag/Name/Description/Keywords/Aliases/Category)

PRODUCES (for the later --search dispatch):
  - search.Search(query, exts) []discover.Extension   # M4.T3.S1 (main.go --search) consumes this;
                                                       #   passes the result to ui.PrintList; exit 1 if empty, else 0.

NO CHANGES TO:
  - go.mod / go.sum (stdlib strings + internal/discover only)
  - internal/discover/* (consumed as-is)
  - main.go (M4.T3.S1 owns the --search dispatch)
  - any other package
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (search.go):
go build ./internal/search/...   # compiles the new package
go vet ./internal/search/...     # clean (catches unused imports, Printf issues)

# Expected: zero errors. Most likely failure: a stale skilldozer import path
# (github.com/dabstractor/skilldozer/...) — go build reports it immediately.

# Project-wide (after Task 3):
go build ./...
go vet ./...
```

### Level 2: Unit Tests (Component Validation)

```bash
# After Task 3 (search_test.go):
go test ./internal/search/... -v        # the 16 ported tests, verbose
go test -race ./...                     # whole-repo race sweep (search is pure; -race is free insurance)

# Targeted re-run while debugging a single failure:
go test -run 'TestSearchMatch' ./internal/search/... -v
go test -run 'TestSearchKeyword|TestSearchMatchesCategory' ./internal/search/... -v

# Expected: all 16 tests pass. On failure, the cause is almost always one of:
#   (a) a stale skilldozer import path (compile error — go build catches it)
#   (b) a missed HasFM → HasPackageJSON swap in a copied inline literal
#   (c) a forgotten rename of TestSearchNoFrontmatterStillMatchesByTag
#   (d) an accidental Join of Keywords/Aliases that breaks TestSearchKeywordSubstringNotJoinBoundary
```

### Level 3: Integration Testing (System Validation)

```bash
# This package is a pure function — it has NO filesystem/I/O integration of its own.
# The end-to-end --search behavior is validated in P1.M4.T3.S1 (main.go dispatch).
# Here, do a smoke-test that search.Search composes with the real discover.Index
# and ui.PrintList, to confirm the contract holds across package boundaries:

go build -o /tmp/weave-search-smoke ./internal/search 2>/dev/null || true
# (search is a library, not a binary — the above is a no-op; instead, write a
# throwaway test or just trust Level 2. The real integration is M4.T3.S1.)

# Sanity: confirm the package is importable from main's perspective:
cat > /tmp/search_smoke_test.go <<'EOF'
package main

import (
	"fmt"
	"github.com/dabstractor/weave/internal/discover"
	"github.com/dabstractor/weave/internal/search"
)

func main() {
	exts := []discover.Extension{
		{RelTag: "gate", Name: "Gatekeeper", Description: "Access gating", Keywords: []string{"auth"}},
		{RelTag: "writing/reddit", Description: "Reddit poster", Aliases: []string{"rdt"}, Category: "social"},
	}
	for _, e := range search.Search("social", exts) {
		fmt.Println(e.RelTag) // expect: writing/reddit (matches Category)
	}
	for _, e := range search.Search("auth", exts) {
		fmt.Println(e.RelTag) // expect: gate (matches Keyword)
	}
	for _, e := range search.Search("", exts) {
		fmt.Println(e.RelTag) // expect: both (empty query matches all)
	}
	for _, e := range search.Search("zzz", exts) {
		fmt.Println("UNEXPECTED:", e.RelTag) // expect: nothing
	}
}
EOF
go run /tmp/search_smoke_test.go
# Expected output (order preserved):
#   writing/reddit
#   gate
#   gate
#   writing/reddit
rm -f /tmp/search_smoke_test.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The boundary-safety contract — the one subtle invariant. Verify a query
# spanning two keywords does NOT match (this is what distinguishes INDIVIDUAL
# matching from a naive Join):
cat > /tmp/search_boundary_test.go <<'EOF'
package main

import (
	"fmt"
	"github.com/dabstractor/weave/internal/discover"
	"github.com/dabstractor/weave/internal/search"
)

func main() {
	exts := []discover.Extension{
		{RelTag: "a", Keywords: []string{"writing", "social"}, HasPackageJSON: true},
	}
	// "wriocial" is NOT a substring of "writing" nor of "social" individually.
	if out := search.Search("wriocial", exts); len(out) != 0 {
		fmt.Println("FAIL: boundary query matched (keywords were joined):", out)
	} else {
		fmt.Println("OK: keyword-boundary query correctly did NOT match")
	}
	// "wri" IS a substring of "writing" → should match.
	if out := search.Search("wri", exts); len(out) != 1 {
		fmt.Println("FAIL: 'wri' should match keyword 'writing':", out)
	} else {
		fmt.Println("OK: 'wri' matched keyword 'writing'")
	}
}
EOF
go run /tmp/search_boundary_test.go
rm -f /tmp/search_boundary_test.go
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./internal/search/... -v` — all 16 ported tests pass.
- [ ] `go test -race ./...` — whole repo green (existing M1/M2/M3 + new search).
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged — stdlib `strings` +
      `internal/discover` only).

### Feature Validation

- [ ] `Search` lowercases the query ONCE and iterates `exts` in order.
- [ ] Input order is preserved (no re-sort inside `Search`).
- [ ] `matches` short-circuits across the six fields: RelTag, Name, Description,
      each Keyword, each Alias, Category.
- [ ] Keywords AND aliases are matched INDIVIDUALLY (boundary-safe).
- [ ] Empty query matches every extension (no special-case guard added).
- [ ] No-match query returns an empty (possibly nil) slice.
- [ ] Multi-field match returns the extension ONCE (not duplicated).
- [ ] `Search` is pure (no filesystem, no globals, no I/O).

### Code Quality Validation

- [ ] Verbatim port of skilldozer's `Search`/`matches` — logic NOT "improved"
      (only the documented type/noun/import swaps).
- [ ] Doc comments carry the noun swap (`extension`, `package.json`, `weave.aliases`,
      `weave.category`, `HasPackageJSON`, §6.1/§7.3 citations).
- [ ] Package layout matches `resolve` (own package, own `_test.go`, white-box
      same-package tests, pure function over `[]discover.Extension`).
- [ ] Imports are exactly `"strings"` + `internal/discover` (no `fmt`, no `sort`).
- [ ] The `mkExt` helper sets `HasPackageJSON: true` (documents intent; production
      code does not read it).
- [ ] `TestSearchNoFrontmatterStillMatchesByTag` renamed to
      `TestSearchNoPackageJSONStillMatchesByTag`.

### Documentation & Deployment

- [ ] `Search` has a doc comment explaining: the 6-field scope, preserve-input-order,
      empty-query-matches-all, and the metadata-less-extension-still-searchable-by-tag
      behavior.
- [ ] `matches` has a doc comment explaining: the 6-field short-circuit cascade and
      the INDIVIDUAL-keyword/alias boundary-safety invariant.
- [ ] No README/docs changes (the `--search` CLI usage is documented in README §4
      in the final M6.T4/M6.T5 doc sweep — this task's PRP §5 says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't `strings.Join` Keywords or Aliases — they MUST be matched INDIVIDUALLY
  for boundary safety. A query spanning two keywords (`"wriocial"` vs
  `["writing","social"]`) must NOT match.
- ❌ Don't add a special case for an empty query — `strings.Contains(hay, "")` is
  always true, so it naturally matches all. The PRD carves out no special case.
- ❌ Don't sort the output — `discover.Index` already sorts; `Search` iterates in
  order; `ui.PrintList` does not re-sort.
- ❌ Don't add a nil guard for Keywords/Aliases — `for _, kw := range nil` is a safe
  no-op in Go.
- ❌ Don't branch on `HasPackageJSON` in production code — `matches`/`Search` never
  read it. It exists only for the "metadata-less extension still searchable by tag"
  test case.
- ❌ Don't import `fmt` or `sort` — search has no error types and no sorting. Unused
  imports fail `go vet`.
- ❌ Don't use the `github.com/dabstractor/skilldozer/...` import path — weave's is
  `github.com/dabstractor/weave/internal/discover`.
- ❌ Don't leave `HasFM` in copied tests — swap to `HasPackageJSON` (same bool, new
  name). A missed swap is a compile error.
- ❌ Don't forget to rename `TestSearchNoFrontmatterStillMatchesByTag` →
  `TestSearchNoPackageJSONStillMatchesByTag` (the noun is skilldozer's).
- ❌ Don't reorder the 6-field cascade in `matches` — the order is cosmetic (any
  true returns true) but keeping skilldozer's order makes the port auditable line
  by line.
- ❌ Don't wire `main.go`'s `--search` dispatch — that is P1.M4.T3.S1. This task
  delivers ONLY the package.
- ❌ Don't touch `go.mod`/`go.sum` — `strings` is stdlib, `internal/discover` is local.

---

**Confidence Score: 10/10** for one-pass success. The work is a documented
verbatim port of two proven, heavily-tested, pure functions from skilldozer
(`Search` and `matches`), with a single well-specified type swap
(`discover.Skill` → `discover.Extension`) whose field names for all 6 searchable
fields are IDENTICAL (confirmed in `internal/discover/extension.go`). The
consumed package (`discover.Extension`) is already landed (P1.M2.T1.S1, Complete).
The sibling package (`internal/resolve`) demonstrates the exact package-layout
and doc-comment conventions to follow. The test file to port is available and
the weave test conventions (`mkExt` helper, inline literals, white-box same-
package tests) are documented in `research/test_conventions.md`. The boundary-
safety invariant (keywords/aliases matched INDIVIDUALLY) and the empty-query-
matches-all semantics are pinned by specific skilldozer tests that port verbatim.
There is no filesystem, no I/O, no error handling, and no integration surface —
the only risk is a mechanical copy-port slip (stale import path, missed
`HasFM`→`HasPackageJSON` swap, forgotten test rename), all of which `go build`
and `go test` catch immediately with clear errors.
