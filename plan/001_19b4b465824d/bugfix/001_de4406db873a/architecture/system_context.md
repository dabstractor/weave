# System Context — Bugfix 001_de4406db873a

## What this changeset is

A creative end-to-end QA pass against the `weave` implementation found 6 issues
(1 Major, 5 Minor). All PRD §13 acceptance gates pass; the issues are narrow
edge cases and one functional correctness gap in package-extension discovery.

This document is the **architectural entry point** for downstream PRP agents.
Pair it with `bug_cascade_map.md` (exact code locations + minimal fixes) and
`test_patterns.md` (how to write regression tests).

## Codebase summary

`weave` is a Go CLI (module `github.com/dabstractor/weave`, Go 1.25) that resolves
extension tags to absolute paths for `pi -e "$(weave <tag>)"`. Key packages:

| Package              | File(s) of interest                | Role                                      |
|----------------------|------------------------------------|-------------------------------------------|
| `internal/discover`  | `discover.go`, `index.go`, `jsdoc.go`, `extension.go` | Walks the store, classifies entries, extracts JSDoc |
| `internal/check`     | `check.go`                         | §9 validation: duplicate tags, entry existence, deps, descriptions |
| `internal/extdir`    | `extdir.go`                        | §8.3 store location priority (env → config → sibling → walk-up) |
| `internal/resolve`   | `resolve.go`                       | §7.2 tag resolution (canonical → basename → name → alias) |
| `internal/search`    | `search.go`                        | `--search` substring filter               |
| `internal/ui`        | `ui.go`                            | `--list` table renderer                   |
| `internal/config`    | `config.go`                        | config.yaml hand-rolled parser            |
| `main.go`            | `main.go`                          | CLI dispatch, arg parsing                 |

## Bug-to-file mapping

| Bug  | Severity | File(s) modified        | Function(s)                      |
|------|----------|------------------------|----------------------------------|
| 1    | Major    | `discover/discover.go` | `classifyDir` case (a)           |
| 2    | Minor    | `discover/jsdoc.go`    | `ExtractJSDoc` single-line branch|
| 3    | Minor    | `discover/index.go`    | `Index` WalkDir callback         |
| 4    | Minor    | `discover/index.go`    | `Index` pre-walk setup           |
| 5    | Minor    | `check/check.go`       | `appendEmptyFolderFindings`      |
| 6    | Minor    | `discover/index.go`    | `Index` WalkDir callback         |

## Dependency graph

```
Bug 1 (classifyDir multi-entry) ───────► Bug 5 (check all-missing ERROR)
  discover.go                               check.go
  NO dependencies                           DEPENDS ON Bug 1 (after multi-entry
                                            fix, "all missing" is the only
                                            remaining fall-through)

Bug 2 (JSDoc /**/) ────── independent ───── jsdoc.go
Bug 3 (root index.ts) ──┐
Bug 4 (symlink root)  ──┤── all in index.go ── SAME FILE, must be sequential
Bug 6 (node_modules)  ──┘

Final docs task ────── depends on ALL implementing subtasks
```

**Critical ordering constraint:** Bugs 3, 4, and 6 all modify
`internal/discover/index.go` — the `Index` function and its WalkDir callback.
They MUST be implemented by a single agent sequentially (or in strict
dependency order) to avoid file conflicts.

## Architecture decisions

### D1: Fix Bug 1 in classifyDir, not in Index
The `entries[0]`-only check is in `classifyDir` case (a). The fix replaces it
with a loop that finds the **first existing** entry, mirroring
`extdir.hasPiExtensions`. This is the single root cause — fixing it here also
resolves the init-vs-discovery inconsistency (init's `hasPiExtensions`
already iterates all entries correctly).

### D2: Fix Bug 5 in check.appendEmptyFolderFindings
After Bug 1's fix, a package.json whose pi.extensions are ALL missing still
falls through classifyDir (correct per §7.1: "≥1 existing entry" fails). The
dir is descended, yields zero entries, and `check` emits a misleading
"empty category folder" WARN. The fix adds a pre-WARN check: if the child dir
has a package.json with non-empty pi.extensions where ALL entries are missing,
emit an ERROR naming the bad entries instead. This resolves the §7.1/§9
tension in favor of the explicit §9 requirement.

### D3: Fix Bug 4 in discover.Index, not in extdir.findEnv
`findEnv` deliberately preserves the symlink path verbatim (documented
intent). `--path` correctly reports it. The problem is `filepath.WalkDir`
cannot walk a symlinked root (it Lstats, sees `ModeSymlink`). Resolving the
symlink in `Index` (via `filepath.EvalSymlinks`) before walking covers ALL
`Find()` callers uniformly and is the smaller blast radius. The existing
`TestFindEnvDoesNotResolveSymlinks` test stays valid (findEnv behavior
unchanged).

### D4: Fix Bugs 3 + 6 in the WalkDir callback
Both modify the `if d.IsDir()` branch of the WalkDir callback in `Index`:
- Bug 3: add `if path == root { return nil }` at the top (root is a container,
  never classified).
- Bug 6: add a skip check for well-known non-extension dirs (`node_modules`,
  `.git`) and hidden entries (base name starts with `.`).

### D5: Fix Bug 2 in ExtractJSDoc single-line branch
The opener `/**` and closer `*/` overlap in `/**/` at position 2. Stripping
`/**` first destroys the closer's `*`. Fix: find `*/` on the **original** line
first, then strip both opener and closer.

## What does NOT change

- No new packages, no new files, no new dependencies.
- No changes to `main.go`, `resolve.go`, `search.go`, `ui.go`, `config.go`.
- No changes to `PRD.md`, `.gitignore`, `install.sh`, `completions/`.
- The public API of every package stays the same (all fixes are internal).
- All existing tests must continue to pass (the fixes are additive/defensive).
