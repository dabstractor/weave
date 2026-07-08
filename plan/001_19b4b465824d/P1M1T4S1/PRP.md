# PRP — P1.M1.T4.S1: config struct + parseArgs (full flag matrix) + run() dispatch (--version, --path)

## Goal

**Feature Goal**: Create `main.go` (the `weave` binary entrypoint) by porting skilldozer's
`main.go` structure: the `version` var (ldflags-overridable), the `config` struct holding
the **complete** PRD §6.1/§6.2 flag matrix, a hand-rolled `parseArgs` that handles every
flag form (long, short, bundles, `=`-forms, value-taking flags, reserved subcommands,
positional tags, unknown-flag capture), the `expandShortBundle` two-phase helper, the
`isTerminal` predicate, and a `run()` dispatcher. **Only `--version` and `--path` are wired
in `run()` this milestone** — every other mode is a no-op (later milestones add dispatch
branches, NOT parser changes). This is the weave port of skilldozer's M1.T3 entrypoint per
`architecture/architecture_mapping.md §8`.

**Deliverable**: A NEW file `main.go` at the repo root (`/home/dustin/projects/weave/main.go`,
`package main`) and a NEW file `main_test.go` at the repo root (`/home/dustin/projects/weave/main_test.go`).
No other files change. The package compiles to a `weave` binary supporting `weave --version`
(prints `weave <version>\n`, exit 0) and `weave --path` (calls `extdir.Find`; on success
prints the dir to stdout + `(found via <src>)\n` to stderr, exit 0; on failure prints the
error to stderr, exit 1). `parseArgs` already understands the full flag matrix so P1.M2/M3/M4/M5
add dispatch logic in `run()`, never parser changes.

**Success Definition**:
- `go build ./...` exits 0 (main.go + internal/config + internal/extdir all compile together).
- `go vet ./...` exits 0 (clean — main.go included).
- `go test ./... -v` passes ALL tests (config + extdir + the new main.go tests).
- `go test -race ./...` passes (no data races).
- `weave --version` prints exactly `weave dev\n` (under `go test`/plain `go build`, no ldflags)
  to stdout, nothing to stderr, exit 0.
- `weave -v` prints `weave <version>\n` (short form works).
- `weave --path` with `weave_EXTENSIONS_DIR` set to an existing dir prints the cleaned dir
  to stdout and `(found via weave_EXTENSIONS_DIR)\n` to stderr, exit 0.
- `weave --path` in an unconfigured env (env unset + cwd in empty temp tree) prints nothing
  to stdout and `weave is not configured; run \`weave init\`` to stderr, exit 1.
- `--version` takes precedence over `--path` (version printed, `extdir.Find` never called,
  exit 0 even when the extensions dir is unresolvable).
- parseArgs correctly parses EVERY flag form: `--version`/`-v`, `--help`/`-h`, `--path`/`-p`,
  `--list`/`-l`, `--all`/`-a`, `--file`/`-f`, `--relative`, `--no-color`, `--search`/`-s`,
  `--store`, `check`, `init`, `=`-forms, short bundles (incl. `-vpl`, `-sfoo`, `-ls foo`),
  value-taking flags consuming the next token, positional tags, and unknown-flag capture.
- go.mod still has no `require` block; no `go.sum` exists; the only non-stdlib import is
  `github.com/dabstractor/weave/internal/extdir`.

## User Persona (if applicable)

**Target User**: CLI users running `weave` from a shell, and shell scripts using
`pi -e "$(weave <tag>)"` / `$(weave --path)`.

**Use Case**: `weave --version` to check the installed version; `weave --path` to see where
`weave` is looking for extensions (and via which discovery rule). Later milestones add tag
resolution, listing, search, validation, and init.

**Pain Points Addressed**: There is currently no `main.go` at all — `go build ./...` builds
only the `internal/` packages and produces no binary. This subtask delivers the first
runnable `weave` binary.

## Why

- **Closes P1.M1 (Foundation)**: M1.T1 (scaffolding), M1.T2 (config), M1.T3 (extdir) are
  complete or landing in parallel. M1.T4 is the entrypoint that consumes `extdir.Find()`
  (from M1.T3.S3) and produces the `weave` binary. Without it, none of the foundation is
  reachable from the command line.
- **The parser is built once, fully**: The item contract pins parseArgs to handle the ENTIRE
  §6.1/§6.2 flag matrix now, even though only `--version`/`--path` dispatch. This is the
  critical design decision — later milestones (M2 `--list`, M3 `<tag>`/`--file`/`--all`,
  M4 `--search`/`check`/`init`, M5 `--help`/precedence/exit-2) add **dispatch branches in
  `run()` only**, never parser changes. Building the full parser up front avoids 4 milestones
  of parser churn and keeps parseArgs testable in isolation from the very first pass.
- **ldflags `version` var is load-bearing for install.sh**: P1.M6.T2 (install.sh) runs
  `go build -ldflags "-X main.version=$(git describe --tags --always ...)"`. `version` MUST
  be a package-level `var` (not `const`) for `-X main.version=...` to work at link time.
  Defining it now, with the default `"dev"`, is what makes `go run`, plain `go build`, and
  the ldflags build all behave correctly.
- **`--path` stderr source label is the Issue-1 (QA) feature**: PRD §8.3's `Source.String()`
  labels tell users WHY a particular dir won — a typo'd `weave_EXTENSIONS_DIR` would otherwise
  silently fall through to sibling/walk-up. `run()`'s `--path` branch emits the label to
  stderr (never stdout, to keep the §13 `test "$(weave --path)" = ...` gate byte-clean).

## What

A NEW `main.go` (`package main`, repo root) containing EXACTLY these symbols, ported from
skilldozer's `main.go` per `architecture/architecture_mapping.md §8`:

1. **Package doc comment** — adapted (weave/extensions/`pi -e`/milestone refs to M2/M3/M4/M5).
2. **Imports** — `fmt`, `io`, `os`, `strings`, and `github.com/dabstractor/weave/internal/extdir`.
   (NOT skilldozer's full import set — only what `--version`/`--path` dispatch + the full
   parser need. See Known Gotchas: Import minimization.)
3. **`var version = "dev"`** — package-level string var, NOT const (ldflags requirement).
4. **`var isTerminal = func(w io.Writer) bool`** — type-asserts `*os.File`, checks
   `ModeCharDevice`. Item-contract-pinned exact form.
5. **`func main()`** — `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`. Verbatim.
6. **`type config struct`** — ALL fields: `version, help, path, list, all, file, relative,
   noColor, searchMode, searchQ, check, init, initStore string, tags []string, unknownFlag string`.
   Verbatim from skilldozer (doc comments adapted).
7. **`func parseArgs(args []string) config`** — hand-rolled index-based loop. Full flag matrix
   (see Implementation Tasks for the exact switch). Verbatim from skilldozer.
8. **`func expandShortBundle(c *config, a string, args []string, i int) (consumeNext, ok bool)`**
   — two-phase validate-then-commit. Verbatim from skilldozer.
9. **`func run(args []string, stdout, stderr io.Writer) int`** — TRIMMED dispatch. ONLY
   `--version` and `--path` are active; everything else is a no-op (returns 0). NO `--help`,
   NO exclusivity check, NO unknown-flag→exit-2, NO no-args-usage — those are M5.T1.S1.

A NEW `main_test.go` (`package main`, repo root) porting the skilldozer M1.T3 tests for
parseArgs + run's --version/--path, with the documented renames:
- parseArgs matrix tests (version/path/any-order/unknown-flag/list/no-color short+long),
- run --version (long + short + precedence over --path),
- run --path success (env rule 1, deterministic), short flag, source-label reporting,
  and the ErrNotFound failure path (stdout empty, stderr has the one-line fix).

### Success Criteria

- [ ] `main.go` exists at repo root with `package main` and the 9 symbols listed above.
- [ ] `version` is declared `var version = "dev"` (NOT const), with the ldflags build-command
      documented in its doc comment.
- [ ] `config` struct has ALL 14 fields (version, help, path, list, all, file, relative,
      noColor, searchMode, searchQ, check, init, initStore, tags, unknownFlag) in skilldozer's
      order with the documented types.
- [ ] `parseArgs` handles: long forms (--version/--help/--path/--list/--all/--file/--relative/
      --no-color/--search/--store), short forms (-v/-h/-p/-l/-a/-f/-s), `=`-forms (--flag=value),
      short bundles (-vpl, -sfoo, -ls foo), value-taking flags consuming the next token
      (--search <q>, --store <dir>), reserved subcommands (check, init; init captures a
      following positional <dir>), positional tags (non-dashed → c.tags), and unknown dashed
      flags (first captured into c.unknownFlag).
- [ ] `expandShortBundle` implements the two-phase validate-then-commit (Phase 1 validates
      bool chars + finds 's'; Phase 2 commits; unknown char rejects the whole bundle with
      nothing applied).
- [ ] `run` dispatches `--version` (prints `weave <version>\n` to stdout, return 0) and
      `--path` (Find; on success dir→stdout + `(found via <src>)`→stderr, return 0; on err
      err→stderr, return 1); all other parsed modes are no-ops (return 0).
- [ ] `--version` precedence over `--path`: with both flags and an unresolvable dir, version
      is printed, Find is never called, exit 0.
- [ ] `main.go` imports ONLY `fmt`, `io`, `os`, `strings`, `github.com/dabstractor/weave/internal/extdir`.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` all pass.
- [ ] go.mod has no `require` block; no go.sum exists.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** This PRP names the exact source-of-truth file to port
(skilldozer's `main.go`, with line numbers for each symbol), the exact symbols in scope (with
verbatim-PORT vs TRIM-vs-DEFER classification), the exact import set, the exact `run()`
dispatch branches that are active vs no-op, and the exact test patterns to mirror (test names
+ the env-var-driven --path approach). The S3 PRP (parallel) establishes `extdir.Find()` and
`extdir.ErrNotFound` and `Source.String()` — the exact symbols `run()`'s `--path` branch calls.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for §6.1/§6.2/§6.3/§6.4 (the CLI contract this milestone's --version
       and --path implement), and §5 (the repo layout that puts main.go at the root).
  critical: §6.1 `weave --version` prints `weave <version>` (single line), exit 0; `weave --path`
            prints the absolute path of the resolved extensions dir, exit 0 (1 if unresolvable).
            §6.4 error semantics: unconfigured ⇒ stderr `weave is not configured; run \`weave init\``,
            exit 1, NOTHING on stdout (so `pi -e "$(weave x)"` fails loudly). §6.3 `--help`/`--version`
            take precedence over everything else. §5: `main.go` at repo root, `package main`.

- file: /home/dustin/projects/skilldozer/main.go
  why: PRIMARY pattern to PORT. This subtask ports the version var (lines 43-52), isTerminal
       (109-119), main (121-123), config struct (128-151), parseArgs (153-324), expandShortBundle
       (326-406), and the --version + --path branches of run (451-499). DEFER usageText/usage
       (54-99 → M5.T1.S1), exclusivityError (686-746 → M5.T1.S1), skillPath (747-770 → M3),
       runInit/chooseStore/resolveStore/setupStore (822-1050 → M4.T4). Drop the discover/resolve/
       ui/check/search/skillsdir/configpkg/bufio/filepath imports (not needed for --version/--path);
       KEEP strings (parseArgs/expandShortBundle use it).
  pattern: |
    // version — ldflags-overridable. MUST be var, not const.
    var version = "dev"
    // isTerminal — type-assert *os.File, ModeCharDevice. EXACT form.
    var isTerminal = func(w io.Writer) bool {
        f, ok := w.(*os.File)
        if !ok { return false }
        fi, err := f.Stat()
        if err != nil { return false }
        return fi.Mode()&os.ModeCharDevice != 0
    }
    func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
    // config — ALL §6.1/§6.2 fields. Port skilldozer's struct verbatim.
    type config struct {
        version     bool
        help        bool
        path        bool
        list        bool
        all         bool
        file        bool
        relative    bool
        noColor     bool
        searchMode  bool
        searchQ     string
        check       bool
        init        bool
        initStore   string
        tags        []string
        unknownFlag string
    }
    // run — TRIMMED dispatch. Only --version and --path active; rest no-op.
    func run(args []string, stdout, stderr io.Writer) int {
        c := parseArgs(args)
        if c.version {
            fmt.Fprintf(stdout, "weave %s\n", version)
            return 0
        }
        if c.path {
            dir, src, err := extdir.Find()
            if err != nil {
                fmt.Fprintln(stderr, err) // ErrNotFound one-line fix, verbatim
                return 1
            }
            fmt.Fprintln(stdout, dir)                  // byte-exact dir + newline
            fmt.Fprintf(stderr, "(found via %s)\n", src) // Issue-1 source label
            return 0
        }
        // All other modes are no-ops for now (M2/M3/M4/M5 add dispatch branches here).
        return 0
    }
  gotcha: The `--version` branch MUST come BEFORE `--path` in run() (precedence: PRD §6.3
          "--version takes precedence over everything else except --help" — help is M5, not
          here). The `--path` branch prints the dir to stdout and the source label to stderr
          (NEVER the label to stdout — it would break the §13 `test "$(weave --path)" = ...`
          gate). `fmt.Fprintln(stderr, err)` on an error value prints err.Error()+newline —
          this is the verbatim skilldozer pattern and produces the exact ErrNotFound bytes.

- file: /home/dustin/projects/skilldozer/main_test.go
  why: PRIMARY test pattern to PORT. Port these tests verbatim with renames
       ("skilldozer"→"weave", "SKILLDOZER_SKILLS_DIR"→"weave_EXTENSIONS_DIR",
       "skillsdir"→"extdir"):
         parseArgs: TestParseArgsEmpty, TestParseArgsVersionLong, TestParseArgsVersionShort,
                    TestParseArgsPathLong, TestParseArgsPathShort, TestParseArgsAnyOrderBothForms,
                    TestParseArgsUnknownFlagCaptured, TestParseArgsListLong, TestParseArgsListShort,
                    TestParseArgsNoColor (and the bundle/=/value-taking tests if present).
         run: TestRunVersionPrintsWeaveVersion (renamed), TestRunVersionShortFlag,
              TestRunPathSuccess, TestRunPathShortFlag, TestRunPathReportsSourceLabel,
              TestRunPathFailureErrNotFound, TestRunVersionPrecedenceOverPath.
       NOTE: the --path SUCCESS tests use the ENV-VAR rule (rule 1, deterministic) via
       t.Setenv("weave_EXTENSIONS_DIR", dir) — NOT a sibling-of-binary build. This mirrors
       skilldozer exactly. The item contract's "sibling-of-binary rule" phrase describes
       production behavior; the TEST uses env (rule 1) for hermetic determinism (skilldozer's
       own comment: "The env case is deterministic; sibling/walk-up are covered by extdir's
       TestSourceString.").
  pattern: |
    func TestRunVersionPrintsWeaveVersion(t *testing.T) {
        var out, errOut bytes.Buffer
        code := run([]string{"--version"}, &out, &errOut)
        if code != 0 { t.Fatalf("run(--version): code=%d; want 0", code) }
        want := "weave " + version + "\n" // version == "dev" under go test
        if got := out.String(); got != want {
            t.Errorf("run(--version) stdout=%q; want %q", got, want)
        }
        if errOut.Len() != 0 {
            t.Errorf("run(--version) stderr=%q; want empty", errOut.String())
        }
    }
    func TestRunPathSuccess(t *testing.T) {
        dir := t.TempDir()
        t.Setenv("weave_EXTENSIONS_DIR", dir) // rule 1 wins deterministically
        var out, errOut bytes.Buffer
        code := run([]string{"--path"}, &out, &errOut)
        if code != 0 { t.Fatalf("run(--path) success: code=%d; want 0", code) }
        want := filepath.Clean(dir) + "\n"
        if got := out.String(); got != want {
            t.Errorf("run(--path) stdout=%q; want %q", got, want)
        }
        if got, want := errOut.String(), "(found via weave_EXTENSIONS_DIR)\n"; got != want {
            t.Errorf("run(--path) stderr=%q; want %q", got, want)
        }
    }
    func TestRunPathFailureErrNotFound(t *testing.T) {
        unsetExtEnv(t)         // neutralize weave_EXTENSIONS_DIR
        t.Setenv("weave_CONFIG", filepath.Join(t.TempDir(), "no-config.yaml")) // neutralize rule 2
        t.Chdir(t.TempDir())   // empty tree -> rules 3/4 miss
        var out, errOut bytes.Buffer
        code := run([]string{"--path"}, &out, &errOut)
        if code != 1 { t.Fatalf("run(--path) failure: code=%d; want 1", code) }
        if out.Len() != 0 {
            t.Errorf("run(--path) stdout=%q; want EMPTY (§6.4)", out.String())
        }
        for _, want := range []string{"run", "weave init"} {
            if !strings.Contains(errOut.String(), want) {
                t.Errorf("run(--path) stderr=%q; missing %q", errOut.String(), want)
            }
        }
    }
    func TestRunVersionPrecedenceOverPath(t *testing.T) {
        unsetExtEnv(t)
        t.Setenv("weave_CONFIG", filepath.Join(t.TempDir(), "no-config.yaml"))
        t.Chdir(t.TempDir()) // would make --path fail, but --version wins first
        var out, errOut bytes.Buffer
        code := run([]string{"--path", "--version"}, &out, &errOut)
        if code != 0 { t.Fatalf("code=%d; want 0 (version precedence)", code) }
        if got, want := out.String(), "weave "+version+"\n"; got != want {
            t.Errorf("stdout=%q; want %q", got, want)
        }
    }
  gotcha: The --path FAILURE test MUST neutralize BOTH weave_EXTENSIONS_DIR (rule 1) AND
          weave_CONFIG (rule 2, LOWERCASE) AND t.Chdir to an empty temp tree (rules 3/4).
          Without neutralizing weave_CONFIG, a dev machine's real ~/.config/weave/config.yaml
          leaks a real dir and the test prints it instead of ErrNotFound. (S3's PRP specifies
          the same weave_CONFIG neutralization for extdir's Find tests.)

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M1T3S3/PRP.md  (S3, parallel)
  why: The CONTRACT for the extdir dependency. run()'s --path branch calls extdir.Find()
       and reads extdir.ErrNotFound / Source.String(). Treat S3 as implemented exactly as
       specified per the parallel_execution_context instruction.
  critical: extdir.Find() returns (dir string, src Source, err error); on miss err is
            extdir.ErrNotFound whose .Error() is `weave is not configured; run \`weave init\``.
            Source.String() returns "weave_EXTENSIONS_DIR" | "config file" | "sibling of binary"
            | "ancestor of cwd". run() prints the dir + the source label; nothing else.

- file: /home/dustin/projects/weave/internal/extdir/extdir.go  (S1+S2 COMPLETE; S3 parallel)
  why: The CONTRACT for the extdir package run() consumes. Confirms Find/ErrNotFound/Source/
       SourceEnv + the envVar const ("weave_EXTENSIONS_DIR", LOWERCASE). S3 adds the rest of
       Find (rules 2-4 + ErrNotFound).
  critical: The env var name is LOWERCASE "weave_EXTENSIONS_DIR" (extdir.go const envVar).
            Tests set/unset it case-sensitively. os.Setenv is case-sensitive on Linux.

- file: /home/dustin/projects/weave/internal/config/config.go  (P1.M1.T2.S1 COMPLETE)
  why: Indirect dependency — extdir.Find() calls config (via findConfig). main.go does NOT
       import config directly. Included only to confirm the configEnv const ("weave_CONFIG",
       LOWERCASE) that the --path failure test must neutralize for hermeticity.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §8 maps skilldozer main.go → weave main.go line by line. Pins the renames (binary name,
       env var names, exampleSkillTemplate→exampleExtensionTemplate, etc.) and the ADAPT
       notes (config struct "same fields, different binary name in usage text").
  critical: §8 lists the full set of main.go symbols to port (main/run/parseArgs/config/
            exclusivityError/extensionPath/isTerminal/stdinIsTerminal/readPrompt/chooseStore/
            resolveStore/setupStore/runInit). This subtask ports ONLY main/run/parseArgs/
            config/expandShortBundle/isTerminal/version — the rest are explicitly DEFERRED to
            later milestones (see the port-notes table in research/).

- file: /home/dustin/projects/weave/go.mod  (P1.M1.T1.S1)
  why: Confirms module `github.com/dabstractor/weave`, `go 1.25`. The `go 1.25` directive is
       what makes t.Chdir (Go 1.24+) available — main_test.go's --path failure test uses it.
  critical: main.go adds ONE import (`github.com/dabstractor/weave/internal/extdir`).
            go.mod gains NO require block; no go.sum.

- file: /home/dustin/projects/weave/.gitignore  (P1.M1.T1.S1)
  why: Confirms `/weave` is gitignored (the built binary). `go build -o weave .` produces it;
       it will NOT be committed. No change needed.
```

### Current Codebase tree

```bash
# After P1.M1.T1.S1 (Complete) + P1.M1.T2.S1 (Complete) + P1.M1.T3.S1+S2 (Complete) +
# P1.M1.T3.S3 (parallel). THIS subtask (M1.T4.S1) ADDS main.go + main_test.go.
$ cd /home/dustin/projects/weave && ls -A1
.git
.gitignore
LICENSE
PRD.md
go.mod                  # module github.com/dabstractor/weave / go 1.25 / no require
internal/
├── config/             # P1.M1.T2.S1 (Complete) — File/Load/Save/Path/DefaultStore
│   ├── config.go
│   └── config_test.go
└── extdir/             # P1.M1.T3.S1+S2 (Complete) + S3 (parallel)
    ├── extdir.go       # Find/ErrNotFound/Source/HasExtensionEntry (+ rules 1-4)
    └── extdir_test.go
plan/
# NOTE: NO main.go exists yet. `go build ./...` currently builds only internal/ (no binary).
```

### Desired Codebase tree with files to be added

```bash
weave/
├── main.go             # NEW (this subtask) — version/isTerminal/main/config/parseArgs/
│                       #                      expandShortBundle/run (dispatch --version/--path)
├── main_test.go        # NEW (this subtask) — parseArgs matrix + run --version/--path tests
├── internal/           # UNCHANGED
│   ├── config/
│   └── extdir/
├── go.mod              # UNCHANGED (no new require; main.go's extdir import is intra-module)
└── ...                 # everything else unchanged
# After `go build -o weave .`, the gitignored `/weave` binary appears (not committed).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: `version` MUST be `var version = "dev"`, NOT `const`. The ldflags flag
// `-X main.version=<value>` rewrites a package-scope string VAR at link time; it CANNOT
// override a const (compile error) or a function-local. Because main.go is `package main`,
// the linker symbol path is exactly `main.version` (NOT the module import path). P1.M6.T2's
// install.sh depends on this: `go build -ldflags "-X main.version=$(git describe ...)" -o weave .`.

// CRITICAL: run()'s --path branch prints the DIR to stdout and the SOURCE LABEL to stderr
// — NEVER the label to stdout. The PRD §13 acceptance gate `test "$(weave --path)" = ...`
// captures stdout only; the `(found via ...)` label on stdout would break it. This is the
// verbatim skilldozer "Issue 1 (QA)" convention.

// CRITICAL: run()'s --version branch prints `weave %s\n` (lowercase "weave", space, version,
// newline) — NOT "Weave", NOT "weave v%s", NOT "version %s". The §6.1 contract is literal:
// "weave <version>" on a single line. Pinned byte-exact by TestRunVersionPrintsWeaveVersion.

// CRITICAL: the --version branch MUST come BEFORE --path in run() (precedence). PRD §6.3:
// "--version takes precedence over everything else except --help." --help is NOT implemented
// here (M5.T1.S1), so --version is the highest-precedence dispatch. With both flags set and
// an unresolvable dir, version is printed and Find is never called, exit 0. Pinned by
// TestRunVersionPrecedenceOverPath.

// CRITICAL: parseArgs/expandShortBundle port VERBATIM from skilldozer — the flag tokens
// themselves do NOT change (--version stays --version; the §6 CLI contract is byte-identical
// to skilldozer's per PRD §6 header). The ONLY renames are in DOC COMMENTS: "skilldozer"→
// "weave", "skill"→"extension", "SKILL.md"→"entry file", `pi --skill`→`pi -e`. Do NOT change
// any case label, any i++ logic, or the two-phase structure of expandShortBundle.

// CRITICAL: expandShortBundle's two-phase (validate-then-commit) is load-bearing. Phase 1
// walks bool chars and finds 's'; an unknown char REJECTS the whole bundle (sets unknownFlag,
// commits NOTHING). If a leaked `version=true` from a partial "-vz" were committed before the
// unknown was detected, run() would print the version (exit 0) and mask the unknown-char
// error (M5 turns unknownFlag into exit 2). Do NOT collapse to a single-phase loop.

// CRITICAL: the env var names are LOWERCASE: "weave_EXTENSIONS_DIR" (extdir envVar) and
// "weave_CONFIG" (config configEnv). os.Setenv/os.LookupEnv are case-sensitive on Linux.
// A test that does t.Setenv("WEAVE_EXTENSIONS_DIR", ...) is a NO-OP against the real rule 1.

// CRITICAL: the --path FAILURE test is hermetic ONLY if it neutralizes rule 1 (env var),
// rule 2 (weave_CONFIG), AND t.Chdir to an empty temp tree (rules 3/4 miss). Without the
// weave_CONFIG neutralization, a dev machine's real ~/.config/weave/config.yaml leaks a real
// dir. The unsetExtEnv helper must neutralize BOTH. (S3's extdir tests have the same
// requirement; mirror its unsetSkillsEnv-equivalent.)

// CRITICAL (import minimization): main.go imports ONLY fmt, io, os, strings, internal/extdir.
// DROP skilldozer's bufio (readPrompt — M4), path/filepath (skillPath — M3), discover/resolve/
// ui/check/search (M2/M3/M4 dispatch), and configpkg (init — M4). BUT KEEP strings —
// parseArgs/expandShortBundle use strings.HasPrefix/Contains/IndexByte. If you leave a
// now-unused import, `go build` fails with "imported and not used".

// GOTCHA: `go vet` will flag `isTerminal` if it's declared but unused (Go allows unused
// package-level vars, so vet is fine — but be aware isTerminal is NOT called by this
// milestone's run() since --version/--path don't use color). It's declared now so later
// milestones (M2 --list color) can use it without re-touching main.go's top. This is correct
// and intended; Go does not error on unused package vars (only unused locals and imports).

// GOTCHA: run()'s "all other modes are no-ops" means `return 0` for any args that aren't
// --version/--path. This is intentional and temporary: M2 adds the --list branch, M3 adds
// <tag>/--file/--all, M4 adds --search/check/init, M5 adds --help/exclusivity/unknown-flag-2.
// Do NOT add a no-args→usage→exit-1 path now (that's M5.T1.S1); a bare `weave` with no args
// currently exits 0, which is acceptable for this milestone (it has no behavior to show).

// GOTCHA: t.Chdir (Go 1.24+; go.mod is 1.25) changes cwd for the test and restores on
// cleanup. The --path failure + precedence tests use it so Find() rules 3/4 miss. Do NOT
// call t.Parallel() on t.Chdir tests or env-mutating tests — cwd and env are process-global.

// GOTCHA (build dependency): main.go imports internal/extdir, whose Find() (S3) calls
// findConfig/findSibling (S2). All of S1/S2/S3 must have landed for `go build ./...` to
// succeed. If S3 has NOT landed, `go build .` fails with "undefined: extdir.Find" (or
// extdir.ErrNotFound). This is EXPECTED — T3 and T4 are a parallel pair that together
// complete M1. Do NOT stub Find/ErrNotFound in main.go to make it build in isolation.

// GOTCHA (boundaries): do NOT add usageText/usage (M5.T1.S1), exclusivityError (M5.T1.S1),
// skillPath/extensionPath (M3.T2.S1), runInit/chooseStore/resolveStore/setupStore (M4.T4),
// stdinIsTerminal/readPrompt (M4.T4), or any internal-package dispatch (M2+). This subtask's
// blast radius is main.go (new) + main_test.go (new); nothing else.
```

## Implementation Blueprint

### Data models and structure

One new struct (`config`), one new package-level var (`version`), one new package-level var
holding a func (`isTerminal`), and five new functions (`main`, `parseArgs`, `expandShortBundle`,
`run`). All in `package main` at the repo root. No new types beyond `config`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE main.go — package doc + imports + version var
  - CREATE /home/dustin/projects/weave/main.go with `package main`.
  - DOC COMMENT (file-level, above package): mirror skilldozer's — "Command weave resolves
    extension tags to on-disk extension paths. main.go is the entrypoint: it parses argv,
    applies PRD §6 precedence (--version/--help win over everything), and dispatches to the
    matching mode. Wired so far (grown milestone by milestone): --version/--path (M1.T4).
    Every other §6 flag is added by later milestones (M2 --list, M3 <tag>/--file/--all,
    M4 --search/check/init, M5 --help + exit codes). The arg parser is intentionally a small
    hand-rolled switch (not Go's flag package) so the full §6 matrix ... can be expressed
    cleanly."
  - IMPORTS (exact set, alphabetical): fmt, io, os, strings, then the internal import:
      "github.com/dabstractor/weave/internal/extdir"
  - ADD `var version = "dev"` with the full ldflags doc comment (port skilldozer lines 43-52
    verbatim, change "skilldozer"→"weave" and the build-command `-o skilldozer`→`-o weave`
    and `pi --skill`→`pi -e`). Emphasize: MUST be var not const; symbol path is main.version.
  - NAMING: version (package-level string var, lowercase — NOT exported; ldflags targets it
    by `main.version` regardless of case).
  - PLACEMENT: top of main.go, after imports, before everything else.

Task 2: CREATE main.go — isTerminal + main()
  - ADD `var isTerminal = func(w io.Writer) bool { ... }` — port skilldozer lines 109-119
    VERBATIM. Type-assert *os.File, f.Stat(), check ModeCharDevice bit. Doc comment notes:
    decides whether --list/--search emit ANSI color by default (PRD §6.2); a *bytes.Buffer
    (tests) / pipe / redirect yields false; package var so tests can override it; NOT safe
    for t.Parallel.
  - ADD `func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }` — verbatim from
    skilldozer line 121-123. Doc comment: thin wrapper; logic in run() for testability
    (tests call run() with *bytes.Buffer, never os.Exit).
  - NAMING: isTerminal (package var), main (func).
  - PLACEMENT: after version, before config.

Task 3: CREATE main.go — config struct
  - ADD `type config struct { ... }` — port skilldozer lines 128-151 VERBATIM (all 14 fields,
    same order, same types, same field-level doc comments adapted: skilldozer→weave,
    skill→extension, SKILL.md→entry file, `pi --skill`→`pi -e`). Fields:
      version bool; help bool; path bool; list bool; all bool; file bool; relative bool;
      noColor bool; searchMode bool; searchQ string; check bool; init bool; initStore string;
      tags []string; unknownFlag string.
  - STRUCT DOC COMMENT: "config holds the parsed CLI flags (PRD §6.1/§6.2 matrix). This
    subtask (P1.M1.T4.S1) lands the full matrix; only --version and --path are DISPATCHED
    in run() so far (M2/M3/M4/M5 add dispatch branches)."
  - NAMING: config (lowercase, package-internal — matches skilldozer).
  - PLACEMENT: after main(), before parseArgs.

Task 4: CREATE main.go — parseArgs (full flag matrix)
  - ADD `func parseArgs(args []string) config` — port skilldozer lines 153-324 VERBATIM
    (the index-based loop, the (a) long-with-'=' branch, the (b) short-bundle branch
    delegating to expandShortBundle, the main switch with ALL cases, and the default branch
    capturing positional tags vs unknown dashed flags). Doc comments adapted (binary name).
  - THE SWITCH CASES (exact, port verbatim): --version/-v, --help/-h, --path/-p, --list/-l,
    --all/-a, --file/-f, --relative, --no-color, --search/-s (value-taking: consume next via
    i++), check (reserved subcommand), --store (value-taking: consume next via i++), init
    (reserved subcommand: capture following positional <dir> via i++ if not dashed and not
    check/init). The '=' branch handles --flag=value for all of these. The default branch:
    dashed → unknownFlag (first only); non-dashed → c.tags.
  - NAMING: parseArgs (package-internal).
  - PLACEMENT: after config struct, before expandShortBundle.
  - GOTCHA: keep the i++ (not range) loop so value-taking flags can consume the next token.
    Keep every `continue` after each form-normalization branch.

Task 5: CREATE main.go — expandShortBundle (two-phase)
  - ADD `func expandShortBundle(c *config, a string, args []string, i int) (consumeNext, ok bool)`
    — port skilldozer lines 326-406 VERBATIM (Phase 1 validate-walk finding sIdx; Phase 2
    commit bool flags in [0, end]; handle the 's' remainder/embedded/next-token/no-value
    cases; unknown char → reject whole bundle, set unknownFlag, return false,true). Doc
    comment adapted.
  - THE BOOL SHORT SET is exactly {v, h, p, l, a, f} (NO short for --relative/--no-color/
    --store per §6.1/§6.2). 's' is the value-taking flag (--search). Any other char is unknown.
  - NAMING: expandShortBundle (package-internal).
  - PLACEMENT: immediately after parseArgs.

Task 6: CREATE main.go — run() (dispatch --version + --path only)
  - ADD `func run(args []string, stdout, stderr io.Writer) int` — TRIMMED dispatch. Port
    skilldozer's run() STRUCTURE but keep ONLY:
      (1) `c := parseArgs(args)`
      (2) if c.version → `fmt.Fprintf(stdout, "weave %s\n", version); return 0`
      (3) if c.path → `dir, src, err := extdir.Find();` on err `fmt.Fprintln(stderr, err);
          return 1`; else `fmt.Fprintln(stdout, dir); fmt.Fprintf(stderr, "(found via %s)\n",
          src); return 0`
      (4) `return 0` (all other modes are no-ops for now).
  - DOC COMMENT: "run is the testable dispatcher. It returns the process exit code so main()
    can call os.Exit(run(...)) without tests ever invoking os.Exit. stdout/stderr injected so
    tests capture output via *bytes.Buffer. This milestone (P1.M1.T4.S1) dispatches ONLY
    --version and --path; all other parsed modes are no-ops (M2/M3/M4/M5 add dispatch
    branches here — the parser is already complete). Precedence: --version over --path (PRD
    §6.3). --help/exclusivity/unknown-flag-exit-2 land in M5.T1.S1."
  - DO NOT port: the --help branch, the unknownFlag→exit-2 branch, the exclusivityError call,
    the runInit branch, the --list/--search/check/--all/<tag> branches, the no-args→usage
    branch. Those are M5/M2/M3/M4.
  - NAMING: run (package-internal, but called by main).
  - PLACEMENT: after expandShortBundle, last function in main.go.
  - GOTCHA: `fmt.Fprintln(stderr, err)` where err is an error value prints err.Error()+"\n".
    For extdir.ErrNotFound this yields exactly `weave is not configured; run \`weave init\`\n`.
    Do NOT do `fmt.Fprintln(stderr, err.Error())` or add a prefix — the verbatim pattern is
    `fmt.Fprintln(stderr, err)`.

Task 7: CREATE main_test.go — imports + unsetExtEnv helper
  - CREATE /home/dustin/projects/weave/main_test.go with `package main`.
  - IMPORTS: bytes, os, path/filepath, strings, testing. (NO extdir/config import — tests
    exercise run()/parseArgs as black boxes via env + t.Chdir.)
  - ADD `func unsetExtEnv(t *testing.T)` helper — neutralizes BOTH weave_EXTENSIONS_DIR
    (extdir rule 1) AND weave_CONFIG (extdir rule 2, lowercase). Mirror skilldozer's
    unsetSkillsEnv. Pattern:
      t.Helper()
      // rule 1: weave_EXTENSIONS_DIR
      t.Setenv("weave_EXTENSIONS_DIR", "") // t.Setenv("","") effectively unsets on LookupEnv miss
      // Actually use os.Unsetenv + restore to be safe; OR set to a ghost path. Simplest:
      //   point both at non-existent ghost paths in a temp dir.
    RECOMMENDED (deterministic, mirrors S3's approach): set BOTH env vars to ghost paths:
      ghostExt := filepath.Join(t.TempDir(), "no-ext")
      ghostCfg := filepath.Join(t.TempDir(), "no-config.yaml")
      t.Setenv("weave_EXTENSIONS_DIR", ghostExt)   // rule 1 misses (dir doesn't exist)
      t.Setenv("weave_CONFIG", ghostCfg)           // rule 2 misses (file doesn't exist) — LOWERCASE
    (t.Setenv restores the original on cleanup automatically — no manual tb.Cleanup needed.)
  - DOC COMMENT: "unsetExtEnv neutralizes the two env-based extdir rules (weave_EXTENSIONS_DIR
    and weave_CONFIG, both LOWERCASE) so Find() falls through to the cwd/sibling rules. Required
    for hermetic --path failure + precedence tests (a dev machine's real config would otherwise
    leak). t.Setenv restores on cleanup."
  - GOTCHA: t.Setenv requires the test NOT call t.Parallel() (env is process-global). All
    tests using unsetExtEnv omit t.Parallel().

Task 8: CREATE main_test.go — parseArgs matrix tests
  - PORT verbatim from skilldozer main_test.go with renames. The parseArgs tests (these call
    parseArgs directly, NOT run, so no env mutation needed — they CAN be t.Parallel but follow
    the file convention of omitting it):
      TestParseArgsEmpty              (parseArgs(nil) → all zero values)
      TestParseArgsVersionLong        (--version → c.version=true, c.path=false)
      TestParseArgsVersionShort       (-v)
      TestParseArgsPathLong           (--path → c.path=true, c.version=false)
      TestParseArgsPathShort          (-p)
      TestParseArgsAnyOrderBothForms  (-p --version → both true)
      TestParseArgsUnknownFlagCaptured(--bogus → c.unknownFlag=="--bogus"; a SECOND unknown
        does NOT overwrite the first)
      TestParseArgsListLong           (--list → c.list=true)
      TestParseArgsListShort          (-l)
      TestParseArgsNoColor            (--no-color → c.noColor=true)
    - ALSO port the bundle/=/value-taking parseArgs tests if skilldozer has them (search
      main_test.go for TestParseArgsBundle, TestParseArgsEqualsForm, TestParseArgsSearchValue,
      TestParseArgsShortBundleWithSearch, TestParseArgsStoreEquals, TestParseArgsInitPositional,
      TestParseArgsTags, TestParseArgsCheck, TestParseArgsInit). Port whichever exist — they
      encode the parser contract and pin behavior for later milestones.
  - ADAPT assertions: "skilldozer"→"weave" only where a string literal appears (parseArgs
    tests rarely have binary-name literals; mostly field-value assertions).

Task 9: CREATE main_test.go — run --version tests
  - PORT verbatim with renames:
      TestRunVersionPrintsWeaveVersion  (run(["--version"]) → code 0, stdout == "weave "+version+"\n",
        stderr empty. NOTE: version=="dev" under `go test` since no ldflags. Assert the EXACT
        bytes, not just a substring — pins the §6.1 contract.)
      TestRunVersionShortFlag           (run(["-v"]) → code 0, stdout HasPrefix "weave ",
        HasSuffix "\n".)
  - These do NOT mutate env (version dispatch doesn't call Find) — but follow file convention
    (no t.Parallel).

Task 10: CREATE main_test.go — run --path tests
  - PORT verbatim with renames (SKILLDOZER_SKILLS_DIR → weave_EXTENSIONS_DIR, "skilldozer init"
    → "weave init", "(found via SKILLDOZER_SKILLS_DIR)" → "(found via weave_EXTENSIONS_DIR)"):
      TestRunPathSuccess          (t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()); run(["--path"])
        → code 0, stdout == filepath.Clean(dir)+"\n", stderr == "(found via weave_EXTENSIONS_DIR)\n".
        The env rule wins deterministically; filepath.Abs cleans the dir.)
      TestRunPathShortFlag        (same with -p.)
      TestRunPathReportsSourceLabel (same as Success but asserts both stdout and stderr byte-exact
        — the Issue-1 source-label feature.)
      TestRunPathFailureErrNotFound (unsetExtEnv(t); t.Chdir(t.TempDir()); run(["--path"]) →
        code 1, stdout EMPTY (§6.4), stderr Contains "run" AND "weave init".)
      TestRunVersionPrecedenceOverPath (unsetExtEnv(t); t.Chdir(t.TempDir()); run(["--path",
        "--version"]) → code 0, stdout == "weave "+version+"\n", stderr empty. Proves --version
        wins and Find is never called.)
  - Each --path test that mutates env or cwd calls unsetExtEnv(t) FIRST and does NOT call
    t.Parallel().

Task 11: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...           # expect exit 0
    (REQUIRES S3 to have landed extdir.Find/ErrNotFound — if not, see GOTCHA above)
  - RUN: go build -o /tmp/weave-smoke . && /tmp/weave-smoke --version  # expect "weave dev"
  - RUN: go vet ./...                                              # expect exit 0, clean
  - RUN: go test ./... -v                                          # expect ALL PASS
    (config + extdir + main)
  - RUN: go test -race ./...                                       # expect no data races
  - RUN: grep -rn "yaml.v3\|gopkg.in" --include=*.go .             # expect nothing
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK         # expect OK (no require)
  - RUN: test ! -f go.sum && echo OK || echo FAIL                  # expect OK (no go.sum)
  - RUN: ! grep -q 'skilldozer\|SKILLDOZER\|SKILL.md\|HasSkillMD\|skillsdir' main.go \
      && echo "no skilldozer leftovers (correct)" || echo "FAIL: skilldozer leftover in main.go"
  - RUN: grep -q '"weave %s\\n"' main.go && echo "version format correct" \
      || echo "FAIL: --version format wrong"
  - RUN: grep -q '(found via %s)' main.go && echo "source label present" \
      || echo "FAIL: source label missing"
  - EXPECT: build clean, vet clean, all tests pass, no third-party import, no require block,
    no go.sum, no skilldozer/skillsdir leftovers, version format and source label present.
```

### Implementation Patterns & Key Details

```go
// version — MUST be var, not const. ldflags `-X main.version=...` targets it.
var version = "dev"

// isTerminal — verbatim from skilldozer. Type-asserts *os.File, ModeCharDevice.
var isTerminal = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// config — ALL §6.1/§6.2 fields. Port skilldozer's struct verbatim (doc comments adapted).
type config struct {
	version     bool   // --version / -v
	help        bool   // --help / -h           (dispatched in M5.T1.S1)
	path        bool   // --path / -p           (DISPATCHED HERE)
	list        bool   // --list / -l           (M2.T5.S1)
	all         bool   // --all / -a            (M3.T2.S1)
	file        bool   // --file / -f           (M3.T2.S1)
	relative    bool   // --relative            (M3.T2.S1)
	noColor     bool   // --no-color            (M2.T5.S1)
	searchMode  bool   // --search <q> / -s <q> (M4.T3.S1)
	searchQ     string // the --search query value
	check       bool   // `weave check`         (M4.T3.S1)
	init        bool   // `weave init [<dir>]`  (M4.T4.S2)
	initStore   string // non-interactive store path: `init <dir>` or `--store <dir>`
	tags        []string // positional <tag> args (M3.T2.S1)
	unknownFlag string // first unknown dashed token (M5.T1.S1 → exit 2)
}

// run — TRIMMED dispatch. Only --version and --path active; rest no-op (return 0).
// Precedence: --version over --path (PRD §6.3). --help/exclusivity/exit-2 are M5.T1.S1.
func run(args []string, stdout, stderr io.Writer) int {
	c := parseArgs(args)

	// 1) --version (PRD §6.3: precedes everything except --help, which is M5).
	if c.version {
		fmt.Fprintf(stdout, "weave %s\n", version)
		return 0
	}

	// 2) --path (PRD §6.1/§6.4). Find() locates the dir; on miss ErrNotFound is printed
	//    verbatim to stderr (the one-line fix), stdout stays EMPTY (§6.4 $(...) safety),
	//    exit 1. On success: dir→stdout (byte-exact, §13 gate), source label→stderr (Issue 1).
	if c.path {
		dir, src, err := extdir.Find()
		if err != nil {
			fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline
			return 1
		}
		fmt.Fprintln(stdout, dir)
		fmt.Fprintf(stderr, "(found via %s)\n", src)
		return 0
	}

	// 3) All other parsed modes are no-ops for now. M2 adds --list, M3 adds <tag>/--file/
	//    --all, M4 adds --search/check/init, M5 adds --help/exclusivity/unknown-flag-2.
	//    The parser is ALREADY complete; later milestones add dispatch branches HERE only.
	return 0
}

// unsetExtEnv (test helper) — neutralize rule 1 (weave_EXTENSIONS_DIR) AND rule 2
// (weave_CONFIG, LOWERCASE) so Find() falls through to cwd/sibling. t.Setenv restores
// on cleanup. Do NOT call t.Parallel() (env is process-global).
func unsetExtEnv(t *testing.T) {
	t.Helper()
	t.Setenv("weave_EXTENSIONS_DIR", filepath.Join(t.TempDir(), "no-ext"))
	t.Setenv("weave_CONFIG", filepath.Join(t.TempDir(), "no-config.yaml")) // LOWERCASE
}

// GOTCHA (version format): `weave %s\n` — lowercase "weave", single space, version, newline.
// NOT "Weave", NOT "weave v%s". Pinned byte-exact by TestRunVersionPrintsWeaveVersion.

// GOTCHA (source label destination): the `(found via %s)` line goes to STDERR, never stdout.
// stdout gets ONLY the dir + newline (§13 `test "$(weave --path)" = ...` gate).

// GOTCHA (error printing): `fmt.Fprintln(stderr, err)` on an error value prints err.Error()
// + "\n". For extdir.ErrNotFound this is exactly the §6.4/§8.2 one-line fix. Do NOT prefix.
```

### Integration Points

```yaml
DATABASE:
  - none. main.go calls extdir.Find() (filesystem + env + config, all in extdir/config).

CONFIG:
  - main.go does NOT touch config directly. extdir.Find() → findConfig() → config.Path()/
    config.Load(). The --path failure test neutralizes weave_CONFIG (lowercase) so a dev
    machine's real config doesn't leak.

ROUTES / API:
  - none. weave is a CLI; main.go is the entrypoint. Consumed by:
      * the shell: `weave --version`, `weave --path`, `pi -e "$(weave <tag>)"` (later).
      * install.sh (M6.T2): `go build -ldflags "-X main.version=$(git describe ...)" -o weave .`
        — the version var MUST be a package-level var for this to work.

MODULE:
  - main.go adds ONE import: "github.com/dabstractor/weave/internal/extdir". go.mod gains NO
    require block (intra-module); no go.sum. The other imports (fmt, io, os, strings) are stdlib.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave

# Format check.
gofmt -l main.go main_test.go
# EXPECTED: no output. If a file is listed, run `gofmt -w main.go main_test.go` and re-check.

# go vet (whole repo — main.go included).
go vet ./...
# EXPECTED: exit 0, no output.

# No third-party deps leaked in (the only non-stdlib import should be internal/extdir).
grep -rn "yaml.v3\|gopkg.in" --include=*.go . || echo "no third-party import (correct)"
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
# EXPECTED: "no third-party import (correct)" and "no require block (correct)".

# No skilldozer/skillsdir leftovers (the single most likely copy-paste bug).
! grep -q 'skilldozer\|SKILLDOZER\|SKILL.md\|HasSkillMD\|skillsdir' main.go \
  && echo "no skilldozer leftovers (correct)" || echo "FAIL: skilldozer leftover in main.go"
# EXPECTED: "no skilldozer leftovers (correct)".

# --version format pinned byte-exact (PRD §6.1: "weave <version>").
grep -q '"weave %s\\n"' main.go && echo "version format correct (correct)" \
  || echo "FAIL: --version format wrong"
# EXPECTED: "version format correct (correct)".

# Source label present (Issue-1 QA feature).
grep -q '(found via %s)' main.go && echo "source label present (correct)" \
  || echo "FAIL: source label missing"
# EXPECTED: "source label present (correct)".

# version is a var, not a const (ldflags requirement).
grep -q '^var version = "dev"' main.go && echo "version is var (correct)" \
  || echo "FAIL: version must be 'var version = \"dev\"'"
# EXPECTED: "version is var (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the main package tests verbosely.
go test -run 'TestParseArgs|TestRun' -v .
# EXPECTED: ALL tests pass. Pay special attention to:
#   - TestRunVersionPrintsWeaveVersion (byte-exact "weave dev\n")
#   - TestRunPathSuccess (byte-exact dir + "(found via weave_EXTENSIONS_DIR)\n")
#   - TestRunPathFailureErrNotFound (exit 1, EMPTY stdout, stderr has "run" + "weave init")
#   - TestRunVersionPrecedenceOverPath (--version wins, Find never called)
#   - TestParseArgsUnknownFlagCaptured (first unknown only; --bogus captured)
#   - any bundle/=/value-taking parseArgs tests ported from skilldozer
# If any fail, read the failure, fix main.go (NOT the test — tests encode the contract).

# Whole-repo test suite.
go test ./... -v
# EXPECTED: ALL tests pass (config + extdir + main).

# Race detector sanity.
go test -race ./...
# EXPECTED: passes, no data races.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (main.go + internal/config + internal/extdir must all compile).
go build ./...
# EXPECTED: exit 0. NOTE: this REQUIRES S3 to have landed extdir.Find/ErrNotFound —
# if S3 hasn't landed, `go build .` fails with "undefined: extdir.Find". That is expected
# (T3 and T4 complete M1 together); do NOT stub Find/ErrNotFound in main.go.

# Build the actual binary and smoke-test the two wired commands.
go build -o /tmp/weave-smoke .
/tmp/weave-smoke --version
# EXPECTED: prints exactly "weave dev" (no ldflags under plain go build), exit 0.

# --path success via env (rule 1).
extdir=$(mktemp -d)
weave_EXTENSIONS_DIR="$extdir" /tmp/weave-smoke --path
# EXPECTED: stdout = "$extdir" (the cleaned dir); stderr = "(found via weave_EXTENSIONS_DIR)";
# exit 0. Verify: test "$(/tmp/weave-smoke --path 2>/dev/null)" = "$extdir" && echo OK.

# --path failure (unconfigured).
env -u weave_EXTENSIONS_DIR -u weave_CONFIG /tmp/weave-smoke --path
# EXPECTED: stdout empty; stderr = "weave is not configured; run \`weave init\`"; exit 1.
# Run from /tmp to avoid rule 4 (walk-up): (cd /tmp && env -u weave_EXTENSIONS_DIR -u weave_CONFIG /tmp/weave-smoke --path)

# ldflags override smoke (proves the version var is ldflags-targetable).
go build -ldflags "-X main.version=v9.9.9-test" -o /tmp/weave-smoke-ld .
/tmp/weave-smoke-ld --version
# EXPECTED: prints exactly "weave v9.9.9-test" (proves var, not const; symbol path main.version).

rm -f /tmp/weave-smoke /tmp/weave-smoke-ld
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove --version precedence + $(...) safety end-to-end (the §6.4 contract
# that makes `pi -e "$(weave <tag>)"` fail loudly). This is informational — the unit tests are
# the authoritative gate, but this mirrors real shell usage.
go build -o /tmp/weave-smoke .
# Version inside command substitution (captures stdout only):
v=$( /tmp/weave-smoke --version 2>/dev/null )
echo "captured: '$v'"   # EXPECTED: 'weave dev'
# --path inside command substitution:
extdir=$(mktemp -d)
p=$( weave_EXTENSIONS_DIR="$extdir" /tmp/weave-smoke --path 2>/dev/null )
test "$p" = "$extdir" && echo "--path $(...) OK" || echo "FAIL"
# --path failure inside command substitution (empty capture + nonzero exit):
( cd /tmp && env -u weave_EXTENSIONS_DIR -u weave_CONFIG /tmp/weave-smoke --path 2>/dev/null ) \
  || echo "--path failure exits nonzero (correct: $(...) safety)"
rm -f /tmp/weave-smoke

# go mod tidy is a no-op on a zero-external-dep module.
go mod tidy
git diff --stat go.mod   # expect: empty / unchanged
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l main.go main_test.go` empty; `go vet ./...` clean; no
      third-party import; no `require` block in go.mod; no skilldozer/skillsdir leftovers;
      `--version` format `"weave %s\n"` present; source label `(found via %s)` present;
      `var version = "dev"` (not const).
- [ ] Level 2 passed: `go test -run 'TestParseArgs|TestRun' -v .` — ALL tests pass, including
      byte-exact version/path assertions, the ErrNotFound failure path (empty stdout), and
      version precedence.
- [ ] Level 3 passed: `go build ./...` exit 0 (S3 landed); the built binary's `--version`
      prints "weave dev"; `--path` succeeds via env and fails loudly unconfigured; the ldflags
      build overrides version (proves var not const).
- [ ] `go test -race ./...` passes (no data races).

### Feature Validation

- [ ] `main.go` exists at repo root with `package main` and the 9 symbols (version, isTerminal,
      main, config, parseArgs, expandShortBundle, run — plus the file-level doc comment and imports).
- [ ] `version` is `var version = "dev"` (ldflags-targetable; symbol path main.version).
- [ ] `config` struct has all 14 fields in skilldozer's order with the documented types.
- [ ] `parseArgs` handles every flag form (long, short, `=`, bundles, value-taking, reserved
      subcommands, positional tags, unknown-flag capture).
- [ ] `expandShortBundle` is two-phase (validate-then-commit); unknown char rejects the whole
      bundle with nothing applied.
- [ ] `run` dispatches `--version` (stdout `weave <version>\n`, exit 0) and `--path` (Find;
      dir→stdout + label→stderr on success exit 0; err→stderr + exit 1 on miss); all other
      modes are no-ops (return 0).
- [ ] `--version` precedence over `--path` (version printed, Find never called, exit 0).

### Code Quality Validation

- [ ] Mirrors skilldozer's version/isTerminal/main/config/parseArgs/expandShortBundle/run
      structure verbatim (with the documented doc-comment renames and the run() trim).
- [ ] Import set is exactly {fmt, io, os, strings, internal/extdir} — no unused imports, no
      deferred-milestone imports leaked in.
- [ ] File placement: `main.go` and `main_test.go` at repo root, `package main`.
- [ ] Anti-patterns avoided: no usageText/exclusivityError/skillPath/runInit (deferred); no
      third-party import; no require block; no stubbing of extdir.Find; no const version;
      no source-label on stdout; no prefix on the ErrNotFound error line.

### Documentation & Deployment

- [ ] main.go is self-documenting via the package doc comment + per-symbol doc comments
      (version's ldflags note, run's precedence/dispatch notes, parseArgs's matrix notes).
      Per the item contract: "DOCS: none — CLI surface (--version, --path) is documented in
      README (Mode B final task). The version var JSDoc is inline in main.go."
- [ ] No new user-facing env vars introduced by this subtask (weave_EXTENSIONS_DIR and
      weave_CONFIG already exist from M1.T2/M1.T3).

---

## Anti-Patterns to Avoid

- ❌ Don't declare `version` as `const`. It MUST be `var version = "dev"` — `-X main.version=...`
  ldflags cannot override a const (compile error) or a function-local. Pinned by the Level 3
  ldflags smoke test (`-X main.version=v9.9.9-test` → `weave v9.9.9-test`).
- ❌ Don't print the `(found via %s)` source label to stdout. It goes to STDERR only — stdout
  gets the dir + newline, byte-exact, so the §13 `test "$(weave --path)" = ...` gate passes.
  This is the verbatim skilldozer "Issue 1 (QA)" convention.
- ❌ Don't format `--version` output as anything other than `weave <version>\n`. Not "Weave",
  not "weave v%s", not "version %s", not "weave: %s". PRD §6.1 is literal. Pinned byte-exact
  by TestRunVersionPrintsWeaveVersion.
- ❌ Don't collapse `expandShortBundle` to a single-phase loop. The two-phase
  (validate-then-commit) is load-bearing: a leaked partial commit (e.g. `version=true` from
  "-vz" before detecting the unknown 'z') would make run() print the version (exit 0) and mask
  the unknown-char error that M5 turns into exit 2.
- ❌ Don't add `--help`, `exclusivityError`, the unknownFlag→exit-2 branch, the no-args→usage
  branch, or any internal-package dispatch (list/search/check/init/tags/all/file/relative).
  Those are M5.T1.S1 / M2 / M3 / M4. This milestone's run() dispatches ONLY --version and --path;
  everything else returns 0 (no-op). The parser is complete; later milestones add dispatch
  branches, not parser changes.
- ❌ Don't import bufio, path/filepath (in main.go — main_test.go CAN use it), discover, resolve,
  ui, check, search, or configpkg. main.go's import set is exactly {fmt, io, os, strings,
  internal/extdir}. An unused import fails `go build` ("imported and not used").
- ❌ Don't prefix or wrap the ErrNotFound error. `fmt.Fprintln(stderr, err)` prints err.Error()
  + newline — for extdir.ErrNotFound that's exactly the §6.4/§8.2 one-line fix. Do NOT do
  `fmt.Fprintf(stderr, "error: %v\n", err)` or `fmt.Fprintln(stderr, err.Error())` with extra text.
- ❌ Don't forget the weave_CONFIG (LOWERCASE) neutralization in the --path failure test. Without
  it, a dev machine's real ~/.config/weave/config.yaml leaks a real dir and the test prints it
  instead of ErrNotFound. (S3's extdir tests have the same requirement.)
- ❌ Don't call `t.Parallel()` on tests that mutate env (t.Setenv) or cwd (t.Chdir). Both are
  process-global. t.Setenv and t.Chdir enforce this by marking the test non-parallel, but
  omitting t.Parallel() explicitly is the file convention.
- ❌ Don't change the flag tokens in parseArgs. The §6 CLI contract is byte-identical to
  skilldozer's (PRD §6 header). --version stays --version; the renames are ONLY in doc comments
  (skilldozer→weave, skill→extension, SKILL.md→entry file, `pi --skill`→`pi -e`).
- ❌ Don't stub `extdir.Find` or `extdir.ErrNotFound` in main.go to make it build in isolation.
  T3 (S3) and T4 (S1) are a parallel pair that together complete M1. If S3 hasn't landed,
  `go build .` fails with "undefined: extdir.Find" — that is EXPECTED. main.go calls the real
  extdir.Find.
