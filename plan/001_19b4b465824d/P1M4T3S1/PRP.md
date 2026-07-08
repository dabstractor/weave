# PRP — P1.M4.T3.S1: `--search` dispatch + `check` dispatch in `run()`

## Goal

**Feature Goal**: Wire two new dispatch branches into `main.go`'s `run()` so that
`weave --search <q>` and `weave check` work end-to-end. This is pure assembly:
the "gears" all exist from prior milestones — `search.Search` (M4.T1.S1),
`check.Check` (M4.T2.S1, built in parallel), `ui.PrintList` (M2.T4.S1),
`discover.Index` (M2.T3.S1), `extdir.Find` (M1.T3). The arg parser already
captures `--search`/`-s` and `check` (M1.T4.S1, with full tests). This task adds
the two `if c.searchMode { … }` / `if c.check { … }` blocks that call the gears
and render the results. It is a near-verbatim port of skilldozer's `--search` and
`check` branches in `skilldozer/main.go` (read in full during research), with
four documented deltas (§"Implementation Patterns").

**Deliverable**: ONE file MODIFIED — `main.go`:
- add two imports (`internal/check`, `internal/search`);
- add the `--search` branch and the `check` branch in `run()`, between the
  existing `--list` branch and the `--all` branch (skilldozer's proven order:
  `list → search → check → all → tags`);
- add one tiny helper, `relTagBase`, for the check report's "name or basename" rule.

ONE file MODIFIED — `main_test.go`: add ~9 new run-level tests
(`--search` match/no-match/short/empty-query/unresolvable + check
clean/empty-store/with-error/with-warning/unresolvable). NO new files.
NO `go.mod` changes (stdlib + 4 internal packages, all present).

**Success Definition**:
- `go build ./...` exits 0; `go vet ./...` exits 0.
- `go test ./... -v` passes (all existing tests + the ~9 new ones).
- `weave --search reddit` on a store containing `writing/reddit-poster` → TAG
  table on stdout, exit 0; `weave --search zzz` → stderr `no extensions matched zzz`,
  stdout EMPTY, exit 1.
- `weave check` on a clean store → `OK` line per extension + `N extensions, 0
  errors, 0 warnings`, exit 0; `weave check` with any ERROR → exit 1, report on
  STDOUT (pipeable); warnings never change the exit code.

## User Persona (if applicable)

**Target User**: weave CLI users running `weave --search <q>` to find an extension
by substring, or `weave check` to validate their store before relying on
`pi -e "$(weave <tag>)"`.

**Use Case**: "I forgot the exact tag — `weave --search reddit` shows me the
match." / "Did my hand-edits break anything? `weave check` walks the store and
reports ERROR/WARN per extension."

**User Journey**:
1. `weave --search reddit` → `run()` dispatches to the `--search` branch →
   `extdir.Find` → `discover.Index` → `search.Search("reddit", exts)` →
   `ui.PrintList` renders the filtered table; or, on no match, one stderr line +
   exit 1.
2. `weave check` → `run()` dispatches to the `check` branch → `extdir.Find` →
   `discover.Index` → `check.Check(dir, exts)` → main renders one OK/WARN/ERROR
   line per finding + a summary line; exit 0 unless any ERROR.

**Pain Points Addressed**: tag-discoverability (search) and silent-breakage
detection (check) — the two PRD §6.1 rows this milestone closes.

## Why

- **PRD §6.1**: `--search` (row 4) and `check` (row 5) are authoritative command
  rows. This task IS those two rows, assembled. Without it, `--search`/`check`
  are silent no-ops (parseArgs captures them, run() does not yet act).
- **Closes Milestone M4 (PRD §6.1 search/check, §9, build order step 4).**
  `init` (M4.T4) is the remaining M4 work; it is independent of these two
  branches.
- **Near-verbatim port minimizes risk**: skilldozer's `--search` and `check`
  branches are PROVEN, tested implementations. Porting them (with the four
  documented deltas) inherits the error-message wording, exit codes, and TTY-gate
  wiring skilldozer already settled. Each branch is ~15-25 lines.
- **Scope boundary**: this task touches ONLY `main.go` (dispatch) + `main_test.go`
  (tests). It does NOT modify any `internal/*` package, `parseArgs`, `go.mod`,
  or PRD.md. Exclusivity (e.g. `check --search` → exit 2) is M5.T1.S1; for now
  the branches coexist via dispatch ORDER, exactly as `--list`/`--all`/`tags`
  already do.

## What

### 1. Imports (main.go)

Add to the existing import block, internal group, alphabetical:
```go
"github.com/dabstractor/weave/internal/check"   // 'c' < 'd' → goes FIRST in the internal group
"github.com/dabstractor/weave/internal/search"  // between resolve and ui
```
No new stdlib imports (`fmt`, `io`, `os`, `path/filepath`, `strings` are all
already imported and used).

### 2. The `--search` branch (insert AFTER the `--list` block, BEFORE `--all`)

```go
// 4) --search / -s <q> (PRD §6.1). Filters the index to extensions where <q>
//    is a case-insensitive substring of the tag, package.json
//    name/description/keywords, weave.aliases, or weave.category
//    (internal/search), then renders the SAME table as --list via ui.PrintList
//    (PRD §6.1: "same table format as --list, filtered"). The filtered slice
//    keeps discover.Index's RelTag sort. Exit 0 with the table on matches;
//    exit 1 (stderr message, EMPTY stdout) when nothing matches (PRD §6.1:
//    "1 if no matches"). --no-color / TTY color gating is shared with --list;
//    --file/--relative do NOT apply (search prints a TABLE, not paths — PRD
//    §6.2: modifiers combine with tag resolution or --all only).
if c.searchMode {
	dir, _, err := extdir.Find() // src DISCARDED: --search does NOT print "(found via ...)"
	if err != nil {
		fmt.Fprintln(stderr, err) // one-line fix (PRD §6.4/§8); stdout stays empty
		return 1
	}
	exts, err := discover.Index(dir)
	if err != nil {
		fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
		return 1
	}
	matched := search.Search(c.searchQ, exts)
	if len(matched) == 0 {
		// PRD §6.1: exit 1 "if no matches". Mirror --list's "no extensions found"
		// convention: message to stderr, stdout stays clean.
		fmt.Fprintln(stderr, "no extensions matched "+c.searchQ)
		return 1
	}
	ui.PrintList(stdout, matched, isTerminal(stdout) && !c.noColor)
	return 0
}
```

### 3. The `check` branch (insert immediately AFTER the `--search` block)

```go
// 5) `weave check` subcommand (PRD §9). Validates every extension in the store
//    and prints a report: one OK line per clean extension, one line per finding
//    (ERROR/WARN), ending with a "N extensions, M errors, K warnings" summary.
//    Exit 0 if there are no ERRORs, 1 if there are any (WARNs never change the
//    exit code, so `if weave check; then …` works as a gate). An empty store is
//    clean (0 extensions, 0 errors, 0 warnings) → exit 0 (unlike --list, which
//    exits 1 on empty).
//
//    check is a REPORT, not a path emitter: it ALWAYS prints its full findings
//    to STDOUT (pipeable to less/grep, like eslint/ruff/govet) and signals
//    pass/fail via the exit code. It is NOT subject to §6.4's "nothing on
//    stdout on failure" — that contract is for tag/path emitters used inside
//    $(...); check never participates in command substitution.
//
//    check.Check(dir, exts) takes the extensions DIR first because the §9
//    empty-category-folder rule needs a filesystem walk (it cannot be derived
//    from []Extension alone — discover.Index prunes empty subtrees). `dir` is
//    already in scope from extdir.Find above. --file/--relative/--no-color do
//    NOT apply (status report, not paths/table).
if c.check {
	dir, _, err := extdir.Find() // src DISCARDED: check does NOT print "(found via ...)"
	if err != nil {
		fmt.Fprintln(stderr, err) // one-line fix (PRD §6.4/§8); stdout stays empty
		return 1
	}
	exts, err := discover.Index(dir)
	if err != nil {
		fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
		return 1
	}
	rep := check.Check(dir, exts)
	// Render: status word left-padded to width 5 (OK/WARN/ERROR align); a clean
	// extension gets ONE OK line, a problem extension gets ONE line PER finding.
	// Name falls back to the BASENAME of RelTag when package.json name is empty
	// (a single-file or metadata-less extension has no name) — NOT "(none)": the
	// item description pins "<name or basename>".
	for _, er := range rep.ByExt {
		name := er.Extension.Name
		if name == "" {
			name = relTagBase(er.Extension.RelTag)
		}
		if len(er.Findings) == 0 {
			fmt.Fprintf(stdout, "%-5s %s (%s)\n", "OK", er.Extension.RelTag, name)
			continue
		}
		for _, f := range er.Findings {
			fmt.Fprintf(stdout, "%-5s %s (%s): %s\n", f.Level, er.Extension.RelTag, name, f.Message)
		}
	}
	// N = len(exts): the count of discovered EXTENSIONS. rep.ByExt may include
	// synthetic empty-folder entries (§9), which are NOT extensions, so do NOT
	// use len(rep.ByExt) for the count.
	fmt.Fprintf(stdout, "%d extensions, %d errors, %d warnings\n", len(exts), rep.Errors, rep.Warnings)
	if rep.HasErrors() {
		return 1
	}
	return 0
}
```

### 4. The `relTagBase` helper (add next to `extensionPath`, bottom of main.go)

```go
// relTagBase returns the final '/'-component of a canonical RelTag, used as the
// display-name fallback in `weave check` when an extension has no package.json
// name (a single-file or metadata-less extension). "writing/reddit-poster" →
// "reddit-poster"; "example" → "example". It mirrors resolve's basename
// resolution (PRD §7.2) so the "(<name or basename>)" parenthetical in the
// check report is the SAME short name a user would type.
func relTagBase(relTag string) string {
	if i := strings.LastIndex(relTag, "/"); i >= 0 {
		return relTag[i+1:]
	}
	return relTag
}
```

### 5. Comment renumbering (cosmetic)

After insertion, renumber the trailing dispatch comments so they stay sequential:
`--list` stays `// 3)`, `--search` is `// 4)`, `check` is `// 5)`, `--all`
becomes `// 6)` (was `// 4)`), tag-resolution becomes `// 7)` (was `// 5)`), and
the no-op fallthrough becomes `// 8)` (was `// 6)`). Comment-only; no logic
change. (If you prefer, you may leave the existing numbers and just add the new
ones — but sequential is cleaner.)

### Success Criteria

- [ ] `weave --search <q>` with a match → TAG/NAME/DESCRIPTION table on stdout,
      exit 0, stderr empty.
- [ ] `weave --search <q>` with NO match → stderr `no extensions matched <q>`,
      stdout EMPTY, exit 1.
- [ ] `weave -s <q>` (short form) behaves identically to `--search <q>`.
- [ ] `weave check` on a clean store → one `OK` line per extension + summary
      `N extensions, 0 errors, 0 warnings`, exit 0, report on STDOUT.
- [ ] `weave check` on an empty store → `0 extensions, 0 errors, 0 warnings`,
      exit 0 (NOT exit 1).
- [ ] `weave check` with any ERROR → exit 1; the report is on STDOUT (pipeable).
- [ ] `weave check` with only WARNs → exit 0 (warnings never fail).
- [ ] `--file`/`--relative` are IGNORED by both `--search` and `check`
      (table/report, not paths — PRD §6.2).
- [ ] No changes to `parseArgs`, any `internal/*` package, or `go.mod`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the two branches to port are in
`/home/dustin/projects/skilldozer/main.go` (read in full), the four consumed
function/type contracts are pinned in `research/contract_notes.md` with exact
signatures and field names (incl. the `check.Check(dir, exts)` dir-first contract
from the parallel P1.M4.T2.S1 PRP), the insertion point and ordering are
specified, the exact wording deltas are tabulated, and the test-fixture
reliability for each ERROR/WARN case is verified against `discover.classifyDir`
source. No guessing required.

### Documentation & References

```yaml
# MUST READ — the branches to port
- file: /home/dustin/projects/skilldozer/main.go
  why: contains the proven --search branch (search.Search → ui.PrintList) and
       check branch (check.Check → render loop → HasErrors exit) to port nearly
       verbatim. Read the `if c.searchMode {…}` and `if c.check {…}` blocks.
  pattern: Find() → Index() → (search.Search | check.Check) → render; color gate
           `isTerminal(stdout) && !c.noColor`; check renders "%-5s %s (%s)" per
           finding + a summary line; exit 1 iff HasErrors().
  gotcha: skilldozer calls check.Check(skills); weave MUST call check.Check(dir,
          exts) — the dir-first signature is the parallel P1.M4.T2.S1 contract
          (the §9 empty-folder rule needs a FS walk). skilldozer's skilldozer.Check
          uses rep.BySkill/sr.Skill; weave's check uses rep.ByExt/er.Extension.

# The current file being modified (read the run() dispatch ladder + imports)
- file: main.go
  why: the file under edit. Locate the `--list` block (the insertion anchor) and
       the `--all` block (insert BEFORE it). The import block needs +check, +search.
  pattern: every dispatch branch is `if c.X { dir,_,err := extdir.Find(); …; return N }`
           — the Find→Index prefix is identical across --list/--all/--search/check.
           isTerminal + extensionPath are already defined here.
  gotcha: insert --search and check BETWEEN --list and --all (skilldozer order),
          NOT at the end. Renumber trailing comments to stay sequential.

# The check package CONTRACT (built in parallel by P1.M4.T2.S1)
- file: plan/001_19b4b465824d/P1M4T2S1/PRP.md
  why: defines check.Check(dir, exts) Report and the Report/ExtensionReport/
       Finding/Severity/HasErrors types this task CONSUMES. Treat as a contract.
  section: "What" §2-§3 (the type bodies + the three-pass Check signature).
  gotcha: Check's FIRST parameter is `dir` (string), second is `exts`. Field names
          are ByExt/Extension/Findings/Errors/Warnings (noun-swapped from
          skilldozer's BySkill/Skill). Severity has a VALUE-receiver String() so
          `fmt "%s"` on a Finding.Level works.

# The consumed gears (signatures confirmed by reading source)
- file: internal/search/search.go
  why: Search(query, exts) []discover.Extension — pure, order-preserving, empty-q matches all.
- file: internal/ui/ui.go
  why: PrintList(w, exts, useColor) — the SAME table renderer --list uses.
- file: internal/discover/index.go
  why: Index(dir) ([]Extension, error) — sorted by RelTag; empty store → (nil,nil).
- file: internal/extdir/extdir.go
  why: Find() (dir, src, err); ErrNotFound.Error() == the one-line fix string.

# The item's test-fixture reliability (why the ERROR/WARN fixtures work)
- docfile: plan/001_19b4b465824d/P1M4T3S1/research/contract_notes.md
  section: §5 (verified that a broken-package.json dir + index.ts is indexed by
           discover as a dir-kind ext AND flagged by check as an unparseable-JSON
           ERROR — the reliable, non-filesystem-dependent error trigger).

# PRD spec (authoritative)
- docfile: PRD.md
  section: §6.1 (the --search and check rows: exit codes, stdout shapes), §9
           (check output format: OK/<LEVEL> lines + summary line; exit 1 iff ERROR;
           NOT subject to §6.4's nothing-on-stdout).
```

### Current Codebase tree (relevant subset)

```bash
main.go                  # ← MODIFIED: +2 imports, +2 branches, +relTagBase
main_test.go             # ← MODIFIED: +~9 run-level tests
internal/
├── search/search.go     # Search() — CONSUMED (M4.T1.S1, done)
├── check/check.go       # Check(dir, exts) Report — CONSUMED (M4.T2.S1, parallel)
├── ui/ui.go             # PrintList() — CONSUMED (M2.T4.S1, done)
├── discover/{index,discover,extension,jsdoc}.go  # Index/Extension — CONSUMED
└── extdir/extdir.go     # Find() — CONSUMED
```

### Desired Codebase tree with files to be added/modified

```bash
main.go          # MODIFIED — +check/+search imports; --search branch; check branch; relTagBase
main_test.go     # MODIFIED — ~9 new TestRun* functions
# (no new files; no go.mod change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: call check.Check(dir, exts), NOT check.Check(exts). The item
// description writes "check.Check(exts)" but the P1.M4.T2.S1 PRP (the contract)
// defines Check(dir string, exts []discover.Extension) Report because the §9
// empty-category-folder rule needs a filesystem walk (discover.Index prunes empty
// subtrees, so an empty top-level folder is invisible to []Extension). `dir` is
// already in scope from `dir, _, err := extdir.Find()` in the same branch.

// CRITICAL: the check Report fields are noun-swapped from skilldozer.
// rep.ByExt (NOT BySkill); each element er.Extension (NOT er.Skill);
// er.Extension.Name / er.Extension.RelTag; er.Findings[i].Level / .Message.
// A stale skilldozer field name (BySkill/Skill) fails to compile — go build
// lists it.

// CRITICAL: the check name fallback is the BASENAME of RelTag, NOT "(none)".
// The item description pins "(<name or basename>)". skilldozer prints "(none)";
// weave diverges. relTagBase("writing/reddit-poster") == "reddit-poster".

// CRITICAL: the summary count N is len(exts), NOT len(rep.ByExt). check appends
// empty-category-folder findings as SYNTHETIC ExtensionReport entries in ByExt;
// those are folders, not extensions, and must not inflate the count. Matches
// skilldozer's len(skills).

// GOTCHA: insert --search and check BETWEEN --list and --all (skilldozer's
// order: list → search → check → all → tags). Each branch is a standalone
// `if … { return }`, so order = precedence when multiple modes are set
// (exclusivity exit-2 is M5.T1.S1, NOT this task). This is the SAME way
// --list/--all/tags already coexist via ordering today.

// GOTCHA: both branches DISCARD extdir.Find's `src` (2nd return) — only --path
// prints "(found via ...)". Use `dir, _, err := extdir.Find()`.

// GOTCHA: check prints its report to STDOUT (pipeable), even on failure. It is
// NOT subject to §6.4's "nothing on stdout on failure" — that contract is for
// path/tag emitters used inside $(...). check never participates in command
// substitution. --search, by contrast, DOES honor §6.4 (empty stdout on no-match).

// GOTCHA: `%-5s` left-pads the status word to width 5 so OK/WARN/ERROR align.
// "%s" on a check.Severity (Finding.Level) invokes its VALUE-receiver String()
// → "OK"/"WARN"/"ERROR". (Confirmed value-receiver in the check PRP.)

// GOTCHA: an empty query `weave --search ""` matches EVERY extension
// (strings.Contains(hay,"") is always true) → behaves like --list (exit 1 only
// if the store is empty). This is search.Search's natural semantics; do not
// special-case it. (search_test.go already pins TestSearchEmptyQueryMatchesAll.)
```

## Implementation Blueprint

### Data models and structure

None new. The consumed models are `[]discover.Extension` (from Index),
`check.Report`/`check.ExtensionReport`/`check.Finding` (from Check), and
`config{searchMode, searchQ, check, noColor …}` (already parsed). This task adds
only the `relTagBase` string helper (pure, no state).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD imports to main.go
  - EDIT main.go import block: add
      "github.com/dabstractor/weave/internal/check"
      "github.com/dabstractor/weave/internal/search"
    in the internal group, alphabetical (check first; search between resolve & ui).
  - RUN: go build ./...
  - EXPECT: compile error "imported and not used" until Task 2/3 add the calls —
    that is expected; proceed. (Or do Task 1+2+3 in one edit pass.)

Task 2: ADD the --search branch in run()
  - EDIT main.go: insert the --search `if c.searchMode { … }` block (see "What"
    §2) immediately AFTER the --list block's closing `return 0` and BEFORE the
    --all block. Prefix it with the `// 4) --search / -s <q> (PRD §6.1)` comment.
  - PORT FROM: skilldozer/main.go `if c.searchMode {…}` (search.Search → PrintList).
  - DELTAS vs skilldozer: "no skills matched"→"no extensions matched "+c.searchQ.
  - RUN: go build ./... ; go vet ./...
  - FILES TOUCHED: 1 (main.go).

Task 3: ADD the check branch in run()
  - EDIT main.go: insert the check `if c.check { … }` block (see "What" §3)
    immediately AFTER the --search block and BEFORE the --all block. Prefix with
    `// 5) \`weave check\` subcommand (PRD §9)`.
  - PORT FROM: skilldozer/main.go `if c.check {…}`.
  - DELTAS vs skilldozer: check.Check(skills)→check.Check(dir, exts);
    rep.BySkill→rep.ByExt; sr.Skill→er.Extension; "(none)"→relTagBase(RelTag);
    "%d skills"→"%d extensions"; N=len(exts).
  - RENumber trailing comments (--all→6), tags→7), no-op→8)) — cosmetic.
  - RUN: go build ./... ; go vet ./...
  - FILES TOUCHED: 1 (main.go).

Task 4: ADD the relTagBase helper
  - EDIT main.go: add `func relTagBase(relTag string) string` (see "What" §4)
    next to extensionPath at the bottom of main.go. It uses strings.LastIndex
    (strings is already imported).
  - FILES TOUCHED: 1 (main.go).

Task 5: VALIDATE main.go compiles + vets clean
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. Most likely failure: a stale skilldozer field name
    (BySkill/Skill) or calling check.Check(exts) without dir — go build/vet lists it.

Task 6: ADD ~9 run-level tests to main_test.go
  - EDIT main_test.go: add the tests under "--- run: --search (M4.T3.S1) ---" and
    "--- run: check (M4.T3.S1) ---" section headers, following the EXISTING
    `var out, errOut bytes.Buffer; code := run(args, &out, &errOut)` shape and the
    existing helpers (writeExtTree, sampleStore, writeDirExt, unsetExtEnv,
    withTerminal, t.Setenv). Tests (full bodies in Validation Loop §Level 2):
    --search: TestRunSearchMatch, TestRunSearchShortFlag, TestRunSearchNoMatchExit1,
              TestRunSearchEmptyQueryMatchesAll, TestRunSearchUnresolvableExit1.
    check:    TestRunCheckClean, TestRunCheckEmptyStoreClean,
              TestRunCheckWithErrorExit1, TestRunCheckWithWarningExit0,
              TestRunCheckUnresolvableExit1.
  - FIXTURE NOTE: the check ERROR fixture is `broken/package.json`="{ not json"
    + `broken/index.ts` (discover indexes it as dir-kind via case (b); check
    re-parses → ERROR). The WARN fixture is `nodesc.ts` with NO JSDoc.
  - FILES TOUCHED: 1 (main_test.go).

Task 7: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./... -v ; go test -race ./...
  - EXPECT: all green. The new branches must not disturb the existing --list/
    --all/<tag>/--version/--path tests (they are EARLIER in the dispatch order).
```

### Implementation Patterns & Key Details

```go
// The shared Find → Index prefix is IDENTICAL across --list/--search/--all/check.
// Both new branches reuse it verbatim (src discarded):
dir, _, err := extdir.Find()
if err != nil {
	fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline (PRD §6.4/§8)
	return 1
}
exts, err := discover.Index(dir)
if err != nil {
	fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
	return 1
}

// --search: filter then render the SAME table as --list (PRD §6.1).
matched := search.Search(c.searchQ, exts)
if len(matched) == 0 {
	fmt.Fprintln(stderr, "no extensions matched "+c.searchQ) // DELTA: "extensions" not "skills"
	return 1
}
ui.PrintList(stdout, matched, isTerminal(stdout) && !c.noColor)

// check: validate then render the report. NOTE dir-first Check signature.
rep := check.Check(dir, exts) // DELTA: skilldozer is check.Check(skills)
for _, er := range rep.ByExt { // DELTA: ByExt not BySkill
	name := er.Extension.Name // DELTA: er.Extension not er.Skill
	if name == "" {
		name = relTagBase(er.Extension.RelTag) // DELTA: basename, not "(none)"
	}
	if len(er.Findings) == 0 {
		fmt.Fprintf(stdout, "%-5s %s (%s)\n", "OK", er.Extension.RelTag, name)
		continue
	}
	for _, f := range er.Findings {
		fmt.Fprintf(stdout, "%-5s %s (%s): %s\n", f.Level, er.Extension.RelTag, name, f.Message)
	}
}
fmt.Fprintf(stdout, "%d extensions, %d errors, %d warnings\n", len(exts), rep.Errors, rep.Warnings)
// len(exts), NOT len(rep.ByExt): synthetic empty-folder entries are not extensions.
if rep.HasErrors() {
	return 1
}
```

### Integration Points

```yaml
CONSUMES:
  - extdir.Find()                  # (dir, src, err) — src discarded in both branches
  - discover.Index(dir)            # []Extension sorted by RelTag
  - search.Search(q, exts)         # --search only
  - check.Check(dir, exts)         # check only — dir FIRST
  - ui.PrintList(w, exts, color)   # --search only (same renderer as --list)
  - isTerminal(stdout)             # already defined in main.go (the TTY gate)

PRODUCES:
  - Two new run() dispatch branches (the final assembly of M4's search/check modes).
  - One pure helper, relTagBase (used only by the check renderer).

NO CHANGES TO:
  - go.mod / go.sum (stdlib + 4 present internal packages)
  - parseArgs (already captures --search/-s and check since M1.T4.S1)
  - any internal/* package
  - PRD.md / tasks.json
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1-4 (main.go edits):
go build ./...
go vet ./...

# Expected: zero errors. Most likely failure: a stale skilldozer field name
# (BySkill/Skill) or check.Check called without dir — go build/vet lists it.
# Also check: search/check imports added alphabetically in the internal group.
```

### Level 2: Unit Tests (Component Validation)

Add these tests to `main_test.go` under two new `--- run: --search (M4.T3.S1) ---`
and `--- run: check (M4.T3.S1) ---` section headers. They follow the EXISTING
`var out, errOut bytes.Buffer; code := run(args, &out, &errOut)` shape.

```go
// === --search ===

// --search with a match → filtered TAG table on stdout, exit 0, stderr empty.
func TestRunSearchMatch(t *testing.T) {
	dir := sampleStore(t) // example + writing/reddit-poster
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "reddit"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--search reddit): code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"TAG", "reddit-poster"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	// A non-matching extension must NOT appear.
	if strings.Contains(got, "example") {
		t.Errorf("stdout has 'example' (filter not applied):\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty", errOut.String())
	}
}

// -s short form behaves identically to --search.
func TestRunSearchShortFlag(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-s", "example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-s example): code=%d; want 0", code)
	}
	if !strings.Contains(out.String(), "example") {
		t.Errorf("stdout missing example:\n%s", out.String())
	}
}

// --search with NO match → exit 1, stderr "no extensions matched <q>", stdout EMPTY.
func TestRunSearchNoMatchExit1(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "zzz"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--search zzz): code=%d; want 1 (no matches)", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (§6.4: nothing on no-match)", out.String())
	}
	if !strings.Contains(errOut.String(), "no extensions matched zzz") {
		t.Errorf("stderr=%q; want 'no extensions matched zzz'", errOut.String())
	}
}

// --search "" matches EVERY extension (empty substring) → behaves like --list.
func TestRunSearchEmptyQueryMatchesAll(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", ""}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--search ''): code=%d; want 0 (empty query matches all)", code)
	}
	got := out.String()
	for _, want := range []string{"example", "reddit-poster"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q (empty query should match all):\n%s", want, got)
		}
	}
}

// --search when the dir is unresolvable → exit 1, stdout empty, one-line fix.
func TestRunSearchUnresolvableExit1(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // all §8.3 rules miss
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "x"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--search x) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}

// === check ===

// check on a clean store → one OK line per extension + summary, exit 0.
func TestRunCheckClean(t *testing.T) {
	dir := sampleStore(t) // example + writing/reddit-poster, both have JSDoc descs
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(check) clean: code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"OK", "example", "reddit-poster",
		"2 extensions, 0 errors, 0 warnings"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty (check prints to stdout)", errOut.String())
	}
}

// check on an EMPTY store → "0 extensions, 0 errors, 0 warnings", exit 0.
// (Unlike --list which exits 1 on empty; check is validation: nothing == nothing wrong.)
func TestRunCheckEmptyStoreClean(t *testing.T) {
	t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()) // exists, no entries
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("run(check) empty: code=%d; want 0 (empty store is clean)", code)
	}
	if !strings.Contains(out.String(), "0 extensions, 0 errors, 0 warnings") {
		t.Errorf("stdout=%q; want the 0/0/0 summary", out.String())
	}
}

// check with an ERROR → exit 1, the ERROR printed to STDOUT (pipeable).
// Fixture: a dir with a BROKEN package.json + an index.ts. discover indexes it
// as a dir-kind ext (case (b): broken JSON nulls Pi.Extensions, index.ts wins);
// check re-parses package.json(dir) → ERROR "package.json is not valid JSON".
func TestRunCheckWithErrorExit1(t *testing.T) {
	root := t.TempDir()
	brokenDir := filepath.Join(root, "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "package.json"),
		[]byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "index.ts"),
		[]byte("/** x */\nexport default function() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(check) with error: code=%d; want 1 (any ERROR)", code)
	}
	got := out.String()
	// The report is on STDOUT (check is a report, not a path emitter).
	if !strings.Contains(got, "ERROR") || !strings.Contains(got, "broken") {
		t.Errorf("stdout missing ERROR/broken line:\n%s", got)
	}
	if !strings.Contains(got, "1 errors") {
		t.Errorf("stdout summary missing '1 errors':\n%s", got)
	}
}

// check with only a WARN → exit 0 (warnings never change the exit code).
// Fixture: a single-file ext with NO JSDoc and no package.json → Description=""
// → check WARN "no description".
func TestRunCheckWithWarningExit0(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nodesc.ts"),
		[]byte("export default function() {}\n"), 0o644); err != nil { // NO JSDoc
		t.Fatal(err)
	}
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(check) with warning: code=%d; want 0 (warnings don't fail)", code)
	}
	got := out.String()
	if !strings.Contains(got, "WARN") || !strings.Contains(got, "nodesc") {
		t.Errorf("stdout missing WARN/nodesc line:\n%s", got)
	}
	if !strings.Contains(got, "0 errors, 1 warnings") {
		t.Errorf("stdout summary missing '0 errors, 1 warnings':\n%s", got)
	}
}

// check when the dir is unresolvable → exit 1, stdout empty, one-line fix.
func TestRunCheckUnresolvableExit1(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(check) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}
```

```bash
# Run after Task 6:
go test ./... -v
# Targeted re-run while debugging one branch:
go test -run 'TestRunSearch' ./... -v
go test -run 'TestRunCheck' ./... -v
go test -race ./...

# Expected: all green. On failure, the most common causes:
#   (a) check.Check called as Check(exts) instead of Check(dir, exts) — compile error.
#   (b) a stale skilldozer field name (BySkill/Skill) — compile error.
#   (c) the broken-pkg fixture not indexed — confirm discover sees it (case (b)
#       index.ts must be present alongside the broken package.json).
#   (d) N=len(rep.ByExt) instead of len(exts) — the summary count is wrong when
#       there's an empty-folder WARN.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and exercise both modes end-to-end against a temp store.
go build -o /tmp/weave .
STORE=$(mktemp -d)
printf '/** A demo extension. */\nexport default function() {}\n' > "$STORE/example.ts"
mkdir -p "$STORE/writing"
printf '/** Posts to reddit. */\nexport default function() {}\n' > "$STORE/writing/reddit-poster.ts"
# a broken entry (dir + broken package.json + index.ts) → check ERROR
mkdir -p "$STORE/broken"
printf '{ not json' > "$STORE/broken/package.json"
printf '/** x */\n' > "$STORE/broken/index.ts"

export weave_EXTENSIONS_DIR="$STORE"

echo "--- --search match ---"
/tmp/weave --search reddit; echo "exit=$?"            # table, exit 0
echo "--- --search no match ---"
/tmp/weave --search zzz; echo "exit=$?"               # stderr msg, exit 1
echo "--- check (has 1 ERROR) ---"
/tmp/weave check; echo "exit=$?"                       # report on stdout, exit 1
echo "--- check piped to grep (proves STDOUT) ---"
/tmp/weave check | grep ERROR; echo "exit=${PIPESTATUS[0]}"

# Clean-store check → exit 0
rm -rf "$STORE/broken"
/tmp/weave check; echo "exit=$?"                       # all OK, exit 0

unset weave_EXTENSIONS_DIR
rm -rf "$STORE" /tmp/weave
# Expected: search-match exit 0; search-no-match exit 1; check-with-error exit 1
# and the ERROR line visible on stdout; check-clean exit 0.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §6.4 contract probe: --search on no-match writes NOTHING to stdout (so
# `pi -e "$(weave --search zzz)"`-style misuse fails loudly), while check on
# failure DOES write its report to stdout (it's a report, not a path emitter).
STORE=$(mktemp -d); printf '/** d */\n' > "$STORE/example.ts"
export weave_EXTENSIONS_DIR="$STORE"
go build -o /tmp/weave .

echo "search-no-match stdout bytes: $(/tmp/weave --search zzz 2>/dev/null | wc -c)"   # expect 0
echo "check-failure   stdout bytes: $(/tmp/weave check 2>/dev/null | wc -c)"         # expect > 0

unset weave_EXTENSIONS_DIR; rm -rf "$STORE" /tmp/weave
# Expected: search=0 bytes (§6.4 honored); check=>0 bytes (report on stdout).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./... -v` — all tests pass (existing + ~9 new).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged).

### Feature Validation

- [ ] `weave --search <q>` match → TAG table on stdout, exit 0.
- [ ] `weave --search <q>` no-match → stderr `no extensions matched <q>`, stdout EMPTY, exit 1.
- [ ] `weave -s <q>` short form identical to `--search`.
- [ ] `weave check` clean → OK lines + `N extensions, 0 errors, 0 warnings`, exit 0.
- [ ] `weave check` empty store → `0 extensions, 0 errors, 0 warnings`, exit 0.
- [ ] `weave check` with ERROR → report on STDOUT, exit 1.
- [ ] `weave check` with only WARN → exit 0 (warnings don't fail).
- [ ] check calls `check.Check(dir, exts)` (dir-first), NOT `check.Check(exts)`.
- [ ] check name fallback is `relTagBase(RelTag)` (basename), NOT "(none)".
- [ ] summary count is `len(exts)`, NOT `len(rep.ByExt)`.
- [ ] `--file`/`--relative` ignored by both branches (table/report, not paths).

### Code Quality Validation

- [ ] Both branches follow the existing `dir, _, err := extdir.Find()` →
      `discover.Index(dir)` prefix used by `--list`/`--all`.
- [ ] `--search` and `check` inserted between `--list` and `--all`
      (skilldozer's `list → search → check → all → tags` order).
- [ ] Comments renumbered sequentially (cosmetic).
- [ ] `relTagBase` placed next to `extensionPath` (pure helpers at file bottom).
- [ ] No `parseArgs` change; no `internal/*` change.

### Documentation & Deployment

- [ ] Both branches have doc comments citing PRD §6.1 (--search) / §9 (check).
- [ ] The check branch's doc comment notes it is NOT subject to §6.4 (report,
      not path emitter) and explains the dir-first `Check(dir, exts)` signature.
- [ ] No README/docs changes (CLI usage is documented in README §4 in the final
      M6.T4/M6.T5 doc sweep — this task's item desc says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't call `check.Check(exts)` — the contract is `check.Check(dir, exts)`.
  The item description's shorthand omits `dir`; the P1.M4.T2.S1 PRP (built in
  parallel) defines the dir-first signature because the §9 empty-folder rule
  needs a filesystem walk.
- ❌ Don't use skilldozer's field names `rep.BySkill`/`sr.Skill` — weave's check
  package uses `rep.ByExt`/`er.Extension` (noun swap). A stale name fails to
  compile.
- ❌ Don't print "(none)" for a nameless extension in the check report — the item
  description pins "(<name or basename>)". Use `relTagBase(RelTag)`.
- ❌ Don't use `len(rep.ByExt)` for the summary count — it includes synthetic
  empty-folder entries (folders, not extensions). Use `len(exts)`.
- ❌ Don't print the check report to stderr — check is a REPORT: stdout, pipeable,
  NOT subject to §6.4's "nothing on stdout on failure" (that's for path/tag
  emitters used in `$(...)`). `--search`, by contrast, DOES honor §6.4 (empty
  stdout on no-match).
- ❌ Don't print `extdir.Find`'s `src` (the "(found via ...)" label) in either
  branch — that label is `--path`-only. Discard via `dir, _, err :=`.
- ❌ Don't apply `--file`/`--relative` to `--search` or `check` — both print a
  table/report, not paths (PRD §6.2: modifiers combine with tag resolution or
  `--all` only).
- ❌ Don't reorder `--all`/`tags` ahead of `--search`/`check` — the branches must
  sit between `--list` and `--all` (skilldozer's proven order). Exclusivity
  exit-2 is M5.T1.S1; for now order = precedence, same as today.
- ❌ Don't modify `parseArgs`, any `internal/*` package, `go.mod`, or PRD.md —
  this task is dispatch wiring + tests only.
- ❌ Don't hardcode the status-word padding — use `%-5s` (5 = len "ERROR"/"WARN "+
  alignment) so OK/WARN/ERROR columns line up, matching skilldozer exactly.

---

**Confidence Score: 9/10** for one-pass success. Both branches are near-verbatim
ports of skilldozer's proven, tested `--search`/`check` dispatch (read in full).
The four deltas from skilldozer are each pinned to a specific contract: the
`check.Check(dir, exts)` dir-first signature (P1.M4.T2.S1 PRP), the
`ByExt`/`Extension` field names (same PRP), the "name or basename" rule (item
description), and the "extensions"/"no extensions matched" noun swap. The consumed
gears (`search.Search`, `ui.PrintList`, `discover.Index`, `extdir.Find`) are all
landed and unit-tested. The check dispatch test's ERROR fixture (broken
package.json + index.ts) is verified reliable against `discover.classifyDir` case
(b). The ONE residual risk is depending on the parallel P1.M4.T2.S1 deliverable
(check package) — mitigated by treating its PRP as a hard contract and by `go
build` catching any field-name/signature drift the moment the two land together.
