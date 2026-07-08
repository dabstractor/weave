# PRP — P1.M1.T2.S1: `internal/config` — File struct + hand-rolled Load/Save + Path() + DefaultStore()

## Goal

**Feature Goal**: Create the `internal/config` package — the settings sidecar that records
**only** the store location (the absolute path to the extensions directory). This is the
"one fixed, well-known path the binary can bootstrap from" (PRD §8.1): a trivial YAML file
with a single `store:` key, parsed by a hand-rolled ~10-line line scanner with **zero**
third-party dependencies (no `gopkg.in/yaml.v3` — that is skilldozer's choice, explicitly
forbidden for weave per PRD §4/§17 and architecture_mapping.md §1).

**Deliverable**: One new Go file, `internal/config/config.go`, exporting:
- `type File struct { Store string }`
- `func Load(path string) (File, error)` — hand-rolled line scan for `^store:\s*(.+)$`
- `func Save(path string, f File) error` — deterministic `store: <value>\n` writer
- `func Path() (string, error)` — `$weave_CONFIG` override or `$XDG_CONFIG_HOME/weave/config.yaml`
- `func DefaultStore() (string, error)` — `$XDG_DATA_HOME/weave/extensions` or `~/.local/share/weave/extensions`

Plus one test file `internal/config/config_test.go` mirroring skilldozer's contract tests
(adapted for the hand-rolled scanner's "no `store:` line ⇒ `File{}, nil`" delta).

All four functions are **pure functions of env vars / filesystem** — no mocks, no globals,
no state. Consumed downstream by `extdir.findConfig` (P1.M1.T3) and `main.runInit`
(P1.M4.T4).

**Success Definition**:
- `go build ./...` exits 0 (and finds the new package — no more "matched no packages").
- `go vet ./internal/config/` exits 0 (clean).
- `go test ./internal/config/ -v` passes ALL tests, including the round-trip, exact-format,
  `fs.ErrNotExist`, unknown-keys-ignored, present-but-no-store-line-⇒-`File{},nil`, and all
  Path/DefaultStore env-var resolution tests.
- No `gopkg.in/yaml.v3` anywhere (no import, no `require` block, no `go.sum`).
- The package compiles standalone: `go build ./internal/config/` exits 0.

## Why

- PRD §8.1 makes the config file "the one fixed, well-known path the binary can bootstrap
  from" — every downstream resolution rule (§8.3 rule 2: config-file `store`) flows through
  this package. extdir cannot locate the extensions dir (P1.M1.T3) until `Load` exists;
  `weave init` (P1.M4.T4) cannot write the store until `Save` + `Path` + `DefaultStore`
  exist.
- PRD §4/§17 + architecture_mapping.md §1 mandate **zero** third-party deps. skilldozer
  pulls in `yaml.v3` for one key; weave hand-rolls the parser to kill that dependency.
  This subtask is where that decision is realized in code.
- The error convention (return `os.ReadFile` error verbatim so callers can
  `errors.Is(err, fs.ErrNotExist)`) is load-bearing: extdir's config-file resolution rule
  (P1.M1.T3.S2) MUST be able to distinguish "config missing" (fall through to rule 3) from
  "config present but unreadable" (also fall through, but differently). This convention is
  identical to skilldozer's and is pinned by tests.
- The "present-but-no-`store:`-line ⇒ `File{}, nil`" rule (PRD §8.1 "A missing or
  unreadable config is treated as 'not yet configured' and falls through … never a hard
  error") is the single biggest behavioral delta from skilldozer and is pinned by a
  dedicated test.

## What

A single internal Go package with five exported symbols. No CLI surface, no user-facing
behavior change (this package is consumed by later subtasks, not invoked directly).

### Success Criteria

- [ ] `internal/config/config.go` exists and defines `File`, `Load`, `Save`, `Path`,
      `DefaultStore` exactly as specified in the contract.
- [ ] `Load` uses a hand-rolled line scan (NO `yaml.v3`); unknown keys are ignored; a
      present file with no `store:` line returns `File{}, nil`.
- [ ] `Load` returns the raw `os.ReadFile` error verbatim (so `errors.Is(err, fs.ErrNotExist)`
      works) when the file is missing.
- [ ] `Save` writes exactly `store: <value>\n`, creates parent dirs via `MkdirAll(0o755)`,
      writes the file with mode `0o644`.
- [ ] `Path` honors `$weave_CONFIG` (lowercase) as a literal cleaned path (relative values
      NOT joined to config home), else falls back to `os.UserConfigDir()/weave/config.yaml`,
      returning the `UserConfigDir` error verbatim.
- [ ] `DefaultStore` honors an absolute `$XDG_DATA_HOME` (relative is ignored), else falls
      back to `os.UserHomeDir()/.local/share/weave/extensions`, returning the `UserHomeDir`
      error verbatim.
- [ ] `go build ./...`, `go vet ./internal/config/`, and `go test ./internal/config/` all pass.
- [ ] No third-party import (`grep -r yaml.v3` returns nothing in the weave repo).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP gives the exact contract for every
function (signature, error semantics, return values), the exact file to mirror
(skilldozer's `internal/config/config.go`, read in full during research), the exact test
patterns to port (skilldozer's `config_test.go`, with the one weave-specific delta called
out), the exact env-var names and path segments, and the verified build/vet/test gates.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec for §8.1 (config file format + location), §8.2 (default store),
       §8.3 (resolution priority — this package feeds rule 2), §4/§17 (zero-deps constraint).
  critical: §8.1 env var is lowercase `weave_CONFIG`. §8.1 "missing or unreadable config is
            treated as 'not yet configured' … never a hard error" ⇒ Load returns File{},nil
            for a present-but-no-store-line file (NOT an error).

- file: /home/dustin/projects/skilldozer/internal/config/config.go
  why: The PRIMARY pattern to mirror — same File/Load/Save/Path/DefaultStore API surface,
       same error conventions (verbatim os.ReadFile / UserConfigDir / UserHomeDir errors),
       same MkdirAll+WriteFile Save shape, same Path/DefaultStore env resolution logic.
  pattern: |
    type File struct { Store string }                 // weave: drop the yaml struct tag
    func Load(path) (File, error)                      // weave: hand-rolled scan, NOT yaml.Unmarshal
    func Save(path, f) error                           // weave: []byte("store: "+f.Store+"\n"), NOT yaml.Marshal
    func Path() (string, error)                        // weave: configEnv="weave_CONFIG", segs "weave"/"config.yaml"
    func DefaultStore() (string, error)                // weave: segs "weave"/"extensions"
  gotcha: skilldozer IMPORTS gopkg.in/yaml.v3 — weave MUST NOT. skilldozer treats broken
          YAML as a HARD error (yaml.Unmarshal fails); weave treats "no store: line" as
          File{},nil (not an error). This is the single biggest behavioral delta.

- file: /home/dustin/projects/skilldozer/internal/config/config_test.go
  why: The PRIMARY test pattern to port — writeConfig helper, t.Setenv for env tests
       (NO t.Parallel), contract tests for round-trip / unknown-keys / fs.ErrNotExist /
       exact-format / parent-dir-creation / Path env resolution / DefaultStore env resolution.
  pattern: Mirror every test 1:1 with "skilldozer"→"weave" and "skills"→"extensions".
  gotcha: DROP TestLoadMalformedYAMLIsHardError (weave has no hard-error case — junk input
          → File{},nil). ADD TestLoadPresentButNoStoreLineIsFileEmptyNil and
          TestLoadMalformedOrJunkIsFileEmptyNil for the weave delta.
          configEnv is package-internal in skilldozer (lowercase const) — the weave test
          can reference it the same way since the test is in package config.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §1 maps skilldozer config → weave config line by line, including the exact code
       skeleton and the "no yaml.v3 / hand-roll" decision.
  critical: §1 explicitly says "A present-but-unparseable config is ALSO treated as
            fall-through … because our hand-rolled parser is so simple (one regex per line)
            that 'unparseable' really just means 'no store: line found', which is the same
            as 'not configured.'" This is the authoritative interpretation of the contract.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/external_deps.md
  why: Confirms zero-deps invariant and lists the exact stdlib packages config needs
       (os, path/filepath; regexp is listed as optional for the line scan).
  critical: "Config file (store: key) → Hand-rolled ~10-line line scanner: split lines,
            regex ^store:\s*(.+)$". Keep the parser small.

- file: /home/dustin/projects/weave/go.mod  (produced by P1.M1.T1.S1)
  why: Establishes the module root `github.com/dabstractor/weave` and `go 1.25`. The
       config package imports as `github.com/dabstractor/weave/internal/config`.
  critical: go.mod MUST already exist (P1.M1.T1.S1 contract) with NO require block. If a
            require block appears after this subtask, something went wrong.
```

### Current Codebase tree

```bash
# After P1.M1.T1.S1 (assumed complete per its PRP contract):
$ cd /home/dustin/projects/weave && ls -A1
.git
.gitignore      # PRD §16 five-line list (from P1.M1.T1.S1)
LICENSE         # MIT, Copyright (c) 2026 Dustin Schultz (from P1.M1.T1.S1)
PRD.md          # read-only spec
go.mod          # module github.com/dabstractor/weave / go 1.25 / no require (from P1.M1.T1.S1)
plan/
# (NO source yet — P1.M1.T1.S1 deliberately creates none)
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   └── config/
│       ├── config.go          # NEW — File/Load/Save/Path/DefaultStore (this subtask)
│       └── config_test.go     # NEW — contract tests mirroring skilldozer (this subtask)
├── go.mod                     # unchanged (no new require — zero deps)
└── ...                        # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: env var is LOWERCASE `weave_CONFIG` (PRD §8.1), NOT `WEAVE_CONFIG`.
// os.Getenv is case-sensitive on Linux. skilldozer uses uppercase SKILLDOZER_CONFIG;
// weave does NOT. This is pinned by TestPathWeaveConfigAbsoluteOverride.

// CRITICAL: A present file with NO `store:` line returns File{}, nil — NOT an error.
// This is the weave delta from skilldozer (where broken YAML is a hard error).
// Pinned by TestLoadPresentButNoStoreLineIsFileEmptyNil.
// Reasoning (architecture_mapping.md §1): the hand-rolled parser is so simple that
// "unparseable" == "no store: line found" == "not configured" → fall through.

// CRITICAL: Load returns the raw os.ReadFile error VERBATIM (do NOT wrap with fmt.Errorf).
// extdir.findConfig (P1.M1.T3.S2) relies on errors.Is(err, fs.ErrNotExist) to fall through.
// Same convention as skilldozer — pinned by TestLoadMissingFileIsErrNotExist.

// CRITICAL: Path/DefaultStore return UserConfigDir/UserHomeDir errors VERBATIM too.
// extdir treats ANY Path/DefaultStore error as "unavailable → fall through".

// CRITICAL: Save output is DETERMINISTIC — exactly `store: <value>\n`. No trailing
// newline-pair, no BOM, no "..." YAML doc-end, no struct-field sorting. Pinned by
// TestSaveWritesExactFormat (reads raw bytes, NOT via Load).

// CRITICAL: Path does NOT join a relative $weave_CONFIG to the config home — it returns
// the cleaned literal value (relative OR absolute both work, for tests/profiles).
// Pinned by TestPathWeaveConfigRelativeOverrideNotJoined (asserts result stays relative
// and contains no "weave" segment).

// CRITICAL: DefaultStore IGNORES a relative $XDG_DATA_HOME (per XDG spec) — guarded by
// filepath.IsAbs. Pinned by TestDefaultStoreRelativeXDGDataHomeIgnored.

// GOTCHA (build): once config.go exists, `go build ./...` no longer prints the
// "matched no packages" warning (P1.M1.T1.S1 gotcha). That warning disappearing is
// EXPECTED and correct here — do not try to "restore" it.

// GOTCHA (test): every Path/DefaultStore test calls t.Setenv, so NONE of them may call
// t.Parallel(). This mirrors skilldozer's config_test.go convention exactly (every env
// test has a "Do NOT call t.Parallel()" comment). t.Setenv(var, "") correctly simulates
// "unset" because Path/DefaultStore use os.Getenv + `!=""`.

// GOTCHA (test): os.UserConfigDir() and os.UserHomeDir() read $XDG_CONFIG_HOME / $HOME
// on Linux. Tests that exercise the fallback branches MUST t.Setenv those vars to a
// controlled t.TempDir() so the result is deterministic and machine-independent.
```

## Implementation Blueprint

### Data models and structure

```go
// File is the parsed weave settings config (PRD §8.1). The single field records the
// absolute path to the extensions directory (the "store"). There is intentionally no
// catalog/index here — PRD §2/§17 forbid it.
type File struct {
    Store string
}
```

No other types. No sentinels (errors are the raw stdlib errors). No interfaces.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/config.go
  - WRITE the package with a doc comment (see "Implementation Patterns" below).
  - IMPORTS (stdlib only):
      import (
          "bufio"
          "bytes"
          "os"
          "path/filepath"
      )
    (bufio + bytes for the line scan; os for ReadFile/MkdirAll/WriteFile/Getenv/
    UserConfigDir/UserHomeDir; path/filepath for Clean/Join/IsAbs/Dir.)
    NOTE: do NOT import gopkg.in/yaml.v3 or regexp. A bufio.Scanner + bytes.TrimRight
    + a manual "store:" prefix check is ~10 lines and avoids regexp entirely.
    (regexp is acceptable per external_deps.md but unnecessary — plain string ops
    are simpler and the contract says "~10-line hand-rolled reader".)
  - DEFINE: type File struct { Store string }
  - IMPLEMENT Load(path string) (File, error):
      data, err := os.ReadFile(path)
      if err != nil { return File{}, err }   // VERBATIM — do not wrap
      // scan lines for ^store:\s*(.+)$
      scan := bufio.NewScanner(bytes.NewReader(data))
      for scan.Scan() {
          line := scan.Text()
          // strip trailing whitespace (handles \r if present)
          line = strings.TrimRight(line, " \t\r")
          // match "store:" prefix then require non-whitespace after optional spaces
          rest := strings.TrimPrefix(line, "store:")
          if rest == line {
              // line did not start with "store:" — skip (unknown key, ignored)
              continue
          }
          // after "store:", skip optional spaces/tabs; remainder is the value
          val := strings.TrimLeft(rest, " \t")
          if val == "" {
              // "store:" with nothing after it — keep scanning (treat as not set)
              continue
          }
          return File{Store: val}, nil
      }
      if err := scan.Err(); err != nil {
          return File{}, err   // scanner I/O error (rare for an in-memory reader)
      }
      return File{}, nil   // no store: line found → "not configured" → NOT an error
    (You will need to add "strings" to imports. Final import block: bufio, bytes, os,
    path/filepath, strings.)
  - IMPLEMENT Save(path string, f File) error:
      out := []byte("store: " + f.Store + "\n")
      if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
          return err
      }
      return os.WriteFile(path, out, 0o644)
  - DEFINE: const configEnv = "weave_CONFIG"   // package-internal (lowercase)
  - IMPLEMENT Path() (string, error):
      if v := os.Getenv(configEnv); v != "" {
          return filepath.Clean(v), nil    // literal path, NOT joined to config home
      }
      configHome, err := os.UserConfigDir()
      if err != nil {
          return "", err    // VERBATIM
      }
      return filepath.Join(configHome, "weave", "config.yaml"), nil
  - IMPLEMENT DefaultStore() (string, error):
      if v := os.Getenv("XDG_DATA_HOME"); v != "" && filepath.IsAbs(v) {
          return filepath.Join(v, "weave", "extensions"), nil
      }
      home, err := os.UserHomeDir()
      if err != nil {
          return "", err    // VERBATIM
      }
      return filepath.Join(home, ".local", "share", "weave", "extensions"), nil
  - NAMING: package `config`; file `config.go`; dir `internal/config/`.
  - PLACEMENT: internal/config/config.go (PRD §5 layout; mirrors skilldozer internal/config/).
  - DEPENDENCIES: none beyond stdlib. Imports ONLY from the Go standard library.

Task 2: CREATE internal/config/config_test.go  (package config)
  - PACKAGE: `package config` (same package — white-box, so it can reference the
    package-internal `configEnv` const, exactly as skilldozer's test does).
  - IMPORTS: errors, io/fs, os, path/filepath, strings, testing. (NO yaml.v3.)
  - HELPER: writeConfig(t, content string) string — mirrors skilldozer's helper:
      writes content to t.TempDir()/config.yaml, returns the path. t.Helper().
  - PORT these tests from skilldozer's config_test.go VERBATIM with renames
    ("skilldozer"→"weave", "skills"→"extensions", configEnv already "weave_CONFIG"):
      * TestSaveLoadRoundTrip          — Save{Store:"/home/u/extensions"} then Load == same.
      * TestLoadIgnoresUnknownKeys     — "store: /abs\nversion: 3\ncolors: red\n" → /abs, nil.
      * TestLoadMissingFileIsErrNotExist — errors.Is(err, fs.ErrNotExist) true.
      * TestSaveWritesExactFormat      — raw bytes == "store: /x\n".
      * TestSaveCreatesParentDir       — nested missing dirs created.
      * TestPathWeaveConfigAbsoluteOverride — configEnv abs wins over XDG_CONFIG_HOME.
      * TestPathWeaveConfigRelativeOverrideNotJoined — relative NOT joined, stays relative,
        contains no "weave" segment.
      * TestPathWeaveConfigEmptyFallsToXDG — configEnv="" → UserConfigDir/weave/config.yaml.
      * TestPathRejectsRelativeXDGConfigHome — relative XDG_CONFIG_HOME → err != nil, path "".
      * TestDefaultStoreAbsoluteXDGDataHome — XDG_DATA_HOME=/abs/data → /abs/data/weave/extensions.
      * TestDefaultStoreEmptyXDGDataHomeFallsToHome — XDG_DATA_HOME="" → HOME/.local/share/weave/extensions.
      * TestDefaultStoreRelativeXDGDataHomeIgnored — relative XDG_DATA_HOME → HOME fallback.
      * TestDefaultStoreHomeUnsetErrors — HOME="" → err != nil, path "".
  - ADD weave-specific tests (the deltas skilldozer does NOT have):
      * TestLoadPresentButNoStoreLineIsFileEmptyNil — file with content but no store: line
        (e.g. "version: 3\nnotes: hello\n") → File{}, nil (NOT an error). Pins PRD §8.1
        "never a hard error" + the contract clause "A present-but-no-store-line file
        returns File{}, nil".
      * TestLoadMalformedOrJunkIsFileEmptyNil — arbitrary junk (e.g. "this is not yaml\n@@@\n")
        → File{}, nil. Pins that the hand-rolled scanner never hard-errors on "unparseable"
        input (contrast skilldozer's TestLoadMalformedYAMLIsHardError which weave DROPS).
      * TestLoadPicksFirstStoreLine — a file with two "store:" lines returns the FIRST one
        (documents first-match-wins; matches the `return` inside the scan loop).
      * TestLoadStoreLineWithTrailingWhitespace — "store: /path   \n" → Store == "/path"
        (trailing whitespace trimmed per contract "trim trailing whitespace").
  - EVERY env test must have a "Do NOT call t.Parallel() — mutates env" comment and must
    NOT call t.Parallel(). (Mirrors skilldozer convention.)
  - COVERAGE: all 5 exported symbols (File via round-trip, Load, Save, Path, DefaultStore),
    positive + negative + edge cases.
  - PLACEMENT: internal/config/config_test.go.

Task 3: VALIDATE build, vet, test
  - RUN: cd /home/dustin/projects/weave && go build ./...           # expect exit 0, NO warning
  - RUN: go build ./internal/config/                                # standalone, exit 0
  - RUN: go vet ./internal/config/                                  # expect exit 0, clean
  - RUN: go test ./internal/config/ -v                              # expect ALL PASS
  - RUN: grep -rn "yaml.v3" . --include=*.go ; grep -q "require" go.mod  # both must find NOTHING
  - EXPECT: build clean, vet clean, all tests pass, no yaml.v3, no require block.
```

### Implementation Patterns & Key Details

```go
// Package config doc comment — mirror skilldozer's structure, adapt for hand-rolled parser
// and the weave deltas. Roughly:
//
// // Package config reads and writes the weave settings file (PRD §8.1), the small
// // sidecar that records only the store location (the absolute path to the extensions
// // directory). The whole §8 config model funnels through this package: extdir.findConfig
// // (P1.M1.T3) reads the store via Load, and main.runInit (P1.M4.T4) writes it via Save.
// //
// // This is a SETTINGS SIDECAR, not a catalog index. PRD §2 constraint #1 (and §17)
// // forbid a catalog enumerating the extension set — extensions are discovered by walking
// // the on-disk store. The only thing this file persists is a value the filesystem cannot
// // express (where the store lives). Do not grow this struct into a catalog.
// //
// // Parsing is a HAND-ROLLED line scan (PRD §8.1: "~10-line hand-rolled reader … we are
// // not pulling in yaml.v3 for one key"). The scanner looks for the first line matching
// // `^store:\s*(.+)$` and takes the captured value (trailing whitespace trimmed). Unknown
// // keys are ignored (room to grow). Because the parser is this simple, "unparseable"
// // reduces to "no store: line found", which is indistinguishable from "not configured":
// // Load returns File{}, nil in both cases (PRD §8.1 "never a hard error"). This differs
// // from skilldozer, where broken YAML is a hard error via yaml.v3 — weave has no such
// // case by construction.
// //
// // Path/DefaultStore are pure functions of the environment (PRD §8.1/§8.2): they read
// // env vars and compute a path, they do NOT touch the filesystem.

// Load error-convention pattern (IDENTICAL to skilldozer — return raw error, do not wrap):
func Load(path string) (File, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return File{}, err // VERBATIM — callers do errors.Is(err, fs.ErrNotExist)
    }
    // ... hand-rolled scan (see Task 1) ...
}

// Save determinism pattern (weave-specific — hand-built []byte, NOT yaml.Marshal):
func Save(path string, f File) error {
    out := []byte("store: " + f.Store + "\n") // EXACT bytes — pinned by TestSaveWritesExactFormat
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return os.WriteFile(path, out, 0o644)
}

// Path override pattern (IDENTICAL to skilldozer — literal cleaned value, NOT joined):
func Path() (string, error) {
    if v := os.Getenv(configEnv); v != "" {
        return filepath.Clean(v), nil // relative values stay relative (for tests/profiles)
    }
    // ... UserConfigDir fallback ...
}

// The line-scan gotcha: after stripping "store:", you MUST TrimLeft the optional
// whitespace BEFORE checking emptiness, so "store:/abs" (no space) and "store: /abs"
// (one space) and "store:   /abs" (many spaces) all yield "/abs". And you MUST
// TrimRight trailing whitespace from the WHOLE line first (to handle \r on files
// saved by Windows editors), per the contract "trim trailing whitespace".
```

### Integration Points

```yaml
DATABASE:
  - none. Settings sidecar only; no DB, no migrations.

CONFIG (this IS the config package):
  - The package itself defines where the config file lives (Path) and what the default
    store is (DefaultStore). No separate settings.py / config.json.

ROUTES / API:
  - none. weave is a CLI; this package has no handlers. It is consumed by:
      * extdir.findConfig (P1.M1.T3.S2) — calls config.Path() then config.Load() to
        implement §8.3 rule 2 (config-file store). On Load error or empty Store,
        findConfig falls through to rule 3 (sibling of binary).
      * main.runInit (P1.M4.T4) — calls config.Path() to pick the write target, config.DefaultStore()
        for the out-of-the-box offer, and config.Save() to persist the chosen store.

MODULE:
  - The package imports as github.com/dustomabs/weave/internal/config (module path from
    P1.M1.T1.S1's go.mod). go.mod gains NO require block — stdlib only.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/weave

# Format check (Go has no ruff; gofmt is the equivalent gate).
gofmt -l internal/config/
# EXPECTED: no output (empty list = all files already formatted). If a file is listed,
# run `gofmt -w internal/config/` and re-check.

# go vet (the Go equivalent of mypy+lint — catches shadowed vars, misused printf, etc.).
go vet ./internal/config/
# EXPECTED: exit 0, no output.

# No third-party deps leaked in.
grep -rn "yaml.v3" --include=*.go . || echo "no yaml.v3 (correct)"
grep -q "^require" go.mod && echo "FAIL: require block appeared" || echo "no require block (correct)"
# EXPECTED: "no yaml.v3 (correct)" and "no require block (correct)".
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/weave

# Run the config package tests verbosely.
go test ./internal/config/ -v
# EXPECTED: ALL tests pass (the ~17 tests listed in Task 2). Pay special attention to:
#   - TestLoadPresentButNoStoreLineIsFileEmptyNil (weave delta)
#   - TestLoadMalformedOrJunkIsFileEmptyNil       (weave delta)
#   - TestSaveWritesExactFormat                   (deterministic bytes)
#   - TestLoadMissingFileIsErrNotExist            (errors.Is contract)
#   - TestPathWeaveConfigRelativeOverrideNotJoined (no-join contract)
# If any fail, read the failure, fix config.go (NOT the test — the tests encode the contract),
# and re-run.

# Race detector sanity (config is pure/env-only, but cheap to confirm).
go test -race ./internal/config/
# EXPECTED: passes, no data races.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# Whole-repo build (the package must compile in the module context, not just standalone).
go build ./...
# EXPECTED: exit 0. NOTE: the P1.M1.T1.S1 "matched no packages" warning is GONE now —
# that is correct and expected once any package exists. Do not try to "fix" its absence.

# Standalone package build (proves no hidden cross-package coupling).
go build ./internal/config/
# EXPECTED: exit 0.

# Confirm no go.sum was generated (zero-deps invariant).
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum exists — a dep leaked in"

# go mod tidy is a no-op on a zero-dep module (sanity only; should not modify go.mod).
go mod tidy
git diff --stat go.mod   # expect: empty / unchanged
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/weave

# Domain-specific: prove the Load/Save round-trip and the "no store: line ⇒ File{},nil"
# rule end-to-end with a real temp file (a mini integration check beyond the unit tests).
tmp=$(mktemp -d)
cfg="$tmp/config.yaml"
printf 'store: %s/extensions\n' "$tmp" > "$cfg"
go run ./internal/config/ 2>/dev/null || true   # no main; just confirms the package builds

# (If a tiny throwaway main is desired for manual smoke, write it OUTSIDE the repo, e.g.
# /tmp/smoke/main.go importing github.com/dabstractor/weave/internal/config, then
# `go run /tmp/smoke/main.go`. Do NOT add a main.go to this package — main.go is P1.M1.T4.)

# Manual cross-check against skilldozer's behavior (optional, confidence only):
# skilldozer's config.Load on a junk file HARD-errors; weave's returns File{},nil.
# This is the intended divergence — do not "fix" weave to match skilldozer here.
diff <(grep -c 'yaml.v3' /home/dustin/projects/skilldozer/internal/config/config.go) \
      <(grep -c 'yaml.v3' internal/config/config.go 2>/dev/null || echo 0)
# EXPECTED: "1\n0\n" (skilldozer has 1 yaml.v3 import; weave has 0).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `gofmt -l internal/config/` empty; `go vet ./internal/config/` clean;
      no `yaml.v3` import; no `require` block in go.mod.
- [ ] Level 2 passed: `go test ./internal/config/ -v` — ALL tests pass, including the four
      weave-delta tests (no-store-line, junk, first-store-line, trailing-whitespace).
- [ ] Level 3 passed: `go build ./...` exit 0; `go build ./internal/config/` exit 0; no go.sum.
- [ ] `go test -race ./internal/config/` passes (no data races).

### Feature Validation

- [ ] `File`, `Load`, `Save`, `Path`, `DefaultStore` all exported with the exact signatures
      from the contract.
- [ ] `Load` returns `File{}, nil` for a present file with no `store:` line (NOT an error).
- [ ] `Load` returns the raw `os.ReadFile` error verbatim (errors.Is(err, fs.ErrNotExist) works).
- [ ] `Save` writes exactly `store: <value>\n` (raw-byte check, not via Load).
- [ ] `Path` honors lowercase `weave_CONFIG` as a literal cleaned path (relative NOT joined).
- [ ] `Path` falls back to `os.UserConfigDir()/weave/config.yaml`, error verbatim.
- [ ] `DefaultStore` honors absolute `XDG_DATA_HOME`, ignores relative, falls back to
      `os.UserHomeDir()/.local/share/weave/extensions`, error verbatim.

### Code Quality Validation

- [ ] Package doc comment explains: sidecar-not-catalog, hand-rolled (no yaml.v3),
      "no store: line ⇒ not configured ⇒ File{},nil", Path/DefaultStore are pure env functions.
- [ ] Mirrors skilldozer's config.go structure and error conventions (verbatim raw errors).
- [ ] Diverges from skilldozer ONLY where the contract requires (no yaml.v3; no hard-error
      on junk input; "weave"/"extensions" segments; lowercase `weave_CONFIG`).
- [ ] File placement: `internal/config/config.go` + `internal/config/config_test.go`.
- [ ] Anti-patterns avoided: no `fmt.Errorf` wrapping of stdlib errors; no `regexp` for a
      ~10-line job; no global state; no `init()`; no exported sentinel errors.

### Documentation & Deployment

- [ ] Package is self-documenting via the doc comment (no separate README — internal package).
- [ ] No new user-facing env vars introduced BY THIS SUBTASK (it only READS the env vars
      PRD §8.1 already specifies: `weave_CONFIG`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `HOME`).
- [ ] Per the item contract: "DOCS: none — internal package; `weave_CONFIG` env var is
      documented in README (Mode B final task P1.M6.T5)." Do NOT add README content here.

---

## Anti-Patterns to Avoid

- ❌ Don't import `gopkg.in/yaml.v3` (or any third-party package). Zero deps is a hard PRD
  §4/§17 constraint. skilldozer uses yaml.v3; weave explicitly does NOT.
- ❌ Don't make `Load` return an error for a present-but-no-`store:`-line file. That file is
  "not configured" (PRD §8.1 "never a hard error") and returns `File{}, nil`. This is the
  weave delta from skilldozer — do NOT copy skilldozer's hard-error-on-broken-YAML behavior.
- ❌ Don't wrap the `os.ReadFile` / `os.UserConfigDir` / `os.UserHomeDir` errors with
  `fmt.Errorf`. Return them VERBATIM so callers can `errors.Is(err, fs.ErrNotExist)`.
- ❌ Don't join a relative `$weave_CONFIG` to the config home. It is a LITERAL cleaned path
  (relative or absolute both valid). skilldozer does not join either — port that behavior.
- ❌ Don't accept a relative `$XDG_DATA_HOME` in `DefaultStore`. Guard with `filepath.IsAbs`
  and fall through to the `~/.local/share` default (XDG spec; skilldozer does the same).
- ❌ Don't use `regexp` for the line scan — `strings.TrimRight` + `strings.TrimPrefix` +
  `strings.TrimLeft` is ~10 lines and clearer. (regexp is acceptable per external_deps.md
  but the contract says "~10-line hand-rolled reader"; plain string ops keep it smallest.)
- ❌ Don't add a `main.go`, an `init()`, exported sentinel errors, or any catalog/index
  fields to `File`. This is a one-key sidecar. Growing it violates PRD §2/§17.
- ❌ Don't call `t.Parallel()` in any Path/DefaultStore test — they all mutate env via
  `t.Setenv`. (Mirror skilldozer's "Do NOT call t.Parallel()" comment in each env test.)
- ❌ Don't lower the test bar to "make it pass" by editing the tests. The tests encode the
  contract; if a test fails, fix `config.go`.
- ❌ Don't run `go mod tidy` expecting it to do anything — on a zero-dep module it is a
  no-op. If it creates a `go.sum` or adds a `require`, STOP: a dependency leaked in.
- ❌ Don't write tests that depend on the developer's real `$HOME` / `$XDG_*`. Always
  `t.Setenv` to a `t.TempDir()` so tests are deterministic and machine-independent.
