# Research: Go `path/filepath.WalkDir` "classify-then-descend" semantics

## Summary
`WalkDir` calls your `fn` for a directory BEFORE descending into it; if `fn` returns `filepath.SkipDir` for that directory entry, WalkDir skips the directory's entire subtree (all descendants) and continues with the next sibling. This is exactly the "classify-then-descend" primitive you need. Entries are walked in lexical filename order, symlinks are not followed, and `fs.DirEntry.IsDir()` reliably distinguishes directories from files. `filepath.Rel` returns the cleaned sub-path and `ToSlash` swaps OS separators for `/`.

> Note: this run had no live web-fetch tooling available. Findings below are from the stable, well-known Go standard-library documentation (unchanged since `WalkDir` shipped in Go 1.16), with canonical pkg.go.dev URLs cited.

## Findings

1. **SkipDir on a directory skips its entire contents, then continues with the next sibling.** `fs.WalkDirFunc` doc: *"If the function returns `SkipDir` when invoked on a directory, WalkDir skips the directory's contents entirely."* The directory itself was already visited (fn was called for it); its whole subtree is then skipped and the walk proceeds to the next entry in the parent. This is precisely "classify-then-descend." [pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir), [pkg.go.dev/io/fs#WalkDirFunc](https://pkg.go.dev/io/fs#WalkDirFunc), [pkg.go.dev/path/filepath#SkipDir](https://pkg.go.dev/path/filepath#SkipDir)

2. **Root is passed to fn FIRST, before children; you can SkipDir the root.** WalkDir *"calls fn for each file or directory in the tree, including root."* Returning `SkipDir` for the root directory skips all of root's contents (i.e., the entire walk after root). [pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir)

3. **SkipDir on a FILE skips the remaining entries in the current directory.** *"If the function returns `SkipDir` when invoked on a file, WalkDir skips the remaining files in the current directory."* It does NOT skip a subtree (files have none); it just skips later siblings in the same directory. [pkg.go.dev/io/fs#WalkDirFunc](https://pkg.go.dev/io/fs#WalkDirFunc)

4. **WalkDir does NOT follow symlinks.** *"WalkDir does not follow symbolic links."* Symlinks appear as entries with `ModeSymlink` type; their `IsDir()` is false even if the target is a directory, so they will never be auto-recursed. [pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir)

5. **`fs.DirEntry.IsDir()` is reliable for file/dir distinction.** *"IsDir reports whether the entry describes a directory."* The type comes from the directory listing (ReadDir), so no extra `stat` is needed, and symlinks correctly report `IsDir()==false`. [pkg.go.dev/io/fs#DirEntry](https://pkg.go.dev/io/fs#DirEntry)

6. **Traversal order is lexical by filename within each directory.** *"The files are walked in lexical order, which makes the output deterministic but requires WalkDir to read an entire directory into memory before proceeding to walk that directory."* [pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir)

7. **`filepath.Rel` returns the cleaned relative sub-path; `ToSlash` swaps OS separators for `/`.** *"Rel returns a relative path that is lexically equivalent to targpath when joined to basepath with an intervening separator… Rel calls Clean on the result."* `Rel("/a/b","/a/b/c/d")` → `"c/d"`; `Rel("/a/b","/a/b")` → `"."`. *"ToSlash returns the result of replacing each separator character in path with a slash ('/')."* No-op on Unix; replaces `\`→`/` on Windows. [pkg.go.dev/path/filepath#Rel](https://pkg.go.dev/path/filepath#Rel), [pkg.go.dev/path/filepath#ToSlash](https://pkg.go.dev/path/filepath#ToSlash)

## Implications for your "classify-then-descend" walker
- In `fn`, on a directory entry where `entry.IsDir()`: classify; if it is an "extension unit", `return filepath.SkipDir, nil` to prune the whole subtree; if "plain", `return nil` to descend.
- Root: classify before the first child; `SkipDir` on root prunes everything.
- Paths delivered by WalkDir use OS separators; use `filepath.Rel(base, path)` then `filepath.ToSlash` for portable, normalized relative keys.
- No symlink-follow risk: a symlink-to-dir reports `IsDir()==false` and is never auto-recursed, so it cannot accidentally violate your "plain folder only" descent rule (but you also will not enter it unless you manually resolve it).

## Sources
- Kept: [pkg.go.dev/path/filepath](https://pkg.go.dev/path/filepath) — `WalkDir`, `SkipDir`, `Rel`, `ToSlash` (official Go stdlib, canonical reference).
- Kept: [pkg.go.dev/io/fs](https://pkg.go.dev/io/fs) — `DirEntry`, `WalkDirFunc` (the underlying fs types WalkDir delegates to; authoritative for the SkipDir semantics wording).

## Gaps
- No live fetch / quote-verification possible this run (no web tooling in this environment). The Go stdlib docs are version-stable and these behaviors are unchanged since `WalkDir`'s introduction in Go 1.16; if you target a very old toolchain, confirm against that version.
- `filepath.Walk` (the older, `os.FileInfo`-based API) was intentionally not covered since you specified `WalkDir`.
- Error-return handling (the `error` param to fn, including the special handling of a non-nil `error` from fn and the `ErrSkipDir`-style cases) was not detailed; say the word if you want a follow-up on error/retry semantics.
