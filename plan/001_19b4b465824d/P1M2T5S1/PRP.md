# PRP — P1.M2.T5.S1: `--list` mode dispatch in `run()`

## Goal

**Feature Goal**: Wire `--list`/`-l` into `main.go`'s `run()` so `weave --list` resolves the extensions dir (`extdir.Find`), builds the catalog (`discover.Index`), and renders it via `ui.PrintList` — the first end-to-end exercise of the Find → Index → PrintList data flow. It is a near-verbatim port of skilldozer's `if c.list { ... }` branch in `skilldozer/main.go` (read in full during research). The ONLY non-cosmetic deltas are (a) the import path (`skillsdir` → `extdir`; `[]discover.Skill` stays `[]discover.Extension` — already correct because weave's discover package was built around `Extension` from day one), and (b) the empty-store stderr message `"... skills found in "` → `"... extensions found in "` (PRD §6.1 says "1 if no extensions found"; weave is an extension catalog, not a skill catalog).

**Deliverable**: ONE file MODIFIED — `main.go` (add the `ui` import + the `--list` dispatch branch in `run()`, between the existing `--path` branch and the no-op `return 0`). ONE file MODIFIED — `main_test.go` (add the `withTerminal` helper, the `writeExtTree` helper, and six new `--list` run-level tests; PLUS modify the existing `TestRunUndispatchedModeIsNoOp` which currently asserts `--list` is a no-op — that assertion is now WRONG and must be updated/removed). NO new files. NO changes to `go.mod` (stdlib + 2 internal packages: `extdir`, `discover`, `ui` — all PRESENT).

**Success Definition**:
- `go build ./...` exits 0 (main.go now imports `internal/ui`; the import resolves because `internal/ui` lands in P1.M2.T4.S1, a parallel predecessor treated as PRESENT).
- `go vet ./...` exits 0 (clean).
- `go test ./... -v` passes ALL tests (existing + the six new `--list` tests + the modified no-op test).
- `go test -race ./...` passes.
- `weave --list` with a non-empty store exits 0, stdout = a `TAG`/`NAME`/`DESCRIPTION` table (header + one row per extension), stderr empty (NO `(found via ...)` label — that is `--path`-only; `--list` does not report its source).
- `weave --list` with an empty store (dir exists, zero extensions) exits 1, stdout EMPTY, stderr `no extensions found in <dir>` + newline.
- `weave --list` when unconfigured (`extdir.Find` → `ErrNotFound`) exits 1, stdout EMPTY, stderr `weave is not configured; run \`weave init\`` + newline (the `ErrNotFound.Error()` verbatim, no wrapping).
- Color is on IFF `isTerminal(stdout) && !c.noColor` (PRD §6.2). A non-TTY `*bytes.Buffer` (the test default) → no ANSI. `--no-color` suppresses ANSI even when `isTerminal` is forced true (TestRunListNoColorFlagSuppressesANSI). TTY + no `--no-color` → ANSI bold header + cyan tags (TestRunListColorWhenTTY).
- `--file`/`--relative` are IGNORED by `--list` (it prints a TABLE, not paths — PRD §6.2 header: modifiers combine with tag resolution or `--all`). The branch does not consult `c.file` or `c.relative`. (Exclusivity error for `--list --file` is a M5.T1.S1 concern, NOT this task.)

## User Persona (if applicable)

**Target User**: weave CLI users running `weave --list` / `weave -l` to browse the human-readable catalog (PRD §6.1). Also: downstream `main.go` milestones (M3/M4) that reuse the same Find → Index data flow for `<tag>`, `--all`, `--search`, `check`.

**Use Case**: A user has installed an extension (e.g. via the example shipped in P1.M6.T1) and runs `weave --list` to confirm it is discoverable, read its tag/name/description, and decide how to invoke `pi -e "$(weave <tag>)"`. This subtask is the assembly layer: the gears (`extdir.Find`, `discover.Index`, `ui.PrintList`) all exist from prior milestones; this task wires them together for the first time.

**User Journey**:
1. User runs `weave --list` in a shell where weave is configured (env var, config file, sibling, or walk-up rule resolves the extensions dir).
2. `run()` dispatches to the `--list` branch (after `--version`, after `--path`, before the no-op fallthrough).
3. `extdir.Find()` returns the dir (or `ErrNotFound`).
4. `discover.Index(dir)` returns the sorted `[]Extension` (or an error, e.g. the dir vanished between Find and Index).
5. `ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)` renders the table.
6. `return 0`.

**Pain Points Addressed**:
- **Discoverability**: before this task, `weave --list` was a silent no-op (exit 0, no output). Users had no way to see what was installed. This task makes the catalog visible.
- **`$(...)` safety (PRD §6.4)**: failure paths (`ErrNotFound`, empty store) print NOTHING to stdout and exit 1 — so `pi -e "$(weave --list)"`-style misuse fails loudly rather than passing a garbage table. Empty stdout on failure is the load-bearing contract, pinned by three tests.
- **Pipe cleanliness**: color is opt-in via TTY detection; `weave --list | grep foo` and `weave --list > file` see clean, ANSI-free text. Pinned by TestRunListSuccess (default non-TTY buffer → no `\x1b`).

## Why

- **PRD §6.1 contract surface**: `--list` is row 3 of the authoritative command table — "Table: `TAG`, `NAME`, `DESCRIPTION` (wrapped)", exit `0` (`1` if no extensions found). This subtask IS that row, assembled. Without it, the catalog is invisible and the milestone M2 (discovery + rendering) cannot close.
- **First end-to-end data flow**: this is the first place `run()` calls BOTH `extdir.Find` AND `discover.Index` AND `ui.PrintList` in sequence. It proves the three modules compose (the Find output feeds Index's input; Index's output feeds PrintList's input). M3 (`<tag>`, `--all`) and M4 (`--search`, `check`) will reuse the same Find → Index prefix; getting it right here is the template.
- **Near-verbatim port minimizes risk**: skilldozer's `if c.list { ... }` branch is a PROVEN, tested implementation (six run-level tests cover success, short-flag, empty-store-exit-1, unresolvable-exit-1, `--no-color`-suppresses-ANSI, TTY-color-on). Porting it verbatim (with the two documented deltas) inherits all that coverage. The branch is ~15 lines; writing it from scratch would re-litigate the error-message wording, the exit codes, and the TTY-gate wiring that skilldozer already settled.
- **The `"... extensions found in <dir>"` message is the load-bearing weave change**: skilldozer prints `"no skills found in " + dir`; weave prints `"no extensions found in " + dir`. The `+dir` suffix is intentional (it tells the user WHICH dir was found-but-empty — a debugging aid when the env var / config points somewhere unexpected). PRD §6.1's "1 if no extensions found" is the spec; the message wording is a skilldozer-blessed convention ported with the noun swap.
- **`--list` does NOT print the `(found via ...)` source label**: that label is `--path`-only (Issue 1, P1.M1.T4.S1). `--list`'s stdout is the TABLE (source label would corrupt it); its stderr is EMPTY on success (or the one-line error on failure). Do NOT reuse the `--path` branch's `fmt.Fprintf(stderr, "(found via %s)\n", src)` — `src` is discarded via `_, _, err := extdir.Find()`.

## What

A MODIFIED `main.go`:

### 1. Add the `ui` import

The existing import block gains one line. The import group is currently `fmt`, `io`, `os`, `strings`, then the internal package `github.com/dabstractor/weave/internal/extdir`. Add `github.com/dabstractor/weave/internal/ui` to the internal group (alphabetical: `extdir` < `ui`).

```go
import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dabstractor/weave/internal/discover"  // NEW
	"github.com/dabstractor/weave/internal/extdir"
	"github.com/dabstractor/weave/internal/ui"        // NEW
)
```
NOTE: `discover` must ALSO be imported (it was NOT imported before — `run()` did not call it). Both `discover` and `ui` are new imports; `extdir` was already imported. The `strings` import is already present (used by parseArgs); it stays.

### 2. Add the `--list` branch in `run()`

Insert it AFTER the existing `--path` branch (the `if c.path { ... }` block ending `return 0`) and BEFORE the final `// 3) All other parsed modes are no-ops ... return 0`. Renumber the trailing comment (the `// 3)` becomes `// 3) --list`; add a new `// 4) All other parsed modes are no-ops` for the fallthrough). Port skilldozer's branch verbatim with the two deltas:

```go
	// 3) --list / -l (PRD §6.1). The FIRST end-to-end wiring of the
	//    Find() -> discover.Index() -> ui.PrintList() data flow. Exit 1 on any
	//    failure path. This branch does NOT consult c.file / c.relative (--list
	//    prints a TABLE, not paths — PRD §6.2 header: modifiers combine with tag
	//    resolution or --all). Exclusivity (--list + --file, etc.) is M5.T1.S1.
	if c.list {
		// PRD §6.1 `weave --list`: resolve the store, build the index, render the
		// table. Find locates the dir via the §8.3 rules; Index walks it and
		// returns []Extension sorted by RelTag; PrintList renders TAG/NAME/DESCRIPTION.
		dir, _, err := extdir.Find()
		if err != nil {
			fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline (PRD §6.4/§8.2)
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		if len(exts) == 0 {
			// PRD §6.1: --list exits 1 "if no extensions found". Message to stderr
			// so stdout stays clean for any consumer. The +dir suffix is a
			// debugging aid (which dir was found-but-empty).
			fmt.Fprintln(stderr, "no extensions found in "+dir)
			return 1
		}
		// Color only when stdout is a TTY AND --no-color was not given (PRD §6.2).
		// A *bytes.Buffer (tests) / pipe / file is not a TTY -> plain output.
		// Note: --list prints a TABLE, so the --file/--relative path modifiers do
		// NOT apply to it (PRD §6.2 header).
		ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)
		return 0
	}
```

The two deltas from skilldozer, called out:
- `skills, err := discover.Index(dir)` → `exts, err := discover.Index(dir)` (variable rename; the TYPE is already `[]Extension` in weave's discover — no type change, just the noun).
- `fmt.Fprintln(stderr, "no skills found in "+dir)` → `fmt.Fprintln(stderr, "no extensions found in "+dir)` (noun swap; `+dir` suffix preserved).

Everything else — the `_, _, err := extdir.Find()` (discard the `src` label — NOT printed, unlike `--path`), the `fmt.Fprintln(stderr, err)` verbatim-error convention (NO prefix, NO wrapping), the empty-store `len(exts) == 0` gate, the `isTerminal(stdout) && !c.noColor` color gate, the `return 0` — ports byte-for-byte.

### 3. Update the trailing fallthrough comment

The existing `// 3) All other parsed modes are no-ops for now. M2 adds --list, ...` comment is now stale (M2 has arrived). Renumber to `// 4)` and drop the "M2 adds --list" clause (it is done). Keep the M3/M4/M5 forward-refs.

### A MODIFIED `main_test.go`:

Add two helpers and six tests; MODIFY the existing `TestRunUndispatchedModeIsNoOp`.

**Critical: `TestRunUndispatchedModeIsNoOp` currently asserts `run([]string{"--list"})` returns exit 0 with empty output.** That assertion is now FALSE (`--list` has real behavior). This test must be modified: replace the `--list` case with a different still-undispatched mode (e.g. `--all`, which lands in M3.T2.S1), OR split it into a `TestRunUndispatchedAllIsNoOp` covering `--all`. Do NOT just delete it — the "parser complete, dispatch trimmed" contract it pins is still valuable for the remaining undispatched modes (`--all`, `<tag>`, `check`, `init`, `--search`). See Task 8.

### Success Criteria

- [ ] `main.go` imports `internal/discover` AND `internal/ui` (both NEW; `extdir` was already imported).
- [ ] The `--list` branch sits AFTER `--version` and AFTER `--path`, BEFORE the no-op fallthrough (precedence: `--version` > `--path` > `--list`; `--help` lands in M5 and will sit ABOVE `--version`).
- [ ] `dir, _, err := extdir.Find()` — the `src` (second return) is DISCARDED. `--list` does NOT print `(found via ...)`.
- [ ] Empty-store message is EXACTLY `"no extensions found in " + dir` (with the `+dir` suffix; noun = "extensions", NOT "skills").
- [ ] Color gate is `isTerminal(stdout) && !c.noColor` (short-circuit AND: non-TTY never colors, `--no-color` always suppresses).
- [ ] The branch does NOT reference `c.file` or `c.relative` (modifiers do not apply to `--list`).
- [ ] `TestRunUndispatchedModeIsNoOp` updated to use `--all` (or removed and replaced) — it no longer asserts `--list` is a no-op.
- [ ] Six new `--list` tests pass: success (non-TTY, no ANSI), short-flag `-l`, empty-store exit 1, unresolvable exit 1, `--no-color` suppresses ANSI (TTY forced), TTY color on.
- [ ] `go build ./...`, `go vet ./...`, `go test ./... -v`, `go test -race ./...` all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_ **Yes.** This PRP names: the authoritative port source (`/home/dustin/projects/skilldozer/main.go` `if c.list` branch — read in full, quoted verbatim above with the two deltas marked); the authoritative spec (PRD §6.1 row 3, §6.2 `--no-color`, §6.4 error semantics — all quoted); the exact three function signatures the branch calls (`extdir.Find() (dir, src, err)`, `discover.Index(dir) ([]Extension, error)`, `ui.PrintList(w, []Extension, bool)` — all read in full from the PRESENT source); the exact test patterns to port (skilldozer's six `TestRunList*` tests + `withTerminal` + `writeSkillTree`, all read in full, with the `.md`→`.ts` fixture adaptation spelled out); the existing test that MUST be modified (`TestRunUndispatchedModeIsNoOp`, quoted, with the exact change); and the insertion point in `run()` (after `--path`, before the no-op fallthrough, quoted). The change is ~15 lines of Go + ~6 tests + 2 helpers. No external library docs needed (stdlib only).

### Documentation & References

```yaml
- file: /home/dustin/projects/skilldozer/main.go  (READ-ONLY — the port source)
  why: THIS IS THE REFERENCE. Read the `if c.list { ... }` branch (lines ~478-505)
        in full and port it. The COMPLETE branch is quoted in the "What" section of
        this PRP. Port VERBATIM; the two deltas (noun swap in the empty-store
        message + variable rename skills->exts) are marked.
  pattern: |
    Port the branch byte-for-byte with these edits:
    (1) Variable rename: `skills, err := discover.Index(dir)` -> `exts, err := ...`.
        The TYPE is already []Extension in weave (no Skill type exists); only the
        local var name changes. Update the len() check and the PrintList call to
        use `exts`.
    (2) Empty-store message: `fmt.Fprintln(stderr, "no skills found in "+dir)`
        -> `fmt.Fprintln(stderr, "no extensions found in "+dir)`.
        Keep the `+dir` suffix (debugging aid). Change noun skills->extensions.
    Everything else ports unchanged:
      - `dir, _, err := extdir.Find()` (the SECOND return `src` is DISCARDED with
        `_` — --list does NOT print the source label, unlike --path).
      - `fmt.Fprintln(stderr, err)` on both error paths (verbatim err.Error() +
        newline, NO prefix, NO wrapping — PRD §6.4).
      - `return 1` on all three failure paths (Find err, Index err, empty store).
      - `ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)`.
      - `return 0` on success.
  critical: |
    The NON-OBVIOUS decision: `--list` discards the `src` return from extdir.Find.
    The `--path` branch (P1.M1.T4.S1) prints `(found via %s)` to stderr; `--list`
    does NOT. --list's stdout is the TABLE (a source label there would corrupt
    it), and its stderr is EMPTY on success. Use `dir, _, err := extdir.Find()`
    (underscore on src), NOT `dir, src, err := ...`. An unused `src` would be a
    Go COMPILE ERROR (unused local), so the discard is REQUIRED, not stylistic.

- file: /home/dustin/projects/skilldozer/main_test.go  (READ-ONLY — the test source)
  why: Port six TestRunList* tests + the withTerminal + writeSkillTree helpers.
        Lines ~315-435 contain: TestRunListSuccess, TestRunListShortFlag,
        TestRunListNoSkillsExit1, TestRunListSkillsDirUnresolvableExit1,
        TestRunListNoColorFlagSuppressesANSI, TestRunListColorWhenTTY. The helpers
        withTerminal (lines ~18-24) and writeSkillTree (lines ~41-54) are the
        test infrastructure to port + adapt.
  pattern: |
    PORT withTerminal VERBATIM (it is pure plumbing — swaps the package-level
        isTerminal func var for one test, restores on cleanup; NOT t.Parallel-safe).
    PORT writeSkillTree but ADAPT the fixture: weave extensions are .ts/.js FILES
        (single-file extensions per PRD §7.1), NOT .md files. So writeExtTree
        writes `<tag>.ts` (NOT `<tag>/SKILL.md`) with a leading JSDoc block for the
        description. See "Known Gotchas" for the exact fixture content. RENAME the
        helper writeExtTree (noun swap) and update all call sites.
    PORT the six tests with noun swaps (skills->extensions, SKILLDOZER_SKILLS_DIR
        -> weave_EXTENSIONS_DIR, "no skills found" -> "no extensions found",
        "skilldozer init" -> "weave init"). The assertion logic is unchanged.
  critical: |
    writeSkillTree's skilldozer form writes a DIRECTORY per tag with a SKILL.md
        inside (skilldozer extensions are markdown-with-YAML-frontmatter). weave's
        single-file extensions are a .ts FILE directly under the store root. The
        fixture MUST be `<store>/example.ts`, NOT `<store>/example/SKILL.md`. If
        you port writeSkillTree verbatim you will create a dir that
        discover.classifyDir does NOT recognize as an extension (no index.ts,
        no package.json) -> Index returns nil -> --list exits 1 "no extensions
        found" -> TestRunListSuccess fails. This is THE test-port gotcha.

- file: /home/dustin/projects/weave/main.go  (MODIFY — the deliverable)
  why: The file being edited. run() currently dispatches --version (branch 1) and
        --path (branch 2), then has a `// 3) All other parsed modes are no-ops`
        fallthrough returning 0. The --list branch slots in as the NEW branch 3,
        and the fallthrough becomes branch 4.
  pattern: |
    INSERT the --list branch between the closing `}` of `if c.path { ... }`
        (around line 230) and the `// 3) All other parsed modes are no-ops`
        comment. The exact insertion point is quoted in "What" section step 2.
    ADD two imports to the internal group: discover and ui (alphabetical order).
    UPDATE the fallthrough comment: `// 3)` -> `// 4)`, drop "M2 adds --list"
        (done), keep the M3/M4/M5 forward-refs.
  critical: |
    Do NOT touch parseArgs (the parser already sets c.list on --list/-l from
        P1.M1.T4.S1). Do NOT touch isTerminal (declared + ready from
        P1.M1.T4.S1; --list is its first CALLER). Do NOT touch the --version or
        --path branches. Do NOT add exclusivity logic (--list + --file etc. is
        M5.T1.S1). The change is: 2 imports + 1 branch + 1 comment renumber.

- file: /home/dustin/projects/weave/internal/extdir/extdir.go  (READ — PRESENT, P1.M1.T3)
  why: Defines extdir.Find() (dir, src, err) — the FIRST call in the --list data
        flow. CONFIRMED signature: `func Find() (dir string, src Source, err error)`.
        On a miss returns ("", 0, ErrNotFound); ErrNotFound.Error() is the verbatim
        one-line fix "weave is not configured; run `weave init`" (backticks literal).
  critical: |
    Find returns THREE values. --list DISCARDS src (`dir, _, err :=`). An unused
    src is a Go compile error — the discard is mandatory. ErrNotFound.Error() is
    printed VERBATIM via fmt.Fprintln(stderr, err) (no prefix, no wrapping) — this
    is the PRD §6.4 contract that makes `pi -e "$(weave x)"` fail loudly. Do NOT
    add "error: " or "weave: " prefix.

- file: /home/dustin/projects/weave/internal/discover/index.go  (READ — PRESENT, P1.M2.T3.S1)
  why: Defines discover.Index(extensionsDir) ([]Extension, error) — the SECOND
        call. CONFIRMED signature: `func Index(extensionsDir string) ([]Extension, error)`.
        Returns []Extension SORTED by RelTag (PrintList relies on the sort; it does
        NOT re-sort). An empty store yields (nil, nil) — callers test len()==0.
        A missing/unreadable/non-dir root yields (nil, err).
  critical: |
    Index makes extensionsDir absolute FIRST (filepath.Abs), so Extension.Path /
    EntryFile are absolute. --list does not care (it renders only RelTag/Name/
    Description), but the absoluteness is why the empty-store message's `+dir` is
    the absolute path. An empty []Extension is a NIL slice (len==0), not
    []Extension{} — `len(exts) == 0` works for both, so no special handling.

- file: /home/dustin/projects/weave/internal/ui/ui.go  (READ — parallel predecessor P1.M2.T4.S1, treat as PRESENT)
  why: Defines ui.PrintList(w io.Writer, exts []discover.Extension, useColor bool)
        — the THIRD call. CONFIRMED signature: takes an io.Writer, a []Extension,
        and a bool. Renders TAG/NAME/DESCRIPTION table; empty slice prints nothing
        (early return — --list owns the exit-1 decision via the len()==0 gate
        BEFORE calling PrintList). Color is opt-in via useColor.
  critical: |
    PrintList does NOT exit or return an error — it just writes. The exit-code
    decisions (0 on success, 1 on empty/unresolvable) are run()'s job, made
    BEFORE the PrintList call. The len(exts)==0 gate returns 1 WITHOUT calling
    PrintList (so the "prints nothing on failure" contract holds: empty store
    -> no header, no table, just the stderr message).

- file: /home/dustin/projects/weave/internal/discover/extension.go  (READ — PRESENT, P1.M2.T1.S1)
  why: Defines the Extension struct that PrintList renders. CONFIRMED fields:
        Path, EntryFile, RelTag, Kind, Name, Description string; Keywords,
        Aliases []string; Category string; HasPackageJSON bool.
        PrintList reads ONLY RelTag, Name, Description.
  critical: |
    Extension has NO HasFM field (unlike skilldozer's Skill). The (none) sentinel
    for empty Name/Description is handled INSIDE PrintList (P1.M2.T4.S1); run()
    does not preprocess exts. Pass the raw []Extension from Index straight to
    PrintList.

- file: /home/dustin/projects/weave/internal/discover/jsdoc.go  (READ — PRESENT, P1.M2.T1.S2)
  why: Defines ExtractJSDoc(path) — the description source for single-file .ts
        extensions. It reads the LEADING `/** ... */` block. This matters for the
        TEST FIXTURE: a single-file extension's Description comes from its JSDoc
        block, so writeExtTree must emit a `/** ... */` comment for the description
        to be non-empty (and thus NOT render as (none)).
  critical: |
    The JSDoc opener is `/**` (slash-star-star, EXACTLY two stars) — a leading
    `//` or `/*` (one star) comment is NOT detected. The fixture MUST use
    `/** description here */` or a multi-line `/**\n * description\n */` block.
    A plain `// description` yields Description="" -> renders as (none) in the
    table -> TestRunListSuccess still passes (it checks the TAG is present, not
    the description), BUT it is misleading. Use a JSDoc block for realism.

- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec. §6.1 row 3 (--list: "Table: TAG, NAME, DESCRIPTION
        (wrapped)", exit "0 (1 if no extensions found)"). §6.2 --no-color modifier
        ("Disable ANSI color even on a TTY"). §6.4 error semantics (nothing to
        stdout on failure; ErrNotFound one-line fix verbatim; "critical for $(...) use").
  critical: |
    PRD §6.1 row 3 VERBATUM exit column: "0 (1 if no extensions found)". The
    "(1 if no extensions found)" is the empty-store gate. §6.4: "print NOTHING to
    stdout" on failure — this is why all three failure paths (Find err, Index err,
    empty store) print to STDERR and return 1 BEFORE any stdout write. §6.2
    --no-color: "even on a TTY" — so the gate is `isTerminal(stdout) && !c.noColor`
    (AND, not OR); --no-color WINS over TTY.

- file: /home/dustin/projects/weave/main_test.go  (MODIFY — the test file)
  why: Existing tests. TestRunUndispatchedModeIsNoOp (quoted in "Known Gotchas")
        currently asserts run(--list) is a no-op (exit 0, empty output). That
        assertion is NOW FALSE. Modify it: change the --list case to --all (still
        undispatched in M2), OR delete the --list subtest. Also: the existing
        TestRunPathFailureErrNotFound and unsetExtEnv helper are the PATTERNS to
        reuse for the new --list failure tests.
  critical: |
    If you do NOT modify TestRunUndispatchedModeIsNoOp, the test suite FAILS
    (run(--list) now returns 1 on the empty/unconfigured env, or 0 with table
    output on a configured env — either way it is NOT the no-op the test asserts).
    This is the single most easily-missed step. See Task 8.

- file: /home/dustin/projects/weave/go.mod
  why: Module `github.com/dabstractor/weave`, `go 1.25`. main.go's new imports are
        all internal (extdir, discover, ui) — no new require, no go.sum.
  critical: Adding any third-party dep would violate PRD §2 (stdlib-only).
```

### Current Codebase tree

```bash
# After P1.M1 (Complete) + M2.T1/T2/T3 (Complete or parallel-PRESENT) + M2.T4.S1 (parallel, PRESENT).
# THIS subtask MODIFIES main.go + main_test.go (no new files).
$ cd /home/dustin/projects/weave && find . -name '*.go' -not -path './.git/*' -not -path './.pi-subagents/*' | sort
./internal/config/config.go            # P1.M1.T2.S1 (Complete)
./internal/config/config_test.go
./internal/extdir/extdir.go            # P1.M1.T3.S1+S2+S3 (Complete — Find, ErrNotFound, Source)
./internal/extdir/extdir_test.go
./internal/discover/extension.go       # P1.M2.T1.S1 (Complete — Extension struct)
./internal/discover/extension_test.go
./internal/discover/jsdoc.go           # P1.M2.T1.S2 (Complete — ExtractJSDoc)
./internal/discover/jsdoc_test.go
./internal/discover/discover.go        # P1.M2.T2.S1 (parallel — classifyFile/classifyDir)
./internal/discover/discover_test.go
./internal/discover/index.go           # P1.M2.T3.S1 (parallel — Index)
./internal/discover/index_test.go
./internal/ui/ui.go                    # P1.M2.T4.S1 (parallel — PrintList)  [treat as PRESENT]
./internal/ui/ui_test.go               # P1.M2.T4.S1 (parallel)              [treat as PRESENT]
./main.go                              # P1.M1.T4.S1 (Complete — MODIFY: add ui+discover imports, --list branch)
./main_test.go                         # P1.M1.T4.S1 (Complete — MODIFY: 2 helpers, 6 tests, fix no-op test)
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                    # UNCHANGED
│   ├── extdir/                    # UNCHANGED (Find, ErrNotFound consumed by main.go)
│   ├── discover/                  # UNCHANGED (Index consumed by main.go)
│   └── ui/                        # UNCHANGED (PrintList consumed by main.go) [from P1.M2.T4.S1]
├── main.go                        # MODIFIED — +2 imports (discover, ui), +--list branch, renumbered comment
├── main_test.go                   # MODIFIED — +withTerminal, +writeExtTree, +6 TestRunList*, fixed TestRunUndispatchedModeIsNoOp
├── go.mod                         # UNCHANGED (no new require)
└── ...                            # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (TestRunUndispatchedModeIsNoOp MUST be modified): main_test.go currently
// contains this test, which is now WRONG:
//   func TestRunUndispatchedModeIsNoOp(t *testing.T) {
//       var out, errOut bytes.Buffer
//       code := run([]string{"--list"}, &out, &errOut)   // <-- --list is NO LONGER a no-op!
//       if code != 0 { ... }
//       if out.Len() != 0 { ... }
//   }
// This test was written in P1.M1.T4.S1 when --list was parsed but undispatched.
// NOW --list has real behavior. Fix: change the arg from "--list" to "--all"
// (still undispatched until M3.T2.S1) and rename to TestRunUndispatchedAllIsNoOp,
// OR keep the name and switch the arg. Do NOT delete the test outright — it pins
// the "parser complete, dispatch trimmed" contract for the remaining undispatched
// modes. If you forget this, `go test ./...` FAILS on this pre-existing test.

// CRITICAL (writeExtTree fixture is .ts, NOT .md): skilldozer's writeSkillTree
// writes <tag>/SKILL.md (markdown with YAML frontmatter). weave's single-file
// extensions are .ts/.js FILES directly under the store root (PRD §7.1). The
// ported helper MUST write <store>/<tag>.ts with a leading JSDoc block:
//   func writeExtTree(t *testing.T, layout map[string]string) string {
//       t.Helper()
//       root := t.TempDir()
//       for tag, jsdocBody := range layout {
//           // jsdocBody is the description text; wrap in a JSDoc block so
//           // ExtractJSDoc picks it up (a plain // comment yields Description="").
//           content := "/** " + jsdocBody + " */\nexport default function() {}\n"
//           path := filepath.Join(root, filepath.FromSlash(tag)+".ts")
//           if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
//               t.Fatalf("write %s: %v", path, err)
//           }
//       }
//       return root
//   }
// NOTE the .ts extension on the FILENAME (tag+".ts"), and the JSDoc opener "/**"
// (two stars). A directory <tag>/ with no index.ts/package.json is NOT recognized
// by classifyDir -> Index returns nil -> TestRunListSuccess fails with exit 1.

// CRITICAL (JSDoc opener is exactly "/**", two stars): ExtractJSDoc (P1.M2.T1.S2)
// detects a LEADING "/**" block. A leading "/*" (one star) or "//" is NOT a JSDoc
// block and yields Description="". The fixture MUST use "/** ... */". For a
// multi-line description use "/**\n * line one\n * line two\n */". stripStarPrefix
// (jsdoc.go) cleans the " * " continuation prefixes.

// CRITICAL (src from extdir.Find is DISCARDED, not printed): the --path branch
// (P1.M1.T4.S1) does `dir, src, err := extdir.Find()` and prints
// `fmt.Fprintf(stderr, "(found via %s)\n", src)`. --list does NOT. Use
// `dir, _, err := extdir.Find()` (underscore on src). --list's stdout is the
// TABLE; a source label would corrupt it. --list's stderr is EMPTY on success
// (or the verbatim error on failure). An unused `src` local is a Go COMPILE
// ERROR, so the `_` discard is mandatory, not stylistic.

// CRITICAL (empty-store message noun is "extensions", with +dir suffix): the
// message is `fmt.Fprintln(stderr, "no extensions found in "+dir)`. Three things
// to get right: (1) noun "extensions" NOT "skills" (skilldozer port gotcha);
// (2) the `+dir` suffix is INTENTIONAL (debugging aid — which dir was empty);
// (3) Fprintln adds the trailing newline (do NOT add "\n" yourself). The exact
// bytes on stderr: "no extensions found in /abs/path/to/store\n".

// CRITICAL (ErrNotFound printed VERBATIM, no prefix): on the Find-error path,
// `fmt.Fprintln(stderr, err)` prints err.Error() + "\n". err.Error() is exactly
// "weave is not configured; run `weave init`" (backticks LITERAL — they are part
// of the message for shell copy-paste). Do NOT add "error: " / "weave: " prefix,
// do NOT wrap in fmt.Errorf. PRD §6.4: the one-line fix is printed VERBATIM so
// `pi -e "$(weave x)"` fails loudly with a copy-pasteable fix.

// CRITICAL (stdout EMPTY on all failure paths): the three failure returns (Find
// err, Index err, empty store) happen BEFORE any stdout write. --list never
// writes a partial table then errors. The len(exts)==0 gate returns 1 WITHOUT
// calling ui.PrintList (PrintList's own empty-slice early-return is a defense-in-
// depth backstop, but run() never reaches it on the empty path). Pinned by
// TestRunListNoExtensionsExit1 (assert out.Len()==0) and
// TestRunListUnresolvableExit1 (assert out.Len()==0).

// GOTCHA (color gate is AND, not OR): `isTerminal(stdout) && !c.noColor`. Both
// must hold for color. A non-TTY buffer (the test default) -> false -> no color
// (short-circuit, !c.noColor not even evaluated). A TTY with --no-color ->
// false -> no color. A TTY without --no-color -> true -> color. The
// withTerminal(t, true) helper forces isTerminal to return true so the !c.noColor
// leg can be exercised in a test. Pin both legs: TestRunListColorWhenTTY (TTY,
// no --no-color -> ANSI present) and TestRunListNoColorFlagSuppressesANSI (TTY,
// --no-color -> ANSI absent).

// GOTCHA (withTerminal mutates package state — no t.Parallel): the withTerminal
// helper swaps the package-level `isTerminal` func var and restores it on
// t.Cleanup. t.Parallel would race two tests swapping the same var. The repo
// convention (established in main.go's isTerminal doc comment and config/extdir
// tests) is NO t.Parallel on tests that touch package state or t.Setenv. Do NOT
// add t.Parallel to any of the six new tests.

// GOTCHA (imports: add discover AND ui): main.go currently imports extdir only
// (internal group). The --list branch calls discover.Index AND ui.PrintList, so
// BOTH must be added. Order in the internal group (alphabetical): discover,
// extdir, ui. (goimports would sort them; if not using goimports, order manually
// — gofmt does NOT reorder imports, but a vet/build will still succeed with any
// order; alphabetical is the repo convention from main.go's existing block.)

// GOTCHA (do NOT add exclusivity logic): --list combined with --file/--relative/
// --all/<tag> is a PRD §6.3 mutual-exclusivity violation, but the exclusivity
// check + exit-2 behavior lands in M5.T1.S1. THIS task's --list branch ignores
// c.file/c.relative entirely (it does not read them). If a user runs
// `weave --list --file`, this milestone renders the table and exits 0 (the
// --file is silently inert). M5.T1.S1 will add the exit-2 rejection. Do NOT
// pre-empt it.

// GOTCHA (precedence: --version > --path > --list): the branch order in run()
// IS the precedence. --version (branch 1) wins over --path (branch 2) wins over
// --list (branch 3). So `weave --list --version` prints the version and exits 0
// (--list never runs). `weave --list --path` prints the path and exits 0. This
// is already correct by construction (the new branch goes AFTER --path). --help
// (M5.T1.S1) will go ABOVE --version. Do NOT reorder existing branches.
```

## Implementation Blueprint

### Data models and structure

NO new data models. This subtask CONSUMES `extdir.Find`, `discover.Index`, `ui.PrintList` (all PRESENT) and adds a dispatch branch. The only "structure" is the `run()` control flow.

```go
// The COMPLETE --list branch to insert into run() (after --path, before no-op):
if c.list {
	dir, _, err := extdir.Find()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	exts, err := discover.Index(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(exts) == 0 {
		fmt.Fprintln(stderr, "no extensions found in "+dir)
		return 1
	}
	ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)
	return 0
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD imports to main.go
  - OPEN /home/dustin/projects/weave/main.go.
  - FIND the import block (the `import (` ... `)` near the top, after the version var).
  - EDIT the internal group to add discover (before extdir) and ui (after extdir):
        "github.com/dabstractor/weave/internal/discover"
        "github.com/dabstractor/weave/internal/extdir"
        "github.com/dabstractor/weave/internal/ui"
    (extdir is already present; add discover above it and ui below it, alphabetical.)
  - VALIDATE: cd /home/dustin/projects/weave && go build ./... (expect exit 0 — the
    imports are used by the branch in Task 2; if you run build BEFORE Task 2, Go
    errors on unused imports "imported and not used". So do Task 1 + Task 2 together,
    OR expect the unused-import error mid-way and proceed to Task 2.)

Task 2: INSERT the --list branch into run()
  - IN main.go run(), LOCATE the end of the `if c.path { ... }` block (the line
    `return 0` followed by `}` closing the --path branch, with the trailing comment
    `// 3) All other parsed modes are no-ops for now.`).
  - INSERT the --list branch (quoted in "Data models" above + "What" section step 2)
    BETWEEN the --path branch's closing `}` and the `// 3)` comment.
  - RENUMBER the fallthrough comment: `// 3)` -> `// 4)`. DROP the "M2 adds --list"
    clause (it is done). Keep the M3/M4/M5 forward-refs:
        // 4) All other parsed modes are no-ops for now. M3 adds <tag>/--file/--all,
        //    M4 adds --search/check/init, M5 adds --help/exclusivity/unknown-flag-2
        //    and the no-args->usage path. The parser is ALREADY complete; later
        //    milestones add dispatch branches HERE only.
  - VALIDATE: go build ./... (expect exit 0). go vet ./... (expect clean).

Task 3: ADD the withTerminal helper to main_test.go
  - PORT from /home/dustin/projects/skilldozer/main_test.go lines ~18-24 VERBATIM.
        func withTerminal(t *testing.T, isTTY bool) {
            t.Helper()
            prev := isTerminal
            isTerminal = func(io.Writer) bool { return isTTY }
            t.Cleanup(func() { isTerminal = prev })
        }
  - NOTE: `isTerminal` is the package-level func VAR in main.go (declared
    P1.M1.T4.S1). withTerminal swaps it for one test. NOT t.Parallel-safe.
  - PLACEMENT: near the top of main_test.go, after the existing unsetExtEnv helper
    (around line 30), before the `// --- parseArgs matrix ---` section.

Task 4: ADD the writeExtTree helper to main_test.go
  - PORT skilldozer's writeSkillTree but ADAPT for weave's .ts single-file extensions:
        func writeExtTree(t *testing.T, layout map[string]string) string {
            t.Helper()
            root := t.TempDir()
            for tag, desc := range layout {
                // Wrap the description in a JSDoc block so ExtractJSDoc picks it
                // up. A plain // comment yields Description="" -> renders (none).
                content := "/** " + desc + " */\nexport default function() {}\n"
                path := filepath.Join(root, filepath.FromSlash(tag)+".ts")
                if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
                    t.Fatalf("write %s: %v", path, err)
                }
            }
            return root
        }
  - CRITICAL: the filename is `tag+".ts"` (a FILE directly under root), NOT
    `tag/SKILL.md` (a dir). The JSDoc opener is `/**` (two stars).
  - NOTE: existing main_test.go imports are bytes, path/filepath, strings, testing.
    writeExtTree needs os.WriteFile -> ADD "os" to the import block. (Check: does
    main_test.go already import os? It does NOT currently — add it.)
  - PLACEMENT: right after withTerminal.

Task 5: ADD TestRunListSuccess + TestRunListShortFlag (port + noun swap)
  - PORT skilldozer's TestRunListSuccess (lines ~315-340) with swaps:
        func TestRunListSuccess(t *testing.T) {
            dir := writeExtTree(t, map[string]string{
                "example": "A demo extension.",
            })
            t.Setenv("weave_EXTENSIONS_DIR", dir) // rule 1 wins; Find returns dir
            var out, errOut bytes.Buffer
            code := run([]string{"--list"}, &out, &errOut)
            if code != 0 {
                t.Fatalf("run(--list): code=%d; want 0", code)
            }
            got := out.String()
            for _, want := range []string{"TAG", "NAME", "DESCRIPTION", "example", "A demo extension."} {
                if !strings.Contains(got, want) {
                    t.Errorf("run(--list) stdout missing %q:\n%s", want, got)
                }
            }
            if strings.Contains(got, "\x1b[") { // non-TTY buffer -> no ANSI
                t.Errorf("run(--list) on a non-TTY must not emit ANSI:\n%s", got)
            }
            if errOut.Len() != 0 {
                t.Errorf("run(--list) stderr=%q; want empty (no source label for --list)", errOut.String())
            }
        }
  - PORT TestRunListShortFlag (lines ~340-355) — same shape, arg "-l", assert
    "example" present, code 0.
  - NOTE: the "A demo extension." assertion proves the JSDoc description flowed
    through ExtractJSDoc -> BuildExtension -> Index -> PrintList. If it is missing,
    the writeExtTree fixture is wrong (check the /** opener).

Task 6: ADD TestRunListNoExtensionsExit1 + TestRunListUnresolvableExit1 (port + noun swap)
  - PORT TestRunListNoSkillsExit1 (lines ~358-372) with swaps:
        func TestRunListNoExtensionsExit1(t *testing.T) {
            t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()) // exists, no .ts -> empty
            var out, errOut bytes.Buffer
            code := run([]string{"--list"}, &out, &errOut)
            if code != 1 {
                t.Fatalf("run(--list) empty store: code=%d; want 1 (PRD §6.1 '1 if no extensions found')", code)
            }
            if out.Len() != 0 {
                t.Errorf("run(--list) empty store stdout=%q; want empty", out.String())
            }
            if !strings.Contains(errOut.String(), "no extensions found") {
                t.Errorf("run(--list) empty store stderr=%q; want a 'no extensions found' message", errOut.String())
            }
        }
  - PORT TestRunListSkillsDirUnresolvableExit1 (lines ~375-390) with swaps ->
    TestRunListUnresolvableExit1: use unsetExtEnv(t) + t.Chdir(t.TempDir()),
    assert code==1, out.Len()==0, stderr contains "weave init" (NOT "skilldozer init").
  - NOTE: "no extensions found" (noun swap from "no skills found"). "weave init"
    (noun swap from "skilldozer init") — ErrNotFound.Error() contains the literal
    backtick command `weave init`.

Task 7: ADD TestRunListNoColorFlagSuppressesANSI + TestRunListColorWhenTTY (port)
  - PORT TestRunListNoColorFlagSuppressesANSI (lines ~394-410):
        func TestRunListNoColorFlagSuppressesANSI(t *testing.T) {
            dir := writeExtTree(t, map[string]string{"example": "d"})
            t.Setenv("weave_EXTENSIONS_DIR", dir)
            withTerminal(t, true) // pretend stdout is a TTY
            var out, errOut bytes.Buffer
            code := run([]string{"--list", "--no-color"}, &out, &errOut)
            if code != 0 { t.Fatalf("run(--list --no-color): code=%d; want 0", code) }
            if strings.Contains(out.String(), "\x1b[") {
                t.Errorf("--no-color must suppress ANSI even on a TTY:\n%s", out.String())
            }
        }
  - PORT TestRunListColorWhenTTY (lines ~412-430):
        func TestRunListColorWhenTTY(t *testing.T) {
            dir := writeExtTree(t, map[string]string{"example": "d"})
            t.Setenv("weave_EXTENSIONS_DIR", dir)
            withTerminal(t, true)
            var out, errOut bytes.Buffer
            code := run([]string{"--list"}, &out, &errOut)
            if code != 0 { t.Fatalf("run(--list) tty: code=%d; want 0", code) }
            got := out.String()
            if !strings.Contains(got, "\x1b[1m") || !strings.Contains(got, "\x1b[36m") || !strings.Contains(got, "\x1b[0m") {
                t.Errorf("TTY output should contain ANSI bold/cyan/reset:\n%s", got)
            }
        }
  - NOTE: these prove the `&& !c.noColor` leg AND the `isTerminal(stdout)` leg of
    the color gate. Both use withTerminal(t, true) to force the TTY branch.

Task 8: MODIFY TestRunUndispatchedModeIsNoOp (FIX the now-false assertion)
  - FIND in main_test.go:
        func TestRunUndispatchedModeIsNoOp(t *testing.T) {
            var out, errOut bytes.Buffer
            code := run([]string{"--list"}, &out, &errOut)   // <-- NOW WRONG
            ...
        }
  - EDIT: change the arg from "--list" to "--all" (still undispatched in M2;
    --all lands in M3.T2.S1). Rename to TestRunUndispatchedAllIsNoOp. Update the
    comment: "no-op until M3.T2.S1" (was "M2.T5.S1"):
        func TestRunUndispatchedAllIsNoOp(t *testing.T) {
            // --all is parsed but NOT yet dispatched (M3.T2.S1 wires it). A no-op:
            // exit 0, no output. (Contrast: --list WAS a no-op until M2.T5.S1, which
            // added its dispatch branch — so --list is no longer tested here.)
            var out, errOut bytes.Buffer
            code := run([]string{"--all"}, &out, &errOut)
            if code != 0 {
                t.Errorf("run(--all): code=%d; want 0 (no-op until M3.T2.S1)", code)
            }
            if out.Len() != 0 || errOut.Len() != 0 {
                t.Errorf("run(--all) produced output; want none (no-op): stdout=%q stderr=%q", out.String(), errOut.String())
            }
        }
  - ALTERNATIVELY: if you prefer to keep the name TestRunUndispatchedModeIsNoOp,
    just swap "--list" -> "--all" in the body and update the comment. Either is fine;
    the KEY change is the arg must NOT be "--list".
  - CRITICAL: if you skip this task, `go test ./...` FAILS on this pre-existing test.

Task 9: VALIDATE build, vet, test, race
  - RUN: cd /home/dustin/projects/weave && go build ./...                    # expect exit 0
  - RUN: go vet ./...                                                        # expect exit 0, clean
  - RUN: go test ./... -v                                                    # expect ALL PASS
  - RUN: go test -race ./...                                                 # expect no data races
  - RUN: gofmt -l main.go main_test.go                                       # expect no output
  - RUN: go test ./... -run 'TestRunList' -v                                 # the six new tests
  - RUN: go test ./... -run 'TestRunUndispatched' -v                         # the fixed no-op test
  - RUN: grep -c "ui.PrintList" main.go                                      # expect 1
  - RUN: grep -c "no extensions found" main.go                               # expect 1
  - RUN: ! grep -q "no skills found" main.go && echo "OK (no skilldozer noun leak)" || echo "FAIL"
  - RUN: grep -c "discover.Index" main.go                                    # expect 1
  - RUN: grep -qE '"--list", "-l"' main_test.go && echo "OK (list parse test still present)" || echo "check"
```

### Implementation Patterns & Key Details

```go
// The run() function's branch structure AFTER this task (precedence top-to-bottom):
func run(args []string, stdout, stderr io.Writer) int {
	c := parseArgs(args)

	if c.version {                       // 1) --version (highest precedence)
		fmt.Fprintf(stdout, "weave %s\n", version)
		return 0
	}

	if c.path {                          // 2) --path (prints dir + source label)
		dir, src, err := extdir.Find()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, dir)
		fmt.Fprintf(stderr, "(found via %s)\n", src)  // <-- --path prints src
		return 0
	}

	if c.list {                          // 3) --list (NEW — this task)
		dir, _, err := extdir.Find()     //    <-- src DISCARDED (not printed)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(exts) == 0 {
			fmt.Fprintln(stderr, "no extensions found in "+dir)  // noun + +dir
			return 1
		}
		ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)  // color gate
		return 0
	}

	return 0                             // 4) no-op fallthrough (M3/M4/M5 fill in)
}

// Contrast --path vs --list on the `src` return:
//   --path:  dir, src, err := extdir.Find()   // src PRINTED to stderr
//   --list:  dir, _,   err := extdir.Find()   // src DISCARDED
// This is the single most subtle difference between the two branches. --list's
// stdout is a TABLE (a source label would corrupt column alignment); --list's
// stderr is EMPTY on success. --path's stdout is a single PATH (byte-exact, for
// the §13 `test "$(weave --path)" = ...` gate); --path's stderr is the source
// label (Issue 1). Different contracts, different handling of the same Find().

// The color gate, decomposed:
//   isTerminal(stdout) && !c.noColor
//   -----------------    -----------
//        |                   |
//        |                   +-- PRD §6.2 --no-color: "even on a TTY" -> overrides TTY
//        |
//        +-- TTY detection (isTerminal func var, P1.M1.T4.S1). A *bytes.Buffer
//            (tests) / pipe / redirect -> false -> no color. A real terminal ->
//            true -> color (unless --no-color). Tests force true via withTerminal.
```

### Integration Points

```yaml
MAIN.GO:
  - ADD imports: discover, ui (extdir already present).
  - ADD branch: the --list block in run() (after --path, before no-op fallthrough).
  - RENUMBER comment: `// 3)` fallthrough -> `// 4)`, drop "M2 adds --list".

MAIN_TEST.GO:
  - ADD import: os (for os.WriteFile in writeExtTree). Check existing imports first.
  - ADD helper: withTerminal (port verbatim from skilldozer).
  - ADD helper: writeExtTree (port writeSkillTree, adapt .md->.ts + JSDoc fixture).
  - ADD tests: TestRunListSuccess, TestRunListShortFlag, TestRunListNoExtensionsExit1,
    TestRunListUnresolvableExit1, TestRunListNoColorFlagSuppressesANSI,
    TestRunListColorWhenTTY.
  - MODIFY test: TestRunUndispatchedModeIsNoOp -> swap "--list" arg to "--all"
    (or rename TestRunUndispatchedAllIsNoOp). The --list assertion is now FALSE.

CONFIG:
  - none. --list uses extdir.Find (env/config/sibling/walk-up) — no new config.

DATABASE:
  - none.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after editing main.go + main_test.go — fix before proceeding.
cd /home/dustin/projects/weave
gofmt -l main.go main_test.go              # expect no output
go vet ./...                                # expect clean

# Project-wide build
go build ./...                              # expect exit 0

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
# Common errors at this stage:
#   - "imported and not used: github.com/dabstractor/weave/internal/ui" -> you
#     added the import but not the branch (do Task 1 + Task 2 together).
#   - "imported and not used: os" (main_test.go) -> writeExtTree needs os; added?
#   - "declared and not used: src" -> you wrote `dir, src, err` instead of `dir, _, err`.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The six new --list tests.
cd /home/dustin/projects/weave
go test ./... -run 'TestRunList' -v         # expect ALL PASS

# The fixed no-op test.
go test ./... -run 'TestRunUndispatched' -v # expect PASS

# Targeted: the failure paths (exit 1, empty stdout).
go test ./... -run 'TestRunListNoExtensionsExit1|TestRunListUnresolvableExit1' -v

# Targeted: the color gate (both legs).
go test ./... -run 'TestRunListColorWhenTTY|TestRunListNoColorFlagSuppressesANSI' -v

# Full suite + race detector.
go test ./... -v                            # expect ALL PASS (existing + new)
go test -race ./...                         # expect no data races

# Expected: All tests pass. If failing, debug root cause and fix implementation.
# Common failures:
#   - TestRunListSuccess fails "missing 'A demo extension.'" -> writeExtTree
#     fixture used // or /* (one star) instead of /** (JSDoc opener). Fix the helper.
#   - TestRunListSuccess fails "exit code 1" -> writeExtTree wrote <tag>/SKILL.md
#     (a dir) instead of <tag>.ts (a file). classifyDir does not recognize it.
#   - TestRunListNoExtensionsExit1 fails "stderr missing 'no extensions found'" ->
#     you ported the skilldozer noun "no skills found". Fix the message.
#   - TestRunUndispatchedModeIsNoOp fails "exit code 1 / output not empty" ->
#     you forgot Task 8 (the test still asserts --list is a no-op).
```

### Level 3: Integration Testing (System Validation)

```bash
# End-to-end: build the binary and run --list against a real (temp) store.
cd /home/dustin/projects/weave
TMPSTORE=$(mktemp -d)
cat > "$TMPSTORE/example.ts" <<'EOF'
/** A demo extension for end-to-end testing. */
export default function() {}
EOF

# Point weave at the temp store via env (rule 1) and run --list.
weave_EXTENSIONS_DIR="$TMPSTORE" go run . --list
# Expected: a TAG/NAME/DESCRIPTION table with one row:
#   TAG       NAME        DESCRIPTION
#   example   example     A demo extension for end-to-end testing.
# (NAME is "example" — the tag fallback when no package.json name; or "(none)"
# if the single-file path yields Name="". Either way, exit 0.)

# Exit code check.
weave_EXTENSIONS_DIR="$TMPSTORE" go run . --list >/dev/null; echo "exit=$?"
# Expected: exit=0

# Empty store -> exit 1, stderr message, empty stdout.
EMPTY=$(mktemp -d)
weave_EXTENSIONS_DIR="$EMPTY" go run . --list; echo "exit=$?"
# Expected: stderr "no extensions found in <EMPTY>"; exit=1; NO table on stdout.

# Unconfigured -> exit 1, one-line fix.
env -u weave_EXTENSIONS_DIR -u weave_CONFIG go run . --list 2>&1 | head -1
# Expected (if no config/sibling/walk-up resolves): "weave is not configured; run `weave init`"
# (If your dev env DOES resolve a dir via sibling/walk-up, this prints that dir's
# table instead — use t.Chdir(t.TempDir()) in the hermetic test, which is the
# authoritative check. This manual step is a smoke test, not the gate.)

# --no-color on a forced TTY (smoke): hard to force a TTY in a script; the
# withTerminal(t, true) unit test is the authoritative check for the color gate.

rm -rf "$TMPSTORE" "$EMPTY"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Verify the data-flow contract: Find -> Index -> PrintList, no short-circuit.
cd /home/dustin/projects/weave
# (The unit tests in Level 2 already pin each stage. This level is a manual
# confirmation that the three modules compose and the table is human-readable.)

# 1. Multiple extensions, sorted by RelTag (discover.Index sorts).
TMPSTORE=$(mktemp -d)
printf '/** Zeta extension. */\nexport default function(){}\n' > "$TMPSTORE/zeta.ts"
printf '/** Alpha extension. */\nexport default function(){}\n' > "$TMPSTORE/alpha.ts"
printf '/** Mid extension. */\nexport default function(){}\n' > "$TMPSTORE/mid.ts"
weave_EXTENSIONS_DIR="$TMPSTORE" go run . --list | cat -A
# Expected: rows in order alpha, mid, zeta (RelTag sort). cat -A shows no ^[[ ANSI
# escapes (pipe -> non-TTY -> no color). Columns aligned.
rm -rf "$TMPSTORE"

# 2. Long description wraps at 40 cols (ui.PrintList's descWrapWidth).
TMPSTORE=$(mktemp -d)
printf '/** %s */\nexport default function(){}\n' \
  "This is a very long description that should wrap at forty columns because the descWrapWidth constant in internal/ui is set to 40." \
  > "$TMPSTORE/wordy.ts"
weave_EXTENSIONS_DIR="$TMPSTORE" go run . --list
# Expected: the DESCRIPTION column wraps; continuation lines leave TAG/NAME blank.
rm -rf "$TMPSTORE"

# 3. Nested extension (category/tag) — proves discover.Index recursion + RelTag.
TMPSTORE=$(mktemp -d)
mkdir -p "$TMPSTORE/writing"
printf '/** Reddit poster. */\nexport default function(){}\n' > "$TMPSTORE/writing/reddit.ts"
weave_EXTENSIONS_DIR="$TMPSTORE" go run . --list
# Expected: a row with TAG "writing/reddit" (the RelTag includes the category path).
rm -rf "$TMPSTORE"
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `cd /home/dustin/projects/weave && go test ./... -v`
- [ ] `go test -race ./...` passes (no data races).
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l main.go main_test.go` (no output)
- [ ] go.mod unchanged (no new require block; no go.sum).

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] `run(["--list"])` on a populated store returns 0, table on stdout, empty stderr (TestRunListSuccess).
- [ ] `run(["-l"])` short flag behaves identically (TestRunListShortFlag).
- [ ] `run(["--list"])` on an empty store returns 1, empty stdout, `"no extensions found in <dir>"` on stderr (TestRunListNoExtensionsExit1).
- [ ] `run(["--list"])` when unconfigured returns 1, empty stdout, `"weave init"` fix on stderr (TestRunListUnresolvableExit1).
- [ ] `run(["--list", "--no-color"])` with forced TTY produces NO ANSI (TestRunListNoColorFlagSuppressesANSI).
- [ ] `run(["--list"])` with forced TTY produces ANSI bold+cyan+reset (TestRunListColorWhenTTY).
- [ ] Color gate is `isTerminal(stdout) && !c.noColor` (both legs exercised).
- [ ] `--list` does NOT print `(found via ...)` (src discarded; stderr empty on success).
- [ ] `--list` does NOT consult `c.file` / `c.relative` (modifiers N/A to a table).
- [ ] TestRunUndispatchedModeIsNoOp updated (no longer asserts `--list` is a no-op).

### Code Quality Validation

- [ ] main.go imports `discover` AND `ui` (alphabetical in the internal group).
- [ ] The `--list` branch is a near-verbatim port of skilldozer's (only noun + var-name deltas).
- [ ] NO "no skills found" string in main.go (noun swap done).
- [ ] NO "skilldozer init" string in main_test.go (noun swap done).
- [ ] `dir, _, err := extdir.Find()` (src discarded, not `dir, src, err`).
- [ ] Precedence preserved: `--version` > `--path` > `--list` (branch order unchanged).
- [ ] writeExtTree writes `<tag>.ts` files with `/** ... */` JSDoc (not `<tag>/SKILL.md`).
- [ ] withTerminal restores `isTerminal` on cleanup; no t.Parallel on color tests.

### Documentation & Deployment

- [ ] The run() fallthrough comment is renumbered (`// 4)`) and the "M2 adds --list" stale clause removed.
- [ ] No new env vars or config keys introduced.
- [ ] Code is self-documenting (the branch comment explains the Find->Index->PrintList flow + the src-discard + the modifier-N/A note).

---

## Anti-Patterns to Avoid

- ❌ Don't print the `(found via ...)` source label in `--list` — that's `--path`-only. `--list`'s stdout is a table; the label would corrupt it. Discard `src` with `_`.
- ❌ Don't port skilldozer's `writeSkillTree` verbatim — weave extensions are `.ts` files, not `<tag>/SKILL.md` dirs. Use `writeExtTree` writing `<tag>.ts` with a `/** ... */` JSDoc block.
- ❌ Don't use a single-star `/*` or `//` comment in the test fixture — `ExtractJSDoc` requires the `/**` opener (two stars). A wrong opener yields `Description=""` → renders `(none)`.
- ❌ Don't forget to modify `TestRunUndispatchedModeIsNoOp` — it currently asserts `--list` is a no-op, which is now false. The suite will fail.
- ❌ Don't add exclusivity logic (`--list --file` → exit 2) — that's M5.T1.S1. This task's branch ignores `c.file`/`c.relative` entirely.
- ❌ Don't add a `--no-color` parse branch — `parseArgs` already sets `c.noColor` (P1.M1.T4.S1). This task only READS it.
- ❌ Don't reorder the existing `--version`/`--path` branches — precedence is encoded in branch order. Insert `--list` AFTER `--path`.
- ❌ Don't wrap `err` in `fmt.Errorf` or add an "error: " prefix on the failure paths — print `err` verbatim (`fmt.Fprintln(stderr, err)`) so the `ErrNotFound` one-line fix stays copy-pasteable.
- ❌ Don't call `ui.PrintList` before the `len(exts) == 0` gate — empty store must print NOTHING to stdout (the gate returns 1 first).
- ❌ Don't add `t.Parallel()` to the color tests — `withTerminal` mutates the package-level `isTerminal` var.
