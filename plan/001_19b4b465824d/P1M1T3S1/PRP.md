# PRP — P1.M1.T3.S1: Source type + env-var resolution (rule 1) + HasExtensionEntry predicate

## Goal

**Feature Goal**: Create the `internal/extdir` package skeleton — the package that will
locate the on-disk extensions directory per PRD §8.3 priority order. **This subtask
delivers the package shell (Source type + String labels + envVar constant), rule 1
(`findEnv`: `weave_EXTENSIONS_DIR` resolution), and the `HasExtensionEntry` predicate**
that doubles as the `weave init` cwd-auto-detect check (PRD §8.2) and the walk-up
qualifier (PRD §8.3 rule 4). The remaining rules (config, sibling, walk-up) and the
`Find()` combiner are deliberately deferred to P1.M1.T3.S2 and P1.M1.T3.S3.

This is the weave port of skilldozer's `internal/skillsdir/skillsdir.go` (rule-1 +
predicate portions only), per architecture_mapping.md §2. PRD §13 calls extdir the
"single most failure-prone area — implement and test it first."

**Deliverable**: One new Go file, `internal/extdir/extdir.go`, exporting:
- `type Source int` with constants `SourceEnv`, `SourceConfig`, `SourceSibling`, `SourceWalkUp`
- `func (s Source) String() string` returning labels `"weave_EXTENSIONS_DIR"`, `"config file"`, `"sibling of binary"`, `"ancestor of cwd"`
- `func HasExtensionEntry(dir string) bool` — walks `dir` for ANY extension entry at any depth (PRD §7.1), short-circuiting on first find

Plus package-internal (unexported): `const envVar = "weave_EXTENSIONS_DIR"`, `func findEnv() (dir string, src Source, found bool)` (rule 1).

Plus one test file `internal/extdir/extdir_test.go` porting skilldozer's rule-1 + Source
tests verbatim (with renames) and adding comprehensive `HasExtensionEntry` tests covering
all three entry kinds (file / dir / package) from PRD §7.1.

**Success Definition**:
- `go build ./...` exits 0.
- `go vet ./internal/extdir/` exits 0 (clean).
- `go test ./internal/extdir/ -v` passes ALL tests, including: every findEnv branch
  (unset/empty/existing/nonexistent/file/relative/symlink-not-resolved), all four Source
  labels + "unknown", and every HasExtensionEntry case (file/dir/package entry found,
  empty dir, non-entry files only, package.json with no existing entry, nested at depth).
- The `findEnv` symlink test passes: `weave_EXTENSIONS_DIR` pointing at a symlink-to-dir
  returns the SYMLINK path absolutized, NOT the resolved target (no `EvalSymlinks`).
- No third-party import; `go.mod` still has no `require` block.

## Why

- PRD §8.3 makes rule 1 (`weave_EXTENSIONS_DIR`) the highest-priority resolution rule —
  CI/tests/temporary redirects win without editing config. This is the one rule a user
  can change without re-running `weave init`, so it must be airtight.
- `HasExtensionEntry` is **load-bearing for two later features**: (1) `weave init`
  cwd-auto-detect (PRD §8.2: "if the current working directory already looks like a store
  … contains at least one extension entry at any depth"), and (2) the walk-up qualifier
  (PRD §8.3 rule 4: "for `go run` / dev" — an ancestor only wins if its `extensions/`
  subdir actually contains entries). Both consume this exact predicate.
- PRD §13 acceptance: extdir is "the single most failure-prone area — implement and test
  it first." Splitting it into S1 (rule 1 + predicate), S2 (rules 2-3), S3 (rule 4 +
  combiner) keeps each PRP's blast radius small while front-loading the riskiest logic.
- The predicate name `HasExtensionEntry` (not `HasSkillMD`) and the env var
  `weave_EXTENSIONS_DIR` (lowercase) are pinned by the item contract and
  architecture_mapping.md §2 — these are the weave-specific renames that distinguish the
  port from skilldozer.

## What

A single internal Go package with one Source type, four constants, one String method, one
exported predicate, and one unexported rule-1 helper (plus small unexported helpers the
predicate needs). No CLI surface, no `Find()` yet (that is S3), no config import yet
(that is S2).

### Success Criteria

- [ ] `internal/extdir/extdir.go` exists and defines `Source`, the four constants,
      `String()`, `HasExtensionEntry`, `findEnv`, and `envVar` exactly as specified.
- [ ] `findEnv` reads `os.LookupEnv(envVar)`; unset or empty ⇒ `("", 0, false)`;
      `os.Stat` fails or not-a-dir ⇒ `("", 0, false)`; existing dir ⇒
      `(filepath.Abs(val), SourceEnv, true)`. **Never calls `EvalSymlinks`.**
- [ ] `HasExtensionEntry` returns `true` as soon as ANY extension entry (PRD §7.1) is found
      at ANY depth under `dir`; short-circuits via a sentinel error returned from the
      `WalkDir` callback.
- [ ] The three entry kinds are recognized: (a) `*.ts`/`*.js` file whose base name is NOT
      `index.ts`/`index.js`; (b) a directory directly containing `index.ts` or `index.js`;
      (c) a directory whose `package.json` has a `pi.extensions` array naming ≥1 existing
      entry.
- [ ] `String()` returns the four PRD §8 labels verbatim and `"unknown"` for any other value.
- [ ] `go build ./...`, `go vet ./internal/extdir/`, and `go test ./internal/extdir/` pass.
- [ ] No third-party import; no `require` block in `go.mod`; no `go.sum`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP gives the exact contract for every symbol
(signature, return values, branch conditions), the exact skilldozer file to port (read in
full during research, with line-level mapping), the exact test patterns to mirror, the
exact env-var name (`weave_EXTENSIONS_DIR`, lowercase), the exact String labels (PRD §8.3),
the exact PRD §7.1 entry definition for the predicate, and the verified build/vet/test
gates.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for §8.3 rule 1 (env var), §8.2 (cwd-auto-detect predicate use),
       §7.1 (extension-entry definition — the predicate's recognition rule), §13 (extdir is
       most failure-prone; test first).
  critical: §8.3 labels are EXACT: "weave_EXTENSIONS_DIR", "config file", "sibling of binary",
            "ancestor of cwd". §7.1 index.ts/index.js as a FILE is NOT a single-file entry
            (case a excludes them); a DIR containing index.ts/index.js IS an entry (case b).

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir.go
  why: The PRIMARY pattern to port — the Source type, the four iota constants, the String()
       switch, the envVar const pattern, and the findEnv() body ALL port nearly verbatim.
       Only the env-var name (SKILLDOZER_SKILLS_DIR → weave_EXTENSIONS_DIR) and the first
       String() label change for THIS subtask.
  pattern: |
    type Source int
    const ( SourceEnv Source = iota; SourceConfig; SourceSibling; SourceWalkUp )
    func (s Source) String() string { switch s { ... } }   // weave: label[0] = "weave_EXTENSIONS_DIR"
    const envVar = "weave_EXTENSIONS_DIR"                  // weave: lowercase, was SKILLDOZER_SKILLS_DIR
    func findEnv() (dir string, src Source, found bool) {  // PORT BODY VERBATIM
        val, ok := os.LookupEnv(envVar)
        if !ok || val == "" { return "", 0, false }
        info, err := os.Stat(val)
        if err != nil || !info.IsDir() { return "", 0, false }
        abs, err := filepath.Abs(val)
        if err != nil { return "", 0, false }
        return abs, SourceEnv, true
    }
  gotcha: skilldozer's HasSkillMD ports in SHAPE (WalkDir + sentinel + early-exit) but NOT
          in recognition logic. weave's predicate recognizes 3 entry kinds (PRD §7.1), not
          one filename. See HasExtensionEntry contract below.

- file: /home/dustin/projects/skilldozer/internal/skillsdir/skillsdir_test.go
  why: The PRIMARY test pattern to port for findEnv + Source. Mirror these tests 1:1 with
       renames ("skilldozer"→"weave", "SKILLDOZER_SKILLS_DIR"→"weave_EXTENSIONS_DIR"):
         TestSourceString, TestFindEnvUnset, TestFindEnvEmpty, TestFindEnvExistingDir,
         TestFindEnvNonexistent, TestFindEnvRegularFile, TestFindEnvRelativePathAbsolutized,
         TestFindEnvDoesNotResolveSymlinks.
  pattern: unsetEnvVar helper points the env var at a non-existent ghost path (so a real
           machine config can't leak into all-miss assertions) — but for THIS subtask, only
           the envVar neutralization is needed (config neutralization is S2/S3's concern;
           S1 has no findConfig yet, so omit that part of unsetEnvVar).
  gotcha: Do NOT port the findConfig/findSibling/findWalkUp/Find/HasSkillMD tests — they
          belong to S2/S3. Port ONLY the rule-1 + Source tests above, then ADD the
          HasExtensionEntry tests (new for weave).

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §2 maps skilldozer skillsdir → weave extdir line by line and pins the renames.
  critical: §2 says "Port resolveSiblingFromExe verbatim — only the sibling dir name
            changes: extensions instead of skills" (that is S2, NOT this subtask). §2 also
            pins the predicate: "weave uses HasExtensionEntry(dir) — walks for ANY extension
            entry at any depth."

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/pi_extension_facts.md
  why: §4 + §6 document how pi itself resolves a directory (package.json pi.extensions
       checked FIRST, then index.ts/index.js) and how readPiManifest parses it. This is the
       factual grounding for HasExtensionEntry case (c).
  critical: §6 — pi reads package.json, takes pkg.pi.extensions (array), resolves each
            relative to the dir, counts only entries that EXIST. weave's predicate mirrors
            this: "package.json with pi.extensions array naming ≥1 existing entry."

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/external_deps.md
  why: Confirms zero-deps invariant; lists the exact stdlib packages extdir needs
       (os, path/filepath, io/fs, errors; + encoding/json + strings for the predicate).
  critical: encoding/json is stdlib — use it for package.json parsing (NOT a third-party
            yaml lib). No go.sum should appear.

- file: /home/dustin/projects/weave/go.mod  (from P1.M1.T1.S1)
  why: Establishes module `github.com/dabstractor/weave`, `go 1.25`, no require block.
  critical: extdir imports as github.com/dabstractor/weave/internal/extdir. This subtask
            does NOT import internal/config (findConfig is S2), so extdir compiles even
            if config is not yet present. (It will be — P1.M1.T2.S1 runs in parallel —
            but S1 has no compile dependency on it.)

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M1T2S1/PRP.md
  why: The CONTRACT for the parallel config package. S1 does NOT consume config yet, but
       S2 will (config.Path + config.Load). Reading this PRP confirms the config API
       surface S2 will call, so S1's package skeleton is shaped to receive S2's additions
       without refactoring.
  critical: config.Load returns (File{Store string}, error) with the raw os.ReadFile error
            verbatim. S2's findConfig will do errors.Is(err, fs.ErrNotExist). S1 does not
            need this — noted for continuity only.
```

### Current Codebase tree

```bash
# After P1.M1.T1.S1 (complete) + P1.M1.T2.S1 (in parallel, "Ready"):
$ cd /home/dustin/projects/weave && ls -A1
.git
.gitignore
LICENSE
PRD.md
go.mod                  # module github.com/dabstractor/weave / go 1.25 / no require
internal/
└── config/             # from P1.M1.T2.S1 (parallel) — File/Load/Save/Path/DefaultStore
    ├── config.go
    └── config_test.go
plan/
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                      # from P1.M1.T2.S1 (unchanged by this subtask)
│   └── extdir/
│       ├── extdir.go                # NEW — Source/String/envVar/findEnv/HasExtensionEntry + helpers
│       └── extdir_test.go           # NEW — rule-1 tests (ported) + HasExtensionEntry tests (new)
├── go.mod                           # unchanged (no new require — zero deps)
└── ...                              # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: env var is LOWERCASE `weave_EXTENSIONS_DIR` (PRD §8.3 + item contract),
// NOT `WEAVE_EXTENSIONS_DIR`. os.Getenv/os.LookupEnv are case-sensitive on Linux.
// skilldozer uses uppercase SKILLDOZER_SKILLS_DIR; weave does NOT. Pinned by
// TestSourceString (label) and every findEnv test (env var name).

// CRITICAL: findEnv must NOT call filepath.EvalSymlinks on the env path. The user points
// exactly where they want; a symlink is preserved verbatim, only made absolute/clean via
// filepath.Abs. os.Stat follows the symlink (so IsDir=true for a symlink-to-dir), but
// the RETURNED path is the symlink, not the target. Pinned by TestFindEnvDoesNotResolveSymlinks.
// (Contrast: rule 3 sibling resolution DOES use EvalSymlinks — but that is S2, not here.)

// CRITICAL: HasExtensionEntry recognizes index.ts/index.js ONLY as a DIR marker (case b),
// NEVER as a single-file entry (case a explicitly excludes them). So a directory
// containing ONLY index.ts is a resolvable entry (case b → true), but a lone index.ts
// file floating in a plain dir does not by itself make that dir an entry via case (a).
// Pinned by TestHasExtensionEntryRootWithIndexTS (true, via case b) and
// TestHasExtensionEntryIgnoresIndexAsStandaloneFileInPlainDir (the index.ts is a dir
// marker for its containing dir, so that dir qualifies — see test for the precise shape).

// CRITICAL: HasExtensionEntry case (c) requires ≥1 EXISTING entry in the pi.extensions
// array. A package.json with pi.extensions:["./missing.ts"] and no such file → FALSE
// (mirrors pi's own resolver, pi_extension_facts.md §4: "only entries that actually
// EXIST on disk are included"). Pinned by TestHasExtensionEntryPackageJSONNoExistingEntry.

// CRITICAL: short-circuit via a sentinel error returned from the WalkDir callback.
// Returning any non-nil error stops the walk. Do NOT walk the whole tree — return
// errExtensionFound as soon as the first entry is recognized. (Mirrors skilldozer's
// errSkillMDFound.) filepath.Glob with "**" is NOT supported by Go's path/filepath
// (it behaves like single-level "*"); WalkDir is the correct stdlib tool.

// GOTCHA: WalkDir visits the root dir FIRST. If the root itself is a resolvable dir
// (e.g. HasExtensionEntry(somePkgDir) where somePkgDir/index.ts exists), case (b) fires
// on the very first callback invocation → true immediately. This is correct per PRD §7.1
// (a dir containing index.ts is an entry).

// GOTCHA: encoding/json into a typed []string field (Pi.Extensions []string) silently
// drops non-string elements — this is the lenient "wrong-typed fields coerced or ignored"
// behavior PRD §7.3 wants. If pi.extensions is absent or not an array, the slice is nil
// and the loop is a no-op → false. Do NOT write custom coercion; the typed struct field
// does it for free.

// GOTCHA: WalkDir callback receives an `err` for unreadable entries; return nil to skip
// and keep walking (do NOT propagate — a single unreadable subdir must not abort the
// whole predicate). Mirrors skilldozer's HasSkillMD error handling.

// GOTCHA (build): this subtask does NOT import internal/config. findConfig (rule 2) is
// S2's work. If you add a config import now, the package won't compile if config isn't
// present yet, AND it violates the subtask boundary. Keep imports to: errors,
// encoding/json, io/fs, os, path/filepath, strings.
```

## Implementation Blueprint

### Data models and structure

```go
// Source identifies which §8.3 rule located the extensions directory. Reported by
// `weave --path` so users can tell how the dir was found. Ports skilldozer's Source
// type verbatim (iota order preserved so S2/S3 constants slot in unchanged).
type Source int

const (
    SourceEnv Source = iota     // weave_EXTENSIONS_DIR was set and pointed at an existing dir
    SourceConfig                // config file `store` key (S2)
    SourceSibling               // sibling of running binary (S2)
    SourceWalkUp                // walk up from cwd (S3)
)
```

No other exported types. One unexported sentinel error (`errExtensionFound`) for the
predicate short-circuit. No interfaces.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/extdir/extdir.go
  - WRITE the package with a doc comment (see "Implementation Patterns" below).
  - IMPORTS (stdlib only — NO internal/config in this subtask):
      import (
          "encoding/json"
          "errors"
          "io/fs"
          "os"
          "path/filepath"
          "strings"
      )
  - DEFINE: type Source int + the four iota constants (SourceEnv first).
  - IMPLEMENT String() — switch on s; cases SourceEnv/SourceConfig/SourceSibling/SourceWalkUp
    return the four PRD §8.3 labels ("weave_EXTENSIONS_DIR", "config file",
    "sibling of binary", "ancestor of cwd"); default returns "unknown".
  - DEFINE: const envVar = "weave_EXTENSIONS_DIR"   // package-internal, lowercase
  - IMPLEMENT findEnv() (dir string, src Source, found bool) — PORT skilldozer's findEnv
    body VERBATIM (see Documentation & References pattern block). Branches:
      val, ok := os.LookupEnv(envVar); if !ok || val == "" → return "", 0, false
      info, err := os.Stat(val); if err != nil || !info.IsDir() → return "", 0, false
      abs, err := filepath.Abs(val); if err != nil → return "", 0, false
      return abs, SourceEnv, true
    NO EvalSymlinks call anywhere.
  - DEFINE: var errExtensionFound = errors.New("extension entry found")   // sentinel
  - IMPLEMENT HasExtensionEntry(dir string) bool — see "Implementation Patterns" for the
    full body. WalkDir(dir) with a callback that:
      * on callback err != nil → return nil (skip unreadable, keep walking)
      * if d.IsDir():
          - if isResolvableDir(path) → found=true; return errExtensionFound (short-circuit)
          - else return nil (plain dir, WalkDir descends naturally)
      * else (file): if isExtensionFile(d.Name()) → found=true; return errExtensionFound
      * else return nil
    Return found after WalkDir returns (ignore the propagated errExtensionFound via `_ =`).
  - IMPLEMENT helpers (all package-internal / lowercase):
      func isExtensionFile(name string) bool
          // false for index.ts/index.js; true for *.ts/*.js; false otherwise
          if name == "index.ts" || name == "index.js" { return false }
          return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")
      func isResolvableDir(dir string) bool
          // case (b): index.ts or index.js directly in dir
          if fileExists(filepath.Join(dir, "index.ts")) ||
             fileExists(filepath.Join(dir, "index.js")) { return true }
          // case (c): package.json with pi.extensions naming >=1 existing entry
          return hasPiExtensions(dir)
      func hasPiExtensions(dir string) bool
          data, err := os.ReadFile(filepath.Join(dir, "package.json"))
          if err != nil { return false }
          var pkg struct {
              Pi struct {
                  Extensions []string `json:"extensions"`
              } `json:"pi"`
          }
          if json.Unmarshal(data, &pkg) != nil { return false }
          for _, e := range pkg.Pi.Extensions {
              if fileExists(filepath.Join(dir, e)) { return true }
          }
          return false
      func fileExists(path string) bool
          info, err := os.Stat(path); return err == nil && !info.IsDir()
          // (fileExists = a non-dir entry exists; used for index.* and pi.extensions entries.
          //  os.Stat follows symlinks, matching pi's fs.existsSync semantics — fine for both.)
  - NAMING: package `extdir`; file `extdir.go`; dir `internal/extdir/`.
  - PLACEMENT: internal/extdir/extdir.go (PRD §5 layout; mirrors skilldozer internal/skillsdir/).
  - DEPENDENCIES: stdlib only. NO import of internal/config in this subtask.

Task 2: CREATE internal/extdir/extdir_test.go  (package extdir)
  - PACKAGE: `package extdir` (same package — white-box, so it can reference the
    package-internal envVar const and the unexported findEnv, exactly as skilldozer's
    test does).
  - IMPORTS: os, path/filepath, testing. (encoding/json not needed in tests directly.)
  - HELPER: unsetEnvVar(tb) — mirror skilldozer's helper but ONLY neutralize envVar
    (do NOT touch SKILLDOZER_CONFIG/weave_CONFIG — findConfig is S2, not present here).
    unset envVar for the test, restore on cleanup. t.Helper(). Mark "Do NOT call
    t.Parallel() — mutates env".
  - PORT from skilldozer (verbatim with renames "skilldozer"→"weave",
    "SKILLDOZER_SKILLS_DIR"→"weave_EXTENSIONS_DIR"):
      * TestSourceString — 4 labels + Source(-1)→"unknown" + Source(99)→"unknown".
      * TestFindEnvUnset — unsetEnvVar(t); findEnv() → found=false.
      * TestFindEnvEmpty — t.Setenv(envVar, ""); findEnv() → found=false.
      * TestFindEnvExistingDir — t.TempDir(); found=true, src=SourceEnv, dir=Clean(temp).
      * TestFindEnvNonexistent — env=TempDir/does-not-exist; found=false.
      * TestFindEnvRegularFile — write a file, env=that; found=false (IsDir guard).
      * TestFindEnvRelativePathAbsolutized — env="."; found=true, dir=Abs(".").
      * TestFindEnvDoesNotResolveSymlinks — symlink-to-dir; found=true, dir=Abs(symlink),
        NOT the resolved target (assert got != realDir). t.Skipf on platforms w/o symlinks.
  - ADD new HasExtensionEntry tests (weave-specific; skilldozer has HasSkillMD analogs
    but the recognition logic differs). Use t.TempDir() + os.MkdirAll/os.WriteFile:
      * TestHasExtensionEntryFoundNestedFile — dir/a/b/foo.ts → true (case a, nested).
      * TestHasExtensionEntryFoundShallowFile — dir/foo.ts → true (case a).
      * TestHasExtensionEntryFoundJSDir — dir/pkg/index.ts → true (case b).
      * TestHasExtensionEntryFoundJSIndexJs — dir/pkg/index.js → true (case b, .js variant).
      * TestHasExtensionEntryFoundPackageJSON — dir/pkg/package.json with
        {"pi":{"extensions":["./src/index.ts"]}} + dir/pkg/src/index.ts → true (case c).
      * TestHasExtensionEntryPackageJSONNoExistingEntry — package.json with
        {"pi":{"extensions":["./missing.ts"]}} and no such file → false (case c requires
        ≥1 existing entry).
      * TestHasExtensionEntryPackageJSONNoPiField — package.json with {"name":"x"} (no pi)
        → false. But if that dir ALSO has index.ts, case (b) fires first → true. Test the
        pure case: package.json with no pi field AND no index.* AND no *.ts/*.js → false.
      * TestHasExtensionEntryEmptyDir — empty dir → false.
      * TestHasExtensionEntryOnlyNonEntryFiles — only README.md → false.
      * TestHasExtensionEntryRootItselfIsResolvable — HasExtensionEntry(dir) where
        dir/index.ts exists → true (case b applies to the root; WalkDir visits root first).
      * TestHasExtensionEntryIgnoresIndexAsSingleFileInNonResolvableDir — a dir containing
        ONLY index.ts: that dir IS resolvable (case b) → true. To test the case-a EXCLUSION
        of index.* in isolation, assert that a dir containing ONLY `index.ts` is recognized
        via case (b) (true), AND separately that `index.ts` does NOT count as a single-file
        entry by checking a dir whose ONLY .ts file is a nested index.ts inside a subdir
        that has nothing else — that subdir is still resolvable via case (b). The exclusion
        matters for Index() (P1.M2.T2), not for the predicate; document this in a comment.
      * TestHasExtensionEntryShortCircuits — (best-effort) create a huge fake tree where
        the first entry is shallow and a non-entry file is deep; assert true is returned
        quickly. (Optional / informational — the sentinel already guarantees early exit.)
  - EVERY env test must have a "Do NOT call t.Parallel() — mutates env" comment and must
    NOT call t.Parallel(). (Mirrors skilldozer convention.)
  - COVERAGE: Source (via String), findEnv (all 7 branches), HasExtensionEntry (all 3
    entry kinds + negative cases + root-is-resolvable + package.json variants).
  - PLACEMENT: internal/extdir/extdir_test.go.

Task 3: VALIDATE build, vet, test
  - RUN: cd /home/dustin/projects/weave && go build ./...           # expect exit 0
  - RUN: go build ./internal/extdir/                                # standalone, exit 0
  - RUN: go vet ./internal/extdir/                                  # expect exit 0, clean
  - RUN: go test ./internal/extdir/ -v                              # expect ALL PASS
  - RUN: grep -rn "yaml.v3\|gopkg.in" . --include=*.go ; grep -q "^require" go.mod  # both nothing
  - EXPECT: build clean, vet clean, all tests pass, no third-party import, no require block.
```

### Implementation Patterns & Key Details

```go
// Package extdir doc comment — mirror skilldozer's structure, adapt for weave renames
// and note that THIS file implements only rule 1 + the predicate (rules 2-4 + Find land
// in S2/S3). Roughly:
//
// // Package extdir locates the on-disk extensions/ directory for weave.
// //
// // It implements the PRD §8.3 priority order (first hit wins):
// //
// //   1. weave_EXTENSIONS_DIR env var — override; if set and an existing dir, use it as-is.
// //   2. Config file `store` (PRD §8.1) — the primary, set by `weave init`.   [S2]
// //   3. Sibling of the running binary (symlink-aware via os.Executable + EvalSymlinks). [S2]
// //   4. Walk up from the current working directory.                          [S3]
// //   5. None ⇒ unconfigured: Find returns ErrNotFound.                       [S3]
// //
// // THIS FILE (P1.M1.T3.S1) implements rule 1 (findEnv) and the HasExtensionEntry
// // predicate. Find() and rules 2-4 are added by S2/S3.
// //
// // HasExtensionEntry reports whether a directory contains at least one extension
// // entry (PRD §7.1) at any depth. It is the §8.2 cwd-auto-detect predicate used by
// // `weave init`, and the qualifier for the §8.3 rule 4 walk-up. It short-circuits
// // on the first entry found.

// findEnv — PORT skilldozer's body VERBATIM (only envVar's value differs):
func findEnv() (dir string, src Source, found bool) {
    val, ok := os.LookupEnv(envVar)
    if !ok || val == "" {
        return "", 0, false
    }
    info, err := os.Stat(val)
    if err != nil || !info.IsDir() {
        return "", 0, false // not an existing dir -> let the next rule try
    }
    abs, err := filepath.Abs(val)
    if err != nil {
        return "", 0, false // cwd unresolvable -> let the next rule try
    }
    return abs, SourceEnv, true
}

// HasExtensionEntry — NEW for weave (shape from HasSkillMD, recognition from PRD §7.1):
func HasExtensionEntry(dir string) bool {
    found := false
    _ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // skip unreadable entry, keep walking
        }
        if d.IsDir() {
            if isResolvableDir(path) {
                found = true
                return errExtensionFound // this dir IS an entry -> short-circuit
            }
            return nil // plain category dir -> WalkDir descends naturally
        }
        // file entry:
        if isExtensionFile(d.Name()) {
            found = true
            return errExtensionFound // single-file extension -> short-circuit
        }
        return nil
    })
    return found
}

// GOTCHA (WalkDir + sentinel): returning errExtensionFound from the callback makes
// WalkDir stop AND return that error; we discard it with `_ =`. The `found` closure
// var is the actual signal. This is identical to skilldozer's HasSkillMD shape.

// GOTCHA (root visit): WalkDir calls the callback with `dir` itself first. If `dir`
// is resolvable (e.g. dir/index.ts exists), isResolvableDir(dir) fires immediately →
// true. Correct per PRD §7.1.

// lenient package.json parse (case c): typed struct field silently drops non-string
// elements; missing pi.extensions → nil slice → no-op loop → false. No custom coercion.
```

### Integration Points

```yaml
DATABASE:
  - none. Pure filesystem + env.

CONFIG:
  - This subtask does NOT read the config file. findConfig (rule 2) is S2 and will
    import internal/config (File/Load/Path from P1.M1.T2.S1). S1's package skeleton
    (Source type, findEnv, HasExtensionEntry) is shaped so S2 can add findConfig without
    refactoring existing symbols.

ROUTES / API:
  - none. weave is a CLI; this package has no handlers. Consumed by:
      * Find() (P1.M1.T3.S3) — will call findEnv() first (rule 1), then findConfig (S2),
        findSibling (S2), findWalkUp (S3), and return the first hit.
      * main.runInit / chooseStore (P1.M4.T4) — will call HasExtensionEntry(cwd) to decide
        whether cwd already looks like a store (PRD §8.2 auto-detect).
      * findWalkUp / findWalkUpAncestor (P1.M1.T3.S3) — will call HasExtensionEntry on
        each ancestor's extensions/ subdir as the match qualifier.

MODULE:
  - Package imports as github.com/dabstractor/weave/internal/extdir. go.mod gains NO
    require block — stdlib only (errors, encoding/json, io/fs, os, path/filepath, strings).
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

# No third-party deps leaked in.
grep -rn "yaml.v3\|gopkg.in" --include=*.go . || echo "no third-party import (correct)"
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
# EXPECTED: "no third-party import (correct)" and "no require block (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the extdir package tests verbosely.
go test ./internal/extdir/ -v
# EXPECTED: ALL tests pass. Pay special attention to:
#   - TestFindEnvDoesNotResolveSymlinks (no-EvalSymlinks contract)
#   - TestFindEnvExistingDir / TestFindEnvRelativePathAbsolutized (filepath.Abs)
#   - TestHasExtensionEntryFoundPackageJSON (case c, with existing entry)
#   - TestHasExtensionEntryPackageJSONNoExistingEntry (case c, requires existing)
#   - TestHasExtensionEntryRootItselfIsResolvable (WalkDir visits root first)
#   - TestSourceString (4 exact labels + "unknown")
# If any fail, read the failure, fix extdir.go (NOT the test — tests encode the contract).

# Race detector sanity (closure var `found` is written from the WalkDir callback).
go test -race ./internal/extdir/
# EXPECTED: passes, no data races. (WalkDir is single-threaded, but -race is cheap.)
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (config from P1.M1.T2.S1 may also be present; both must compile).
go build ./...
# EXPECTED: exit 0.

# Standalone package build (proves no hidden cross-package coupling — S1 must NOT
# import internal/config).
go build ./internal/extdir/
# EXPECTED: exit 0.

# Confirm extdir does not depend on config at this stage (boundary check).
! grep -q "internal/config" internal/extdir/extdir.go && echo "no config import (correct)" \
  || echo "FAIL: S1 imported config (belong to S2)"

# Confirm no go.sum was generated.
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum exists — a dep leaked in"

# go mod tidy is a no-op on a zero-dep module.
go mod tidy
git diff --stat go.mod   # expect: empty / unchanged
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove HasExtensionEntry behaves like pi's own resolver for the
# package.json case, using a realistic package layout from PRD §7.1's worked example.
tmp=$(mktemp -d)
mkdir -p "$tmp/pkg/src"
cat > "$tmp/pkg/package.json" <<'JSON'
{ "name": "summarizer", "pi": { "extensions": ["./src/index.ts"] } }
JSON
echo 'export default (pi) => {};' > "$tmp/pkg/src/index.ts"
# A tiny throwaway main OUTSIDE the repo to exercise the exported predicate.
mkdir -p /tmp/weave-smoke
cat > /tmp/weave-smoke/main.go <<'GO'
package main
import (
    "fmt"
    "github.com/dabstractor/weave/internal/extdir"
)
func main() {
    fmt.Println("entry?", extdir.HasExtensionEntry("/tmp/weave-pkg-probe"))
    fmt.Println("label:", extdir.SourceEnv)
}
GO
sed -i "s|/tmp/weave-pkg-probe|$tmp/pkg|" /tmp/weave-smoke/main.go
( cd /tmp/weave-smoke && go run main.go )   # expect: entry? true  /  label: weave_EXTENSIONS_DIR
rm -rf "$tmp" /tmp/weave-smoke
# EXPECTED: "entry? true" and "label: weave_EXTENSIONS_DIR".
# (If you skip the throwaway main, the unit tests already cover this — the smoke is optional confidence.)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l internal/extdir/` empty; `go vet ./internal/extdir/` clean;
      no third-party import; no `require` block in go.mod.
- [ ] Level 2 passed: `go test ./internal/extdir/ -v` — ALL tests pass, including the
      no-EvalSymlinks symlink test and every HasExtensionEntry case.
- [ ] Level 3 passed: `go build ./...` exit 0; `go build ./internal/extdir/` exit 0;
      no `internal/config` import in extdir.go (S2's job); no go.sum.
- [ ] `go test -race ./internal/extdir/` passes (no data races).

### Feature Validation

- [ ] `Source`, the four constants, `String()`, `findEnv`, `envVar`, `HasExtensionEntry`
      all present with the exact signatures/labels from the contract.
- [ ] `findEnv` returns `("", 0, false)` for unset/empty/nonexistent/non-dir; returns
      `(filepath.Abs(val), SourceEnv, true)` for an existing dir; NEVER calls EvalSymlinks.
- [ ] `String()` returns exactly the four PRD §8.3 labels + `"unknown"` default.
- [ ] `HasExtensionEntry` recognizes all three PRD §7.1 entry kinds (file/dir/package),
      short-circuits on first find, returns false for empty/non-entry dirs, and requires
      ≥1 existing entry in a `pi.extensions` array for case (c).

### Code Quality Validation

- [ ] Package doc comment explains: §8.3 priority order, that THIS file is rule 1 +
      predicate only (rules 2-4 + Find in S2/S3), the no-EvalSymlinks contract for findEnv,
      and the dual role of HasExtensionEntry (init auto-detect + walk-up qualifier).
- [ ] Mirrors skilldozer's Source/findEnv/findEnv-test structure verbatim (with renames).
- [ ] HasExtensionEntry shape mirrors skilldozer's HasSkillMD (WalkDir + sentinel +
      early-exit) but with weave's 3-kind recognition logic from PRD §7.1.
- [ ] File placement: `internal/extdir/extdir.go` + `internal/extdir/extdir_test.go`.
- [ ] Anti-patterns avoided: no `fmt.Errorf` wrapping; no third-party import; no
      `internal/config` import (S2's boundary); no `init()`; no exported sentinel; no
      `filepath.Glob("**")` (unsupported); WalkDir callback does not propagate unreadable-entry errors.

### Documentation & Deployment

- [ ] Package is self-documenting via the doc comment (no separate README — internal package).
- [ ] No new user-facing env vars introduced BY THIS SUBTASK beyond `weave_EXTENSIONS_DIR`,
      which PRD §8.3 already specifies and is documented in README at P1.M6.T5.
- [ ] Per the item contract: "DOCS: none — internal package." Do NOT add README content here.

---

## Anti-Patterns to Avoid

- ❌ Don't use uppercase `WEAVE_EXTENSIONS_DIR`. The env var is **lowercase**
  `weave_EXTENSIONS_DIR` (PRD §8.3 + item contract). os.LookupEnv is case-sensitive.
  skilldozer uses uppercase; weave does NOT.
- ❌ Don't call `filepath.EvalSymlinks` on the env path in `findEnv`. The user points exactly
  where they want; a symlink is preserved verbatim, only absolutized via `filepath.Abs`.
  (Rule 3 sibling resolution DOES use EvalSymlinks — but that is S2, not this subtask.)
- ❌ Don't port `HasSkillMD`'s recognition logic. skilldozer walks for one filename
  (`SKILL.md`); weave walks for THREE entry kinds (PRD §7.1): a `*.ts`/`*.js` file (not
  `index.*`), a dir with `index.ts`/`index.js`, or a dir with `package.json` whose
  `pi.extensions` names ≥1 existing entry. Port only the WalkDir + sentinel SHAPE.
- ❌ Don't count a `package.json` with `pi.extensions` if none of its entries exist on disk.
  pi's own resolver (pi_extension_facts.md §4) counts only existing entries; the predicate
  mirrors that. A `pi.extensions:["./missing.ts"]` with no such file → FALSE.
- ❌ Don't use `filepath.Glob` with `**`. Go's `path/filepath` does NOT support `**` (it
  behaves like single-level `*`). `filepath.WalkDir` is the correct stdlib tool and recurses
  to arbitrary depth.
- ❌ Don't propagate unreadable-entry errors from the WalkDir callback. Return `nil` to skip
  and keep walking — a single unreadable subdir must not abort the whole predicate (mirrors
  skilldozer's HasSkillMD).
- ❌ Don't import `internal/config` in this subtask. `findConfig` (rule 2) is S2's work.
  Adding the import now breaks the subtask boundary and couples S1 to S2/config.
- ❌ Don't add `findConfig`, `findSibling`, `resolveSiblingFromExe`, `findWalkUp`,
  `findWalkUpAncestor`, `Find`, or `ErrNotFound` — they belong to S2/S3. This subtask is
  rule 1 + predicate ONLY.
- ❌ Don't write custom JSON coercion for `package.json`. A typed struct field
  (`Pi.Extensions []string`) silently drops non-string elements — that IS the lenient
  "wrong-typed fields coerced or ignored" behavior PRD §7.3 wants.
- ❌ Don't call `t.Parallel()` in any env test — they all mutate env via `t.Setenv`/
  `unsetEnvVar`. Mirror skilldozer's "Do NOT call t.Parallel()" comment.
- ❌ Don't lower the test bar by editing tests to make them pass. The tests encode the
  contract; if a test fails, fix `extdir.go`.
- ❌ Don't import any third-party package. Zero deps is a hard PRD §4/§17 constraint.
