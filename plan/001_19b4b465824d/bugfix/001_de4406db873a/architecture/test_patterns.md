# Test Patterns Guide — Bugfix 001_de4406db873a

Reference for downstream agents writing regression tests. All helpers and
conventions are drawn from existing test files.

## Package layout (white-box tests)

| Package            | Test files                                           |
|--------------------|------------------------------------------------------|
| `internal/discover`| `discover_test.go`, `jsdoc_test.go`, `extension_test.go`, `index_test.go` |
| `internal/check`   | `check_test.go`                                      |
| `internal/extdir`  | `extdir_test.go`                                     |

All tests are **white-box** (`package discover` etc.) so they call unexported
helpers directly.

## Discover test helpers

- `writeFile(t, dir, name, content string) string` — writes file, returns path. `jsdoc_test.go`.
- `writeFileBytes(t, dir, name string, b []byte) string` — raw bytes (BOM). `jsdoc_test.go`.
- `writePackageJSON(t, dir, content string)` — writes dir/package.json. `extension_test.go`.
- `strEq(a, b []string) bool` — slice comparison. `extension_test.go`.
- `walkClassified(root string) []Extension` — mini-Index WalkDir skeleton. `discover_test.go`.
- `relTags(exts []Extension) []string` — sorted relTags. `discover_test.go`.

## Check test helpers

- `mkFileExt(t, root, relTag, body string) discover.Extension` — single-file ext.
- `mkDirExt(t, root, relTag, body string) discover.Extension` — dir ext (index.ts).
- `mkPackageExt(t, root, relTag, pkgJSON, indexBody string) discover.Extension` — package ext.
- `findExt(t, root, relTag string) discover.Extension` — Index + match by RelTag.
- `finding(rep Report, substr string) (Finding, bool)` — search findings by substring.

## Check assertion idiom

```go
rep := Check(root, exts)
if rep.Errors != N { t.Errorf("Errors=%d; want %d", rep.Errors, N) }
f, ok := finding(rep, "substr of message")
if !ok || f.Level != LevelError { t.Errorf("missing ERROR; got %+v", rep.ByExt) }
```

## ExtDir test patterns

- `unsetEnvVar(t)` — unsets weave_EXTENSIONS_DIR + neutralizes config rule. **No t.Parallel().**
- `t.Setenv(envVar, dir)` — sets env var.
- `writeCfg(t, content string) (cfgPath, cfgDir string)` — temp config.yaml.
- Symlink tests: guard with `t.Skipf("symlinks not supported: %v", err)`.
- `TestFindEnvDoesNotResolveSymlinks` — asserts findEnv does NOT EvalSymlinks (stays valid after Bug 4 fix since fix is in Index, not findEnv).

## Conventions

- Every test starts with `root := t.TempDir()` (absolute, auto-cleaned).
- `t.Fatal` for setup/primary guard failures; `t.Errorf` for field assertions.
- Message format: `"Field=%q; want %q"`.
- Table-driven tests: `cases := []struct{...}{}` + `t.Run(c.name, ...)`.
- No `t.Parallel()` for env/cwd tests (process-global state).

## Existing related tests (gaps to fill)

| Concern                          | Existing test                                | Gap                                    |
|----------------------------------|----------------------------------------------|----------------------------------------|
| Single missing pi.extensions     | `TestClassifyDirPackageNonExistentEntry`     | Only single-entry; no multi-entry      |
| Empty JSDoc                      | (none for `/**/`)                            | Degenerate overlap case                |
| Root index.ts                    | (none)                                       | Store collapse                         |
| Symlinked root                   | `TestFindEnvDoesNotResolveSymlinks`          | findEnv only; no Index test            |
| All-missing pi.extensions check  | `TestCheckEmptyCategoryFolder`               | No package.json; no ERROR case         |
| node_modules/hidden skip         | (none)                                       | Catalog pollution                      |
