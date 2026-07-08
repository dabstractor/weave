# PRP — P1.M3.T1.S1: `Resolve()` with 4-step precedence + `UnknownError` + `AmbiguousError`

## Goal

**Feature Goal**: Implement `internal/resolve` — a PURE function that maps a user-supplied tag to a single `discover.Extension` using the PRD §7.2 precedence (exact RelTag → basename → package.json name → declared alias → unknown), with typed `*AmbiguousError` (sorted candidates) and `*UnknownError` (the two error values `main.go`'s tag-resolution loop, P1.M3.T2.S1, switches on for PRD §6.4 error semantics). This is a **near-verbatim port** of skilldozer's `internal/resolve/resolve.go` (read in full during research); the ONLY semantic change is the element type `discover.Skill` → `discover.Extension`, plus the noun swap `skill` → `extension` in error strings and doc comments.

**Deliverable**: TWO new files in ONE new package —
- `internal/resolve/resolve.go` — package `resolve`: `MatchKind` (+ `String()`), `Result`, `UnknownError`, `AmbiguousError`, `Resolve()`, and the unexported helpers `collectMatches`, `basename`, `sortedRelTags`.
- `internal/resolve/resolve_test.go` — package `resolve`: table-driven tests covering the PRD §7.2 examples (exact tag, basename fallback `reddit-poster` → `writing/reddit-poster`, name fallback, alias fallback), ambiguity at each step (sorted candidates), unknown tag, the empty-tag guard (`Name==""`/empty-alias never match), the duplicate-alias-counted-once case, `errors.As` extraction, exact `.Error()` text, and `MatchKind.String()`.

NO other files change. NO new deps (`go.mod` untouched — stdlib `fmt`/`sort`/`strings` + `internal/discover`, all present). `main.go` does NOT wire this yet (that is P1.M3.T2.S1); this task delivers the package `main` will import.

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `go test ./internal/resolve/... -v` — ALL tests pass (the §7.2 examples table + ambiguity + unknown + guards + errors.As + message text + MatchKind.String + basename).
- `go test -race ./...` passes (no data races; `Resolve` is pure/allocs-only).
- `Resolve` is a pure function over its arguments: no filesystem, no globals, no I/O. `main` (M3.T2.S1) supplies `exts` from `discover.Index(dir)`.
- The 4-step precedence is exactly PRD §7.2, and an ambiguity at ANY step short-circuits (later steps are NOT consulted).

## User Persona (if applicable)

**Target User**: `main.go`'s tag-resolution loop (P1.M3.T2.S1) — the sole consumer. Indirectly: the end user running `weave <tag>` / `weave -f <tag>` / `weave --all`, and scripts using `pi -e "$(weave <tag>)"` (whose correctness depends on PRD §6.4 error semantics, which depend on these typed errors).

**Use Case**: `main.go` has already called `extdir.Find()` and `discover.Index(dir)` to get `exts []discover.Extension`. For each positional `<tag>` it calls `resolve.Resolve(tag, exts)`. On success it prints the resolved `Result.Extension.Path` (or `.EntryFile` for `--file`). On `*UnknownError`/`*AmbiguousError` it prints one stderr line per problem tag and exits 1 (PRD §6.4). This task is the resolution engine those calls dispatch into.

**User Journey**:
1. `weave reddit-poster` → `main` calls `Resolve("reddit-poster", exts)`.
2. Step 1 (exact RelTag): no `RelTag == "reddit-poster"`.
3. Step 2 (basename): `writing/reddit-poster` has basename `reddit-poster` → exactly one hit → `Result{Extension: ..., Match: Basename}, nil`.
4. `main` prints `.../extensions/writing/reddit-poster.ts`, exits 0.

**Pain Points Addressed**:
- **Disambiguation**: a short tag like `reddit-poster` that could mean `writing/reddit-poster` must resolve unambiguously to a single path when only one extension has that basename, but fail loudly (with the candidate list) when two do.
- **`$(...)` safety (PRD §6.4)**: `*UnknownError`/`*AmbiguousError` are distinct typed errors so `main` can branch and print one error line per problem tag, print NOTHING to stdout, and exit 1 — guaranteeing `pi -e "$(weave badtag)"` fails loudly instead of passing a garbage path.

## Why

- **PRD §7.2 is THE tag-resolution contract**: the 4-step precedence (first match wins; later steps consulted only if every earlier step produced nothing; ambiguity at any step short-circuits) is the spec this package implements. Without it, `weave <tag>` cannot run.
- **Near-verbatim port minimizes risk**: skilldozer's `resolve.go` is a PROVEN, heavily-tested implementation (10 test functions cover examples, ambiguity-at-each-step, unknown, empty-tag guard, duplicate-alias-counted-once, `errors.As`, message text, MatchKind.String, basename). Porting it verbatim with the documented type/noun change inherits all that coverage. The precedence logic, the short-circuit rule, and the `sortedRelTags` determinism are subtle enough that re-deriving them would re-litigate settled decisions.
- **Typed errors are load-bearing for PRD §6.4**: `main` (M3.T2.S1) MUST distinguish "unknown tag" (one line: `unknown extension tag %q`) from "ambiguous tag" (one line listing sorted candidates: `ambiguous extension tag %q matches: ...`). `errors.As(err, &ae)` is the contract `main` relies on; this package owns those two error types.
- **Scope boundary**: this task delivers ONLY the resolution package. It does NOT touch `main.go` (M3.T2.S1), does NOT add modifiers (`--file`/`--relative`/`--all`), does NOT add `--search`/`check`. Keeping it isolated lets the package be unit-tested in pure-table form with synthetic `[]Extension` inputs — no filesystem, no `extdir`, no `discover.Index` required.

## What

A new package `internal/resolve` (package `resolve`) with the public surface below. Full code structure mirrors skilldozer's `internal/resolve/resolve.go` (read in full; see `research/port_mapping.md`).

### Public types & function

```go
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// MatchKind: which §7.2 step resolved the tag. Zero value not a valid success.
type MatchKind int
const (
	Canonical MatchKind = iota  // tag == Extension.RelTag (step 1)
	Basename                     // tag == basename(RelTag) (step 2)
	Name                         // tag == Extension.Name, non-empty (step 3)
	Alias                        // tag in Extension.Aliases (step 4)
)
func (m MatchKind) String() string  // "canonical"|"basename"|"name"|"alias"|"unknown"

// Result: outcome of resolving one tag. Zero value NOT a valid success.
type Result struct {
	Extension discover.Extension
	Match     MatchKind
}

type UnknownError struct{ Tag string }
func (e *UnknownError) Error() string  // `unknown extension tag %q`

type AmbiguousError struct {
	Tag        string
	Candidates []string  // SORTED RelTags of the matching extensions
}
func (e *AmbiguousError) Error() string  // `ambiguous extension tag %q matches: <a>, <b>`

func Resolve(tag string, exts []discover.Extension) (Result, error)
```

### 4-step precedence (identical to skilldozer; PRD §7.2)

1. **Exact RelTag** (`Canonical`): `ext.RelTag == tag`. RelTag is unique per entry ⇒ at most one hit, no ambiguity possible. First match wins.
2. **Basename** (`Basename`): `basename(ext.RelTag) == tag`. If exactly 1 hit → return it; if >1 → `*AmbiguousError` (sorted candidates). SHORT-CIRCUITS.
3. **Name** (`Name`): `ext.Name != "" && ext.Name == tag`. If exactly 1 → return; if >1 → `*AmbiguousError`. SHORT-CIRCUITS. The `Name != ""` guard prevents a missing-name extension (or a degenerate empty tag) from spuriously matching.
4. **Alias** (`Alias`): `tag` appears in `ext.Aliases`. If exactly 1 → return; if >1 → `*AmbiguousError`. SHORT-CIRCUITS.
5. **Nothing** → `*UnknownError{Tag: tag}`.

### Unexported helpers (ported verbatim, type change only)

- `collectMatches(exts []discover.Extension, pred func(discover.Extension) bool) []discover.Extension` — every ext where `pred` is true, in input order. Each ext appears at most once.
- `basename(relTag string) string` — final `/`-component via `strings.LastIndex` (zero-alloc; no `path` import). `"writing/reddit-poster"` → `"reddit-poster"`; `"gate"` → `"gate"`. RelTag is slash-normalized by `discover`, so splitting on `/` is correct on every platform.
- `sortedRelTags(exts []discover.Extension) []string` — RelTags sorted ascending → `AmbiguousError.Candidates` (deterministic regardless of input slice order; PRD §6.4 wants stable stderr for scripting).

### Success Criteria

- [ ] `internal/resolve/resolve.go` exists, package `resolve`, with the types/function/helpers above.
- [ ] `internal/resolve/resolve_test.go` exists with passing tests for: PRD §7.2 examples (exact, basename, name, alias); ambiguity at steps 2/3/4 (sorted candidates, reverse-order input to prove sorting); unknown tag + nil/empty index; empty-tag guard; duplicate-alias-counted-once; `errors.As` for both error types (incl. negative cross-check); exact `.Error()` text; `MatchKind.String()` (incl. out-of-range `unknown`); `basename` direct.
- [ ] `go build ./...` && `go vet ./...` && `go test -race ./...` all clean.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_ **Yes** — the source to port verbatim is at a known absolute path, the type-to-change is fully specified (`discover.Extension`, fields documented in `internal/discover/extension.go`), the noun swap is explicit, the import path is in `go.mod`, the test pattern is established in sibling packages, and the validation commands are project-standard. No guessing required.

### Documentation & References

```yaml
# MUST READ — the verbatim port source (read it in full before writing a line)
- file: /home/dustin/projects/skilldozer/internal/resolve/resolve.go
  why: THIS is what we port. MatchKind, Result, UnknownError, AmbiguousError, Resolve,
       collectMatches, basename, sortedRelTags — all defined here, all correct.
  pattern: 4-step precedence with short-circuit; sortedRelTags for determinism;
           comma-ok / len-based branching; zero-alloc basename via strings.LastIndex.
  gotcha: the type changes Skill→Extension and Result.Skill→Result.Extension; error
          strings change skill→extension; doc comments change "frontmatter name"→
          "package.json name". Logic is UNCHANGED.

- file: /home/dustin/projects/skilldozer/internal/resolve/resolve_test.go
  why: THIS is the test to port. 10 test functions cover everything the success
       criteria list. Port nearly verbatim with the noun swap + the exampleExts
       fixture change (Skill{Dir,SourceFile} → Extension{Path,EntryFile}).
  pattern: table-driven + t.Run subtests; plain == asserts; errors.As extraction;
           reverse-sorted input to prove Candidates sorting.
  gotcha: the fixture's exampleSkills uses reddit-poster as the nested skill; weave's
          exampleExts should keep RelTag "writing/reddit-poster" so the PRD §7.2
          basename example (`reddit-poster` → `writing/reddit-poster`) is hit literally.

# The input type — fields Resolve touches
- file: internal/discover/extension.go
  why: defines discover.Extension (RelTag, Name, Aliases are all Resolve consumes).
       Confirms field names/types so the port's predicate closures compile.
  pattern: Extension is BUILT (never unmarshaled), no json tags; Aliases/Keywords
           are []string (nil if absent, non-nil empty if present-but-empty) — test
           with len(), not nil.
  gotcha: Name is "" when no package.json / no name field — that's the empty-tag
          guard's trigger. RelTag is slash-normalized + .ts/.js-stripped by discover.

# Module path
- file: go.mod
  why: module github.com/dabstractor/weave — the import path for internal/discover.
  critical: use "github.com/dabstractor/weave/internal/discover" (NOT skilldozer).

# Test conventions in this repo
- file: internal/discover/extension_test.go
  why: shows the repo's test style — stdlib testing only (no testify), table-driven,
       t.Run subtests, helper funcs, plain == / reflect.DeepEqual asserts.
  pattern: mirror this style in resolve_test.go (the skilldozer test already follows it).

# PRD spec (authoritative for the precedence)
- docfile: PRD.md
  section: §7.2 (Tag resolution precedence) — the 5 numbered rules + examples.
  why: confirms the precedence order and the example mappings the tests must hit.

- docfile: PRD.md
  section: §6.4 (Error semantics) — why AmbiguousError.Candidates must be sorted
           (stable stderr for scripting; main lists them on stderr, exit 1).

# Architecture mapping (the item cites §4)
- docfile: plan/001_19b4b465824d/architecture/architecture_mapping.md
  section: §4 (internal/resolve — PORT NEARLY VERBATIM)
  why: explicitly says "Port Resolve(), UnknownError, AmbiguousError, collectMatches,
       basename, sortedRelTags verbatim. Only the type changes: discover.Skill →
       discover.Extension."
```

### Current Codebase tree (relevant subset)

```bash
go.mod                          # module github.com/dabstractor/weave, go 1.25
main.go                         # M1.T4 — parses argv; tag loop is M3.T2.S1 (NOT this task)
internal/
├── config/                     # M1.T2 (done) — not used here
├── extdir/                     # M1.T3 (done) — not used here
├── discover/
│   ├── extension.go            # Extension struct (THE input type) + BuildExtension
│   ├── jsdoc.go                # M2.T1.S2 (done)
│   ├── discover.go             # M2.T2 (classify-then-descend)
│   └── index.go                # M2.T3 — Index() returns []Extension (Resolve's input)
└── ui/                         # M2.T4 (done) — not used here
# internal/resolve/  ← DOES NOT EXIST YET; this task creates it
```

### Desired Codebase tree with files to be added

```bash
internal/resolve/
├── resolve.go        # NEW — package resolve: types + Resolve() + helpers (verbatim port)
└── resolve_test.go   # NEW — table-driven tests (ported from skilldozer + noun swap)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: this is a VERBATIM PORT. Do not "improve" the logic. The precedence,
// the short-circuit, the sortedRelTags determinism, and the empty-tag guard are
// all settled in skilldozer and re-derived here only invites regressions.

// CRITICAL: the type is discover.Extension (NOT Skill). The import path is
// github.com/dabstractor/weave/internal/discover (module = github.com/dabstractor/weave).

// CRITICAL: error strings say "extension", not "skill":
//   UnknownError:  `unknown extension tag %q`
//   AmbiguousError: `ambiguous extension tag %q matches: <c1>, <c2>`
// (main's §6.4 stderr lines are built FROM these; the noun must match the binary.)

// CRITICAL: Result.Extension (NOT Result.Skill). MatchKind values are Canonical/Basename/Name/Alias.

// CRITICAL: Extension.Aliases and .Name come from package.json (NOT frontmatter).
//   doc comments should say "package.json name" / "weave.aliases", never "frontmatter".

// GOTCHA: Aliases/Keywords are nil-if-absent, non-nil-empty-if-present-but-empty.
//   ALWAYS test with len(), never a nil check. (Resolve's alias loop ranges over
//   Aliases; a nil slice ranges zero times — safe, no guard needed.)

// GOTCHA: RelTag is slash-normalized by discover (filepath.ToSlash) and has .ts/.js
//   stripped for single-file entries. So basename() splitting on '/' is correct
//   cross-platform; do NOT re-import "path" or call filepath.Base.

// GOTCHA: step 1 (exact RelTag) is inlined in Resolve (not via collectMatches) because
//   RelTag is unique ⇒ at most one hit ⇒ no ambiguity branch. Keep it inlined.
```

## Implementation Blueprint

### Data models and structure

No new data models — this package only DEFINES the resolution types and consumes `discover.Extension`. The types (`MatchKind`, `Result`, `UnknownError`, `AmbiguousError`) ARE the data model and are specified in full in the **What** section above. Do not add fields beyond the skilldozer originals.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/resolve/resolve.go
  - IMPLEMENT: package resolve with MatchKind (+String), Result, UnknownError,
    AmbiguousError, Resolve(), and unexported collectMatches/basename/sortedRelTags.
  - PORT FROM: /home/dustin/projects/skilldozer/internal/resolve/resolve.go (read in full)
  - TYPE CHANGE: discover.Skill → discover.Extension EVERYWHERE (struct field, func
    params, predicate closures, Result.Skill → Result.Extension).
  - IMPORT CHANGE: "github.com/dabstractor/skilldozer/internal/discover"
                  → "github.com/dabstractor/weave/internal/discover".
  - NOUN SWAP: error strings "skill" → "extension"; doc comments "skill(s)" →
    "extension(s)", "frontmatter name" → "package.json name".
  - LOGIC: UNCHANGED — same 4-step precedence, same short-circuit, same sortedRelTags,
    same empty-tag guards (Name != "" in step 3; alias loop naturally skips empty).
  - PLACEMENT: internal/resolve/resolve.go (new package dir).
  - FILES TOUCHED: 1 (new).

Task 2: CREATE internal/resolve/resolve_test.go
  - IMPLEMENT: package resolve tests — table-driven, stdlib testing only.
  - PORT FROM: /home/dustin/projects/skilldozer/internal/resolve/resolve_test.go (read in full)
  - FIXTURE CHANGE: exampleSkills → exampleExts; use Extension{Path, EntryFile, RelTag,
    Kind, Name, Aliases}. Keep RelTag/Name/Aliases values so coverage is identical:
      {RelTag:"gate", Kind:"file", Path:"/repo/extensions/gate.ts", EntryFile:".../gate.ts"}
      {RelTag:"writing/reddit-poster", Name:"reddit-poster", Aliases:[]string{"social"},
       Path:"/repo/extensions/writing/reddit-poster.ts", EntryFile:".../reddit-poster.ts"}
    (mirrors PRD §7.1 worked example: gate.ts, writing/reddit-poster.ts).
  - EXAMPLES TABLE (TestResolveExamples) MUST hit the PRD §7.2 cases:
      "gate"                 → Canonical, RelTag "gate"
      "writing/reddit-poster"→ Canonical, RelTag "writing/reddit-poster"
      "reddit-poster"        → Basename,  RelTag "writing/reddit-poster"  (← item's required basename test)
      "reddit-poster" via Name→ Name       (reddit-poster is BOTH basename AND name; basename wins —
                                            assert Match==Basename, and add a SEPARATE fixture for a
                                            pure name fallback, e.g. summarizer with Name "@my-org/summarizer")
      alias "social"         → Alias,     RelTag "writing/reddit-poster"
    To hit a PURE name fallback (item's "@my-org/summarizer ← summarizer dir"), add a third
    example ext: {RelTag:"summarizer", Name:"@my-org/summarizer", Kind:"package",
                  Path:"/repo/extensions/summarizer", EntryFile:".../src/index.ts"} and assert
    Resolve("@my-org/summarizer") → Name, RelTag "summarizer".
  - AMBIGUITY (TestResolveAmbiguous): basename / name / alias subtests; pass skills in
    REVERSE sorted order; assert Candidates are sorted ascending.
  - UNKNOWN (TestResolveUnknown): unknown tag + nil index + empty [] index → *UnknownError.
  - GUARDS (TestResolveEmptyTagGuard): Name=="" never matches step 3; empty tag "" → *UnknownError.
  - DUP-ALIAS (TestResolveDuplicateAliasCountedOnce): Aliases []string{"dup","dup"} → ONE match, resolves.
  - ERRORS.AS (TestResolveErrorsAs): both types extractable; negative cross-check.
  - MESSAGE TEXT (TestErrorMessages): exact `unknown extension tag "foo"` and
    `ambiguous extension tag "reddit" matches: coding/reddit, writing/reddit`.
  - MATCHKIND.STRING (TestMatchKindString): all 4 + out-of-range → "unknown".
  - BASENAME (TestBasename): "writing/reddit-poster"→"reddit-poster", "gate"→"gate", "a/b/c"→"c", ""→"".
  - NAMING: test funcs TestResolveExamples / TestResolveAmbiguous / TestResolveUnknown /
    TestResolvePrecedence / TestResolveEmptyTagGuard / TestResolveDuplicateAliasCountedOnce /
    TestResolveErrorsAs / TestErrorMessages / TestMatchKindString / TestBasename.
  - PLACEMENT: internal/resolve/resolve_test.go (package resolve — white-box, so basename/
    collectMatches are reachable).
  - FILES TOUCHED: 1 (new).

Task 3: VALIDATE (no file changes)
  - RUN: go build ./... ; go vet ./... ; go test ./internal/resolve/... -v ; go test -race ./...
  - EXPECT: all clean / all pass. If anything fails, READ the output and fix the port
    (most likely a missed Skill→Extension or skill→extension swap).
```

### Implementation Patterns & Key Details

```go
// The 4-step Resolve body (ported verbatim; only the type + noun change). The
// short-circuit is structural: each step's if/else-if returns before the next
// step's collectMatches is even called.

func Resolve(tag string, exts []discover.Extension) (Result, error) {
	// Step 1 — exact canonical tag. RelTag unique ⇒ at most one, no ambiguity.
	for _, e := range exts {
		if e.RelTag == tag {
			return Result{Extension: e, Match: Canonical}, nil
		}
	}
	// Step 2 — basename.
	if m := collectMatches(exts, func(e discover.Extension) bool {
		return basename(e.RelTag) == tag
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Basename}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}
	// Step 3 — package.json name (skip empty: a missing name is not matchable).
	if m := collectMatches(exts, func(e discover.Extension) bool {
		return e.Name != "" && e.Name == tag
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Name}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}
	// Step 4 — declared alias.
	if m := collectMatches(exts, func(e discover.Extension) bool {
		for _, a := range e.Aliases {
			if a == tag {
				return true
			}
		}
		return false
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Alias}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}
	return Result{}, &UnknownError{Tag: tag}
}

// basename — zero-alloc slash split (RelTag is slash-normalized by discover).
func basename(relTag string) string {
	if i := strings.LastIndex(relTag, "/"); i >= 0 {
		return relTag[i+1:]
	}
	return relTag
}

// GOTCHA: error string noun is "extension" (binary is `weave`).
func (e *UnknownError) Error() string {
	return fmt.Sprintf("unknown extension tag %q", e.Tag)
}
func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous extension tag %q matches: %s", e.Tag, strings.Join(e.Candidates, ", "))
}
```

### Integration Points

```yaml
# This package INTEGRATES nothing yet — main.go wiring is P1.M3.T2.S1.
# It is a LEAF: depends only on internal/discover (already present) + stdlib.
# Documenting the future integration so the contract is clear:

CONSUMED BY (P1.M3.T2.S1, NOT this task):
  - file: main.go (run() tag-resolution loop)
  - call: for each c.tags[i]: res, err := resolve.Resolve(tag, exts)
  - on err *resolve.AmbiguousError: fmt.Fprintf(stderr, "%v\n", err); exit 1
  - on err *resolve.UnknownError:   fmt.Fprintf(stderr, "%v\n", err); exit 1
  - on success: print res.Extension.Path (default) or res.Extension.EntryFile (--file)

NO CHANGES TO:
  - go.mod (stdlib + internal/discover only)
  - main.go / main_test.go (M3.T2.S1 / M5.T1.S1)
  - any other internal/ package
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating resolve.go (before writing tests):
go build ./internal/resolve/...     # compiles the new package
go vet   ./internal/resolve/...     # clean

# Project-wide (catches anything that broke elsewhere — should be a no-op):
go build ./...
go vet ./...

# Expected: zero errors. If errors, READ them — most likely a missed
# Skill→Extension swap or a stale skilldozer import path.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new package, verbose:
go test ./internal/resolve/... -v

# With the race detector (Resolve is pure, but the test fixtures build slices;
# -race confirms no aliasing surprises):
go test -race ./internal/resolve/...

# Expected: all tests pass. On failure, the failure is almost certainly a
# noun-swap miss in an error-string assertion (TestErrorMessages) or a
# fixture field mismatch (exampleExts vs the asserted RelTag).
```

### Level 3: Integration Testing (System Validation)

```bash
# This package has NO integration surface of its own (it is a pure function).
# The project-wide test sweep confirms it did not break siblings:
go test ./... -v        # all packages, including the new one
go test -race ./...     # whole-repo race sweep

# A quick sanity that the package is importable the way main will use it
# (optional, throwaway — delete after, or skip):
# cat > /tmp/sanity_test.go <<'EOF'
# package resolve_sanity
# import ("testing"; "github.com/dabstractor/weave/internal/discover";
#         "github.com/dabstractor/weave/internal/resolve")
# func TestSanity(t *testing.T) {
#   exts := []discover.Extension{{RelTag:"gate", Kind:"file"}}
#   r, err := resolve.Resolve("gate", exts)
#   if err != nil || r.Match != resolve.Canonical { t.Fatal(err) }
# }
# EOF
# (Not required — Level 2 already proves this. Prefer just `go test ./...`.)

# Expected: whole repo green; the new package's tests are the new coverage.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None for this task. It is a pure function with no I/O, no network, no fs, no
# config, no rendering. The §7.2 precedence IS the domain logic and is fully
# covered by the table-driven unit tests in Level 2 (examples, ambiguity at each
# step, unknown, guards, errors.As, message text).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./internal/resolve/... -v` — all tests pass.
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new dependencies added (`go.mod` unchanged; `go.sum` unchanged).

### Feature Validation

- [ ] `Resolve` implements the exact PRD §7.2 5-rule precedence.
- [ ] Ambiguity at ANY step short-circuits (later steps NOT consulted).
- [ ] `AmbiguousError.Candidates` is sorted ascending (deterministic stderr for §6.4).
- [ ] PRD §7.2 examples all resolve correctly: `gate` (exact), `writing/reddit-poster` (exact nested), `reddit-poster` (basename), `@my-org/summarizer` (name), alias `social`.
- [ ] Unknown tag → `*UnknownError{Tag: tag}`; nil/empty index → `*UnknownError` (no panic).
- [ ] Empty-tag guard: `Name==""` never matches step 3; empty `""` tag → `*UnknownError`.
- [ ] `.Error()` text exact: `unknown extension tag "foo"`, `ambiguous extension tag "reddit" matches: coding/reddit, writing/reddit`.

### Code Quality Validation

- [ ] Verbatim port — logic NOT "improved" or refactored (only type + noun swap).
- [ ] Error string noun is "extension" (binary is `weave`), not "skill".
- [ ] Import path is `github.com/dabstractor/weave/internal/discover`.
- [ ] Doc comments say "package.json name"/"weave.aliases", not "frontmatter".
- [ ] File placement: `internal/resolve/resolve.go` + `internal/resolve/resolve_test.go` only.
- [ ] No other files modified (main.go is M3.T2.S1; go.mod untouched).

### Documentation & Deployment

- [ ] Package doc comment explains the 4-step precedence + short-circuit (port skilldozer's).
- [ ] Type/function doc comments carry the noun swap (extension, package.json).
- [ ] No README/docs changes (this is an internal function; resolution rules are documented in README §5 only in the final M6.T5 doc sweep).

---

## Anti-Patterns to Avoid

- ❌ Don't "improve" or refactor the ported logic — it is settled and tested in skilldozer; change ONLY the type and the noun.
- ❌ Don't import `"path"` or use `filepath.Base` in `basename` — `strings.LastIndex` on a slash-normalized RelTag is the faithful, zero-alloc port.
- ❌ Don't drop the `Name != ""` guard in step 3 — it prevents a missing-name extension (or a degenerate empty tag) from spuriously matching.
- ❌ Don't skip the short-circuit — an ambiguity at step 2 must NOT fall through to steps 3/4 (a looser match cannot rescue an ambiguity; PRD §6.4 wants the candidates).
- ❌ Don't leave error strings saying "skill" — main's §6.4 stderr is built from these and the binary is `weave`.
- ❌ Don't wire this into `main.go` — that is P1.M3.T2.S1. This task ships the package only.
- ❌ Don't add fields to `Result`/`UnknownError`/`AmbiguousError` beyond the skilldozer originals — `main` (M3.T2.S1) is written against this exact surface.
- ❌ Don't touch `go.mod`/`go.sum` — stdlib + `internal/discover` only.

---

**Confidence Score: 9/10** for one-pass success. The work is a documented verbatim port of a known-good, heavily-tested source file with a fully-specified type change and noun swap; the input type (`discover.Extension`) is already implemented and its field semantics documented; the test file to port is available and follows the repo's established test style; validation is the standard `go build/vet/test -race` suite. The one residual risk (not 10/10) is a missed `Skill`→`Extension` or `skill`→`extension` token in a comment/assertion — caught immediately by `go build`/`go test`, but it is the kind of mechanical slip that can cost a second pass.
