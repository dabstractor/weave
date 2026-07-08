# PRP — P1.M1.T3.S2: Config-based resolution (rule 2) + sibling-of-binary (rule 3, symlink-aware)

## Goal

**Feature Goal**: Extend `internal/extdir` (created by P1.M1.T3.S1) with **rule 2**
(`findConfig`: resolve the extensions dir from the config file's `store` key, PRD §8.1/§8.3
priority 2) and **rule 3** (`findSibling` + `resolveSiblingFromExe`: locate the `extensions/`
sibling of the running binary, symlink-aware via `os.Executable()` + `filepath.EvalSymlinks()`,
PRD §8.3 priority 3). These are the two middle rules of the §8.3 first-hit-wins order; they
feed the `Find()` combiner that lands in P1.M1.T3.S3.

This is the weave port of skilldozer's `internal/skillsdir/skillsdir.go` (rules 2 and 3
portions only), per `architecture/architecture_mapping.md §2`. The item contract pins it
explicitly: "Port skilldozer's `findConfig` and `findSibling`/`resolveSiblingFromExe` verbatim
with renames. `EvalSymlinks` is REQUIRED on macOS (redundant on Linux via `/proc/self/exe`) —
do NOT drop it."

**Deliverable**: Modify `internal/extdir/extdir.go` (add the `internal/config` import and
the two new unexported functions `findConfig` and `findSibling`/`resolveSiblingFromExe`) and
modify `internal/extdir/extdir_test.go` (upgrade `unsetEnvVar` to also neutralize the config
rule, and add the rule-2 and rule-3 test suites ported from skilldozer with renames). No new
files; no changes to go.mod (the `internal/config` import is intra-module, no `require`).

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./internal/extdir/` exits 0 (clean).
- `go test ./internal/extdir/ -v` passes ALL tests — both the S1 suite (unchanged, still
  green) and the newly added S2 suites for `findConfig`, `findSibling`, and
  `resolveSiblingFromExe`.
- The symlink-install scenario test passes: a temp "binary" (1-byte regular file) with a
  sibling `extensions/` dir, PLUS a symlink to that binary placed in a DIFFERENT temp dir,
  resolves via `resolveSiblingFromExe(symlinkPath)` back to the REAL binary's repo dir's
  `extensions/` — not the symlink's dir. This is what makes `~/.local/bin/weave ->
  ~/projects/weave/weave` resolve back to the repo (PRD §12.1 symlink install rationale).
- `go.mod` still has no `require` block; no `go.sum` exists.

## User Persona (if applicable)

N/A — internal package consumed by `Find()` (P1.M1.T3.S3) and eventually `main.go`. No
direct user surface.

## Why

- PRD §8.3 makes the config-file `store` (rule 2) **the primary** resolution rule — it is
  what `weave init` (P1.M4.T4) writes. Without rule 2, a `go install` user (PRD §12.2) who
  ran `weave init` once has no way for that configuration to take effect.
- PRD §12.1's entire install design rests on rule 3: `install.sh` creates a **symlink**
  `<target>/weave → <repo>/weave` precisely so "the sibling-of-binary rule (§8.3) then gives
  a zero-config default store (the repo's own `extensions/`), and `git pull && go build`
  updates the linked binary in place." If rule 3 is missing or broken, the symlink install
  silently degrades to "unconfigured" for clone-and-build users.
- Rule 3's symlink-awareness is the single most platform-sensitive line in the codebase:
  on Linux `os.Executable()` already resolves symlinks via `/proc/self/exe` (so
  `EvalSymlinks` is redundant), but on macOS `os.Executable()` may return the symlink path
  and rule 3 SILENTLY misses without `EvalSymlinks`. The item contract and
  `architecture_mapping.md §2` both forbid dropping it.
- S1 deliberately deferred rules 2-4 + `Find()` to keep each PRP's blast radius small.
  This subtask closes rules 2 and 3; S3 adds rule 4 (walk-up) and the `Find()` combiner.

## What

Two new unexported functions added to the existing `internal/extdir/extdir.go`:

1. `findConfig() (dir string, src Source, found bool)` — rule 2. Calls `config.Path()` for
   the bootstrap path; on error returns miss. Calls `config.Load(path)`; on error or
   `f.Store == ""` returns miss. Resolves a relative `store` against `filepath.Dir(path)`
   (the config file's dir, NOT cwd). If the resolved store is an existing dir, returns
   `(store, SourceConfig, true)`. **Never errors** — any failure falls through.
2. `findSibling() (dir string, src Source, found bool)` + `resolveSiblingFromExe(exe string)
   (dir string, found bool)` — rule 3. `findSibling` calls `os.Executable()`; on error
   returns miss; otherwise delegates to `resolveSiblingFromExe(exe)`. That helper does
   `real, err := filepath.EvalSymlinks(exe)` (on error `real = exe`), `repoDir := filepath.Dir(real)`,
   `candidate := filepath.Join(repoDir, "extensions")`, and wins iff `os.Stat(candidate)`
   reports an existing dir.

One new import added to `extdir.go`: `github.com/dabstractor/weave/internal/config`. The
stdlib imports from S1 (`encoding/json`, `errors`, `io/fs`, `os`, `path/filepath`, `strings`)
are unchanged.

### Success Criteria

- [ ] `findConfig` exists with signature `func findConfig() (dir string, src Source, found bool)`,
      ports skilldozer's body verbatim (only the `config` import path differs), and returns
      `("", 0, false)` on ANY error or non-dir store — never propagates an error.
- [ ] `findConfig` resolves a relative `store` against `filepath.Dir(configPath)` (the config
      file's dir), NOT against cwd. Absolute `store` values are passed through `filepath.Clean`.
- [ ] `findSibling` exists with signature `func findSibling() (dir string, src Source, found bool)`,
      ports skilldozer's body verbatim, calls `os.Executable()`, delegates to
      `resolveSiblingFromExe`.
- [ ] `resolveSiblingFromExe` exists with signature `func resolveSiblingFromExe(exe string) (dir string, found bool)`,
      uses the sibling dir name `"extensions"` (NOT `"skills"`), and **calls
      `filepath.EvalSymlinks(exe)`** with the `real = exe` fallback on error.
- [ ] `go build ./...`, `go vet ./internal/extdir/`, and `go test ./internal/extdir/` pass,
      including the symlink-cross-dir test.
- [ ] go.mod has no `require` block; no go.sum exists; the only non-stdlib import in
      `extdir.go` is `github.com/dabstractor/weave/internal/config`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** This PRP gives the exact source-of-truth file to port
(`internal/skillsdir/skillsdir.go`, with the two functions quoted verbatim), the exact
rename table, the exact test patterns to mirror (with the test names listed), the exact
new import, the verified macOS-EvalSymlinks rationale, and the verified build/vet/test gates.
The previous PRP (S1) establishes the exact package skeleton, the `Source` constants, and
the `unsetEnvVar` helper that S2 upgrades.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for §8.1 (config file format/semantics), §8.3 priority order
       (rule 2 = config `store`, rule 3 = sibling-of-binary symlink-aware), §12.1
       (install.sh symlink rationale that rule 3 enables).
  critical: §8.3 rule 3 says "sibling of the running binary (symlink-aware:
            os.Executable() + filepath.EvalSymlinks())". §12.1 "Why symlink, not copy"
            explains rule 3's purpose. §8.1 "A missing or unreadable config is treated as
            'not yet configured' and falls through … never a hard error" governs findConfig's
            error-to-fallthrough behavior.

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir.go
  why: PRIMARY pattern to port — findConfig, findSibling, and resolveSiblingFromExe all
       port VERBATIM. Only two things change for weave: (1) the sibling dir literal
       "skills" → "extensions" inside resolveSiblingFromExe, and (2) the config import path
       (github.com/dabstractor/skilldozer/internal/config → github.com/dabstractor/weave/internal/config).
  pattern: |
    func findConfig() (dir string, src Source, found bool) {
        p, err := config.Path()
        if err != nil { return "", 0, false }
        f, err := config.Load(p)
        if err != nil { return "", 0, false }
        if f.Store == "" { return "", 0, false }
        var store string
        if filepath.IsAbs(f.Store) {
            store = filepath.Clean(f.Store)
        } else {
            store = filepath.Join(filepath.Dir(p), f.Store)  // relative to config file's dir
        }
        info, err := os.Stat(store)
        if err != nil || !info.IsDir() { return "", 0, false }
        return store, SourceConfig, true
    }
    func findSibling() (dir string, src Source, found bool) {
        exe, err := os.Executable()
        if err != nil { return "", 0, false }
        d, ok := resolveSiblingFromExe(exe)
        if !ok { return "", 0, false }
        return d, SourceSibling, true
    }
    func resolveSiblingFromExe(exe string) (dir string, found bool) {
        real, err := filepath.EvalSymlinks(exe)
        if err != nil { real = exe }              // EvalSymlinks-error fallback (REQUIRED)
        repoDir := filepath.Dir(real)
        candidate := filepath.Join(repoDir, "extensions")  // weave: "extensions" not "skills"
        info, err := os.Stat(candidate)
        if err != nil || !info.IsDir() { return "", false }
        return candidate, true
    }
  gotcha: skilldozer uses yaml.v3 so a malformed config is a HARD error from config.Load,
          which findConfig converts to fall-through. weave's config (P1.M1.T2.S1) uses a
          hand-rolled line scanner: a present-but-no-"store:"-line file returns File{}, nil
          (NOT an error), so weave's findConfig never sees a "malformed" hard error — the
          "unparseable" case collapses to File{Store:""} → the f.Store == "" branch →
          fall-through. Same end behavior; do NOT add special "malformed" handling.

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir_test.go
  why: PRIMARY test pattern to port. Port these tests verbatim with renames
       ("skills"→"extensions", "SKILLDOZER_CONFIG"→"weave_CONFIG"):
         TestResolveSiblingFromExeSymlinkCrossDir (THE critical symlink test),
         TestResolveSiblingFromExeDirect, TestResolveSiblingFromExeEvalSymlinksFallback,
         TestResolveSiblingFromExeNoSkillsDir→TestResolveSiblingFromExeNoExtensionsDir,
         TestResolveSiblingFromExeSkillsIsFile→TestResolveSiblingFromExeExtensionsIsFile,
         TestFindSiblingNoSkillsNextToTestBinary→TestFindSiblingNoExtensionsNextToTestBinary,
         TestFindConfigHit, TestFindConfigMissingFile, TestFindConfigMissingStoreKey,
         TestFindConfigStoreDirAbsent, TestFindConfigRelativeStoreResolvedAgainstConfigDir,
       plus ADAPT TestFindConfigMalformedYAML → TestFindConfigMalformedSyntax (weave's parser
       has no hard-error case, so the test asserts a garbage-content file with no "store:"
       line falls through — equivalent to MissingStoreKey for weave's parser shape).
       Also UPGRADE the unsetEnvVar helper to also neutralize weave_CONFIG (see Known Gotchas).
  pattern: |
    // makeFakeBinary — port verbatim. EvalSymlinks + os.Stat on the sibling do NOT
    // require a real ELF executable; a 1-byte regular file suffices (skilldozer research §5).
    func makeFakeBinary(t *testing.T, dir, name string) string {
        t.Helper()
        p := filepath.Join(dir, name)
        if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
            t.Fatalf("write fake binary %s: %v", p, err)
        }
        return p
    }
  gotcha: Do NOT port makeSkill (it writes SKILL.md for walk-up/Find tests — those are S3).
          Do NOT port TestFindEnvBeatsConfig / TestFindRuleEnvWins / TestFindRuleWalkUpWins /
          TestFindAllMissReturnsErrNotFound / TestErrNotFoundMessageHasFix — those exercise
          Find() and ErrNotFound, which belong to S3. Port ONLY the rule-2 + rule-3 tests
          listed above.

- file: /home/dustin/projects/weave/internal/config/config.go  (from P1.M1.T2.S1, COMPLETE)
  why: The CONTRACT for the config API S2 consumes. Confirms the exact signatures of
       config.Path() (string, error) and config.Load(path) (File{Store string}, error),
       and that Load returns the raw os.ReadFile error verbatim (so a missing file is
       errors.Is-able with fs.ErrNotExist, though findConfig does not need to inspect the
       error type — ANY error → fall-through).
  critical: config.Load returns File{Store: ""} for (a) a missing "store:" line and
            (b) a present-but-empty "store:" value. findConfig's `f.Store == ""` guard
            handles both. The relative-vs-absolute split uses filepath.IsAbs(f.Store);
            relative values join to filepath.Dir(p) where p is the config file's path
            returned by config.Path().

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §2 maps skilldozer skillsdir → weave extdir line by line and pins the renames.
  critical: §2 "Port resolveSiblingFromExe verbatim — only the sibling dir name changes:
            extensions instead of skills." §2 "Rule 3 sibling-aware: os.Executable() +
            filepath.EvalSymlinks()." Confirms S2's scope (rules 2-3) and that S3 owns
            rule 4 + Find.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/pi_extension_facts.md
  why: §1/§2 background only — confirms why a sibling `extensions/` dir is the right
       default (pi loads via -e <path>, and the repo's own extensions/ is a valid store).
       No direct code dependency for S2; included for rationale continuity.

- file: /home/dustin/projects/skilldozer/plan/001_fcde63e5bb60/architecture/verified_symlink_resolution.md
  why: The empirically-verified rationale for why EvalSymlinks MUST stay. Documents the
       Linux /proc/self/exe behavior (os.Executable already returns the real path) vs
       macOS (os.Executable may return the symlink path → EvalSymlinks is REQUIRED).
  critical: Conclusion #2: "EvalSymlinks is redundant-but-harmless on Linux, and NECESSARY
            on macOS. The PRD's two-call sequence os.Executable() → filepath.EvalSymlinks()
            → filepath.Dir() is CORRECT and cross-platform. Implement exactly that; do not
            'simplify' by dropping EvalSymlinks (breaks macOS)."

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M1T3S1/PRP.md
  why: The CONTRACT for the extdir package skeleton S2 extends. Establishes: the Source
       type + four iota constants (SourceEnv, SourceConfig, SourceSibling, SourceWalkUp)
       already exist; String() already returns the four PRD §8.3 labels; findEnv + envVar
       + HasExtensionEntry already exist; the unsetEnvVar helper currently ONLY neutralizes
       envVar (S2 UPGRADES it to also neutralize weave_CONFIG); the package doc comment
       already describes the §8.3 priority order and notes rules 2-4 + Find are added by
       S2/S3.
  critical: S1's extdir.go imports {encoding/json, errors, io/fs, os, path/filepath, strings}
            and does NOT import internal/config. S2 ADDS the config import. S1's
            unsetEnvVar must be upgraded (see Known Gotchas) or the new findConfig tests
            that expect a miss could leak a real machine config into the result.

- file: /home/dustin/projects/weave/go.mod  (from P1.M1.T1.S1)
  why: Confirms module `github.com/dabstractor/weave`, `go 1.25`, no require block. The
       new import `github.com/dabstractor/weave/internal/config` is intra-module — Go
       resolves it from the local source tree, so NO require line is added and NO go.sum
       is generated.
```

### Current Codebase tree

```bash
# After P1.M1.T1.S1 (Complete) + P1.M1.T2.S1 (Complete) + P1.M1.T3.S1 (in parallel, "Implementing"):
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
└── extdir/             # P1.M1.T3.S1 (parallel) — Source/String/envVar/findEnv/HasExtensionEntry
    ├── extdir.go
    └── extdir_test.go
plan/
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                      # UNCHANGED by this subtask
│   └── extdir/
│       ├── extdir.go                # MODIFIED — add config import + findConfig + findSibling + resolveSiblingFromExe
│       └── extdir_test.go           # MODIFIED — upgrade unsetEnvVar; add rule-2 + rule-3 test suites
├── go.mod                           # UNCHANGED (no new require — internal/config is intra-module)
└── ...                              # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: EvalSymlinks MUST stay in resolveSiblingFromExe. On Linux os.Executable()
// resolves the symlink via /proc/self/exe (so EvalSymlinks is redundant-but-harmless),
// but on macOS os.Executable() may return the SYMLINK path, and without EvalSymlinks
// filepath.Dir() points at the bin dir (e.g. ~/.local/bin) instead of the repo — rule 3
// SILENTLY misses. The item contract + architecture_mapping.md §2 + verified_symlink_resolution.md
// all forbid dropping it. Pinned by TestResolveSiblingFromExeSymlinkCrossDir.

// CRITICAL: the sibling dir literal is "extensions" (NOT "skills"). This is the ONLY
// behavioral change from skilldozer's resolveSiblingFromExe. A leftover "skills" string
// is the single most likely copy-paste bug — pinned by every resolveSiblingFromExe test
// asserting the returned dir ends in "/extensions".

// CRITICAL: findConfig must NEVER propagate an error. config.Path() or config.Load() may
// return an error (missing/unreadable config, relative $XDG_CONFIG_HOME, $HOME unset); all
// of these → return ("", 0, false) so Find() (S3) falls through to rule 3. PRD §8.1: "A
// missing or unreadable config is treated as 'not yet configured' and falls through … never
// a hard error." Pinned by TestFindConfigMissingFile + TestFindConfigMissingStoreKey +
// TestFindConfigStoreDirAbsent.

// CRITICAL: a relative `store` in the config is resolved against filepath.Dir(configPath)
// (the config FILE's dir), NOT against cwd. PRD §8.1: "store may be relative to the config
// file." A test that chdir's would be fragile; instead the test writes the config into a
// temp cfgDir, sets weave_CONFIG to it, creates cfgDir/mystore, and asserts findConfig
// returns cfgDir/mystore regardless of cwd. Pinned by TestFindConfigRelativeStoreResolvedAgainstConfigDir.

// CRITICAL: unsetEnvVar MUST be upgraded in S2. S1's version only neutralizes envVar
// (weave_EXTENSIONS_DIR). S2 adds findConfig, which reads config.Path() — and config.Path()
// honors weave_CONFIG (lowercase) or falls back to $XDG_CONFIG_HOME/weave/config.yaml. On
// a dev machine with a real ~/.config/weave/config.yaml (or weave_CONFIG set), the new
// findConfig tests that expect a miss would leak that real config and return a real dir.
// Fix: S2's unsetEnvVar ALSO points weave_CONFIG at a non-existent ghost path
// (filepath.Join(tb.TempDir(), "no-config.yaml")) and restores it on cleanup. This mirrors
// skilldozer's unsetEnvVar which already does this for SKILLDOZER_CONFIG. Without this,
// TestFindEnv* (the rule-1 tests from S1) and any future Find() all-miss test (S3) become
// machine-dependent and flaky.

// GOTCHA: os.Executable() cannot be controlled from a test (it returns the test binary's
// own path in a temp go-build dir). The symlink-resolution behavior — the entire point of
// rule 3 — therefore CANNOT be exercised through findSibling() directly. The design splits
// rule 3 into findSibling() (thin entry, calls os.Executable) + resolveSiblingFromExe(exe)
// (testable core that takes the exe path as a parameter). Test resolveSiblingFromExe with
// arbitrary paths via t.TempDir() + os.Symlink. The findSibling() test is a smoke test
// asserting found=false (the test binary's temp dir has no sibling extensions/).

// GOTCHA (verified): a 1-byte regular file suffices as a fake "binary". filepath.EvalSymlinks
// and os.Stat on the sibling "extensions" dir do NOT require a real ELF executable. The
// makeFakeBinary helper writes []byte("x") with mode 0o644. (skilldozer research §5.)

// GOTCHA: weave's config.Load uses a hand-rolled line scanner, NOT yaml.v3. So unlike
// skilldozer (where broken YAML is a HARD error that findConfig converts to fall-through),
// weave has NO "malformed config hard error" case — a present-but-garbage file with no
// "store:" line returns File{Store: ""}, nil, and findConfig's f.Store == "" branch handles
// it. TestFindConfigMalformedSyntax therefore asserts the same outcome as
// TestFindConfigMissingStoreKey (fall-through), via a different file content. Do NOT add
// special malformed-handling logic — the hand-rolled parser already makes it impossible.

// GOTCHA (build): the new import is github.com/dabstractor/weave/internal/config — an
// INTRA-MODULE import. Go resolves it from the local source tree; it does NOT add a line
// to go.mod's require block and does NOT create go.sum. If `go mod tidy` adds anything to
// go.mod, something is wrong (a third-party dep leaked in). The config package is Complete
// per plan_status, so the import resolves.

// GOTCHA (boundaries): do NOT add findWalkUp, findWalkUpAncestor, Find, or ErrNotFound in
// this subtask — they belong to S3. do NOT add HasExtensionEntry tests — those belong to S1
// (done). do NOT modify the config package. do NOT modify main.go. The blast radius of S2
// is exactly: extdir.go (2 functions + 1 import) + extdir_test.go (helper upgrade + ~12 tests).
```

## Implementation Blueprint

### Data models and structure

No new types. S2 consumes the existing `Source` type and its constants (`SourceConfig`,
`SourceSibling`) already declared by S1. No new exported symbols (both new functions are
package-internal, feeding the S3 `Find()` combiner). No new sentinel errors.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/extdir/extdir.go — add the config import
  - LOCATE the import block (currently: encoding/json, errors, io/fs, os, path/filepath, strings).
  - ADD: "github.com/dabstractor/weave/internal/config"  (in its own group below the stdlib block,
    following the standard Go stdlib/3rd-party grouping convention; since this is the only
    non-stdlib import, a single grouped block is also acceptable — match gofmt's preference).
  - VERIFY: the import resolves (config package is Complete per plan_status). `go build ./internal/extdir/`
    must still exit 0 after this change even before the functions are added (Go rejects unused
    imports — so add the import in the SAME edit as Task 2, or the build breaks transiently).

Task 2: MODIFY internal/extdir/extdir.go — add findConfig (rule 2)
  - APPEND findConfig() AFTER findEnv() (logical grouping: rule 1 then rule 2). Port the body
    VERBATIM from the skilldozer pattern in Documentation & References. Signature:
      func findConfig() (dir string, src Source, found bool)
  - DOC COMMENT: 6-12 lines explaining: PRD §8.3 rule 2 / §8.1; config.Path() gives the
    bootstrap path; config.Load reads it; ANY error → fall-through (PRD §8.1 "never a hard
    error"); relative store resolved against filepath.Dir(path); never errors (locked
    per-rule shape). Note the weave-vs-skilldozer difference (no yaml.v3 → no malformed
    hard-error case; File{Store:""} handles it).
  - NAMING: findConfig (lowercase, package-internal — matches findEnv and the locked
    per-rule shape).
  - PLACEMENT: internal/extdir/extdir.go, between findEnv and the HasExtensionEntry block
    (or after HasExtensionEntry — either is fine; keep rule helpers grouped if possible).

Task 3: MODIFY internal/extdir/extdir.go — add findSibling + resolveSiblingFromExe (rule 3)
  - APPEND findSibling() and resolveSiblingFromExe(exe) AFTER findConfig(). Port both bodies
    VERBATIM from the skilldozer pattern, with the SINGLE change: the sibling dir literal
    "skills" → "extensions" inside resolveSiblingFromExe.
  - SIGNATURES:
      func findSibling() (dir string, src Source, found bool)
      func resolveSiblingFromExe(exe string) (dir string, found bool)
  - DOC COMMENT on findSibling: PRD §8.3 rule 3; thin entry calling os.Executable(); the
    testable core lives in resolveSiblingFromExe because os.Executable() is not controllable
    in tests (returns the test binary's own path in a temp go-build dir).
  - DOC COMMENT on resolveSiblingFromExe: the EvalSymlinks sequence; REQUIRED on macOS
    (redundant-but-harmless on Linux via /proc/self/exe); the real=exe fallback on
    EvalSymlinks error; reference architecture/verified_symlink_resolution.md. THIS is the
    rule that makes the §12.1 symlink install work.
  - NAMING: findSibling + resolveSiblingFromExe (lowercase, package-internal).
  - PLACEMENT: internal/extdir/extdir.go, after findConfig.

Task 4: MODIFY internal/extdir/extdir.go — update the package doc comment
  - THE S1 package doc already lists the §8.3 priority order and notes "rules 2-4 + Find
    land in S2/S3". After S2 lands rules 2-3, the doc comment should reflect that rules 2
    and 3 are NOW implemented (only rule 4 + Find remain for S3). Update the bracketed
    annotations from "[S2]" to "implemented below" for rules 2 and 3, and leave "[S3]" on
    rule 4 + Find. Do NOT rewrite the whole comment — surgical update only.
  - PRESERVE: the existing note about findEnv's no-EvalSymlinks contract and the dual role
    of HasExtensionEntry (init auto-detect + walk-up qualifier).

Task 5: MODIFY internal/extdir/extdir_test.go — upgrade unsetEnvVar
  - THE S1 unsetEnvVar(tb) currently only neutralizes envVar (weave_EXTENSIONS_DIR). S2
    MUST also neutralize the config rule: point weave_CONFIG at a non-existent ghost path
    (filepath.Join(tb.TempDir(), "no-config.yaml")) so findConfig deterministically misses
    in all-miss / env-only tests. Mirror skilldozer's unsetEnvVar which does this for
    SKILLDOZER_CONFIG.
  - ADD (after the envVar neutralization block):
      cfgGhost := filepath.Join(tb.TempDir(), "no-config.yaml")
      prevCfg, hadCfg := os.LookupEnv("weave_CONFIG")
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
  - PRESERVE: the existing "Do NOT call t.Parallel() — mutates env" comment and the
    t.Helper() call and the envVar neutralization block.

Task 6: MODIFY internal/extdir/extdir_test.go — add the makeFakeBinary helper
  - ADD the makeFakeBinary helper (port verbatim from skilldozer, see Documentation &
    References pattern block). A 1-byte regular file suffices as a fake binary.
  - PLACEMENT: near the top of the test file, after unsetEnvVar (or alongside the other
    test helpers if S1 added any).

Task 7: MODIFY internal/extdir/extdir_test.go — add resolveSiblingFromExe tests
  - PORT verbatim from skilldozer with renames ("skills"→"extensions"). The 5 tests:
      TestResolveSiblingFromExeSymlinkCrossDir   (THE critical symlink test — symlink in a
        DIFFERENT dir resolves back to the REAL binary's repo dir's extensions/)
      TestResolveSiblingFromExeDirect            (direct non-symlinked binary with sibling extensions/)
      TestResolveSiblingFromExeEvalSymlinksFallback (non-existent exe whose parent HAS sibling
        extensions/ wins via real=exe fallback)
      TestResolveSiblingFromExeNoExtensionsDir    (renamed from NoSkillsDir — binary exists,
        no sibling extensions/ → miss)
      TestResolveSiblingFromExeExtensionsIsFile   (renamed from SkillsIsFile — sibling
        "extensions" is a regular FILE → miss, IsDir guard)
  - Each test uses t.TempDir() + makeFakeBinary + os.Mkdir for the sibling dir, and
    TestResolveSiblingFromExeSymlinkCrossDir uses os.Symlink with a t.Skipf fallback for
    platforms without symlink support.
  - ASSERTIONS in the symlink test: got == skillsA (the REAL binary's extensions dir) AND
    filepath.Dir(got) == tempA (the REAL binary's dir), explicitly NOT tempB (the symlink's dir).

Task 8: MODIFY internal/extdir/extdir_test.go — add findSibling smoke test
  - PORT TestFindSiblingNoExtensionsNextToTestBinary (renamed from NoSkillsNextToTestBinary).
    Asserts findSibling() returns found=false without panicking, because the test binary
    runs from a temp go-build dir with no sibling extensions/. This is the ONLY deterministic
    assertion possible for findSibling (os.Executable not controllable); the symlink logic
    is covered by the resolveSiblingFromExe tests.
  - DOC COMMENT noting why this is a smoke test only.

Task 9: MODIFY internal/extdir/extdir_test.go — add findConfig tests
  - PORT verbatim from skilldozer with renames ("SKILLDOZER_CONFIG"→"weave_CONFIG"). Add a
    writeCfg helper (writes content to a temp config.yaml, sets weave_CONFIG, returns
    cfgPath + cfgDir). The 6 tests:
      TestFindConfigHit                             (existing store dir via absolute store: → SourceConfig)
      TestFindConfigMissingFile                     (config file does not exist → fall through)
      TestFindConfigMissingStoreKey                 (config has no store: key → fall through)
      TestFindConfigStoreDirAbsent                  (store: names a nonexistent dir → fall through)
      TestFindConfigMalformedSyntax                 (ADAPT from MalformedYAML: weave's hand-rolled
        parser has no hard-error case; a garbage-content file with no store: line falls through
        via File{Store:""} → f.Store == "" branch. Assert found=false.)
      TestFindConfigRelativeStoreResolvedAgainstConfigDir (relative store: joined against
        filepath.Dir(configPath), NOT cwd)
  - Each findConfig test calls unsetEnvVar(t) FIRST (to neutralize both envVar and any real
    weave_CONFIG), then sets weave_CONFIG via writeCfg or t.Setenv as needed.
  - writeCfg helper:
      func writeCfg(t *testing.T, content string) (cfgPath, cfgDir string) {
          t.Helper()
          cfgDir = t.TempDir()
          cfgPath = filepath.Join(cfgDir, "config.yaml")
          if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
              t.Fatalf("write %s: %v", cfgPath, err)
          }
          t.Setenv("weave_CONFIG", cfgPath)
          return cfgPath, cfgDir
      }

Task 10: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...           # expect exit 0
  - RUN: go build ./internal/extdir/                                # standalone, exit 0
  - RUN: go vet ./internal/extdir/                                  # expect exit 0, clean
  - RUN: go test ./internal/extdir/ -v                              # expect ALL PASS (S1 + S2 suites)
  - RUN: go test -race ./internal/extdir/                           # expect no data races
  - RUN: grep -rn "yaml.v3\|gopkg.in" --include=*.go .              # expect nothing
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK          # expect OK (no require block)
  - RUN: test ! -f go.sum && echo OK || echo FAIL                   # expect OK (no go.sum)
  - RUN: ! grep -q '"skills"' internal/extdir/extdir.go && echo OK  # expect OK (no leftover "skills" literal)
  - EXPECT: build clean, vet clean, all tests pass, no third-party import, no require block,
    no go.sum, no leftover "skills" literal.
```

### Implementation Patterns & Key Details

```go
// findConfig — PORT skilldozer's body VERBATIM (only the config import path differs).
// PRD §8.3 rule 2 / §8.1. Never errors — any failure falls through to rule 3.
func findConfig() (dir string, src Source, found bool) {
    p, err := config.Path()
    if err != nil {
        return "", 0, false // no bootstrap path (e.g. relative $XDG_CONFIG_HOME) -> fall through
    }
    f, err := config.Load(p)
    if err != nil {
        return "", 0, false // missing/unreadable -> "not yet configured" -> fall through
    }
    if f.Store == "" {
        return "", 0, false // no `store` key (or empty value) -> fall through
    }
    var store string
    if filepath.IsAbs(f.Store) {
        store = filepath.Clean(f.Store)
    } else {
        store = filepath.Join(filepath.Dir(p), f.Store) // relative to config file's dir (PRD §8.1)
    }
    info, err := os.Stat(store)
    if err != nil || !info.IsDir() {
        return "", 0, false // store path is not an existing dir -> fall through
    }
    return store, SourceConfig, true
}

// GOTCHA (weave vs skilldozer): weave's config.Load is a hand-rolled line scanner, NOT
// yaml.v3. So config.Load NEVER returns a "malformed YAML" hard error — a present-but-
// garbage file with no "store:" line returns File{Store: ""}, nil, and the f.Store == ""
// branch above handles it. Do NOT add special malformed-handling; it is impossible by
// construction. (skilldozer's findConfig had to convert a yaml.v3 hard error into a
// fall-through; weave does not.)

// findSibling — PORT skilldozer's body VERBATIM. Thin entry; testable core is below.
func findSibling() (dir string, src Source, found bool) {
    exe, err := os.Executable()
    if err != nil {
        return "", 0, false // no binary path at all -> skip rule
    }
    d, ok := resolveSiblingFromExe(exe)
    if !ok {
        return "", 0, false
    }
    return d, SourceSibling, true
}

// resolveSiblingFromExe — PORT skilldozer's body VERBATIM, with "skills" → "extensions".
// CRITICAL: EvalSymlinks MUST stay. On Linux os.Executable() resolves the symlink via
// /proc/self/exe (redundant-but-harmless); on macOS os.Executable() may return the symlink
// path and rule 3 SILENTLY misses without EvalSymlinks. See architecture/
// verified_symlink_resolution.md.
func resolveSiblingFromExe(exe string) (dir string, found bool) {
    real, err := filepath.EvalSymlinks(exe)
    if err != nil {
        real = exe // EvalSymlinks could not resolve -> use exe verbatim (REQUIRED fallback)
    }
    repoDir := filepath.Dir(real)
    candidate := filepath.Join(repoDir, "extensions") // weave: "extensions" not "skills"
    info, err := os.Stat(candidate)
    if err != nil || !info.IsDir() {
        return "", false // no existing extensions/ sibling -> rule misses
    }
    return candidate, true
}

// unsetEnvVar (test helper) — UPGRADE in S2 to also neutralize weave_CONFIG.
// Without the weave_CONFIG neutralization, a dev machine with a real
// ~/.config/weave/config.yaml (or weave_CONFIG set) makes findConfig return a real dir
// in tests that expect a miss. Mirrors skilldozer's unsetEnvVar (SKILLDOZER_CONFIG).
func unsetEnvVar(tb testing.TB) {
    tb.Helper()
    // ... existing envVar (weave_EXTENSIONS_DIR) neutralization from S1, unchanged ...
    // NEW in S2: neutralize the config rule (PRD §8.3 rule 2):
    cfgGhost := filepath.Join(tb.TempDir(), "no-config.yaml")
    prevCfg, hadCfg := os.LookupEnv("weave_CONFIG") // LOWERCASE
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
}
```

### Integration Points

```yaml
DATABASE:
  - none. Pure filesystem + env + intra-module config call.

CONFIG:
  - findConfig consumes config.Path() + config.Load(path) from internal/config (P1.M1.T2.S1,
    Complete). No changes to the config package. The contract: config.Load returns
    (File{Store string}, error) with the raw os.ReadFile error verbatim; a present-but-no-
    "store:"-line file returns File{}, nil. findConfig treats ANY error or Store=="" as a miss.

ROUTES / API:
  - none. weave is a CLI; this package has no handlers. Consumed by:
      * Find() (P1.M1.T3.S3) — will call findEnv() (rule 1), findConfig() (rule 2, this
        subtask), findSibling() (rule 3, this subtask), findWalkUp() (rule 4, S3), and
        return the first hit or ErrNotFound.

MODULE:
  - The new import github.com/dabstractor/weave/internal/config is INTRA-MODULE — Go resolves
    it from the local source tree. go.mod gains NO require block; no go.sum is generated.
    If `go mod tidy` modifies go.mod, a third-party dependency leaked in (bug).
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

# No third-party deps leaked in (the only non-stdlib import should be internal/config).
grep -rn "yaml.v3\|gopkg.in" --include=*.go . || echo "no third-party import (correct)"
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
# EXPECTED: "no third-party import (correct)" and "no require block (correct)".

# No leftover "skills" literal (the single most likely copy-paste bug from skilldozer).
! grep -q '"skills"' internal/extdir/extdir.go && echo "no 'skills' literal (correct)" \
  || echo "FAIL: leftover 'skills' literal in extdir.go"
# EXPECTED: "no 'skills' literal (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the extdir package tests verbosely (S1 suite + new S2 suites).
go test ./internal/extdir/ -v
# EXPECTED: ALL tests pass. Pay special attention to:
#   - TestResolveSiblingFromExeSymlinkCrossDir (THE critical symlink test — the whole
#     point of rule 3; asserts the symlink in a different dir resolves back to the REAL
#     binary's repo dir's extensions/)
#   - TestResolveSiblingFromExeEvalSymlinksFallback (real=exe fallback contract)
#   - TestFindConfigRelativeStoreResolvedAgainstConfigDir (relative store joined to
#     filepath.Dir(configPath), NOT cwd)
#   - TestFindConfigMissingFile / MissingStoreKey / StoreDirAbsent / MalformedSyntax
#     (all must fall through — never a hard error)
#   - TestFindConfigHit (SourceConfig label + cleaned absolute dir)
# If any fail, read the failure, fix extdir.go (NOT the test — tests encode the contract).

# Race detector sanity.
go test -race ./internal/extdir/
# EXPECTED: passes, no data races.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (config from P1.M1.T2.S1 + extdir from S1+S2 must all compile).
go build ./...
# EXPECTED: exit 0.

# Standalone package build (proves the internal/config import resolves intra-module).
go build ./internal/extdir/
# EXPECTED: exit 0.

# Confirm the config import is present (S2 added it) and resolves to the local package.
grep -q "github.com/dabstractor/weave/internal/config" internal/extdir/extdir.go \
  && echo "config import present (correct)" || echo "FAIL: config import missing"

# Confirm no go.sum was generated (internal/config is intra-module — no dep fetch).
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum exists — a dep leaked in"

# go mod tidy is a no-op on a zero-external-dep module.
go mod tidy
git diff --stat go.mod   # expect: empty / unchanged
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove the symlink-install scenario end-to-end via resolveSiblingFromExe,
# mirroring PRD §12.1's "~/.local/bin/weave -> ~/projects/weave/weave" rationale.
# A throwaway test outside the repo is NOT needed — TestResolveSiblingFromExeSymlinkCrossDir
# already covers this with t.TempDir() + os.Symlink. But for extra confidence, build a tiny
# Go program that calls resolveSiblingFromExe directly:
tmp=$(mktemp -d)
mkdir -p /tmp/weave-sib-smoke
cat > /tmp/weave-sib-smoke/main.go <<'GO'
package main
import (
    "fmt"
    "github.com/dabstractor/weave/internal/extdir"
)
func main() {
    // resolveSiblingFromExe is package-internal, so this smoke is informational only.
    // The unit test TestResolveSiblingFromExeSymlinkCrossDir is the authoritative gate.
    _ = extdir.SourceSibling
    fmt.Println("SourceSibling label:", extdir.SourceSibling)
}
GO
( cd /tmp/weave-sib-smoke && go run main.go )   # expect: SourceSibling label: sibling of binary
rm -rf "$tmp" /tmp/weave-sib-smoke
# EXPECTED: "SourceSibling label: sibling of binary" (confirms the label + that the package
# compiles standalone). The symlink-resolution behavior itself is pinned by the unit test.

# Cross-platform note: on macOS, TestResolveSiblingFromExeSymlinkCrossDir is the ONLY test
# that exercises the EvalSymlinks-required code path meaningfully (Linux's os.Executable
# already resolves via /proc/self/exe, so the test passes with OR without EvalSymlinks on
# Linux — but removing EvalSymlinks would break macOS). Do NOT remove EvalSymlinks based
# on Linux-only test runs.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l internal/extdir/` empty; `go vet ./internal/extdir/` clean;
      no third-party import; no `require` block in go.mod; no leftover `"skills"` literal.
- [ ] Level 2 passed: `go test ./internal/extdir/ -v` — ALL tests pass (S1 suite still green
      + new S2 suites for findConfig / findSibling / resolveSiblingFromExe), including the
      symlink-cross-dir test and the EvalSymlinks-fallback test.
- [ ] Level 3 passed: `go build ./...` exit 0; `go build ./internal/extdir/` exit 0;
      config import present and resolves intra-module; no go.sum; `go mod tidy` is a no-op.
- [ ] `go test -race ./internal/extdir/` passes (no data races).

### Feature Validation

- [ ] `findConfig` exists, ports skilldozer's body verbatim, resolves relative `store` against
      `filepath.Dir(configPath)`, and returns `("", 0, false)` on ANY error / empty store /
      non-dir store — never propagates an error.
- [ ] `findSibling` + `resolveSiblingFromExe` exist, port skilldozer's bodies verbatim with
      the single `"skills" → "extensions"` change.
- [ ] `resolveSiblingFromExe` calls `filepath.EvalSymlinks(exe)` with the `real = exe` fallback
      on error (REQUIRED for macOS; redundant-but-harmless on Linux).
- [ ] The symlink-install scenario works: a symlink to a binary in a DIFFERENT dir resolves
      back to the REAL binary's repo dir's `extensions/` (pinned by
      `TestResolveSiblingFromExeSymlinkCrossDir`).

### Code Quality Validation

- [ ] Package doc comment updated to reflect that rules 2 and 3 are now implemented (only
      rule 4 + Find remain for S3); the findEnv no-EvalSymlinks contract note and the
      HasExtensionEntry dual-role note are preserved.
- [ ] Mirrors skilldozer's findConfig / findSibling / resolveSiblingFromExe structure verbatim
      (with the two documented renames).
- [ ] unsetEnvVar upgraded to also neutralize `weave_CONFIG` (lowercase), preventing machine
      config from leaking into all-miss / env-only tests.
- [ ] File placement: `internal/extdir/extdir.go` (modified) + `internal/extdir/extdir_test.go`
      (modified); no new files.
- [ ] Anti-patterns avoided: no `fmt.Errorf` wrapping; no third-party import; no
      `findWalkUp`/`Find`/`ErrNotFound` (S3's boundary); no modification to config or main.go;
      no `init()`; EvalSymlinks not dropped.

### Documentation & Deployment

- [ ] Package is self-documenting via the updated doc comment (no separate README — internal
      package). Per the item contract: "DOCS: none — internal package; symlink-install behavior
      documented in README (Mode B final task)."
- [ ] No new user-facing env vars introduced by this subtask.

---

## Anti-Patterns to Avoid

- ❌ Don't drop `filepath.EvalSymlinks` from `resolveSiblingFromExe`. It is redundant-but-
  harmless on Linux (os.Executable resolves via /proc/self/exe) but REQUIRED on macOS (where
  os.Executable may return the symlink path). The item contract, architecture_mapping.md §2,
  and verified_symlink_resolution.md all forbid dropping it. Linux-only test runs pass with
  OR without it — do NOT use that as justification to remove it.
- ❌ Don't leave the sibling dir literal as `"skills"`. It MUST be `"extensions"` — this is
  the ONLY behavioral change from skilldozer's resolveSiblingFromExe and the single most
  likely copy-paste bug. Pinned by `grep -q '"skills"'` in Level 1.
- ❌ Don't propagate errors from `findConfig`. PRD §8.1: "A missing or unreadable config is
  treated as 'not yet configured' and falls through … never a hard error." Any error from
  config.Path / config.Load / os.Stat → return `("", 0, false)`.
- ❌ Don't resolve a relative `store` against cwd. It MUST be joined against
  `filepath.Dir(configPath)` (the config file's dir). PRD §8.1. Pinned by
  `TestFindConfigRelativeStoreResolvedAgainstConfigDir`.
- ❌ Don't forget to upgrade `unsetEnvVar`. S1's version only neutralizes `envVar`
  (weave_EXTENSIONS_DIR); without ALSO neutralizing `weave_CONFIG` (lowercase), a dev machine
  with a real config makes the new findConfig tests (and S1's env tests, and S3's Find
  all-miss test) machine-dependent and flaky.
- ❌ Don't add special "malformed config" handling. weave's config.Load is a hand-rolled line
  scanner — it NEVER returns a hard error for "malformed" input (it just finds no `store:`
  line and returns `File{Store: ""}, nil`). skilldozer needed malformed-handling because of
  yaml.v3; weave does not. The `f.Store == ""` branch handles it.
- ❌ Don't test `findSibling`'s symlink logic through `findSibling()` directly.
  `os.Executable()` returns the test binary's own path in a temp go-build dir (not
  controllable), so the symlink behavior can ONLY be tested through `resolveSiblingFromExe`
  with an injected exe path. `findSibling()` gets a smoke test asserting `found=false`.
-- ❌ Don't use a real ELF executable in tests. A 1-byte regular file (the `makeFakeBinary`
  helper writes `[]byte("x")`, mode 0o644) suffices — `filepath.EvalSymlinks` and `os.Stat`
  on the sibling dir do not require a real executable. (Verified in skilldozer research §5.)
- ❌ Don't add `findWalkUp`, `findWalkUpAncestor`, `Find`, or `ErrNotFound` — they belong to
  S3. This subtask is rules 2 and 3 ONLY.
- ❌ Don't modify the config package or main.go. S2's blast radius is extdir.go +
  extdir_test.go, nothing else.
- ❌ Don't add a third-party import or a `require` line to go.mod. The only new import is
  `github.com/dabstractor/weave/internal/config`, which is intra-module and resolves from
  the local source tree.
- ❌ Don't call `t.Parallel()` in any env-mutating test (all findConfig tests call
  `unsetEnvVar` / `t.Setenv`). Mirror skilldozer's "Do NOT call t.Parallel()" convention.
- ❌ Don't lower the test bar by editing tests to make them pass. The tests encode the
  contract; if a test fails, fix `extdir.go`.
