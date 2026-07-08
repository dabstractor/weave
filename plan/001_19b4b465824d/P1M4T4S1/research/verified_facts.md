# Verified Facts — P1.M4.T4.S1 (chooseStore core + TTY-gated prompt)

Researched against the LIVE weave codebase (`main.go`, `main_test.go`,
`internal/config/config.go`, `internal/extdir/extdir.go` all read in full) AND
the skilldozer reference implementation (`/home/dustin/projects/skilldozer/main.go`
read in full, including the exact `stdinIsTerminal`/`readPrompt`/`chooseStore`/
`resolveStore` bodies at lines 808-960, plus skilldozer's `chooseStore` tests at
`main_test.go:2340-2435`). skilldozer's own prior research
(`plan/002_38acb6d28a6a/P1M2T2S1/research/verified_facts.md`) was read in full —
it is the exact-same task shape, ported skill→extension.

The parallel sibling **P1.M4.T3.S1** (`--search` + `check` dispatch) is being
implemented concurrently. Its PRP was read as a contract: it touches the MIDDLE of
`run()` (the dispatch ladder) and adds 2 imports (`internal/check`,
`internal/search`). This subtask touches the END of `main.go` (4 appended
functions) and the END of `main_test.go` (8 appended tests), and adds 2 DIFFERENT
imports (`bufio`, `internal/config`). **No overlap.**

---

## §1. What P1.M4.T4.S1 IS — exact scope (4 functions, ported from skilldozer)

A near-VERBATIM port of skilldozer's `main.go` store-selection quartet, with the
noun swap skill→extension. The 4 functions (signatures pinned by the item
description, identical to skilldozer):

```go
func stdinIsTerminal() bool                                               // stat os.Stdin, check ModeCharDevice
func readPrompt(r *bufio.Reader, w io.Writer, label, def string) (string, error)  // "%s [%s]: ", ReadString('\n'), empty/EOF⇒def
func chooseStore(haveStore, cwd string, isTTY bool, defaultStore string, prompt func(label, def string) (string, error)) (string, error)
func resolveStore(haveStore string) (string, error)                       // supplies real deps, absolutizes via filepath.Abs
```

**Scope boundary (do NOT cross):**
- S1 does NOT add `if c.init { … }` to `run()` — that is **P1.M4.T4.S2**.
- S1 does NOT mkdir/seed/write-config — that is **S2's `setupStore`**.
- S1 does NOT add `expandHome` — skilldozer added that in a LATER subtask
  (P1.M2.T3.S1) and wired it into `runInit` (S3). The item description for THIS
  task pins `resolveStore` as "return chooseStore result absolutized via
  filepath.Abs" — NO `expandHome`. Do not invent it.
- S1 does NOT touch `parseArgs` (already captures `init`/`--store`/`initStore`
  since M1.T4.S1) or any `internal/*` package.

After S1: the 4 functions EXIST and are unit-tested, but `resolveStore` is
**UNCALLED** (Go allows unused package-level functions; `go build` is fine). S2
wires it into `runInit`.

---

## §2. The 4 weave-specific DELTAS from skilldozer (all mechanical noun-swap)

| # | skilldozer | weave | where |
|---|-----------|-------|-------|
| 1 | `skillsdir.HasSkillMD(cwd)` | `extdir.HasExtensionEntry(cwd)` | chooseStore auto-detect (step 2) |
| 2 | `"Where should skilldozer keep your skills?"` | `"Where should weave keep your extensions?"` | chooseStore prompt label (matches PRD §8.2 VERBATIM: `Where should weave keep your extensions? [<default>]`) |
| 3 | error prefix `"skilldozer init:"` | `"weave init:"` | resolveStore error wrapping (cwd/absolutize) |
| 4 | `configpkg "github.com/dabstractor/skilldozer/internal/config"` | `configpkg "github.com/dabstractor/weave/internal/config"` | import block |

Everything else is IDENTICAL to skilldozer's shipped code.

---

## §3. CRITICAL: the `configpkg` import alias is REQUIRED (naming conflict)

weave's `main.go:77` declares `type config struct { … }` (the parsed-CLI struct).
This subtask must call `config.DefaultStore()` from `internal/config` (package
name `config`). A bare import `"github.com/dabstractor/weave/internal/config"`
makes the identifier `config` refer to BOTH the package and the local struct →
**compile error `config redeclared in this block`**.

skilldozer hit the SAME conflict and solved it with the alias `configpkg`. weave
MUST use the identical alias:
```go
configpkg "github.com/dabstractor/weave/internal/config"
```
Then `configpkg.DefaultStore()`. (Verified: weave does NOT currently import
`internal/config` anywhere — confirmed `grep` found zero hits in main.go /
main_test.go before this task.)

---

## §4. CRITICAL GOTCHA: the prompt writes to os.Stderr (NOT os.Stdout)

**Discrepancy in the source material — the shipped code wins:**
- skilldozer's early research (`external_deps.md §4` line 290, and the prior
  `verified_facts.md §3`) sketches `readPrompt(r, os.Stdout, …)`.
- skilldozer's ACTUAL SHIPPED `resolveStore` (main.go:947) uses `os.Stderr`:
  ```go
  prompt := func(label, def string) (string, error) {
      return readPrompt(r, os.Stderr, label, def)   // <-- STDERR
  }
  ```
  with the comment: *"Prompt dialog goes to stderr, not stdout. PRD §6.1 pins
  init's stdout to exactly the configured store path; the interactive 'Where
  should skilldozer keep your skills? [default]:' line is user-facing prose, not
  result data, so a caller doing store='$(skilldozer init)' must not capture it."*

**weave MUST follow the shipped code: prompt → os.Stderr.** The rationale is
load-bearing for `$(weave init)` capture safety (PRD §6.4 spirit). The
`readPrompt` function itself still takes a generic `w io.Writer` (so it is
unit-testable with a `*bytes.Buffer`); it is `resolveStore`'s prompt CLOSURE that
binds the real writer to `os.Stderr`.

`stdinIsTerminal` (the TTY gate) reads `os.Stdin` — a DIFFERENT stream. It is a
plain FUNCTION (not a `var`): the contract's test seam is `chooseStore`'s `isTTY`
PARAMETER, not a global override. Do NOT make `stdinIsTerminal` a `var` (contrast
with the package-level `isTerminal` `var` used for color gating, which IS a var
because `run()` has no isTTY param).

---

## §5. The chooseStore 4-step decision (logic → the 5 OUTPUT test cases)

`chooseStore(haveStore, cwd, isTTY, defaultStore, prompt)`:

| Step | Condition | Action | OUTPUT test case |
|------|-----------|--------|------------------|
| 1 | `haveStore != ""` | `return haveStore, nil` (prompt NEVER called) | #1: `init --store /tmp/x` ⇒ /tmp/x |
| 2 | (haveStore=="") auto-detect | `def := defaultStore; if extdir.HasExtensionEntry(cwd) { def = cwd }` | feeds #2/#3/#4 |
| 4 | `!isTTY` (and no haveStore) | `return def, nil` (prompt NEVER called) | #2: cwd+entry,non-TTY⇒cwd; #3: cwd-empty,non-TTY⇒defaultStore |
| 3 | `isTTY` | `choice, err := prompt(label, def)`; empty⇒def, else choice; err propagates | #4: prompt""⇒default; #5: prompt"/custom"⇒/custom |

**VERBATIM vs ABS:** `chooseStore` returns the chosen string VERBATIM (cwd/default
as-passed, or the user's typed string). The I/O wrapper `resolveStore` applies
`filepath.Abs` after. This keeps `chooseStore` a pure decision fn so the unit
tests assert exact strings (e.g. `prompt "/custom"` ⇒ `"/custom"`, NOT
`filepath.Abs("/custom")`).

**Ordering note:** skilldozer implements step 4 (`!isTTY`) BEFORE step 3 (`isTTY`)
in the function body — the auto-detect (step 2) runs first, THEN the TTY branch.
Both produce the same result; weave ports skilldozer's exact ordering for fidelity.

---

## §6. Consumed APIs — ALL verified present in the live weave codebase

- `extdir.HasExtensionEntry(dir string) bool` — `internal/extdir/extdir.go:331`.
  Walks `dir` for ANY §7.1 entry (*.ts/*.js non-index, dir with index.ts/js, or
  package.json with a pi.extensions array) at ANY depth; early-exits on first hit.
  This is the weave §8.2 cwd-auto-detect predicate (skilldozer's HasSkillMD
  equivalent). EXPORTED (P1.M1.T3.S1 = Complete). ✓
- `configpkg.DefaultStore() (string, error)` — `internal/config/config.go:158`.
  Pure fn of env: `$XDG_DATA_HOME` (if set AND abs) ⇒
  `$XDG_DATA_HOME/weave/extensions`; else `~/.local/share/weave/extensions`. The
  config package ALREADY appends `weave/extensions` — do NOT add it again in
  main.go. Returns its error verbatim ($HOME unset). (P1.M1.T2.S1 = Complete). ✓
- `os.Getwd() (string, error)` — stdlib. On error, `resolveStore` returns a
  wrapped error (`weave init: resolve cwd: %w`) — cwd-unresolvable is a HARD fail
  for init (unlike the walk-up rule which silently misses).
- `filepath.Abs(string) (string, error)` — stdlib, imported (`path/filepath`).
  Absolutizes the chosen store. On a relative typed path like "myext" ⇒ cwd/myext.
- `bufio.NewReader(os.Stdin).ReadString('\n')` — stdlib, NEW import `"bufio"`.
  ONE shared reader in `resolveStore`, captured by the prompt closure (a fresh
  reader per prompt can swallow buffered bytes).

`io`, `fmt`, `os`, `path/filepath`, `strings` are ALREADY imported in main.go.

---

## §7. Test design — chooseStore is the unit-test surface; resolveStore is NOT unit-tested

skilldozer's tests (read in full at `main_test.go:2340-2435`) are the exact
template. 8 tests, ALL against `chooseStore` (fake prompt fn + injected isTTY +
cwd/defaultStore as params — NO `t.Chdir`, NO real stdin):

| # | Test name (weave) | Inputs | Asserts |
|---|-------------------|--------|---------|
| 1 | `TestChooseStoreExplicitOverrideNoPrompt` | haveStore="/tmp/x", isTTY=true | returns "/tmp/x"; prompt fn FAILS if called |
| 2 | `TestChooseStoreCwdDetectNonTTY` | cwd=dir-with-entry, isTTY=false | returns cwd; prompt fn FAILS if called |
| 3 | `TestChooseStoreNoExtNonTTYUsesDefault` | cwd=empty, isTTY=false | returns defaultStore; prompt fn FAILS if called |
| 4 | `TestChooseStoreTTYEmptyPromptAcceptsDefault` | cwd=empty, isTTY=true, prompt returns "" | returns defaultStore |
| 5 | `TestChooseStoreTTYTypedPathOverrides` | cwd=empty, isTTY=true, prompt returns "/custom" | returns "/custom" (VERBATIM, no Abs in core) |
| 6 | `TestChooseStoreCwdDetectIsAlsoTheTTYDefault` | cwd=dir-with-entry, isTTY=true, prompt checks def==cwd then returns "" | returns cwd; auto-detect default IS cwd even on TTY |
| 7 | `TestChooseStorePropagatesPromptError` | isTTY=true, prompt returns a sentinel error | chooseStore returns ("", err wrapping sentinel) |
| 8 | `TestReadPromptFormatsDefAndTrims` | (optional, on readPrompt itself) | confirms `"%s [%s]: "` format + TrimSpace + empty⇒def |

Helpers to add to main_test.go (mirror skilldozer's `mkdirWithSkillMD` →
`mkdirWithExtEntry`):
```go
// mkdirWithExtEntry makes a temp dir that extdir.HasExtensionEntry reports as a
// store (contains a §7.1 entry at depth): tmp/sub/index.ts. cwd is a PARAM to
// chooseStore, so no t.Chdir is needed.
func mkdirWithExtEntry(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    sub := filepath.Join(dir, "sub")
    if err := os.MkdirAll(sub, 0o755); err != nil { t.Fatalf("MkdirAll: %v", err) }
    if err := os.WriteFile(filepath.Join(sub, "index.ts"), []byte("export default function() {}\n"), 0o644); err != nil {
        t.Fatalf("WriteFile: %v", err)
    }
    return dir
}

// failIfCalled returns a prompt fn that fails the test if chooseStore invokes it.
func failIfCalled(t *testing.T) func(string, string) (string, error) {
    t.Helper()
    return func(label, def string) (string, error) {
        t.Errorf("chooseStore: prompt must not be called (label=%q)", label)
        return "", nil
    }
}
```

**There is NO unit test for `resolveStore`** (it touches os.Stdin/os.Getwd which
are process-globals). It is exercised by S2's `runInit` dispatch + the eventual
§13 acceptance suite. `stdinIsTerminal` is likewise not directly unit-tested
(the TTY bit is not controllable without a pty); its logic is covered indirectly
by chooseStore's isTTY-parameterized tests.

---

## §8. main.go imports — exactly 2 lines ADDED

Current weave import block (main.go:14-24):
```go
import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/dabstractor/weave/internal/discover"
    "github.com/dabstractor/weave/internal/extdir"
    "github.com/dabstractor/weave/internal/resolve"
    "github.com/dabstractor/weave/internal/ui"
)
```
ADD (2 lines, alphabetical within each group):
- `"bufio"` — stdlib, goes FIRST (bufio < fmt).
- `configpkg "github.com/dabstractor/weave/internal/config"` — aliased (§3),
  alphabetical: `config` < `discover` < `extdir` → goes ABOVE `internal/discover`.

**IMPORTANT:** the parallel sibling P1.M4.T3.S1 adds `internal/check` and
`internal/search` to the SAME import block. If S3 lands first, the block already
has check + search; insert `bufio` first and `configpkg config` between check and
discover (alphabetical: check < config < discover). If both edit the same lines,
resolve the merge so the final block is: bufio, fmt, io, os, path/filepath,
strings, then check, config, discover, extdir, resolve, search, ui. No semantic
conflict — just alphabetical ordering.

`go.mod`/`go.sum` UNCHANGED (`bufio` is stdlib; `internal/config` is part of THIS
module, already a resolved dependency of `internal/extdir`).

---

## §9. Placement in main.go

Append the 4 functions at the END of main.go, AFTER the current last function
`extensionPath` (main.go ends ~line 570 with `extensionPath`'s closing brace).
Order: `stdinIsTerminal`, `readPrompt`, `chooseStore`, `resolveStore` (the
definition order skilldozer uses, matching the dependency chain:
stdinIsTerminal→readPrompt are leaves; chooseStore uses HasExtensionEntry;
resolveStore uses all three). Each gets a doc comment citing PRD §8.2 and noting
it is wired by S2's `runInit`.

`go vet`/`go build` will pass with these functions UNUSED (Go only errors on
unused LOCALS and unused IMPORTS, not unused package-level functions). The
`bufio` and `configpkg` imports WILL be "used" (referenced by readPrompt/
resolveStore), so no "imported and not used" error.

---

## §10. The 5 OUTPUT test cases from the item description — mapped

The item's TEST list maps directly onto skilldozer's test matrix (§7):

1. "chooseStore with haveStore set → returns it, no prompt" → Test #1.
2. "chooseStore with isTTY=false → returns default, no prompt" → Test #3 (and #2
   for the cwd-detected default).
3. "chooseStore with isTTY=true and cwd containing extensions → default is cwd"
   → Test #6 (the auto-detect default IS cwd even on a TTY).
4. "chooseStore with prompt returning '' → returns default" → Test #4.
5. "chooseStore with prompt returning typed path → returns it" → Test #5.

All five are covered. Tests #2 and #7 (error propagation) and the prompt-not-called
guards are ADDITIONAL rigor that makes the implementation trustworthy.
