# Research: Index() vs walkClassified — what T3 adds over T2's test scaffold

## The critical discovery

P1.M2.T2.S1 (parallel, being implemented) creates TWO files:
- `internal/discover/discover.go` — `classifyFile`, `classifyDir`, `isExtensionFile`, `fileExists`, `relTagForDir`
- `internal/discover/discover_test.go` — unit tests + `walkClassified` helper + `TestWorkedExample`

**`walkClassified` (discover_test.go:24-46) is ALREADY a working WalkDir skeleton.**
It drives a real `filepath.WalkDir` over the classify functions and applies the
SkipDir rule correctly. It is the canonical reference for `Index()`.

```go
func walkClassified(root string) []Extension {
    var result []Extension
    _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() {
            ext, isExt, descend := classifyDir(root, path)
            if isExt { result = append(result, *ext) }
            if !descend { return filepath.SkipDir }
            return nil
        }
        ext, ok := classifyFile(root, path)
        if ok { result = append(result, *ext) }
        return nil
    })
    sort.Slice(result, func(i, j int) bool { return result[i].RelTag < result[j].RelTag })
    return result
}
```

## What Index() adds on top of walkClassified (exactly 4 things)

1. **Stat-guard the root BEFORE WalkDir.** skilldozer pattern (verified bug: without
   it, a missing root is swallowed by `if err != nil { return nil }` → returns
   `(nil, nil)`, masking a real error). See skilldozer index.go:64-72 and
   research/walkdir_skipdir_semantics.md point — root is visited FIRST and its
   lstat error feeds the callback; the per-entry error-skip would hide it.
2. **`filepath.Abs(root)` first** — make absolute before stat+walk, so every
   `Extension.Path`/`EntryFile` is absolute (PRD §6.1 absolute-output contract;
   PRD §13 acceptance gate `case "$(./weave example)" in /*)`).
3. **Return `([]Extension, error)`** — walkClassified returns only `[]Extension`
   (discards the WalkDir error via `_ =`). Index surfaces root errors + walk errors.
4. **Empty store → nil slice, nil error.** walkClassified returns whatever the walk
   produced (which IS nil for an empty tree, so this is already the case — but it's
   Index's explicit contract per §3d).

## Why Index() does NOT duplicate walkClassified's logic in a different shape

`walkClassified`'s WalkDir callback is the literal body of `Index()`'s WalkDir
callback. The ONLY differences are the stat-guard/Abs preamble and the error
return. There is no reason to write a second, different callback — `walkClassified`
IS the proven reference implementation.

## The root-entry classification question (resolved)

WalkDir visits the ROOT FIRST (`path == root`, `d.IsDir() == true`).
- `classifyDir(root, root)`: root is the extensions container, NOT an extension itself
  (it has no index.ts and no pi.extensions → `packageJSON{}`). It returns
  `(nil, false, true)` — plain dir, descend.
- → Index's callback returns `nil` → WalkDir descends into root's children.
- We do NOT need an explicit `if path == root { return nil }` guard: classifyDir
  already returns descend==true for it. BUT skilldozer's index.go does NOT guard
  the root either (it filters on `d.Name() != "SKILL.md"`, and root's name is the
  dir name, not "SKILL.md", so root is skipped by the file-filter).
- weave's Index walks by `d.IsDir()`, so the root IS processed by classifyDir.
  This is CORRECT and harmless (classifyDir(root) returns plain/descend). No
  special-casing needed. Confirmed by walkClassified already handling root this way
  (TestWorkedExample calls walkClassified(root) and root is the temp dir — it works).

## SkipDir-on-root edge case (does NOT occur in weave)

If classifyDir(root) were to return `shouldDescend==false` (it won't — root has no
extension markers), returning SkipDir for root would skip ALL of root's contents and
abort the walk (root has no parent siblings). This is a non-issue because root is the
extensions container, never an extension. No guard required.

## Test strategy for index_test.go

`discover_test.go` (T2) already has `walkClassified` + `TestWorkedExample` + 16
classify unit tests. These PROVE the classify-then-descend rule through a real walk.

For T3, `index_test.go` should focus on what Index() adds that walkClassified lacks:

1. **Stat-guard error paths** (walkClassified silently returns nil):
   - `TestIndexMissingRoot` — non-existent root → error (not nil,nil)
   - `TestIndexRootIsFile` — root is a regular file → error ("not a directory")
2. **Absolute-path contract** (walkClassified doesn't Abs):
   - `TestIndexRelativeInputStillAbsolute` — relative input → absolute Extension.Path
3. **Empty-dir contract** (already true via walkClassified, but pin it for Index):
   - `TestIndexEmptyDir` — empty existing dir → (nil or len-0, nil error)
4. **End-to-end via the public Index() function** (retarget TestWorkedExample's
   assertion style onto Index, OR add a thin TestIndexWorkedExample that calls
   `Index(root)` and asserts the same 5 entries — proving Index == walkClassified + guard).
5. **Sort verification** — Index returns sorted-by-RelTag (lexical); pin a
   non-lexical-input case.

### Retarget vs keep walkClassified — DECISION

Keep `walkClassified` in `discover_test.go` (it is the unit-test seam for the
classify functions and is referenced by T2's TestWorkedExample). T3's `index_test.go`
calls the PUBLIC `Index()` directly (black-box would work, but white-box
`package discover` matches the sibling test files and shares helpers). T3's
TestIndexWorkedExample DUPLICATES TestWorkedExample's assertions but via Index —
this is intentional: it proves Index (the real public function) produces the same
5-entry result as walkClassified (the scaffold), so the scaffold can be removed
later without losing coverage. (Or: refactor TestWorkedExample to call a shared
assertion helper that both walkClassified and Index feed — optional, lower priority.)

## Sources
- /home/dustin/projects/skilldozer/internal/discover/index.go (the port pattern)
- /home/dustin/projects/skilldozer/internal/discover/index_test.go (test patterns)
- /home/dustin/projects/weave/internal/discover/discover_test.go:24-46 (walkClassified)
- /home/dustin/projects/weave/plan/001_19b4b465824d/P1M2T2S1/research/walkdir_skipdir_semantics.md
- /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md §3d
- https://pkg.go.dev/path/filepath#WalkDir (root visited first)
- https://pkg.go.dev/io/fs#WalkDirFunc (SkipDir semantics)
