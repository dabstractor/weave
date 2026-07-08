# Research Notes — P1.M1.T3.S2 (rules 2 + 3)

## 1. Source: the file to port

`/home/dustin/projects/skilldozer/internal/skillsdir/skillsdir.go` — read in full.
The two functions this subtask ports verbatim (with renames) are:

- `findConfig()` (rule 2)
- `findSibling()` + `resolveSiblingFromExe(exe string)` (rule 3)

### RENAMES (skilldozer → weave)
| skilldozer symbol | weave symbol | notes |
|---|---|---|
| `"skills"` (sibling dir literal) | `"extensions"` | inside `resolveSiblingFromExe` |
| `config.Path()` / `config.Load()` | same names, `github.com/dabstractor/weave/internal/config` | weave's config pkg (P1.M1.T2.S1) has the same API surface |
| `SourceConfig` / `SourceSibling` | same | already declared in S1 (P1.M1.T3.S1) |
| everything else | verbatim | findConfig body, resolveSiblingFromExe body, findSibling body are identical |

## 2. Rule 2 — findConfig (verbatim body)

Direct from skilldozer source:

```go
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
        return "", 0, false // no `store` key -> fall through
    }
    var store string
    if filepath.IsAbs(f.Store) {
        store = filepath.Clean(f.Store)
    } else {
        store = filepath.Join(filepath.Dir(p), f.Store) // relative to config file's dir
    }
    info, err := os.Stat(store)
    if err != nil || !info.IsDir() {
        return "", 0, false // store path is not an existing dir -> fall through
    }
    return store, SourceConfig, true
}
```

### KEY DIFFERENCE: weave vs skilldozer on malformed config
- skilldozer uses `yaml.v3` → broken YAML = hard error from `config.Load`.
  skilldozer's `findConfig` converts that hard error to `found=false` (fall through).
- weave uses a hand-rolled line scanner (P1.M1.T2.S1 config.go). A present-but-no-`store:`-line
  file returns `File{}, nil` (NOT an error). A missing/unreadable file returns the raw
  `os.ReadFile` error. So weave's `findConfig` never sees a "malformed" hard error —
  the "unparseable" case collapses to "no store key" → `File{Store:""}` → falls through
  via the `f.Store == ""` branch. Same end result, fewer branches. The contract note in
  the skilldozer test `TestFindConfigMalformedYAML` is therefore N/A for weave's parser
  shape — but we keep an equivalent test that confirms a broken-syntax file still falls
  through (the hand-rolled parser just won't find a `store:` line).

## 3. Rule 3 — findSibling + resolveSiblingFromExe (verbatim body, `skills`→`extensions`)

```go
func findSibling() (dir string, src Source, found bool) {
    exe, err := os.Executable()
    if err != nil {
        return "", 0, false
    }
    d, ok := resolveSiblingFromExe(exe)
    if !ok {
        return "", 0, false
    }
    return d, SourceSibling, true
}

func resolveSiblingFromExe(exe string) (dir string, found bool) {
    real, err := filepath.EvalSymlinks(exe)
    if err != nil {
        real = exe
    }
    repoDir := filepath.Dir(real)
    candidate := filepath.Join(repoDir, "extensions")  // <-- ONLY change: "skills" -> "extensions"
    info, err := os.Stat(candidate)
    if err != nil || !info.IsDir() {
        return "", false
    }
    return candidate, true
}
```

### CRITICAL: EvalSymlinks MUST stay (do NOT "simplify")
From `architecture_mapping.md §2` + `verified_symlink_resolution.md`:
- On Linux, `os.Executable()` resolves the symlink via `/proc/self/exe` and returns
  the REAL path directly, so `EvalSymlinks` is redundant-but-harmless.
- On macOS, `os.Executable()` may return the SYMLINK path; without `EvalSymlinks`,
  `Dir()` points at the bin dir (e.g. `~/.local/bin`) instead of the repo, and the
  sibling-of-binary rule SILENTLY misses. EvalSymlinks is REQUIRED on macOS.
- The item contract explicitly says: "EvalSymlinks is REQUIRED on macOS (redundant on
  Linux via /proc/self/exe) — do NOT drop it."

## 4. Imports needed (extdir.go additions for S2)

S1 already imports: `encoding/json`, `errors`, `io/fs`, `os`, `path/filepath`, `strings`.
S2 ADDS exactly one import:
- `"github.com/dabstractor/weave/internal/config"` (for findConfig's `config.Path` + `config.Load`)

That import introduces a package-to-package dependency within the module (no external
dep, no `require` block, no go.sum). The module already compiles config (P1.M1.T2.S1 is
Complete per plan_status).

## 5. Test patterns to port (from skillsdir_test.go)

Port these verbatim with renames (`"skills"`→`"extensions"`, `SKILLDOZER_CONFIG`→`weave_CONFIG`,
`makeFakeBinary` helper stays). The `makeSkill` helper does NOT port (it writes SKILL.md —
S2 has no HasExtensionEntry tests; those belong to S1 which is already done).

### resolveSiblingFromExe tests (the testable core — port verbatim):
- `TestResolveSiblingFromExeSymlinkCrossDir` — THE critical test: symlink in a different dir
  resolves back to the REAL binary's repo dir's `extensions/`.
- `TestResolveSiblingFromExeDirect` — direct (non-symlinked) binary with sibling `extensions/`.
- `TestResolveSiblingFromExeEvalSymlinksFallback` — non-existent exe whose parent HAS sibling
  `extensions/` wins via `real=exe` fallback.
- `TestResolveSiblingFromExeNoExtensionsDir` — binary exists, no sibling `extensions/` → miss.
- `TestResolveSiblingFromExeExtensionsIsFile` — sibling `extensions` is a regular FILE → miss (IsDir).

### findSibling test (smoke — os.Executable not controllable):
- `TestFindSiblingNoExtensionsNextToTestBinary` — the test binary runs from a temp build dir
  with no sibling `extensions/`, so `findSibling()` must return `found=false` without panic.

### findConfig tests (port verbatim, change env var + parser-shape notes):
- `TestFindConfigHit` — existing store dir via absolute `store:` → SourceConfig.
- `TestFindConfigMissingFile` — config file does not exist → fall through.
- `TestFindConfigMissingStoreKey` — config has no `store:` key → fall through.
- `TestFindConfigStoreDirAbsent` — `store:` names a nonexistent dir → fall through.
- `TestFindConfigMalformedSyntax` — ADAPT: weave's hand-rolled parser never hard-errors on
  "malformed" input (it just finds no `store:` line). Test that a file with garbage content
  and no `store:` line still falls through. (Equivalent to MissingStoreKey for weave.)
- `TestFindConfigRelativeStoreResolvedAgainstConfigDir` — relative `store:` joined against
  `filepath.Dir(configPath)`, NOT cwd.

### unsetEnvVar helper UPGRADE:
S1's `unsetEnvVar` only neutralized `envVar`. S2's version MUST ALSO neutralize the config
rule: point `weave_CONFIG` at a non-existent ghost path so `findConfig` deterministically
misses in all-miss / env-only tests. (Directly mirrors skilldozer's `unsetEnvVar` which
already does this for `SKILLDOZER_CONFIG`.) Without this, a machine with a real
`~/.config/weave/config.yaml` would make the all-miss / env-wins tests return a real dir.

## 6. makeFakeBinary helper (port verbatim)

```go
// makeFakeBinary creates a regular file at dir/name to stand in for a compiled
// binary. EvalSymlinks + os.Stat(Join(dir,"extensions")) do not require a real ELF,
// so a 1-byte file is sufficient.
func makeFakeBinary(t *testing.T, dir, name string) string {
    t.Helper()
    p := filepath.Join(dir, name)
    if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
        t.Fatalf("write fake binary %s: %v", p, err)
    }
    return p
}
```

Verified fact (skilldozer research §5): a 1-byte regular file suffices as a fake binary —
`filepath.EvalSymlinks` and `os.Stat` on its sibling do not require a real ELF executable.

## 7. Boundary: what S2 does NOT do

- Does NOT add `findWalkUp`, `findWalkUpAncestor`, `Find`, or `ErrNotFound` — those are S3.
- Does NOT add `HasExtensionEntry` tests — those belong to S1 (done).
- Does NOT modify the config package.
- Does NOT modify `main.go`.
- Does NOT touch go.mod (no require block; the internal/config import is intra-module).
