# PRP — P1.M5.T1.S1: `--help` usage text + `run()` precedence ladder + `exclusivityError` + unknown-flag exit 2 + no-args default

> **Subtask:** The CLI-contract layer of weave (PRD §6.3/§6.4). Adds the user-facing
> `--help`/`-h` block, installs the full §6.3 precedence ladder at the TOP of `run()`
> (help → version → unknownFlag → exclusivity → init → dispatch), enforces §6.3 mutual
> exclusivity via a new `exclusivityError`, makes unknown dashed flags exit 2, and makes
> the no-recognized-mode / no-args case print usage to stderr + exit 1. PRD §6 mandates
> the contract is **byte-identical to skilldozer's** except `--file` — so the structure is
> ported from skilldozer `main.go` with `weave:` prefixes and weave nouns.

---

## Goal

**Feature Goal**: Enforce the complete §6.3/§6.4 CLI contract in `run()`: `--help`/`-h`
prints the usage block to **stdout** (exit 0) and **wins over everything else** (incl.
`--version` and unknown flags); unknown dashed flags print `weave: unknown flag '<x>'` to
**stderr** + exit **2**; mutually-exclusive mode combinations (per `exclusivityError`) exit
**2**; and any invocation that selects NO recognized mode (no args, or modifiers-only like
`--no-color`) prints usage to **stderr** + exit **1**. stdout stays EMPTY on every error
path (§6.4 $(...) discipline).

**Deliverable**: Additive edits to TWO existing files only:
1. `main.go` — (a) define `const usageText = <raw string>` + `func usage() string { return usageText }`;
   (b) insert the `c.help` branch at the very top of `run()` (before version); (c) insert the
   `c.unknownFlag != ""` → exit 2 branch and the `exclusivityError(c)` → exit 2 branch
   between the `--version` block and the `init` dispatch; (d) change the final `return 0`
   fallthrough to `fmt.Fprint(stderr, usage()); return 1`; (e) append `func exclusivityError`.
2. `main_test.go` — UPDATE `TestRunNoArgsIsNoOp` → `TestRunNoArgsPrintsUsageExit1` (exit 1 +
   usage on stderr), and ADD ~12 new tests for help/precedence/unknown/exclusivity/no-args.

NO new files. NO new imports (`fmt`, `io` already imported). NO `internal/*` change.
NO `go.mod`/`go.sum` change. NO `parseArgs` change (already captures every flag since
M1.T4.S1). NO `storeMissingValue` (weave deliberately defers that — see "Known Gotchas").

**Success Definition**:
- `go build ./...`, `go vet ./...`, `go test ./... -v`, and `gofmt -l main.go main_test.go`
  all pass (existing tests + 1 update + ~12 new).
- `--help` / `-h` → usage to **stdout**, exit 0; `--version` → version to stdout exit 0.
- `--bogus` / `-z` → `weave: unknown flag '<x>'` to **stderr**, stdout EMPTY, exit **2**.
- `--list --search foo`, `foo --list`, `foo --path`, `check foo`, `init --list`, `init foo`
  → one-line message to stderr, stdout EMPTY, exit **2**.
- No args / `--no-color` alone → usage to **stderr**, stdout EMPTY, exit **1**.
- `--help --version` → help wins (usage on stdout, exit 0). `--help --bogus` → help wins.
- `--version --bogus` → version wins (exit 0, version on stdout). (unknownFlag is checked
  AFTER version, so version masks it — matches skilldozer and the item's stated precedence.)

## User Persona (if applicable)

**Target User**: Two groups. (1) A human running `weave --help` to discover the flag matrix
and the canonical `pi -e "$(weave <tag>)"` one-liner. (2) Scripts/CI / shells driving
`$(weave <tag>)` — for whom the §6.4 stdout-empty-on-error discipline and exit codes
(1 vs 2) are load-bearing: `pi -e "$(weave --bogus)"` must fail loudly (empty $(...), exit
2), not pass a garbage path.

**Use Case**: `weave --help` (learn the CLI); `weave` with no args (typo / first run — get
usage + a non-zero exit so a wrapper script notices); `weave --list --search x` (a user
mistake — get a clear "mutually exclusive" message, exit 2, no partial output).

**User Journey**: `weave --help` → the full USAGE/EXAMPLES/OPTIONS/Exit-codes block on
stdout, exit 0, ready to pipe to `grep`/`less`. `weave --bogus` → `weave: unknown flag
'--bogus'` on stderr, nothing on stdout, exit 2 (a CI step `if weave ...; then` correctly
fails). `weave` (no args) → usage on stderr, exit 1 (parity with skilldozer /
get-server-config.sh).

**Pain Points Addressed**: `--help` is no longer a silent no-op (M1-M4 left it parsed-but-
undispatched, exit 0 with empty output). Unknown flags no longer fall through to a
silent exit 0. Mutually-exclusive modes no longer silently pick one by dispatch order
(surprising). No-args no longer exits 0 (which masked "the user invoked nothing useful").

## Why

- **Implements PRD §6.3 default behavior** — "No arguments and no flag ⇒ print usage to
  stderr, exit 1" and "`--help` / `--version` take precedence over everything else."
- **Implements PRD §6 header** — "Unknown flags ⇒ error + exit 2."
- **Implements PRD §6.3 exclusivity** — "Mixing `<tag>` with `--list`/`--search`/`--all`
  is an error (exit 2): these are mutually exclusive modes." (Extended per PRD §6
  byte-identity mandate to the full skilldozer set: tags+path, check+tags, check+mode,
  init+tags, init+mode — see "Known Gotchas" for the (b)+path reconciliation.)
- **Locks the §6.4 $(...) safety discipline** — EVERY error path (unknown flag, exclusivity,
  no-args) keeps stdout EMPTY so `pi -e "$(weave ...)"` fails loudly instead of passing a
  partial/garbage path. This is the one contract that makes weave safe inside command
  substitution.
- **Closes the M5.T1 milestone** — the last piece of the §6 CLI contract. After this, the
  §13 acceptance gate (P1.M6.T1.S1) can run the full error-semantics checks.

## What

[User-visible behavior: see Goal. The complete §6.1/§6.2/§6.3/§6.4 matrix is now enforced.]

### Success Criteria

- [ ] `const usageText` is a single Go raw-string literal placed at package scope (near
      `version` / `exampleExtensionTemplate`), structured as USAGE/EXAMPLES/OPTIONS/Exit-codes,
      byte-identical to skilldozer's structure with weave nouns (`weave`, `extension`,
      `pi -e "$(weave <tag>)"`, entry-file `--file`, `extensions` dir). Exactly one trailing
      `\n`, no trailing blank line.
- [ ] `func usage() string { return usageText }` is defined (wraps the const).
- [ ] `run()` has the 7-step precedence ladder (help → version → unknownFlag → exclusivity
      → init → dispatch → no-args) in that EXACT order. help wins over version; version wins
      over unknownFlag; unknownFlag wins over exclusivity (so `--bogus --list` reports the
      unknown flag); exclusivity gates init and dispatch.
- [ ] `c.help` → `fmt.Fprint(stdout, usage()); return 0` (STDOUT, exit 0).
- [ ] `c.unknownFlag != ""` → `fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag); return 2`.
- [ ] `exclusivityError(c)` → `fmt.Fprintln(stderr, msg); return 2`.
- [ ] Final fallthrough → `fmt.Fprint(stderr, usage()); return 1` (STDERR, exit 1).
- [ ] `func exclusivityError(c config) (bad bool, msg string)` implements the six families
      in order (a)-(f), each returning a one-line `weave:`-prefixed message; modifiers
      (`file`/`relative`/`noColor`) NEVER trigger it.
- [ ] `main_test.go`: `TestRunNoArgsIsNoOp` updated to `TestRunNoArgsPrintsUsageExit1`
      (exit 1 + usage on stderr + empty stdout); ~12 new tests added; `go test ./...` green.
- [ ] NO `storeMissingValue` introduced; NO `parseArgs` change; NO new imports.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** Every consumed symbol is pinned to a verified fact: `version`
(L43 var, `"dev"` under test), `parseArgs` already sets `c.help`/`c.unknownFlag`/`c.init`/
`c.tags`/all mode flags (M1.T4.S1, no parser change needed), `runInit` already exists
(L899, M4.T4.S2 landed). The insert anchors are the exact, unique comment strings
`// 1) --version` (L403) and `// 1.5) weave init dispatch` (L411), and the final `return 0`
(L643). The `usageText` body, the `exclusivityError` families, the exact message strings,
and the precise test bodies are all transcribed in `verified_facts.md` and the
Implementation Blueprint below. The one ambiguity (item desc family (b) omits `--path`) is
resolved with rationale (PRD §6 byte-identity mandate). An implementer who has never seen
this repo can complete it in one pass.

### Documentation & References

```yaml
# MUST READ — the verified facts (anchors + ladder + exclusivity families + usageText + tests)
- docfile: plan/001_19b4b465824d/P1M5T1S1/research/verified_facts.md
  why: "§1 = the two files + what is ALREADY landed (init dispatch at L411-423). §2 = CRITICAL:
        weave has NO storeMissingValue (ladder is one step shorter than skilldozer). §3 = the
        7-step ladder with EXACT insert anchors (before // 1) --version; between version and
        // 1.5) init; change the final return 0). §4 = exclusivityError's 6 families with the
        exact message strings + the (b)+path reconciliation decision. §5 = the usageText
        structure + weave nouns + the column-alignment FIX. §6 = the 1 test to UPDATE + the
        ~12 to ADD (with bodies). §7 = validation commands. §8 = scope discipline."
  critical: "§2 (no storeMissingValue) and §4 (include --path in family (b), matching PRD §6
             byte-identity) are the two places a naive skilldozer port goes WRONG."

# CONTRACT — skilldozer is the byte-identical reference (PRD §6 header)
- file: /home/dustin/projects/skilldozer/main.go
  why: "L52-97 = usageText (port structure, swap nouns). L431-560 = run() precedence ladder
        (port SHAPE; weave drops the storeMissingValue step 3.5). L722-770 = exclusivityError
        (port verbatim, swap skilldozer: → weave:). L695-700 = the no-args fallthrough
        (fmt.Fprint(stderr, usage()); return 1)."
  pattern: "help/version/unknownFlag/exclusivity are each a self-contained `if cond { fmt.Fx(stderr
            or stdout, ...); return N }` block BEFORE the mode dispatch. exclusivityError is a
            pure predicate over config (no I/O)."
  gotcha: "skilldozer has a storeMissingValue step 3.5 — weave does NOT (no field, no signal).
           Skip it. skilldozer's usageText has a latent alignment bug on the init/--store rows
           (col 20 not 21) — weave fixes it."

# CONTRACT — skilldozer's tests (port the SHAPE, apply weave deltas)
- file: /home/dustin/projects/skilldozer/main_test.go
  why: "The --help / help-beats-version / unknown-flag / exclusivity / no-args tests are the
        near-verbatim source. Deltas: binary 'weave' (not skilldozer); 'weave:' prefix; the
        `pi -e \"$(weave example)\"` one-liner substring; SKIP every *StoreNoValue* test
        (weave has no storeMissingValue)."

# The file under edit — locate symbols by NAME/anchor-comment (line numbers shift as edits land)
- file: main.go
  why: "THE edit target. run() lives here; the insert anchors are the comment lines // 1) --version
        (insert help BEFORE it), // 1.5) weave init dispatch (insert unknownFlag+exclusivity
        BETWEEN the version block and this), and the final // 8) ... return 0 (change to
        usage→stderr→return 1). version var (L43), usageText/exampleExtensionTemplate consts
        (package scope) are the placement neighborhood."
  pattern: "A run()-level precedence check = a self-contained `if c.<cond> { fmt.Fprint<w>(
            <stream>, <usage or msg>); return <code> }` block taking (stdout, stderr io.Writer)
            and returning int. exclusivityError is a pure `func(c config) (bool, string)` — no
            I/O, directly unit-testable."
  gotcha: "main.go ~L82 declares `type config struct`; internal/config is imported as configpkg
           (NOT relevant here — exclusivityError only reads config FIELDS, never the config pkg).
           The init dispatch (L420-422) is ALREADY present from M4.T4.S2; do NOT move or re-add it
           — exclusivity slots in ABOVE it and now correctly gates init."

# The file under edit — the single test to update + the new tests
- file: main_test.go
  why: "TestRunNoArgsIsNoOp (L554-563) is the ONE existing test that breaks (it asserts no-args
        → exit 0; new contract is exit 1 + usage on stderr). UPDATE it (rename + flip). ADD ~12
        new tests under a new section header. All other run()-level tests still PASS (verified:
        none of them pass --help, unknown flags, or exclusive combos). parseArgs tests are
        unaffected (parser-only)."
  pattern: "run()-level tests use `var out, errOut bytes.Buffer; code := run([]string{...},
            &out, &errOut)` then assert code (t.Fatalf) and stream contents (t.Errorf). The
            exclusivity/unknown/help tests need NO store fixture / NO unsetExtEnv / NO t.Chdir
            — they all exit BEFORE extdir.Find."
  gotcha: "The three existing TestRunVersionPrecedenceOver{Path,Tag,All} tests still PASS but
           their NAMES/comments now overstate ('version takes precedence over everything') since
           help now precedes version. Cosmetic; OPTIONAL non-blocking cleanup — do not let it
           expand scope."

# PRD spec (authoritative)
- docfile: PRD.md
  section: "§6 header ('byte-identical to Skilldozer's except --file'; 'Unknown flags ⇒ error
           + exit 2'), §6.1 (the command matrix the usageText enumerates), §6.2 (the 3 modifiers),
           §6.3 ('--help/--version take precedence over everything else'; 'Mixing <tag> with
           --list/--search/--all is an error (exit 2)'; 'No arguments and no flag ⇒ usage to
           stderr, exit 1'), §6.4 (stdout EMPTY on every error path — $(...) safety)."
```

### Current Codebase tree (relevant subset)

```bash
main.go          # ← MODIFIED: +const usageText; +func usage(); +help/unknownFlag/exclusivity branches in run(); +func exclusivityError; final return 0 → usage→stderr→return 1
main_test.go     # ← MODIFIED: UPDATE TestRunNoArgsIsNoOp → TestRunNoArgsPrintsUsageExit1; ADD ~12 tests
internal/        # NOT touched (config/extdir/discover/resolve/search/check/ui all consumed-but-unchanged)
```

### Desired Codebase tree with files to be added/modified

```bash
main.go          # MODIFIED — +usageText const; +usage() fn; +3 run() branches (help top, unknownFlag, exclusivity); +exclusivityError fn; final fallthrough → usage/stderr/exit1
main_test.go     # MODIFIED — 1 UPDATE + ~12 ADD under a new "--- run: CLI contract (M5.T1.S1) ---" header
# (no new files; no go.mod/go.sum change; no internal/* change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: weave has NO storeMissingValue. skilldozer's run() inserts a step 3.5
// (`if c.storeMissingValue → exit 2`); weave DELIBERATELY omits it — parseArgs's --store case
// comment says "no exit-2 'needs argument' here; the codebase defers that repo-wide", and
// `grep storeMissingValue` is empty. So weave's ladder is help → version → unknownFlag →
// exclusivity → init → dispatch → no-args (NO step 3.5). Do NOT port skilldozer's
// TestRun*StoreNoValue* family — there is no signal to assert on.

// CRITICAL: exclusivityError family (b) MUST include --path, even though the item description
// literally writes "tags + (list/searchMode/all)" (omitting path). Reconciliation: PRD §6
// header mandates the contract is "byte-identical to Skilldozer's", and skilldozer's Issue 3
// deliberately added tags+path to avoid SILENTLY DROPPING a stray tag on `weave foo --path`
// (without it, --path would print the dir and discard `foo` with no error). The item's own
// families (d) and (f) include --path, confirming path belongs in the set. Decision: check
// `len(c.tags) > 0 && (c.path || c.list || c.searchMode || c.all)` — match skilldozer / PRD.
// The item's stated tests (tag --list, check tag) still pass; we ADD tag --path coverage.

// CRITICAL: help is checked BEFORE version ("help wins" tiebreak, PRD §6.3). The current code
// checks version FIRST (it was the highest-precedence dispatch in M1-M4). So the help branch
// must be INSERTED ABOVE the `// 1) --version` comment — NOT below it. (If inserted below
// version, `--help --version` would print the version, violating §6.3.)

// CRITICAL: unknownFlag is checked AFTER version but BEFORE exclusivity. So:
//   `--version --bogus` → version wins (exit 0, version on stdout) — version masks the unknown.
//   `--bogus --list`    → unknown wins (exit 2, 'unknown flag') — unknown masks exclusivity.
//   `--help --bogus`    → help wins (exit 0, usage on stdout).
// This ordering is the item's stated precedence and matches skilldozer exactly. Do NOT reorder.

// CRITICAL: the init dispatch (main.go L420-422) is ALREADY present (M4.T4.S2 landed). Do NOT
// move or re-add it. exclusivityError slots in ABOVE it (between version and init), so init is
// now correctly GATED: `init --list` / `init foo` hit exclusivity → exit 2 before runInit runs.

// GOTCHA: every error path keeps stdout EMPTY (§6.4). help → stdout (it is NOT an error);
// unknownFlag/exclusivity/no-args → STDERR, and NOTHING on stdout. This is what makes
// `pi -e "$(weave --bogus)"` fail loudly (empty $(...)) instead of passing garbage.

// GOTCHA: the no-args / no-recognized-mode fallthrough is the SAME code path for `weave` (no
// args) and `weave --no-color` (modifiers only). Modifiers are NOT a mode — they combine with
// a mode. So modifiers-only selects no mode → usage to stderr, exit 1. (skilldozer's
// TestRunModifiersOnlyNoMode covers this.)

// GOTCHA: usageText is a SINGLE raw-string literal. The body contains an em-dash `—` (U+2014)
// in the tagline, NOT a hyphen-minus — matching skilldozer's tagline. The body contains NO
// backticks, so no `+ "`" +` splicing is needed. The closing backtick is on its OWN line so
// the string ends with exactly one trailing `\n` (no trailing blank line). Write it exactly
// as transcribed in the Implementation Blueprint.

// GOTCHA: fix the latent skilldozer OPTIONS-alignment bug. skilldozer's `init [<dir>]` and
// --store <dir> rows start their descriptions at column 20 (one space short of the other
// rows' column 21). weave pads every OPTIONS row so descriptions begin at column 21:
// option-spec field left-justified to width 16, then a fixed 3-space gap.
```

## Implementation Blueprint

### Data models and structure

None new. This task consumes the existing `config` struct fields (`help`, `version`,
`unknownFlag`, `init`, `check`, `path`, `list`, `searchMode`, `all`, `tags`, `file`,
`relative`, `noColor` — all populated by `parseArgs` since M1.T4.S1) and the existing
`version` var. It produces: one `const` (string), one tiny `func usage()`, three new
`if`-blocks in `run()`, one `func exclusivityError`, and a one-line change to the final
return. No structs, no interfaces, no state.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: DEFINE const usageText + func usage() in main.go
  - EDIT main.go: add `const usageText = <raw string>` at package scope, placed NEAR the
    other top-level consts (e.g. right after `exampleExtensionTemplate`'s closing backtick,
    BEFORE run()). Immediately follow with `func usage() string { return usageText }`. Add
    a doc comment citing PRD §6.1/§6.3, the skilldozer byte-identity mandate, the weave noun
    deltas, the column-alignment fix, and "same text → stdout for --help, stderr for no-args".
  - CONTENT: the EXACT body in "Implementation Patterns" below (USAGE/EXAMPLES/OPTIONS/Exit
    codes; weave nouns; `pi -e "$(weave <tag>)"`; entry-file --file; em-dash tagline; col-21
    alignment; exactly one trailing \n).
  - VERIFY: `grep -c '`' ` on the body returns 0 (no splicing needed).
  - RUN: go build ./...  (the const/func are unused until Task 2 references usage(); Go allows
    unused package-level consts/funcs, so build passes.)
  - FILES TOUCHED: 1 (main.go).

Task 2: INSERT the help branch at the TOP of run()
  - EDIT main.go: in run(), insert IMMEDIATELY BEFORE the `// 1) --version (PRD §6.3 …)`
    comment (L403):
        // 0) --help / -h (PRD §6.3 "help wins"): takes precedence over EVERYTHING, including
        //    --version and unknown flags. Usage to STDOUT, exit 0. Help is PLAIN (no ANSI).
        if c.help {
            fmt.Fprint(stdout, usage())
            return 0
        }
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. (parseArgs already sets c.help since M1.T4.S1.)
  - FILES TOUCHED: 1 (main.go).

Task 3: INSERT the unknownFlag + exclusivity branches between version and init
  - EDIT main.go: in run(), insert AFTER the `if c.version { … return 0 }` block's closing
    `}` and BEFORE the `// 1.5) weave init dispatch (PRD §8.2) …` comment (L411):
        // 1.1) Unknown dashed flag → exit 2 (PRD §6 header). stdout stays EMPTY (§6.4). Checked
        //      AFTER --help/--version so those still win; BEFORE exclusivity so --bogus --list
        //      reports the unknown flag first (both exit 2; unknown is the more fundamental err).
        if c.unknownFlag != "" {
            fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag)
            return 2
        }

        // 1.2) Mode mutual exclusivity → exit 2 (PRD §6.3). Pure predicate over config;
        //      no I/O. GATES init (below) and the mode dispatch: any exclusive combo exits
        //      before runInit/Find/Index ever run. --file/--relative/--no-color are modifiers
        //      and never trigger it (exclusivityError excludes them).
        if bad, msg := exclusivityError(c); bad {
            fmt.Fprintln(stderr, msg)
            return 2
        }
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean (exclusivityError lands in Task 4; do Task 3+4 together to avoid a transient
    "undefined: exclusivityError" during the build). The init dispatch (L420-422) STAYS where
    it is — it is now correctly gated.
  - FILES TOUCHED: 1 (main.go).

Task 4: APPEND func exclusivityError to main.go
  - EDIT main.go: append `func exclusivityError(c config) (bad bool, msg string)` AFTER the
    last run()-level helper (e.g. after `relTagBase`, near the other run-supporting funcs).
    Add the doc comment (cites PRD §6.3; the 6 families; the (b)+path decision; modifiers
    excluded). Port from skilldozer main.go L722-770 verbatim, swap `skilldozer:` → `weave:`.
  - CONTENT: the EXACT body in "Implementation Patterns" below (6 families, in order).
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean.
  - FILES TOUCHED: 1 (main.go).

Task 5: CHANGE the final fallthrough in run() to usage→stderr→exit 1
  - EDIT main.go: replace the final `// 8) All other parsed modes are no-ops …` comment + its
    `return 0` (L640-643) with:
        // No recognized mode → usage to STDERR, exit 1 (PRD §6.3: parity with
        // get-server-config.sh / skilldozer). Covers both truly-no-args and modifiers-only
        // (e.g. `weave --no-color`): if weave was asked to DO nothing, show usage. stdout
        // stays EMPTY so $(...) never sees garbage.
        fmt.Fprint(stderr, usage())
        return 1
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. (This is the line TestRunNoArgsIsNoOp currently keys on; Task 6 updates it.)
  - FILES TOUCHED: 1 (main.go).

Task 6: UPDATE TestRunNoArgsIsNoOp + ADD the new tests
  - EDIT main_test.go:
    (a) UPDATE `TestRunNoArgsIsNoOp` (L554-563): rename to `TestRunNoArgsPrintsUsageExit1`,
        flip `code != 0` → `code != 1`, and change the output assertion to: stdout EMPTY +
        stderr CONTAINS "USAGE". (Body in Validation Loop §Level 2.)
    (b) ADD a new section header `// --- run: CLI contract — help / precedence / unknown /
        exclusivity / no-args (M5.T1.S1) ---` and append the ~12 tests (bodies in Validation
        Loop §Level 2). Port the SHAPE from skilldozer main_test.go; deltas = binary `weave`,
        `weave:` prefix, `pi -e "$(weave example)"` substring, SKIP the *StoreNoValue* family.
  - RUN: go test ./... -v
  - EXPECT: all green (existing + 1 updated + ~12 new).
  - FILES TOUCHED: 1 (main_test.go).

Task 7: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./... -v ; gofmt -l main.go main_test.go
  - EXPECT: all green; gofmt output empty.
```

### Implementation Patterns & Key Details

```go
// usageText — the full --help / no-args usage block (PRD §6.1, §6.3). Byte-identical in
// STRUCTURE to skilldozer's (PRD §6 header), with weave nouns: binary `weave`, `extension`,
// the canonical `pi -e "$(weave <tag>)"` one-liner, --file = ENTRY FILE (not SKILL.md), and
// `extensions` dir. PLAIN (no ANSI): `weave --help | grep` must work and tests use non-TTY
// buffers. The SAME text is printed to STDOUT for --help (exit 0) and to STDERR for the
// no-args default (exit 1) — only the destination differs. Em-dash tagline; OPTIONS aligned
// to column 21 (fixes skilldozer's latent init/--store misalignment); exactly one trailing \n.
const usageText = `weave — manifest-free extension path printer

Resolve extension tags to on-disk extension paths (manifest-free).

USAGE:
  weave <tag> [<tag>...]
  weave --all
  weave --list
  weave --search <query>
  weave check
  weave init [<dir>]
  weave --path
  weave --help
  weave --version

EXAMPLES:
  pi -e "$(weave example)"
  pi -e "$(weave writing/reddit)"
  weave example reddit          # one absolute path per line, input order
  weave -f example              # print the entry file path
  weave --relative --all        # every extension path, relative to the extensions dir
  weave --list                  # human-readable catalog
  weave --search reddit         # substring search over tag/name/description/keywords/aliases/category
  weave check                   # validate every extension on disk
  weave init --store <dir>      # non-interactive first-run setup

OPTIONS:
  <tag> [<tag>...]   Resolve tags to extension paths (one absolute path per line)
  --all, -a          Print every extension's path, sorted by tag
  --list, -l         Human-readable catalog (TAG, NAME, DESCRIPTION)
  --search <q>, -s   Substring search over tag / name / description / keywords / aliases / category
  check              Validate every extension on disk (report OK / WARN / ERROR)
  init [<dir>]       First-run setup: pick/create the extensions store and write the config
  --store <dir>      Non-interactive store path for init
  --path, -p         Print the resolved extensions directory (discovery rule printed to stderr)
  --file, -f         Print the entry file path instead of the resolvable path (modifier)
  --relative         Print paths relative to the extensions directory (modifier)
  --no-color         Disable ANSI color even on a TTY (modifier)
  --help, -h         Show this help message
  --version, -v      Print the weave version

Exit codes: 0 success/help/version | 1 unresolved/no extensions/unresolvable dir | 2 unknown flag / mutually-exclusive modes
`

// usage returns the help block. A tiny indirection so the constant is wrapped by a function
// (keeps the print sites uniform: fmt.Fprint(w, usage())).
func usage() string { return usageText }

// In run(), the THREE new precedence blocks (insert in this order):

//   // 0) --help / -h (PRD §6.3 "help wins"): precedes EVERYTHING (version, unknown, exclusivity).
//   if c.help {
//       fmt.Fprint(stdout, usage())
//       return 0
//   }
//   … (the existing `if c.version { … return 0 }` stays here, unchanged) …
//   // 1.1) unknown flag → exit 2 (after version, before exclusivity).
//   if c.unknownFlag != "" {
//       fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag)
//       return 2
//   }
//   // 1.2) exclusivity → exit 2 (gates the init dispatch and the mode ladder below).
//   if bad, msg := exclusivityError(c); bad {
//       fmt.Fprintln(stderr, msg)
//       return 2
//   }
//   … (the existing `if c.init { return runInit(c, stdout, stderr) }` stays, now gated) …

// exclusivityError reports whether c combines modes that PRD §6.3 forbids, returning a
// one-line stderr message. Six families, checked in order (first hit wins). Ported verbatim
// from skilldozer main.go exclusivityError, with `skilldozer:` → `weave:`. --file/--relative/
// --no-color are MODIFIERS and never trigger it (they combine with a single mode).
//
// FAMILY (b) NOTE: the item description's (b) literally omits --path, but PRD §6 mandates the
// contract is byte-identical to skilldozer (whose Issue 3 deliberately added tags+path to
// avoid silently dropping a stray tag on `weave foo --path`), and the item's own (d)/(f)
// include path. So (b) checks the FULL set {path,list,searchMode,all}.
func exclusivityError(c config) (bad bool, msg string) {
	// (a) Issue 6: any 2+ of the listing modes are mutually exclusive.
	n := 0
	for _, b := range []bool{c.path, c.list, c.searchMode, c.all} {
		if b {
			n++
		}
	}
	if n >= 2 {
		return true, "weave: listing modes --path/--list/--search/--all are mutually exclusive"
	}
	hasTags := len(c.tags) > 0
	// (b) tags + an inspection mode (PRD §6.3; +path per byte-identity — see doc comment).
	if hasTags && (c.path || c.list || c.searchMode || c.all) {
		return true, "weave: tags cannot be combined with --path/--list/--search/--all"
	}
	// (c) check + tags.
	if c.check && hasTags {
		return true, "weave: 'check' cannot be combined with tag arguments"
	}
	// (d) check + a listing mode.
	if c.check && (c.path || c.list || c.searchMode || c.all) {
		return true, "weave: 'check' cannot be combined with --path/--list/--search/--all"
	}
	// (e)+(f) init is its own exclusive mode (PRD §6.3 / §8.2): rejects stray tags AND modes.
	if c.init {
		if hasTags {
			return true, "weave: 'init' cannot be combined with tag arguments"
		}
		if c.check || c.list || c.searchMode || c.all || c.path {
			return true, "weave: 'init' cannot be combined with --list/--search/--all/--path/check"
		}
	}
	return false, ""
}

// The final fallthrough change (replace the existing `// 8) … return 0`):
//   // No recognized mode → usage to STDERR, exit 1 (PRD §6.3). Covers no-args AND
//   // modifiers-only (e.g. `weave --no-color`): asked to DO nothing → show usage.
//   fmt.Fprint(stderr, usage())
//   return 1
```

### Integration Points

```yaml
CONSUMES:
  - config.help / config.unknownFlag              # set by parseArgs since M1.T4.S1 (NO parser change)
  - config.version / init / check / path / list / searchMode / all / tags  # existing mode flags
  - config.file / relative / noColor              # MODIFIERS — exclusivityError EXCLUDES them
  - var version                                   # already present (L43); "dev" under `go test`
  - func runInit(c, stdout, stderr) int           # M4.T4.S2 (landed, L899); init dispatch already in run()

PRODUCES:
  - const usageText (string)                      # the §6.1/§6.3 usage block (weave nouns)
  - func usage() string                           # wraps the const for uniform print sites
  - the 3 run() precedence branches (help / unknownFlag / exclusivity)
  - the final-fallthrough change (return 0 → usage→stderr→return 1)
  - func exclusivityError(c config) (bad bool, msg string)

NO CHANGES TO:
  - parseArgs (already captures every flag since M1.T4.S1 — help, unknownFlag, all modes).
  - the import block (fmt, io already imported; exclusivityError is pure, usage() uses only fmt).
  - the init dispatch block (already present from M4.T4.S2 — exclusivity now gates it, do not move).
  - any internal/* package.
  - go.mod / go.sum.
  - PRD.md / tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1-5 (main.go edits):
go build ./...
go vet ./...
gofmt -l main.go main_test.go   # after Task 6 too

# Expected: zero errors / empty gofmt output. Most likely failures:
#   (a) "undefined: exclusivityError" during Task 3 — do Task 3 + Task 4 together (the branch
#       references exclusivityError before it is defined). Build is whole-file.
#   (b) a stale skilldozer string in a message ("skilldozer:" / "skill") → grep:
#         grep -ni "skilldozer\|skills\b\|SKILL.md" main.go
#       (the word "extension"/"extensions" is CORRECT; flag only the stale ones.)
#   (c) inserted help BELOW version → `--help --version` prints the version (TestRunHelpBeatsVersion
#       fails). Fix: help goes ABOVE the `// 1) --version` comment.
#   (d) family (b) omits c.path → `weave foo --path` silently resolves (TestRunExclusivityTagsAndPath
#       fails). Fix: add c.path to the (b) condition (see Known Gotchas).
#   (e) changed the final `return 0` to stderr+exit1 but forgot usage() → compile/runtime miss.
#       Fix: `fmt.Fprint(stderr, usage()); return 1`.
```

### Level 2: Unit Tests (Component Validation)

First UPDATE `TestRunNoArgsIsNoOp` (main_test.go L554-563):

```go
// No args → usage to STDERR, exit 1 (PRD §6.3). stdout stays EMPTY so $(...) sees nothing.
// (Was TestRunNoArgsIsNoOp in M1-M4, asserting exit 0; the no-args→usage→exit 1 path lands here.)
func TestRunNoArgsPrintsUsageExit1(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(nil, &out, &errOut)
	if code != 1 {
		t.Errorf("run(nil): code=%d; want 1 (no-args → usage on stderr, exit 1)", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(nil) stdout=%q; want EMPTY (usage goes to stderr)", out.String())
	}
	if !strings.Contains(errOut.String(), "USAGE") {
		t.Errorf("run(nil) stderr=%q; want the usage block", errOut.String())
	}
}
```

Then APPEND the new tests under a `// --- run: CLI contract — help / precedence / unknown / exclusivity / no-args (M5.T1.S1) ---` header. None of these need a store fixture / `unsetExtEnv` / `t.Chdir` — they all exit BEFORE `extdir.Find`:

```go
// --help → usage on STDOUT, exit 0, no ANSI, stderr empty.
func TestRunHelpToStdoutExit0(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--help): code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"USAGE:", "EXAMPLES:", "OPTIONS:", `pi -e "$(weave example)"`} {
		if !strings.Contains(got, want) {
			t.Errorf("run(--help) stdout missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") { // help is PLAIN
		t.Errorf("run(--help) must not emit ANSI:\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--help) stderr=%q; want empty", errOut.String())
	}
}

// -h short form behaves identically.
func TestRunHelpShortFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-h"}, &out, &errOut); code != 0 {
		t.Fatalf("run(-h): code=%d; want 0", code)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Errorf("run(-h) stdout=%q; want the usage block", out.String())
	}
}

// --help wins over --version (PRD §6.3 "help wins"): stdout IS usage, NOT the version line.
func TestRunHelpBeatsVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--help", "--version"}, &out, &errOut); code != 0 {
		t.Fatalf("run(--help --version): code=%d; want 0 (help wins)", code)
	}
	if strings.Contains(out.String(), "weave "+version) {
		t.Errorf("stdout must NOT contain the version line (help won):\n%s", out.String())
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Errorf("stdout must BE the usage block:\n%s", out.String())
	}
}

// --help wins over an unknown flag too.
func TestRunHelpBeatsUnknownFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--help", "--bogus"}, &out, &errOut); code != 0 {
		t.Fatalf("run(--help --bogus): code=%d; want 0 (help wins)", code)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Errorf("stdout must BE the usage block (help won):\n%s", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty (help wins, no error printed)", errOut.String())
	}
}

// --version wins over an unknown flag (unknown is checked AFTER version).
func TestRunVersionBeatsUnknownFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--version", "--bogus"}, &out, &errOut); code != 0 {
		t.Fatalf("run(--version --bogus): code=%d; want 0 (version wins)", code)
	}
	if got := out.String(); got != "weave "+version+"\n" {
		t.Errorf("stdout=%q; want the version line (version beats unknown)", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty", errOut.String())
	}
}

// Unknown long flag → exit 2, 'unknown flag' to stderr, stdout EMPTY (§6.4).
func TestRunUnknownFlagExit2(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(--frobnicate): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (§6.4)", out.String())
	}
	if want := "weave: unknown flag '--frobnicate'\n"; errOut.String() != want {
		t.Errorf("stderr=%q; want %q", errOut.String(), want)
	}
}

// Unknown short flag → exit 2 (a single unknown char like -z).
func TestRunUnknownShortFlagExit2(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"-z"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(-z): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if want := "weave: unknown flag '-z'\n"; errOut.String() != want {
		t.Errorf("stderr=%q; want %q", errOut.String(), want)
	}
}

// 2+ listing modes → exit 2, "mutually exclusive" on stderr, stdout EMPTY.
func TestRunExclusivityListAndSearch(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--list", "--search", "foo"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(--list --search foo): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "mutually exclusive") {
		t.Errorf("stderr=%q; want 'mutually exclusive'", errOut.String())
	}
}

// tags + --path → exit 2 (family (b) WITH path, per PRD §6 byte-identity).
func TestRunExclusivityTagsAndPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"foo", "--path"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(foo --path): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (no silent tag drop)", out.String())
	}
	if !strings.Contains(errOut.String(), "cannot be combined") {
		t.Errorf("stderr=%q; want 'cannot be combined'", errOut.String())
	}
}

// tags + --list → exit 2 (PRD §6.3 explicit).
func TestRunExclusivityTagsAndList(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"foo", "--list"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(foo --list): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "cannot be combined") {
		t.Errorf("stderr=%q; want 'cannot be combined'", errOut.String())
	}
}

// check + tags → exit 2.
func TestRunExclusivityCheckAndTags(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"check", "foo"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(check foo): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "check") {
		t.Errorf("stderr=%q; want 'check' in the message", errOut.String())
	}
}

// init + --list → exit 2 (init is exclusive).
func TestRunExclusivityInitAndList(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"init", "--list"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(init --list): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "init") {
		t.Errorf("stderr=%q; want 'init' in the message", errOut.String())
	}
}

// init + a stray tag → exit 2.
func TestRunExclusivityInitAndStrayTag(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"init", "foo"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("run(init foo): code=%d; want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "tag") {
		t.Errorf("stderr=%q; want 'tag' in the message", errOut.String())
	}
}

// Modifiers-only (no mode) → exit 1, usage on stderr, stdout EMPTY. Modifiers are NOT a mode.
func TestRunModifiersOnlyNoMode(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--no-color"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--no-color): code=%d; want 1 (modifiers-only → no mode)", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "USAGE") {
		t.Errorf("stderr=%q; want the usage block", errOut.String())
	}
}
```

```bash
# After Task 6:
go test ./... -v
# Targeted re-run while debugging:
go test -run 'TestRunHelp|TestRunUnknown|TestRunExclusivity|TestRunNoArgs|TestRunVersionBeats|TestRunModifiersOnly' -v

# Expected: all green. On failure, the most common causes:
#   (a) TestRunHelpBeatsVersion fails (stdout has the version line) → help inserted below version.
#   (b) TestRunExclusivityTagsAndPath fails (exit 0/1, not 2) → family (b) omits c.path. Add it.
#   (c) TestRunUnknownFlagExit2 stderr mismatch → message must be EXACTLY
#       "weave: unknown flag '--frobnicate'\n" (note the single quotes around the flag, the
#       space after the colon, lowercase 'weave').
#   (d) TestRunNoArgs/stdout non-empty → you printed usage to stdout instead of stderr in the
#       final fallthrough. Fix: fmt.Fprint(stderr, usage()) in the fallthrough (stdout only in --help).
#   (e) a stale "skilldozer"/"skills" string in usageText or a message → grep -ni.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and exercise the contract end to end (no real config needed — error paths
# run before any dir resolution).
go build -o /tmp/weave .

# --help → usage on STDOUT, exit 0.
/tmp/weave --help | head -5            # tagline + USAGE block on stdout
echo "help exit=${PIPESTATUS[0]}"      # expect 0
/tmp/weave --help 2>/dev/null | grep -q 'pi -e "$(weave example)"' && echo "one-liner OK"

# no-args → usage on STDERR, exit 1, NOTHING on stdout.
out=$(/tmp/weave 2>/tmp/err); rc=$?
[ -z "$out" ] && [ "$rc" = "1" ] && grep -q USAGE /tmp/err && echo "no-args OK"

# unknown flag → STDERR 'unknown flag', exit 2, nothing on stdout.
out=$(/tmp/weave --bogus 2>/tmp/err); rc=$?
[ -z "$out" ] && [ "$rc" = "2" ] && grep -q "unknown flag '--bogus'" /tmp/err && echo "unknown-flag OK"

# exclusivity → exit 2, nothing on stdout.
out=$(/tmp/weave --list --search x 2>/dev/null); rc=$?
[ -z "$out" ] && [ "$rc" = "2" ] && echo "exclusivity OK"

# help precedence: --help --version → help wins (usage, not version).
/tmp/weave --help --version 2>/dev/null | grep -q USAGE && echo "help-beats-version OK"
/tmp/weave --help --version 2>/dev/null | grep -qv '^weave dev$' && echo "no-version-line OK"

# Expected: every check prints its "… OK" line; no check fails.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §6.4 $(...) safety: every error path yields an EMPTY $(...) so `pi -e "$(weave …)"` fails
# loudly rather than passing a garbage path.
[ -z "$(/tmp/weave --bogus 2>/dev/null)" ]      && echo "unknown-flag empty-\$() OK"
[ -z "$(/tmp/weave 2>/dev/null)" ]              && echo "no-args empty-\$() OK"
[ -z "$(/tmp/weave --list --search x 2>/dev/null)" ] && echo "exclusivity empty-\$() OK"
[ -z "$(/tmp/weave foo --path 2>/dev/null)" ]   && echo "tags+path empty-\$() OK"

# usageText byte-shape: no ANSI, single trailing newline, the em-dash tagline.
/tmp/weave --help | cat -A | head -1            # expect "weave M-bM-^YM-^D manifest-free …" (em-dash UTF-8)
[ -z "$(/tmp/weave --help | grep $'\x1b')" ]    && echo "no-ANSI OK"
# (the em-dash line shows M-bM-^YM-^D in cat -A = the UTF-8 bytes e2 80 94 of U+2014)
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test ./... -v` green (existing + 1 updated + ~12 new).
- [ ] `gofmt -l main.go main_test.go` empty.

### Feature Validation

- [ ] `--help`/`-h` → usage on **stdout**, exit 0, no ANSI, contains `pi -e "$(weave example)"`.
- [ ] `--help --version` → help wins (usage, NOT version line).
- [ ] `--version` → `weave <version>` on stdout, exit 0 (unchanged).
- [ ] `--bogus`/`-z` → `weave: unknown flag '<x>'` on stderr, stdout EMPTY, exit 2.
- [ ] `--list --search foo`, `foo --list`, `foo --path`, `check foo`, `init --list`, `init foo`
      → message on stderr, stdout EMPTY, exit 2.
- [ ] No args / `--no-color` alone → usage on **stderr**, stdout EMPTY, exit 1.
- [ ] §6.4: every error path yields an EMPTY `$(weave …)`.

### Code Quality Validation

- [ ] `usageText` mirrors skilldozer's structure with weave nouns; OPTIONS aligned to col 21;
      exactly one trailing newline.
- [ ] `exclusivityError` is a pure predicate (no I/O), 6 families in order, `weave:` prefix,
      modifiers excluded, family (b) includes `--path` (PRD §6 byte-identity).
- [ ] NO `storeMissingValue` introduced; NO `parseArgs` change; NO new imports.
- [ ] The init dispatch block (M4.T4.S2) is untouched — exclusivity now correctly gates it.

### Documentation & Deployment

- [ ] `usageText` IS the user-facing help (self-documenting; README §4 is a Mode B final task).
- [ ] The run() precedence comments cite PRD §6.3 and explain the ordering (why help→version→
      unknown→exclusivity, and why unknown before exclusivity).

---

## Anti-Patterns to Avoid

- ❌ Don't insert `--help` BELOW `--version` — §6.3 "help wins" requires help checked FIRST.
- ❌ Don't port skilldozer's `storeMissingValue` step — weave has no such field/signal.
- ❌ Don't omit `--path` from exclusivityError family (b) — it would silently drop a stray tag
  on `weave foo --path` (skilldozer's Issue 3 bug). PRD §6 mandates byte-identity.
- ❌ Don't print usage to stdout in the no-args/error fallthrough — usage goes to STDERR there
  (stdout only for `--help`). stdout must stay EMPTY on every error path (§6.4).
- ❌ Don't reorder the precedence (unknown before version, or exclusivity before unknown) — the
  item's stated ordering makes `--version --bogus` → version wins and `--bogus --list` → unknown
  wins, matching skilldozer exactly.
- ❌ Don't move or re-add the init dispatch — it already exists (M4.T4.S2); exclusivity slots
  ABOVE it.
- ❌ Don't add a `switch` or new imports for exclusivityError — it's a pure `func(c config)
  (bool, string)` reading existing fields.
- ❌ Don't let the "version takes precedence over everything" comment/names on the 3 existing
  `TestRunVersionPrecedenceOver*` tests expand scope — they still PASS; cosmetic cleanup is
  OPTIONAL and non-blocking.
