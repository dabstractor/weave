# Post-fix discovery behavior — the ground truth

This is the authoritative list of what the code NOW does (post-bugfix), so the
README update can be verified against reality. All facts confirmed by reading
the current source.

## `discover.Index` walk behavior (internal/discover/index.go)

### Walk root
1. `filepath.Abs(extensionsDir)` — make absolute.
2. **`filepath.EvalSymlinks(root)`** — resolve symlinks on the root (Issue 4).
   If it fails (broken chain), fall through to `os.Stat` which also fails and
   propagates the error. No-op for non-symlinked paths.
3. `os.Stat(root)` — must exist and be a directory, else error.

### WalkDir callback, per entry
**DIR branch (`d.IsDir()`):**
1. **Root guard:** `if path == root { return nil }` (Issue 3) — the root is
   the store container, never an extension. Without this, an `index.ts` at the
   root would collapse the whole store into one entry tagged `.`.
2. **Skip guard:** `if skipDirs[name] || strings.HasPrefix(name, ".") { return filepath.SkipDir }`
   (Issue 6) — prune `node_modules/`, `.git/`, and hidden dirs.
3. `classifyDir(root, path)` — if recognized as an extension, emit + `SkipDir`
   (don't descend into a dir/package extension's internals). If plain category
   dir, `return nil` (descend).

**FILE branch:**
1. **Hidden-file guard:** `if strings.HasPrefix(d.Name(), ".") { return nil }`
   (Issue 6) — skip hidden files. Uses `return nil` (NOT `SkipDir`): SkipDir on
   a file prunes remaining siblings, and `.` sorts first, so real extensions
   would vanish.
2. `classifyFile(root, path)` — if a `*.ts`/`.js` (not `index.*`), emit as a
   single-file extension.

### `skipDirs` set (package-level var)
```go
var skipDirs = map[string]bool{
    "node_modules": true,
    ".git":         true,
}
```

### Sort
`sort.Slice` by `RelTag` — output is always sorted by canonical tag.

## `classifyDir` (internal/discover/discover.go) — Issue 1 fix

For the package case (case a), it now iterates ALL `pi.extensions` entries:
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
        // ... BuildExtension with the first EXISTING entry
        return &ext, true, false
    }
    // all missing → fall through (not a package)
}
```
So: `pi.extensions: ["./missing.ts", "./real.ts"]` now selects `./real.ts`.
The README's "names at least one existing entry" was always correct.

## `ExtractJSDoc` (internal/discover/jsdoc.go) — Issue 2 fix

The single-line branch now handles the degenerate `/**/` case (opener and
closer overlap at position 2) by extracting `line[3:c]` where `c` is the
position of `*/` on the original line; `c <= 3` → empty content. Normal
`/** desc */` blocks are unaffected. No README surface.

## `check` (internal/check/check.go) — Issue 5 fix (P1.M3.T2.S1, parallel/Ready)

`appendEmptyFolderFindings` now emits an ERROR (not WARN) when a top-level dir
has a `package.json` whose `pi.extensions` are ALL missing. The README's
`weave check` example shows a clean store and does not enumerate ERROR cases,
so no change is needed.

## The 6 issues and their README impact (final matrix)

| Issue | Fix status | User-facing README change? |
|-------|-----------|----------------------------|
| 1 (multi-entry pi.extensions) | COMPLETE | NO — README "names at least one existing entry" was always right |
| 2 (JSDoc `/**/`) | COMPLETE | NO — internal; JSDoc example unaffected |
| 3 (root index.ts) | COMPLETE | NO — internal; no README claim about root index.ts |
| 4 (symlinked EXTENSIONS_DIR) | COMPLETE | OPTIONAL — env-var rule now accurate; a "symlinks followed" note helps |
| 5 (check all-missing ERROR) | Ready (parallel) | NO — internal; check example unaffected |
| 6 (skip node_modules/.git/hidden) | COMPLETE | **YES (PRIMARY)** — "How extensions are organized" needs the skip note |

## Sources (all read during research)
- `internal/discover/index.go` — the full Index() walk (read in full).
- `internal/discover/discover.go` — classifyDir (Bug 1 fix, via fix_design.md).
- `plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md` — §Documentation plan (Mode B).
- `plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T2S1/research/skipdir_hidden_semantics.md` — the exact skip semantics.
- `README.md` — current text (read in full).
