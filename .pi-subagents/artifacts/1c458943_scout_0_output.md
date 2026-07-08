# Code Context — `discover.Index()` consumer contract

Scout of the weave codebase to confirm the consumer contract for the about-to-be-implemented `discover.Index()`. This document is reference-only (no production code was changed); the only file written is this scout artifact. It captures: exact wiring points in `main.go`, the `Extension`/`BuildExtension` shapes `Index()` must populate, the **existing `walkClassified` test scaffold that IS a working WalkDir reference for `Index()`**, and the go.mod facts.

> ⚠️ Correction from an earlier draft: the claim "no walk tests exist yet" was **wrong**. `internal/discover/discover_test.go` contains `walkClassified` — a complete, tested WalkDir skeleton (see §4). The initial `grep` *tool* missed it because `discover_test.go` is **untracked** (the grep tool skips git-ignored/untracked files); only `bash grep` surfaced it. Use `bash grep` for untracked files.

---

## 1. Consumer wiring points in `main.go`

`main.go` currently dispatches ONLY `--version` and `--path` (milestone P1.M1.T4). `discover.Index()` is **NOT called anywhere yet**, and `main.go` does **NOT** import `discover` — it imports only `github.com/dabstractor/weave/internal/extdir` (the `extdir.Find()` used by `--path`).

### `run()` dispatch structure (`main.go` lines 358-392)

`run(args []string, stdout, stderr io.Writer) int` is the testable dispatcher. Current branch order:

| Lines | Branch | Status |
|-------|--------|--------|
| 360-366 | `// 1) --version` → `if c.version { ... return 0 }` | **DISPATCHED** |
| 368-384 | `// 2) --path` → `if c.path { dir, src, err := extdir.Find(); ... return 0/1 }` | **DISPATCHED** |
| 387-391 | `// 3) All other parsed modes are no-ops` → bare `return 0` | **STUB — future home of Index() callers** |

The `config` struct (lines ~52-70) ALREADY parses the full §6.1/§6.2 matrix: `list`, `all`, `file`, `relative`, `searchMode/searchQ`, `check`, `init`, `tags`. The parser is complete; later milestones add dispatch branches ONLY in `run()`.

### Where `Index()` will later be wired

Per the file's own doc comments, `Index()` first consumer is **M2.T5.S1** (`--list`). The insertion point is **between line 384 (end of the `--path` block) and line 387 (the `// 3)` no-op comment)**. Future branches:

- **M2.T5.S1** — `--list`: `exts, err := discover.Index(dir)` then `ui.PrintList(stdout, exts)`.
- **M3.T2.S1** — `<tag>`/`--file`/`--all`/`--relative`: `discover.Index(dir)` → `resolve.Resolve(exts, tag)`.
- **M4.T3.S1** — `--search`/`check`: `discover.Index(dir)` → `search.Search` / `check.Check`.

Each future branch resolves `dir` via its own `extdir.Find()` then calls `discover.Index(dir)`; there is **no** shared "resolve dir once" step today (a helper may be factored later).

### Key files
1. `main.go` (lines 1-15) — package doc + imports; `extdir` imported, `discover` NOT.
2. `main.go` (lines 52-70) — `config` struct, full §6.1/§6.2 matrix parsed.
3. `main.go` (lines 358-392) — `run()` dispatch: `--version` (360-366), `--path` (368-384), no-op tail (387-391).

---

## 2. Architecture mapping §3d — Index() signature & behavior spec

`plan/001_19b4b465824d/architecture/architecture_mapping.md` §3d ("`index.go` — Index() walk + sort (ADAPT)") specifies **behavior**:

- Use `filepath.WalkDir` with **custom descend logic**: return `filepath.SkipDir` when a dir is recognized as an extension (emit the entry first); else `nil` (descend naturally). Classify file entries.
- **Stat-guard the root before walking** (skilldozer pattern).
- **Sort by `RelTag`**.
- **Empty store → nil slice, nil error.**

The exact function signature is **not spelled out literally in §3d**, but is strongly implied by `extension.go:61` ("`Index()` (T3) returns a `[]Extension`") and the "nil slice, nil error" rule:

```go
// index.go, package discover
func Index(root string) ([]Extension, error)
```

**This implied signature is now corroborated** by the existing `walkClassified` test helper (§4), which is `walkClassified(root string) []Extension` — `Index()` is essentially `walkClassified` + a root stat-guard + the `error` return + nil-slice-on-empty. The consumer classification is also corroborated by `extdir/extdir.go:285` and `extdir/extdir_test.go:401` ("discover.Index (P1.M2.T3)").

---

## 3. `Extension` struct + `BuildExtension` signature (extension.go)

The exact shapes `Index()` populates via the classify functions. From `internal/discover/extension.go`.

### `Extension` struct (lines 82-95)

```go
type Extension struct {
    Path           string
    EntryFile      string
    RelTag         string
    Kind           string   // "file" | "dir" | "package"
    Name           string
    Description    string
    Keywords       []string
    Category       string
    Aliases        []string
    HasPackageJSON bool
}
```

All 10 fields confirmed. `Keywords`/`Aliases` are nil-vs-empty-sensitive (callers MUST test with `len()`, not a nil check). `Path` is the default output; `EntryFile` is the `--file` output. `Kind` ∈ {"file","dir","package"}.

### `BuildExtension` signature (extension.go line ~223)

```go
func BuildExtension(path, entryFile, relTag, kind string, pkg packageJSON, hasPkg bool, jsdocDesc string) Extension
```

Total (no error return, no panic). `pkg` is the unexported `packageJSON` type; `hasPkg` comes from `parsePackageJSON`'s second return and is passed verbatim to `HasPackageJSON`. `Index()` does NOT call `BuildExtension` directly — the classify functions already do. `Index()` calls the classify functions.

### Classify functions `Index()` will drive (discover.go)

> The architecture doc §3c names a single `classifyEntry(dir, name, isDir)`. The **actual** implemented API in `discover.go` is **two functions**, not one:

- `classifyFile(root, path string) (*Extension, bool)` — discover.go ~line 60. Returns `(ext, true)` for single-file `.ts`/`.js` (not `index.*`); `(nil, false)` otherwise.
- `classifyDir(root, path string) (ext *Extension, isExtension bool, shouldDescend bool)` — discover.go ~line 90. Recognized extension → `(ext, true, false)` (caller returns `SkipDir`); plain dir → `(nil, false, true)` (caller returns nil, descend).

Both take `root`+`path`; neither takes `name`/`isDir` args. This two-function API is **confirmed by `walkClassified`** (§4), which branches on `d.IsDir()`.

### Key files
1. `internal/discover/extension.go` (lines 82-95) — `Extension` struct (10 fields).
2. `internal/discover/extension.go` (lines ~223-247) — `BuildExtension` total constructor.
3. `internal/discover/extension.go` (lines ~134-184) — `parsePackageJSON(dir) (pkg, hasPkg, err)` 3-valued contract.
4. `internal/discover/discover.go` (lines 60-72) — `classifyFile`.
5. `internal/discover/discover.go` (lines 90-150) — `classifyDir` with package > index.ts > index.js > descend precedence.

---

## 4. Walk tests — a working reference scaffold EXISTS (`walkClassified`)

`discover_test.go` is **untracked** (not yet committed). It contains `walkClassified` (lines 16-44), described in its own comment as *"the mini-Index: the T3 WalkDir skeleton lived in the test file until T3 (P1.M2.T3) implements it for real in index.go."*

**`walkClassified` is the canonical reference implementation for `Index()`.** It drives a real `filepath.WalkDir` over the classify functions and applies the SkipDir rule correctly. Read it in full before implementing `Index()`:

```go
// internal/discover/discover_test.go:16
func walkClassified(root string) []Extension {
	var result []Extension
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entry, keep walking
		}
		if d.IsDir() {
			ext, isExt, descend := classifyDir(root, path)
			if isExt {
				result = append(result, *ext)
			}
			if !descend {
				return filepath.SkipDir // prune subtree (load-bearing recursion rule)
			}
			return nil // plain dir → descend
		}
		ext, ok := classifyFile(root, path)
		if ok {
			result = append(result, *ext)
		}
		return nil // NOT SkipDir (SkipDir on a file skips siblings)
	})
	sort.Slice(result, func(i, j int) bool { return result[i].RelTag < result[j].RelTag })
	return result
}
```

**Critical SkipDir nuance documented in the test comment:** `SkipDir` on a DIR skips its subtree (used to prune recognized extension dirs); `SkipDir` on a FILE skips **siblings** — which is why the file branch returns `nil`, NOT `SkipDir`. Reference: `research/walkdir_skipdir_semantics.md`.

### What `Index()` adds on top of `walkClassified`

`walkClassified` is the skeleton; `Index()` must additionally (per §3d + skilldozer pattern):

1. **Stat-guard the root** before walking (return an error if `root` doesn't exist/isn't a dir — `walkClassified` ignores the WalkDir root error via `if err != nil { return nil }`).
2. **Return `error`** (walkClassified returns `[]Extension` only; `Index` returns `([]Extension, error)`).
3. **Empty store → nil slice, nil error** (walkClassified returns whatever the walk produced; the nil-vs-empty semantics for the empty-store case are Index's contract).

### Test coverage already in place (drives `walkClassified`)

18+ table-driven tests in `discover_test.go` exercise the classify functions through `walkClassified`, including `TestWorkedExample` (line 409) — an end-to-end walk. These tests will pass against a correctly-implemented `Index()` once they're retargeted to call `Index` instead of `walkClassified`. The classify-then-descend rule (no double-counting of dir-extension internals) is PROVEN by these tests today.

### Directory listing (`ls internal/discover/`)

`discover.go`, `discover_test.go` (untracked), `extension.go`, `extension_test.go`, `jsdoc.go`, `jsdoc_test.go`. **No `index.go` and no `index_test.go` yet** — that is what you'll create.

---

## 5. go.mod

`go.mod` (2 lines):
- Module path: **`github.com/dabstractor/weave`** ✓
- Go version: **`go 1.25`** ✓ → import path for `discover` is `github.com/dabstractor/weave/internal/discover`; `t.Chdir` (Go 1.24+) and `t.TempDir()` are available for walk tests.

---

## Architecture — how the pieces connect

```
run() [main.go]
  └─ (M2+) extdir.Find() → dir
        └─ discover.Index(dir)            [index.go — DOES NOT EXIST YET]
              ├─ stat-guard root
              └─ filepath.WalkDir(dir, fn)   ← mirror walkClassified
                    ├─ dir  → classifyDir(root, path) [discover.go]
                    │         ├─ parsePackageJSON(path) [extension.go]
                    │         ├─ ExtractJSDoc(entryFile) [jsdoc.go]
                    │         └─ BuildExtension(...) [extension.go] → *Extension
                    │         (isExt → emit + return SkipDir; else nil)
                    └─ file → classifyFile(root, path) [discover.go]
                              └─ (same S1+S2+BuildExtension)
                              (ok → emit; return nil — NOT SkipDir)
              └─ sort.Slice by RelTag
              └─ return ([]Extension, error)  (nil slice + nil err if empty)
```

`Extension` is consumed downstream by `resolve.Resolve` (M3.T1), `search.Search` (M4.T1), `check.Check` (M4.T2), `ui.PrintList` (M2.T4) — none exist yet.

## Start Here

1. **`internal/discover/discover_test.go:16`** (`walkClassified`) — the canonical reference. `Index()` is this + a root stat-guard + the `error` return + nil-slice-on-empty. Copy its WalkDir callback structure verbatim.
2. **`internal/discover/discover.go:60` & `:90`** — `classifyFile` / `classifyDir`, the two functions `Index()`'s callback calls. Their doc comments spell out the exact `SkipDir`-vs-`nil` contract.
3. **`internal/discover/extension.go:82`** — the `Extension` struct you collect into the returned slice.

## Open questions / residual risks (scout, no edits made)

1. **`Index(root) ([]Extension, error)` signature** is implied (not literal in §3d) but corroborated by `walkClassified`'s `[]Extension` return. Low risk.
2. **Root stat-guard error semantics** — §3d says "stat-guard the root" but doesn't specify the error type/wording. Skilldozer's pattern (port) is the guide; the implementer should pick a clear error.
3. **Test retargeting** — existing `discover_test.go` tests call `walkClassified`. After `Index()` lands, decide whether to (a) retarget them to `Index`, (b) keep `walkClassified` as a unit-test seam and add separate `Index` tests, or (c) delete `walkClassified`. The file comment says the skeleton "lived in the test file until T3 implements it for real" — implying `walkClassified` should eventually be removed or the tests retargeted. Confirm with the implementer's intent.
4. **`classifyEntry` ≠ actual API** (§3c names one function; reality is `classifyFile`+`classifyDir`). Resolved by `walkClassified`; flagged for implementers reading only the architecture doc.