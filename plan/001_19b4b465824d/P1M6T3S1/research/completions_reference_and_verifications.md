# Research — P1.M6.T3.S1: weave shell completions (bash/zsh/fish)

A near-mechanical **port of skilldozer's `completions/` directory** with renames.
The skilldozer files are LOCAL, working, tested ground truth:
`/home/dustin/projects/skilldozer/completions/{skilldozer.bash,_skilldozer,skilldozer.fish}`.
Every validation command in this PRP was RUN against the skilldozer originals and PASSES.

---

## 1. CONTRACT AMBIGUITY — RESOLVED (the single most important finding)

The item description contains an internal tension:

- "Port skilldozer's completions/ directory **verbatim** with renames"
  → the reference files ship `--store`, `check`, AND `init`.
- "Flag set is **frozen to main.go parseArgs()**" + "all three files must stay in
  **LOCKSTEP with the flag matrix**"
  → main.go parseArgs() DOES handle `--store`, `check`, and `init` (all shipped,
  all COMPLETE per plan status: P1.M4.T3 = check; P1.M4.T4 = init).
- BUT the item's explicit parenthetical flag list omits `--store`:
  `(--version -v --help -h --path -p --list -l --all -a --file -f --relative --no-color --search -s)`
  and mentions only `check` (never `init`).

### Verified main.go parseArgs() flag matrix (the authoritative source)

Read directly from `/home/dustin/projects/weave/main.go`:

| Token            | main.go line | short? | value?        | notes                                           |
|------------------|--------------|--------|---------------|-------------------------------------------------|
| `--version`/`-v` | 234/277      | yes    | bool          |                                                 |
| `--help`/`-h`    | 236/279      | yes    | bool          | help wins tiebreak (§6.3)                       |
| `--path`/`-p`    | 238/284      | yes    | bool          |                                                 |
| `--list`/`-l`    | 240/286      | yes    | bool          |                                                 |
| `--all`/`-a`     | 242/288      | yes    | bool          |                                                 |
| `--file`/`-f`    | 244/290      | yes    | bool (mod)    | §6.2: entry file path                           |
| `--relative`     | 246/292      | NO     | bool (mod)    | §6.2: no short form                             |
| `--no-color`     | 248/294      | NO     | bool (mod)    | §6.2: no short form                             |
| `--search`/`-s`  | 250/296      | yes    | VALUE (query) | free-text; next token consumed                  |
| `--store`        | 253/318      | NO     | VALUE (dir)   | implies init; `--store` / `--store=`            |
| `check`          | 308          | —      | subcommand    | RESERVED positional; exclusive (§6.3)           |
| `init`           | 329          | —      | subcommand    | RESERVED positional; exclusive; takes `<dir>`   |

So the frozen matrix INCLUDES `--store`, `check`, AND `init`.

### Resolution

**Port skilldozer verbatim → KEEP `--store`, `check`, AND `init`.** The two
governing meta-principles ("verbatim port" + "lockstep with main.go parseArgs()")
both require all three. The item's parenthetical list is an incomplete summary of
the §6.1/§6.2 modifier flags; the author omitted `--store` and only highlighted
`check` (the one with the trickiest exclusivity+suppression logic).

Risk analysis (why including is correct):
- Including a VALID flag/subcommand is **never wrong behavior** for the binary —
  the worst case is the menu shows one more entry.
- Omitting `init`/`--store` would (a) break parity with skilldozer, (b) break
  the "lockstep main.go" contract, (c) leave `weave in<TAB>` completing to nothing
  for a real, shipped subcommand. That is a regression.
- `init` is in the §6.1 table; `--store` is in §6.1's `init` row ("`weave init
  --store <dir>`"). Both are real user-facing surfaces.

This is documented as a prominent section in the PRP. No code is changed to make
this work — it's purely what the ported completion files list.

---

## 2. The three reference files (read in full)

- `/home/dustin/projects/skilldozer/completions/skilldozer.bash` (2938 B)
  → `completions/weave.bash`. Function `_skilldozer_completion` → `_weave_completion`.
- `/home/dustin/projects/skilldozer/completions/_skilldozer` (2938 B, zsh)
  → `completions/_weave`. `#compdef skilldozer` → `#compdef weave`.
- `/home/dustin/projects/skilldozer/completions/skilldozer.fish` (3239 B)
  → `completions/weave.fish`. `complete -c skilldozer` → `complete -c weave`.

### Exhaustive rename map

```
skilldozer            -> weave                 (binary name: complete -F/-c target, compdef, calls)
_skilldozer_completion-> _weave_completion     (bash function name)
_skilldozer           -> _weave                (zsh function name + FILENAME)
skilldozer.bash       -> weave.bash            (filename)
skilldozer.fish       -> weave.fish            (filename)
skills                -> extensions            (descriptions: "resolved skills directory" etc.)
SKILL.md              -> entry file            ("Print the SKILL.md path" -> "entry file path")
skill directory path  -> extension path        (descriptions)
"skill tag"           -> "extension tag"       (fish -d description)
PRD §2.1              -> PRD §2                (manifest-free reference; weave hard constraints §2)
```
Note: `PRD §8.2` (init store) and `PRD §6.3` (exclusivity) keep the SAME section
numbers in weave's PRD — no rename needed there.

### Deltas over a pure find/replace

1. **bash: add `# shellcheck disable=SC2207` as LINE 1** (before the header
   comment). The verbatim `COMPREPLY=($(compgen …))` idiom triggers SC2207 (3×);
   this is the canonical bash-completion idiom (tags/flags never contain spaces,
   so word-splitting is safe — the skilldozer file documents this in a comment
   but has no disable directive, so its `shellcheck` exits 1). A file-level
   disable directive makes `shellcheck weave.bash` exit 0 CLEAN (matches the
   install.sh PRP precedent). VERIFIED: with the directive on line 1,
   `shellcheck` → exit 0.
2. Keep the skilldozer explanatory comments verbatim (they document the SC2207
   + SC2317 rationale). In shellcheck 0.11.0 SC2317 does NOT fire (the
   `_init_completion` fallback is genuinely reachable) — the comment is
   defensive for older versions; leave it.

---

## 3. Gotchas (verified — mostly inherited from skilldozer's inline comments)

- **bash `_init_completion` fallback**: `_init_completion` is from the
  bash-completion PACKAGE (absent on minimal Linux / default macOS bash). Without
  the fallback, `_init_completion || return` silently offers NOTHING. The
  `{ … COMP_WORDS … }` fallback is load-bearing. (skilldozer comment.)
- **bash/zsh `--search` vs `--store` are OPPOSITES**: `--search/-s` → free-text
  query → offer NOTHING (return 0 / `:query:`); `--store` → directory value →
  COMPLETE DIRECTORIES (`compgen -d` / `:directory:_files`). Mirrors main.go.
- **fish `-r` is an INVERSE knob**: in fish 4.x `-r` switches the directive into
  "complete the option's value" mode, BYPASSING the global `complete -c weave -f`
  (no-files). So `--search/-s` deliberately do NOT pass `-r` (free-text → nothing,
  global `-f` applies), while `--store` DOES pass `-r` (it WANTS file/dir paths).
  Getting this backwards breaks the no-files guarantee. (skilldozer comment;
  VERIFIED behavior via `complete -C`.)
- **all three suppress FILE completion**: weave takes tags/flags, not paths.
  bash: only offers dirs after `--store` (else `compgen -W`, never `compgen -f`).
  zsh: `_arguments -C` with `:query:`/`:directory:_files` only on value slots.
  fish: `complete -c weave -f` (the `-f`).
- **all three call `weave --relative --all` at completion time**: relative output
  = relTags (paths relative to the extensions dir), one per line, sorted by tag —
  exactly what to complete on. (PRD §14 says `--all`; the contract + skilldozer
  use `--relative --all` for the RELTAGS form — use `--relative --all`.)
- **`check`/`init` offered ONLY as first positional; suppressed once seen**:
  exclusive subcommands (§6.3: +tags → exit 2). bash: walk earlier words, return
  0 if either seen. zsh: `->first`/`->rest` state, suppress in `rest` if either
  in `$words`. fish: `__fish_is_first_arg` for offering;
  `not __fish_seen_subcommand_from check init` for the tag directive.
- **tags swallowed on error**: `2>/dev/null` on the `weave --relative --all`
  call — a missing/broken binary degrades to "no tags", not a stderr dump.

---

## 4. Validation gates — ALL VERIFIED on the skilldozer originals

Toolchain present: `shellcheck` 0.11.0, `bash`, `zsh` (/usr/bin/zsh), `fish`
(/usr/bin/fish).

### Level 1 (syntax/lint) — verified
- `bash -n skilldozer.bash` → exit 0. ✓
- `shellcheck skilldozer.bash` → exit 1 with SC2207 ×3 ONLY (no disable directive).
  With file-level `# shellcheck disable=SC2207` added → exit 0 CLEAN. ✓
- `zsh -n _skilldozer` → exit 0. ✓
- `fish -c 'source skilldozer.fish'` → exit 0 (parse + register OK). ✓
  (`fish_indent --check` exits 1 = formatting, NOT syntax — do NOT use as a gate.)

### Level 3 (functional) — verified
bash (COMP_WORDS):
- `COMP_WORDS=(skilldozer --v) COMP_CWORD=1` → `--version` ✓
- `COMP_WORDS=(skilldozer) COMP_CWORD=1` → includes `check init` (+ tags) ✓
- `COMP_WORDS=(skilldozer check) COMP_CWORD=2` → empty (suppressed) ✓
- `prev=--search` → empty (free-text) ✓ (the `case "$prev" in --search|-s) return 0`)
fish (`complete -C`):
- `complete -C "skilldozer --"` → 8 long flags with descriptions ✓
- `complete -C "skilldozer "` → tags + `check` + `init` ✓
zsh: `zsh -n` for syntax; full interactive functional is manual (cp to fpath,
`compinit`, type TAB). Documented.

Functional tag completion needs `weave` on PATH + `extensions/` present
(`weave --relative --all` is called at completion time). Flag/subcommand
completion does NOT need the binary (hardcoded). The Level 3 gate builds weave
into a temp bin dir and prepends it to PATH.
