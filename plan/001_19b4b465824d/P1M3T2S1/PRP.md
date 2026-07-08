# PRP — P1.M3.T2.S1: Tag resolution loop + `extensionPath()` + atomicity + modifiers

## Goal

**Feature Goal**: Wire the last piece of Mode B path-emission into `main.go`: a
`func extensionPath(ext discover.Extension, extensionsDir string, c config) string`
helper (the weave analog of skilldozer's `skillPath`) plus the `--all` branch and
the `<tag>`-resolution branch in `run()`. This makes `weave <tag>`, `weave -f <tag>`,
`weave --all`, `weave --relative <tag>`, and their combinations work **end-to-end**
— `discover.Index(dir)` produces the extensions, `resolve.Resolve(tag, exts)`
maps tags to a single `discover.Extension`, and this task prints the right path
field per the PRD §6.1/§6.2 contract, honoring PRD §6.4 atomicity (any failing
tag ⇒ nothing on stdout, one stderr line per problem, exit 1).

**Deliverable**: MODIFY TWO existing files (no new files, no new packages, no
new deps):
- `main.go` — add `import "path/filepath"` + `import resolve pkg`; add the
  `extensionPath` function; add the `--all` and `if len(c.tags) > 0` branches in
  `run()` (before the final no-op `return 0`).
- `main_test.go` — add ~20 test functions ported from skilldozer's `main_test.go`
  (lines 460–870) with the Skill→Extension noun swap + the weave env-var
  (`weave_EXTENSIONS_DIR`) + the weave `.ts`-file store layout, PLUS two NEW
  weave-specific tests for `--file` on dir/package extensions (the one case
  skilldozer's `skillPath` could not exercise, because skilldozer's SourceFile was
  always `Dir+"/SKILL.md"`).

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- `go test ./... -v` — ALL tests pass (existing M1/M2 tests still green; the new
  ~20 tag/--all tests all pass).
- `go test -race ./...` passes.
- `weave example` (on a store with `example.ts`) prints the absolute file path +
  newline, exit 0.
- `weave -f example` (single-file) prints the SAME path (no-op for file kind);
  `weave -f git-checkpoint` (dir) prints `…/git-checkpoint/index.ts`;
  `weave -f summarizer` (package) prints `…/summarizer/src/index.ts`.
- `weave badtag` → empty stdout, one stderr line `unknown extension tag "badtag"`,
  exit 1.
- `weave good bad` → empty stdout, exit 1 (atomicity: good does NOT leak).
- `weave --all` → one absolute path per line, sorted by tag, exit 0 (even for an
  empty store).
- All emitted paths are absolute (start with `/` on Unix, or a drive letter on
  Windows) unless `--relative` is given.

## User Persona (if applicable)

**Target User**: A developer or script using `pi -e "$(weave <tag>)"` to load an
extension into pi by tag. The correctness of that command-substitution depends
ENTIRELY on this task: the path must be absolute (pi needs an absolute/anchored
path), and a bad tag must produce an EMPTY stdout + non-zero exit so pi fails
loudly instead of loading a garbage path.

**Use Case**:
```
pi -e "$(weave example)"            # → pi -e "/home/me/extensions/example.ts"
pi -e "$(weave -f summarizer)"      # → pi -e "/.../summarizer/src/index.ts"
for t in reddit-poster example; do pi -e "$(weave $t)"; done   # bulk
```

**User Journey**:
1. User runs `weave reddit-poster`.
2. `main` → `run` → `parseArgs` captures `c.tags=["reddit-poster"]`.
3. `run`'s tag branch: `extdir.Find()` → the store dir; `discover.Index(dir)` →
   `[]Extension` (incl. the nested `writing/reddit-poster.ts`).
4. `resolve.Resolve("reddit-poster", exts)` → `Result{Extension: ..., Match: Basename}`.
5. `extensionPath(ext, dir, c)` → `ext.Path` (default; absolute).
6. Loop buffers the path; no error; flush → stdout `<abs>/writing/reddit-poster.ts\n`; exit 0.

**Pain Points Addressed**:
- **Tag → path**: users think in tags (`reddit-poster`), not paths; this prints the path.
- **`$(...)` safety (PRD §6.4)**: a typo (`weave redit-poster`) must NOT pass a
  partial/garbage path to pi — atomicity + empty-on-failure is the contract.
- **Entry-file convenience**: `weave -f <tag>` saves the user from knowing whether
  an extension is a file/dir/package and where pi's entry file lives.

## Why

- **PRD §6.1 `weave <tag>` / `weave --all` and §6.2 modifiers are the core CLI
  contract**: this task is what makes those two table rows actually print paths.
  Everything before it (config, extdir, discover, resolve) is plumbing; this task
  is the faucet.
- **Near-verbatim port minimizes risk**: skilldozer's `run()` tag branch, `--all`
  branch, and `skillPath` are PROVEN and heavily tested (skilldozer main_test.go
  lines 460–870 cover single/multiple/atomic/ambiguous/unresolvable/absolute/
  duplicate, plus all modifier combos and all `--all` cases). Porting them with
  the documented single semantic change (`Dir`/`SourceFile` → `Path`/`EntryFile`)
  inherits all that coverage.
- **The ONE semantic change is load-bearing and well-specified**: in skilldozer
  `SourceFile` was always `Dir+"/SKILL.md"`; in weave `EntryFile` is
  kind-dependent (file→itself, dir→index.ts, package→first pi.extensions entry).
  But `extensionPath` does NOT branch on Kind — it just reads `ext.Path` (default)
  or `ext.EntryFile` (--file), both ALREADY populated by `discover.Index`. The
  kind-dependence is encoded IN THE DATA, not in this function. (See
  `research/port_mapping.md`.)
- **Atomicity is the script-safety contract**: buffering all paths and flushing
  only on full success is non-negotiable for `pi -e "$(weave a b c)"`.
- **Scope boundary**: this task does NOT add `--search`/`check`/`init`/`--help`/
  exclusivity/unknown-flag-exit-2 (those are M4/M5). It does NOT touch the parser
  (already complete from M1.T4.S1). It ONLY adds dispatch branches + one helper.

## What

### `extensionPath(ext discover.Extension, extensionsDir string, c config) string`

The shared formatter used by BOTH the `<tag>` loop and the `--all` loop (PRD §6.2
header: "modifiers combine with tag resolution or `--all`"). Precedence of effects:

| flags set              | output                                              |
|------------------------|-----------------------------------------------------|
| (neither)              | `ext.Path` — the resolvable path (abs). DEFAULT.    |
| `--file`               | `ext.EntryFile` — the `.ts`/`.js` pi loads (abs).   |
| `--relative`           | `filepath.Rel(extensionsDir, ext.Path)` — relative. |
| `--file --relative`    | `filepath.Rel(extensionsDir, ext.EntryFile)` — combines. |

Implementation (mirrors skilldozer `skillPath` exactly, only the field names change):
```go
func extensionPath(ext discover.Extension, extensionsDir string, c config) string {
	p := ext.Path           // default: absolute resolvable path (PRD §6.1/§6.2)
	if c.file {
		p = ext.EntryFile    // --file: the .ts/.js pi loads (PRD §6.2)
	}
	if c.relative {
		if rel, err := filepath.Rel(extensionsDir, p); err == nil {
			p = rel          // --relative: path relative to the extensions dir
		}
	}
	return p
}
```
The `filepath.Rel` err-guard is defensive only: both args are absolute and `ext.Path`
is always under `extensionsDir` (it was discovered by walking it), so Rel cannot
fail in practice. On a (theoretical) failure, fall back to the absolute path — still
a correct, usable answer rather than crashing.

### `--all` branch (in `run()`, before the tags branch)

```go
if c.all {
	dir, _, err := extdir.Find()
	if err != nil {
		fmt.Fprintln(stderr, err)   // ErrNotFound verbatim + newline (§6.4/§8)
		return 1
	}
	exts, err := discover.Index(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)   // e.g. dir vanished between Find and Index
		return 1
	}
	for _, e := range exts {
		fmt.Fprintln(stdout, extensionPath(e, dir, c))   // --file/--relative apply
	}
	return 0   // exit 0 even for empty store (PRD §6.1)
}
```
`discover.Index` already sorts `[]Extension` by `RelTag` (index.go), so the output
is sorted by tag with no extra work. Empty store → empty stdout, exit 0.

### Tag-resolution branch (in `run()`, after `--all`)

```go
if len(c.tags) > 0 {
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
	paths := make([]string, 0, len(c.tags))   // buffered; flushed ONLY if all resolve
	hadErr := false
	for _, tag := range c.tags {
		res, rerr := resolve.Resolve(tag, exts)
		if rerr != nil {
			fmt.Fprintln(stderr, rerr)   // one error line per problem tag (verbatim)
			hadErr = true
			continue
		}
		paths = append(paths, extensionPath(res.Extension, dir, c))
	}
	if hadErr {
		return 1   // buffered paths NEVER written → stdout empty (§6.4)
	}
	for _, p := range paths {
		fmt.Fprintln(stdout, p)   // one path per line, input order
	}
	return 0
}
```

### ATOMICITY (PRD §6.4 — the critical-for-`$(...)` contract)

Resolve EVERY tag first, buffering paths. If ANY tag fails (unknown/ambiguous):
print one error line per problem tag to stderr, print NOTHING to stdout, exit 1.
The buffered paths are flushed ONLY when the whole invocation is known-good. This
makes `pi -e "$(weave bad)"` fail loudly (empty `$(...)`, exit 1) instead of passing
a partial/garbage path. Each error is printed verbatim from `resolve`'s typed errors
(`*UnknownError`: `unknown extension tag "foo"`; `*AmbiguousError`:
`ambiguous extension tag "x" matches: a, b`) — NO `weave:` prefix, matching the
`extdir.ErrNotFound` convention used by `--path`/`--list`.

### Success Criteria

- [ ] `extensionPath` exists in `main.go` with the exact signature above.
- [ ] `run()` has an `if c.all` branch and an `if len(c.tags) > 0` branch, both
      placed before the final `return 0`, in that order.
- [ ] The `--all` branch iterates `exts` in Index order (sorted) and prints
      `extensionPath(e, dir, c)` per line; exit 0 even for an empty store.
- [ ] The tag branch buffers paths, sets `hadErr` on any error, prints one stderr
      line per problem tag, and returns 1 WITHOUT writing buffered paths.
- [ ] `--file` on a single-file ext is a no-op (EntryFile == Path); on a dir ext
      prints `index.ts`; on a package ext prints the first `pi.extensions` entry.
- [ ] `--relative` makes the chosen path relative to the extensions dir;
      `--file --relative` combines.
- [ ] All default/`--file` outputs are absolute; all `--relative` outputs are relative.
- [ ] `main.go` imports `"path/filepath"` and `internal/resolve`; no unused imports.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes** — the verbatim port source is at a known
absolute path (`/home/dustin/projects/skilldozer/main.go`), the exact edits are
spelled out in `research/port_mapping.md` (every weave substitution listed), the
input/output types are fully documented (`discover.Extension` in
`internal/discover/extension.go`; `resolve.Result.Extension` per the P1.M3.T1.S1
PRP contract), the env-var name and test fixtures are in
`research/test_conventions.md`, and the validation commands are project-standard.
No guessing required.

### Documentation & References

```yaml
# MUST READ — the verbatim port source (read the THREE pieces in full)
- file: /home/dustin/projects/skilldozer/main.go
  why: contains skillPath(), the --all branch, and the tag-resolution branch — the
       three things to port. Located at: skillPath near the bottom; the two run()
       branches between the --list/search/check branches and the no-args tail.
  pattern: shared formatter (skillPath) used by BOTH loops; buffered-paths atomicity;
           verbatim typed-error printing via fmt.Fprintln(stderr, rerr).
  gotcha: the ONLY semantic change is s.Dir→ext.Path and s.SourceFile→ext.EntryFile.
          In skilldozer SourceFile was always Dir+"/SKILL.md"; in weave EntryFile is
          kind-dependent BUT extensionPath does NOT switch on Kind — the fields are
          already populated by discover.Index. See research/port_mapping.md.

- file: /home/dustin/projects/skilldozer/main_test.go
  why: lines 460–870 are the tests to port (TestRunTag*, TestRunAll*, modifier tests).
       They cover single/multiple/atomic/ambiguous/unresolvable/absolute/duplicate +
       all modifier combos + all --all cases.
  pattern: writeSkillTree + sampleStore fixtures; t.Setenv(SKILLDOZER_SKILLS_DIR);
           bytes.Buffer capture; strings.Split(TrimRight(...)) line checks.
  gotcha: env var is weave_EXTENSIONS_DIR (lowercase) in weave, NOT SKILLDOZER_SKILLS_DIR.
          writeExtTree (weave) writes .ts FILES, not <tag>/SKILL.md dirs — so the
          default output is the FILE path (.../example.ts), not a dir. And --file on a
          single-file ext is a NO-OP (EntryFile==Path), unlike skilldozer where it
          appended /SKILL.md. See research/test_conventions.md.

# The input types
- file: internal/discover/extension.go
  why: defines discover.Extension — Path, EntryFile, RelTag, Kind. extensionPath
       reads Path/EntryFile; the tag loop passes res.Extension. Confirms field
       names/types so the port compiles.
  pattern: Extension is BUILT by discover.Index (never unmarshaled); Path is the
           resolvable path (file OR dir); EntryFile is the .ts/.js pi loads.
  gotcha: for a single-file ext, Path == EntryFile (both the .ts file). For a dir
          ext, Path is the dir and EntryFile is dir+"/index.ts". For a package ext,
          Path is the dir and EntryFile is the first pi.extensions entry. extensionPath
          does NOT care — it reads whichever field the flags select.

- file: internal/discover/index.go
  why: discover.Index(dir) returns []Extension SORTED by RelTag (sort.Slice at the
       end). Confirms the --all output is already sorted — no extra sort needed.
  pattern: Index calls filepath.Abs(root) first, so every Path/EntryFile is absolute.
  gotcha: empty store → (nil, nil); --all must print nothing and exit 0 (NOT 1).

# The CONSUMED resolve package (P1.M3.T1.S1 — being implemented in parallel)
- file: plan/001_19b4b465824d/P1M3T1S1/PRP.md
  why: defines the resolve.Resolve signature + Result.Extension + UnknownError +
       AmbiguousError that this task consumes. THIS IS A CONTRACT — assume it lands
       exactly as specified.
  pattern: resolve.Resolve(tag string, exts []discover.Extension) (Result, error);
           Result{Extension discover.Extension, Match MatchKind}; errors print via
           fmt.Fprintln(stderr, rerr) (their .Error() is the right text).
  gotcha: res.Extension (NOT res.Skill). Match is unused by this task (we only need
          the Extension); do not branch on Match.Kind.

# The current main.go (what we MODIFY)
- file: main.go
  why: contains parseArgs (COMPLETE — do not touch), run() (add 2 branches), version,
       isTerminal, the --version/--path/--list dispatch. Shows where to insert the
       new branches (before the final `return 0`) and what imports exist.
  pattern: run() returns int (exit code); stdout/stderr injected; each branch is
           self-contained (Find → Index → render → return).
  gotcha: the final `return 0` is the "no recognized mode" path — M5.T1.S1 turns it
          into usage→stderr→exit 1. For NOW it stays `return 0`; our new branches go
          ABOVE it. Do not move or delete it.

# extdir.Find (the store locator)
- file: internal/extdir/extdir.go
  why: extdir.Find() returns (dir string, src Source, err error). On miss err is
       extdir.ErrNotFound whose .Error() is `weave is not configured; run \`weave init\``.
  pattern: the 2nd return (src) is DISCARDED by --all/--tags (only --path prints it).
  gotcha: env var is weave_EXTENSIONS_DIR (lowercase). Tests drive it via t.Setenv.

# PRD spec (authoritative)
- docfile: PRD.md
  section: §6.1 (Commands/flags table — weave <tag>, weave --all rows)
  why: pins stdout format (one absolute path per line, input order for tags; sorted
       for --all), exit codes (0 all resolve / 1 any fail; --all always 0).
- docfile: PRD.md
  section: §6.2 (Modifiers — --file, --relative)
  why: pins the per-kind --file behavior and the --relative + combine semantics.
- docfile: PRD.md
  section: §6.4 (Error semantics — critical for $(...) use)
  why: the atomicity contract: any unresolved/ambiguous tag ⇒ one stderr line per
       problem, NOTHING on stdout, exit 1.
- docfile: PRD.md
  section: §7.1 (Discovery — path/entryFile/relTag/kind field definitions)
  why: defines what Path and EntryFile mean per Kind (the table in the What section).

# Architecture mapping
- docfile: plan/001_19b4b465824d/architecture/architecture_mapping.md
  section: the main.go / Mode B entry (§5 Mode B)
  why: confirms main.go is the wiring point and that --all/--tags/--file/--relative
       land in this milestone.
```

### Current Codebase tree (relevant subset)

```bash
go.mod                          # module github.com/dabstractor/weave, go 1.25
main.go                         # MODIFY: +import filepath,resolve; +extensionPath; +2 run() branches
main_test.go                    # MODIFY: +~20 tests (port skilldozer 460-870 + 2 new dir/pkg --file tests)
internal/
├── config/                     # M1.T2 (done) — not used here
├── extdir/                     # M1.T3 (done) — extdir.Find() consumed
├── discover/
│   ├── extension.go            # Extension struct (Path/EntryFile/RelTag/Kind) — consumed
│   ├── jsdoc.go                # M2.T1.S2 (done)
│   ├── discover.go             # classifyFile/classifyDir (M2.T2)
│   └── index.go                # Index() → []Extension sorted by RelTag — consumed
├── resolve/                    # M3.T1.S1 (BEING IMPLEMENTED IN PARALLEL) — resolve.Resolve consumed
│   ├── resolve.go
│   └── resolve_test.go
└── ui/                         # M2.T4 (done) — not used by --all/--tags (those print paths, not tables)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Both targets are MODIFICATIONS:
main.go           # +extensionPath(); +--all branch; +tag-resolution branch; +2 imports
main_test.go      # +sampleStore, writeDirExt, writePkgExt helpers; +~20 TestRunTag*/TestRunAll* tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: weave_extensions_DIR is LOWERCASE (extdir.go:74 const envVar).
// Tests use t.Setenv("weave_EXTENSIONS_DIR", dir) — NOT SKILLDOZER_SKILLS_DIR.

// CRITICAL: weave extensions are .ts FILES (single-file kind), NOT <tag>/SKILL.md
// dirs as in skilldozer. So writeExtTree writes <tag>.ts and the DEFAULT output
// of `weave <tag>` is the FILE path (.../example.ts), not a directory.

// CRITICAL: --file on a single-file ext is a NO-OP (EntryFile == Path). This is
// the OPPOSITE of skilldozer, where --file appended /SKILL.md. The test
// TestRunTagFileOnSingleFileIsNoOp must assert the SAME path as the default,
// not .../example.ts/SKILL.md.

// CRITICAL: extensionPath does NOT switch on ext.Kind. Path and EntryFile are
// already populated by discover.Index per Kind. Reading the field the flag
// selects is the entire logic. Adding a `switch ext.Kind` is an anti-pattern.

// CRITICAL: the tag-resolution branch must BUFFER paths and flush only on full
// success. Do NOT print each path inline as it resolves — a later failing tag
// would leak earlier successes to stdout, violating §6.4.

// CRITICAL: errors are printed with fmt.Fprintln(stderr, rerr) — NO "weave:"
// prefix. The typed errors' .Error() is already the complete, correct line
// (`unknown extension tag "foo"`, `ambiguous extension tag "x" matches: a, b`).
// A prefix would double up with the existing extdir.ErrNotFound convention.

// GOTCHA: res.Extension (NOT res.Skill) — the P1.M3.T1.S1 contract names the
// field Extension. Match is unused here; do not branch on it.

// GOTCHA: the 2nd return of extdir.Find() (src Source) is DISCARDED by --all and
// the tag branch. Only --path prints "(found via <src>)". Assign to `_, _, err`
// or `dir, _, err` — do not capture src.

// GOTCHA: --all on an EMPTY store prints nothing and exits 0 (PRD §6.1: --all is
// ALWAYS 0). This DIFFERS from --list (which exits 1 "if no extensions found").
// Do not copy --list's empty-check into --all.

// GOTCHA: filepath.Rel uses the OS path separator. Tests comparing --relative
// output must use filepath.FromSlash("writing/reddit-poster") not the literal
// slash string, or they fail on Windows. (Existing tests already do this.)

// GOTCHA: the final `return 0` at the end of run() is the "no recognized mode"
// path. M5.T1.S1 will turn it into usage→stderr→exit 1. For THIS task, leave it
// as `return 0` and insert the new branches ABOVE it. Do not move/delete it.
```

## Implementation Blueprint

### Data models and structure

No new data models. This task consumes `discover.Extension` (Path/EntryFile fields)
and `resolve.Result` (Extension field) and `config` (file/relative/all/tags fields,
all already populated by the complete parser). `extensionPath` is a pure function
over its three args.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY main.go — add imports
  - ADD to the import block: "path/filepath"
  - ADD to the import block: "github.com/dabstractor/weave/internal/resolve"
  - PLACEMENT: the existing import block at the top of main.go (keep alpha-ish order
    matching the existing style: stdlib first, then internal/*).
  - VERIFY: go build ./... must still compile (resolve package must exist — it is
    being built in parallel by P1.M3.T1.S1; if it is not yet present, this build
    will fail until that lands — that is expected and correct).

Task 2: MODIFY main.go — add extensionPath()
  - ADD: func extensionPath(ext discover.Extension, extensionsDir string, c config) string
  - PORT FROM: /home/dustin/projects/skilldozer/main.go func skillPath (read in full)
  - EDITS: s discover.Skill → ext discover.Extension; s.Dir → ext.Path;
    s.SourceFile → ext.EntryFile; param skillsDir → extensionsDir; doc comment
    noun swap ("skill directory"→"resolvable path", "SKILL.md"→"entry file").
  - PLACEMENT: near the bottom of main.go, after run() and before any init helpers
    (there are none yet — M4.T4 adds them). Match skilldozer's placement (skillPath
    sits right after run()).
  - FILES TOUCHED: 1 (main.go).

Task 3: MODIFY main.go — add the --all branch in run()
  - INSERT: the `if c.all { ... }` block (see the What section) into run(), AFTER
    the existing `if c.list { ... }` branch and BEFORE the final `return 0`.
  - PORT FROM: skilldozer run() `if c.all` branch (read in full).
  - EDITS: skillsdir.Find() → extdir.Find(); skills → exts; skillPath(s, dir, c)
    → extensionPath(e, dir, c).
  - LOGIC: Find → Index → loop exts (Index order = sorted by RelTag) → print
    extensionPath per line → return 0. Empty store → no iterations → empty stdout,
    return 0.
  - FILES TOUCHED: 1 (main.go).

Task 4: MODIFY main.go — add the tag-resolution branch in run()
  - INSERT: the `if len(c.tags) > 0 { ... }` block (see the What section) into
    run(), AFTER the `if c.all` branch and BEFORE the final `return 0`.
  - PORT FROM: skilldozer run() `if len(c.tags) > 0` branch (read in full).
  - EDITS: skillsdir.Find() → extdir.Find(); skills → exts; resolve.Resolve(tag,
    skills) stays (signature identical); res.Skill → res.Extension; skillPath →
    extensionPath.
  - LOGIC: Find → Index → buffer loop (resolve each tag; on err print stderr line +
    hadErr=true + continue; on success buffer extensionPath) → if hadErr return 1
    (buffered paths NEVER written) → else flush paths in input order → return 0.
  - FILES TOUCHED: 1 (main.go).

Task 5: VALIDATE main.go compiles + vets
  - RUN: go build ./... ; go vet ./...
  - EXPECT: clean. If resolve package not yet present (parallel task), wait for it
    then rebuild. Most likely failure: a stale skilldozer import path or a missed
    Skill→Extension/res.Skill→res.Extension swap.

Task 6: MODIFY main_test.go — add fixtures + tag-resolution tests
  - ADD helper: sampleStore(t) — writeExtTree with example + writing/reddit-poster.
  - ADD helper: writeDirExt(t, root, tag, desc) — <tag>/index.ts (see research).
  - ADD helper: writePkgExt(t, root, tag, name) — <tag>/package.json + src/index.ts.
  - ADD tests (port from skilldozer main_test.go 460-700, with noun/env/path swaps):
      TestRunTagSingleResolvesToPath, TestRunTagMultipleInInputOrder,
      TestRunTagAtomicityUnknownPrintsNothing, TestRunTagAllFailMultipleErrorLines,
      TestRunTagDuplicateArgResolvesTwice, TestRunTagAmbiguousListsCandidates,
      TestRunTagUnresolvable, TestRunTagPathIsAbsolute,
      TestRunVersionPrecedenceOverTag,
      TestRunTagFileOnSingleFileIsNoOp (renamed from PrintsSourceFile),
      TestRunTagRelativePrintsRelativePath, TestRunTagFileRelativeCombine,
      TestRunTagFileAtomicity.
  - NAMING: TestRunTag<Scenario> (mirror skilldozer; the noun is already "tag",
    not "skill", so most names are identical).
  - ENV: t.Setenv("weave_EXTENSIONS_DIR", dir) for every test; unsetExtEnv(t) for
    the unresolvable case.
  - ASSERTIONS: output is ext.Path (the .ts FILE, absolute); --file on single-file
    is the SAME path; --relative uses filepath.FromSlash; atomicity = empty stdout.
  - PLACEMENT: after the existing TestRunList* block in main_test.go.
  - FILES TOUCHED: 1 (main_test.go).

Task 7: MODIFY main_test.go — add the 2 NEW dir/package --file tests
  - ADD: TestRunTagFileOnDirExtPrintsIndexTS — build a dir ext (writeDirExt:
    git-checkpoint/index.ts), `weave -f git-checkpoint` → .../git-checkpoint/index.ts.
  - ADD: TestRunTagFileOnPkgExtPrintsPiExtensionsEntry — build a package ext
    (writePkgExt: summarizer/package.json with pi.extensions→["./src/index.ts"] +
    src/index.ts), `weave -f summarizer` → .../summarizer/src/index.ts.
  - WHY NEW: skilldozer had no analog (its SourceFile was always Dir+"/SKILL.md");
    these are the ONE case where weave's EntryFile diverges per Kind. Confirms the
    PRD §6.2 row "--file on dir ext → index.ts; on package ext → first pi.extensions
    entry."
  - PLACEMENT: right after TestRunTagFileOnSingleFileIsNoOp (group the --file tests).
  - FILES TOUCHED: 1 (main_test.go).

Task 8: MODIFY main_test.go — add --all tests
  - ADD tests (port from skilldozer main_test.go 751-870):
      TestRunAllPrintsAllSorted, TestRunAllShortFlag,
      TestRunAllFilePrintsAllEntryFiles (single-file store: EntryFile==Path),
      TestRunAllRelativePrintsAllRelative, TestRunAllEmptyStoreExit0,
      TestRunAllUnresolvable, TestRunVersionPrecedenceOverAll.
  - NAMING: TestRunAll<Scenario>.
  - GOTCHA: TestRunAllFilePrintsAllEntryFiles on a single-file store asserts the
    SAME paths as TestRunAllPrintsAllSorted (because EntryFile==Path for file kind).
    To make it MEANINGFUL, build a MIXED store (a single-file + a dir ext) so
    --all vs --all --file actually differ — OR keep it single-file and document that
    it asserts the no-op property. Prefer the mixed-store version for real coverage.
  - PLACEMENT: after the TestRunTag* block.
  - FILES TOUCHED: 1 (main_test.go).

Task 9: VALIDATE — full sweep
  - RUN: go build ./... ; go vet ./... ; go test ./... -v ; go test -race ./...
  - EXPECT: all green. On failure, READ the output — the failure is almost always
    a noun/path swap miss (res.Skill vs res.Extension), an env-var typo
    (SKILLDOZER_ vs weave_), or an assertion that forgot single-file --file is a
    no-op (expecting .../example.ts/SKILL.md).
```

### Implementation Patterns & Key Details

```go
// extensionPath — the weave analog of skilldozer's skillPath. The ONLY semantic
// change: ext.Path/ext.EntryFile are the resolvable/entry paths (kind-dependent),
// not s.Dir/s.SourceFile (always dir / dir+SKILL.md). extensionPath does NOT
// switch on Kind — the fields are already populated per Kind by discover.Index.
func extensionPath(ext discover.Extension, extensionsDir string, c config) string {
	p := ext.Path           // default: absolute resolvable path (PRD §6.1/§6.2)
	if c.file {
		p = ext.EntryFile    // --file: the .ts/.js pi loads (PRD §6.2)
	}
	if c.relative {
		if rel, err := filepath.Rel(extensionsDir, p); err == nil {
			p = rel          // --relative: relative to the extensions dir
		}
	}
	return p
}

// The tag-resolution loop body (ported verbatim; only Discover/Resolve swaps).
// ATOMICITY: buffer first, flush only on full success.
paths := make([]string, 0, len(c.tags))
hadErr := false
for _, tag := range c.tags {
	res, rerr := resolve.Resolve(tag, exts)
	if rerr != nil {
		fmt.Fprintln(stderr, rerr)   // verbatim typed-error text; NO "weave:" prefix
		hadErr = true
		continue
	}
	paths = append(paths, extensionPath(res.Extension, dir, c))  // res.Extension, NOT res.Skill
}
if hadErr {
	return 1                       // buffered paths NEVER written → stdout empty (§6.4)
}
for _, p := range paths {
	fmt.Fprintln(stdout, p)
}
return 0
```

### Integration Points

```yaml
# This task INTEGRATES the three already-landed pieces into main.go's run():
CONSUMES:
  - extdir.Find()              # M1.T3 — (dir, src, err); src discarded here
  - discover.Index(dir)        # M2.T3 — []Extension sorted by RelTag
  - resolve.Resolve(tag, exts) # M3.T1.S1 — (Result{Extension, Match}, error)
  - config (parseArgs output)  # M1.T4 — c.all, c.file, c.relative, c.tags

PRODUCES (for later milestones):
  - extensionPath() helper     # M4 (search/check) do NOT use it (they print tables/reports);
                                #   but M5 exclusivity + the final --help will coexist with these branches.

NO CHANGES TO:
  - go.mod / go.sum (stdlib path/filepath + internal/resolve only)
  - internal/* (all consumed as-is)
  - parseArgs (COMPLETE since M1.T4 — do not touch)
  - the --version/--path/--list branches (already dispatched)
  - the final `return 0` no-op tail (M5 owns its → usage/exit-1 transformation)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Tasks 1-4 (main.go edits):
go build ./...          # compiles main + all internal packages
go vet ./...            # clean (catches the Printf format / unreachable issues)

# Expected: zero errors. If the resolve package is not yet present (parallel
# P1.M3.T1.S1 still building), `go build` will fail on the new import — that is
# EXPECTED and resolves the moment P1.M3.T1.S1 lands. Do NOT work around it by
# stubbing resolve; the contract says it will exist.
```

### Level 2: Unit Tests (Component Validation)

```bash
# After Tasks 6-8 (main_test.go edits):
go test ./... -v                 # all packages, including main (white-box, package main)
go test -race ./...              # race sweep (run is single-goroutine, but -race is free insurance)

# Targeted re-runs while debugging a single failure:
go test -run 'TestRunTag' . -v
go test -run 'TestRunAll' . -v
go test -run 'TestRunTagFileOn' . -v   # the 3 --file tests (file/dir/pkg)

# Expected: all tests pass. On failure, the cause is almost always one of:
#   (a) res.Skill vs res.Extension (compile error — go build catches it)
#   (b) SKILLDOZER_SKILLS_DIR vs weave_EXTENSIONS_DIR in a copied test (env miss →
#       Find returns ErrNotFound → wrong exit/stderr)
#   (c) asserting .../example.ts/SKILL.md for --file on a single-file ext (weave's
#       --file is a no-op for file kind — EntryFile == Path)
#   (d) a --relative assertion using a literal "/" instead of filepath.FromSlash
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary:
go build -o /tmp/weave .

# Set up a tiny store:
STORE=$(mktemp -d)
cat > "$STORE/example.ts" <<'EOF'
/** Reference example extension. */
export default function() {}
EOF
mkdir -p "$STORE/writing"
cat > "$STORE/writing/reddit-poster.ts" <<'EOF'
/** Posts to reddit. */
export default function() {}
EOF
mkdir -p "$STORE/git-checkpoint"
cat > "$STORE/git-checkpoint/index.ts" <<'EOF'
/** Git checkpoint extension. */
export default function() {}
EOF

# Smoke-test the §6.1 contract end-to-end:
export weave_EXTENSIONS_DIR="$STORE"

weave_arg() { /tmp/weave "$@"; }   # helper

# 1. Single tag → absolute file path, exit 0.
out=$(/tmp/weave example); code=$?
echo "example → $out (exit $code)"
test "$code" = 0
case "$out" in /*) ;; *) echo "FAIL: not absolute"; exit 1;; esac
test -f "$out"   # the path must be a REAL file (the .ts)

# 2. Basename resolution → writing/reddit-poster.
out=$(/tmp/weave reddit-poster); code=$?
test "$code" = 0
test "$out" = "$STORE/writing/reddit-poster.ts"

# 3. Multiple tags → input order, one per line.
out=$(/tmp/weave reddit-poster example); code=$?
test "$code" = 0
echo "$out" | sed -n '1p' | grep -q 'reddit-poster'
echo "$out" | sed -n '2p' | grep -q 'example'

# 4. Unknown tag → empty stdout, stderr, exit 1.
out=$(/tmp/weave nope 2>/dev/null); code=$?
test "$code" = 1
test -z "$out"   # stdout empty (§6.4)

# 5. Atomicity: good+bad → empty stdout, exit 1.
out=$(/tmp/weave example nope 2>/dev/null); code=$?
test "$code" = 1
test -z "$out"

# 6. --file on single-file → no-op (same as default).
f=$(/tmp/weave -f example)
test "$f" = "$STORE/example.ts"

# 7. --file on dir ext → index.ts.
f=$(/tmp/weave -f git-checkpoint)
test "$f" = "$STORE/git-checkpoint/index.ts"

# 8. --all → sorted, exit 0.
out=$(/tmp/weave --all); code=$?
test "$code" = 0
echo "$out" | sed -n '1p' | grep -q 'example.ts'

# 9. --all on empty store → exit 0, empty.
EMPTY=$(mktemp -d); export weave_EXTENSIONS_DIR="$EMPTY"
out=$(/tmp/weave --all); code=$?
test "$code" = 0
test -z "$out"

# 10. Unconfigured → exit 1, the one-line fix.
export weave_EXTENSIONS_DIR="$EMPTY/nonexistent"
out=$(/tmp/weave example 2>/dev/null); code=$?
test "$code" = 1
/tmp/weave example 2>&1 1>/dev/null | grep -q 'weave init'

echo "ALL INTEGRATION CHECKS PASSED"
rm -rf "$STORE" "$EMPTY" /tmp/weave
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The §6.4 $(...) safety contract — verify it behaves correctly inside command
# substitution (the actual deployment shape for pi -e "$(weave <tag>)"):
STORE=$(mktemp -d); export weave_EXTENSIONS_DIR="$STORE"
echo '/** demo */' > "$STORE/example.ts"
go build -o /tmp/weave .

# Good tag: $(...) captures the path, exit 0 propagates.
path=$(/tmp/weave example) && test -f "$path" && echo "OK: pi -e \"$path\" would work"

# Bad tag: $(...) captures EMPTY, exit 1 propagates (set -e / if-gate catches it).
if path=$(/tmp/weave nope); then
  echo "FAIL: bad tag should have failed"; exit 1
else
  test -z "$path" && echo "OK: bad tag → empty \$(), exit 1 (pi -e fails loudly)"
fi

rm -rf "$STORE" /tmp/weave
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `go test ./... -v` — all tests pass (existing M1/M2 + new M3 tag/--all tests).
- [ ] `go test -race ./...` — whole repo green.
- [ ] No new dependencies (`go.mod`/`go.sum` unchanged — stdlib `path/filepath` +
      `internal/resolve` only).

### Feature Validation

- [ ] `extensionPath` implements the 4 modifier combos (default/file/relative/file+relative).
- [ ] `extensionPath` does NOT switch on `ext.Kind` (reads Path/EntryFile only).
- [ ] `--all` prints one absolute path per line, sorted by RelTag, exit 0 (even empty).
- [ ] Tag branch prints one path per line in INPUT order (not sorted).
- [ ] ATOMICITY: any failing tag ⇒ nothing on stdout, one stderr line per problem, exit 1.
- [ ] `--file` on single-file ext is a no-op; on dir ext → `index.ts`; on package ext →
      first `pi.extensions` entry.
- [ ] `--relative` makes the chosen path relative; `--file --relative` combines.
- [ ] Default and `--file` outputs are absolute; `--relative` outputs are relative.
- [ ] Errors print with NO `weave:` prefix (verbatim typed-error text).
- [ ] The 2nd return of `extdir.Find()` (src) is discarded by both new branches.

### Code Quality Validation

- [ ] Verbatim port of skilldozer's `skillPath`/`--all`/tag branches — logic NOT
      "improved" (only the documented field/noun/env swaps).
- [ ] Doc comments carry the noun swap ("resolvable path", "entry file", "extensions dir").
- [ ] `res.Extension` used (NOT `res.Skill`).
- [ ] `extensionPath` placed near the bottom of main.go (after run()), matching skilldozer.
- [ ] New branches inserted ABOVE the final `return 0` (not replacing it).
- [ ] Tests reuse `writeExtTree`/`unsetExtEnv`/`withTerminal` (no duplicate helpers);
      new helpers (`sampleStore`, `writeDirExt`, `writePkgExt`) are added only for
      what `writeExtTree` cannot build.
- [ ] No `t.Parallel()` on env-mutating tests.

### Documentation & Deployment

- [ ] `extensionPath` has a doc comment explaining the 4 modifier combos + the
      Kind-independence + the defensive Rel err-guard.
- [ ] The `--all` and tag branches have doc comments citing PRD §6.1/§6.4.
- [ ] No README/docs changes (CLI usage is documented in README §4 in the final
      M6.T4/M6.T5 doc sweep — this task's PRP §5 says "DOCS: none").

---

## Anti-Patterns to Avoid

- ❌ Don't `switch ext.Kind` in `extensionPath` — Path/EntryFile are already populated
  per Kind by discover.Index. Reading the selected field is the entire logic.
- ❌ Don't print each tag's path inline as it resolves — buffer first, flush only on
  full success (a later failing tag must not leak earlier successes to stdout; §6.4).
- ❌ Don't prefix errors with `weave:` — the typed errors' `.Error()` is the complete
  line; a prefix would double up with the `extdir.ErrNotFound` convention.
- ❌ Don't assert `…/example.ts/SKILL.md` for `--file` on a single-file ext — weave's
  `--file` is a NO-OP for file kind (EntryFile == Path). This is the OPPOSITE of skilldozer.
- ❌ Don't use `SKILLDOZER_SKILLS_DIR` in tests — weave's env var is `weave_EXTENSIONS_DIR`
  (lowercase). A copied skilldozer test that forgets the swap will silently hit
  ErrNotFound and fail mysteriously.
- ❌ Don't copy `--list`'s "exit 1 if empty" check into `--all` — `--all` is ALWAYS
  exit 0, even on an empty store (PRD §6.1).
- ❌ Don't touch `parseArgs` — it is COMPLETE since M1.T4.S1. This task adds dispatch
  branches only.
- ❌ Don't wire `--search`/`check`/`init`/`--help`/exclusivity — those are M4/M5.
- ❌ Don't move or delete the final `return 0` in run() — M5.T1.S1 owns its transformation.
- ❌ Don't touch `go.mod`/`go.sum` — `path/filepath` is stdlib, `internal/resolve` is local.
- ❌ Don't use `res.Skill` — the field is `res.Extension` (P1.M3.T1.S1 contract).

---

**Confidence Score: 9/10** for one-pass success. The work is a documented verbatim
port of three proven, heavily-tested pieces from skilldozer (skillPath, the --all
branch, the tag-resolution branch), with a single well-specified semantic change
(Dir/SourceFile → Path/EntryFile) whose Kind-dependence is encoded in the DATA
(not in a switch). The consumed packages (`extdir.Find`, `discover.Index`,
`resolve.Resolve`) are all landed or being landed in parallel under explicit
contracts. The test file to port is available and the weave test conventions
(`writeExtTree`, env-var name, bytes.Buffer capture) are documented in
`research/test_conventions.md`. The one residual risk (not 10/10) is a mechanical
slip in a copied test — either a stale `SKILLDOZER_SKILLS_DIR`, a `res.Skill`
leftover, or asserting `…/SKILL.md` for `--file` on a file-kind ext — all caught
immediately by `go build`/`go test`, but they are the kind of copy-port slip that
can cost a second pass.
