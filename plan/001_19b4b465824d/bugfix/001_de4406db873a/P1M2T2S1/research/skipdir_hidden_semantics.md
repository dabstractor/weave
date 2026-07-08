# Research: hidden-entry and skipDir semantics for WalkDir (Bug 6 / Issue 6)

Verified against Go stdlib docs (pkg.go.dev path/filepath, io/fs) and the
existing weave codebase (internal/discover/index.go, internal/extdir/extdir.go).

## 1. The exact bug (Issue 6 / Bug 6)

`discover.Index`'s WalkDir callback descends into EVERY plain directory and
classifies EVERY `.ts`/`.js` file. Three footguns result:

1. **`node_modules/`** — created by `npm install` at the store root (a supported
   pattern for sharing deps across extensions). WalkDir descends; every nested
   package with an `index.js`/`index.ts` becomes an extension (e.g. tag
   `node_modules/somepkg`). This can pollute the catalog with HUNDREDS of
   dependency packages.
2. **`.git/`** — a git repo at the store root (or anywhere). Contains `.ts`-
   like artifacts in hooks/, etc. Spurious entries.
3. **Hidden files** (`.secret.ts`, `.DS_Store`-as-.ts, etc.) — `isExtensionFile`
   accepts ANY `*.ts`/`.js` not named `index.*`. A `.secret.ts` passes (it ends
   in `.ts`, is not `index.ts`). It becomes an extension tagged `.secret`.

The bug PRD (Issue 6) calls these "footguns" — not strict spec violations
(PRD §7.1 does not require skipping these) but real catalog pollution.

## 2. The fix (from fix_design.md §Bug 6)

Add two skip checks to the WalkDir callback:

```go
// Package-level skip set:
var skipDirs = map[string]bool{"node_modules": true, ".git": true}

// In the DIR branch, AFTER the root guard, BEFORE classifyDir:
name := d.Name()
if skipDirs[name] || strings.HasPrefix(name, ".") {
    return filepath.SkipDir // prune node_modules, .git, hidden dirs
}

// In the FILE branch, BEFORE classifyFile:
if strings.HasPrefix(d.Name(), ".") {
    return nil // skip hidden files (.secret.ts, etc.)
}
```

Plus: add `"strings"` to the import block (index.go currently imports errors,
io/fs, os, path/filepath, sort — NO strings).

## 3. Why `return filepath.SkipDir` for dirs but `return nil` for files

This is the load-bearing distinction (verified in research from the parent
PRP's walkdir_skipdir_semantics.md):

- **SkipDir on a DIRECTORY** skips that directory's ENTIRE subtree (all
  descendants) and continues with the next sibling. This is what we want for
  `node_modules/` and `.git/` — prune the whole subtree, do not walk into it.
  [pkg.go.dev/io/fs#WalkDirFunc](https://pkg.go.dev/io/fs#WalkDirFunc): *"If the
  function returns `SkipDir` when invoked on a directory, WalkDir skips the
  directory's contents entirely."*

- **SkipDir on a FILE** skips the REMAINING entries in the current directory
  (the file's siblings), NOT a subtree. This would be CATASTROPHIC here: if
  `.secret.ts` is visited before `myext.ts` (lexical order: `.` sorts before
  letters), returning SkipDir would skip `myext.ts` too — the real extension
  vanishes. So hidden FILES get `return nil` (skip just that one file, continue
  to siblings). [pkg.go.dev/io/fs#WalkDirFunc](https://pkg.go.dev/io/fs#WalkDirFunc):
  *"If the function returns `SkipDir` when invoked on a file, WalkDir skips the
  remaining files in the current directory."*

- For hidden DIRECTORIES, `return nil` would DESCEND into them (WalkDir walks
  the hidden dir's children). We want to PRUNE them, so `SkipDir` is correct.

**Summary**: dirs that should be excluded → `SkipDir` (prune subtree). Files
that should be excluded → `nil` (skip the one file, keep walking siblings).

## 4. Lexical order means `.`-prefixed names sort FIRST

WalkDir visits entries in LEXICAL (filename) order within each directory.
[pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir):
*"The files are walked in lexical order."*

In ASCII/Unicode, `.` (0x2E) sorts before letters (`a`=0x61, `A`=0x41) and
digits (`0`=0x30). So `.secret.ts` is visited BEFORE `myext.ts`. This is why
the file-branch MUST use `return nil` (not SkipDir) — if it used SkipDir,
`.secret.ts` would prune `myext.ts` (and every other sibling).

## 5. `d.Name()` and `d.IsDir()` on `fs.DirEntry`

[pkg.go.dev/io/fs#DirEntry](https://pkg.go.dev/io/fs#DirEntry):
- `Name() string` — "returns the name of the file (or subdirectory) described
  by the entry. This name is the final element of the path (the base name), not
  the entire path." So for `/store/.secret.ts`, `d.Name()` == `.secret.ts`
  (the base name, including the leading dot). For `/store/node_modules`,
  `d.Name()` == `node_modules`.
- `IsDir() bool` — "reports whether the entry describes a directory." Reliable;
  comes from ReadDir, no extra stat.

So `strings.HasPrefix(d.Name(), ".")` correctly detects hidden files AND hidden
dirs by their BASE name. `skipDirs[d.Name()]` checks the exact base name
against the set.

## 6. The `.` and `..` entries are NOT visited by WalkDir

WalkDir does NOT yield `.` (self) or `..` (parent) entries — those are shell
conveniences, not real directory entries. So `d.Name()` is never literally
`.` or `..` (they would otherwise match `strings.HasPrefix(name, ".")`).
[pkg.go.dev/path/filepath#WalkDir](https://pkg.go.dev/path/filepath#WalkDir)
walks the entries returned by `ReadDir`, which excludes `.` and `..`.
[Verified: os.ReadDir](https://pkg.go.dev/os#ReadDir) returns entries "sorted by
filename" and does not include `.` or `..`.

So the hidden-entry check is safe — it only catches user-named hidden files/dirs
(`.secret.ts`, `.config/`, `.git/`).

## 7. Symlinked `.ts` files are NOT addressed by this fix

The bug PRD (Issue 6) lists THREE footguns: node_modules/, hidden files, and
**symlinked `.ts` files**. This subtask (per its item contract) fixes only the
first two. Symlinked files are a separate concern:
- WalkDir does NOT follow symlinked DIRECTORIES (stdlib default), but it DOES
  visit symlinked FILES as entries.
- A symlinked `.ts` file (`ln -s shared.ts linked.ts`) is visited, `d.IsDir()`
  is false, `classifyFile` classifies it as a single-file extension.
- The item contract for P1.M2.T2.S1 does NOT mention symlinked files — only
  `node_modules`, `.git`, hidden dirs, and hidden files. Do NOT add symlink
  handling here (out of scope; would need its own decision + test).

## 8. Coexistence with the root-skip guard (P1.M2.T1.S1, already landed)

The root-skip guard (`if path == root { return nil }`) is ALREADY in index.go
(lines 62-70, verified). It is the FIRST check in `if d.IsDir()`. The skipDirs
check MUST go AFTER it (the root is never named `node_modules` or `.git`, so
order is functionally irrelevant, but placing it after the root guard keeps the
"root first" question first). The item contract specifies: "AFTER the root-skip
guard and BEFORE classifyDir."

The hidden-DIR check (`strings.HasPrefix(name, ".")`) and the `skipDirs` check
are combined into one `if` with `||` — both return `SkipDir`. This matches
fix_design.md §Bug 6's `shouldSkipDir` helper (though the item contract inlines
the check rather than extracting a helper — either is fine; inlining is simpler
and matches the contract verbatim).

## 9. Coexistence with the symlink-root fix (P1.M2.T3.S1, planned)

P1.M2.T3.S1 will add `filepath.EvalSymlinks(root)` ABOVE the callback. This
does NOT affect the skipDirs/hidden check: `d.Name()` is the base name of each
entry under the (resolved) root, regardless of whether the root itself was a
symlink. The two fixes compose cleanly.

## 10. Doc comment update (Mode A)

The Index doc comment (lines 11-50) currently describes classify-then-descend
and mentions "avoiding descent into node_modules/" (line 28, already there —
aspirational before this fix). The item contract (point 5, DOCS) asks to
"mention that well-known non-extension directories (node_modules, .git) and
hidden entries are skipped during the walk." The existing line 28 already
half-says this; the Mode A update should make it precise: name node_modules,
.git, AND hidden entries (files and dirs whose base name starts with `.`).

## Sources
- https://pkg.go.dev/path/filepath#WalkDir — lexical order, no symlink-follow.
- https://pkg.go.dev/io/fs#WalkDirFunc — SkipDir semantics (dir vs file).
- https://pkg.go.dev/io/fs#DirEntry — Name(), IsDir().
- https://pkg.go.dev/os#ReadDir — excludes `.` and `..`.
- /home/dustin/projects/weave/internal/discover/index.go — current code (root
  guard already present at lines 62-70).
- /home/dustin/projects/weave/plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md §Bug 6.
