# Research: skilldozer config.go (the pattern weave adapts)

## What skilldozer's config.go looks like (the contract weave MIRRORS)

Read `/home/dustin/projects/skilldozer/internal/config/config.go` in full. Key facts:

### Package doc comment
- Long block comment explaining: SETTINGS SIDECAR (not catalog), PRD §8.1/§17 forbid catalog,
  lenient = ignore unknown KEYS (not tolerate broken syntax), Path/DefaultStore are pure
  functions of env.

### File struct
```go
type File struct {
    Store string `yaml:"store,omitempty"`
}
```
weave: NO yaml tag (we hand-roll). Just `type File struct { Store string }`.

### Load(path string) (File, error)
- `os.ReadFile(path)` → on err, return `File{}, err` VERBATIM (NOT wrapped) so callers can
  `errors.Is(err, fs.ErrNotExist)`.
- skilldozer: `yaml.Unmarshal(data, &f)` → broken YAML is a HARD error.
- weave ADAPT: hand-rolled line scan. "Broken syntax" = "no `store:` line found" = same as
  "not configured" → return `File{}, nil` (NOT an error). This is the KEY delta.

### Save(path string, f File) error
- skilldozer: `yaml.Marshal(&f)` then `os.MkdirAll(filepath.Dir(path), 0o755)` then
  `os.WriteFile(path, out, 0o644)`.
- weave ADAPT: `out := []byte("store: " + f.Store + "\n")` then same MkdirAll + WriteFile.
  Deterministic output: EXACTLY `store: <value>\n`.

### configEnv constant
- skilldozer: `const configEnv = "SKILLDOZER_CONFIG"` (package-internal).
- weave: `const configEnv = "weave_CONFIG"` (LOWERCASE per PRD §8.1).

### Path() (string, error)
- If `os.Getenv(configEnv) != ""` → `return filepath.Clean(v), nil` (literal path, NOT joined
  to config home — relative values work for tests/profiles).
- Else `configHome, err := os.UserConfigDir()` → on err return `"", err` verbatim.
- Else `return filepath.Join(configHome, "skilldozer", "config.yaml"), nil`.
- weave: `"weave", "config.yaml"` instead of `"skilldozer", "config.yaml"`.

### DefaultStore() (string, error)
- If `os.Getenv("XDG_DATA_HOME") != "" && filepath.IsAbs(v)` →
  `return filepath.Join(v, "skilldozer", "skills"), nil`.
- Else `home, err := os.UserHomeDir()` → on err return `"", err` verbatim.
- Else `return filepath.Join(home, ".local", "share", "skilldozer", "skills"), nil`.
- weave: `"weave", "extensions"` instead of `"skilldozer", "skills"`.

## Imports weave needs (stdlib ONLY — no yaml.v3)
```go
import (
    "os"
    "path/filepath"
)
```
(Load uses `os.ReadFile`; Save uses `os.MkdirAll` + `os.WriteFile`; Path uses `os.Getenv`
+ `os.UserConfigDir` + `filepath.Clean`/`Join`; DefaultStore uses `os.Getenv` + `filepath.IsAbs`
+ `os.UserHomeDir` + `filepath.Join`. The hand-rolled scan uses `bufio.Scanner` OR
`bytes.Split`/`strings` — pick one; `bufio.Scanner` is cleanest for line-by-line.)

## Test patterns (from skilldozer config_test.go)

### Helpers
- `writeConfig(t, content)` writes content to `t.TempDir()/config.yaml`, returns path.
- Env tests use `t.Setenv` (so NO `t.Parallel` — this is critical and called out in every test).
- `t.Setenv(var, "")` simulates "unset" because Path/DefaultStore use `os.Getenv + !=""`.

### Contract tests weave MUST mirror (adapted for hand-rolled scanner):
1. `TestSaveLoadRoundTrip` — `Save` then `Load` preserves Store value.
2. `TestLoadIgnoresUnknownKeys` — `store: /abs\nversion: 3\ncolors: red\n` → Store=/abs, no err.
3. `TestLoadMissingFileIsErrNotExist` — `errors.Is(err, fs.ErrNotExist)` true.
4. `TestSaveWritesExactFormat` — raw bytes == `"store: /x\n"`.
5. `TestSaveCreatesParentDir` — nested missing dirs created by MkdirAll.
6. Path tests: absolute override, relative override NOT joined, empty→XDG, relative XDG rejected.
7. DefaultStore tests: abs XDG_DATA_HOME, empty→HOME, relative ignored, HOME unset errors.

### weave-specific test (NOT in skilldozer — the KEY delta):
- `TestLoadPresentButNoStoreLineIsFileEmptyNil` — a file with content but NO `store:` line
  returns `File{}, nil` (NOT an error). This is the "treated as not configured" rule from
  the item contract.
- `TestLoadMalformedOrJunkIsFileEmptyNil` — arbitrary junk text (not YAML, not `store:`)
  → `File{}, nil`. (In skilldozer this was a HARD error via yaml.v3; in weave it's nil.)

## go vet / go build on skilldozer config: CLEAN
Verified `cd /home/dustin/projects/skilldozer && go vet ./internal/config/` → no output (pass).
So weave's config must also `go vet` clean and `go build` clean.
