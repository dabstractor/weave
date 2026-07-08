# Fix Design — Bugfix 001_de4406db873a

Detailed implementation design for each of the 6 bug fixes. Downstream PRP agents
use this to write precise context_scope contracts. Pair with `bug_cascade_map.md`
(exact line numbers) and `test_patterns.md` (test helpers).

---

## Bug 1 (Major): classifyDir must iterate ALL pi.extensions entries

**File:** `internal/discover/discover.go`, `classifyDir`, case (a) (~line 136).

**Current code (broken):**
```go
entries := toStringSlice(pkg.Pi.Extensions)
if len(entries) > 0 {
    entryFile := filepath.Join(path, entries[0])   // ← ONLY entries[0]
    if fileExists(entryFile) {
        // ... BuildExtension, return package
    }
    // fall through if entries[0] missing
}
```

**Fixed code:**
```go
entries := toStringSlice(pkg.Pi.Extensions)
if len(entries) > 0 {
    var entryFile string
    for _, e := range entries {
        if f := filepath.Join(path, e); fileExists(f) {
            entryFile = f
            break
        }
    }
    if entryFile != "" {
        relTag := relTagForDir(root, path)
        jsdoc := ExtractJSDoc(entryFile)
        ext := BuildExtension(path, entryFile, relTag, "package", pkg, hasPkg, jsdoc)
        return &ext, true, false
    }
    // no existing entry → fall through to b/c/d/e
}
```

**What this fixes:**
- `pi.extensions: ["./src/MISSING.ts", "./src/real.ts"]` now correctly selects
  `./src/real.ts` as the entry file. The package is classified, not fragmented.
- The init-vs-discovery inconsistency is resolved: both `extdir.hasPiExtensions`
  (used by `weave init`) and `discover.classifyDir` now agree on what qualifies
  as a package.
- The cascade spurious-duplicate-relTag ERROR from `real.js` + `real.ts`
  collapsing to the same tag no longer occurs (both are internal to the
  pruned package, not separate entries).

**Doc comment update (Mode A):** The classifyDir doc comment currently says
case (a) checks "entries[0]". Update it to say "the first EXISTING entry
(iterated in order)". The comment block describing case (a) (~line 96) and
the inline comment at the fall-through should both reflect iteration.

**Regression tests to add:**
1. `TestClassifyDirPackageMultiEntryFirstMissing` — `["./src/MISSING.ts",
   "./src/real.ts"]`, real.ts exists → classified as package, EntryFile points
   at real.ts, Kind="package", shouldDescend=false.
2. `TestClassifyDirPackageMultiEntryAllMissing` — `["./a.ts", "./b.ts"]`,
   neither exists → NOT a package, falls through (isExt=false, descend=true).
   (Extends the existing `TestClassifyDirPackageNonExistentEntry` which only
   tests single-entry-missing.)
3. End-to-end via `Index` or `walkClassified`: a store with a multi-entry
   package resolves correctly (`weave <tag>` and `weave -f <tag>` work,
   `--list` shows the package, not stray files).

---

## Bug 2 (Minor): ExtractJSDoc degenerate empty /**/ block

**File:** `internal/discover/jsdoc.go`, `ExtractJSDoc`, single-line branch
(~line 135, `case i == startIdx && i == closeIdx`).

**Current code (broken):**
```go
case i == startIdx && i == closeIdx:
    rest := strings.TrimPrefix(line, "/**")   // for "/**/" → rest = "/"
    if c := strings.Index(rest, "*/"); c >= 0 { // Index("/", "*/") = -1
        rest = rest[:c]
    }
    line = rest   // line = "/" ← WRONG, should be ""
```

**Root cause:** In `/**/`, the opener `/**` (positions 0–2) and the closer
`*/` (positions 2–3) **overlap** at position 2. `TrimPrefix(line, "/**")`
consumes positions 0–2, destroying the closer's `*`. The remaining `/` has no
`*/` to find.

**Fixed code:**
```go
case i == startIdx && i == closeIdx:
    // Single-line block: find "*/" on the ORIGINAL line (before stripping),
    // cut there, THEN strip the "/**" opener. This handles the degenerate
    // case "/**/" where opener and closer overlap at position 2.
    if c := strings.Index(line, "*/"); c >= 0 {
        line = line[:c+2] // keep through the closer
    }
    rest := strings.TrimPrefix(line, "/**")
    rest = strings.TrimSuffix(rest, "*/")
    line = rest
```

**Trace for `/**/`:**
- `Index("/**/", "*/")` = 2 → `line[:4]` = `"/**/"`.
- `TrimPrefix("/**/", "/**")` = `"/"`.
- `TrimSuffix("/", "*/")` = `"/"` (no suffix to trim) ← STILL WRONG.

Wait — that doesn't work. Let me reconsider.

Actually the issue is that after `TrimPrefix`, the closer `*/` has already been
partially consumed. The correct approach: cut the content between `/**` and the
last `*/` directly on the original line:

```go
case i == startIdx && i == closeIdx:
    // Strip "/**" opener, then find and cut at "*/" in what remains.
    // For "/**/" the opener overlaps the closer: after stripping "/**",
    // only "/" remains, which has no "*/" → content is empty.
    rest := strings.TrimPrefix(line, "/**")
    // If the opener consumed the closer's "*", check for a trailing "/" that
    // is the remnant of "*/" after the "*" was consumed by "/**".
    rest = strings.TrimSuffix(rest, "/")
    if c := strings.Index(rest, "*/"); c >= 0 {
        rest = rest[:c]
    }
    line = rest
```

**Trace for `/**/`:**
- `TrimPrefix("/**/", "/**")` = `"/"`.
- `TrimSuffix("/", "/")` = `""`.
- `Index("", "*/")` = -1 → no cut.
- `line = ""`. ✓

**Trace for `/** desc */`:**
- `TrimPrefix("/** desc */", "/**")` = `" desc */"`.
- `TrimSuffix(" desc */", "/")` = `" desc *"` ← WRONG, we trimmed the `/`.

That approach breaks the normal case. Let me think differently.

**Better approach:** Don't use TrimPrefix. Instead, extract the substring
between `len("/**")` and the position of `*/` on the original line:

```go
case i == startIdx && i == closeIdx:
    // Find "*/" on the original line. The content is between the opener
    // "/**" (3 chars) and the closer "*/" at position c.
    c := strings.Index(line, "*/")
    if c < 0 {
        line = "" // defensive; closeIdx guarantees "*/" exists
    } else if c <= 3 {
        // "*/" starts at or before the opener ends → degenerate empty block
        // (e.g. "/**/" has "*/" at position 2, opener ends at 3).
        line = ""
    } else {
        line = line[3:c] // content between "/**" and "*/"
    }
```

**Trace for `/**/`:** c=2, 2 <= 3 → `""`. ✓
**Trace for `/** desc */`:** c=9, 9 > 3 → `line[3:9]` = `" desc "`. Then `stripStarPrefix(" desc ")` → TrimLeft → `"desc "`... hmm, that's `"desc "` with trailing space. But then `TrimSpace` in the collection loop trims it. Wait, let me re-read the collection loop:

```go
if cleaned := strings.TrimSpace(stripStarPrefix(line)); cleaned != "" {
    parts = append(parts, cleaned)
}
```

So `stripStarPrefix(" desc ")` → TrimLeft `" desc "` → `"desc "` (no leading `*`), then the optional space after `*` check doesn't apply. Then `TrimSpace("desc ")` → `"desc"`. ✓

**Trace for `/**desc*/`:** c=7, 7 > 3 → `line[3:7]` = `"desc"`. ✓
**Trace for `/** */`:** c=4, 4 > 3 → `line[3:4]` = `" "`. TrimSpace → `""` → dropped. ✓ (empty block with space)

This approach is clean and correct. The key insight: for a single-line block,
find `*/` on the ORIGINAL line and extract `line[3:c]` (the content strictly
between opener `/**` and closer `*/`). When `c <= 3` the opener and closer
overlap → empty content.

**Regression tests to add:**
1. `{"empty-block-degenerate", "/**/\n", ""}` — the core bug case.
2. Verify existing cases still pass: `single-line`, `single-line-no-space`.

---

## Bug 3 (Minor): Root-level index.ts collapses the store

**File:** `internal/discover/index.go`, `Index`, WalkDir callback (~line 61).

**Current code (broken):**
```go
if d.IsDir() {
    ext, isExt, descend := classifyDir(root, path)  // ← runs on root too
    if isExt {
        result = append(result, *ext)
    }
    if !descend {
        return filepath.SkipDir  // ← SkipDir at root prunes EVERYTHING
    }
    return nil
}
```

**Fixed code:**
```go
if d.IsDir() {
    // The walk root is the store container, never an extension itself.
    // Without this guard, an index.ts at the root would classify the root
    // as a dir extension (relTag "."), then SkipDir would prune the ENTIRE
    // store — every real extension becomes invisible.
    if path == root {
        return nil // always descend into the root
    }
    ext, isExt, descend := classifyDir(root, path)
    if isExt {
        result = append(result, *ext)
    }
    if !descend {
        return filepath.SkipDir
    }
    return nil
}
```

**Regression tests to add:**
1. `TestIndexRootIndexTSDoesNotCollapse` — root has `index.ts` + `sub/x.ts`.
   Assert: 1 entry (`sub/x`), NOT a `.` entry. Both `--list` and tag resolution
   work.

---

## Bug 4 (Minor): Symlinked EXTENSIONS_DIR yields empty catalog

**File:** `internal/discover/index.go`, `Index`, after `filepath.Abs` (~line 45).

**Current code (broken):**
```go
root, err := filepath.Abs(extensionsDir)  // symlink path preserved
if err != nil { return nil, err }
info, err := os.Stat(root)  // follows symlink → passes (sees real dir)
if !info.IsDir() { ... }
// WalkDir(root) ← Lstats root → ModeSymlink → walks nothing
```

**Fixed code:**
```go
root, err := filepath.Abs(extensionsDir)
if err != nil { return nil, err }
// Resolve symlinks on the walk root. filepath.WalkDir Lstats the root and
// will not descend a symlinked directory (stdlib default). os.Stat above
// follows the symlink and passes the IsDir guard, but WalkDir then sees
// ModeSymlink and walks nothing — producing an empty catalog with no error.
// EvalSymlinks resolves the chain so WalkDir sees a real directory.
// This covers ALL extdir.Find() callers (env, config, sibling, walk-up)
// uniformly. The --path output still shows the original (possibly symlinked)
// path because --path uses extdir.Find() directly, not discover.Index().
if resolved, err := filepath.EvalSymlinks(root); err == nil {
    root = resolved
}
info, err := os.Stat(root)
// ... rest unchanged
```

**Note:** If `EvalSymlinks` fails (e.g. broken symlink chain), we fall through
to the existing `os.Stat` which will also fail, and the error propagates
normally. This is a no-op for non-symlinked paths (EvalSymlinks returns the
same path).

**Regression tests to add:**
1. `TestIndexSymlinkedRoot` — create a real dir with extensions, symlink it,
   call `Index` on the symlink path. Assert entries are found. Guard with
   `t.Skipf` for platforms without symlink support.

---

## Bug 5 (Minor): check surfaces ERROR for all-missing pi.extensions

**File:** `internal/check/check.go`, `appendEmptyFolderFindings` (~line 287).

**Current code (broken):** When a top-level child dir has zero `discover.Index`
entries, it unconditionally emits `WARN "empty category folder: <name>"`. This
happens even when the dir has a `package.json` with non-empty `pi.extensions`
pointing at all-missing files — the real problem (broken pi.extensions refs) is
hidden behind an unrelated WARN at WARN severity (exit 0).

**Fixed code:** Before the WARN, check if the child is a package dir with
all-missing pi.extensions:

```go
func appendEmptyFolderFindings(rep *Report, dir string) {
    entries, err := os.ReadDir(dir)
    if err != nil { return }
    for _, e := range entries {
        if !e.IsDir() { continue }
        child := filepath.Join(dir, e.Name())
        sub, ierr := discover.Index(child)
        if ierr != nil { continue }
        if len(sub) == 0 {
            // Before emitting an "empty category folder" WARN, check whether
            // this is a package dir whose pi.extensions are ALL missing — a
            // §9 ERROR, not a benign empty folder (PRD Issue 5).
            if missingEntries := allMissingPiExtensions(child); len(missingEntries) > 0 {
                rep.ByExt = append(rep.ByExt, ExtensionReport{
                    Extension: discover.Extension{RelTag: e.Name()},
                    Findings: []Finding{{
                        Level:   LevelError,
                        Message: "pi.extensions entry does not exist: " + strings.Join(missingEntries, ", "),
                    }},
                })
                continue // reported as ERROR, skip the WARN
            }
            rep.ByExt = append(rep.ByExt, ExtensionReport{
                Extension: discover.Extension{RelTag: e.Name()},
                Findings: []Finding{{
                    Level:   LevelWarn,
                    Message: "empty category folder: " + e.Name(),
                }},
            })
        }
    }
}
```

**Helper function:**
```go
// allMissingPiExtensions checks whether dir has a package.json with a
// non-empty pi.extensions array where NONE of the declared entries exist on
// disk. Returns the list of missing entry paths (relative, as declared in
// pi.extensions) if all are missing; returns nil otherwise (including: no
// package.json, parse error, empty pi.extensions, or at least one entry exists).
func allMissingPiExtensions(dir string) []string {
    pkg, hasPkg, perr := discover.ParsePackageJSON(dir)
    if !hasPkg || perr != nil {
        return nil // no package.json or unparseable → not this check's concern
    }
    // pkg.Pi.Extensions is lenientAnySlice ([]any). Range over it and collect
    // string entries that don't exist on disk.
    var missing []string
    hasAny := false
    for _, e := range pkg.Pi.Extensions {
        s, ok := e.(string)
        if !ok {
            continue // lenient: non-string entries are dropped
        }
        hasAny = true
        if !fileExists(filepath.Join(dir, s)) {
            missing = append(missing, s)
        } else {
            return nil // at least one exists → not all-missing
        }
    }
    if !hasAny {
        return nil // pi.extensions was empty/absent
    }
    return missing // all declared entries are missing
}
```

Note: `fileExists` needs to be available in the check package. It currently
exists in `discover` and `extdir` but not in `check`. The check package already
imports `os` and `filepath`, so add a local `fileExists` helper (the same
2-line `os.Stat` pattern). Alternatively, use `os.Stat` inline.

**Interaction with Bug 1:** After Bug 1's fix, a package.json with at least one
existing entry is classified as a package by `classifyDir` and never reaches
`appendEmptyFolderFindings`. Only the ALL-missing case falls through. This check
catches exactly that case.

**Regression tests to add:**
1. `TestCheckAllMissingPiExtensionsIsError` — a top-level dir with
   package.json `{"pi":{"extensions":["./missing.ts"]}}`, no files → ERROR
   "pi.extensions entry does not exist", exit 1 (HasErrors=true).
2. Verify the existing `TestCheckEmptyCategoryFolder` still passes (a dir with
   NO package.json still gets the WARN, not the ERROR).

---

## Bug 6 (Minor): Skip node_modules, .git, and hidden entries during walk

**File:** `internal/discover/index.go`, `Index`, WalkDir callback.

**Current behavior:** WalkDir descends into every directory (including
`node_modules/`, `.git/`) and discovers every `.ts`/`.js` file (including
hidden files like `.secret.ts`). This pollutes the catalog with non-extension
entries.

**Fixed code:** Add skip logic to the WalkDir callback:

```go
// skipDirs are well-known directories that should never be treated as
// extension containers or category folders. They are common in extension
// stores (npm install at the root creates node_modules) but their contents
// are dependencies, not user-authored extensions.
var skipDirs = map[string]bool{
    "node_modules": true,
    ".git":         true,
}

// shouldSkipEntry reports whether a WalkDir entry should be skipped:
//   - a directory in skipDirs (returns true; caller returns SkipDir)
//   - a hidden file or directory (base name starts with ".")
func shouldSkipDir(name string) bool {
    return skipDirs[name] || strings.HasPrefix(name, ".")
}
```

In the WalkDir callback, before classification:
```go
if d.IsDir() {
    if path == root {
        return nil // Bug 3: root is never classified
    }
    name := d.Name()
    if shouldSkipDir(name) {
        return filepath.SkipDir // prune node_modules, .git, hidden dirs
    }
    // ... existing classifyDir logic
}
```

And for files:
```go
// File entry:
if strings.HasPrefix(d.Name(), ".") {
    return nil // skip hidden files (.secret.ts, etc.)
}
ext, ok := classifyFile(root, path)
// ...
```

**Note:** `strings` is already imported in `index.go`? No — index.go currently
imports `errors`, `io/fs`, `os`, `path/filepath`, `sort`. Need to add `strings`.

**Regression tests to add:**
1. `TestIndexSkipsNodeModules` — root has `myext.ts` + `node_modules/somepkg/index.js`.
   Assert: only `myext` discovered, NOT `node_modules/somepkg`.
2. `TestIndexSkipsHiddenFiles` — root has `myext.ts` + `.secret.ts`.
   Assert: only `myext` discovered.
3. `TestIndexSkipsGitDir` — root has `myext.ts` + `.git/` with a `.ts` inside.
   Assert: only `myext` discovered.

---

## Documentation plan

### Mode A (with-work doc updates)
Each subtask updates the doc comment / inline comment in the file it modifies:
- Bug 1: classifyDir doc comment (case a description)
- Bug 2: ExtractJSDoc single-line branch comment
- Bug 3: Index doc comment (root-skip note)
- Bug 4: Index doc comment (symlink resolution note)
- Bug 5: appendEmptyFolderFindings doc comment
- Bug 6: Index doc comment (skip rules)

### Mode B (changeset-level docs sweep)
Final task updates `README.md`:
- The "How extensions are organized" section should note that `node_modules/`,
  `.git/`, and hidden entries (`.foo.ts`) are skipped during discovery (Bug 6).
- Verify the package description ("names at least one existing entry") is
  still accurate post-Bug-1 (it already is — the README was right, the code
  was wrong).
- No changes needed for Bugs 2, 3, 4, 5 (internal fixes with no user-facing
  doc surface beyond what Mode A covers).
