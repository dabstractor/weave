# PRP — P1.M1.T3.S3: Walk-up-from-cwd (rule 4) + Find() combiner + ErrNotFound

## Goal

**Feature Goal**: Complete the `internal/extdir` package by adding **rule 4**
(`findWalkUp` + `findWalkUpAncestor`: ascend from `os.Getwd()` until an ancestor's
`extensions/` subdir contains at least one extension entry, PRD §8.3 priority 4),
the **`ErrNotFound` sentinel** (PRD §8.2 / §6.4 exact one-line user-facing fix), and the
**`Find()` public combiner** (PRD §8.3 first-hit-wins across all four rules, returning
`ErrNotFound` when every rule misses). This closes task P1.M1.T3 — the package that
PRD §13 calls "the single most failure-prone area — implement and test it first."

This is the weave port of skilldozer's `internal/skillsdir/skillsdir.go` (rule-4 + Find +
ErrNotFound portions), per `architecture/architecture_mapping.md §2`. The item contract
pins it explicitly: "Port skilldozer's `findWalkUp`/`findWalkUpAncestor` verbatim with
predicate change."

**Deliverable**: Modify `internal/extdir/extdir.go` — append `findWalkUpAncestor`,
`findWalkUp`, `ErrNotFound`, and `Find()` — and modify `internal/extdir/extdir_test.go`
— append the `makeExtension` helper and the rule-4 + Find + ErrNotFound test suites
(ported from skilldozer with renames). Also UPGRADE `unsetEnvVar` to neutralize
`weave_CONFIG` (so `Find()` all-miss / env-only tests are hermetic — see Known Gotchas).
No new files; no new imports (the stdlib block from S1/S2 already covers `errors`, `os`,
`path/filepath`; the `internal/config` import from S2 is unchanged).

**Success Definition**:
- `go build ./...` exits 0 (requires S2 to have landed `findConfig`/`findSibling`, since
  `Find()` calls them — S2 and S3 are a parallel pair that together complete T3).
- `go vet ./internal/extdir/` exits 0 (clean).
- `go test ./internal/extdir/ -v` passes ALL tests — S1 suite (Source/findEnv/HasExtensionEntry),
  S2 suite (findConfig/findSibling/resolveSiblingFromExe), and the new S3 suites
  (findWalkUpAncestor/findWalkUp/Find/ErrNotFound).
- The walk-up-from-cwd test passes: `t.Chdir` into a subdir of a temp repo whose ancestor
  has `extensions/<tag>/<tag>.ts`, and `findWalkUp()` resolves back to `<repo>/extensions`
  with `src=SourceWalkUp` — proving the `os.Getwd()` → ascent path works end-to-end
  (the whole point of rule 4 for `go run` / dev).
- The `Find()` all-miss test passes: `errors.Is(err, ErrNotFound)` is true and the
  returned `dir==""`, `src==0` when env + config + sibling + walk-up all miss.
- `ErrNotFound.Error()` equals exactly `weave is not configured; run \`weave init\``
  (PRD §8.2 / §6.4 — literal backticks around the command).
- go.mod still has no `require` block; no `go.sum` exists.

## User Persona (if applicable)

N/A — internal package consumed by `main.go` (P1.M1.T4) and `discover.Index` (P1.M2.T3).
The exported `Find()` is THE public entrypoint for locating the extensions directory;
`ErrNotFound` is the exact string main prints to stderr on the unconfigured path. No
direct user surface in this package (the CLI surface is P1.M1.T4).

## Why

- PRD §8.3 makes rule 4 (walk-up from cwd) the catch-all for `go run` / dev workflows
  where the binary is in a temp build dir and rules 1-3 (env, config, sibling-of-binary)
  all miss. Without rule 4, `go run github.com/dabstractor/weave@latest` from inside a
  repo with an `extensions/` dir would report "unconfigured" even though the store is
  right there. This is the most common dev ergonomics path.
- `ErrNotFound` is **load-bearing for PRD §6.4 error semantics** and §8.2 prompt safety:
  the bare `weave <tag>` path must NEVER prompt. When unconfigured, main prints
  `ErrNotFound.Error()` verbatim to stderr, writes nothing to stdout, and exits 1 — so
  `pi -e "$(weave x)"` fails loudly inside command substitution instead of hanging. The
  exact string is pinned by the PRD; a typo or rewording here breaks the documented UX.
- `Find()` is the single public entrypoint consumed by `main.run` (P1.M1.T4) and
  `discover.Index` (P1.M2.T3). It is the seam where the four-rule priority order becomes
  one callable. S1 (rule 1 + predicate) and S2 (rules 2-3) deliberately deferred Find to
  S3 so the combiner lands exactly when its last dependency (rule 4) lands.
- PRD §13 acceptance: extdir is "the single most failure-prone area — implement and test
  it first." S3 closes the package; after it, every rule is implemented and the priority
  order is exercised end-to-end by `TestFindRuleEnvWins`, `TestFindRuleWalkUpWins`, and
  `TestFindAllMissReturnsErrNotFound`.

## What

Three new unexported functions, one new exported variable, and one new exported function
added to the existing `internal/extdir/extdir.go`:

1. `findWalkUpAncestor(start string) (dir string, found bool)` — the testable ascent core.
   Starts at `filepath.Clean(start)`. For `cur`, computes `candidate := filepath.Join(cur, "extensions")`;
   if `os.Stat(candidate)` succeeds and is a dir AND `HasExtensionEntry(candidate)` is true,
   returns `(candidate, true)`. (An `extensions/` dir with NO entries is SKIPPED — ascent
   continues; PRD §8.3 qualifies the match with "at least one extension entry.") Ascends via
   `cur = filepath.Dir(cur)` until `parent == cur` (filesystem root) → returns `("", false)`.
   **Checks the start dir FIRST** (the loop body runs before any ascent).
2. `findWalkUp() (dir string, src Source, found bool)` — rule 4 entry. Calls `os.Getwd()`;
   on error returns `("", 0, false)`. Otherwise delegates to `findWalkUpAncestor(cwd)` and
   returns `(dir, SourceWalkUp, true)` on a hit, `("", 0, false)` on a miss. Never errors.
3. `var ErrNotFound = errors.New("weave is not configured; run \`weave init\`")` — the PRD
   §8.2 / §6.4 exact one-line fix. Returned by `Find()` on total miss; main prints
   `err.Error()` verbatim to stderr. Do NOT wrap or prefix.
4. `func Find() (dir string, src Source, err error)` — the public combiner. Tries rule 1
   (`findEnv`) → rule 2 (`findConfig`) → rule 3 (`findSibling`) → rule 4 (`findWalkUp`)
   in order; returns the first hit as `(absDir, src, nil)`. If all miss, returns
   `("", 0, ErrNotFound)`. **NEVER prompts** (PRD §8.2 prompt safety) — the unconfigured
   case returns ErrNotFound and lets the caller decide; no isatty / auto-init logic here.

No new imports are added beyond what S1/S2 already declared (`errors`, `os`, `path/filepath`
are all present; `internal/config` from S2 is unchanged).

### Success Criteria

- [ ] `findWalkUpAncestor` exists with signature `func findWalkUpAncestor(start string) (dir string, found bool)`,
      ports skilldozer's body verbatim with the two documented changes (`"skills"`→`"extensions"`,
      `HasSkillMD`→`HasExtensionEntry`), checks `start` FIRST, skips an empty `extensions/`
      dir and keeps ascending, and terminates via `parent == cur` at the filesystem root.
- [ ] `findWalkUp` exists with signature `func findWalkUp() (dir string, src Source, found bool)`,
      ports skilldozer's body verbatim, calls `os.Getwd()`, delegates to `findWalkUpAncestor`,
      returns `(dir, SourceWalkUp, true)` on hit, `("", 0, false)` on miss/cwd-error. Never errors.
- [ ] `ErrNotFound` is declared as `var ErrNotFound = errors.New("weave is not configured; run \`weave init\`")`
      — EXACTLY this string (PRD §8.2/§6.4), with literal backticks around `weave init`.
- [ ] `Find` exists with signature `func Find() (dir string, src Source, err error)`, tries
      the four rules in priority order, returns the first hit as `(absDir, src, nil)`, and
      returns `("", 0, ErrNotFound)` on total miss. Contains NO prompting / isatty / init logic.
- [ ] `go build ./...`, `go vet ./internal/extdir/`, and `go test ./internal/extdir/` pass,
      including the `t.Chdir` walk-up test and the `errors.Is(err, ErrNotFound)` all-miss test.
- [ ] `unsetEnvVar` is upgraded (or confirmed already upgraded by S2) to neutralize BOTH
      `envVar` (`weave_EXTENSIONS_DIR`) AND `weave_CONFIG` (lowercase) — so `Find()` all-miss
      and env-only tests are hermetic against a real machine config.
- [ ] go.mod has no `require` block; no go.sum exists; the only non-stdlib import in
      `extdir.go` is `github.com/dabstractor/weave/internal/config` (from S2).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** This PRP gives the exact source-of-truth file to port
(`internal/skillsdir/skillsdir.go`, with all four functions quoted verbatim), the exact
two-item rename table, the exact test patterns to mirror (test names listed), the exact
ErrNotFound string, the verified `t.Chdir` (Go 1.24+, go.mod says 1.25) testing approach,
and the verified build/vet/test gates. The S1 PRP establishes `HasExtensionEntry` (the
predicate rule 4 calls) and the `Source`/`SourceWalkUp` constant; the S2 PRP establishes
`findConfig`/`findSibling` (the rules Find calls between env and walk-up).

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for §8.3 priority order (rule 4 = walk-up from cwd; rule 5 =
       None ⇒ ErrNotFound), §8.2 (ErrNotFound exact string + prompt safety — bare weave
       <tag> NEVER prompts), §6.4 (error semantics for $(...) use).
  critical: §8.3 rule 4 says "Walk up from cwd — for go run / dev." §8.2 prompt safety:
            "the bare weave <tag> path never prompts. If unconfigured … it writes to stderr
            exactly `weave is not configured; run \`weave init\``, exits 1, and writes
            nothing to stdout." §6.4: "Extensions dir cannot be located / weave is
            unconfigured ⇒ stderr: `weave is not configured; run \`weave init\``."

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir.go
  why: PRIMARY pattern to port — findWalkUpAncestor, findWalkUp, ErrNotFound, and Find ALL
       port VERBATIM. Only TWO things change for weave: (1) the sibling dir literal
       "skills" → "extensions" inside findWalkUpAncestor, and (2) the predicate call
       HasSkillMD → HasExtensionEntry. ErrNotFound's string changes from "skilldozer" to
       "weave" (and the command from "skilldozer init" to "weave init").
  pattern: |
    func findWalkUpAncestor(start string) (dir string, found bool) {
        cur := filepath.Clean(start)
        for {
            candidate := filepath.Join(cur, "extensions")   // weave: "extensions" not "skills"
            if info, err := os.Stat(candidate); err == nil && info.IsDir() {
                if HasExtensionEntry(candidate) {            // weave: HasExtensionEntry not HasSkillMD
                    return candidate, true
                }
                // extensions/ exists here but has no entries -> keep ascending.
            }
            parent := filepath.Dir(cur)
            if parent == cur {
                return "", false // reached filesystem root, no match
            }
            cur = parent
        }
    }
    func findWalkUp() (dir string, src Source, found bool) {
        cwd, err := os.Getwd()
        if err != nil {
            return "", 0, false // cwd unresolvable -> rule misses
        }
        d, ok := findWalkUpAncestor(cwd)
        if !ok {
            return "", 0, false
        }
        return d, SourceWalkUp, true
    }
    var ErrNotFound = errors.New("weave is not configured; run `weave init`")
    func Find() (dir string, src Source, err error) {
        if d, s, ok := findEnv(); ok {
            return d, s, nil
        }
        if d, s, ok := findConfig(); ok { // PRD §8.3 priority #2 (S2)
            return d, s, nil
        }
        if d, s, ok := findSibling(); ok { // PRD §8.3 priority #3 (S2)
            return d, s, nil
        }
        if d, s, ok := findWalkUp(); ok {
            return d, s, nil
        }
        return "", 0, ErrNotFound
    }
  gotcha: ErrNotFound's message is the user-facing one-line fix. main prints err.Error()
          VERBATIM to stderr. Do NOT wrap with fmt.Errorf, do NOT prefix with "error:", do
          NOT reword. The literal backticks around `weave init` are part of the message
          (shell users copy-paste the command). Pinned by TestErrNotFoundMessageHasFix.

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir_test.go
  why: PRIMARY test pattern to port. Port these tests verbatim with renames
       ("skilldozer"→"weave", "skills"→"extensions", "SKILL.md"→"<tag>.ts", "makeSkill"→
       "makeExtension"):
         TestFindWalkUpAncestorAtStart, TestFindWalkUpAncestorDeep,
         TestFindWalkUpAncestorNestedSkillMD→TestFindWalkUpAncestorNestedEntry,
         TestFindWalkUpAncestorSkipsEmptyAndContinues,
         TestFindWalkUpAncestorNoSkills→TestFindWalkUpAncestorNoExtensions,
         TestFindWalkUpAncestorSkillsIsFile→TestFindWalkUpAncestorExtensionsIsFile,
         TestFindWalkUpFindsAncestor (uses t.Chdir),
         TestFindRuleEnvWins, TestFindRuleWalkUpWins (uses t.Chdir),
         TestFindAllMissReturnsErrNotFound (uses t.Chdir),
         TestErrNotFoundMessageHasFix.
  pattern: |
    func makeExtension(t *testing.T, dir, tag string) string {
        t.Helper()
        ext := filepath.Join(dir, "extensions")
        entry := filepath.Join(ext, tag)
        if err := os.MkdirAll(entry, 0o755); err != nil {
            t.Fatalf("mkdir %s: %v", entry, err)
        }
        if err := os.WriteFile(filepath.Join(entry, tag+".ts"), []byte("x"), 0o644); err != nil {
            t.Fatalf("write %s.ts: %v", tag, err)
        }
        return ext
    }
    // TestFindWalkUpFindsAncestor uses t.Chdir (Go 1.24+; go.mod says 1.25):
    func TestFindWalkUpFindsAncestor(t *testing.T) {
        root := t.TempDir()
        ext := makeExtension(t, root, "example")
        sub := filepath.Join(root, "sub")
        if err := os.MkdirAll(sub, 0o755); err != nil { t.Fatal(err) }
        t.Chdir(sub)
        got, src, found := findWalkUp()
        if !found { t.Fatalf("findWalkUp(): found=false; want true") }
        if src != SourceWalkUp { t.Errorf("findWalkUp(): src=%v; want SourceWalkUp", src) }
        if got != ext { t.Errorf("findWalkUp(): dir=%q; want %q", got, ext) }
    }
    func TestFindAllMissReturnsErrNotFound(t *testing.T) {
        unsetEnvVar(t)
        t.Chdir(t.TempDir())
        got, src, err := Find()
        if !errors.Is(err, ErrNotFound) {
            t.Fatalf("Find() all-miss: err=%v; want ErrNotFound", err)
        }
        if got != "" || src != 0 {
            t.Errorf("Find() all-miss: got=%q src=%v; want \"\" and 0", got, src)
        }
    }
  gotcha: t.Chdir (Go 1.24+) changes cwd for the test and restores it on cleanup, so
          findWalkUp (os.Getwd) and Find walk-up tests are hermetic WITHOUT global cwd
          pollution. go.mod declares `go 1.25`, so t.Chdir is available. Do NOT use
          os.Chdir + manual restore (t.Chdir is the stdlib-blessed, cleanup-safe way).

- file: /home/dustin/projects/weave/internal/extdir/extdir.go  (S1 COMPLETE; S2 in parallel)
  why: The CONTRACT for the package S3 extends. Confirms: Source type + SourceWalkUp
       constant already exist; String() returns "ancestor of cwd" for SourceWalkUp;
       findEnv (rule 1) + HasExtensionEntry (the predicate rule 4 calls) + envVar const
       already exist; the package doc comment already describes the §8.3 priority order
       and notes rules 2-4 + Find land in S2/S3.
  critical: S3 ADDS findWalkUpAncestor/findWalkUp/ErrNotFound/Find AFTER the existing
            findEnv (and after findConfig/findSibling once S2 lands). S3 does NOT modify
            findEnv, HasExtensionEntry, Source, or any S1 symbol. The `errors`/`os`/
            `path/filepath` imports S3 needs are ALREADY in the S1 import block — S3 adds
            NO new import lines.

- file: /home/dustin/projects/weave/internal/extdir/extdir_test.go  (S1 COMPLETE; S2 in parallel)
  why: The CONTRACT for the test file S3 extends. S1's unsetEnvVar currently neutralizes
       ONLY envVar. S2's PRP (Task 5) UPGRADES unsetEnvVar to also neutralize weave_CONFIG.
       S3's Find/walk-up tests DEPEND on that upgrade (or the all-miss test leaks a real
       machine config). See Known Gotchas: S3 must ensure the upgrade is present (re-spec
       it idempotently if S2 hasn't landed).
  critical: S3 APPENDS the makeExtension helper + the rule-4 + Find + ErrNotFound tests.
            S3 does NOT modify the S1 tests. If S2 already upgraded unsetEnvVar, S3's
            upgrade is a no-op (idempotent); if S2 hasn't, S3 performs the upgrade so S3
            is self-contained.

- file: /home/dustin/projects/weave/internal/config/config.go  (P1.M1.T2.S1 COMPLETE)
  why: Indirect dependency. Find() calls findConfig() (S2), which calls config.Path() +
       config.Load(). S3 does NOT call config directly. Included only to confirm the
       findConfig contract S3 relies on: ANY error or Store=="" → findConfig returns
       ("", 0, false), so Find falls through to rule 3 / rule 4. No S3 change to config.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §2 maps skilldozer skillsdir → weave extdir line by line and pins the renames +
       the predicate change. Confirms S3 owns rule 4 + Find + ErrNotFound.
  critical: §2 "Port findWalkUpAncestor verbatim — only the predicate changes." §2
            "Predicate change: skilldozer uses HasSkillMD(dir); weave uses
            HasExtensionEntry(dir) — walks for ANY extension entry at any depth." §2
            "Find() and HasExtensionEntry are the two EXPORTED symbols of extdir."

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M1T3S1/PRP.md  (S1, COMPLETE)
  why: Establishes the exact S1 symbols S3 consumes: HasExtensionEntry (exported predicate,
       the qualifier rule 4 calls), Source + SourceWalkUp (the constant Find returns),
       findEnv + envVar (rule 1, called first by Find), and the unsetEnvVar helper that
       S3 must ensure is upgraded (S2 also specifies this upgrade).
  critical: HasExtensionEntry's recognition logic is FIXED by S1 and tested there. S3
            treats it as a black box returning bool — do NOT re-test it in S3.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M1T3S2/PRP.md  (S2, parallel)
  why: The CONTRACT for the parallel S2 work. S2 adds findConfig (rule 2), findSibling +
       resolveSiblingFromExe (rule 3), the internal/config import, and the unsetEnvVar
       upgrade. S3's Find() calls findConfig and findSibling — treat them as EXISTING
       per the parallel_execution_context instruction ("assume it will be implemented
       exactly as specified").
  critical: S2 and S3 together complete T3. If S2 has NOT landed when S3 is implemented,
            `go build ./internal/extdir/` will FAIL with "undefined: findConfig /
            undefined: findSibling" inside Find(). That is EXPECTED — S3's PRP compiles
            only after S2 lands. Do NOT stub findConfig/findSibling to make S3 build
            in isolation; the pair is designed to land together.

- file: /home/dustin/projects/weave/go.mod  (P1.M1.T1.S1)
  why: Confirms module `github.com/dabstractor/weave`, `go 1.25`. The `go 1.25` directive
       is what makes t.Chdir (Go 1.24+) available — S3's walk-up tests depend on it.
  critical: S3 adds NO import (the stdlib packages it needs — errors, os, path/filepath —
            are already imported by S1). go.mod gains NO require block; no go.sum.
```

### Current Codebase tree

```bash
# After P1.M1.T1.S1 (Complete) + P1.M1.T2.S1 (Complete) + P1.M1.T3.S1 (Complete) +
# P1.M1.T3.S2 (parallel, "Ready"):
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
└── extdir/             # P1.M1.T3.S1 (Complete) + S2 (parallel)
    ├── extdir.go       # S1: Source/String/envVar/findEnv/HasExtensionEntry
    │                   # S2: + config import + findConfig + findSibling + resolveSiblingFromExe
    └── extdir_test.go  # S1: Source/findEnv/HasExtensionEntry tests
                        # S2: + makeFakeBinary + unsetEnvVar upgrade + rule-2/rule-3 tests
plan/
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                      # UNCHANGED by this subtask
│   └── extdir/
│       ├── extdir.go                # MODIFIED — append findWalkUpAncestor/findWalkUp/ErrNotFound/Find
│       └── extdir_test.go           # MODIFIED — append makeExtension + rule-4/Find/ErrNotFound tests;
│                                     #           ensure unsetEnvVar upgrade (idempotent w/ S2)
├── go.mod                           # UNCHANGED (no new import, no new require)
└── ...                              # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the sibling dir literal in findWalkUpAncestor is "extensions" (NOT "skills").
// This is one of the TWO documented renames from skilldozer (the other is HasSkillMD →
// HasExtensionEntry). A leftover "skills" string is the single most likely copy-paste bug
// — pinned by every findWalkUpAncestor test asserting the returned dir ends in "/extensions".

// CRITICAL: the predicate call is HasExtensionEntry(candidate), NOT HasSkillMD. This is
// the second rename. HasExtensionEntry is ALREADY exported by S1 and recognizes the three
// PRD §7.1 entry kinds. S3 treats it as a black box; do NOT re-implement or re-test it.

// CRITICAL: ErrNotFound's message is EXACTLY `weave is not configured; run \`weave init\``.
// PRD §8.2 / §6.4 pin it verbatim. The literal backticks around `weave init` are part of
// the message. main (P1.M1.T4) prints err.Error() to stderr with no prefix/wrapping. A
// typo here breaks the documented UX and the TestErrNotFoundMessageHasFix gate.

// CRITICAL: Find NEVER prompts. PRD §8.2 prompt safety is load-bearing: the bare
// `weave <tag>` path must never call init interactively. Find returns ErrNotFound on total
// miss; the caller (main) prints it and exits 1. Do NOT add isatty(stdin) / auto-init /
// "did you mean" logic to Find. The ONLY place weave prompts is `weave init` (P1.M4.T4).

// CRITICAL: findWalkUpAncestor checks the START dir FIRST. `cur := filepath.Clean(start)`,
// then the loop body runs BEFORE any ascent. So if start itself has extensions/ with
// entries, it wins immediately. Pinned by TestFindWalkUpAncestorAtStart.

// CRITICAL: an extensions/ dir that EXISTS but has NO entries is SKIPPED and ascent
// continues. PRD §8.3 qualifies the match with "at least one extension entry." This is
// the HasExtensionEntry(candidate) guard inside the IsDir branch. Pinned by
// TestFindWalkUpAncestorSkipsEmptyAndContinues — a lower empty extensions/ is passed over
// for a higher one with real entries.

// CRITICAL: loop termination is `parent == cur` (filepath.Dir(root) == root at the
// filesystem root). Do NOT use a depth counter, a max-iterations cap, or a strings.HasPrefix
// root check — the parent==cur idiom is the portable stdlib way and ports verbatim from
// skilldozer.

// CRITICAL: t.Chdir (Go 1.24+) is REQUIRED for findWalkUp and Find walk-up tests.
// os.Getwd() is not otherwise controllable from a test. go.mod declares `go 1.25`, so
// t.Chdir is available — it changes cwd for the test and restores on cleanup. Do NOT use
// os.Chdir + tb.Cleanup(restore); t.Chdir is the stdlib-blessed, race-safe way. Tests
// using t.Chdir should NOT call t.Parallel() (cwd is process-global).

// CRITICAL: unsetEnvVar MUST neutralize BOTH envVar (weave_EXTENSIONS_DIR) AND weave_CONFIG
// (lowercase). Without the weave_CONFIG neutralization, a dev machine with a real
// ~/.config/weave/config.yaml makes Find() return a real dir in TestFindAllMissReturnsErrNotFound
// and TestFindRuleEnvWins. S2's PRP specifies this same upgrade; if S2 already landed it,
// S3's version is an idempotent no-op. If S2 hasn't landed, S3 must perform it so S3 is
// self-contained. The env var name is LOWERCASE "weave_CONFIG" (config.go const configEnv).

// GOTCHA: findSibling deterministically MISSES in tests. The test binary runs from a temp
// go-build dir with no sibling extensions/. So TestFindRuleWalkUpWins can rely on rule 3
// missing and rule 4 winning — no need to neutralize rule 3 explicitly. (Same rationale
// as S2's TestFindSiblingNoExtensionsNextToTestBinary.)

// GOTCHA (build dependency): S3's Find() calls findConfig and findSibling, which S2 defines.
// If S2 has NOT landed, `go build ./internal/extdir/` fails with "undefined: findConfig /
// undefined: findSibling". This is EXPECTED — S2 and S3 are a parallel pair that together
// complete T3. Do NOT stub those functions to make S3 build in isolation.

// GOTCHA (build): S3 adds NO new import. The stdlib packages it needs (errors for
// errors.New, os for os.Getwd/os.Stat, path/filepath for filepath.Clean/Dir/Join) are
// ALREADY imported by S1's extdir.go. The internal/config import (from S2) is unchanged.
// If the implementation adds an import, something is wrong.

// GOTCHA (boundaries): do NOT modify findEnv, HasExtensionEntry, Source, String, or any
// S1 symbol. do NOT modify findConfig/findSibling/resolveSiblingFromExe (S2's boundary).
// do NOT modify the config package or main.go. S3's blast radius is extdir.go (append 4
// symbols) + extdir_test.go (append 1 helper + ~11 tests + ensure unsetEnvVar upgrade).

// GOTCHA: the makeExtension helper writes extensions/<tag>/<tag>.ts (a case-(a) single-
// file entry per PRD §7.1, recognized by HasExtensionEntry). Do NOT write SKILL.md or a
// bare extensions/<tag>/ dir with no entry — that would make HasExtensionEntry return
// false and the walk-up tests would miss. The .ts file (NOT index.ts — index.* are dir
// markers, case a excludes them) is what makes the candidate qualify.
```

## Implementation Blueprint

### Data models and structure

No new types. S3 consumes the existing `Source` type and its `SourceWalkUp` constant
(declared by S1). One new exported variable (`ErrNotFound`, a sentinel `error`). One new
exported function (`Find`). Two new unexported functions (`findWalkUpAncestor`,
`findWalkUp`). No new imports.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/extdir/extdir.go — add findWalkUpAncestor (rule 4 testable core)
  - APPEND findWalkUpAncestor(start string) AFTER the last existing rule helper (findEnv
    from S1, or findSibling from S2 if it has landed — keep rule helpers grouped in
    priority order). Port the body VERBATIM from the skilldozer pattern in Documentation
    & References, with the TWO documented changes:
      (1) candidate literal "skills" → "extensions"
      (2) predicate HasSkillMD(candidate) → HasExtensionEntry(candidate)
  - SIGNATURE: func findWalkUpAncestor(start string) (dir string, found bool)
  - DOC COMMENT: 8-14 lines explaining: PRD §8.3 rule 4 ascent core; factored out so it
    can be tested with an arbitrary start dir (os.Getwd is not controllable without chdir);
    checks start FIRST; skips an empty extensions/ dir and keeps ascending (PRD §8.3
    "at least one extension entry" qualifier); terminates via parent==cur at the FS root.
    Note the two renames from skilldozer.
  - NAMING: findWalkUpAncestor (lowercase, package-internal — matches findEnv/findConfig/
    findSibling and the locked per-rule shape).
  - PLACEMENT: internal/extdir/extdir.go, after findSibling (S2) or after findEnv if S2
    hasn't landed — logical grouping is rule 1 → 2 → 3 → 4.

Task 2: MODIFY internal/extdir/extdir.go — add findWalkUp (rule 4 entry)
  - APPEND findWalkUp() immediately AFTER findWalkUpAncestor. Port the body VERBATIM from
    the skilldozer pattern. Calls os.Getwd(); on error returns ("", 0, false); otherwise
    delegates to findWalkUpAncestor(cwd) and returns (dir, SourceWalkUp, true) on hit.
  - SIGNATURE: func findWalkUp() (dir string, src Source, found bool)
  - DOC COMMENT: PRD §8.3 rule 4; thin entry calling os.Getwd(); the testable core lives
    in findWalkUpAncestor because os.Getwd is not controllable in tests (use t.Chdir).
    This is the rule that makes `go run` work when the binary is in a temp build dir and
    rules 1-3 miss. Never errors (locked per-rule shape).
  - NAMING: findWalkUp (lowercase, package-internal).
  - PLACEMENT: immediately after findWalkUpAncestor.

Task 3: MODIFY internal/extdir/extdir.go — add ErrNotFound
  - APPEND after findWalkUp (before Find, since Find returns it). Declare:
      var ErrNotFound = errors.New("weave is not configured; run `weave init`")
    EXACTLY this string (PRD §8.2/§6.4), with literal backticks around `weave init`.
  - DOC COMMENT: returned by Find when every §8.3 rule misses (unconfigured); the message
    is the user-facing one-line fix; main prints err.Error() VERBATIM to stderr and exits 1
    (PRD §8.2 prompt safety — do NOT wrap or prefix). Note errors.Is(err, ErrNotFound) works.
  - NAMING: ErrNotFound (exported — main and discover consume it).
  - PLACEMENT: between findWalkUp and Find.

Task 4: MODIFY internal/extdir/extdir.go — add Find (the public combiner)
  - APPEND Find() after ErrNotFound. Port the body VERBATIM from the skilldozer pattern.
    Tries findEnv → findConfig → findSibling → findWalkUp in order; returns first hit as
    (absDir, src, nil); on total miss returns ("", 0, ErrNotFound).
  - SIGNATURE: func Find() (dir string, src Source, err error)
  - DOC COMMENT: PRD §8.3 first-hit-wins priority order; the single public entrypoint
    consumed by main.run (P1.M1.T4) and discover.Index (P1.M2.T3); NEVER prompts (PRD §8.2
    prompt safety — the unconfigured case returns ErrNotFound and the caller decides);
    list the 5 cases (4 rules + None⇒ErrNotFound). Note the absDir invariant (each rule
    helper already returns an absolute path).
  - NAMING: Find (exported — the package's primary entrypoint).
  - PLACEMENT: last function in the file (it depends on all four rule helpers).

Task 5: MODIFY internal/extdir/extdir.go — update the package doc comment
  - THE S1 package doc lists the §8.3 priority order with [S2]/[S3] annotations. After S3
    lands rule 4 + Find + ErrNotFound, update the annotations: rules 2-4 and Find/ErrNotFound
    are NOW implemented (remove the [S2]/[S3] brackets for rules 2-4 and case 5). Do NOT
    rewrite the whole comment — surgical update of the bracketed annotations only.
  - PRESERVE: the findEnv no-EvalSymlinks contract note and the HasExtensionEntry dual-role
    note (init auto-detect + walk-up qualifier) from S1/S2.

Task 6: MODIFY internal/extdir/extdir_test.go — ensure unsetEnvVar is upgraded
  - CHECK whether unsetEnvVar already neutralizes weave_CONFIG (S2 may have done it). If
    YES, this task is a no-op. If NO, perform the upgrade: after the existing envVar
    neutralization block, add a weave_CONFIG neutralization that points it at a non-existent
    ghost path (filepath.Join(tb.TempDir(), "no-config.yaml")) and restores it on cleanup.
    Mirror skilldozer's unsetEnvVar (which does this for SKILLDOZER_CONFIG).
  - ADD (only if not already present):
      cfgGhost := filepath.Join(tb.TempDir(), "no-config.yaml")
      prevCfg, hadCfg := os.LookupEnv("weave_CONFIG")  // LOWERCASE
      if err := os.Setenv("weave_CONFIG", cfgGhost); err != nil {
          tb.Fatalf("setenv weave_CONFIG: %v", err)
      }
      tb.Cleanup(func() {
          if hadCfg {
              _ = os.Setenv("weave_CONFIG", prevCfg)
          } else {
              _ = os.Unsetenv("weave_CONFIG")
          }
      })
  - NOTE: env var name is LOWERCASE "weave_CONFIG" (config.go const configEnv), NOT
    "WEAVE_CONFIG". os.Setenv is case-sensitive on Linux.
  - PRESERVE: the existing "Do NOT call t.Parallel() — mutates env" comment, t.Helper(),
    and the envVar neutralization block.

Task 7: MODIFY internal/extdir/extdir_test.go — add the makeExtension helper
  - ADD the makeExtension helper (see Documentation & References pattern block). Writes
    extensions/<tag>/<tag>.ts (a case-(a) single-file entry recognized by HasExtensionEntry).
    Returns the extensions/ dir path. t.Helper().
  - PLACEMENT: near the top of the test file, after unsetEnvVar (and alongside makeFakeBinary
    if S2 added it). Do NOT name it makeSkill.
  - GOTCHA: the entry file is <tag>.ts, NOT index.ts (index.* are dir markers, case a
    excludes them) and NOT SKILL.md (that's skilldozer's marker, not recognized by weave).

Task 8: MODIFY internal/extdir/extdir_test.go — add findWalkUpAncestor tests
  - PORT verbatim from skilldozer with renames ("skills"→"extensions", "SKILL.md"→
    "<tag>.ts", "makeSkill"→"makeExtension"). The 6 tests:
      TestFindWalkUpAncestorAtStart          (start IS the repo → extensions at start wins;
        no ascent needed — loop body runs before ascent)
      TestFindWalkUpAncestorDeep             (extensions several levels up; MkdirAll a deep
        subdir as start; ascent finds the repo's extensions/)
      TestFindWalkUpAncestorNestedEntry      (renamed from NestedSkillMD; a nested
        extensions/x/y/foo.ts counts — HasExtensionEntry recurses, case a)
      TestFindWalkUpAncestorSkipsEmptyAndContinues  (CONTRACT: a lower extensions/ dir
        with NO entries is skipped; ascent continues to a higher ancestor with real entries)
      TestFindWalkUpAncestorNoExtensions     (renamed from NoSkills; no extensions anywhere
        up to root → miss)
      TestFindWalkUpAncestorExtensionsIsFile (renamed from SkillsIsFile; "extensions" is a
        regular FILE → miss, IsDir guard)
  - Each test uses t.TempDir() + makeExtension + os.MkdirAll. No env mutation, so these
    CAN call t.Parallel() — but for consistency with the rest of the file, omit it (the
    file's convention is no t.Parallel on env tests; these aren't env tests, so parallel
    is allowed but not required).

Task 9: MODIFY internal/extdir/extdir_test.go — add findWalkUp test
  - PORT TestFindWalkUpFindsAncestor (uses t.Chdir). makeExtension in a temp root, MkdirAll
    a sub dir, t.Chdir(sub), call findWalkUp(), assert found=true, src=SourceWalkUp,
    dir=<root>/extensions. This exercises os.Getwd() → ascent end-to-end.
  - DOC COMMENT noting t.Chdir (Go 1.24+, go.mod 1.25) restores cwd on cleanup, so the
    test is hermetic. Do NOT call t.Parallel() — t.Chdir mutates process-global cwd.

Task 10: MODIFY internal/extdir/extdir_test.go — add Find tests
  - PORT verbatim from skilldozer with renames. The 4 tests:
      TestFindRuleEnvWins        (unsetEnvVar(t); t.Setenv(envVar, dir); Find() → SourceEnv,
        nil err, dir=Clean(dir). Proves rule 1 wins when env is set.)
      TestFindRuleWalkUpWins     (unsetEnvVar(t); makeExtension; t.Chdir(sub); Find() →
        SourceWalkUp, nil err. Relies on findSibling deterministically missing in the test
        binary's temp build dir — same rationale as S2's smoke test.)
      TestFindAllMissReturnsErrNotFound  (unsetEnvVar(t); t.Chdir(t.TempDir()); Find() →
        errors.Is(err, ErrNotFound)==true, dir=="", src==0. The walk ascends to / which has
        no extensions/. unsetEnvVar MUST neutralize weave_CONFIG or a real config leaks.)
      TestErrNotFoundMessageHasFix  (asserts ErrNotFound.Error() contains "run" and
        "weave init". Imports "errors" and "strings" if not already imported.)
  - Each Find test calls unsetEnvVar(t) FIRST (to neutralize envVar AND weave_CONFIG).
  - The walk-up / all-miss tests use t.Chdir — do NOT call t.Parallel() on them.
  - IMPORTS: the test file may need "errors" (for errors.Is) and "strings" (for
    strings.Contains in TestErrNotFoundMessageHasFix). Check the existing import block
    and add them if missing (S1's test file imports only os, path/filepath, testing).

Task 11: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...           # expect exit 0
    (REQUIRES S2 to have landed findConfig/findSibling — if not, see GOTCHA above)
  - RUN: go build ./internal/extdir/                                # standalone, exit 0
  - RUN: go vet ./internal/extdir/                                  # expect exit 0, clean
  - RUN: go test ./internal/extdir/ -v                              # expect ALL PASS
    (S1 + S2 + S3 suites)
  - RUN: go test -race ./internal/extdir/                           # expect no data races
  - RUN: grep -rn "yaml.v3\|gopkg.in" --include=*.go .              # expect nothing
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK          # expect OK (no require)
  - RUN: test ! -f go.sum && echo OK || echo FAIL                   # expect OK (no go.sum)
  - RUN: ! grep -q '"skills"' internal/extdir/extdir.go && echo OK  # expect OK (no "skills")
  - RUN: grep -q 'weave is not configured' internal/extdir/extdir.go && echo OK  # expect OK
  - EXPECT: build clean, vet clean, all tests pass, no third-party import, no require block,
    no go.sum, no leftover "skills" literal, ErrNotFound string present verbatim.
```

### Implementation Patterns & Key Details

```go
// findWalkUpAncestor — PORT skilldozer's body VERBATIM, with the two documented renames.
// PRD §8.3 rule 4 ascent core. Testable (takes start as a param; os.Getwd is not
// controllable without chdir). Checks start FIRST; skips empty extensions/ and continues.
func findWalkUpAncestor(start string) (dir string, found bool) {
	cur := filepath.Clean(start)
	for {
		candidate := filepath.Join(cur, "extensions") // weave: "extensions" not "skills"
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if HasExtensionEntry(candidate) { // weave: HasExtensionEntry not HasSkillMD
				return candidate, true
			}
			// extensions/ exists here but has no entries -> keep ascending.
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false // reached filesystem root, no match
		}
		cur = parent
	}
}

// findWalkUp — PORT skilldozer's body VERBATIM. Thin entry; testable core is above.
func findWalkUp() (dir string, src Source, found bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, false // cwd unresolvable -> rule misses
	}
	d, ok := findWalkUpAncestor(cwd)
	if !ok {
		return "", 0, false
	}
	return d, SourceWalkUp, true
}

// ErrNotFound — EXACT PRD §8.2/§6.4 string. main prints err.Error() verbatim to stderr.
var ErrNotFound = errors.New("weave is not configured; run `weave init`")

// Find — PORT skilldozer's body VERBATIM. The public entrypoint. NEVER prompts.
func Find() (dir string, src Source, err error) {
	if d, s, ok := findEnv(); ok {
		return d, s, nil
	}
	if d, s, ok := findConfig(); ok { // PRD §8.3 priority #2 (S2)
		return d, s, nil
	}
	if d, s, ok := findSibling(); ok { // PRD §8.3 priority #3 (S2)
		return d, s, nil
	}
	if d, s, ok := findWalkUp(); ok {
		return d, s, nil
	}
	return "", 0, ErrNotFound
}

// makeExtension (test helper) — writes extensions/<tag>/<tag>.ts (case-a entry).
func makeExtension(t *testing.T, dir, tag string) string {
	t.Helper()
	ext := filepath.Join(dir, "extensions")
	entry := filepath.Join(ext, tag)
	if err := os.MkdirAll(entry, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", entry, err)
	}
	if err := os.WriteFile(filepath.Join(entry, tag+".ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s.ts: %v", tag, err)
	}
	return ext
}

// GOTCHA (loop termination): `parent == cur` is how filepath.Dir signals the FS root
// (filepath.Dir("/") == "/"). Do NOT replace with a depth counter or a HasPrefix check.

// GOTCHA (test hermeticity): t.Chdir (Go 1.24+; go.mod is 1.25) changes cwd for the test
// and restores on cleanup. findWalkUp and Find walk-up tests use it. Do NOT call
// t.Parallel() on t.Chdir tests — cwd is process-global.
```

### Integration Points

```yaml
DATABASE:
  - none. Pure filesystem + env + intra-module config call (via findConfig).

CONFIG:
  - Find() calls findConfig() (S2), which reads config.Path()/config.Load(). S3 does NOT
    touch config directly. The contract: findConfig returns ("", 0, false) on ANY error
    or Store=="", so Find falls through to rule 3 / rule 4.

ROUTES / API:
  - none. weave is a CLI; this package has no handlers. Consumed by:
      * main.run (P1.M1.T4) — calls Find() to locate the store; on ErrNotFound prints
        err.Error() to stderr and exits 1 (PRD §8.2/§6.4).
      * discover.Index (P1.M2.T3) — calls Find() to get the dir to walk.

MODULE:
  - S3 adds NO import. The stdlib packages it needs (errors, os, path/filepath) are
    already imported by S1's extdir.go. go.mod gains NO require block; no go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave

# Format check.
gofmt -l internal/extdir/
# EXPECTED: no output. If a file is listed, run `gofmt -w internal/extdir/` and re-check.

# go vet.
go vet ./internal/extdir/
# EXPECTED: exit 0, no output.

# No third-party deps leaked in (the only non-stdlib import should be internal/config from S2).
grep -rn "yaml.v3\|gopkg.in" --include=*.go . || echo "no third-party import (correct)"
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
# EXPECTED: "no third-party import (correct)" and "no require block (correct)".

# No leftover "skills" literal (the single most likely copy-paste bug from skilldozer).
! grep -q '"skills"' internal/extdir/extdir.go && echo "no 'skills' literal (correct)" \
  || echo "FAIL: leftover 'skills' literal in extdir.go"
# EXPECTED: "no 'skills' literal (correct)".

# ErrNotFound string present verbatim (PRD §8.2/§6.4).
grep -q 'weave is not configured; run `weave init`' internal/extdir/extdir.go \
  && echo "ErrNotFound string present (correct)" || echo "FAIL: ErrNotFound string missing/wrong"
# EXPECTED: "ErrNotFound string present (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the extdir package tests verbosely (S1 + S2 + S3 suites).
go test ./internal/extdir/ -v
# EXPECTED: ALL tests pass. Pay special attention to:
#   - TestFindWalkUpAncestorSkipsEmptyAndContinues (CONTRACT: empty extensions/ skipped)
#   - TestFindWalkUpAncestorAtStart (loop body runs before ascent)
#   - TestFindWalkUpFindsAncestor (t.Chdir + os.Getwd end-to-end)
#   - TestFindRuleWalkUpWins (Find combiner, rule 4 wins after 1-3 miss)
#   - TestFindAllMissReturnsErrNotFound (errors.Is(err, ErrNotFound); hermetic via unsetEnvVar)
#   - TestErrNotFoundMessageHasFix (exact user-facing fix substring)
# If any fail, read the failure, fix extdir.go (NOT the test — tests encode the contract).

# Race detector sanity.
go test -race ./internal/extdir/
# EXPECTED: passes, no data races.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (config from P1.M1.T2.S1 + extdir from S1+S2+S3 must all compile).
go build ./...
# EXPECTED: exit 0. NOTE: this REQUIRES S2 to have landed findConfig/findSibling —
# if S2 hasn't landed, this fails with "undefined: findConfig/findSibling" inside Find().
# That is expected (S2+S3 complete T3 together); do NOT stub those functions.

# Standalone package build.
go build ./internal/extdir/
# EXPECTED: exit 0 (same S2 caveat).

# Confirm ErrNotFound is exported and Find is the public entrypoint.
grep -q 'var ErrNotFound' internal/extdir/extdir.go && echo "ErrNotFound exported (correct)" \
  || echo "FAIL: ErrNotFound missing"
grep -q 'func Find()' internal/extdir/extdir.go && echo "Find present (correct)" \
  || echo "FAIL: Find missing"

# Confirm no go.sum was generated.
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum exists — a dep leaked in"

# go mod tidy is a no-op on a zero-external-dep module.
go mod tidy
git diff --stat go.mod   # expect: empty / unchanged
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove the walk-up-from-cwd scenario end-to-end via a throwaway main,
# mirroring PRD §8.3 rule 4's "go run / dev" rationale. This is informational — the unit
# tests (TestFindWalkUpFindsAncestor, TestFindRuleWalkUpWins) are the authoritative gate.
tmp=$(mktemp -d)
mkdir -p "$tmp/repo/extensions/foo" "$tmp/repo/sub"
echo 'x' > "$tmp/repo/extensions/foo/foo.ts"
mkdir -p /tmp/weave-find-smoke
cat > /tmp/weave-find-smoke/main.go <<GO
package main
import (
    "fmt"
    "github.com/dabstractor/weave/internal/extdir"
)
func main() {
    d, src, err := extdir.Find()
    if err != nil {
        fmt.Println("err:", err)
        return
    }
    fmt.Printf("dir=%s src=%s\\n", d, src)
}
GO
# Build a throwaway module that replace-resolves the local weave, run from the repo subdir.
( cd /tmp/weave-find-smoke && go mod init smoke 2>/dev/null
  echo "replace github.com/dabstractor/weave => $tmp/repo-not-needed" # placeholder
  go run main.go 2>&1 | head -5 ) || true
# EXPECTED (if wired): the smoke prints dir=<tmp>/repo/extensions src=ancestor of cwd.
# In practice the unit tests cover this hermetically — the smoke is optional confidence.
rm -rf "$tmp" /tmp/weave-find-smoke

# Cross-check the exact ErrNotFound byte sequence (PRD §8.2/§6.4 — literal backticks matter).
go test ./internal/extdir/ -run TestErrNotFoundMessageHasFix -v
# EXPECTED: PASS. The test asserts the message contains "run" and "weave init"; combined
# with the Level 1 grep for the full string, the exact bytes are pinned.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l internal/extdir/` empty; `go vet ./internal/extdir/` clean;
      no third-party import; no `require` block in go.mod; no leftover `"skills"` literal;
      ErrNotFound string present verbatim.
- [ ] Level 2 passed: `go test ./internal/extdir/ -v` — ALL tests pass (S1 + S2 + S3 suites),
      including the skip-empty walk-up test, the t.Chdir walk-up test, and the
      errors.Is(err, ErrNotFound) all-miss test.
- [ ] Level 3 passed: `go build ./...` exit 0 (S2 landed); `go build ./internal/extdir/`
      exit 0; ErrNotFound + Find present and exported; no go.sum; `go mod tidy` is a no-op.
- [ ] `go test -race ./internal/extdir/` passes (no data races).

### Feature Validation

- [ ] `findWalkUpAncestor` exists, ports skilldozer's body verbatim with the two documented
      renames (`"extensions"`, `HasExtensionEntry`), checks start first, skips empty
      `extensions/` and continues, terminates via `parent == cur`.
- [ ] `findWalkUp` exists, ports skilldozer's body verbatim, calls `os.Getwd()`, delegates
      to `findWalkUpAncestor`, returns `(dir, SourceWalkUp, true)` on hit — never errors.
- [ ] `ErrNotFound` is declared with the EXACT PRD §8.2/§6.4 string (literal backticks
      around `weave init`).
- [ ] `Find` exists, tries the four rules in priority order, returns the first hit as
      `(absDir, src, nil)`, returns `("", 0, ErrNotFound)` on total miss, and contains NO
      prompting / isatty / auto-init logic.
- [ ] The walk-up-from-cwd scenario works via `t.Chdir` (pinned by TestFindWalkUpFindsAncestor
      and TestFindRuleWalkUpWins).

### Code Quality Validation

- [ ] Package doc comment updated: rules 2-4 + Find + ErrNotFound annotations changed from
      [S2]/[S3] to "implemented"; the findEnv no-EvalSymlinks note and HasExtensionEntry
      dual-role note preserved.
- [ ] Mirrors skilldozer's findWalkUpAncestor/findWalkUp/ErrNotFound/Find structure verbatim
      (with the two documented renames + the ErrNotFound string change).
- [ ] unsetEnvVar neutralizes BOTH envVar and weave_CONFIG (idempotent with S2's upgrade).
- [ ] File placement: `internal/extdir/extdir.go` (modified) + `internal/extdir/extdir_test.go`
      (modified); no new files.
- [ ] Anti-patterns avoided: no `fmt.Errorf` wrapping of ErrNotFound; no third-party import;
      no new import line; no modification to S1/S2 symbols, config, or main.go; no `init()`;
      no isatty/auto-init logic in Find; no depth-counter loop termination.

### Documentation & Deployment

- [ ] Package is self-documenting via the updated doc comment (no separate README — internal
      package). Per the item contract: "DOCS: none — internal package; resolution rules
      documented in README §7 section (Mode B final task)."
- [ ] No new user-facing env vars introduced by this subtask.

---

## Anti-Patterns to Avoid

- ❌ Don't leave the sibling dir literal as `"skills"` in `findWalkUpAncestor`. It MUST be
  `"extensions"` — one of the two documented renames. Pinned by `grep -q '"skills"'` in
  Level 1 and by every findWalkUpAncestor test asserting the returned dir ends in `/extensions`.
- ❌ Don't call `HasSkillMD`. It MUST be `HasExtensionEntry` (the S1 predicate) — the second
  documented rename. `HasSkillMD` does not exist in weave; this is a compile error, not a
  silent bug, but worth flagging.
- ❌ Don't reword `ErrNotFound`. The string `weave is not configured; run \`weave init\`` is
  pinned verbatim by PRD §8.2/§6.4 and TestErrNotFoundMessageHasFix. Literal backticks around
  `weave init` are part of the message.
- ❌ Don't add prompting / isatty / auto-init logic to `Find`. PRD §8.2 prompt safety is
  load-bearing: the bare `weave <tag>` path never prompts. Find returns ErrNotFound; main
  prints it and exits 1. The ONLY place weave prompts is `weave init` (P1.M4.T4).
- ❌ Don't use a depth counter, max-iterations cap, or `strings.HasPrefix(cur, "/")` to
  terminate the ascent loop. Use `parent == cur` (the `filepath.Dir(root) == root` idiom).
  It ports verbatim from skilldozer and is the portable stdlib way.
- ❌ Don't check ancestors in the wrong order. `findWalkUpAncestor` checks the START dir
  FIRST (the loop body runs before any ascent), then ascends. Pinned by
  TestFindWalkUpAncestorAtStart.
- ❌ Don't treat an empty `extensions/` dir as a match. PRD §8.3 qualifies the match with
  "at least one extension entry" — the `HasExtensionEntry(candidate)` guard inside the IsDir
  branch enforces this; ascent continues past an empty extensions/. Pinned by
  TestFindWalkUpAncestorSkipsEmptyAndContinues.
- ❌ Don't use `os.Chdir` + manual `tb.Cleanup(restore)` in tests. Use `t.Chdir` (Go 1.24+;
  go.mod is 1.25) — it's the stdlib-blessed, cleanup-safe, race-safe way.
- ❌ Don't call `t.Parallel()` on `t.Chdir` tests or env-mutating tests. cwd and env are
  process-global.
- ❌ Don't forget the `unsetEnvVar` upgrade (neutralize `weave_CONFIG`, lowercase). Without
  it, `TestFindAllMissReturnsErrNotFound` and `TestFindRuleEnvWins` leak a real machine
  config and become flaky. S2 specifies the same upgrade; if S2 landed it, S3 is a no-op
  here (idempotent).
- ❌ Don't write the makeExtension entry as `SKILL.md` or `index.ts`. It MUST be `<tag>.ts`
  (a case-(a) single-file extension per PRD §7.1, recognized by HasExtensionEntry). `index.*`
  are dir markers (case a excludes them); `SKILL.md` is skilldozer's marker, unrecognized
  by weave.
- ❌ Don't stub `findConfig` or `findSibling` to make S3 build in isolation. S2 and S3 are a
  parallel pair; S3's `Find()` calls them. If S2 hasn't landed, `go build ./internal/extdir/`
  fails with "undefined: findConfig/findSibling" — that's expected. Do NOT add placeholder
  definitions; let S2 land and the build resolves.
- ❌ Don't add a new import. The stdlib packages S3 needs (`errors`, `os`, `path/filepath`)
  are already imported by S1's extdir.go. The test file may need `errors` and `strings`
  (for errors.Is and strings.Contains) — add those to the test file's import block only if
  not already present.
- ❌ Don't modify `findEnv`, `HasExtensionEntry`, `Source`, `String`, `findConfig`,
  `findSibling`, `resolveSiblingFromExe`, the config package, or main.go. S3's blast radius
  is extdir.go (append 4 symbols) + extdir_test.go (append 1 helper + ~11 tests + ensure
  unsetEnvVar upgrade).
- ❌ Don't lower the test bar by editing tests to make them pass. The tests encode the
  contract; if a test fails, fix `extdir.go`.
