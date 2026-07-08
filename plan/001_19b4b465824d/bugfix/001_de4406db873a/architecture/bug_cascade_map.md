# Bug Cascade Map — Discovery / Classification Subsystem

All paths relative to `/home/dustin/projects/weave`. Line numbers are exact as of
this commit (8f3819e). Each bug entry: location, code involved, why it breaks, minimal fix.

---

## Bug 1 — `classifyDir` checks only `entries[0]` (multi-entry pi.extensions)

**Location:** `internal/discover/discover.go` — `func classifyDir(root, path string)`

**Code path (case a):**
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

**Cascade when entries[0] is missing but a later entry exists**
(e.g. `pi.extensions = ["./missing.ts", "./real.ts"]` and only `real.ts` exists):

1. entries[0] = "./missing.ts" → fileExists = **false**.
2. No loop over remaining entries. Falls out of case (a).
3. Case (b) hasPkg && perr != nil — JSON valid so perr == nil → skip.
4. Case (c) no index.ts → skip. Case (d) no index.js → skip.
5. Case (e): return nil, false, true — **plain directory, descend**.

**Result:** The dir is misclassified as a plain category folder and descended.
The existing entry file is discovered as a stray single-file extension
(relTag = bare filename), fragmenting the package and losing all package.json
metadata.

**Minimal fix:** Replace entries[0]-only check with a loop finding the first
existing entry (mirrors `extdir.hasPiExtensions`).

---

## Bug 2 — init-vs-discovery inconsistency (resolved by Bug 1 fix)

**Location:** `internal/extdir/extdir.go` — `func hasPiExtensions(dir string) bool`

Already correct: iterates ALL entries, returns true if ANY exists. No separate
fix needed — Bug 1's loop fix makes discover.classifyDir agree.

---

## Bug 3 — check reports WARN for all-missing pi.extensions

**Location:** `internal/check/check.go` — `func appendEmptyFolderFindings`

When a top-level child dir has zero discover.Index entries, it emits
WARN "empty category folder" — even when the dir has a package.json with
non-empty pi.extensions pointing at all-missing files.

**Minimal fix:** Before the WARN, check for a package.json with all-missing
pi.extensions → ERROR instead.

---

## Bug 4 — WalkDir classifies the ROOT dir

**Location:** `internal/discover/index.go` — WalkDir callback

WalkDir always visits root first. classifyDir(root, root) with index.ts at root
→ relTag "." → SkipDir prunes the ENTIRE store.

**Minimal fix:** `if path == root { return nil }` before classifyDir.

---

## Bug 5 — ExtractJSDoc single-line branch returns "/" for `/**/`

**Location:** `internal/discover/jsdoc.go` — single-line branch

For `/**/`: TrimPrefix(line, "/**") = "/" (opener/closer overlap at pos 2).
Index("/", "*/") = -1. Stray "/" kept as content.

**Minimal fix:** Find "*/" on original line first; if position <= 3, content
is empty.

---

## Bug 6 — findEnv returns verbatim symlink path; WalkDir Lstats root

**Location:** `internal/extdir/extdir.go` findEnv + `internal/discover/index.go` WalkDir

findEnv: os.Stat follows symlink (passes), returns symlink path verbatim (no
EvalSymlinks). WalkDir: Lstats root → ModeSymlink → walks nothing → empty catalog.

**Minimal fix:** EvalSymlinks(root) in discover.Index before walking.

---

## Cross-Bug Dependency Graph

```
Bug 1 (entries[0] only) ──────► Bug 3/Issue5 (check all-missing ERROR)
  classifyDir case (a)            check.go
  NO dependencies                 DEPENDS ON Bug 1

Bug 2 (JSDoc /**/) ────── independent ───── jsdoc.go
Bug 4 (root index.ts) ──┐
Bug 6 (symlink root)  ──┤── all in index.go ── SAME FILE, sequential
Bug Issue6 (skip dirs) ──┘
```
