# Test fixtures & conventions for P1.M3.T2.S1

## Existing helpers in weave main_test.go (REUSE, do not duplicate)

### `unsetExtEnv(t)` — neutralize env rules for failure tests
Sets `weave_EXTENSIONS_DIR` and `weave_CONFIG` (both LOWERCASE) to ghost paths so
`extdir.Find()` returns `ErrNotFound`. Use for the "unresolvable dir" tests.

### `writeExtTree(t, layout map[string]string) string` — single-file `.ts` store
Writes one `<tag>.ts` per map entry, each with a `/** <desc> */` JSDoc block.
Returns the store root. **This only creates SINGLE-FILE extensions** (Kind="file").
For this subtask, `writeExtTree` covers:
- `TestRunTagSingleResolvesToPath` — `weave example` → `…/extensions/example.ts`
- `TestRunTagMultipleInInputOrder` — multiple tags, input order
- `TestRunTagAtomicity*` — unknown/bad tag → empty stdout, stderr, exit 1
- `TestRunTagAmbiguousListsCandidates` — basename collision
- `TestRunTagUnresolvable` — env neutralized
- `TestRunTagPathIsAbsolute` — output starts with `/` (or drive on Windows)
- `TestRunTagDuplicateArgResolvesTwice`
- `TestRunTagFileOnSingleFileIsNoOp` — `weave -f example` → the file itself
- `TestRunAllPrintsAllSorted`, `TestRunAllEmptyExit0`, `TestRunAllRelative`,
  `TestRunAllFile` (on a single-file-only store)

### `sampleStore(t)` — the canonical 2-ext single-file store
A NEW helper to add (mirrors skilldozer's `sampleStore`): builds a store with
`example.ts` and `writing/reddit-poster.ts` (single-file, nested via the
`writing/` category dir). This is the PRD §7.1 worked-example layout (file kind).
```go
func sampleStore(t *testing.T) string {
    t.Helper()
    return writeExtTree(t, map[string]string{
        "example":             "Reference example extension.",
        "writing/reddit-poster": "Posts to reddit.",
    })
}
```
Note: weave's `writeExtTree` already handles the nested `writing/reddit-poster.ts`
path (it joins root + FromSlash(tag) + ".ts"). The RelTag is `writing/reddit-poster`
(discover strips the `.ts` and slash-normalizes). So `weave reddit-poster` resolves
by BASENAME to `writing/reddit-poster` — the exact PRD §7.2 example.

## NEW fixtures needed for dir/package --file tests

The PRD §6.2 contract requires `--file` to behave differently per Kind:
- file → the file itself (no-op)
- dir → `index.ts`
- package → first `pi.extensions` entry

`writeExtTree` only makes file kinds. For the dir/package `--file` tests we need
helpers that write `index.ts` into a dir, or a `package.json` with `pi.extensions`.
Mirror the patterns in `internal/discover/discover_test.go` (writeFile helper +
manual os.MkdirAll/WriteFile):

```go
// writeDirExt writes a dir-kind extension: <tag>/index.ts with a JSDoc desc.
func writeDirExt(t *testing.T, root, tag, desc string) string {
    t.Helper()
    d := filepath.Join(root, filepath.FromSlash(tag))
    if err := os.MkdirAll(d, 0o755); err != nil { t.Fatalf("mkdir %s: %v", d, err) }
    f := filepath.Join(d, "index.ts")
    if err := os.WriteFile(f, []byte("/** "+desc+" */\nexport default function() {}\n"), 0o644); err != nil {
        t.Fatalf("write %s: %v", f, err)
    }
    return d
}

// writePkgExt writes a package-kind extension: <tag>/package.json with
// pi.extensions → ["./src/index.ts"], plus that src/index.ts file.
func writePkgExt(t *testing.T, root, tag, name string) string {
    t.Helper()
    d := filepath.Join(root, filepath.FromSlash(tag))
    srcDir := filepath.Join(d, "src")
    if err := os.MkdirAll(srcDir, 0o755); err != nil { t.Fatalf("mkdir %s: %v", srcDir, err) }
    entry := filepath.Join(srcDir, "index.ts")
    if err := os.WriteFile(entry, []byte("/** pkg entry. */\nexport default function() {}\n"), 0o644); err != nil {
        t.Fatalf("write %s: %v", entry, err)
    }
    pkg := `{"name":"` + name + `","pi":{"extensions":["./src/index.ts"]}}`
    if err := os.WriteFile(filepath.Join(d, "package.json"), []byte(pkg), 0o644); err != nil {
        t.Fatalf("write package.json: %v", err)
    }
    return d
}
```

These are needed ONLY for:
- `TestRunTagFileOnDirExtPrintsIndexTS` — `weave -f git-checkpoint` → `…/git-checkpoint/index.ts`
- `TestRunTagFileOnPkgExtPrintsPiExtensionsEntry` — `weave -f summarizer` → `…/summarizer/src/index.ts`

(Discovered during research: discover_test.go's `TestClassifyDirPackage` uses the
SAME layout — a dir with `package.json` containing `"pi":{"extensions":["./src/index.ts"]}`
and a real `src/index.ts` file. Reuse that exact shape so classifyDir recognizes it.)

## Env var name (CRITICAL — do not copy skilldozer's)

Weave uses **lowercase** `weave_EXTENSIONS_DIR` (NOT `SKILLDOZER_SKILLS_DIR`).
Confirmed in `internal/extdir/extdir.go:74`: `const envVar = "weave_EXTENSIONS_DIR"`.
All tests drive the store via `t.Setenv("weave_EXTENSIONS_DIR", dir)`.

## Expected output bytes (the §6.1 contract)

- `weave <tag>` → `<abs path>\n` (default = ext.Path, absolute — discover.Index
  calls filepath.Abs on the root).
- `weave -f <tag>` → `<abs EntryFile>\n`.
- `weave --relative <tag>` → `<rel path>\n` (filepath.Rel; uses OS sep — compare
  with `filepath.FromSlash`).
- `weave --all` → one `<abs Path>\n` per line, SORTED by RelTag (Index pre-sorts).
- Empty store + `--all` → empty stdout, exit 0 (PRD §6.1: --all is ALWAYS 0).
- Unknown tag → empty stdout, `<typed err>\n` on stderr, exit 1.

## The `weave init` string (for unresolvable-dir assertions)

`extdir.ErrNotFound.Error()` == `weave is not configured; run ` + "`" + `weave init` + "`"
(confirmed extdir.go:271). Tests assert `strings.Contains(stderr, "weave init")`.

## Test style (match the repo)

- stdlib `testing` only — NO testify.
- `t.Helper()` in every fixture.
- `t.Setenv` (NOT os.Setenv) — auto-restores, and is what every existing test uses.
- `var out, errOut bytes.Buffer; code := run([]string{...}, &out, &errOut)`.
- Plain `==` / `strings.Contains` / `strings.Split(strings.TrimRight(out.String(),"\n"),"\n")`.
- NO `t.Parallel()` on any test using `unsetExtEnv`/`t.Setenv`/`withTerminal` (mutates globals).
- Function naming: `TestRunTag*`, `TestRunAll*` (mirror skilldozer exactly so the
  coverage map is 1:1, modulo the Skill→Extension noun).

## Test inventory (port from skilldozer main_test.go, lines 460–870)

Tag resolution (single-file store via sampleStore):
- TestRunTagSingleResolvesToPath       (line 473)
- TestRunTagMultipleInInputOrder       (492)
- TestRunTagAtomicityUnknownPrintsNothing (514)
- TestRunTagAllFailMultipleErrorLines  (531)
- TestRunTagDuplicateArgResolvesTwice  (549)
- TestRunTagAmbiguousListsCandidates   (565)  — needs a 2-ext-collision store (writeExtTree with writing/reddit + coding/reddit)
- TestRunTagUnresolvable               (589)
- TestRunTagPathIsAbsolute             (606)
- TestRunVersionPrecedenceOverTag      (620)

Modifiers (single-file store):
- TestRunTagFileOnSingleFileIsNoOp     (685, renamed: skilldozer's "PrintsSourceFile" → weave's file-kind is a no-op)
- TestRunTagRelativePrintsRelativePath (704)
- TestRunTagFileRelativeCombine        (719)
- TestRunTagFileAtomicity              (734)

NEW (dir/package --file — weave-specific, no skilldozer analog):
- TestRunTagFileOnDirExtPrintsIndexTS
- TestRunTagFileOnPkgExtPrintsPiExtensionsEntry

--all (single-file store):
- TestRunAllPrintsAllSorted             (751)
- TestRunAllShortFlag                   (776)
- TestRunAllFilePrintsAllEntryFiles     (790, renamed: single-file store → EntryFile == Path)
- TestRunAllRelativePrintsAllRelative   (811)
- TestRunAllEmptyStoreExit0             (834)
- TestRunAllUnresolvable                (847)
- TestRunVersionPrecedenceOverAll       (864)
