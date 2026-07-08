# Research: Go `filepath.WalkDir` + `filepath.SkipDir` for classify-then-descend

Verified against Go stdlib docs (pkg.go.dev/path/filepath, pkg.go.dev/io/fs).
These behaviors are stable since `WalkDir` shipped in Go 1.16.

## Key semantics (load-bearing for T2/T3)

1. **SkipDir on a DIRECTORY skips its entire subtree, then continues with the next sibling.**
   `fs.WalkDirFunc` doc: *"If the function returns `SkipDir` when invoked on a
   directory, WalkDir skips the directory's contents entirely."* The directory itself
   was already visited (fn was called for it); its whole subtree is then skipped and the
   walk proceeds to the next entry in the parent. **This is precisely the
   "classify-then-descend" primitive** weave needs for §7.1.

2. **Root is passed to fn FIRST, before children; you can SkipDir the root.**
   WalkDir *"calls fn for each file or directory in the tree, including root."*

3. **SkipDir on a FILE skips the remaining entries in the current directory.**
   *"If the function returns `SkipDir` when invoked on a file, WalkDir skips the
   remaining files in the current directory."* Not a subtree (files have none).
   — **GOTCHA**: returning SkipDir on a recognized FILE extension entry would wrongly
   skip sibling entries in the same category dir. T3's WalkDir callback must return
   `nil` for file entries (emit the entry, continue), NOT SkipDir.

4. **WalkDir does NOT follow symlinks.** *"WalkDir does not follow symbolic links."*
   Symlinks report `IsDir()==false` even when the target is a dir → never auto-recursed.
   Matches pi's own loader behavior (no symlink-follow requirement in PRD §7.1).

5. **`fs.DirEntry.IsDir()` is reliable** — comes from ReadDir, no extra stat needed.
   Symlinks report `IsDir()==false`.

6. **Traversal order is lexical by filename within each directory.** Deterministic.
   NOTE: this is lexical BY FILENAME, not by relTag. The final sort by RelTag in T3's
   Index() (skilldozer pattern) is STILL REQUIRED — WalkDir's lexical order ≠ relTag
   order for nested entries (e.g. `gate.ts` vs `writing/reddit-poster.ts` visit order
   differs from relTag sort).

7. **`filepath.Rel(basepath, targpath)`** returns the cleaned relative sub-path.
   `Rel("/a/b","/a/b/c/d")` → `"c/d"`; `Rel("/a/b","/a/b")` → `"."`.
   **`filepath.ToSlash`** replaces each OS separator with `/` (no-op on Unix, `\`→`/`
   on Windows).

## Implications for T2 (classify) + T3 (Index walk)

- T3's WalkDir callback, on a directory entry where `d.IsDir()`:
  - classify via T2's `classifyDir(root, path)`;
  - if recognized as extension → emit the `*Extension`, `return filepath.SkipDir`
    (prune subtree — the load-bearing recursion rule);
  - if plain → `return nil` (WalkDir descends naturally into the category folder).
- T3's callback, on a FILE entry:
  - classify via T2's `classifyFile(root, path)`;
  - if a single-file extension → emit, `return nil` (NOT SkipDir — GOTCHA #3);
  - if `index.ts`/`index.js` → `return nil` (it's a dir extension's entry, handled by
    the dir classifier when WalkDir visited the parent dir).
- Paths from WalkDir use OS separators → `filepath.Rel(root, path)` +
  `filepath.ToSlash(...)` for portable relTag. For single-file extensions, strip the
  trailing `.ts`/`.js` AFTER ToSlash.

## Sources
- https://pkg.go.dev/path/filepath#WalkDir
- https://pkg.go.dev/io/fs#WalkDirFunc
- https://pkg.go.dev/path/filepath#SkipDir
- https://pkg.go.dev/io/fs#DirEntry
- https://pkg.go.dev/path/filepath#Rel
- https://pkg.go.dev/path/filepath#ToSlash
