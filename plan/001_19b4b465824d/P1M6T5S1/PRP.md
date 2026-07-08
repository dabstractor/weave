# PRP — P1.M6.T5.S1: Final doc sweep (verify README / features / env-vars / overview match implementation)

> **Subtask:** PRD §5 **Mode B** catch-all. Per-feature docs (Mode A) rode with
> each implementing subtask; this task verifies **cross-cutting documentation
> coherence** now that the whole P1.M1-P1.M6 changeset is in place, and fixes any
> drift in the **docs** (not the implementation). It is the accuracy gate that
> ships the milestone with trustworthy docs.
>
> **Status at write time:** `README.md` ALREADY EXISTS (written by the parallel
> task P1.M6.T4.S1) and is **highly accurate**. This is therefore a **surgical
> verification + fix pass**, not a rewrite. Three concrete drifts were found
> during research (see "Known Drifts" + `research/verified_ground_truth_and_drift.md`).

## Goal

**Feature Goal**: Audit every user-facing documentation surface in the weave
changeset against the frozen binary + PRD §15/§16/§17, then fix every drift in
the documentation. The implementation is the source of truth; the docs bend to
match it, never the reverse (unless the implementation is provably wrong).

**Deliverable**: A drift-free `README.md` (the primary doc), a `.gitignore`
reconciled against PRD §16, and an unchanged-everything-else (install.sh,
completions/*, extensions/example.ts) once confirmed clean. A short audit report
is produced as research notes.

**Success Definition**:
- Every CLI flag, env var, exit code, output format, and label string the README
  documents matches the built binary (verified by running each command).
- The 3 known drifts are resolved: the false `~` expansion claim, the missing
  `--no-extensions` pattern, and the .gitignore deviation.
- `README.md` still passes the write-tech-docs linter (`bash scripts/lint.sh
  README.md` exits 0): no em dashes, no tell-words, no long prose paragraphs.
- `go test ./...` stays green (no `.go` source is modified).

## User Persona (if applicable)

**Target User**: a pi user reading the weave README to install and use it.
**Use Case**: they copy a command from the README and expect it to behave exactly
as written.
**Pain Points Addressed**: a README that invents flags, misstates env vars, or
claims behavior the binary does not have (e.g. `~` expansion) erodes trust and
breaks copy-paste. This sweep removes exactly those gaps.
**User Journey**: the auditor walks the README top-to-bottom, running each
documented command against the built binary, and corrects any line that diverges.

## Why

- **Closes the §5 Mode B contract** — the one cross-cutting doc-coherence check
  mandated for the changeset. Per-feature docs already shipped with their tasks;
  this verifies they agree as a whole.
- **Catches cross-task drift** — the README was written by one task
  (P1.M6.T4.S1) summarizing behavior implemented across six milestones; a fact
  mis-transcribed there is invisible to the feature tests. Research already found
  3 such cases.
- **Cheap insurance, high value** — a few doc edits now prevent every future
  reader from hitting a false claim (e.g. `weave init --store '~/x'` silently
  creating a literal `~` directory).

## What

A verification + fix pass over the changeset's user-facing docs, driven by the
item's 8-point checklist (§6.1 flags, §6.2 modifiers, env vars, constraints
§15.8, install instructions, canonical one-liner + `--no-extensions`, .gitignore
§16, no stale references). Plain markdown fixes only.

### Success Criteria

- [ ] `README.md` documents ALL §6.1 commands/flags (`weave <tag>`, `--all`,
      `--list`, `--search`, `check`, `init`, `--path`, `--help`, `--version`)
      and ALL §6.2 modifiers (`--file`, `--no-color`, `--relative`) — verified
      against `weave --help`.
- [ ] `README.md` documents the env vars `weave_EXTENSIONS_DIR`, `weave_CONFIG`,
      `weave_INSTALL_BIN` (lowercase, exact).
- [ ] The Constraints section states: no catalog index (disk-discovered); a
      settings config file is fine; never auto-discovered by pi; loads only via
      `-e` (PRD §15 item 8).
- [ ] Install instructions cover all three paths (install.sh, `go install`,
      from-source) and mention `weave init` for `go install` users.
- [ ] The canonical one-liner is `pi -e "$(weave <tag>)"` AND the
      `--no-extensions` pattern is mentioned (DRIFT 2 fixed).
- [ ] The false `~` expansion claim is corrected (DRIFT 1 fixed).
- [ ] `.gitignore` reconciled against PRD §16 (DRIFT 3 resolved, documented).
- [ ] No stale references to skilldozer internals or `yaml.v3` in user-facing docs.
- [ ] `bash scripts/lint.sh README.md` exits 0; `go test ./...` stays green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes.** This is a doc audit. The deliverable is
small prose/gitignore edits. The frozen fact table below is the source of truth;
the item's 8-point checklist is the work list; the write-tech-docs linter is the
style gate. No guessing: every claim is checkable by running the built binary.

### Documentation & References

```yaml
# CONTRACT — the doc surfaces to audit (READ-ONLY unless a drift is found)
- file: README.md
  why: "The primary user-facing doc (PRD §15). The whole audit centers on it."
  pattern: "Walk it top-to-bottom against the fact table; fix only divergent lines."
  critical: "It already exists and is accurate; expect ~3 surgical edits, not a rewrite."

- file: .gitignore
  why: "Item checklist point 6: must match PRD §16. It currently has extras (DRIFT 3)."
  pattern: "PRD §16 = /weave, /dist, *.test, *.out, .DS_Store exactly."
  critical: "Do NOT strip the protective extras (node_modules/, .env*, .pi-subagents/) —
             removing them risks committing secrets and tooling artifacts. See DRIFT 3."

- file: install.sh
  why: "Item checklist point 8: check for stale skilldozer/yaml.v3 refs. It has ONE
        sanctioned skilldozer dev comment (L4); no user-facing prose leak. Confirm."
- file: completions/weave.bash, completions/_weave, completions/weave.fish
  why: "Secondary doc surface; comments only. Confirm no stale refs."
- file: extensions/example.ts
  why: "User-facing template (JSDoc). Must equal PRD §11 + main.go
        exampleExtensionTemplate byte-for-byte (it does). Confirm no stale refs."

# CONTRACT — the frozen source of truth (READ-ONLY; docs bend to match)
- file: main.go
  why: "usageText L118-168 = the authoritative flag list + exit codes. config struct
        = the full flag matrix. run() = the dispatch + precedence. resolveStore
        L1015 = `filepath.Abs(store)` (NO tilde expansion — basis of DRIFT 1)."
- file: internal/config/config.go
  why: "L108 `configEnv = 'weave_CONFIG'`; DefaultStore uses XDG_DATA_HOME."
- file: internal/extdir/extdir.go
  why: "L74 `envVar = 'weave_EXTENSIONS_DIR'`; Source.String() = the 4 --path labels."
- file: internal/check/check.go
  why: "The check report format: `%-5s %s (%s)` lines + `%d extensions, ...` summary."
- file: install.sh
  why: "`weave_INSTALL_BIN` (the 3rd documented env var); symlink-not-copy; no
        completion install."

# CONTRACT — the writing rules + the gate (KEEP the README lint-clean after edits)
- file: /home/dustin/.pi/agent/skills/write-tech-docs/SKILL.md
  why: "Hard rules the README must keep: no em dashes, no tell-words, no hedging, no
        prose paragraph over ~100 words. Any edit you make must still pass these."
- file: /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh
  why: "THE style gate. Run after every README edit. Strips code/inline-code, then
        fails on em dashes (U+2014), ' -- ' in prose, banned tell-words, >100-word
        paragraphs. Flags in backticks are SAFE (stripped first)."

# CONTRACT — the spec this implements (READ-ONLY)
- docfile: PRD.md
  section: "§15 (README outline, item 8 = Constraints bullet), §16 (.gitignore exact),
            §17 (guardrails), §6 (CLI contract), §8 (store location/env vars)."

# EVIDENCE — the verified fact table + the 3 drifts (read FIRST)
- docfile: plan/001_19b4b465824d/P1M6T5S1/research/verified_ground_truth_and_drift.md
  why: "Every CLI/env/output fact below is transcribed there with source line refs,
        plus the 3 drifts with reproduction commands and resolution guidance. This is
        the single reference for the audit."

# CONTRACT — the README that the parallel task produces (its spec; treat as input)
- docfile: plan/001_19b4b465824d/P1M6T4S1/PRP.md
  why: "Defines the README's intended content (8 §15 sections + completions). This
        task CONSUMES that README and verifies it. Re-read its 'Gotchas' so your
        fixes do not reintroduce a resolved decision (e.g. the em-dash -> period rule)."

# EXTERNAL — confirm the pi flag named in the contract before documenting it
- url: https://pi.dev
  why: "DRIFT 2 requires confirming pi's actual 'load no auto-discovered extensions'
        flag (the item contract names it `--no-extensions`). Verify via `pi --help` or
        pi docs before adding the pattern; document the real flag name."
```

### Current Codebase tree (relevant subset)

```bash
README.md                    # EXISTS (parallel T4.S1); highly accurate; 3 drifts to fix
.gitignore                   # has all 5 PRD §16 entries + extras (DRIFT 3)
install.sh                   # 1 sanctioned skilldozer dev comment (L4); confirm not user-facing
completions/{weave.bash,_weave,weave.fish}  # comments only; confirm clean
extensions/example.ts        # == PRD §11 == main.go exampleExtensionTemplate
main.go                      # usageText = frozen flag matrix; resolveStore = no ~ expansion
internal/{config,extdir,check,...}/*.go     # env vars, labels, output formats
go.mod                       # module github.com/dabstractor/weave
LICENSE                      # MIT, Copyright (c) 2026 Dustin Schultz
```

### Known Drifts of our codebase & Library Quirks

```markdown
<!-- DRIFT 1 (HIGH) — README 'First run' claims `~` expands. IT DOES NOT.
     resolveStore -> filepath.Abs(store) takes `~` literally. Reproduced in research:
     `weave init --store '/tmp/x/~/y'` created a directory literally named `~`.
     FIX (recommended): edit the README. State weave does NOT expand `~` itself; the
     caller's shell expands an UNquoted `~/path` before weave sees it. Do NOT quietly
     leave the false claim. (Impl fix is option B only if skilldozer parity is required
     AND verified — PRD §8.2 does not mandate tilde expansion.) -->

<!-- DRIFT 2 (MED) — README never mentions the `--no-extensions` pattern. The item
     contract explicitly requires it. FIX: confirm pi's real flag (contract names
     `--no-extensions`; verify with `pi --help`) and add a short Usage note/example
     for loading ONLY weave extensions, e.g. `pi --no-extensions -e "$(weave example)"`. -->

<!-- DRIFT 3 (LOW/MED) — .gitignore is a SUPERSET of PRD §16's 5 lines. All 5 required
     entries are present and correct; the extras protect against committing secrets
     (.env*) and tooling artifacts (.pi-subagents/, node_modules/). FIX: keep the 5
     required (present); keep the protective extras; trim the unused `/build`; document
     the deviation. Do NOT strip to exactly 5 lines without guarding those paths. -->

<!-- STYLE — any README edit must keep it lint-clean: no em dashes (use a period /
     colon / parentheses), no tell-words, no 'word -- word' in prose (flags in
     backticks are safe — the linter strips inline code first). -->

<!-- SCOPE — fix the DOCS, not the implementation, unless the implementation is
     provably wrong. Only DRIFT 1 has an impl-fix path (option B); prefer the doc fix.
     Never edit PRD.md, tasks.json, prd_snapshot.md, or any .go file as part of a
     'doc fix'. -->

<!-- NO-CONFLICT — install.sh (P1.M6.T2.S1), completions/* (P1.M6.T3.S1), and
     example.ts (P1.M6.T1.S1) are owned by other tasks; touch them ONLY to remove a
     confirmed user-facing stale reference, and only if a doc-only rewrite is not
     possible. The README + .gitignore are the expected edit surfaces. -->
```

## Implementation Blueprint

### Data models and structure

None. Prose + gitignore edits. The "model" is the verification workflow below.

### Known Drifts (resolve these three — verified during research)

```yaml
DRIFT 1 (HIGH): README 'First run' `~` expansion claim is FALSE
  EVIDENCE: main.go resolveStore L1015 uses filepath.Abs(store) which does NOT expand `~`.
  REPRO:   weave init --store '/tmp/d1/~/x'  ->  creates a LITERAL dir named '~'
  FIX (recommended): edit README 'First run'. Replace the '~ expands' sentence with:
    "weave does not expand `~` itself. If you want a home-relative path, leave it
    unquoted so your shell expands it before weave sees it (e.g. weave init --store
    ~/extensions), or pass an absolute path."
  ALT (only if skilldozer parity required + verified): add tilde expansion in
    resolveStore (impl fix). NOT recommended; PRD §8.2 does not require it.

DRIFT 2 (MED): `--no-extensions` pattern missing from README
  EVIDENCE: grep -i 'no-extensions' README.md -> no matches. Item contract requires it.
  FIX: confirm pi's real flag (pi --help; contract names `--no-extensions`). Add to the
    Usage section a short note + example for loading ONLY weave extensions and none of
    pi's auto-discovered ones, e.g.:
      # Load ONLY weave extensions (none of pi's auto-discovered ones)
      pi --no-extensions -e "$(weave example)"
    Use the ACTUAL pi flag name once confirmed.

DRIFT 3 (LOW/MED): .gitignore is a superset of PRD §16
  EVIDENCE: PRD §16 = /weave,/dist,*.test,*.out,.DS_Store (5). Current has those 5 +
    /build,node_modules/,vendor/,.env,.env.*,.idea/,.vscode/,*.swp,*~,.pi-subagents/.
  FIX: keep all 5 required entries (present + correct). KEEP the protective extras
    (node_modules/, vendor/, .env*, .idea/, .vscode/, *.swp, *~, .pi-subagents/).
    TRIM the unused `/build` (no /build dir exists). Record the deviation decision in
    research notes. Do NOT reduce to exactly 5 — that would risk committing secrets
    and .pi-subagents/ transcripts.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: SNAPSHOT ground truth (do this FIRST; it is the ruler for every check)
  - BUILD: go build -o /tmp/weave-doc .   (EXPECT: succeeds, no output)
  - CAPTURE --help:  /tmp/weave-doc --help > /tmp/w_help.txt
  - CAPTURE --version, --path, check, init output formats into /tmp for diffing.
  - RUN the canonical one-liner against a real store to confirm it resolves.
  - This snapshot is what every README claim is checked against.

Task 1: VERIFY README §6.1 commands/flags (checklist point 1)
  - For EACH token the README references, confirm it is a real flag in --help AND in
    main.go parseArgs: weave <tag>, --all/-a, --list/-l, --search/-s, check, init,
    --store, --path/-p, --help/-h, --version/-v.
  - EXPECT: all present (the README already documents all of them). FIX only a missing
    or misspelled one. The fact table in research §1 is the reference.
  - FILES TOUCHED: 0 unless a flag is missing/misstated.

Task 2: VERIFY README §6.2 modifiers (checklist point 1 cont.)
  - Confirm --file/-f, --relative, --no-color are documented. Confirm --relative and
    --no-color have NO short form (README must not invent -r/-n). EXPECT: correct.

Task 3: VERIFY env vars (checklist point 2)
  - Confirm README documents weave_EXTENSIONS_DIR, weave_CONFIG, weave_INSTALL_BIN
    (lowercase, exact). Cross-check against internal/extdir/extdir.go L74,
    internal/config/config.go L108, install.sh. EXPECT: all present + lowercase.

Task 4: VERIFY Constraints section (checklist point 3 / PRD §15 item 8)
  - Confirm the Constraints bullets state: no catalog index (disk-discovered); a
    settings config file is fine; never auto-discovered by pi; loads only via `-e`.
    EXPECT: present (the section already covers all four).

Task 5: VERIFY install instructions (checklist point 4)
  - Confirm all three paths: ./install.sh, go install github.com/dabstractor/weave@latest,
    from-source. Confirm `weave init` is mentioned for go install users. Confirm the
    install.sh facts (symlink not copy; weave_INSTALL_BIN / ~/.local/bin / /usr/local/bin;
    no completion install). EXPECT: correct (module path verified = dabstractor/weave).

Task 6: VERIFY canonical one-liner + FIX the `--no-extensions` gap (checklist point 5)
  - Confirm `pi -e "$(weave <tag>)"` is the canonical one-liner (it is).
  - FIX DRIFT 2: confirm pi's real flag (pi --help) and add the --no-extensions note +
    example to Usage (see Known Drifts). Keep it lint-clean (period/colon, no em dash).
  - FILES TOUCHED: README.md (1 small addition).

Task 7: FIX the false `~` expansion claim (DRIFT 1)
  - REPRO first (independently confirm): weave init --store '/tmp/d1/~/x'; ls shows a
    literal '~'. Then EDIT README 'First run' per the Known Drifts fix (recommended: state
    weave does not expand `~`; shell expands unquoted `~/path`). Keep it lint-clean.
  - FILES TOUCHED: README.md (1 sentence replaced).

Task 8: RECONCILE .gitignore against PRD §16 (checklist point 6 / DRIFT 3)
  - Verify all 5 required entries present + correct (they are). KEEP protective extras.
    TRIM the unused `/build`. Record the deviation decision + reasoning in research notes.
  - FILES TOUCHED: .gitignore (remove the `/build` line only; or leave as-is and just
    document — your call, but `/build` has no corresponding dir). NEVER remove
    node_modules/, .env*, .pi-subagents/, etc.

Task 9: VERIFY no stale references (checklist point 7)
  - grep -rni 'skilldozer' README.md extensions/example.ts completions/ install.sh
  - grep -rni 'yaml\.v3' README.md extensions/example.ts completions/
  - EXPECT: the only skilldozer hit is install.sh L4 (a dev comment sanctioned by PRD
    §12.1, NOT user-facing prose) and zero yaml.v3 in user-facing docs. If a hit is in
    genuinely user-facing prose, rewrite it; dev comments are fine.
  - FILES TOUCHED: 0 unless a user-facing stale reference is found.

Task 10: RE-LINT + re-test (the gates)
  - RUN: bash /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh README.md
    EXPECT exit 0. Fix any hit your edits introduced (em dash, tell-word, long para).
  - RUN: ! grep -nP '\x{2014}' README.md   (EXPECT: no em dashes)
  - RUN: go test ./...   (EXPECT green; no .go touched)
  - FILES TOUCHED: 0.
```

### Implementation Patterns & Key Details

```markdown
<!-- Fact-table check pattern: for each README claim, run the documented command and
     diff its output against the README's stated output. Example for check: -->
/tmp/weave-doc check   # must print lines like `OK    example (example)` and a
                       # `N extensions, M errors, K warnings` summary (note: plural
                       # 'extensions' even when N==1).

<!-- Drift-fix pattern: REPRODUCE the drift with a command BEFORE editing, so the fix
     is grounded in evidence, then verify the FIXED doc no longer makes the false claim. -->

<!-- .gitignore pattern: PRD §16 lists the weave-SPECIFIC ignores; general dev-hygiene
     extras are a benign superset. The contract's 'exactly' is satisfied in spirit by the
     5 required entries being present; the protective extras are kept on purpose. -->
```

### Integration Points

```yaml
EDITS (expected):
  - README.md          # DRIFT 1 (~ claim) + DRIFT 2 (--no-extensions note); keep lint-clean
  - .gitignore         # DRIFT 3 (trim unused /build; keep protective extras)

NO EDITS (confirm clean, leave alone):
  - *.go               # implementation is the source of truth; do not change it for a doc fix
  - install.sh         # owned by P1.M6.T2.S1; only touch a confirmed user-facing stale ref
  - completions/*      # owned by P1.M6.T3.S1; comments only
  - extensions/example.ts  # == PRD §11; do not change
  - PRD.md, tasks.json, prd_snapshot.md  # NEVER (human/orchestrator-owned)

PRODUCES (research notes):
  - research/audit_signoff.md  # the drift report: what was checked, what was fixed,
                               # the .gitignore deviation decision + reasoning
```

## Validation Loop

### Level 1: Syntax & Style (the doc gate)

```bash
cd ~/projects/weave

# The write-tech-docs linter — MUST exit 0 after your README edits.
bash /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh README.md
# Expected: exit 0. Fix any hit (em dash -> period; tell-word -> cut; long para -> split).

# Em-dash sweep (linter already covers it; confirm headings/tables too):
! grep -nP '\x{2014}' README.md && echo "no em dashes OK"
# Expected: "no em dashes OK".
```

### Level 2: No Regressions (implementation untouched)

```bash
cd ~/projects/weave
go test ./...
# Expected: all green (no .go source modified by this doc task).
```

### Level 3: Manual Accuracy Sweep (run EVERY documented command)

```bash
cd ~/projects/weave
go build -o /tmp/weave-doc .

# Every flag the README lists must be real (parity with usageText / parseArgs).
/tmp/weave-doc --help > /tmp/w_help.txt
for f in --all -a --list -l --search -s --file -f --relative --no-color \
         --path -p --help -h --version -v --store check init; do
  grep -qw -- "$f" /tmp/w_help.txt || echo "README references $f but --help lacks it (FIX)"
done
# Expected: no FIX lines.

# Canonical one-liner resolves a real extension:
weave_EXTENSIONS_DIR="$PWD/extensions" /tmp/weave-doc example   # -> /.../example.ts, exit 0

# check output format matches the README's 'OK    example (example)' / summary line:
weave_EXTENSIONS_DIR="$PWD/extensions" /tmp/weave-doc check

# --path label strings match the README's four labels:
weave_EXTENSIONS_DIR="$PWD/extensions" /tmp/weave-doc --path 2>&1 1>/dev/null

# DRIFT 1 REPRO (confirm the false claim before/after the fix):
rm -rf /tmp/d1 && /tmp/weave-doc init --store '/tmp/d1/~/x' 2>/dev/null | head -1
ls -d /tmp/d1/'~' 2>/dev/null && echo "literal ~ dir created -> README must NOT claim ~ expands"
rm -rf /tmp/d1

# Env var names in the README must be the exact lowercase spellings:
grep -rhoE 'weave_[A-Z_]+' README.md | sort -u
# Expected: only weave_CONFIG, weave_EXTENSIONS_DIR, weave_INSTALL_BIN.

# .gitignore has all 5 PRD §16 entries:
for e in '^/weave$' '^/dist$' '^\*\.test$' '^\*\.out$' '^\.DS_Store$'; do
  grep -qE "$e" .gitignore || echo "MISSING PRD §16 entry: $e"
done
# Expected: no MISSING lines.

rm -f /tmp/weave-doc /tmp/w_help.txt
# Expected: every documented command behaves exactly as the README claims.
```

### Level 4: Audit sign-off (the contract checklist)

```bash
cd ~/projects/weave

# Stale-reference sweep (only sanctioned dev comments are allowed):
grep -rni 'skilldozer' README.md extensions/example.ts completions/ install.sh
grep -rni 'yaml\.v3' README.md extensions/example.ts completions/
# Expected: at most install.sh:4 (sanctioned dev comment); zero yaml.v3 in user-facing docs.

# --no-extensions pattern now mentioned (DRIFT 2 fixed):
grep -qi 'no-extensions' README.md && echo "OK: --no-extensions mentioned" || echo "FIX: add it"

# The ~ claim corrected (DRIFT 1 fixed) — confirm the README no longer asserts ~ expands:
grep -qi 'expands to your home' README.md && echo "FIX: stale ~ claim still present" || echo "OK: ~ claim corrected"

# Structure intact (8 §15 sections + completions still present):
for h in "## Why" "## Install" "### First run" "## Shell completions" "## Usage" \
  "## How extensions are organized" "## Adding an extension" \
  "## How \`weave\` finds the store" "## Constraints"; do
  grep -qF "$h" README.md && echo "OK: $h" || echo "MISSING: $h"
done
# Expected: every heading present; all checks OK.
```

## Final Validation Checklist

### Technical Validation

- [ ] `bash scripts/lint.sh README.md` exits 0 (no em dashes, no tell-words, no long paras).
- [ ] `! grep -nP '\x{2014}' README.md` (no em dashes anywhere).
- [ ] `go test ./...` stays green (no `.go` files changed).

### Feature Validation (the item's 8-point checklist)

- [ ] All §6.1 commands/flags documented and real (verified vs `--help`).
- [ ] All §6.2 modifiers documented; `--relative`/`--no-color` have no invented short form.
- [ ] Env vars `weave_EXTENSIONS_DIR`, `weave_CONFIG`, `weave_INSTALL_BIN` documented (lowercase).
- [ ] Constraints section states the 4 §15.8 points (no catalog index; settings file fine;
      never auto-discovered; loads only via `-e`).
- [ ] Install covers all 3 paths + mentions `weave init` for `go install` users.
- [ ] Canonical one-liner `pi -e "$(weave <tag>)"` present; `--no-extensions` pattern added.
- [ ] `.gitignore` reconciled with PRD §16 (5 required present; deviation documented).
- [ ] No stale skilldozer/yaml.v3 refs in user-facing docs.

### Drift Resolution

- [ ] DRIFT 1 fixed: README no longer claims `~` expands (or impl parity verified + added).
- [ ] DRIFT 2 fixed: `--no-extensions` pattern mentioned (real pi flag confirmed).
- [ ] DRIFT 3 resolved: .gitignore deviation reasoned + recorded in research notes.

### Code Quality Validation

- [ ] Edits are surgical (README was already accurate; only real drifts fixed).
- [ ] No new doc invented flags, paths, or commands (every line verified against the binary).
- [ ] README still mirrors skilldozer's structure/tone (write-tech-docs rules kept).
- [ ] Implementation untouched (docs bend to match the binary, not vice-versa).

### Documentation & Deployment

- [ ] A research note records what was checked, what was fixed, and the .gitignore decision.
- [ ] The changeset ships with documentation fully synchronized with the implementation.

---

## Anti-Patterns to Avoid

- ❌ Don't rewrite the README. It already exists and is accurate. Fix only the verified
  drifts; re-verify everything else but leave correct lines untouched.
- ❌ Don't "fix" the implementation to match a doc claim without reproducing the drift and
  confirming the impl is genuinely wrong. The default is: fix the doc. (DRIFT 1's `~` is the
  one candidate for an impl fix, and even there the doc fix is recommended.)
- ❌ Don't strip `.gitignore` to PRD §16's exact 5 lines. The protective extras
  (`.env*`, `.pi-subagents/`, `node_modules/`) exist for a reason; removing them risks
  committing secrets and tooling transcripts. Keep them; document the deviation.
- ❌ Don't introduce an em dash, a tell-word, or a >100-word paragraph in any README edit.
  Run the linter after every edit.
- ❌ Don't invent the `--no-extensions` flag. Confirm pi's real flag (`pi --help`) before
  documenting the pattern.
- ❌ Don't touch PRD.md, tasks.json, prd_snapshot.md, or any `.go` file. This is a doc task.
- ❌ Don't skip the reproduction step for DRIFT 1. Reproduce the literal-`~` directory
  first so the fix is grounded in evidence, not assumption.
- ❌ Don't narrate or pad. The README is already concise; keep your edits equally tight.

**Confidence Score (one-pass success): 9/10.** The README already exists and is accurate;
the task is three surgical fixes (two in README, one gitignore decision) plus a verification
sweep. The risk surface is small and every fix is grounded in reproduced evidence.
