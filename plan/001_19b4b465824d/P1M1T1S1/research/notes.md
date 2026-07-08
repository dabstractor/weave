# Research Notes — P1.M1.T1.S1 (go.mod, .gitignore, LICENSE)

## Goal of this subtask
Greenfield repo: create the three repo-hygiene files that anchor the module so that
downstream subtasks (config, extdir, discover, main.go) have a compilable module to
live inside. No source code is produced here.

## Verified facts (run on the live machine, 2026-07-07)

### Go toolchain
- `/usr/bin/go` → `go1.26.4-X:nodwarf5 linux/amd64`
- Setting `go 1.25` in the directive is the "latest two stable releases" policy and is
  safe because the installed toolchain (1.26.4) is strictly newer. Go does NOT auto-add
  a `toolchain` line when the local toolchain is newer than the directive.

### Empty-module build behavior — IMPORTANT GOTCHA
Tested with a minimal `go.mod` (module line + `go 1.25`, no source files):

  $ go build ./...
  go: warning: "./..." matched no packages
  $ echo $?
  0

  $ go vet ./...
  no packages to vet
  $ echo $?
  1

Implications for the Validation Gate:
- `go build ./...` is the correct gate. It exits 0. The stderr warning
  "matched no packages" is EXPECTED and must NOT be treated as failure.
- `go vet ./...` is NOT a usable gate for this subtask (exits 1 on an empty module).
  Do not include it. It becomes valid again once main.go exists (subtask T4).
- `go mod tidy` is a no-op on a zero-dep module (produces no go.sum, leaves go.mod
  unchanged). It is safe but unnecessary; included as an optional sanity check.

### Existing weave repo state
- `/home/dustin/projects/weave` contains: `.git/`, `PRD.md`, `plan/`, `.gitignore`
- The CURRENT `.gitignore` is a generic template (node_modules/, venv/, dist/, build/,
  .env, .DS_Store). It does NOT match PRD §16. It MUST be overwritten with the exact
  PRD §16 content.
- No `go.mod`, no `LICENSE`, no `main.go`, no `internal/`.

### git identity (for LICENSE copyright line)
- `git config user.name`  → Dustin Schultz
- `git config user.email` → dustindschultz@gmail.com
- skilldozer LICENSE uses: `Copyright (c) 2026 Dustin Schultz`  ← mirror exactly.
  Year 2026 confirmed by the skilldozer LICENSE (same author, same era).

## Reference: skilldozer go.mod (the architectural sibling)
```
module github.com/dabstractor/skilldozer

go 1.25

require gopkg.in/yaml.v3 v3.0.1
```
weave drops the `require` block entirely (zero deps). Module path swaps
`skilldozer` → `weave`. `go 1.25` directive is identical.

## Reference: skilldozer LICENSE (MIT, to mirror verbatim)
Standard MIT text. Copyright line: `Copyright (c) 2026 Dustin Schultz`.
weave uses the identical text (MIT is content-identical across projects;
only the copyright line is project-specific, and here it is the same author/year).

## PRD §16 .gitignore — EXACT required content
```
/weave
/dist
*.test
*.out
.DS_Store
```
PRD §16 note: "(`/weave` ignores the locally-built binary; everything else is committed,
including `extensions/example.ts`.)" — so DO NOT add node_modules/, .env, .pi/, etc.
This is a deliberately minimal, author-specified list. Match it byte-for-byte.

## Scope boundaries (what NOT to do here)
- Do NOT create `main.go` (that is P1.M1.T4.S1).
- Do NOT create any `internal/` package (config is T2, extdir is T3).
- Do NOT create `extensions/example.ts` (P1.M6.T1.S1).
- Do NOT create `install.sh`, `completions/`, `README.md` (all P1.M6).
- Do NOT add a `toolchain` directive to go.mod (not needed; local toolchain is newer).
- Do NOT add a `require` block (zero deps is a hard constraint, PRD §4/§5/§17).

## Out-of-scope but worth noting for downstream PRPs
- The `.gitignore` does NOT exclude `.pi-subagents/` or plan artifacts. skilldozer's
  .gitignore added `.pi-subagents/`; weave's PRD §16 does not. Follow PRD §16 exactly
  (do not "improve" it). If the orchestrator wants plan/ ignored, that is a human
  decision, not this subtask's.
- `go.sum` must never appear. If a future subtask is tempted to add a dependency,
  that violates PRD §4 and should be flagged.
