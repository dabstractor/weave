# PRP — P1.M1.T1.S1: `go.mod`, `.gitignore`, `LICENSE`

## Goal

**Feature Goal**: Establish the compilable Go module skeleton for the `weave` repo so
that every downstream subtask (config, extdir, discover, main.go, …) has a valid module
root to add packages into. This is the first subtask in the plan and touches no Go source.

**Deliverable**: Three root-level files in the repo:
1. `go.mod` — module `github.com/dabstractor/weave`, `go 1.25` directive, **no** `require`
   block, **no** `toolchain` line.
2. `.gitignore` — overwritten to EXACTLY the five-line PRD §16 list.
3. `LICENSE` — standard MIT license text, copyright `Dustin Schultz`, year `2026`.

After creation, `go build ./...` must exit 0 (an empty-but-valid module).

**Success Definition**:
- `go build ./...` exits 0 (the stderr warning `matched no packages` is expected and OK).
- `cat go.mod` shows the module path and `go 1.25`, with NO `require` and NO `go.sum`
  generated.
- `cat .gitignore` is byte-for-byte the PRD §16 content (and nothing else).
- `cat LICENSE` is the standard MIT text matching the sibling `skilldozer` repo.
- No `main.go`, no `internal/`, no other files are created by this subtask.

## Why

- This is the dependency-free foundation for the entire P1 plan. Nothing else compiles
  until the module root exists.
- PRD §4/§5/§17 make "zero third-party dependencies" a hard, non-negotiable constraint.
  Anchoring that decision in `go.mod` now (no `require` block) prevents accidental
  dependency drift in later subtasks.
- PRD §16 specifies the `.gitignore` content authoritatively; the repo currently has a
  *generic* `.gitignore` that violates §16 and must be corrected before any build
  artifacts or example extensions appear.
- PRD §5/§19 require an MIT LICENSE matching the `skilldozer`/`mcpeepants` convention.

## What

Three root files, created/overwritten in repo order. No CLI surface, no user-facing
behavior, no config/API change (these are repo-hygiene artifacts — PRD §5 marks
LICENSE/.gitignore as non-documented infrastructure).

### Success Criteria

- [ ] `go.mod` exists with `module github.com/dabstractor/weave` and `go 1.25`, no
      `require`, no `toolchain`, no `go.sum`.
- [ ] `.gitignore` content equals the PRD §16 five-line list exactly.
- [ ] `LICENSE` is standard MIT, copyright line `Copyright (c) 2026 Dustin Schultz`.
- [ ] `go build ./...` exits 0.
- [ ] No other files created (no `main.go`, no `internal/`, no `extensions/`).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** This PRP gives exact file contents, the precise
`go` directive, the exact `.gitignore` bytes, the exact LICENSE text to mirror, the
verified build-gate behavior (including the empty-module warning gotcha), and explicit
out-of-scope boundaries. No further codebase knowledge is required.

### Documentation & References

```yaml
- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative source for module path (§5), .gitignore content (§16),
       MIT LICENSE requirement (§5/§19), and zero-deps constraint (§4/§17).
  critical: §16 .gitignore is a fixed five-line list — do NOT add entries.
            §17 guardrail: never add a catalog/manifest; never add runtime deps.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/codebase_state.md
  why: Confirms greenfield state, Go 1.26.4 toolchain, module path, and that
       skilldozer is the architectural sibling to mirror conventions from.
  critical: "Minimum Go: latest two stable releases → set go 1.25 in directive."

- file: /home/dustin/projects/skilldozer/go.mod
  why: Reference for the exact go.mod shape weave must follow (minus the require block).
  pattern: |
    module github.com/dabstractor/<repo>
    go 1.25
  gotcha: skilldozer HAS a `require gopkg.in/yaml.v3` line — weave MUST NOT.
          Copy the structure, drop the require block.

- file: /home/dustin/projects/skilldozer/LICENSE
  why: The MIT LICENSE text to mirror verbatim (same author, same year).
  pattern: Standard MIT License body; copyright line `Copyright (c) 2026 Dustin Schultz`.
  gotcha: The MIT body is identical across projects; only the copyright line varies.
          Here the author AND year match skilldozer, so the file is effectively identical.

- file: /home/dustin/projects/weave/.gitignore  (CURRENT — to be overwritten)
  why: Documents the current (wrong) state so the implementer knows to REPLACE, not merge.
  critical: Current content is a generic template (node_modules/, venv/, .env, …) that
            VIOLATES PRD §16. Overwrite entirely; do not preserve any existing lines.
```

### Current Codebase tree

```bash
$ cd /home/dustin/projects/weave && ls -A1
.git
.gitignore     # ← generic template, WRONG, must be overwritten (PRD §16)
PRD.md         # read-only spec
plan/          # orchestration artifacts
# (no go.mod, no LICENSE, no source)
```

### Desired Codebase tree with files to be added/changed

```bash
weave/
├── .gitignore   # OVERWRITE with PRD §16 five-line content (was a generic template)
├── LICENSE      # NEW — standard MIT, Copyright (c) 2026 Dustin Schultz
├── PRD.md       # untouched (read-only)
├── go.mod       # NEW — module github.com/dabstractor/weave / go 1.25 / no require
└── plan/        # untouched
# NOTHING ELSE. No main.go, no internal/, no go.sum, no extensions/.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (verified on this machine, 2026-07-07):
# An empty Go module (go.mod with no packages) behaves like this:
#
#   $ go build ./...
#   go: warning: "./..." matched no packages
#   $ echo $?
#   0
#
#   $ go vet ./...
#   no packages to vet
#   $ echo $?
#   1
#
# => `go build ./...` is the correct gate (exit 0; the warning is expected).
# => `go vet ./...` FAILS on an empty module (exit 1) — DO NOT use it as a gate
#    for this subtask. It only becomes valid once main.go exists (P1.M1.T4.S1).

# CRITICAL: Go does NOT emit a `toolchain` line when the local toolchain (1.26.4)
# is NEWER than the `go` directive (1.25). Do not add one manually.

# CRITICAL: `go mod tidy` on a zero-dep module is a no-op: it writes no go.sum and
# leaves go.mod unchanged. Safe to run as a sanity check; not required.

# CRITICAL: The current /home/dustin/projects/weave/.gitignore is a GENERIC template
# (node_modules/, venv/, dist/, build/, .env, .DS_Store). It does NOT match PRD §16.
# It must be OVERWRITTEN, not edited/merged. See Task 2.

# CRITICAL: PRD §16 is deliberately minimal and author-specified. Do NOT "improve" it
# by adding .pi-subagents/, plan/, *.log, IDE dirs, etc. skilldozer's .gitignore has
# extra entries — those are skilldozer's choices, NOT weave's. Follow PRD §16 byte-for-byte.
```

## Implementation Blueprint

### Data models and structure

None. This subtask produces no Go types, no packages, no executable code. It produces
three plain-text root files. (Pydantic/ORM notes from the generic template do not apply.)

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE go.mod  (path: /home/dustin/projects/weave/go.mod)
  - WRITE the EXACT content:
        module github.com/dabstractor/weave

        go 1.25
  - NAMING: lowercase, root-level, literally `go.mod`.
  - DO NOT add: a `require` block, a `toolchain` line, a comment header.
  - DO NOT run `go mod init` (it may inject a toolchain line or pick a different go
    directive based on the local toolchain). Write the file by hand with the content above.
  - JUSTIFICATION: PRD §5 module path; PRD §4/§17 zero-deps; codebase_state.md "go 1.25".
  - VERIFY (after write): `cat go.mod` shows exactly 3 non-blank lines: the module
    line, a blank line, and `go 1.25`. No fourth content line.

Task 2: OVERWRITE .gitignore  (path: /home/dustin/projects/weave/.gitignore)
  - WRITE the EXACT content (PRD §16, byte-for-byte):
        /weave
        /dist
        *.test
        *.out
        .DS_Store
  - This REPLACES the current generic template entirely (do not preserve old lines).
  - DO NOT add: node_modules/, venv/, .env, build/, .pi-subagents/, plan/, IDE dirs,
    or any trailing comment block. PRD §16 is authoritative and minimal.
  - JUSTIFICATION: PRD §16; the PRD note "everything else is committed, including
    extensions/example.ts" confirms the list is intentionally short.
  - VERIFY (after write): `cat .gitignore` shows exactly those 5 lines and nothing else.

Task 3: CREATE LICENSE  (path: /home/dustin/projects/weave/LICENSE)
  - WRITE the standard MIT License text, copyright line:
        Copyright (c) 2026 Dustin Schultz
  - MIRROR verbatim the body of /home/dustin/projects/skilldozer/LICENSE (same author,
    same year, so the files will be identical). If skilldozer's LICENSE is unavailable,
    use the canonical SPDX-MIT text with the copyright line above.
  - NAMING: uppercase `LICENSE` (no extension), root-level.
  - DO NOT: add an author URL, a signature, or a project description. Plain MIT only.
  - JUSTIFICATION: PRD §5 "MIT (match skilldozer/mcpeepants conventions)"; PRD §19.
  - VERIFY (after write): `head -3 LICENSE` shows "MIT License", blank, and the
    copyright line; file ends with the standard "SOFTWARE." paragraph.

Task 4: VALIDATE the module compiles (empty module)
  - RUN:    cd /home/dustin/projects/weave && go build ./...
  - EXPECT: exit code 0. Stderr MAY contain: go: warning: "./..." matched no packages
           — this warning is EXPECTED and is NOT an error. Treat exit 0 as pass.
  - DO NOT run `go vet ./...` here (it exits 1 on an empty module — see Gotchas).
  - OPTIONAL sanity: `go mod tidy` (no-op, confirms no hidden deps; should not create
    go.sum). If a go.sum appears, STOP — something added a dependency, which violates
    PRD §4.
  - JUSTIFICATION: Success Definition requires a compilable empty module.

Task 5: CONFIRM scope boundary (no stray files)
  - RUN:    cd /home/dustin/projects/weave && ls -A1
  - EXPECT: only .git, .gitignore, LICENSE, PRD.md, go.mod, plan/ — nothing else.
  - DO NOT create: main.go, internal/, extensions/, install.sh, completions/, README.md.
    Those belong to later subtasks (T4, T2/T3, P1.M6.T1, P1.M6.T2, P1.M6.T3, P1.M6.T4).
```

### Implementation Patterns & Key Details

```bash
# There is no Go code pattern in this subtask. The only "pattern" is the file-content
# contract. The single non-obvious detail is the empty-module build behavior:

# After Task 1, this is the expected/valid state — exit 0, warning to stderr is fine:
$ go build ./...
go: warning: "./..." matched no packages      # <- EXPECTED, do not "fix" this
$ echo $?
0

# Do NOT try to silence the warning by adding an empty package or a placeholder main.go.
# An empty module is the correct, intended end state for THIS subtask. The warning
# disappears naturally once P1.M1.T4.S1 adds main.go.
```

### Integration Points

```yaml
DATABASE:
  - none. No database, no migrations.

CONFIG:
  - none. The config/ directory and settings file are P1.M1.T2.S1. This subtask creates
    NO config. (Exception: .gitignore is repo hygiene, not application config.)

ROUTES / API:
  - none. weave is a CLI with no server. No endpoints, no routes.

MODULE ROOT (the one real integration point):
  - go.mod establishes the module root that ALL future packages import under:
      import "github.com/dabstractor/weave/internal/discover"   # P1.M2.T1
      import "github.com/dabstractor/weave/internal/config"      # P1.M1.T2
      ...
  - Every downstream subtask assumes `go build ./...` already works at the repo root.
    This subtask makes that true.

GIT:
  - .gitignore change takes effect immediately; the previously-untracked generic
    .gitignore becomes the PRD §16 version on commit. No git operations are performed
    by this subtask (commit is the orchestrator's concern).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# go.mod / LICENSE / .gitignore are plain text — no linter applies at this stage.
# (ruff/mypy are Python tools; not applicable to a Go repo with no Go source yet.)
# Validate by inspection instead:

cd /home/dustin/projects/weave

# go.mod: exactly module line + blank + go directive, nothing else.
diff <(printf 'module github.com/dabstractor/weave\n\ngo 1.25\n') go.mod \
  && echo "go.mod OK" || echo "go.mod MISMATCH"

# .gitignore: PRD §16 five-line list, byte-for-byte.
diff <(printf '/weave\n/dist\n*.test\n*.out\n.DS_Store\n') .gitignore \
  && echo ".gitignore OK" || echo ".gitignore MISMATCH"

# LICENSE: standard MIT, correct copyright line.
grep -q 'MIT License' LICENSE && grep -q 'Copyright (c) 2026 Dustin Schultz' LICENSE \
  && echo "LICENSE OK" || echo "LICENSE MISMATCH"

# Expected: all three print OK. Fix any MISMATCH before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# NONE. There is no Go source to test in this subtask — no packages, no functions.
# Unit tests begin at P1.M1.T2 (config) / P1.M2 (discover). Do not create a test file
# just to have one; an empty _test.go would be an anti-pattern.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/weave

# THE gate for this subtask: empty module must compile.
go build ./...
# EXPECTED: exit code 0.
# EXPECTED stderr (NOT an error): go: warning: "./..." matched no packages
echo "build exit=$?"   # must be 0

# Confirm zero-dependency invariant: no go.sum should exist.
test ! -f go.sum && echo "no go.sum (correct)" || echo "FAIL: go.sum exists"

# Optional no-op sanity check (should change nothing, create no files):
go mod tidy
git status --short go.mod   # expect: clean (no modification by tidy)

# Confirm the module is importable in principle (no actual import added — dry check):
go list ./... 2>/dev/null || true   # may print the warning again; exit code ignored

# Expected: build exits 0; no go.sum; tidy is a no-op; no source files exist yet.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# None for this subtask. No MCP, no Docker, no DB, no web UI, no performance test.
# The domain-specific check IS Level 3's `go build ./...` against the empty module.

# Optional human review: open LICENSE and confirm it reads as standard MIT with the
# Dustin Schultz / 2026 copyright line, matching the skilldozer repo convention (PRD §5).
diff LICENSE /home/dustin/projects/skilldozer/LICENSE && echo "LICENSE matches skilldozer" || echo "(differs — verify copyright line/year)"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 passed: `go.mod`, `.gitignore`, `LICENSE` match their exact content contracts.
- [ ] Level 3 passed: `go build ./...` exits 0 (empty-module warning is acceptable).
- [ ] No `go.sum` file exists (zero-dependency invariant, PRD §4/§5/§17).
- [ ] `go vet ./...` was NOT used as a gate (it fails on empty modules — see Gotchas).
- [ ] `go mod tidy` is a no-op (confirms no hidden requires).

### Feature Validation

- [ ] `go.mod` contains `module github.com/dabstractor/weave` and `go 1.25`.
- [ ] `go.mod` has NO `require` block and NO `toolchain` line.
- [ ] `.gitignore` is exactly the PRD §16 five-line list (no extra entries).
- [ ] `LICENSE` is standard MIT, `Copyright (c) 2026 Dustin Schultz`.
- [ ] Scope respected: no `main.go`, no `internal/`, no `extensions/`, no other files.

### Code Quality Validation

- [ ] Follows existing convention: matches skilldozer's go.mod shape and LICENSE text.
- [ ] File placement: all three files at repo root (per PRD §5 target layout).
- [ ] Anti-patterns avoided: did not run `go mod init` (would inject toolchain line);
      did not add stray packages to silence the empty-module warning; did not "improve"
      the PRD §16 .gitignore with extra ignores.

### Documentation & Deployment

- [ ] No new env vars (none introduced).
- [ ] No user-facing docs required (LICENSE/.gitignore are repo hygiene, PRD §5 marks
      them as non-documented infrastructure). README.md is a later subtask (P1.M6.T4).

---

## Anti-Patterns to Avoid

- ❌ Don't run `go mod init` — it may inject a `toolchain go1.26.4` line or choose a
  different `go` directive based on the local toolchain. Hand-write go.mod instead.
- ❌ Don't add a `require` block, even an empty one. Zero deps is a hard PRD constraint.
- ❌ Don't add a `toolchain` directive. The local toolchain (1.26.4) is newer than the
  `go 1.25` directive, so none is needed or wanted.
- ❌ Don't create `main.go` or an empty `internal/` package just to silence the
  `matched no packages` build warning. The empty module is the correct end state here.
- ❌ Don't treat the `go build ./...` stderr warning as a failure. Exit code 0 is the
  only signal that matters.
- ❌ Don't use `go vet ./...` as a gate for this subtask (it exits 1 on empty modules).
- ❌ Don't merge the old generic `.gitignore` into the new one — OVERWRITE it entirely.
- ❌ Don't add entries to `.gitignore` beyond PRD §16 (no node_modules/, .env, .pi/, …).
  PRD §16 is deliberately minimal and author-specified.
- ❌ Don't invent a custom license header or add project branding to the MIT text.
- ❌ Don't commit anything — git operations are the orchestrator's concern, not this
  subtask's.
