# PRP — P1.M4.T4.S2: `setupStore` + `runInit` + `exampleExtensionTemplate` + init dispatch in `run()`

> **Subtask:** The EFFECTS + DISPATCH half of `weave init` (PRD §8.2). The store-CHOICE
> half (`resolveStore`/`chooseStore`/`stdinIsTerminal`/`readPrompt`) is the parallel
> sibling P1.M4.T4.S1 — assume it lands exactly as its PRP specifies, leaving those 4
> functions defined-but-uncalled in `main.go` with the `bufio` + `configpkg` imports in
> place. THIS subtask (a) defines `const exampleExtensionTemplate` (the PRD §11 example.ts
> body as a single Go raw-string literal — no backticks, no `go:embed`), (b) defines
> `func setupStore(store, configPath string) (seeded bool, err error)` (mkdir + seed-if-
> empty + write-config), (c) defines `func runInit(c config, stdout, stderr io.Writer) int`
> (orchestrate resolveStore → config.Path → setupStore, then print the `--path` rendering +
> the `check` report), and (d) adds the one-line `if c.init { return runInit(c, stdout,
> stderr) }` dispatch in `run()` before the normal-mode ladder. After this, `weave init`,
> `weave init <dir>`, and `weave init --store <dir>` all work end to end.

---

## Goal

**Feature Goal**: Wire `weave init` end to end so PRD §6.1 (init row) + §8.2 hold:
`weave init` / `init <dir>` / `init --store <dir>` resolve the store (S1's
`resolveStore`), create+seed it, write the config, and report (the configured store path
to stdout per §6.1; the `--path` "found via" annotation to stderr per §8.2 step 5; the
`check` report to stdout per §8.2 step 5); exit 0 on setup success, 1 on setup failure.
The bare `weave <tag>` path stays untouched and is re-asserted to never prompt / write
nothing to stdout / exit 1 when unconfigured (PRD §6.4 / §8.2 prompt-safety).

**Deliverable**: Additive edits to TWO existing files only:
1. `main.go` — (a) define `const exampleExtensionTemplate` (the PRD §11 body); (b) append
   `func setupStore` (after `resolveStore`, which S1 appended last); (c) append
   `func runInit`; (d) insert `if c.init { return runInit(c, stdout, stderr) }` in `run()`
   right after the `if c.version { … }` block, before the `// 2) --path` comment.
2. `main_test.go` — append 6 tests under a `// --- init: setupStore + runInit (M4.T4.S2) ---`
   section header (4 setupStore unit tests + 1 runInit integration test + 1 never-prompt
   regression guard), reusing the existing `unsetExtEnv`/`writeExtTree` helpers.

NO new files. NO new imports (everything `setupStore`/`runInit` call is already imported
after S1 lands: `os`, `path/filepath`, `fmt`, `io`, `configpkg`, `extdir`, `discover`,
`check`, plus S1's `resolveStore`). NO `go.mod`/`go.sum` change. NO `parseArgs` change
(already captures `init`/`--store`/`initStore` since M1.T4.S1). NO `internal/*` change.

**Success Definition**:
- `go build ./...`, `go vet ./...`, `go test ./... -v` all pass (existing tests + 6 new).
- `go.mod`/`go.sum` byte-for-byte unchanged; `gofmt -l main.go main_test.go` empty.
- `setupStore(<empty-tmp>, <cfg-tmp>)` → `(true, nil)`; reads
  `<store>/example.ts` and its bytes equal `exampleExtensionTemplate` exactly; the config
  round-trips `Store == store` via `configpkg.Load`.
- `setupStore(<store-with-a-preexisting-file>, <cfg>)` → `(false, nil)`; the pre-existing
  file is byte-intact; NO `example.ts` created; config still written.
- `run(["init","--store",<tmp>], &out, &errOut)` (with `weave_CONFIG`→temp cfg,
  `weave_EXTENSIONS_DIR`="", `t.Chdir(tempdir)`) → exit 0; store dir created;
  `configpkg.Load(cfg).Store == store`; `out` contains the store path AND the check
  summary substring `"extensions,"`; `errOut` contains `"Seeded example extension at"` and
  `"(found via config file)"`.
- `run(["sometag"], &out, &errOut)` under a clean env (`unsetExtEnv` + `t.Chdir(tempdir)`)
  → exit 1; `errOut` contains the `run \`weave init\`` hint; `out` is EMPTY; no stdin block
  (init is never reached — `c.init` is false for tags).

## User Persona (if applicable)

**Target User**: A first-run `weave` user running `weave init` (the documented first
command, PRD §8.2). Also scripts/CI running `weave init <dir>` or `weave init --store <dir>`
non-interactively (a `go install` user gets a seeded store + config in one command).

**Use Case**: `weave init --store ~/.local/share/weave/extensions` (CI/scripts), `weave init`
inside a dir that already looks like a store (S1's resolveStore adopts cwd), or `weave init`
on a fresh TTY (S1 prompts, Enter accepts default). Each lands a working store + config + a
validation report; next `weave --list` / `weave example` resolve via the config rule.

**User Journey**: `weave init --store /tmp/store` → `resolveStore` returns the absolute store
→ `setupStore` creates it + seeds `example.ts` + writes `store:` to config.yaml → `runInit`
prints the store path to stdout, `(found via config file)` to stderr, and the check report
(`OK example (example)` + `1 extensions, 0 errors, 0 warnings`) to stdout → exit 0.

**Pain Points Addressed**: the binary now has a working first-run path (init was a parsed-but-
no-op exit-0 before this); a `go install` user gets a seeded store + config in one command;
`$(weave badtag)` still fails loudly (the bare-tag path never reaches init, so command
substitution can't hang inside a prompt).

## Why

- **Implements PRD §8.2 steps 2-5** — the create/seed/write/report half of `weave init`
  (the choose half is S1). The item description is the authoritative contract: seed
  `example.ts` (single-file) into the store ROOT (NOT `example/SKILL.md` in a subdir), as a
  compiled-in string constant (PRD §8.2 step 3 / §17: "not go:embed of a directory").
- **Implements PRD §6.1 init row** — stdout = the configured store path; exit 0/1.
- **Implements PRD §8.2 step 5** ("Print the output of `weave --path` and `weave check`")
  by reproducing weave's OWN `--path` rendering (dir→stdout, found-via→stderr) and `check`
  rendering (report→stdout) inside `runInit`, so the user immediately sees which §8.3 rule
  won and the validation result after setup.
- **Locks the load-bearing prompt-safety guarantee** (PRD §8.2 / §6.4) — the bare `<tag>`
  path never prompts. The guarantee is STRUCTURAL: stdin access is confined to `resolveStore`
  (S1), which is called ONLY inside `if c.init`. S2 adds a regression test that re-asserts
  the bare-tag path exits 1 with empty stdout and never blocks.
- **Closes the M4.T4 milestone** — S1 (choice) + S2 (effects + dispatch) together make
  `weave init` a working command, unblocking the §13 acceptance gate (P1.M6.T1.S1).

## What

### Success Criteria

- [ ] `const exampleExtensionTemplate` is defined as a SINGLE Go raw-string literal equal to
      the PRD §11 example.ts body BYTE-FOR-EXACT (the `/** … */` JSDoc + the `import type …`
      + the `export default function (pi: ExtensionAPI) { … }` body). Confirmed zero
      backticks in the body (so NO `+ "\`" +` splicing is needed).
- [ ] `func setupStore(store, configPath string) (seeded bool, err error)` is appended and:
      (a) `os.MkdirAll(store, 0o755)`; (b) `os.ReadDir(store)` → if EMPTY,
      `os.WriteFile(filepath.Join(store, "example.ts"), []byte(exampleExtensionTemplate), 0o644)`
      and `seeded=true` (NO `example/` subdirectory, NO second MkdirAll — single file at the
      root); (c) non-empty → adopt in place, NEVER clobber (PRD §17), `seeded` stays false;
      (d) `configpkg.Save(configPath, configpkg.File{Store: store})` ALWAYS runs; returns
      `(seeded, nil)` on success or `(false, err)` wrapped with `"weave init: <step>: %w"`.
- [ ] `func runInit(c config, stdout, stderr io.Writer) int` is appended and: (1)
      `store, err := resolveStore(c.initStore)` → on err, `fmt.Fprintln(stderr, err); return 1`;
      (2) `cfgPath, err := configpkg.Path()` → on err, `fmt.Fprintln(stderr, err); return 1`;
      (3) `seeded, err := setupStore(store, cfgPath)` → on err, `fmt.Fprintln(stderr, err);
      return 1`; (4) report `"Seeded example extension at <store>/example.ts"` OR
      `"Adopted existing store at <store>"` to STDERR; (5) `dir, src, ferr := extdir.Find()`
      (AFTER setupStore so the just-written config is visible) → on err, print to stderr and
      fall back `dir = store`; print `dir` to STDOUT; if `ferr == nil` print
      `"(found via %s)\n", src` to STDERR; (6) `discover.Index(dir)` + `check.Check(dir, exts)`
      → render the report to STDOUT (mirroring the `if c.check` branch: per-extension
      `OK`/finding lines + the `N extensions, M errors, K warnings` summary); (7) `return 0`.
- [ ] `run()` inserts `if c.init { return runInit(c, stdout, stderr) }` immediately AFTER the
      `if c.version { … }` block and BEFORE the `// 2) --path` comment. init is exclusive;
      M5.T1.S1's `exclusivityError` will later slot in between version and init.
- [ ] runInit uses ZERO new imports.
- [ ] `runInit` calls `check.Check(dir, exts)` — dir FIRST, exts SECOND (weave signature;
      NOT skilldozer's `Check(exts)`).
- [ ] The bare-tag branch (`len(c.tags) > 0`) is UNCHANGED — no `resolveStore`/stdin call
      leaks into it.
- [ ] `main_test.go` adds 6 tests (see Validation Loop §Level 2); `go test ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** Every consumed symbol is pinned to a verified, exported
signature: `resolveStore(haveStore) (string, error)` (S1, assumed landed), `configpkg.Path/
Save/Load/File`, `extdir.Find() (dir, src, err)` + `Source.String()` labels, `discover.
Index(dir) ([]Extension, error)`, `check.Check(dir, exts) Report` (dir FIRST), the
`Report`/`ExtensionReport`/`Finding` shape, and the existing `relTagBase` helper in main.go
for the check-render name fallback. The PRD §11 body is transcribed byte-exact (verified
zero backticks via `grep -c`). The run() insertion anchor is transcribed verbatim. The two
renderings to mirror (`--path`, `check`) are copied line-for-line from the LIVE weave
main.go branches. The four weave-vs-skilldozer deltas (seed `example.ts` at root, not a
subdir; `Check(dir, exts)` not `Check(exts)`; check report → stdout not stderr; no
expandHome) are each flagged as CRITICAL. An implementer who has never seen this repo can
complete it in one pass.

### Documentation & References

```yaml
# MUST READ — the verified facts (signatures + anchor + renderings + the weave deltas)
- docfile: plan/001_19b4b465824d/P1M4T4S2/research/verified_facts.md
  why: "§1 = S1's landed resolveStore + the import block S2 inherits (ZERO new imports).
        §2 = the consumed API signatures (CRITICAL: check.Check(dir, exts) — dir FIRST).
        §3 = the run() insertion anchor (after if c.version, before // 2) --path). §4 = the
        exampleExtensionTemplate bytes (verified zero backticks → single raw literal). §5 =
        THE KEY DELTA: seed store/example.ts (root, single file), NOT store/example/SKILL.md
        (subdir). §6 = THE 2ND DELTA: runInit's check report → STDOUT (weave diverges from
        skilldozer's stderr, faithfully reproducing weave's own --path+check). §7 = the test
        bodies to port. §8 = lowercase env var names (weave_CONFIG / weave_EXTENSIONS_DIR)."
  critical: "§5 (single-file at root, no exampleDir mkdir) and §6 (check report on stdout)
             are the two places a naive skilldozer port goes WRONG. §2's Check(dir, exts)
             arg order is the third."

# CONTRACT — the parallel sibling whose outputs S2 consumes
- docfile: plan/001_19b4b465824d/P1M4T4S1/PRP.md
  why: "S1 defines resolveStore(haveStore) (string, error) — runInit calls resolveStore(c.
        initStore), NOT chooseStore (chooseStore is S1's pure 5-arg core). S1 lands the
        bufio + configpkg imports S2 inherits unchanged. S1 explicitly defers expandHome
        (out of scope) and leaves the 4 functions UNCALLED — S2 is the sole wiring site."
  pattern: "resolveStore absolutizes via filepath.Abs (NO tilde expansion); the prompt
            closure writes to os.Stderr so init's stdout stays clean for the store path."

# The file under edit — locate symbols by NAME (line numbers shift as S1 + this task land)
- file: main.go
  why: "THE edit target. run() dispatch ladder; the if c.version block is the init-dispatch
        ANCHOR (insert after its `return 0`, before `// 2) --path`). The if c.path branch =
        the --path rendering to MIRROR (dir→stdout, found-via→stderr). The if c.check branch
        = the check rendering to MIRROR (report→stdout). The bare-tag branch (len(c.tags)>0)
        = UNCHANGED. resolveStore (S1) is the append-after target (append setupStore, then
        runInit, after it). relTagBase() is the existing name-fallback helper to reuse."
  pattern: "run()-level mode handler = a self-contained `if c.<mode> { …; return <code> }`
            block taking (stdout, stderr io.Writer) and returning int (so main() calls
            os.Exit(run(...)) and tests capture output via *bytes.Buffer). runInit is the
            init analogue of the check/path branches but ORCHESTRATES S1/S2 helpers."
  gotcha: "main.go ~L77 declares `type config struct` — internal/config is imported as the
           configpkg ALIAS (S1 lands it). Call configpkg.Path() / configpkg.Save() /
           configpkg.File{} / configpkg.Load() — never bare `config.`."

# The consumed config package (configpkg alias; Path = config-file LOCATION; Save = write)
- file: internal/config/config.go
  why: "Path() (pure env fn: $weave_CONFIG or XDG default); Save(path, File) writes
        `store: <value>\n` with MkdirAll(parent)+WriteFile 0o644; Load(path) round-trips for
        tests; File{Store string}. runInit calls configpkg.Path() → cfgPath for setupStore;
        setupStore calls configpkg.Save(cfgPath, configpkg.File{Store: store})."
  gotcha: "Path() can error (relative $XDG_CONFIG_HOME, or $HOME unset). runInit treats that
           as a setup failure: Fprintln(stderr, err); return 1 (cannot determine where to
           write the config). It does NOT fall through silently."

# The consumed extdir package (Find + Source labels + ErrNotFound)
- file: internal/extdir/extdir.go
  why: "Find() (L291) = the 5-rule §8.3 ladder; returns (dir, src, ErrNotFound) when all
        miss. ErrNotFound.Error() = the one-line `weave is not configured; run \`weave init\``
        (verbatim, literal backticks). Source.String() labels (L55+): 'weave_EXTENSIONS_DIR'
        | 'config file' | 'sibling of binary' | 'ancestor of cwd'. After setupStore writes
        the config, Find() resolves via the config rule (src='config file') UNLESS
        weave_EXTENSIONS_DIR is set (env is priority #1)."
  gotcha: "runInit calls Find() AFTER setupStore (not before) so the just-written config is
           visible. In the common case dir==store (config rule wins). If weave_EXTENSIONS_DIR
           is set, dir==env value (env beats config) — runInit honestly reports that."

# The consumed check package — CRITICAL arg order
- file: internal/check/check.go
  why: "Check(dir string, exts []discover.Extension) Report — DIR FIRST, exts SECOND (unlike
        skilldozer's Check(exts)). The dir arg is required for the §9 empty-category-folder
        walk. Report{ByExt []ExtensionReport, Errors, Warnings int}; HasErrors(); Extension
        Report{Extension discover.Extension, Findings []Finding}; Finding{Level Severity,
        Message string}; Severity.String() → 'OK'|'WARN'|'ERROR'. check.Check does NOT print;
        runInit renders the report per the §9 format."
  gotcha: "Call check.Check(dir, exts) — passing exts first is a COMPILE ERROR (type mismatch
           on the first arg: string vs []Extension) — so a swapped arg fails loudly at build."

# The reference port (skilldozer main.go) — port the SHAPE, apply the 4 weave deltas
- file: /home/dustin/projects/skilldozer/main.go
  why: "L1016 setupStore + L1057 runInit + L969 exampleSkillTemplate + L482 `if c.init` are
        the near-verbatim source. Port the structure; apply deltas (§5/§6 of verified_facts:
        seed example.ts at root not example/SKILL.md subdir; check.Check(dir,exts) not
        Check(exts); check report → stdout not stderr; error prefix weave init:)."

# The test-template source — port with the example.ts deltas
- file: /home/dustin/projects/skilldozer/main_test.go
  why: "L2472 TestSetupStoreEmptyDirSeeds…, L2504 TestSetupStoreNonEmpty…, L2543 TestSetupStore
        Idempotent, L2584 TestSetupStoreMkdirAllFailure…, L2611 TestRunInitStoreWritesConfig…
        are the tests to port. Delta: read store/example.ts (not store/example/SKILL.md);
        assert NO example.ts (not no example/ dir); weave env vars (weave_CONFIG,
        weave_EXTENSIONS_DIR, lowercase); runInit check report on STDOUT (assert stdout
        CONTAINS the check summary, NOT exactly-one-line)."

# PRD spec (authoritative)
- docfile: PRD.md
  section: "§8.2 (the init flow: cwd-auto-detect first, then XDG default, then prompt only on
           TTY; step 2 mkdir -p; step 3 seed example.ts as a string constant if empty, adopt
           if non-empty, never clobber; step 4 write config.yaml; step 5 print the output of
           `weave --path` and `weave check`). §11 (the exact example.ts body). §6.1 (init row:
           stdout = the configured store path; exit 0/1). §6.4/§8.2 prompt-safety (bare
           `weave <tag>` never prompts). §17 (nothing about the user's collection is compiled
           in — so a string constant, NOT go:embed)."
```

### Current Codebase tree (relevant subset)

```bash
main.go                  # ← MODIFIED: +const exampleExtensionTemplate, +setupStore, +runInit, +init dispatch in run()
main_test.go             # ← MODIFIED: +6 init tests (setupStore ×4, runInit ×1, never-prompt ×1)
internal/
├── config/config.go     # configpkg.Path/Save/Load/File — CONSUMED (M1.T2.S1, done)
├── extdir/extdir.go     # Find()/Source — CONSUMED (M1.T3.S3, done)
├── discover/{extension,index}.go  # Extension struct, Index() — CONSUMED (M2, done)
├── check/check.go       # Check(dir, exts)/Report — CONSUMED (M4.T2.S1, done)
├── resolve/…            # NOT touched (tag resolution)
├── search/…             # NOT touched
└── ui/…                 # NOT touched
```

### Desired Codebase tree with files to be added/modified

```bash
main.go          # MODIFIED — +exampleExtensionTemplate const; +setupStore; +runInit; +`if c.init` dispatch in run()
main_test.go     # MODIFIED — +6 tests under "--- init: setupStore + runInit (M4.T4.S2) ---"
# (no new files; no go.mod/go.sum change; no internal/* change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: seed store/example.ts (a SINGLE FILE at the store ROOT), NOT
// store/example/SKILL.md in a subdirectory. skilldozer's setupStore does
// `MkdirAll(store/example)` then `WriteFile(store/example/SKILL.md, …)`; weave does NOT
// create an `example/` subdir — it writes `example.ts` directly into the store root. There
// is only ONE MkdirAll in weave's setupStore: `os.MkdirAll(store, 0o755)`. Do NOT add a
// second MkdirAll for an exampleDir.

// CRITICAL: check.Check(dir, exts) — DIR FIRST, exts SECOND. This is weave's signature
// (the dir is needed for the §9 empty-category-folder walk). skilldozer's check.Check(exts)
// takes only exts. Passing exts first is a compile error (string vs []Extension type
// mismatch on arg 1) — so a swapped arg fails at `go build`, not silently.

// CRITICAL: runInit's check report goes to STDOUT (the item description's step 6: "print
// check report to stdout"). This DIVERGES from skilldozer, which renders init's check
// report to stderr. The weave choice is internally consistent: weave's standalone `check`
// subcommand ALSO uses stdout, and PRD §8.2 step 5 ("print the output of `weave --path` and
// `weave check`") under weave's conventions = dir→stdout + found-via→stderr + report→stdout.
// So the TestRunInit assertion is: stdout CONTAINS the store path AND the check summary
// ("extensions,"), NOT `stdout == store+"\n"` (that skilldozer assertion would FAIL on weave
// because the report is intentionally on stdout here).

// CRITICAL: import internal/config WITH THE configpkg ALIAS. main.go ~L77 declares
// `type config struct` (the parsed-CLI struct); the package name `config` collides. S1
// lands the alias; S2 must use configpkg.Path() / configpkg.Save() / configpkg.File{} /
// configpkg.Load() — never bare `config.`.

// CRITICAL: do NOT add expandHome. S1's PRP explicitly defers it (weave's resolveStore
// absolutizes via filepath.Abs only). `init --store ~/x` will create a dir literally named
// "~" — that is a KNOWN, ACCEPTED limitation for M4.T4, fixed in a later milestone if at
// all. Do NOT port skilldozer's TestRunInitStoreTildeExpandsHome.

// CRITICAL: the init dispatch goes AFTER `if c.version` and BEFORE the normal-mode ladder
// (`// 2) --path`). There is NO exclusivityError/unknownFlag/storeMissingValue dispatch
// yet (all M5.T1.S1). §6.3 precedence is preserved: --version still wins over init
// (`init --version` prints the version). When M5 lands exclusivityError, it slots BETWEEN
// version and init — stable, conflict-free.

// GOTCHA: runInit calls extdir.Find() AFTER setupStore (not before) so the just-written
// config.yaml is visible to the config rule. In the common case dir==store and
// src=="config file". If weave_EXTENSIONS_DIR is set, env beats config and dir/src reflect
// that honestly (the report says "(found via weave_EXTENSIONS_DIR)").

// GOTCHA: the seeded example.ts is a §7.1 single-file entry. discover.Index finds it with
// RelTag "example" (trailing .ts stripped), Kind "file", and a description extracted from
// its leading JSDoc. So check.Check reports it OK (no description-WARN, entry exists), and
// the runInit check report for a freshly-seeded store is:
//   "OK    example (example)\n1 extensions, 0 errors, 0 warnings\n"
// (Name="" → fallback relTagBase("example")="example"). The TestRunInit assertion keys off
// the summary substring "extensions,".

// GOTCHA: the bare-tag never-prompt guarantee is STRUCTURAL — stdin access lives only in
// resolveStore (S1), called only inside `if c.init`. The bare-tag branch (len(c.tags)>0)
// calls extdir.Find directly and never reaches resolveStore. The TestRunBareTag regression
// test asserts exit 1 + hint on stderr + EMPTY stdout under a clean env; if a future change
// leaked resolveStore into the tag path, the test's stdout-empty assertion would catch the
// wrong output (the prompt itself goes to stderr, so stdout-empty alone doesn't prove
// no-prompt — but combined with the structural confinement it is a meaningful guard).

// GOTCHA: exampleExtensionTemplate is a SINGLE raw-string literal because the PRD §11 body
// has ZERO backticks (verified via grep -c). Do NOT splice with `+ "\`" +` (that is
// skilldozer's exampleSkillTemplate pattern, needed only because ITS body has 8 backticks).
// A future editor who edits the template MUST re-verify zero backticks or convert to the
// splice form.

// GOTCHA: Go raw-string literals preserve leading/trailing newlines verbatim. Write the
// constant as `const exampleExtensionTemplate = ` + "`" + `/** … ` (content starts with
// `/**` immediately after the opening backtick) … `}` + newline + closing backtick. This
// matches skilldozer's exampleSkillTemplate framing and makes the seeded file end with a
// trailing newline (POSIX text-file convention).
```

## Implementation Blueprint

### Data models and structure

None new. This task consumes:
- `resolveStore(haveStore string) (string, error)` (S1, assumed landed).
- `configpkg.Path() (string, error)`, `configpkg.Save(path, File) error`, `configpkg.File{Store string}`.
- `extdir.Find() (dir, src, err)`.
- `discover.Index(dir) ([]Extension, error)`.
- `check.Check(dir, exts) Report` + `Report.ByExt/Errors/Warnings`, `ExtensionReport.Extension/Findings`, `Finding.Level/Message`.
- stdlib `os` (MkdirAll, ReadDir, WriteFile), `path/filepath` (Join), `fmt` (Errorf, Fprintln, Fprintf).

It produces: one `const` (string), two `func`s, one dispatch line in `run()`. No structs,
no interfaces, no state.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: DEFINE const exampleExtensionTemplate in main.go
  - EDIT main.go: add `const exampleExtensionTemplate = <raw string>` as a package-level
    const, placed NEAR the other top-level declarations (e.g. right after the `version` var
    or alongside `isTerminal`, BEFORE run()). Add the doc comment shown in "Implementation
    Patterns" (cites PRD §11, §17 "not go:embed", notes zero backticks → single raw literal,
    notes the M6.T1.S1 repo-asset `extensions/example.ts` MUST byte-match it).
  - CONTENT: the PRD §11 body BYTE-EXACT (see "Implementation Patterns"). Starts with `/**`
    right after the opening backtick; ends with `}\n` before the closing backtick.
  - VERIFY: `grep -c '`' ` on the body returns 0 (no splicing needed).
  - RUN: go build ./...  (the const is unused until Task 2 references it; Go allows unused
    package-level consts, so build passes. If you do Task 1+2 together, even cleaner.)
  - FILES TOUCHED: 1 (main.go).

Task 2: APPEND func setupStore to main.go
  - EDIT main.go: append `func setupStore(store, configPath string) (seeded bool, err error)`
    AFTER resolveStore (S1's last-appended function). Add the doc comment (cites PRD §8.2
    steps 2-4; notes the weave single-file-at-root delta vs skilldozer's subdir).
  - PORT FROM: /home/dustin/projects/skilldozer/main.go L1016-1042 (read in full).
  - APPLY DELTAS vs skilldozer (ALL of these):
    (a) NO `exampleDir := filepath.Join(store, "example")` and NO `MkdirAll(exampleDir)`.
        Instead write DIRECTLY: `os.WriteFile(filepath.Join(store, "example.ts"),
        []byte(exampleExtensionTemplate), 0o644)`.
    (b) The constant is `exampleExtensionTemplate` (NOT exampleSkillTemplate).
    (c) Error prefix "skilldozer init:" → "weave init:" (4 sites: create store dir, read
        store dir, seed example.ts, write config).
    (d) `config.Save(path, config.File{…})` → `configpkg.Save(configPath, configpkg.File{Store: store})`
        (the configpkg alias; arg order is path-FIRST — same as skilldozer).
  - KEEP from skilldozer: MkdirAll(store, 0o755) first; ReadDir to test emptiness;
    `len(entries) == 0` gates the seed; non-empty → adopt (do nothing to files, seeded stays
    false); config.Save ALWAYS runs (seeded or adopted); return (seeded, nil) on success or
    (false, wrapped err) on any fs failure.
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. Likely failure: a stale "skilldozer"/"SKILL.md"/"example/" string.
  - FILES TOUCHED: 1 (main.go).

Task 3: APPEND func runInit to main.go
  - EDIT main.go: append `func runInit(c config, stdout, stderr io.Writer) int` AFTER
    setupStore. Add the doc comment (cites PRD §8.2; notes the stdout check-report delta vs
    skilldozer; notes the never-prompt structural guarantee).
  - PORT FROM: /home/dustin/projects/skilldozer/main.go L1057-1134 (read in full).
  - APPLY DELTAS vs skilldozer (ALL of these):
    (a) Step (4) report: "Seeded example skill at %s" → "Seeded example extension at %s",
        path `filepath.Join(store, "example.ts")` (NOT store/example/SKILL.md). "Adopted
        existing store at %s" unchanged. STILL to stderr.
    (b) Step (5): `skillsdir.Find()` → `extdir.Find()`. dir → stdout, "(found via %s)" →
        stderr, fall back dir=store on Find error. UNCHANGED.
    (c) Step (6) — THE KEY DELTA: skilldozer renders the check report to STDERR; WEAVE
        renders it to STDOUT (the `w` argument to every Fprintf in the check-render loop is
        `stdout`, NOT `stderr`). Mirror the `if c.check` branch in weave main.go verbatim:
        name fallback `relTagBase(er.Extension.RelTag)` (NOT "(none)"); the per-extension
        `"%-5s %s (%s)\n"` / `"%-5s %s (%s): %s\n"` lines; the
        `"%d extensions, %d errors, %d warnings\n"` summary.
    (d) `check.Check(skills)` → `check.Check(dir, exts)` — DIR FIRST (weave signature).
        `discover.Index(dir)` → `exts`.
    (e) Error prefix where present: "weave init:" (resolveStore/setupStore already wrap with
        it; runInit just Fprintlns the wrapped err verbatim, like skilldozer).
  - KEEP from skilldozer: the 6-step structure (resolveStore → configpkg.Path → setupStore
    → report → Find+print → Index+Check+render → return 0); exit 1 on any of the first 3
    errors; check report is best-effort (an Index failure is Fprintln'd to stderr then
    `return 0`, NOT a gate); exit 0 once create+config succeed.
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. Likely failure: check.Check arg order swapped (compile error), or a
    stale "skills"/"skilldozer"/"stderr" in the render loop.
  - FILES TOUCHED: 1 (main.go).

Task 4: INSERT the init dispatch in run()
  - EDIT main.go: in run(), insert between the `if c.version { … return 0 }` block's closing
    brace and the `// 2) --path` comment:
        // 1.5) `weave init` dispatch (PRD §8.2). … (see Implementation Patterns for the
        //       full comment). init is exclusive; M5's exclusivityError will later slot in
        //       between version and init. The bare-tag path never reaches here.
        if c.init {
            return runInit(c, stdout, stderr)
        }
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. The dispatch is one line; runInit is defined in Task 3.
  - FILES TOUCHED: 1 (main.go).

Task 5: ADD 6 tests to main_test.go
  - EDIT main_test.go: append under a `// --- init: setupStore + runInit (M4.T4.S2) ---`
    section header, following the existing t.Helper()+t.TempDir()+filepath.Join style
    (white-box `package main`; reuse `unsetExtEnv` where noted). Add the 6 tests with full
    bodies in Validation Loop §Level 2.
  - PORT FROM: /home/dustin/projects/skilldozer/main_test.go L2472-2700 (read in full).
  - APPLY DELTAS: read `store/example.ts` (not store/example/SKILL.md); assert NO example.ts
    (not no example/ dir); `exampleExtensionTemplate` (not exampleSkillTemplate); env vars
    weave_CONFIG / weave_EXTENSIONS_DIR (lowercase); the runInit test asserts stdout
    CONTAINS the store path AND "extensions," (weave puts the report on stdout) and stderr
    CONTAINS "Seeded example extension at" + "(found via config file)".
  - DO NOT port TestRunInitStoreTildeExpandsHome (expandHome is out of scope).
  - FILES TOUCHED: 1 (main_test.go).

Task 6: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./... -v ; gofmt -l main.go main_test.go
  - EXPECT: all green; gofmt output empty.
```

### Implementation Patterns & Key Details

```go
// exampleExtensionTemplate — PRD §11 body, compiled-in STRING CONSTANT (NOT go:embed;
// PRD §17 "nothing about the user's collection is compiled in"). weave init writes this
// verbatim into an EMPTY store's example.ts (PRD §8.2 step 3). CROSS-TASK CONTRACT: the
// repo asset extensions/example.ts created by P1.M6.T1.S1 MUST equal this byte-for-byte.
//
// Single raw-string literal: the §11 body has ZERO backticks (verified), so NO `+ "`" +`
// splicing is needed (contrast skilldozer's exampleSkillTemplate, which splices 8). A
// future editor MUST re-verify zero backticks after any edit, or switch to the splice form.
const exampleExtensionTemplate = `/**
 * Reference example extension for weave. Demonstrates a minimal pi extension
 * and how weave resolves a tag to an absolute path. Registers a harmless
 * /weave-example command and greets on session start. Safe to delete once
 * you add real extensions.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("weave example extension loaded", "info");
  });

  pi.registerCommand("weave-example", {
    description: "Prove the weave example extension is loaded",
    handler: async (_args, ctx) => {
      ctx.ui.notify("Hello from the weave example extension!", "info");
    },
  });
}
`

// setupStore — the create+seed+writeconfig half of `weave init` (PRD §8.2 steps 2-4); the
// store-CHOICE half is resolveStore (S1), and run()'s `if c.init` dispatch (this task)
// calls both. Both targets are INJECTED strings (store is already absolute — resolveStore
// absolutized it; configPath is configpkg.Path()'s result from runInit), so this is
// directly unit-testable with temp paths (no wrapper layer).
//
// KEY WEAVE DELTA vs skilldozer: seed store/example.ts (a SINGLE FILE at the store ROOT),
// NOT store/example/SKILL.md in a subdirectory. There is NO exampleDir and NO second
// MkdirAll — only the one `os.MkdirAll(store, 0o755)`.
//
// Returns (seeded, nil) on success or (false, err) on any fs failure. `seeded` is a
// SUCCESS-PATH signal (runInit prints "Seeded" vs "Adopted"); callers MUST check err first.
func setupStore(store, configPath string) (seeded bool, err error) {
	if err := os.MkdirAll(store, 0o755); err != nil { // (a) ensure store exists (idempotent)
		return false, fmt.Errorf("weave init: create store dir %q: %w", store, err)
	}
	entries, err := os.ReadDir(store) // (b) seed only if EMPTY (zero entries of any kind)
	if err != nil {
		return false, fmt.Errorf("weave init: read store dir %q: %w", store, err)
	}
	if len(entries) == 0 {
		// Single-file extension at the store ROOT (PRD §11/§7.1). NO example/ subdir.
		if err := os.WriteFile(filepath.Join(store, "example.ts"), []byte(exampleExtensionTemplate), 0o644); err != nil {
			return false, fmt.Errorf("weave init: seed example.ts: %w", err)
		}
		seeded = true
	}
	// (c) Non-empty: adopt in place. Do NOTHING to existing files (PRD §17). seeded stays false.
	if err := configpkg.Save(configPath, configpkg.File{Store: store}); err != nil { // (d) ALWAYS write config
		return false, fmt.Errorf("weave init: write config %q: %w", configPath, err)
	}
	return seeded, nil
}

// runInit — the `weave init` orchestrator (PRD §8.2). run()'s dispatch calls it when
// c.init is true (init is exclusive; M5's exclusivityError will enforce that, but is not
// present yet — init is placed right after --version). It assembles S1's resolveStore +
// configpkg.Path + this task's setupStore, then reports: the configured store path to
// stdout (PRD §6.1), the `--path` "found via" annotation to stderr (PRD §8.2 step 5), and
// the `check` report to STDOUT (PRD §8.2 step 5 — weave reproduces its own `check` output;
// this DIVERGES from skilldozer, which put init's check report on stderr). Exit 0 once
// create+config succeed; check findings NEVER change init's exit code (best-effort report).
//
// The bare `weave <tag>` path NEVER reaches here (c.init is false for tags), so tag
// resolution never prompts (PRD §6.4/§8.2): stdin access is confined to resolveStore.
func runInit(c config, stdout, stderr io.Writer) int {
	store, err := resolveStore(c.initStore) // (1) S1: choose + absolutize (haveStore!="" never blocks)
	if err != nil {
		fmt.Fprintln(stderr, err) // resolveStore wraps with "weave init: …"
		return 1
	}
	cfgPath, err := configpkg.Path() // (2) config-file location ($weave_CONFIG or XDG default)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	seeded, err := setupStore(store, cfgPath) // (3) create + seed-if-empty + write config
	if err != nil {
		fmt.Fprintln(stderr, err) // setupStore wraps with "weave init: …"
		return 1
	}
	if seeded { // (4) report (STDERR — stdout stays the store-path headline)
		fmt.Fprintf(stderr, "Seeded example extension at %s\n", filepath.Join(store, "example.ts"))
	} else {
		fmt.Fprintf(stderr, "Adopted existing store at %s\n", store)
	}
	dir, src, ferr := extdir.Find() // (5) effective store + which §8.3 rule won (AFTER setupStore)
	if ferr != nil {
		fmt.Fprintln(stderr, ferr) // should not happen (config just written); fall back to store
		dir = store
	}
	fmt.Fprintln(stdout, dir) // §6.1: stdout = the configured store path
	if ferr == nil {
		fmt.Fprintf(stderr, "(found via %s)\n", src) // mirror `weave --path`
	}
	exts, ierr := discover.Index(dir) // (6) check report on the effective store (best-effort)
	if ierr != nil {
		fmt.Fprintln(stderr, ierr)
		return 0 // setup OK; the report is best-effort
	}
	rep := check.Check(dir, exts) // DIR FIRST, exts SECOND (weave signature)
	// Render to STDOUT (mirrors the standalone `weave check` branch; diverges from skilldozer).
	for _, er := range rep.ByExt {
		name := er.Extension.Name
		if name == "" {
			name = relTagBase(er.Extension.RelTag) // reuse the existing main.go helper
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
	return 0 // setup succeeded; check findings do not change init's exit code
}

// In run(), the dispatch (insert after `if c.version { … return 0 }`, before `// 2) --path`):
//
//	// 1.5) `weave init` dispatch (PRD §8.2). init is exclusive; until M5.T1.S1 lands
//	//      exclusivityError, init is placed right after --version (the highest-precedence
//	//      dispatch present) and before the normal mode ladder. runInit orchestrates
//	//      resolveStore → configpkg.Path → setupStore, then prints the --path rendering
//	//      (dir→stdout, found-via→stderr) + the check report (→stdout, mirroring the
//	//      standalone check) per §8.2 step 5. The bare-tag path never reaches here, so
//	//      tag resolution never prompts (§6.4/§8.2 prompt-safety).
//	if c.init {
//	    return runInit(c, stdout, stderr)
//	}
```

### Integration Points

```yaml
CONSUMES:
  - resolveStore(haveStore string) (string, error)   # S1 (assumed landed); runInit calls resolveStore(c.initStore)
  - configpkg.Path() (string, error)                 # config-file location
  - configpkg.Save(path, File) error                  # write config (path FIRST)
  - configpkg.File{Store string}                      # the one-key config struct
  - extdir.Find() (dir, src, err)                     # §8.3 ladder; called AFTER setupStore
  - discover.Index(dir) ([]Extension, error)          # walk the effective store
  - check.Check(dir, exts) Report                     # DIR FIRST; §9 validation
  - relTagBase(relTag string) string                  # existing main.go helper (check name fallback)
  - os.MkdirAll / os.ReadDir / os.WriteFile           # setupStore fs effects
  - filepath.Join                                     # store/example.ts path

PRODUCES:
  - const exampleExtensionTemplate (string)           # the PRD §11 body (byte-matches the M6.T1.S1 repo asset)
  - func setupStore(store, configPath string) (seeded bool, err error)
  - func runInit(c config, stdout, stderr io.Writer) int
  - the `if c.init { return runInit(c, stdout, stderr) }` dispatch in run()

NO CHANGES TO:
  - the import block (S1 owns it; S2 needs zero new imports — touching it risks a merge conflict).
  - parseArgs (already captures init/--store/initStore since M1.T4.S1).
  - any internal/* package.
  - go.mod / go.sum.
  - PRD.md / tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1-4 (main.go edits):
go build ./...
go vet ./...
gofmt -l main.go main_test.go   # after Task 5 too

# Expected: zero errors / empty gofmt output. Most likely failures:
#   (a) check.Check(exts) instead of check.Check(dir, exts) → compile error "cannot use exts
#       (type []discover.Extension) as string value in argument to check.Check". Fix: swap args.
#   (b) a stale skilldozer string ("skills"/"skilldozer"/"SKILL.md"/"example/") → grep:
#         grep -ni "skilldozer\|SKILL.md\|skills\b\|example/" main.go
#       (the word "example.ts" and "example extension" are CORRECT; flag only the stale ones.)
#   (c) wrote the check report to stderr instead of stdout → the runInit test's stdout
#       assertion fails (re-read §6 of verified_facts: stdout).
#   (d) created an example/ subdir in setupStore → the adopt test's "NO example.ts" check
#       is the wrong shape; re-read §5 of verified_facts (single file at root).
#   (e) `config.` instead of `configpkg.` → compile error "config redeclared". Fix: alias.
```

### Level 2: Unit Tests (Component Validation)

Append to `main_test.go` under a `// --- init: setupStore + runInit (M4.T4.S2) ---` header.
White-box `package main`; reuse `unsetExtEnv(t)` where noted; `t.TempDir()` + `filepath.Join`.

```go
// --- init: setupStore + runInit (M4.T4.S2) ---

// TestSetupStoreEmptyDirSeedsExampleAndWritesConfig locks the empty-store seed path: an
// empty store is created, example.ts is written with exampleExtensionTemplate EXACTLY, and
// the config is written with store=<abs> (round-tripped via configpkg.Load).
func TestSetupStoreEmptyDirSeedsExampleAndWritesConfig(t *testing.T) {
	store := t.TempDir() // empty
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	seeded, err := setupStore(store, cfg)
	if err != nil {
		t.Fatalf("setupStore(empty): %v; want nil", err)
	}
	if !seeded {
		t.Errorf("setupStore(empty): seeded=false; want true")
	}
	// example.ts exists at the store ROOT with the template bytes EXACTLY.
	got, err := os.ReadFile(filepath.Join(store, "example.ts"))
	if err != nil {
		t.Fatalf("read seeded example.ts: %v", err)
	}
	if string(got) != exampleExtensionTemplate {
		t.Errorf("seeded example.ts != exampleExtensionTemplate:\ngot:\n%s\nwant:\n%s", got, exampleExtensionTemplate)
	}
	// config written with store=<abs> (round-trip via configpkg.Load).
	f, err := configpkg.Load(cfg)
	if err != nil {
		t.Fatalf("configpkg.Load: %v", err)
	}
	if f.Store != store {
		t.Errorf("config.Store=%q; want %q", f.Store, store)
	}
}

// TestSetupStoreNonEmptyDirAdoptsInPlaceAndWritesConfig locks the §17 never-clobber
// guardrail: a store with ANY pre-existing entry is adopted — the file is byte-intact, NO
// example.ts is created, seeded is false — but the config is still written.
func TestSetupStoreNonEmptyDirAdoptsInPlaceAndWritesConfig(t *testing.T) {
	store := t.TempDir()
	preExisting := filepath.Join(store, "mynotes.md") // a non-extension file
	if err := os.WriteFile(preExisting, []byte("# my stuff\n"), 0o644); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	seeded, err := setupStore(store, cfg)
	if err != nil {
		t.Fatalf("setupStore(non-empty): %v; want nil", err)
	}
	if seeded {
		t.Errorf("setupStore(non-empty): seeded=true; want false (adopt in place)")
	}
	if got, err := os.ReadFile(preExisting); err != nil || string(got) != "# my stuff\n" {
		t.Errorf("pre-existing file changed: got %q, err=%v; want %q", got, err, "# my stuff\n")
	}
	// NO example.ts was created (single-file seed is gated on an EMPTY store).
	if _, err := os.Stat(filepath.Join(store, "example.ts")); !os.IsNotExist(err) {
		t.Errorf("example.ts must NOT be created in a non-empty store; stat err=%v", err)
	}
	f, err := configpkg.Load(cfg)
	if err != nil {
		t.Fatalf("configpkg.Load: %v", err)
	}
	if f.Store != store {
		t.Errorf("config.Store=%q; want %q", f.Store, store)
	}
}

// TestSetupStoreIdempotent locks the re-run contract: run 1 (empty) seeds, run 2 (non-empty)
// adopts and does NOT clobber example.ts (byte-identical), config stays valid.
func TestSetupStoreIdempotent(t *testing.T) {
	store := t.TempDir()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	seeded1, err := setupStore(store, cfg)
	if err != nil || !seeded1 {
		t.Fatalf("first run: (%v,%v); want (true,nil)", seeded1, err)
	}
	first, err := os.ReadFile(filepath.Join(store, "example.ts"))
	if err != nil {
		t.Fatalf("read after first run: %v", err)
	}
	seeded2, err := setupStore(store, cfg) // store now non-empty → adopt
	if err != nil {
		t.Fatalf("second run: %v; want nil", err)
	}
	if seeded2 {
		t.Errorf("second run: seeded=true; want false (store already has content)")
	}
	second, err := os.ReadFile(filepath.Join(store, "example.ts"))
	if err != nil {
		t.Fatalf("read after second run: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotent re-run changed example.ts:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	f, err := configpkg.Load(cfg)
	if err != nil {
		t.Fatalf("configpkg.Load after re-run: %v", err)
	}
	if f.Store != store {
		t.Errorf("config.Store=%q; want %q", f.Store, store)
	}
}

// TestSetupStoreMkdirAllFailureReturnsWrappedError locks the error path: when the store
// path points at an existing regular file, os.MkdirAll fails, setupStore returns (false,
// err), and NO config.yaml is written (the failure precedes configpkg.Save).
func TestSetupStoreMkdirAllFailureReturnsWrappedError(t *testing.T) {
	parent := t.TempDir()
	store := filepath.Join(parent, "notadir") // make the store path a regular FILE
	if err := os.WriteFile(store, []byte("x"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	seeded, err := setupStore(store, cfg)
	if err == nil {
		t.Fatalf("expected MkdirAll error; got (%v,nil)", seeded)
	}
	if seeded {
		t.Errorf("on error: seeded=true; want false")
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Errorf("config must NOT be written on MkdirAll failure; stat err=%v", err)
	}
}

// TestRunInitStoreWritesConfigCreatesStorePrintsPathExit0 — the full init dispatch: `init
// --store <tmp>` routes through run() → runInit, which resolves the store, creates+seeds
// it, writes the config, and reports. Asserts PRD §6.1 init row + §8.2 contract. WEAVE
// DELTA: the check report goes to STDOUT (item desc step 6), so stdout CONTAINS both the
// store path AND the check summary — NOT the skilldozer "exactly one line" assertion.
func TestRunInitStoreWritesConfigCreatesStorePrintsPathExit0(t *testing.T) {
	parent := t.TempDir()
	store := filepath.Join(parent, "newstore") // does NOT exist yet → assert setupStore CREATES it
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("weave_CONFIG", cfg)            // redirect the config write to a temp file (LOWERCASE env var)
	t.Setenv("weave_EXTENSIONS_DIR", "")     // ensure the config rule wins (env unset) (LOWERCASE)
	t.Chdir(t.TempDir())                     // escape the repo's walk-up rule (deterministic)

	var out, errOut bytes.Buffer
	code := run([]string{"init", "--store", store}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(init --store): code=%d; want 0; stderr=%q", code, errOut.String())
	}
	// store created (setupStore MkdirAll).
	if info, err := os.Stat(store); err != nil || !info.IsDir() {
		t.Errorf("store %q not created: stat err=%v", store, err)
	}
	// example.ts seeded at the store root.
	if _, err := os.ReadFile(filepath.Join(store, "example.ts")); err != nil {
		t.Errorf("seeded example.ts missing: %v", err)
	}
	// config written with store=<abs> (store is absolute; resolveStore's Abs is idempotent).
	f, err := configpkg.Load(cfg)
	if err != nil {
		t.Fatalf("configpkg.Load: %v", err)
	}
	if f.Store != store {
		t.Errorf("config.Store=%q; want %q", f.Store, store)
	}
	// §6.1 + §8.2 step 5: stdout carries the store path (one line) AND the check report.
	if !strings.Contains(out.String(), store) {
		t.Errorf("init stdout=%q; missing the store path %q", out.String(), store)
	}
	if !strings.Contains(out.String(), "extensions,") { // weave: check report on STDOUT
		t.Errorf("init stdout=%q; missing the check summary (report must be on stdout)", out.String())
	}
	// STDERR carries the seeded note + the found-via annotation.
	if !strings.Contains(errOut.String(), "Seeded example extension at") {
		t.Errorf("init stderr=%q; missing 'Seeded example extension at'", errOut.String())
	}
	if !strings.Contains(errOut.String(), "(found via config file)") {
		t.Errorf("init stderr=%q; missing '(found via config file)'", errOut.String())
	}
}

// TestRunBareTagUnconfiguredNeverPrompts — the §6.4/§8.2 prompt-safety regression guard.
// With a clean env (unsetExtEnv) and cwd in a temp dir, `weave sometag` must exit 1, print
// the one-line `run \`weave init\`` hint to stderr, write NOTHING to stdout, and NEVER reach
// resolveStore (c.init is false for tags, so no stdin access). stdout-empty is the §6.4
// $(...) contract; the never-prompt is structural (resolveStore is called only in runInit).
func TestRunBareTagUnconfiguredNeverPrompts(t *testing.T) {
	unsetExtEnv(t)   // weave_EXTENSIONS_DIR + weave_CONFIG → ghost temp paths (rules 1&2 miss)
	t.Chdir(t.TempDir()) // escape the repo's walk-up rule (rule 4 must miss too)

	var out, errOut bytes.Buffer
	code := run([]string{"sometag"}, &out, &errOut)
	if code != 1 {
		t.Errorf("run(sometag) unconfigured: code=%d; want 1", code)
	}
	if out.String() != "" { // §6.4: NOTHING on stdout so $(...) fails loudly
		t.Errorf("run(sometag) stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "run `weave init`") {
		t.Errorf("run(sometag) stderr=%q; missing the 'run `weave init`' hint", errOut.String())
	}
}
```

```bash
# After Task 5:
go test ./... -v
# Targeted re-run while debugging:
go test -run 'TestSetupStore|TestRunInit|TestRunBareTag' -v

# Expected: all green. On failure, the most common causes:
#   (a) TestRunInit stdout missing "extensions," → you rendered the check report to stderr
#       instead of stdout (re-read §6: weave puts it on stdout).
#   (b) TestSetupStoreNonEmpty fails on "example.ts must NOT be created" → you seeded into
#       a subdirectory or did not gate the seed on len(entries)==0.
#   (c) TestSetupStoreEmpty fails on byte mismatch → the exampleExtensionTemplate bytes
#       diverged from what was written (check for a stray trailing newline / indentation).
#   (d) compile error in check.Check → arg order (dir FIRST).
#   (e) TestRunBareTag stdout non-empty or exit != 1 → the dispatch leaked, or unsetExtEnv
#       was not called so a real config leaked a dir.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and exercise init end to end against a throwaway config (do NOT pollute
# the real ~/.config/weave/config.yaml — redirect via weave_CONFIG).
go build -o /tmp/weave .

export weave_CONFIG=/tmp/weave-test-config.yaml
rm -f "$weave_CONFIG"
rm -rf /tmp/weave-test-store

# (a) init --store <dir> (non-interactive, CI shape): creates, seeds, writes config, exit 0.
/tmp/weave init --store /tmp/weave-test-store
echo "init exit=$?"
echo "--- store contents (expect example.ts) ---"
ls -1 /tmp/weave-test-store
echo "--- config (expect 'store: /tmp/weave-test-store') ---"
cat "$weave_CONFIG"
echo "--- stdout had the store path + check report; stderr had Seeded + found-via ---"

# (b) re-run init on the now-non-empty store: adopts, NO clobber, exit 0.
/tmp/weave init --store /tmp/weave-test-store 2>&1 | grep -E "Adopted|Seeded"
echo "--- example.ts still byte-identical (idempotent) ---"
head -3 /tmp/weave-test-store/example.ts

# (c) bare tag under a clean env never prompts, writes nothing to stdout, exit 1.
env -u weave_EXTENSIONS_DIR weave_CONFIG=/tmp/nonexistent.yaml /tmp/weave init </dev/null >/tmp/o.txt 2>/tmp/e.txt; echo "init /dev/null exit=$?"
# (interactive prompt gated on isatty(stdin); /dev/null yields EOF → accepts default, no hang)
cd /tmp && env -u weave_EXTENSIONS_DIR weave_CONFIG=/tmp/nonexistent.yaml /tmp/weave sometag >/tmp/o.txt 2>/tmp/e.txt; echo "bare tag exit=$? (want 1)"; echo "stdout=[$(cat /tmp/o.txt)] (want empty)"; cat /tmp/e.txt

rm -f /tmp/weave /tmp/weave-test-config.yaml
rm -rf /tmp/weave-test-store /tmp/o.txt /tmp/e.txt
# Expected: (a) exit 0, store has example.ts, config has the store path; (b) "Adopted",
#   example.ts unchanged; (c) bare tag exit 1, empty stdout, the run `weave init` hint on stderr.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Static proofs for the two load-bearing weave deltas + prompt-safety.

echo "--- (1) seed target is example.ts at the ROOT (no example/ subdir) ---"
awk '/^func setupStore/,/^}/' main.go | grep -n "example.ts\|example\"" && echo "OK" || echo "FAIL"
awk '/^func setupStore/,/^}/' main.go | grep -n 'MkdirAll(exampleDir\|filepath.Join(store, "example")' \
  && echo "FAIL: found a subdir seed (skilldozer pattern)" || echo "OK: no example/ subdir"

echo "--- (2) check.Check called with dir FIRST ---"
grep -n "check.Check(" main.go
# Expected: check.Check(dir, exts) in BOTH the standalone check branch AND runInit.

echo "--- (3) runInit renders the check report to STDOUT (the `w` arg is stdout) ---"
awk '/^func runInit/,/^}/' main.go | grep -nE 'Fprintf\(stdout, "%-5s|Fprintf\(stdout, "%d extensions' \
  && echo "OK: report on stdout" || echo "FAIL: report not on stdout"
awk '/^func runInit/,/^}/' main.go | grep -nE 'Fprintf\(stderr, "%-5s' \
  && echo "FAIL: report lines on stderr (skilldozer pattern)" || echo "OK: no report lines on stderr"

echo "--- (4) prompt-safety: resolveStore is called ONLY inside runInit (never in the tag path) ---"
grep -n "resolveStore(" main.go
# Expected: exactly TWO hits — the func definition and runInit's call. NONE in the tag branch.

echo "--- (5) init dispatch is after version, before --path ---"
grep -n "if c.version\|if c.init\|// 2) --path\|if c.path" main.go
# Expected line order: if c.version  <  if c.init  <  // 2) --path  <  if c.path

echo "--- (6) exampleExtensionTemplate has ZERO backticks (single raw literal) ---"
awk '/const exampleExtensionTemplate = `/{f=1} f{print} f&&/^`$/{f=0}' main.go | grep -c '`'
# Expected: 0 (the opening/closing backticks are the delimiters, not content; awk captures
# only the content lines). If non-zero, the body changed and now needs splice form.

echo "--- (7) no stale skilldozer strings ---"
grep -ni "skilldozer\|SKILL.md\|skillsdir\|keep your skills" main.go && echo "FAIL: leftover skilldozer" || echo "OK"

# Expected: all seven checks OK.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l main.go main_test.go` is empty.
- [ ] `go test ./... -v` — all tests pass (existing + 6 new).
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged); no new imports in main.go.

### Feature Validation

- [ ] `const exampleExtensionTemplate` equals the PRD §11 body byte-for-byte (zero backticks → single raw literal).
- [ ] `setupStore` seeds `store/example.ts` at the ROOT (NO `example/` subdir; single MkdirAll).
- [ ] `setupStore` adopts a non-empty store in place (never clobbers; `seeded=false`); writes config either way.
- [ ] `runInit` calls `resolveStore(c.initStore)` → `configpkg.Path()` → `setupStore(store, cfgPath)`.
- [ ] `runInit` prints "Seeded example extension at <store>/example.ts" | "Adopted existing store at <store>" to STDERR.
- [ ] `runInit` prints the effective store path to STDOUT and "(found via <src>)" to STDERR.
- [ ] `runInit` renders the check report to STDOUT (mirrors the `if c.check` branch; `check.Check(dir, exts)` — dir FIRST).
- [ ] `runInit` exits 0 once create+config succeed; check findings never change the exit code.
- [ ] `run()` dispatches `if c.init { return runInit(c, stdout, stderr) }` AFTER `if c.version`, BEFORE the normal-mode ladder.
- [ ] The bare-tag path never reaches resolveStore (exit 1 + hint + empty stdout when unconfigured).

### Code Quality Validation

- [ ] ZERO new imports (S1 owns the import block; S2 inherits `bufio`/`configpkg`/`os`/`filepath`/`fmt`/`io`/`extdir`/`discover`/`check` + resolveStore).
- [ ] `configpkg` alias used for all `internal/config` references (never bare `config.`).
- [ ] Error wrappers use the `"weave init: <step>: %w"` prefix (NOT "skilldozer init:").
- [ ] No `expandHome` added (out of scope; S1 explicitly defers it).
- [ ] No `parseArgs` change; no `internal/*` change; no new files; no `go.mod`/`go.sum` change.
- [ ] The `exampleExtensionTemplate` doc comment cites PRD §11/§17 and notes the M6.T1.S1 repo-asset byte-match contract.

### Documentation & Deployment

- [ ] `setupStore`/`runInit` doc comments cite PRD §8.2 and explain their role (create/seed/write; orchestrate+report).
- [ ] `runInit`'s doc comment notes the stdout check-report choice and why (weave reproduces its own `--path`+`check`; diverges from skilldozer).
- [ ] No README/docs changes (init behavior is documented in README §3 in the final M6.T4/M6.T5 doc sweep — this task's item desc says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't seed `store/example/SKILL.md` in a subdirectory — weave seeds `store/example.ts`, a
  SINGLE FILE at the store ROOT. There is NO `exampleDir` and NO second `MkdirAll`. This is
  the #1 skilldozer-port trap (skilldozer's setupStore creates `store/example/` then writes
  `SKILL.md` inside it).
- ❌ Don't call `check.Check(exts)` — weave's signature is `check.Check(dir, exts)`, DIR FIRST.
  Swapping the args is a compile error (string vs []Extension), so it fails loudly at build —
  but only if you actually build. Do not copy skilldozer's `check.Check(skills)` verbatim.
- ❌ Don't render runInit's check report to STDERR — the weave item description step 6 says
  STDOUT. skilldozer put it on stderr; weave diverges (its standalone `check` is also stdout,
  and PRD §8.2 step 5 under weave's conventions = dir→stdout + report→stdout). The
  TestRunInit assertion keys off the stdout check summary, so a stderr mistake fails the test.
- ❌ Don't import `internal/config` WITHOUT the `configpkg` alias — main.go ~L77's
  `type config struct` collides → compile error. S1 lands the alias; S2 must use `configpkg.`.
- ❌ Don't touch the import block — S1 owns it (it adds `bufio` + `configpkg`); S2 needs ZERO
  new imports. Editing it risks a merge conflict with S1's still-landing edit.
- ❌ Don't add `expandHome` — S1's PRP explicitly defers it; weave's resolveStore absolutizes
  via filepath.Abs only. Do NOT port skilldozer's `TestRunInitStoreTildeExpandsHome`.
- ❌ Don't splice backticks into `exampleExtensionTemplate` — the PRD §11 body has ZERO
  backticks (verified), so a single raw-string literal works. The `+ "\`" +` splice is
  skilldozer's pattern (its body has 8 backticks); copying it here is wrong.
- ❌ Don't place the `if c.init` dispatch at the TOP of run() (before `if c.version`) — §6.3
  precedence says --version wins over init. Place it AFTER `if c.version` and BEFORE the
  normal-mode ladder. M5's exclusivityError will later slot in between version and init.
- ❌ Don't make runInit's check report an exit-code gate — check findings NEVER change init's
  exit code (item description: "check findings don't change init exit code"). Exit 0 once
  create+config succeed, regardless of check findings.
- ❌ Don't call `extdir.Find()` BEFORE `setupStore` — call it AFTER, so the just-written
  config.yaml is visible to the config rule and `dir == store` / `src == "config file"`.
- ❌ Don't modify `parseArgs`, the bare-tag branch, any `internal/*` package, `go.mod`, or
  PRD.md — this task is one const + two funcs + one dispatch line + 6 tests.

---

**Confidence Score: 9/10** for one-pass success. The three functions are a near-verbatim
port of skilldozer's shipped, tested `main.go` (L969-1134, read in full), and the 6 tests
are a near-verbatim port of skilldozer's `main_test.go` (L2472-2700). The FOUR weave-specific
deltas (seed `example.ts` at root not `example/SKILL.md` subdir; `check.Check(dir, exts)` dir
first; check report → stdout not stderr; no expandHome) are each pinned to an exact location
and verified against the live weave codebase (the `if c.check` branch confirms the rendering;
check.go confirms the arg order; config.go confirms the `configpkg` API). The consumed APIs
(`resolveStore` from S1; `configpkg.Path/Save/Load/File`; `extdir.Find`; `discover.Index`;
`check.Check` + Report types; `relTagBase`) are all confirmed exported. The example.ts body
is transcribed byte-exact with zero backticks verified. The ONE residual risk is the
parallel-sibling dependency on S1 (resolveStore + the configpkg/bufio imports must land as
specified) — mitigated by treating S1's PRP as a hard contract and by `go build` catching any
missing symbol immediately. The second residual risk is the deliberate stdout-vs-stderr
divergence from skilldozer for the check report — explicitly pinned in the contract, the test,
and the static-proof validation, so a regression is caught.
