# Research Notes — P1.M1.T1.S1 (iterate pi.extensions entries, pick first existing)

## What this subtask is
Fix Bug 1 (Issue 1, Major) from the bugfix PRD: `discover.classifyDir` case (a) only
stat-checks `entries[0]`. When `pi.extensions` is `["./missing.ts","./src/real.ts"]` and
only `real.ts` exists, the dir is misclassified as a plain category folder, descended
into, and `real.ts` is discovered as a stray single-file extension — fragmenting the
package, losing all package.json metadata, and (with a sibling sharing a stripped stem)
cascading into a spurious duplicate-relTag ERROR from `check`.

Fix: iterate ALL entries, select the FIRST existing one (mirrors `extdir.hasPiExtensions`
and PRD §7.1 "first existing entry"). Fall through only if NO entry exists.

## Verified codebase facts (commit 8f3819e, read live 2026-07-08)

### The buggy code — internal/discover/discover.go, classifyDir case (a)
Exact current code (the `if hasPkg && perr == nil {` block, ~lines 132-149):

```go
pkg, hasPkg, perr := ParsePackageJSON(path) // S1 — lenient parse

// Case (a): package extension — package.json with pi.extensions → existing entry.
if hasPkg && perr == nil {
    entries := toStringSlice(pkg.Pi.Extensions) // []any → []string, drops non-strings
    if len(entries) > 0 {
        entryFile := filepath.Join(path, entries[0])   // ← BUG: only entries[0]
        if fileExists(entryFile) {
            relTag := relTagForDir(root, path)
            jsdoc := ExtractJSDoc(entryFile)
            e := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
            return &e, true, false
        }
        // pi.extensions names a non-existent file → NOT a package; fall through.
    }
}
```

The fix is a LOCALIZED change to the body of `if len(entries) > 0 { ... }`. Everything
outside that block (ParsePackageJSON call, cases b/c/d/e) is UNTOUCHED.

### The correct iteration to mirror — internal/extdir/extdir.go:381-400
```go
func hasPiExtensions(dir string) bool {
    ...
    for _, e := range pkg.Pi.Extensions {
        if fileExists(filepath.Join(dir, e)) {
            return true
        }
    }
    return false
}
```
This is the authoritative pattern. `discover.classifyDir` must do the same loop but
capture the first existing path (not just a bool). The contract item 3 gives the exact
loop shape.

### Signatures (verified, do NOT change)

- `func toStringSlice(v any) []string` — extension.go:88. Converts `pkg.Pi.Extensions`
  (a `lenientAnySlice`) to `[]string`, dropping non-string elements.
- `func BuildExtension(path, entryFile, relTag, kind string, pkg PackageJSON, hasPkg bool, jsdocDesc string) Extension` — extension.go:289.
  Args: (dir path, entry file, relTag, kind, pkg, hasPkg, jsdoc desc).
- `type Extension struct` — extension.go:156. Fields: `Path, EntryFile, RelTag, Kind,
  Name, Description, Keywords, Category, Aliases, HasPackageJSON`.
- `func fileExists(path string) bool` — discover.go:21 (and a copy in extdir.go).
  `os.Stat`, follows symlinks, `!info.IsDir()`.
- `func relTagForDir(root, dir string) string` — discover.go:35. `filepath.Rel` +
  `ToSlash`, NO .ts/.js strip.
- `func ExtractJSDoc(path string) string` — jsdoc.go (S2).
- `classifyDir` returns `(ext *Extension, isExtension, shouldDescend bool)`.

### Why `pkg.Pi.Extensions` not `[]string` directly
`PackageJSON.Pi.Extensions` is a `lenientAnySlice` (custom json.Unmarshaler) so a
non-array JSON value coerces to nil instead of hard-erroring the whole parse (PRD §7.3
leniency). `toStringSlice` then normalizes to `[]string`. So the loop iterates the
OUTPUT of `toStringSlice(pkg.Pi.Extensions)`, which is already `[]string`. This is
exactly what the buggy code already does on line `entries := ...`; only the check after
changes.

### PRD §7.1 exact wording (verified in PRD.md)
- Case definition: "a `package.json` with a `pi.extensions` array naming **≥1 existing
  entry**. The entry path is the directory."
- entryFile: "the **first existing** `pi.extensions` entry (package). **`--file` output.**"
So the fix is a direct implementation of the spec; the doc comment already says "first
existing pi.extensions entry" — the CODE was the part that didn't match.

## Test landscape (verified live)

### Test files & helpers (internal/discover/*_test.go, white-box `package discover`)
- `writeFile(t, dir, name, content string) string` — jsdoc_test.go:14. Writes file,
  MkdirAll(dir), returns full path.
- `writePackageJSON(t, dir, content string)` — extension_test.go:14. Writes
  dir/package.json, MkdirAll(dir).
- `walkClassified(root string) []Extension` — discover_test.go:24. Mini-Index WalkDir
  skeleton (drives classifyFile/classifyDir + SkipDir rule). Sorted by RelTag.
- `relTags(exts []Extension) []string` — discover_test.go:499. Sorted relTags slice.

### The existing test I'm extending — discover_test.go:212
`TestClassifyDirPackageNonExistentEntry`: single-entry `["./missing.ts"]`, no file →
asserts `isExt=false, ext=nil, descend=true`. This is the ALL-MISSING SINGLE-entry case.
The new tests extend it to (a) multi-entry FIRST-missing and (b) multi-entry ALL-missing.

### Assertion idiom (from existing TestClassifyDirPackage)
```go
ext, isExt, descend := classifyDir(root, dir)
if !isExt { t.Fatal("isExt=false; want true ...") }
if descend { t.Error("descend=true; want false ...") }
if ext == nil { t.Fatal("ext=nil; want non-nil") }
if ext.Kind != "package" { t.Errorf("Kind=%q; want package", ext.Kind) }
if ext.EntryFile != entryFile { t.Errorf("EntryFile=%q; want %q", ...) }
if ext.RelTag != "summarizer" { t.Errorf("RelTag=%q; want summarizer", ext.RelTag) }
```
For the multi-entry test, `EntryFile` should be asserted with `strings.HasSuffix` (the
abs path varies per t.TempDir) — exactly as `TestClassifyDirIndexTS` does for
`"git-checkpoint/index.ts"`.

### End-to-end via walkClassified (the regression-of-the-regression test)
The whole point of the bug: the package must resolve as ONE entry (relTag = dir name),
NOT fragment into stray single-file entries. So the e2e test uses `walkClassified(root)`
and asserts:
- exactly one extension has the package's relTag (the dir name), and
- NO extension has a relTag derived from the bare entry filename (e.g. "mypkg/src/real").
Use `relTags(...)` to compare the full set. This is the test that would have caught the
original bug.

## Validation commands (verified working on this machine)
- `go vet ./internal/discover/` → clean (exit 0) currently.
- `go test ./internal/discover/ -run 'TestClassifyDirPackage' -count=1` → ok (exit 0).
- `go build ./...` → exit 0.
- Full: `go test ./internal/discover/ -count=1` → ok.

No external services, no mocks, no DB. Pure stdlib (`os`, `path/filepath`, `testing`).
Tests use `t.TempDir()` (auto-cleaned, absolute).

## Scope boundaries (what NOT to touch)
- Do NOT modify cases (b), (c), (d), (e) of classifyDir.
- Do NOT modify `classifyFile`, `Index` (index.go), `relTagForDir`, `fileExists`,
  `BuildExtension`, `toStringSlice`, or `ExtractJSDoc`. The fix is ONE block.
- Do NOT modify `extdir.hasPiExtensions` (it's already correct; it's the reference).
- Do NOT fix the `check` WARN-vs-ERROR issue here — that is Bug 3 / P1.M3.T2.S1, which
  DEPENDS on this fix (see bug_cascade_map.md dependency graph).
- Do NOT fix the root-index.ts / symlink / node_modules bugs — separate tasks (P1.M2).
- The doc-comment updates (contract item 5) are INLINE in discover.go, riding with the
  code change. No separate docs file.

## Reproduction (from the bug PRD, to confirm the fix end-to-end after implementing)
```bash
go build -o /tmp/weave .
rm -rf /tmp/bug1 && mkdir -p /tmp/bug1/mypkg/src
printf '{"name":"mypkg","description":"My package","pi":{"extensions":["./src/MISSING.ts","./src/real.ts"]}}\n' > /tmp/bug1/mypkg/package.json
printf '/** real entry */\nexport default function(){}\n' > /tmp/bug1/mypkg/src/real.ts
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave mypkg      # EXPECT after fix: prints /tmp/bug1/mypkg (rc=0)
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave -f mypkg   # EXPECT after fix: /tmp/bug1/mypkg/src/real.ts
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave --list     # EXPECT after fix: shows package "mypkg"
```
This is a post-implementation manual confirmation, NOT a unit test (unit tests use
classifyDir/walkClassified directly for determinism and speed).
