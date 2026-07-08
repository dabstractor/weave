# PRP — P1.M4.T4.S1: `chooseStore` + `resolveStore` (store selection logic)

## Goal

**Feature Goal**: Port skilldozer's proven `stdinIsTerminal` / `readPrompt` /
`chooseStore` / `resolveStore` quartet into `main.go` — the pure decision logic
that `weave init` uses to pick which directory becomes the extensions store (PRD
§8.2). `chooseStore` is a fully unit-testable pure function: the caller injects
`cwd`, `isTTY`, `defaultStore`, and a `prompt` closure, so all four §8.2 branches
(`haveStore` override → cwd-auto-detect → non-TTY-silent → TTY-prompt) are
exercisable without a real terminal. `resolveStore` is the thin I/O-bearing
wrapper that supplies real deps (`os.Getwd`, `config.DefaultStore`,
`stdinIsTerminal`, a `bufio` prompt over `os.Stdin`/`os.Stderr`) and absolutizes
the result via `filepath.Abs`.

**Deliverable**: TWO files.
- `main.go` MODIFIED — add 2 imports (`"bufio"`, `configpkg "…internal/config"`)
  and append 4 functions (`stdinIsTerminal`, `readPrompt`, `chooseStore`,
  `resolveStore`) after the current last function `extensionPath`.
- `main_test.go` MODIFIED — append 2 helpers (`mkdirWithExtEntry`,
  `failIfCalled`) and 7 `chooseStore`/`readPrompt` unit tests under a
  `--- chooseStore (M4.T4.S1) ---` section header.

NO new files. NO `go.mod`/`go.sum` change (`bufio` is stdlib; `internal/config`
is part of this module). NO `parseArgs` change (already captures `init`/`--store`
since M1.T4.S1). NO `internal/*` change. The 4 functions are defined but
**UNCALLED** in `run()` — wiring into `runInit` is **P1.M4.T4.S2**.

**Success Definition**:
- `go build ./...` exits 0; `go vet ./...` exits 0.
- `go test ./... -v` passes (all existing tests + the 7 new ones).
- `chooseStore("/tmp/x", any, true, "/def", failIfCalled)` → `("/tmp/x", nil)`,
  prompt never called.
- `chooseStore("", cwdWithEntry, false, "/def", failIfCalled)` → `(cwdWithEntry,
  nil)`, prompt never called (cwd-auto-detect wins, non-TTY is silent).
- `chooseStore("", emptyCwd, true, "/def", prompt→"")` → `("/def", nil)`.
- `chooseStore("", emptyCwd, true, "/def", prompt→"/custom")` → `("/custom", nil)`
  (VERBATIM, no `filepath.Abs` in the core).
- `resolveStore` compiles and supplies real deps; it is NOT unit-tested here
  (process-global `os.Stdin`/`os.Getwd`), deferred to S2's `runInit` integration.

## User Persona (if applicable)

**Target User**: a weave CLI user running `weave init` for the first time (the
documented first command, PRD §8.2). Also: scripts/CI running `weave init <dir>`
or `weave init --store <dir>` non-interactively.

**Use Case**: "I just installed weave — `weave init` asks where to keep my
extensions (defaulting to my cwd if it already looks like a store, else
`$XDG_DATA_HOME/weave/extensions`), Enter accepts the default, or I type a path."

**User Journey** (the S2 `runInit` will own steps 2-5; THIS task is step 1):
1. `weave init` → `resolveStore("")` → `chooseStore` decides → returns an
   absolute store path.
2. (S2) `setupStore` mkdir's it, seeds `example.ts` if empty, writes
   `config.yaml`.
3. (S2) prints `weave --path` + `weave check`.

**Pain Points Addressed**: the §8.2 "Prompt safety" load-bearing contract — the
bare `weave <tag>` path NEVER prompts (only `init` does), and `init` only prompts
when stdin is a real TTY (`isatty(stdin)`), so `weave init < /dev/null` and
`echo | weave init` (pipes/CI) accept the default instead of hanging.

## Why

- **PRD §8.2**: `weave init` is the documented first command and the ONLY place
  weave prompts interactively. The store-selection decision (which dir, prompted
  or not) is the heart of §8.2. This task ports the proven decision logic so S2
  only has to write the (mkdir/seed/config/write) effects.
- **Pure-function factoring = testability**: by isolating `chooseStore` from all
  I/O (cwd/isTTY/defaultStore/prompt are PARAMETERS), the entire 4-branch §8.2
  decision is unit-testable with fake closures — no pty, no `t.Chdir`, no
  process-global stdin. This is the single biggest risk-reducer for the `init`
  milestone.
- **Prompt safety is load-bearing (PRD §8.2 last paragraph / §6.4)**: a missed
  TTY gate would hang `weave` inside `$(...)` command substitution. The
  `!isTTY → return default, no prompt` branch is the guard, and it has a dedicated
  "prompt must not be called" test.
- **Scope boundary**: this task is the store-CHOICE half only. The store-EFFECTS
  half (mkdir/seed/config) is S2; the `run()` dispatch (`if c.init`) is S2. This
  keeps the diff small, the tests pure, and the two halves independently
  reviewable. `resolveStore` is defined here (so its contract is pinned and its
  imports land) but left uncalled.

## What

### 1. Imports (main.go) — 2 lines added

```go
import (
	"bufio"                                                       // NEW (alphabetical: bufio < fmt)

	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	configpkg "github.com/dabstractor/weave/internal/config"     // NEW — ALIASED (local `config` struct at line 77 conflicts; see Gotchas)
	"github.com/dabstractor/weave/internal/discover"
	"github.com/dabstractor/weave/internal/extdir"
	"github.com/dabstractor/weave/internal/resolve"
	"github.com/dabstractor/weave/internal/ui"
)
```
> NOTE on the parallel sibling P1.M4.T3.S1: it adds `internal/check` and
> `internal/search` to the SAME import block. Alphabetical final order (stdlib
> group, then internal group): `bufio, fmt, io, os, path/filepath, strings` then
> `check, config, discover, extdir, resolve, search, ui`. No semantic conflict;
> resolve the textual merge so the block stays alphabetical. `bufio` and
> `configpkg` are "used" by `readPrompt`/`resolveStore`, so no "imported and not
> used" error.

### 2. The 4 functions (append AFTER `extensionPath`, the current last function)

```go
// stdinIsTerminal reports whether os.Stdin is an interactive terminal (a
// character device), used by resolveStore to gate init's interactive prompt
// (PRD §8.2 "Prompt safety": prompt ONLY on a real TTY). It stats os.Stdin and
// checks the os.ModeCharDevice bit — the same stdlib heuristic the package-level
// isTerminal var uses for stdout color gating, but on a DIFFERENT stream.
//
// It is a plain FUNCTION, NOT a package var: the contract's test seam is
// chooseStore's isTTY PARAMETER (injected per-call), not a global override.
// (Contrast isTerminal, which IS a var because run() has no isTTY parameter.)
//
// Known harmless caveat: /dev/null is also a char device, so stdinIsTerminal()
// reports true for `weave init < /dev/null`, but a read there yields immediate
// EOF and readPrompt returns the default (never blocks). No golang.org/x/term
// (the ModeCharDevice heuristic is the established repo pattern).
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPrompt prints the prompt (label, with [def] in brackets) to w, reads one
// line from r, and returns the trimmed answer — or def when the user presses
// Enter (empty line) or sends EOF on an otherwise-empty line. A genuine read
// error (non-EOF) is returned. Used by init's interactive prompt (PRD §8.2).
//
// bufio.Reader.ReadString('\n') is preferred over bufio.Scanner (Scanner is
// line-oriented but awkward for a single interactive read; ReadString returns
// (line, error) where error == io.EOF if no newline precedes end-of-input — and
// a bare EOF with empty text means "accept default", NOT a hard error, so
// `weave init < /dev/null` and `echo | weave init` behave like "press Enter").
func readPrompt(r *bufio.Reader, w io.Writer, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	line, err := r.ReadString('\n') // includes the trailing '\n'
	if err != nil && err != io.EOF {
		return "", err
	}
	if s := strings.TrimSpace(line); s != "" {
		return s, nil
	}
	return def, nil // empty Enter OR EOF-with-no-text ⇒ accept default
}

// chooseStore resolves the store directory for `weave init` (PRD §8.2) via a
// 4-step decision that is fully independent of os.Stdin/os.Stdout/os.Getwd: the
// caller injects cwd, isTTY, the default store, and a prompt function, so the
// logic is unit-testable without a real terminal (the contract FACTORING).
//
// Resolution order (first applicable wins):
//  1. haveStore != "" — the non-interactive override from `init <dir>` or
//     `--store <dir>`. Returned VERBATIM; the prompt is NEVER called (scripts/CI).
//  2. auto-detect the default: if cwd already looks like a store (it contains at
//     least one extension entry at any depth — extdir.HasExtensionEntry, PRD §8.2
//     "detected extensions in <cwd>"), default = cwd; else default = defaultStore
//     (the $XDG_DATA_HOME/weave/extensions value from config.DefaultStore).
//  3. !isTTY and no explicit haveStore — return the auto-detected default with NO
//     prompt (scripts / CI / pipes). The prompt is NEVER called.
//  4. isTTY — prompt "Where should weave keep your extensions? [<default>]".
//     readPrompt makes empty line / EOF ⇒ default; a typed path ⇒ override.
//
// The chosen string is returned VERBATIM (it may be relative if the user typed a
// relative path); resolveStore absolutizes it via filepath.Abs. A non-nil error
// is returned ONLY on a genuine prompt read failure (a non-EOF error from the
// prompt fn); empty/EOF is "accept default", never an error.
//
// WIRED BY P1.M4.T4.S2's runInit (via resolveStore). Not yet called in run().
func chooseStore(haveStore, cwd string, isTTY bool, defaultStore string, prompt func(label, def string) (string, error)) (string, error) {
	// (1) Non-interactive override: `init <dir>` / `--store <dir>`. No prompt.
	if haveStore != "" {
		return haveStore, nil
	}
	// (2) Auto-detect the default from cwd (PRD §8.2 "detected extensions in <cwd>").
	def := defaultStore
	if extdir.HasExtensionEntry(cwd) {
		def = cwd
	}
	// (3) Off-TTY (pipe/file/CI): use the default, NO prompt (never blocks).
	if !isTTY {
		return def, nil
	}
	// (4) Interactive: prompt. Empty/EOF answer ⇒ def (the auto-detected default);
	// a typed path ⇒ override (returned verbatim). A genuine read error propagates.
	choice, err := prompt("Where should weave keep your extensions?", def)
	if err != nil {
		return "", err
	}
	if choice == "" {
		return def, nil
	}
	return choice, nil
}

// resolveStore is the I/O-bearing wrapper around chooseStore that run()'s init
// dispatch (P1.M4.T4.S2) calls. It supplies the real dependencies — os.Getwd(),
// configpkg.DefaultStore(), the os.Stdin TTY check (stdinIsTerminal), and a bufio
// prompt reader over os.Stdin/os.Stderr (readPrompt) — and returns chooseStore's
// choice ABSOLUTIZED via filepath.Abs (PRD §8.2 "absolute store path"). The ONE
// shared bufio.NewReader is created here and captured by the prompt closure so a
// future second prompt would reuse it (a fresh reader per prompt can swallow
// buffered bytes).
//
// The os.Stdin / os.Getwd access is confined to THIS function so the pure
// decision logic in chooseStore stays terminal-free and unit-testable. The prompt
// is written to STDERR (not stdout) so init's stdout stays the bare store path —
// a caller doing store="$(weave init)" must not capture the interactive prompt
// line (PRD §6.1/§6.4 spirit). A genuine cwd/default/absolutize/prompt error is
// returned wrapped; an empty or EOF prompt answer is NOT an error (readPrompt ⇒
// default).
//
// WIRED BY P1.M4.T4.S2's runInit. Not yet called in run().
func resolveStore(haveStore string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve cwd: %w", err)
	}
	def, err := configpkg.DefaultStore()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve default store: %w", err)
	}
	r := bufio.NewReader(os.Stdin)
	// Prompt dialog goes to STDERR, not stdout, so it never pollutes a
	// store="$(weave init)" capture. readPrompt takes a generic io.Writer so the
	// choice is made HERE in the wrapper, not baked into the pure readPrompt.
	prompt := func(label, def string) (string, error) {
		return readPrompt(r, os.Stderr, label, def)
	}
	store, err := chooseStore(haveStore, cwd, stdinIsTerminal(), def, prompt)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(store)
	if err != nil {
		return "", fmt.Errorf("weave init: absolutize store: %w", err)
	}
	return abs, nil
}
```

### Success Criteria

- [ ] `chooseStore` has the EXACT signature `func chooseStore(haveStore, cwd string, isTTY bool, defaultStore string, prompt func(label, def string) (string, error)) (string, error)`.
- [ ] `chooseStore` auto-detect uses `extdir.HasExtensionEntry(cwd)` (NOT
      skilldozer's `HasSkillMD`).
- [ ] The prompt label is `"Where should weave keep your extensions?"` (PRD §8.2
      VERBATIM — "weave" and "extensions", not "skilldozer"/"skills").
- [ ] The non-interactive branches (`haveStore != ""`, `!isTTY`) NEVER call the
      prompt fn (enforced by `failIfCalled` tests).
- [ ] `chooseStore` returns the chosen string VERBATIM (no `filepath.Abs` inside
      it); `resolveStore` applies `filepath.Abs` after.
- [ ] `resolveStore` writes the prompt to `os.Stderr` (NOT `os.Stdout`).
- [ ] `resolveStore`'s prompt closure reuses ONE shared `bufio.NewReader(os.Stdin)`.
- [ ] Error wrappers use the `"weave init: …"` prefix (NOT "skilldozer init:").
- [ ] `internal/config` is imported with the `configpkg` ALIAS (the local `config`
      struct at main.go:77 conflicts with the package name).
- [ ] The 4 functions are NOT yet called in `run()` (that is S2). `go build`
      passes with them unused (Go allows unused package-level functions).
- [ ] `go build ./...`, `go vet ./...`, `go test ./... -v` all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the 4 functions are a near-verbatim
port of skilldozer's shipped `main.go:808-960` (read in full), the 4
weave-specific deltas are tabulated (HasSkillMD→HasExtensionEntry; skilldozer→
weave/skills→extensions; error prefix; import path), the consumed APIs
(`extdir.HasExtensionEntry`, `configpkg.DefaultStore`) are confirmed exported with
pinned signatures and line numbers, the `configpkg` alias requirement is
explained (local `config` struct conflict), the prompt-to-stderr decision is
documented with the source discrepancy resolved in favor of shipped code, and the
7 test bodies + 2 helpers are specified verbatim from skilldozer's test suite. No
guessing required.

### Documentation & References

```yaml
# MUST READ — the exact functions to port (the source of truth)
- file: /home/dustin/projects/skilldozer/main.go
  why: contains the shipped stdinIsTerminal (L808), readPrompt (L821),
       chooseStore (L858), resolveStore (L924). Port nearly verbatim with the 4
       weave deltas. resolveStore's prompt closure (L947) writes to os.Stderr.
  pattern: stdinIsTerminal = os.Stdin.Stat() & ModeCharDevice; readPrompt uses
           bufio.Reader.ReadString('\n') with empty/EOF⇒def; chooseStore is a pure
           4-step decision with injected deps; resolveStore supplies real deps +
           filepath.Abs.
  gotcha: skilldozer's early research (external_deps.md §4 / verified_facts.md §3)
          sketches os.Stdout for the prompt writer; the SHIPPED resolveStore uses
          os.Stderr with a clear rationale (init stdout = bare store path). weave
          follows the shipped code: os.Stderr.

# The exact chooseStore tests to port (the test template)
- file: /home/dustin/projects/skilldozer/main_test.go
  why: L2340-2435 has failIfCalled + mkdirWithSkillMD + the 7 TestChooseStore*
        functions. Port with mkdirWithSkillMD→mkdirWithExtEntry (writes
        sub/index.ts instead of sub/SKILL.md so extdir.HasExtensionEntry is true).
  pattern: fake prompt fn closures; failIfCalled sentinel; t.TempDir() for cwd;
           no t.Chdir (cwd is a param). assertions use exact VERBATIM strings.

# The current files being modified
- file: main.go
  why: the file under edit. ADD 2 imports; APPEND 4 functions after extensionPath
       (the current last fn, ~L570). The import block currently has NO bufio and
       NO internal/config.
  pattern: the existing isTerminal var (L40) is the established ModeCharDevice
           TTY pattern on stdout; stdinIsTerminal mirrors it on os.Stdin.
  gotcha: main.go:77 declares `type config struct` — importing
          internal/config (package name `config`) WITHOUT an alias is a
          "config redeclared" compile error. Use configpkg alias.

- file: main_test.go
  why: APPEND 2 helpers (mkdirWithExtEntry, failIfCalled) + 7 tests under a
       "--- chooseStore (M4.T4.S1) ---" section. Existing helpers (writeExtTree,
       sampleStore) are NOT reused here — chooseStore tests build a cwd fixture
       directly (cwd is a param, no store wiring needed).
  pattern: t.Helper() + t.TempDir() + filepath.Join; no t.Parallel (none of the
           existing main_test.go tests use it).

# The consumed APIs (verified exported, with line numbers)
- file: internal/extdir/extdir.go
  why: L331 `func HasExtensionEntry(dir string) bool` — the §8.2 cwd-auto-detect
       predicate AND the §8.3 rule-4 qualifier. Walks for ANY §7.1 entry at any
       depth, early-exits. This is weave's HasSkillMD equivalent.
  gotcha: it needs an actual .ts/.js FILE or a dir with index.ts/package.json to
          report true — an empty dir or a dir of only subdirs reports false. The
          test fixture writes sub/index.ts (a dir-kind §7.1 entry).
- file: internal/config/config.go
  why: L158 `func DefaultStore() (string, error)` — returns the XDG default
       store. The package ALREADY appends "weave/extensions" to the XDG data
       home; do NOT add it again in main.go.
  gotcha: the package name is `config`; main.go:77 has a local `config` struct →
          import with the `configpkg` alias.

# skilldozer's prior research (the exact-same task shape, read for fidelity)
- docfile: /home/dustin/projects/skilldozer/plan/002_38acb6d28a6a/P1M2T2S1/research/verified_facts.md
  why: skilldozer's P1.M2.T2.S1 IS this task (chooseStore core + TTY-gated
       prompt). Its §2-§7 document the factoring, the consumed APIs, and the
       test matrix that this PRP ports. Confirms stdinIsTerminal is a FUNCTION
       (not a var) because the test seam is chooseStore's isTTY PARAM.

# PRD spec (authoritative)
- docfile: PRD.md
  section: §8.2 (the init flow: cwd-auto-detect first, then XDG default, then
           prompt only on TTY; "Where should weave keep your extensions?
           [<default>]" label text VERBATIM; Enter accepts default, typing
           overrides; Prompt safety — bare `weave <tag>` never prompts, init
           prompts only on isatty(stdin)). §8.1 (config.Save, used by S2 not S1).
```

### Current Codebase tree (relevant subset)

```bash
main.go                  # ← MODIFIED: +2 imports, +4 functions appended
main_test.go             # ← MODIFIED: +2 helpers, +7 tests appended
internal/
├── config/config.go     # DefaultStore() — CONSUMED (M1.T2.S1, done)
├── extdir/extdir.go     # HasExtensionEntry() — CONSUMED (M1.T3.S1, done)
├── discover/…           # NOT touched by this task
├── resolve/…            # NOT touched
├── search/…             # NOT touched
├── check/…              # NOT touched
└── ui/…                 # NOT touched
```

### Desired Codebase tree with files to be added/modified

```bash
main.go          # MODIFIED — +bufio/+configpkg imports; +stdinIsTerminal/+readPrompt/+chooseStore/+resolveStore (appended, uncalled)
main_test.go     # MODIFIED — +mkdirWithExtEntry/+failIfCalled helpers; +7 TestChooseStore*/TestReadPrompt* tests
# (no new files; no go.mod/go.sum change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: import internal/config WITH THE configpkg ALIAS. main.go:77 declares
// `type config struct` (the parsed-CLI struct). A bare import of internal/config
// (package name `config`) collides → "config redeclared in this block". Use:
//   configpkg "github.com/dabstractor/weave/internal/config"
// and call configpkg.DefaultStore(). (skilldozer hit the identical conflict.)

// CRITICAL: the prompt writes to os.Stderr, NOT os.Stdout. skilldozer's shipped
// resolveStore (main.go:947) binds the readPrompt writer to os.Stderr so init's
// stdout stays the bare store path (a `store="$(weave init)"` capture must not
// grab the interactive "Where should weave keep your extensions?" line). The
// early research docs (external_deps.md §4, verified_facts.md §3) sketch
// os.Stdout — the SHIPPED CODE wins. readPrompt itself takes a generic io.Writer
// so the choice lives in resolveStore's closure, not baked into readPrompt.

// CRITICAL: stdinIsTerminal is a FUNCTION, not a package var. The contract's test
// seam is chooseStore's isTTY PARAMETER (injected per-call), so there is nothing
// to override globally. (Contrast the package-level isTerminal VAR for stdout
// color gating, which IS a var because run() has no isTTY param — overridden by
// withTerminal in tests.) Do NOT make stdinIsTerminal a var.

// CRITICAL: do NOT add expandHome. skilldozer added it in a LATER subtask and
// wired it into runInit (S3), NOT into resolveStore. The item description pins
// resolveStore as "return chooseStore result absolutized via filepath.Abs" —
// exactly that, nothing more. expandHome is out of scope (S2/future).

// CRITICAL: the config package ALREADY appends "weave/extensions" to the XDG
// data home in DefaultStore(). Do NOT add "/extensions" again in main.go —
// configpkg.DefaultStore() returns the full path ready to use.

// CRITICAL: chooseStore returns the chosen string VERBATIM (cwd/defaultStore/
// typed-path as-passed). filepath.Abs is applied ONLY in resolveStore. This keeps
// chooseStore a pure decision fn so unit tests assert exact strings
// (prompt "/custom" ⇒ "/custom", NOT filepath.Abs("/custom")).

// GOTCHA: bufio.Reader.ReadString('\n') returns (line, io.EOF) when there is no
// newline before end-of-input. A bare io.EOF with empty text = "accept default",
// NOT a hard error — that is what makes `weave init < /dev/null` and
// `echo | weave init` behave like "press Enter". Only `err != nil && err != io.EOF`
// is a genuine error (propagate up).

// GOTCHA: /dev/null is a char device, so stdinIsTerminal() reports TRUE for
// `weave init < /dev/null`. This is harmless: the read yields immediate EOF and
// readPrompt returns the default (never blocks). Do NOT add special-case logic.

// GOTCHA: the 4 functions are defined but NOT called in run() (wiring is S2).
// Go allows unused package-level functions — `go build` passes. BUT the two new
// imports (bufio, configpkg) ARE used (by readPrompt/resolveStore), so there is
// no "imported and not used" error. If you define the functions but forget to
// reference an import, `go build` will flag the unused import.

// GOTCHA: extdir.HasExtensionEntry(cwd) needs a REAL §7.1 entry to return true.
// The test fixture mkdirWithExtEntry writes sub/index.ts (a dir-kind entry) under
// the temp dir. An empty temp dir or a dir of only empty subdirs reports FALSE
// (walks the whole tree, finds nothing) — use t.TempDir() directly for the
// "cwd-without-extensions" negative case.
```

## Implementation Blueprint

### Data models and structure

None new. This task is pure decision logic + a thin I/O wrapper. It consumes:
- `extdir.HasExtensionEntry(dir string) bool` (already exported).
- `configpkg.DefaultStore() (string, error)` (already exported).
- stdlib `os.Stdin`, `os.Stderr`, `os.Getwd`, `filepath.Abs`, `bufio.Reader`.

It produces: four functions, all taking/returning plain `string`/`error`/`func`.
No structs, no interfaces, no state.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD imports to main.go
  - EDIT main.go import block: add `"bufio"` as the FIRST stdlib entry (bufio<fmt)
    and `configpkg "github.com/dabstractor/weave/internal/config"` in the internal
    group, alphabetical BEFORE internal/discover (config < discover).
  - RUN: go build ./...
  - EXPECT: "imported and not used" errors until Task 2 adds the functions that
    reference bufio and configpkg — that is expected; proceed. (Or do Task 1+2 in
    one edit pass.)
  - FILES TOUCHED: 1 (main.go).
  - MERGE NOTE: if the parallel P1.M4.T3.S1 has already landed its check/search
    imports, the final internal group is alphabetical:
    check, config, discover, extdir, resolve, search, ui.

Task 2: APPEND the 4 functions to main.go
  - EDIT main.go: append stdinIsTerminal, readPrompt, chooseStore, resolveStore
    (see "What" §2) AFTER the current last function `extensionPath` (~L570). Each
    gets the doc comment shown (cites PRD §8.2; notes "WIRED BY S2's runInit").
  - PORT FROM: /home/dustin/projects/skilldozer/main.go L808-960 (read in full).
  - DELTAS vs skilldozer (apply ALL FOUR):
    (a) skillsdir.HasSkillMD(cwd) → extdir.HasExtensionEntry(cwd)
    (b) prompt label "Where should skilldozer keep your skills?" →
        "Where should weave keep your extensions?"
    (c) error prefix "skilldozer init:" → "weave init:" (3 sites: resolve cwd,
        resolve default store, absolutize store)
    (d) import path github.com/dabstractor/skilldozer/internal/config →
        github.com/dabstractor/weave/internal/config (keep the configpkg alias)
  - KEEP from skilldozer: prompt closure writes to os.Stderr (NOT os.Stdout);
    chooseStore returns VERBATIM (no Abs in the core); resolveStore applies
    filepath.Abs; the !isTTY branch returns def with NO prompt.
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. Most likely failure: forgot the configpkg alias (compile error
    "config redeclared in this block") or used "skills"/"skilldozer" in a string.
  - FILES TOUCHED: 1 (main.go).

Task 3: VALIDATE main.go compiles + vets clean (functions uncalled)
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. The 4 functions are unused but that is legal Go (unused
    package-level fns are allowed). The 2 new imports are USED by readPrompt/
    resolveStore, so no "imported and not used".

Task 4: ADD 2 helpers + 7 tests to main_test.go
  - EDIT main_test.go: append under a "--- chooseStore (M4.T4.S1) ---" section
    header, following the existing t.Helper()+t.TempDir()+filepath.Join style.
    Add mkdirWithExtEntry + failIfCalled helpers (see Validation Loop §Level 2)
    and the 7 tests (full bodies in Validation Loop §Level 2):
      TestChooseStoreExplicitOverrideNoPrompt,
      TestChooseStoreCwdDetectNonTTY,
      TestChooseStoreNoExtNonTTYUsesDefault,
      TestChooseStoreTTYEmptyPromptAcceptsDefault,
      TestChooseStoreTTYTypedPathOverrides,
      TestChooseStoreCwdDetectIsAlsoTheTTYDefault,
      TestChooseStorePropagatesPromptError.
    (Plus TestReadPromptFormatsDefAndTrims — optional 8th, on readPrompt itself.)
  - PORT FROM: /home/dustin/projects/skilldozer/main_test.go L2340-2435.
  - DELTA: mkdirWithSkillMD (writes sub/SKILL.md) → mkdirWithExtEntry (writes
    sub/index.ts) so extdir.HasExtensionEntry reports true.
  - FILES TOUCHED: 1 (main_test.go).

Task 5: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./... -v ; go test -race ./...
  - EXPECT: all green. The new tests must not disturb the existing
    --version/--path/--list/--all/<tag>/--search/check tests (they append new
    Test* functions; the dispatch ladder in run() is untouched by this task).
```

### Implementation Patterns & Key Details

```go
// stdinIsTerminal: same ModeCharDevice heuristic as the isTerminal var, but on
// os.Stdin and as a plain FUNCTION (the test seam is chooseStore's isTTY param).
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPrompt: bufio.Reader.ReadString('\n'); empty/EOF ⇒ def (NOT an error).
// Generic io.Writer so resolveStore can bind it to os.Stderr.
func readPrompt(r *bufio.Reader, w io.Writer, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if s := strings.TrimSpace(line); s != "" {
		return s, nil
	}
	return def, nil
}

// chooseStore: pure 4-step decision, all deps injected. Returns VERBATIM.
func chooseStore(haveStore, cwd string, isTTY bool, defaultStore string, prompt func(label, def string) (string, error)) (string, error) {
	if haveStore != "" {             // (1) override — no prompt
		return haveStore, nil
	}
	def := defaultStore              // (2) auto-detect from cwd
	if extdir.HasExtensionEntry(cwd) {
		def = cwd
	}
	if !isTTY {                     // (3) non-TTY — no prompt
		return def, nil
	}
	choice, err := prompt("Where should weave keep your extensions?", def) // (4) TTY
	if err != nil {
		return "", err
	}
	if choice == "" {
		return def, nil
	}
	return choice, nil
}

// resolveStore: I/O wrapper. ONE shared bufio reader; prompt → os.Stderr;
// result absolutized. Wired by S2's runInit (not yet called).
func resolveStore(haveStore string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve cwd: %w", err)
	}
	def, err := configpkg.DefaultStore()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve default store: %w", err)
	}
	r := bufio.NewReader(os.Stdin)
	prompt := func(label, def string) (string, error) {
		return readPrompt(r, os.Stderr, label, def) // STDERR, not stdout
	}
	store, err := chooseStore(haveStore, cwd, stdinIsTerminal(), def, prompt)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(store)
	if err != nil {
		return "", fmt.Errorf("weave init: absolutize store: %w", err)
	}
	return abs, nil
}
```

### Integration Points

```yaml
CONSUMES:
  - extdir.HasExtensionEntry(dir string) bool   # §8.2 cwd-auto-detect predicate
  - configpkg.DefaultStore() (string, error)    # XDG default store
  - os.Stdin (TTY stat + bufio reader)          # stdinIsTerminal + readPrompt
  - os.Stderr                                   # prompt writer (NOT stdout)
  - os.Getwd, filepath.Abs                      # cwd + absolutize

PRODUCES:
  - 4 functions in main.go: stdinIsTerminal, readPrompt, chooseStore, resolveStore.
  - chooseStore is the unit-testable pure decision core; resolveStore is its
    I/O wrapper (called by S2's runInit via resolveStore(c.initStore)).

NO CHANGES TO:
  - run() dispatch (the `if c.init { … }` branch is S2).
  - parseArgs (already captures init/--store/initStore since M1.T4.S1).
  - any internal/* package.
  - go.mod / go.sum (bufio is stdlib; internal/config is part of this module).
  - PRD.md / tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1-2 (main.go edits):
go build ./...
go vet ./...

# Expected: zero errors. Most likely failures:
#   (a) "config redeclared in this block" — you imported internal/config WITHOUT
#       the configpkg alias. Fix: add the alias.
#   (b) "imported and not used: bufio" / "...internal/config" — you declared the
#       imports but haven't appended the functions yet. Fix: do Task 1+2 together.
#   (c) a stale skilldozer string ("skills"/"skilldozer") in a label or error —
#       grep main.go for "skill" to catch leftovers.
# Also check: bufio is the FIRST stdlib import; configpkg is alphabetical before
# internal/discover.
```

### Level 2: Unit Tests (Component Validation)

Add these to `main_test.go` under a new `// --- chooseStore (M4.T4.S1) ---`
section header. They follow the existing `t.Helper()` + `t.TempDir()` +
`filepath.Join` style and are white-box (`package main`).

```go
// --- chooseStore (M4.T4.S1) ---

// mkdirWithExtEntry makes a temp dir that extdir.HasExtensionEntry reports as a
// store (contains a §7.1 entry at depth): tmp/sub/index.ts. cwd is a PARAMETER
// to chooseStore, so no t.Chdir is needed (unlike resolveStore which calls
// os.Getwd). Mirrors skilldozer's mkdirWithSkillMD with index.ts for the
// extension-kind predicate.
func mkdirWithExtEntry(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "index.ts"),
		[]byte("export default function() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile index.ts: %v", err)
	}
	return dir
}

// failIfCalled returns a prompt fn that fails the test if chooseStore invokes it.
// Enforces the prompt-safety guarantee (PRD §8.2): the non-interactive branches
// (`haveStore != ""` and `!isTTY`) must NEVER call the prompt fn.
func failIfCalled(t *testing.T) func(string, string) (string, error) {
	t.Helper()
	return func(label, def string) (string, error) {
		t.Errorf("chooseStore: prompt must not be called (label=%q)", label)
		return "", nil
	}
}

// OUTPUT #1: `init --store /tmp/x` ⇒ /tmp/x; prompt NEVER called.
func TestChooseStoreExplicitOverrideNoPrompt(t *testing.T) {
	got, err := chooseStore("/tmp/x", "/any/cwd", true, "/def", failIfCalled(t))
	if err != nil || got != "/tmp/x" {
		t.Errorf("chooseStore(/tmp/x,...): got (%q,%v); want (/tmp/x,nil)", got, err)
	}
}

// OUTPUT #2: cwd-with-extension + non-TTY ⇒ cwd; prompt NEVER called.
func TestChooseStoreCwdDetectNonTTY(t *testing.T) {
	cwd := mkdirWithExtEntry(t)
	got, err := chooseStore("", cwd, false, "/def", failIfCalled(t))
	if err != nil || got != cwd {
		t.Errorf("chooseStore(cwd-with-ext,non-TTY): got (%q,%v); want (%q,nil)", got, err, cwd)
	}
}

// OUTPUT #3: cwd-without-extension + non-TTY ⇒ defaultStore; prompt NEVER called.
func TestChooseStoreNoExtNonTTYUsesDefault(t *testing.T) {
	got, err := chooseStore("", t.TempDir(), false, "/def", failIfCalled(t))
	if err != nil || got != "/def" {
		t.Errorf("chooseStore(empty-cwd,non-TTY): got (%q,%v); want (/def,nil)", got, err)
	}
}

// OUTPUT #4: isTTY + prompt "" ⇒ default (cwd-without-ext so default=defaultStore).
func TestChooseStoreTTYEmptyPromptAcceptsDefault(t *testing.T) {
	prompt := func(label, def string) (string, error) { return "", nil }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err != nil || got != "/def" {
		t.Errorf("chooseStore(TTY,empty-prompt): got (%q,%v); want (/def,nil)", got, err)
	}
}

// OUTPUT #5: isTTY + prompt "/custom" ⇒ /custom (VERBATIM — no Abs in the core).
func TestChooseStoreTTYTypedPathOverrides(t *testing.T) {
	prompt := func(label, def string) (string, error) { return "/custom", nil }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err != nil || got != "/custom" {
		t.Errorf("chooseStore(TTY,typed-/custom): got (%q,%v); want (/custom,nil)", got, err)
	}
}

// The cwd-auto-detect DEFAULT is cwd even on a TTY; an empty prompt answer
// accepts that cwd default (not defaultStore). Guards against a bug where
// HasExtensionEntry is only consulted on the !isTTY branch.
func TestChooseStoreCwdDetectIsAlsoTheTTYDefault(t *testing.T) {
	cwd := mkdirWithExtEntry(t)
	prompt := func(label, def string) (string, error) {
		if def != cwd {
			t.Errorf("prompt default=%q; want cwd %q (auto-detect)", def, cwd)
		}
		return "", nil // Enter ⇒ accept the cwd default
	}
	got, err := chooseStore("", cwd, true, "/def", prompt)
	if err != nil || got != cwd {
		t.Errorf("chooseStore(cwd-with-ext,TTY,empty): got (%q,%v); want (%q,nil)", got, err, cwd)
	}
}

// A genuine (non-EOF) prompt read error is returned, not swallowed.
func TestChooseStorePropagatesPromptError(t *testing.T) {
	wantErr := errors.New("simulated read failure")
	prompt := func(label, def string) (string, error) { return "", wantErr }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err == nil || !errors.Is(err, wantErr) {
		t.Errorf("chooseStore(prompt-error): got (%q,%v); want error wrapping %v", got, err, wantErr)
	}
}

// (Optional, 8th) readPrompt formats "%s [%s]: " and trims; empty/EOF ⇒ def.
func TestReadPromptFormatsDefAndTrims(t *testing.T) {
	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("/typed/path\n"))
	got, err := readPrompt(r, &out, "Where", "/def")
	if err != nil || got != "/typed/path" {
		t.Errorf("readPrompt(typed): got (%q,%v); want (/typed/path,nil)", got, err)
	}
	if want := "Where [/def]: "; out.String() != want {
		t.Errorf("readPrompt output=%q; want %q", out.String(), want)
	}
	// Empty line ⇒ def.
	var out2 bytes.Buffer
	r2 := bufio.NewReader(strings.NewReader("\n"))
	got2, err := readPrompt(r2, &out2, "Where", "/def")
	if err != nil || got2 != "/def" {
		t.Errorf("readPrompt(empty): got (%q,%v); want (/def,nil)", got2, err)
	}
	// EOF no text ⇒ def (not an error).
	var out3 bytes.Buffer
	r3 := bufio.NewReader(strings.NewReader(""))
	got3, err := readPrompt(r3, &out3, "Where", "/def")
	if err != nil || got3 != "/def" {
		t.Errorf("readPrompt(EOF): got (%q,%v); want (/def,nil)", got3, err)
	}
}
```

```bash
# After Task 4:
go test ./... -v
# Targeted re-run while debugging:
go test -run 'TestChooseStore|TestReadPrompt' -v

# Expected: all green. On failure, the most common causes:
#   (a) chooseStore returned filepath.Abs("/custom") instead of "/custom" — you
#       put Abs in the core instead of in resolveStore.
#   (b) prompt was called on a non-interactive branch — the failIfCalled test
#       fires; check the branch ORDER (override → auto-detect → !isTTY → isTTY).
#   (c) TestChooseStoreCwdDetectNonTTY fails — mkdirWithExtEntry did not create
#       a §7.1 entry HasExtensionEntry recognizes (must be sub/index.ts, a
#       dir-kind entry; an empty dir reports false).
#   (d) TestChooseStoreCwdDetectIsAlsoTheTTYDefault fails — HasExtensionEntry is
#       only consulted inside the !isTTY branch (it must run BEFORE the TTY
#       branch, in the shared auto-detect step).
```

### Level 3: Integration Testing (System Validation)

```bash
# This subtask deliberately does NOT wire resolveStore into run() (that is S2),
# so there is no end-to-end `weave init` command to exercise yet. The integration
# proof is compile + vet + the full test suite. Sanity-check the binary still
# builds and existing commands are unaffected:
go build -o /tmp/weave .
/tmp/weave --version          # "weave dev" (or the ldflags version)
echo "exit=$?"

# Confirm the 4 new functions are present in the binary (unused package-level fns
# are still compiled in):
go tool nm /tmp/weave 2>/dev/null | grep -E "main\.(chooseStore|resolveStore|stdinIsTerminal|readPrompt)$" | sort
# Expected: 4 lines (one per function). If any are missing, the build dropped them
# (e.g. a build-tag or the function was defined inside another function by a
# brace typo).

rm -f /tmp/weave
# Expected: --version exit 0; nm lists all 4 functions.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Prompt-safety static proof: grep that the prompt closure binds os.Stderr (NOT
# os.Stdout) and that chooseStore has NO filepath.Abs call. This is the
# load-bearing §8.2/§6.4 contract — a regression here would hang weave in $(...).
echo "--- prompt writer (must be os.Stderr) ---"
grep -n "readPrompt(r, os\." main.go
# Expected: exactly one hit — readPrompt(r, os.Stderr, label, def)

echo "--- Abs in chooseStore (must be NONE) ---"
awk '/^func chooseStore/,/^}/' main.go | grep -n "filepath.Abs" && \
  echo "FAIL: Abs leaked into chooseStore core" || echo "OK: chooseStore is Abs-free"

echo "--- prompt label (must be weave/extensions) ---"
grep -n "Where should" main.go
# Expected: Where should weave keep your extensions?

echo "--- leftover skilldozer strings (must be NONE) ---"
grep -ni "skilldozer\|keep your skills" main.go && echo "FAIL: leftover skilldozer string" || echo "OK: no skilldozer leftovers"

# Expected: all four checks OK.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./... -v` — all tests pass (existing + 7-8 new).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged).

### Feature Validation

- [ ] `chooseStore` signature matches the item description EXACTLY.
- [ ] Auto-detect uses `extdir.HasExtensionEntry(cwd)` (NOT `HasSkillMD`).
- [ ] Prompt label is `"Where should weave keep your extensions?"` (PRD §8.2).
- [ ] `haveStore != ""` → returns it verbatim, prompt NEVER called (Test #1).
- [ ] `!isTTY` → returns the auto-detected default, prompt NEVER called (Tests #2,
      #3 — the `failIfCalled` sentinel enforces this).
- [ ] `isTTY` + cwd-with-extension → the auto-detected default IS cwd even on a
      TTY (Test #6).
- [ ] `isTTY` + empty prompt → returns the default (Test #4).
- [ ] `isTTY` + typed path → returns it VERBATIM (Test #5, no `filepath.Abs` in
      the core).
- [ ] Prompt read error propagates (Test #7).
- [ ] `resolveStore` supplies `os.Getwd`, `configpkg.DefaultStore`,
      `stdinIsTerminal`, a shared `bufio.NewReader(os.Stdin)` prompt closure, and
      absolutizes via `filepath.Abs`.
- [ ] `resolveStore`'s prompt closure writes to `os.Stderr` (NOT `os.Stdout`).
- [ ] Error wrappers use the `"weave init: …"` prefix.
- [ ] The 4 functions are NOT called in `run()` (wiring is S2).

### Code Quality Validation

- [ ] `internal/config` imported with the `configpkg` alias (local `config` struct
      conflict resolved).
- [ ] `bufio` imported as the first stdlib entry; `configpkg` alphabetical before
      `internal/discover`.
- [ ] `stdinIsTerminal` is a plain FUNCTION (not a `var`); the test seam is
      chooseStore's isTTY PARAMETER.
- [ ] The 4 functions appended AFTER `extensionPath` (current last fn), each with
      a doc comment citing PRD §8.2 and noting "WIRED BY S2's runInit".
- [ ] No `parseArgs` change; no `run()` dispatch change; no `internal/*` change.
- [ ] No `expandHome` added (out of scope — S2/future).

### Documentation & Deployment

- [ ] Each function's doc comment cites PRD §8.2 and explains its role in the
      store-selection decision.
- [ ] `resolveStore`'s doc comment notes the prompt-to-stderr choice and why
      (init stdout = bare store path; `$(weave init)` capture safety).
- [ ] No README/docs changes (init behavior is documented in README §3/§7 in the
      final M6.T4/M6.T5 doc sweep — this task's item desc says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't import `internal/config` WITHOUT the `configpkg` alias — main.go:77's
  `type config struct` collides with the package name → compile error. Use the
  alias (skilldozer's identical solution).
- ❌ Don't write the prompt to `os.Stdout` — skilldozer's shipped `resolveStore`
  binds it to `os.Stderr` so init's stdout stays the bare store path. The early
  research docs (external_deps.md §4, verified_facts.md §3) sketch os.Stdout; the
  SHIPPED CODE wins. `readPrompt` takes a generic `io.Writer` so the choice lives
  in `resolveStore`'s closure.
- ❌ Don't put `filepath.Abs` inside `chooseStore` — the core returns the chosen
  string VERBATIM so unit tests assert exact strings. Absolutize only in
  `resolveStore`. (Test #5 pins this: prompt "/custom" ⇒ "/custom", not
  filepath.Abs("/custom").)
- ❌ Don't make `stdinIsTerminal` a package `var` — the test seam is chooseStore's
  `isTTY` PARAMETER (injected per-call), so there is nothing to override
  globally. (Contrast the `isTerminal` VAR for stdout color, which IS a var
  because run() has no isTTY param.)
- ❌ Don't consult `HasExtensionEntry` only inside the `!isTTY` branch — the
  auto-detect (step 2) must run BEFORE the TTY split so the cwd default is the
  default on BOTH branches. Test #6 guards this.
- ❌ Don't add `expandHome` — it is out of scope (skilldozer added it in a LATER
  subtask; the item pins resolveStore as "filepath.Abs" only).
- ❌ Don't add the `if c.init { … }` dispatch to `run()` — that is S2. The 4
  functions are defined but UNCALLED here (Go allows unused package-level fns).
- ❌ Don't re-append `"weave/extensions"` to the DefaultStore result — the config
  package already appends it. `configpkg.DefaultStore()` returns the full path.
- ❌ Don't use `bufio.Scanner` for the interactive read — use
  `bufio.Reader.ReadString('\n')`, which returns `(line, io.EOF)` on no-newline
  EOF (a bare EOF with empty text = "accept default", not an error). This is what
  makes `weave init < /dev/null` non-hanging.
- ❌ Don't modify `parseArgs`, `run()`'s dispatch ladder, any `internal/*`
  package, `go.mod`, or PRD.md — this task is 4 functions + tests only.

---

**Confidence Score: 9/10** for one-pass success. The 4 functions are a
near-verbatim port of skilldozer's shipped, tested `main.go:808-960` (read in
full), and the 7 tests are a near-verbatim port of skilldozer's
`main_test.go:2340-2435`. The four weave-specific deltas (HasSkillMD→
HasExtensionEntry; prompt label noun-swap; error prefix; import path) are each
pinned to an exact location and verified against the live weave codebase. The two
most subtle points — the `configpkg` alias (local `config` struct conflict) and
the prompt-to-`os.Stderr` choice (resolving the research-docs-vs-shipped-code
discrepancy in favor of shipped code) — are both flagged as CRITICAL gotchas with
their compile-time / capture-safety rationale. The consumed APIs
(`extdir.HasExtensionEntry` L331, `configpkg.DefaultStore` L158) are confirmed
exported. The ONE residual risk is the parallel-sibling import-block merge with
P1.M4.T3.S1 (which adds `internal/check`/`internal/search` to the same block) —
mitigated by the explicit alphabetical final-order note and by `go build` catching
any text-level conflict immediately. No dependency on an unlanded deliverable:
S2 consumes this task's output but this task depends only on landed M1 work.
