# PRP — P1.M4.T1.S1: Update README.md to reflect post-fix discovery behavior

## Goal

**Feature Goal**: Sync the user-facing `README.md` with the post-bugfix discovery
behavior so no claim is stale or misleading. The bugfix sprint (Issues 1–6)
changed `discover.Index` in ways a reader can observe: the walk now skips
`node_modules/`, `.git/`, and hidden entries (Issue 6), and a symlinked
`weave_EXTENSIONS_DIR` now resolves correctly instead of yielding an empty
catalog (Issue 4). The README's "How extensions are organized" section does NOT
currently mention any skip rules — this is the primary addition. The
package-kind description ("names at least one existing entry") was already
correct (Issue 1 fixed the code, not the docs) — confirm it and leave it.

**Deliverable**: MODIFY ONE existing file — `README.md` at the project root.
- Add a 1–2 sentence note in "How extensions are organized" that `node_modules/`,
  `.git/`, and hidden entries (names starting with `.`) are skipped during
  discovery (Issue 6 — the primary change).
- Optionally add a short parenthetical to the env-var rule in "How `weave` finds
  the store" noting symlinked paths are followed (Issue 4 — the description is
  already accurate; the note is a helpful clarification, not a correction).
- Confirm (do NOT change) the package-kind "names at least one existing entry"
  wording (Issue 1).
- Do NOT document Issues 2, 3, 5 (internal fixes with no user-facing README
  surface).

NO new files. NO source code changes. This IS the documentation task (Mode B —
the changeset-level sweep per `architecture/fix_design.md` §Documentation plan).

**Success Definition**:
- `README.md` "How extensions are organized" section states that
  `node_modules/`, `.git/`, and hidden entries are excluded from discovery.
- The package-kind bullet still reads "names at least one existing entry"
  (unchanged — confirmed accurate).
- No stale claims remain: every discovery behavior the README describes matches
  the post-fix code in `internal/discover/index.go`.
- A reader who symlinks their store, drops a `.ts` file under `node_modules/`,
  or creates a `.secret.ts` knows what `weave` will do (skip / resolve).
- `go build ./...` and `go test ./...` still pass (README edits must not break
  any test that greps README — none do, but confirm).
- No markdown is broken (fenced code blocks, lists, headings all render).

## User Persona (if applicable)

**Target User**: A developer reading the README to understand how `weave`
discovers extensions — specifically, what gets included in and excluded from the
catalog. This is the audience for the "How extensions are organized" section.

**Use Case**: A user runs `npm install` at their store root (to share deps
across package extensions) and then runs `weave --list`. They expect to see
their extensions, NOT hundreds of `node_modules` packages. Without the skip note
in the README, they have no way to know `weave` excludes `node_modules/` — they
might assume it pollutes the catalog, or they might not realize a stray
`.secret.ts` is silently ignored.

**User Journey**:
1. User reads "How extensions are organized" to learn the three extension kinds.
2. They see the new skip note and learn: `node_modules/`, `.git/`, and
   dotfiles are excluded.
3. They run `npm install` at the store root confidently, knowing it will not
   pollute `weave --list`.
4. They symlink their store (`ln -s /mnt/shared/extensions ~/extensions`) and,
   seeing the env-var note, know `weave_EXTENSIONS_DIR` will follow the symlink.

**Pain Points Addressed**:
- **Silent exclusion confusion**: a user who creates `.secret.ts` and does not
  see it in `--list` has no explanation without the skip note.
- **node_modules fear**: a user who needs shared deps may avoid `npm install` at
  the store root, believing it will flood the catalog.
- **Symlink uncertainty**: a user with a symlinked store could not tell from the
  README whether it works.

## Why

- **The bugfix sprint changed observable behavior**: Issue 6 (skip
  node_modules/.git/hidden) and Issue 4 (symlinked env path) both affect what a
  user sees from `weave --list`, `weave <tag>`, and `weave --path`. The README
  must reflect the new reality or it misleads.
- **`architecture/fix_design.md` §Documentation plan prescribes this exact
  task**: Mode B (changeset-level docs sweep) says the "How extensions are
  organized" section should note the skip rules (Bug 6); the package
  description is already accurate (Bug 1); Bugs 2/3/4/5 have no user-facing
  surface beyond Mode A doc comments. This PRP implements that plan.
- **The README was RIGHT about Issue 1**: the package-kind bullet "names at
  least one existing entry" described the INTENDED behavior all along; the bug
  was in `classifyDir` (it only checked `entries[0]`), not in the docs. The fix
  made the code match the docs. Confirming (not changing) this bullet prevents a
  well-meaning editor from "fixing" wording that is already correct.
- **Scope boundary**: this task touches ONLY `README.md`. It does not change
  source code, tests, the PRD, or any architecture doc. It does not add new doc
  files (no `CHANGELOG.md`, no `DISCOVERY.md`). It is the final sweep that closes
  the bugfix sprint's documentation loop.

## What

Two edits to `README.md` (one primary, one optional), plus several confirm-no-change passes.

### Edit 1 (PRIMARY): Skip-rules note in "How extensions are organized"

**Location**: After the three-kinds bullet list (after the "names at least one
existing entry" line) and before the canonical-tag paragraph.

**Current text** (~README lines 204–216):
```markdown
## How extensions are organized

Extensions live in the `extensions/` directory at the store root. An extension
is one of three kinds:

- a single **file**: a `*.ts` or `*.js` file whose base name is not
  `index.ts` or `index.js`
- a **directory**: a directory that directly contains `index.ts` or `index.js`
- a **package**: a directory with a `package.json` whose `pi.extensions` array
  names at least one existing entry

The canonical **tag** is the entry's path **relative to `extensions/`**, with
...
```

**Add** a short note (1–2 sentences) between the bullet list and the canonical-tag
paragraph. Example wording (the implementer may refine phrasing; the content
requirements are fixed):

> During discovery, `weave` skips `node_modules/` and `.git/` directories, and
> any file or directory whose name starts with `.` (hidden entries). So an
> `npm install` at the store root will not pollute the catalog, and a stray
> `.secret.ts` is ignored.

**Content requirements** (the note MUST state all three):
1. `node_modules/` is skipped (pruned, not descended).
2. `.git/` is skipped.
3. Hidden entries — any file or directory whose name starts with `.` — are
   skipped.

**Placement options** (either is acceptable):
- A new bullet appended to the three-kinds list (e.g. a 4th bullet starting
  "Skipped during discovery: ...").
- A standalone sentence/paragraph after the bullet list and before the
  canonical-tag paragraph.

Prefer the standalone paragraph — it reads as a rule about the walk, not as a
4th extension kind.

### Edit 2 (OPTIONAL): Symlink note on the env-var rule

**Location**: "How `weave` finds the store", rule #1 (`weave_EXTENSIONS_DIR`).

**Current text** (~README lines 300–303):
```markdown
1. **`weave_EXTENSIONS_DIR` env var**: override; if set and an existing dir,
   use it. Lets CI, tests, and temporary redirects win without editing the
   config.
```

**Add** a short parenthetical noting symlinks are followed. Example:

> **`weave_EXTENSIONS_DIR` env var**: override; if set and an existing dir,
> use it (symlinked paths are resolved). Lets CI, tests, and temporary
> redirects win without editing the config.

**Why optional**: the existing description ("if set and an existing dir, use
it") is already ACCURATE after Issue 4 — a symlinked dir IS an existing dir and
now works. The note is a helpful clarification for users who symlink their
store, not a correction of a stale claim. Include it if it reads cleanly; omit
it if it clutters the sentence.

### Confirm-no-change passes (verify, do NOT edit)

1. **Package-kind bullet** (Issue 1): confirm it still reads "names at least one
   existing entry." The post-fix `classifyDir` iterates all `pi.extensions`
   entries and picks the first existing one, so this wording is correct. Do NOT
   change it.
2. **"How `weave` finds the store" rule #3** (sibling of binary): already says
   "symlink-aware: `os.Executable()` plus `EvalSymlinks()`." Accurate. No change.
3. **`--path` note**: says `--path` shows the resolved directory and the rule
   label on stderr. Still accurate — `--path` uses `extdir.Find()` directly (not
   `discover.Index`), so it shows the original symlink path; `Index` resolves
   internally. No change needed (the distinction is subtle but the README does
   not need to spell it out).
4. **"Adding an extension" JSDoc example** (Issue 2): the `/** ... */` example
   is unaffected by the degenerate-`/**/` fix. No change.
5. **`weave check` example** (Issue 5): shows a clean store; does not enumerate
   ERROR cases. The new all-missing-pi.extensions ERROR is an internal check
   detail. No change.
6. **"Constraints" section** (line ~347): "a `node_modules` package" appears in
   the "never auto-discovered by pi" list. This is about where the STORE must
   not live (pi's discovery), NOT about what weave skips inside its store. It is
   accurate and unrelated to Issue 6. Do NOT confuse the two; do NOT edit.

### Success Criteria

- [ ] "How extensions are organized" states that `node_modules/`, `.git/`, and
      hidden entries (names starting with `.`) are skipped during discovery.
- [ ] The skip note appears AFTER the three-kinds bullet list (not in the
      middle of it).
- [ ] The package-kind bullet STILL reads "names at least one existing entry"
      (unchanged — Issue 1).
- [ ] No claim in README contradicts `internal/discover/index.go`'s post-fix
      walk behavior.
- [ ] (If included) The env-var rule notes symlinks are resolved, without
      changing the rule's meaning.
- [ ] No documentation added for Issues 2, 3, or 5 (internal fixes).
- [ ] No new files created; only `README.md` modified.
- [ ] Markdown is valid (fenced code blocks, lists, headings render).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes** — the exact file (`README.md`), the
exact section ("How extensions are organized"), the exact insertion point (after
the three-kinds bullet, before the canonical-tag paragraph), the exact content
requirements (the three skip rules), the confirm-no-change list (Issues 1, 2, 3,
5, the Constraints `node_modules`), and the authoritative behavior source
(`internal/discover/index.go`, read in full) are all specified. The fix_design
§Documentation plan prescribes this task. No guessing.

### Documentation & References

```yaml
# MUST READ — the file to edit
- file: README.md
  why: The sole deliverable. Read it in full to locate the two edit points and
       the six confirm-no-change points.
  pattern: prose style is direct, second-person, with fenced code blocks for
           examples and inline backticks for paths/flags. Match it.
  gotcha: the "Constraints" section's "a node_modules package" (line ~347) is
          about where the STORE must not live (pi's discovery), NOT about what
          weave skips inside the store. Do NOT conflate it with the Issue 6 skip
          note.

# MUST READ — the authoritative post-fix behavior (the ground truth)
- file: internal/discover/index.go
  why: Contains Index() — the walk that now skips node_modules/.git/hidden (Issue 6)
       and resolves the walk root via EvalSymlinks (Issue 4). This is what the
       README must describe. Read the doc comment AND the WalkDir callback.
  pattern: skipDirs map {node_modules, .git}; hidden = strings.HasPrefix(name, ".");
           dirs → filepath.SkipDir (prune subtree); hidden files → return nil
           (skip one file, NOT SkipDir which would prune siblings).
  gotcha: the root guard (path == root) and the skip guard are SEPARATE checks in
          the DIR branch; the README note does not need to distinguish them — it
          only states what is skipped, not how.

# The prescribe-this-task doc (the Mode B plan)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/architecture/fix_design.md
  section: "Documentation plan" (Mode B - changeset-level docs sweep)
  why: prescribes EXACTLY this task: note node_modules/.git/hidden in "How
       extensions are organized"; confirm the package description; no changes
       for Bugs 2/3/4/5. This PRP implements that plan.
  critical: the plan says "No changes needed for ... Bug 5 (internal fixes with
            no user-facing doc surface beyond what Mode A covers)." Issue 4 is
            listed as needing only the optional symlink note.

# The skip semantics (why the note must say "hidden entries")
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T2S1/research/skipdir_hidden_semantics.md
  why: confirms the exact skip set (node_modules, .git, dotfiles) and the
       lexical-order reason hidden FILES use `return nil` (not SkipDir). The
       README note does not need this level of detail, but the implementer should
       know "hidden entries" means ANY name starting with ".", file or dir.

# The symlink fix (Issue 4 — for the optional env-var note)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M2T3S1/research/evalsymlinks_fix.md
  why: confirms Index resolves the walk root via EvalSymlinks; --path uses
       extdir.Find() directly (shows the symlink path). The optional README note
       ("symlinked paths are resolved") is accurate.

# The bug report (the issues this sprint fixed)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/prd_snapshot.md
  section: Issues 1-6 (the bug list)
  why: confirms which issues are user-facing (1, 4, 6) vs internal (2, 3, 5).
       Issue 1's README was already correct; Issue 4 is the optional symlink note;
       Issue 6 is the primary skip note; Issues 2/3/5 have no README surface.

# The section-by-section review (the implementer's checklist)
- docfile: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M4T1S1/research/readme_section_review.md
  why: line-by-line review of every README section, confirming what changes and
       what stays. Use this as the edit checklist.
```

### Current Codebase tree (relevant subset)

```bash
README.md                      # MODIFY — +skip note (Issue 6), +optional symlink note (Issue 4)
internal/discover/
└── index.go                   # READ-ONLY — the post-fix behavior the README must describe
plan/001_19b4b465824d/bugfix/001_de4406db873a/
└── architecture/fix_design.md # READ-ONLY — the Documentation plan (Mode B) prescribing this task
```

### Desired Codebase tree with files to be added/changed

```bash
README.md                      # MODIFIED — 1 primary edit (skip note), 1 optional edit (symlink note)
# NO new files. NO source code changes.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL: do NOT confuse the two `node_modules` mentions in README. -->
<!-- - "Constraints" section (~line 347): "a node_modules package" = where the STORE -->
<!--   must not live (pi's discovery). UNRELATED to Issue 6. Do NOT touch. -->
<!-- - The NEW skip note (Issue 6): what weave skips INSIDE the store. This is the -->
<!--   addition. The two are different concerns; the note belongs in "How extensions -->
<!--   are organized", NOT in "Constraints". -->

<!-- CRITICAL: the package-kind bullet "names at least one existing entry" is -->
<!-- CORRECT. Issue 1 fixed classifyDir (it now iterates all pi.extensions entries -->
<!-- and picks the first existing). The README described the intended behavior all -->
<!-- along. Do NOT "fix" this wording — it is right. -->

<!-- CRITICAL: do NOT add documentation for Issues 2, 3, or 5. They are internal -->
<!-- fixes (JSDoc degenerate block, root index.ts guard, check all-missing ERROR) -->
<!-- with no user-facing README surface. The fix_design §Documentation plan is -->
<!-- explicit on this. Adding them would be scope creep and would clutter the README -->
<!-- with implementation details. -->

<!-- GOTCHA: the skip note must cover hidden ENTRIES (files AND dirs whose name -->
<!-- starts with "."), not just hidden files. A `.config/` dir is pruned just like -->
<!-- a `.secret.ts` file. Phrase it as "any file or directory whose name starts -->
<!-- with '.'" to cover both. -->

<!-- GOTCHA: the env-var symlink note (Edit 2) is OPTIONAL. The existing description -->
<!-- ("if set and an existing dir, use it") is already accurate after Issue 4. Only -->
<!-- add the parenthetical if it reads cleanly; do not force it. -->

<!-- GOTCHA: --path shows the original (possibly symlinked) path; --list/<tag> -->
<!-- resolve it internally. The README does NOT need to spell out this distinction -->
<!-- (it is subtle and the existing --path description is correct). Do not add it. -->

<!-- GOTCHA: match the README's prose style — direct, second-person, backticks for -->
<!-- paths/flags, fenced blocks for examples. Do not add marketing language or -->
<!-- hedging ("weave intelligently skips..."). State the rule plainly. -->
```

## Implementation Blueprint

### Data models and structure

N/A — this is a documentation task. No data models, no code, no types.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ the authoritative behavior source
  - READ: internal/discover/index.go (the full Index() function + the doc comment).
  - CONFIRM: skipDirs = {node_modules, .git}; hidden = HasPrefix(name, "."); the
    walk root is EvalSymlinks-resolved (Issue 4).
  - PURPOSE: ground the README edits in the actual code, not the fix_design summary.
  - FILES TOUCHED: 0 (read-only).

Task 2: READ README.md in full + the section review
  - READ: README.md (all of it).
  - READ: plan/001_19b4b465824d/bugfix/001_de4406db873a/P1M4T1S1/research/readme_section_review.md
    (the line-by-line edit checklist).
  - LOCATE: the two edit points (after the three-kinds bullet; the env-var rule).
  - LOCATE: the six confirm-no-change points (package bullet, sibling rule, --path,
    JSDoc example, check example, Constraints node_modules).
  - FILES TOUCHED: 0 (read-only).

Task 3: EDIT README.md — add the skip note (Edit 1, PRIMARY)
  - FILE: README.md.
  - FIND: the three-kinds bullet list in "How extensions are organized" (the bullet
    ending "names at least one existing entry"), followed by a blank line and the
    "The canonical **tag**..." paragraph.
  - INSERT (between the bullet list and the canonical-tag paragraph): a 1-2 sentence
    note that node_modules/, .git/, and hidden entries (names starting with .) are
    skipped during discovery. State all three. Use the README's prose style.
  - VERIFY: the package bullet is UNCHANGED (still "names at least one existing entry").
  - FILES TOUCHED: 1 (README.md).

Task 4: EDIT README.md — add the optional symlink note (Edit 2)
  - FILE: README.md.
  - FIND: rule #1 (weave_EXTENSIONS_DIR) in "How weave finds the store".
  - INSERT (a short parenthetical, e.g. "(symlinked paths are resolved)"): noting
    symlinks are followed. Keep it to one clause; do not restructure the rule.
  - IF it clutters the sentence, OMIT this edit — it is optional. The description
    is already accurate.
  - FILES TOUCHED: 1 (README.md).

Task 5: CONFIRM the six no-change points
  - VERIFY (read each, confirm no edit needed):
    1. Package bullet: "names at least one existing entry" — unchanged.
    2. Sibling rule #3: "symlink-aware: os.Executable() plus EvalSymlinks()" — accurate.
    3. --path note: shows the resolved dir + rule label — accurate.
    4. JSDoc example in "Adding an extension": /** ... */ — unaffected by Issue 2.
    5. weave check example: clean-store output — unaffected by Issue 5.
    6. "Constraints" node_modules: about where the store must not live — unrelated
       to Issue 6.
  - FILES TOUCHED: 0 (read-only verification).

Task 6: VALIDATE — markdown + repo health
  - RUN: go build ./... ; go test ./...   (README edits must not break anything)
  - CHECK: render README.md mentally (or via a markdown linter if available) —
    fenced code blocks, lists, headings all intact.
  - CHECK: no stale claims remain — every discovery description matches index.go.
  - EXPECT: build/test green; markdown valid.
```

### Implementation Patterns & Key Details

```markdown
<!-- Edit 1 — the skip note (PRIMARY). Insert between the three-kinds bullet list
     and the "The canonical **tag**..." paragraph in "How extensions are organized".
     Example wording (refine phrasing if desired; content requirements are fixed): -->

During discovery, `weave` skips `node_modules/` and `.git/` directories, and any
file or directory whose name starts with `.` (hidden entries). So an
`npm install` at the store root will not pollute the catalog, and a stray
`.secret.ts` is ignored.

<!-- Edit 2 — the optional symlink note. Modify rule #1 in "How weave finds the
     store". Example (the parenthetical is the only addition): -->

1. **`weave_EXTENSIONS_DIR` env var**: override; if set and an existing dir,
   use it (symlinked paths are resolved). Lets CI, tests, and temporary
   redirects win without editing the config.
```

### Integration Points

```yaml
# This task has NO code integration points. It is a documentation sweep.
# The only "integration" is that the README's claims must match the code.

DEPENDS ON (all COMPLETE or parallel-Ready):
  - P1.M2.T2.S1 (Issue 6 skip rules) — COMPLETE  # the primary note describes this
  - P1.M2.T3.S1 (Issue 4 symlink root) — COMPLETE # the optional note describes this
  - P1.M1.T1.S1 (Issue 1 multi-entry pi.extensions) — COMPLETE # the confirm-no-change
  - P1.M3.T2.S1 (Issue 5 check ERROR) — Ready (parallel) # the confirm-no-change
    [Even if Issue 5 is still in flight, its README impact is nil — the check
     example does not enumerate ERROR cases. No dependency risk.]

PRODUCES:
  - README.md with accurate post-fix discovery documentation. Nothing consumes
    this; it is the final user-facing artifact of the bugfix sprint.

NO CHANGES TO:
  - any .go file (source code is frozen for this task)
  - any test file
  - PRD.md, tasks.json, prd_snapshot.md (orchestrator-owned)
  - any architecture/ or research/ doc (those are planning artifacts)
  - go.mod, go.sum, install.sh, completions/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown has no compiler, but validate structurally:
# 1. Fenced code blocks balance (count ``` opens == closes):
grep -c '```' README.md   # must be even

# 2. Headings are well-formed (## or ### followed by space):
grep -nE '^#{1,6}[^ #]' README.md   # should be empty (malformed headings)

# 3. No accidental tab characters (README uses spaces):
grep -nP '\t' README.md   # should be empty

# 4. The skip note is present and names all three skip targets:
grep -c 'node_modules' README.md   # >= 2 (Constraints + the new note)
grep -c '\.git' README.md          # >= 1 (the new note)
grep -ci 'hidden' README.md        # >= 1 (the new note)

# Expected: all checks pass. If the fenced-block count is odd, a ``` is unbalanced.
```

### Level 2: Content Validation (the "matches the code" check)

```bash
# Confirm the skip note describes exactly what index.go does:
# 1. node_modules and .git are in skipDirs:
grep -A3 'var skipDirs' internal/discover/index.go
# Expected: node_modules: true, .git: true

# 2. Hidden entries are skipped (HasPrefix name "."):
grep 'HasPrefix.*"\."' internal/discover/index.go
# Expected: two matches (dir branch + file branch)

# 3. The walk root is EvalSymlinks-resolved (Issue 4):
grep 'EvalSymlinks' internal/discover/index.go
# Expected: >= 1 match

# 4. The package bullet in README is UNCHANGED (Issue 1 confirm):
grep 'names at least one existing entry' README.md
# Expected: exactly 1 match (the original bullet, unedited)

# 5. The new skip note is present:
grep -i 'node_modules.*\.git\|\.git.*node_modules' README.md
# Expected: >= 1 match in "How extensions are organized"
```

### Level 3: Repo Health (no regressions)

```bash
# README edits must not break the build or any test:
go build ./...
go test ./...

# Expected: all green. No test greps README content (verified: the test suite
# tests .go behavior, not README text), so this is a safety net only.

# If a markdown linter is available, run it (optional):
# markdownlint README.md 2>/dev/null || true
```

### Level 4: Manual Read-Through (the final gate)

```bash
# Read the two edited sections in context and confirm they flow:
sed -n '/## How extensions are organized/,/## Adding an extension/p' README.md
sed -n '/## How .weave. finds the store/,/## Constraints/p' README.md

# Check the rendered README in a markdown viewer if available, OR just read the
# raw text. Confirm:
# - The skip note reads naturally after the three-kinds list.
# - The optional symlink note (if added) does not break the env-var rule's flow.
# - No claim contradicts the post-fix index.go behavior.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0 (README edits do not break the build).
- [ ] `go test ./...` — all tests pass (no test depends on README text).
- [ ] Markdown is structurally valid (balanced fences, well-formed headings).

### Feature Validation

- [ ] "How extensions are organized" states that `node_modules/`, `.git/`, and
      hidden entries (names starting with `.`) are skipped during discovery.
- [ ] The skip note appears after the three-kinds bullet list.
- [ ] The package-kind bullet is UNCHANGED ("names at least one existing entry").
- [ ] (If included) The env-var rule notes symlinks are resolved.
- [ ] No documentation added for Issues 2, 3, or 5.
- [ ] No README claim contradicts `internal/discover/index.go`.

### Code Quality Validation

- [ ] Only `README.md` is modified (no source, no tests, no new files).
- [ ] The edits match the README's existing prose style (direct, second-person,
      backticks for paths/flags).
- [ ] The two `node_modules` mentions (Constraints vs. the new note) are not
      conflated — each is correct in its own section.
- [ ] No marketing language, no hedging, no implementation detail leaking into
      user docs.

### Documentation & Deployment

- [ ] The skip note gives a user enough to predict `weave`'s behavior (an
      `npm install` at the root will not pollute; a `.secret.ts` is ignored).
- [ ] The optional symlink note (if added) reassures users with symlinked stores.
- [ ] This task closes the bugfix sprint's Mode B documentation loop (per
      `architecture/fix_design.md` §Documentation plan).

---

## Anti-Patterns to Avoid

- ❌ Don't document Issues 2, 3, or 5 in the README — they are internal fixes
  (JSDoc degenerate block, root index.ts guard, check all-missing ERROR) with no
  user-facing surface. The fix_design §Documentation plan is explicit. Adding
  them clutters the README with implementation detail.
- ❌ Don't change the package-kind bullet "names at least one existing entry" —
  it was ALWAYS correct (Issue 1 fixed the code to match the docs, not vice
  versa). A well-meaning "fix" here would be wrong.
- ❌ Don't conflate the two `node_modules` mentions — "Constraints" (~line 347)
  is about where the STORE must not live (pi's discovery); the new note is about
  what weave skips INSIDE the store. They are different concerns in different
  sections.
- ❌ Don't spell out the `--path`-shows-symlink-vs-`--list`-resolves distinction
  — it is subtle, the existing `--path` description is correct, and adding it
  would confuse more than it clarifies.
- ❌ Don't add a 4th extension "kind" for skipped entries — the skip rule is a
  property of the walk, not an extension kind. Use a standalone note/paragraph,
  not a bullet in the three-kinds list (or if a bullet, clearly label it as
  "skipped", not as a kind).
- ❌ Don't create new doc files (`CHANGELOG.md`, `DISCOVERY.md`, etc.) — the
  item CONTRACT says "Update existing README.md only."
- ❌ Don't edit any `.go` file, test, the PRD, tasks.json, or any architecture/
  research doc — this task is `README.md` ONLY.
- ❌ Don't add marketing language ("weave intelligently skips...") or hedging
  ("weave tries to skip...") — state the rule plainly, matching the README's
  existing direct style.
- ❌ Don't force the optional symlink note (Edit 2) if it clutters the sentence —
  the env-var description is already accurate. Omit it if it does not read cleanly.
- ❌ Don't forget to run `go build ./...` and `go test ./...` after editing — even
  though no test greps README, it is a free safety net against accidental edits
  to other files.

---

**Confidence Score: 9/10** for one-pass success. The task is a small, well-scoped
documentation edit to one file: a 1–2 sentence skip note (the primary change,
prescribed verbatim by `fix_design.md` §Documentation plan Mode B) plus an
optional one-clause symlink parenthetical. The authoritative behavior source
(`internal/discover/index.go`) has been read in full, so the note will match
reality. The confirm-no-change list (Issues 1, 2, 3, 5 + the Constraints
`node_modules`) is explicit, preventing both under-documenting (missing the skip
note) and over-documenting (leaking internal fixes). The only residual risk
(not 10/10) is phrasing: the implementer must state all three skip targets
(node_modules, .git, hidden) clearly and place the note in the right spot
without disrupting the section's flow — a markdown read-through (Level 4) catches
any awkward result, but prose quality is inherently subjective.
