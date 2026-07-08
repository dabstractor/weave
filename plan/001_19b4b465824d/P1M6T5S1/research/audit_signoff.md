# P1.M6.T5.S1 — Final doc sweep audit sign-off

> Record of the cross-cutting documentation coherence check for the P1.M1-P1.M6
> changeset. The implementation is the source of truth; docs were bent to match
> it. Three drifts were reproduced and fixed. Everything else verified clean.

## Method

Built `/tmp/weave-doc` from a clean `go build -o /tmp/weave-doc .`, captured
`--help`, `--version`, `--path`, `check`, and tag resolution, then walked
`README.md` top to bottom running each documented command against that binary.
The frozen fact table in `verified_ground_truth_and_drift.md` §1-§3 was the
ruler for every check.

## Checklist results (the item's 8 points)

1. **§6.1 commands/flags** — PASS. Every token the README references
   (`weave <tag>`, `--all`/`-a`, `--list`/`-l`, `--search`/`-s`, `check`,
   `init`, `--store`, `--path`/`-p`, `--help`/`-h`, `--version`/`-v`) is present
   in `--help` and in `parseArgs`. No missing or misspelled flags. No edits.
2. **§6.2 modifiers** — PASS. `--file`/`-f`, `--relative`, `--no-color`
   documented. `--relative` and `--no-color` correctly have NO short form (the
   README does not invent `-r`/`-n`). No edits.
3. **Env vars** — PASS. README documents `weave_EXTENSIONS_DIR`,
   `weave_CONFIG`, `weave_INSTALL_BIN` (lowercase, exact). `grep -rhoE
   'weave_[A-Z_]+' README.md | sort -u` returns exactly those three. No edits.
4. **Constraints section (PRD §15 item 8)** — PASS. All four points present:
   no catalog index (disk-discovered); a settings config file is fine; never
   auto-discovered by pi; loads only via `-e`. No edits.
5. **Install instructions** — PASS. All three paths present
   (`./install.sh`, `go install github.com/dabstractor/weave@latest`,
   from-source). `weave init` is mentioned for `go install` users. install.sh
   facts (symlink not copy; `weave_INSTALL_BIN` / `~/.local/bin` /
   `/usr/local/bin`; no completion install) are correct. No edits.
6. **Canonical one-liner + `--no-extensions`** — one-liner PASS; `--no-extensions`
   was MISSING (DRIFT 2). Fixed (see below).
7. **`.gitignore` vs PRD §16** — DRIFT 3. Resolved (see below).
8. **No stale references** — PASS. `grep -rni skilldozer` hits only
   `install.sh:4` (a dev comment sanctioned by PRD §12.1, not user-facing
   prose). Zero `yaml.v3` in any user-facing doc. No edits.

## Drifts fixed

### DRIFT 1 (HIGH) — false `~` expansion claim — FIXED in README

Reproduced independently before editing:

```
$ rm -rf /tmp/d1 && /tmp/weave-doc init --store '/tmp/d1/~/x' 2>/dev/null | head -1
/tmp/d1/~/x
$ ls -d /tmp/d1/'~'
/tmp/d1/~
```

A directory literally named `~` was created. Root cause: `main.go resolveStore`
calls `filepath.Abs(store)`, which does not expand `~`. PRD §8.2 does not
mandinate tilde expansion, so the fix is in the docs (option A), not the
implementation.

Fix: replaced the README "First run" sentence that claimed `~` expands with a
note stating `weave` does NOT expand `~` itself; the caller's shell expands an
unquoted `~/path` before `weave` sees it, or an absolute path can be passed.
After the fix, `grep -qi 'expands to your home' README.md` returns no match.

### DRIFT 2 (MED) — `--no-extensions` pattern missing — FIXED in README

`grep -i 'no-extensions' README.md` returned no matches. The item contract
requires the pattern.

Confirmed the real pi flag before documenting: `pi --help` shows
`--no-extensions, -ne   Disable extension discovery (explicit -e paths still
work)`. The contract's flag name is correct.

Fix: added a short note plus an example to the Usage section, immediately after
the canonical one-liner, showing
`pi --no-extensions -e "$(weave example)"` to load only weave extensions and
none of pi's auto-discovered ones.

### DRIFT 3 (LOW/MED) — `.gitignore` superset of PRD §16 — RESOLVED

PRD §16 lists exactly: `/weave`, `/dist`, `*.test`, `*.out`, `.DS_Store`. All
five were present and correctly spelled (PASS on the required set). The file
also carried protective extras: `node_modules/`, `vendor/`, `.env`, `.env.*`,
`.idea/`, `.vscode/`, `*.swp`, `*~`, `.pi-subagents/`, and an unused `/build`.

Decision: kept all five required entries and kept the protective extras
(removing `.env*` would risk committing secrets; removing `.pi-subagents/`
would risk committing tooling transcripts; `node_modules/`/`vendor/` guard
deps). Trimmed the unused `/build` (no `/build` directory exists in the repo).
This is a deliberate, reasoned deviation from a literal reading of "exactly":
the five required entries are present, and the extras only ignore paths PRD
§16 never wanted committed.

## Gates

- `bash .../write-tech-docs/scripts/lint.sh README.md` exits 0 after edits.
- `! grep -nP '\x{2014}' README.md` — no em dashes.
- `go test ./...` — green (no `.go` source modified by this doc task).

## Files touched

- `README.md` — DRIFT 1 (one paragraph rewritten) + DRIFT 2 (one note + example
  added).
- `.gitignore` — DRIFT 3 (removed the unused `/build` line; kept the 5 required
  entries and all protective extras).

No `.go`, `install.sh`, `completions/*`, or `extensions/example.ts` files were
modified. PRD.md, tasks.json, and prd_snapshot.md were not touched.
