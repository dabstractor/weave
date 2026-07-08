# PRP — P1.M6.T3.S1: `completions/weave.bash` + `_weave` (zsh) + `weave.fish`

> **Subtask:** Ship three shell-completion files for `weave` under a new
> `completions/` directory — bash, zsh, fish (PRD §14, parity with skilldozer).
> It is a near-mechanical **port of skilldozer's working, tested completions**
> (`/home/dustin/projects/skilldozer/completions/`) with the renames
> `skilldozer→weave`, `_skilldozer_completion→_weave_completion`,
> `_skilldozer→_weave`, `skills→extensions`, `SKILL.md→entry file`. All three
> complete **flags + the `check`/`init` subcommands + dynamic tags** from
> `weave --relative --all`, suppress file completion, and stay in LOCKSTEP with
> `main.go parseArgs()`. **Every validation command in this PRP was RUN against
> the skilldozer originals and PASSES.** See
> `research/completions_reference_and_verifications.md`.

---

## ⚠️ CONTRACT AMBIGUITY — RESOLVED (read this FIRST)

The item description's parenthetical flag list omitted `--store` and mentioned
only `check` (never `init`):

> compgen -W with all long+short flags (--version -v --help -h --path -p --list -l
> --all -a --file -f --relative --no-color --search -s) … Offer `check` only …

BUT the same contract also says (a) **"Port skilldozer's completions verbatim"**
and (b) **"Flag set is frozen to main.go parseArgs()"** / **"all three files must
stay in LOCKSTEP with the flag matrix"**. Verified directly against
`/home/dustin/projects/weave/main.go parseArgs()`:

| Token          | main.go line | short? | value?        |
|----------------|--------------|--------|---------------|
| `--store`      | 253, 318     | NO     | VALUE (dir)   |
| `check`        | 308          | —      | subcommand    |
| `init`         | 329          | —      | subcommand    |

All three are real, shipped, COMPLETE features (P1.M4.T3 = check, P1.M4.T4 =
init; `--store` implies init). The skilldozer reference ships all three.

**RESOLUTION: port verbatim → KEEP `--store`, `check`, AND `init` in all three
files.** Both governing meta-principles (verbatim port + lockstep with main.go)
require all three. The parenthetical was an incomplete summary of the §6.1/§6.2
modifier flags. Including a valid flag/subcommand is **never wrong behavior** for
the binary (the menu just shows one more entry); omitting `init`/`--store` would
break parity, break the lockstep contract, and leave `weave in<TAB>` completing
to nothing for a real subcommand. **No code change makes this work — it is purely
what the ported files list.** The full ported content below already includes all
three; do not strip them.

---

## Goal

**Feature Goal**: Create `completions/` with three shell-completion files
(bash/zsh/fish) that complete `weave`'s flags, the `check`/`init` subcommands,
and dynamic tags from `weave --relative --all` — a faithful port of skilldozer's
completions with renames, in lockstep with `main.go parseArgs()`. All three
suppress file completion (weave takes tags/flags, not paths).

**Deliverable**: THREE new files, each with install instructions in its HEADER
comment (Mode A self-documenting):
- `completions/weave.bash` — function `_weave_completion`, registered via
  `complete -F _weave_completion weave`.
- `completions/_weave` — zsh autoload function, `#compdef weave`.
- `completions/weave.fish` — `complete -c weave` directives.

**Success Definition**:
- All three files parse/lint clean: `bash -n` + `shellcheck` (exit 0) for bash,
  `zsh -n` for zsh, `fish -c 'source'` (exit 0) for fish.
- Functional (verified commands in Validation Loop): flags complete on a `-`
  token; `check` + `init` offered ONLY as the first positional; tags complete
  from `weave --relative --all`; tags suppressed once `check`/`init` seen OR after
  `--search`/`-s` (free-text); `--store` completes directories; NO file
  completion anywhere else.
- No `.go` source change; `go test ./...` stays green. install.sh is NOT touched
  (P1.M6.T2.S1 owns it and only PRINTS a pointer to this task).

## User Persona (if applicable)

**Target User**: a shell user typing `weave ` — wants TAB to surface the right
tags/flags without remembering them.

**Use Case**: `weave red<TAB>` → completes to a matching extension tag;
`weave --<TAB>` → offers the flag matrix; `weave ch<TAB>` → `check`.

**User Journey**: source/copy the one completion file for their shell → reload →
`weave <TAB>` works. Tags are always current because they are derived live from
disk (`weave --relative --all`), not a static catalog.

**Pain Points Addressed**: weave is manifest-free (PRD §2) — there is no sidecar
catalog to read, so tags can ONLY be discovered by asking the binary. Manual
tag-typing is error-prone; completion removes the friction.

## Why

- **Closes the §14 contract** — PRD §14 mandates bash/zsh/fish parity with
  skilldozer, completing subcommands/flags after `weave `/`weave --` and tags via
  the binary. This task lands all three files.
- **Manifest-free ⇒ completions MUST call the binary** — weave has no sidecar
  catalog (PRD §2). The only way to enumerate tags is `weave --relative --all`
  (cheap, disk-derived). The completion calls it at completion time and swallows
  errors (a missing/broken `weave` degrades to "no tags", not a stderr dump).
- **Lockstep with `main.go parseArgs()`** — there is no shared source of truth
  the shells can import, so the flag lists are hand-mirrored in each file. The
  header comment of each file states this; if a future task adds a flag, all
  three (and `parseArgs`) must move together.
- **Enables M6.T4 (README) / M6.T5 (doc sync)** — README §3 will reference these
  files for install (Mode B, P1.M6.T4.S1). This task ships the Mode A
  header-comment install instructions; the README cross-reference is deferred.

## What

A new `completions/` directory with three files. No Go source change. Each file:
1. Lists the full flag matrix (long + short forms) frozen to `main.go parseArgs()`.
2. Offers `check` and `init` ONLY as the first positional token; suppresses tags
   once either is seen (exclusive subcommands, §6.3: +tags → exit 2).
3. Routes `--search`/`-s` to free-text (offer nothing) and `--store` to directory
   completion (the inverse knob).
4. Suppresses file completion everywhere (weave takes tags/flags, not paths).
5. Derives tags dynamically from `weave --relative --all`.
6. Carries install instructions in its header comment (Mode A).

### Success Criteria

- [ ] `completions/weave.bash`, `completions/_weave`, `completions/weave.fish`
      exist (NEW directory + 3 NEW files).
- [ ] `bash -n completions/weave.bash` → exit 0; `shellcheck completions/weave.bash` → exit 0 (clean).
- [ ] `zsh -n completions/_weave` → exit 0; first line is `#compdef weave`.
- [ ] `fish -c 'source completions/weave.fish'` → exit 0.
- [ ] bash functional: flags on `-` token; `check`/`init` as first positional;
      empty after `check`; empty when `prev=--search`.
- [ ] fish functional: `complete -C "weave --"` lists flags; `complete -C "weave "`
      lists tags + `check` + `init` (with `weave` on PATH + `extensions/` present).
- [ ] Flag matrix in all three == `main.go parseArgs()` matrix (incl. `--store`,
      `check`, `init`). No file completion except `--store`'s directory value.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes.** The deliverable is THREE shell files.
The reference (skilldozer's completions) is local, working, and tested — the weave
version is a near-mechanical port with a documented rename map. The full ported
content of all three files is transcribed verbatim in "Implementation Patterns"
(copy-paste-ready). The codebase facts an implementer needs: the flag matrix is
frozen to `main.go parseArgs()` (incl. `--store`/`check`/`init` — verified), tags
come from `weave --relative --all` (relTags, one per line), there is no existing
`completions/` dir, `.gitignore` does NOT ignore it, and the toolchain is present
(`shellcheck` 0.11.0, `bash`, `zsh`, `fish`). All validation commands were RUN
and pass.

### Documentation & References

```yaml
# CONTRACT — the three reference files to port (LOCAL, read-only ground truth)
- file: /home/dustin/projects/skilldozer/completions/skilldozer.bash
  why: "PRD §14 = parity with skilldozer. This is the working, tested bash
        completion: function _skilldozer_completion, compgen -W flag matrix,
        _init_completion fallback, --search→nothing / --store→dirs routing,
        check/init exclusive, tags from `skilldozer --relative --all`."
  pattern: "complete -F _weave_completion weave. Port with the rename map; add a
            file-level `# shellcheck disable=SC2207` (line 1) for a clean gate."
  critical: "The COMPREPLY=($(compgen …)) idiom is the canonical bash-completion
             pattern; SC2207 is an acceptable false-positive (tags/flags have no
             spaces). Do NOT 'fix' it with mapfile — keep the idiom + the disable."

- file: /home/dustin/projects/skilldozer/completions/_skilldozer
  why: "The zsh autoload completion. `#compdef skilldozer`, _arguments -C with
        :query: (--search) and :directory:_files (--store), ->first/->rest state,
        check/init in `first`, suppressed in `rest` if seen in $words."
  pattern: "#compdef weave. ${(f)…} newline split on `weave --relative --all`."

- file: /home/dustin/projects/skilldozer/completions/skilldozer.fish
  why: "The fish completion. complete -c weave -f (no files); flag matrix with
        -s/-l/-d; --search/-s WITHOUT -r (free-text→nothing), --store WITH -r
        (dir completion); check/init via __fish_is_first_arg; ONE dynamic tag
        directive suppressed by __fish_seen_subcommand_from / __fish_prev_arg_in."
  critical: "fish -r is an INVERSE knob in fish 4.x: it switches the directive
             into 'complete the option's value' mode, BYPASSING the global -f.
             --search must NOT pass -r; --store MUST pass -r. Getting this backwards
             breaks the no-files guarantee. VERIFIED via complete -C."

# CONTRACT — the frozen flag matrix (the authoritative source the files mirror)
- file: main.go
  why: "parseArgs() (L214-357) + the config struct (L180-202). The EXACT flag set:
        --version/-v --help/-h --path/-p --list/-l --all/-a --file/-f --relative
        --no-color --search/-s --store, plus the check/init subcommands (L308/L329).
        usageText (L106-168) gives the canonical descriptions to mirror."
  critical: "--relative and --no-color have NO short forms. --search/-s take a
             free-text value; --store takes a directory and implies init. check/init
             are RESERVED positional tokens (exclusive, §6.3). The completions must
             list ALL of these — see the CONTRACT AMBIGUITY section."

# CONTRACT — the spec this implements
- docfile: PRD.md
  section: "§14 'Shell completions' (parity with skilldozer; complete
            subcommands/flags + tags via the binary); §6.1 (commands/flags table,
            incl. init/check); §6.2 (modifiers: --file/-f, --relative, --no-color
            — note the two with NO short form); §6.3 (exclusivity: +tags→exit 2)."
  critical: "§14 says tags via `weave --all`; the contract + skilldozer use the
             RELTAGS form `weave --relative --all` (paths relative to the extensions
             dir = the tags to complete on). Use `weave --relative --all`."

# HOW weave locates the store (why `weave --relative --all` works at completion time)
- file: internal/extdir/extdir.go
  why: "resolveSiblingFromExe (rule 3): EvalSymlinks(os.Executable())→Dir→+/extensions.
        With the install.sh symlink, `weave` finds the repo's extensions/ with zero
        config — so `weave --relative --all` returns tags at completion time without
        weave init. A stray config/env can hijack it (clean env for tests — see Gotchas)."

# EVIDENCE — every validation command was RUN; this is the transcript + rename map
- docfile: plan/001_19b4b465824d/P1M6T3S1/research/completions_reference_and_verifications.md
  why: "The verified flag matrix (main.go line-by-line), the CONTRACT AMBIGUITY
        resolution, the exhaustive rename map, the gotchas, and the validation
        gates — ALL verified on the skilldozer originals (shellcheck/zsh -n/fish
        source + bash COMP_WORDS + fish complete -C functional)."
  critical: "§1 (contract ambiguity) and §4 (which checks are syntax vs formatting —
             fish_indent --check is FORMATTING, not syntax; do NOT gate on it)."

# SIBLING TASK — owns install.sh; this task must NOT touch it
- docfile: plan/001_19b4b465824d/P1M6T2S1/PRP.md
  why: "P1.M6.T2.S1 builds install.sh, which explicitly does NOT install
        completions (only PRINTS a pointer to P1.M6.T3.S1). This task ships the
        completion FILES with header install instructions; it does NOT modify
        install.sh (avoids a write conflict with the parallel task). README §3
        cross-reference is P1.M6.T4.S1 (Mode B)."

# AUTHORITATIVE external docs for the three completion systems (gotchas reference these)
- url: https://www.gnu.org/software/bash/manual/html_node/Programmable-Completion.html
  why: "bash `complete -F` + COMPREPLY + compgen -W. Confirms the completion idiom."
- url: https://github.com/scop/bash-completion/blob/master/bash_completion
  why: "_init_completion (sets cur/prev/words/cword). ABSENT on minimal Linux / macOS
        default bash — the manual COMP_WORDS fallback is load-bearing."
- url: https://zsh.sourceforge.net/Doc/Release/Completion-System.html
  why: "#compdef, _arguments -C, the :msg: (value-taking) and ->state forms, compadd."
- url: https://fishshell.com/docs/current/completions.html
  why: "`complete -c name -f` (no files), -s/-l/-d (short/long/desc), -r (takes a
        value / file completion for the value), __fish_is_first_arg,
        __fish_seen_subcommand_from, __fish_prev_arg_in, and `complete -C` (the
        functional-test entry point)."
```

### Current Codebase tree (relevant subset)

```bash
main.go                      # parseArgs() = the FROZEN flag matrix (L214-357); usageText (L106-168)
internal/extdir/extdir.go    # sibling rule → `weave --relative --all` works with zero config (symlink install)
extensions/example.ts        # from P1.M6.T1.S1 (COMPLETE) — the tag source the completions enumerate
.gitignore                   # ignores /weave (binary), NOT completions/ — so completions ARE committed
install.sh                   # from P1.M6.T2.S1 (parallel) — does NOT install completions (prints a pointer)
completions/                 # ← DOES NOT EXIST YET (this task creates it + 3 files)
```

### Desired Codebase tree with files to be added

```bash
completions/
├── weave.bash               # ← NEW: bash completion (function _weave_completion)
├── _weave                   # ← NEW: zsh autoload completion (#compdef weave)
└── weave.fish               # ← NEW: fish completion (complete -c weave …)
# (no .go source change; no go.mod/.gitignore/PRD.md/tasks.json/install.sh change)
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the #1 contract trap): the item's parenthetical flag list omits --store
# and only mentions check. But "lockstep with main.go parseArgs()" + "port verbatim"
# BOTH require --store, check, AND init. KEEP all three (see CONTRACT AMBIGUITY section).
# Stripping them breaks parity + leaves `weave in<TAB>` completing to nothing.

# CRITICAL (bash): `_init_completion` comes from the bash-completion PACKAGE — it is
# ABSENT on minimal Linux and macOS's default bash. Without the manual COMP_WORDS
# fallback, `_init_completion || return` silently offers NOTHING. Keep the fallback.

# CRITICAL (fish -r is an INVERSE knob): in fish 4.x, `-r` switches the directive into
# "complete the option's VALUE" mode, BYPASSING the global `complete -c weave -f`.
#   --search/-s  -> NO -r (free-text query; global -f applies → offer nothing)
#   --store      -> WITH -r (it WANTS a directory; bypass -f to offer file/dir paths)
# Getting this backwards breaks the no-files guarantee. VERIFIED via `complete -C`.

# CRITICAL (shellcheck SC2207): `COMPREPLY=($(compgen …))` is the canonical
# bash-completion idiom and triggers SC2207 (3×). It is SAFE — tags/flags never
# contain spaces. Do NOT rewrite with mapfile. Add a file-level
# `# shellcheck disable=SC2207` on LINE 1 for a clean `shellcheck` exit 0
# (matches the install.sh PRP's clean-shellcheck precedent). VERIFIED exit 0.

# GOTCHA (--search vs --store are OPPOSITES in all three shells):
#   --search/-s -> free-text query -> offer NOTHING (bash: `return 0`; zsh: `:query:`;
#                  fish: no -r). weave itself enforces --search needs a value (exit 1).
#   --store     -> directory value -> COMPLETE DIRECTORIES (bash: `compgen -d`;
#                  zsh: `:directory:_files`; fish: `-r`). Mirrors main.go.

# GOTCHA (tag source is `--relative --all`, NOT `--all`): `--all` prints ABSOLUTE
# paths; `--relative --all` prints RELTAGS (paths relative to the extensions dir) —
# the short tags you want to complete on. Use `weave --relative --all`.

# GOTCHA (errors swallowed): the `weave --relative --all 2>/dev/null` call must keep
# its stderr redirect — a missing/broken weave degrades to "no tags", not a stderr
# dump into the completion menu.

# GOTCHA (functional tag tests need a clean store): the sibling rule is rule 3 —
# a stray ~/.config/weave/config.yaml or weave_EXTENSIONS_DIR makes `weave
# --relative --all` enumerate a DIFFERENT store (or none). For the Level 3 tag test,
# build weave into a temp bin and prepend it to PATH; the repo's extensions/
# (example.ts) is found via the sibling rule when the symlink/binary sits in the repo.

# GOTCHA (fish_indent --check is FORMATTING, not syntax): skilldozer.fish exits 1 on
# `fish_indent --check` (whitespace) but parses + registers fine. Do NOT gate on it.
# Gate on `fish -c 'source completions/weave.fish'` (exit 0 = parse OK) instead.

# GOTCHA (this task runs IN PARALLEL with P1.M6.T2.S1 / install.sh): do NOT modify
# install.sh — that file is owned by the parallel task and explicitly only PRINTS a
# pointer to completions. This task ships the files + header install instructions only.
```

## Implementation Blueprint

### Data models and structure

None. Three shell-completion files. They consume `weave`'s frozen flag matrix
(`main.go parseArgs()`) and its tag output (`weave --relative --all`). No Go code,
no structs, no config.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE completions/weave.bash  (port of skilldozer.bash)
  - WRITE completions/weave.bash with the EXACT content transcribed in
    "Implementation Patterns" §A below (copy-paste-ready, renames applied).
  - LINE 1: `# shellcheck disable=SC2207` (makes `shellcheck` exit 0 — see Gotchas).
  - Then the header comment block (install instructions — Mode A) + function
    `_weave_completion` + `complete -F _weave_completion weave`.
  - FLAG MATRIX (compgen -W): the full set incl. --store:
      "--version -v --help -h --path -p --list -l --all -a --file -f --relative
       --no-color --search -s --store"
  - VALUE ROUTING: --search|-s -> return 0 (free-text); --store -> compgen -d (dirs).
  - EXCLUSIVITY: walk earlier words; if check OR init seen -> return 0 (suppress).
    Offer `check init` ONLY when no positional seen yet (have_pos==0).
  - TAGS: `weave --relative --all 2>/dev/null`.
  - KEEP the `_init_completion` fallback (bash-completion package may be absent).
  - FILES TOUCHED: 1 NEW (completions/weave.bash). Create the completions/ dir.

Task 2: CREATE completions/_weave  (port of _skilldozer, zsh)
  - WRITE completions/_weave with the EXACT content transcribed in "Implementation
    Patterns" §B below. First line `#compdef weave`.
  - HEADER: install instructions (cp to ~/.zsh/completions or
    /usr/local/share/zsh/site-functions; autoload -U compinit && compinit).
  - flags=( … ) with descriptions; :query: on --search/-s; :directory:_files on --store.
  - _arguments -C "$flags[@]" '1: :->first' '*: :->rest'.
  - `first`: compadd -- "$tags[@]" check init.  `rest`: suppress tags if check/init
    in $words, else compadd tags.
  - TAGS: `weave --relative --all 2>/dev/null`, split on newlines via ${(f)…}.
  - FILES TOUCHED: 1 NEW (completions/_weave). NOTE the filename has NO extension.

Task 3: CREATE completions/weave.fish  (port of skilldozer.fish)
  - WRITE completions/weave.fish with the EXACT content transcribed in "Implementation
    Patterns" §C below. First active line `complete -c weave -f` (no file completion).
  - HEADER: install instruction (cp to ~/.config/fish/completions/weave.fish).
  - Flag matrix: 8 `-s/-l/-d` lines (version/help/path/list/all/file; relative &
    no-color have NO -s). --search/-s WITHOUT -r; --store WITH -r (the inverse knob).
  - check/init via `__fish_is_first_arg`. ONE dynamic tag directive guarded by
    `not __fish_seen_subcommand_from check init; and not __fish_prev_arg_in --search -s`,
    `-a '(weave --relative --all 2>/dev/null)' -d 'extension tag'`.
  - FILES TOUCHED: 1 NEW (completions/weave.fish).

Task 4: LINT (Level 1 — must pass before functional tests)
  - RUN: bash -n completions/weave.bash          # expect exit 0
  - RUN: shellcheck completions/weave.bash       # expect exit 0 (file-level SC2207 disable)
  - RUN: zsh -n completions/_weave               # expect exit 0
  - RUN: fish -c 'source completions/weave.fish' # expect exit 0 (parse + register)
  - IF shellcheck flags anything other than (absence of) SC2207: re-diff against the
    skilldozer original; the ported content below triggers NO codes with the disable.
  - FILES TOUCHED: 0.

Task 5: FUNCTIONAL test (Level 3 — the verification deliverable)
  - BUILD weave into a temp bin (do NOT touch the real PATH install):
      go build -o "$WORK/bin/weave" .  ;  PATH="$WORK/bin:$PATH"
  - BASH (COMP_WORDS): assert flags on `-`; check/init as first positional; empty
    after `check`; empty when prev=--search. (Exact commands in Validation Loop §L3.)
  - FISH (complete -C): assert `complete -C "weave --"` lists flags; `complete -C
    "weave "` lists tags + check + init (needs weave on PATH + extensions/ present).
  - ZSH: `zsh -n` covers syntax; interactive functional is MANUAL (cp to fpath,
    compinit, `weave <TAB>`). Document the manual steps; do not block on automation.
  - CONFIRM `go test ./...` still green (no .go files changed).
  - FILES TOUCHED: 0 (testing only; the temp bin is rm -rf'd after).
```

### Implementation Patterns & Key Details

> The three files below are the **verified port** — copy-paste-ready, renames
> applied, `--store`/`check`/`init` retained per the CONTRACT AMBIGUITY resolution,
> and the bash file carrying the line-1 `# shellcheck disable=SC2207`. Each is a
> near-verbatim port of the corresponding skilldozer file with the rename map from
> `research/completions_reference_and_verifications.md` §2.

#### §A — `completions/weave.bash`

```bash
# shellcheck disable=SC2207
# Bash completion for weave.
#
# Install (one of):
#   source /path/to/weave/completions/weave.bash
#   cp completions/weave.bash ~/.local/share/bash-completion/completions/weave
#   cp completions/weave.bash /etc/bash_completion.d/weave
#
# Tags are derived DYNAMICALLY from disk by calling `weave --relative --all`
# (weave is manifest-free, PRD §2: there is no sidecar catalog to read).
#
# LOCKSTEP: the flag set below is frozen to `main.go parseArgs()`. If a future
# task adds/renames a flag there, update this list — and the zsh/fish files —
# identically. There is no shared source of truth the shells can import.
_weave_completion() {
    local cur prev words cword
    # _init_completion (from the bash-completion package) sets cur/prev/words/cword.
    # Fall back to COMP_WORDS manually when the package is absent (minimal Linux,
    # macOS default bash) — otherwise `_init_completion || return` silently offers
    # NOTHING. SC2317 flags the fallback branch as "unreachable"; it is a false
    # positive (the branch runs whenever the helper is missing).
    _init_completion 2>/dev/null || {
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        cword=$COMP_CWORD
        words=("${COMP_WORDS[@]}")
        COMPREPLY=()
    }

    # Value-taking flags: route the value slot away from tag completion.
    #   --search/-s  -> free-text query  -> offer NOTHING (return 0 with empty COMPREPLY).
    #   --store      -> directory value  -> complete DIRECTORIES via compgen -d.
    # (--store WANTS path completion, unlike --search's free-text -> nothing.)
    case "$prev" in
        --search|-s) return 0 ;;
        --store) COMPREPLY=($(compgen -d -- "$cur")); return 0 ;;
    esac

    # Flag completion when the current token starts with '-'.
    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W \
            "--version -v --help -h --path -p --list -l --all -a --file -f --relative --no-color --search -s --store" \
            -- "$cur"))
        return 0
    fi

    # Walk earlier words: `check` AND `init` are EXCLUSIVE subcommands (PRD §6.3 —
    # either +tags → exit 2), so once one appears, offer nothing further. Track
    # whether any non-flag positional was seen so they are only ever offered
    # as the FIRST positional token.
    local i have_pos=0
    for ((i=1; i<cword; i++)); do
        [[ "${words[i]}" == "check" || "${words[i]}" == "init" ]] && return 0
        [[ "${words[i]}" == -* ]] && continue
        have_pos=1
    done

    # Tags straight from the binary (canonical relTags, one per line). Errors
    # swallowed: a missing/broken weave degrades to "no tags" instead of spewing
    # into the completion menu.
    local tags cands
    tags=$(weave --relative --all 2>/dev/null)
    cands="$tags"
    (( have_pos == 0 )) && cands="$cands check init"
    # SC2207 (mapfile preferred) is acceptable here: tags and flags never
    # contain spaces, so word-splitting is safe.
    COMPREPLY=($(compgen -W "$cands" -- "$cur"))
    return 0
}
complete -F _weave_completion weave
```

#### §B — `completions/_weave` (zsh)

```zsh
#compdef weave
# Zsh completion for weave (autoload function).
#
# Install (one of):
#   cp completions/_weave ~/.zsh/completions/_weave
#   cp completions/_weave /usr/local/share/zsh/site-functions/_weave
# then ensure `autoload -U compinit && compinit` in your .zshrc.
#
# Tags are derived DYNAMICALLY from disk by calling `weave --relative --all`
# (weave is manifest-free, PRD §2: there is no sidecar catalog to read).
#
# LOCKSTEP: the flag list below is frozen to `main.go parseArgs()`. If a future
# task adds/renames a flag there, update this list — and the bash/fish files —
# identically.
_weave() {
    local -a tags
    # Canonical relTags, one per line. ${(f)...} splits on newlines. Errors
    # swallowed: a missing/broken weave yields an empty list, not a stderr dump.
    tags=(${(f)"$(weave --relative --all 2>/dev/null)"})

    local -a flags=(
        '--version[Print the weave version]'   '-v[Print the weave version]'
        '--help[Show this help message]'      '-h[Show this help message]'
        '--path[Print the resolved extensions directory]' '-p[Print the resolved extensions directory]'
        '--list[Human-readable catalog (TAG, NAME, DESCRIPTION)]' '-l[Human-readable catalog]'
        '--all[Print every extension path, sorted by tag]' '-a[Print every extension path]'
        '--file[Print the entry file path instead of the directory]' '-f[Print the entry file path]'
        '--relative[Print paths relative to the extensions directory]'
        '--no-color[Disable ANSI color]'
        # `:query:` marks --search/-s as value-taking, so zsh routes the value
        # slot away from $state (no tag completion after them). NO completion is
        # offered for the search value (free-text query).
        '--search[Substring search over tag/name/description/keywords]:query:'
        '-s[Substring search over tag/name/description/keywords]:query:'
        # `:directory:_files` routes --store's value slot to file/path completion
        # (directories). No short form. (The OPPOSITE of --search: --store WANTS
        # path completion; --search offers nothing.)
        '--store[Non-interactive store path for init]:directory:_files'
    )

    _arguments -C "$flags[@]" '1: :->first' '*: :->rest' && return 0

    case "$state" in
        first)
            # check AND init are offered ONLY as the first positional token.
            compadd -- "$tags[@]" check init
            ;;
        rest)
            # `check` AND `init` are exclusive (PRD §6.3: either+tags → exit 2).
            # Once one is seen, suppress tags so completion never invites a
            # guaranteed error.
            if (( ${words[(I)check]} || ${words[(I)init]} )); then
                _message 'subcommand takes no tag arguments'
            else
                compadd -- "$tags[@]"
            fi
            ;;
    esac
}

_weave "$@"
```

#### §C — `completions/weave.fish`

```fish
# Fish completion for weave.
#
# Install:
#   cp completions/weave.fish ~/.config/fish/completions/weave.fish
#
# Tags are derived DYNAMICALLY from disk by calling `weave --relative --all`
# (weave is manifest-free, PRD §2: there is no sidecar catalog to read).
#
# LOCKSTEP: the flag list below is frozen to `main.go parseArgs()`. If a future
# task adds/renames a flag there, update this file — and the bash/zsh files —
# identically.

# No file completion: weave takes tags/flags, not paths.
complete -c weave -f

# Flag matrix (§6.1/§6.2). --relative and --no-color have NO short forms.
complete -c weave -s v -l version  -d 'Print the weave version'
complete -c weave -s h -l help     -d 'Show this help message'
complete -c weave -s p -l path     -d 'Print the resolved extensions directory'
complete -c weave -s l -l list     -d 'Human-readable catalog (TAG, NAME, DESCRIPTION)'
complete -c weave -s a -l all      -d 'Print every extension path, sorted by tag'
complete -c weave -s f -l file     -d 'Print the entry file path instead of the directory'
complete -c weave       -l relative -d 'Print paths relative to the extensions directory'
complete -c weave       -l no-color -d 'Disable ANSI color'
# --search/-s take a free-text query, so NO completion is offered after them.
# We deliberately do NOT pass -r here: in fish 4.x `-r` switches into
# "complete the option's value" mode, which BYPASSES the global `-f` above and
# offers file names for the query. Without -r, --search/-s are treated as plain
# flags, so after `--search ` the global `-f` (no-files) applies and nothing is
# offered -- exactly the PRD §6.1 free-text-query behavior. (fish's -r is only a
# completion hint; weave itself enforces that --search needs a value, exit 1.)
complete -c weave -s s -l search -d 'Substring search over tag/name/description/keywords'

# --store <dir> (PRD §8.2): Non-interactive store path for init. Unlike --search,
# --store's value is a DIRECTORY, so here we DO pass `-r`: in fish 4.x `-r`
# switches into "complete the option's value" mode, which BYPASSES the global
# `-f` above and offers file/dir paths for the value. This is the intentional
# INVERSE of --search's no-`-r` (free-text -> offer nothing). No short form.
complete -c weave -l store -d 'Non-interactive store path for init' -r

# `check` AND `init` are EXCLUSIVE subcommands (PRD §6.3). Offer them only as
# the first arg.
complete -c weave -n '__fish_is_first_arg' -a 'check' -d 'Validate every extension on disk'
complete -c weave -n '__fish_is_first_arg' -a 'init' -d 'First-run setup: pick/create the extensions store and write the config'

# Dynamic tags: ONE directive with command substitution (NOT a hardcoded line per
# tag — the store is manifest-free and changes as extensions are added). Suppressed
# once `check` OR `init` is seen (exclusive subcommand, PRD §6.3) AND when the
# previous arg is --search/-s (free-text query — no tag completion there either).
complete -c weave -n 'not __fish_seen_subcommand_from check init; and not __fish_prev_arg_in --search -s' \
    -a '(weave --relative --all 2>/dev/null)' -d 'extension tag'
```

### Integration Points

```yaml
PRODUCES:
  - completions/weave.bash     # bash completion (function _weave_completion)
  - completions/_weave         # zsh autoload completion (#compdef weave)
  - completions/weave.fish     # fish completion (complete -c weave …)

CONSUMES (read-only, no change):
  - the `weave` flag matrix       # main.go parseArgs() (the frozen matrix the files mirror)
  - weave tag output              # `weave --relative --all` (relTags, one per line)
  - extensions/example.ts         # P1.M6.T1.S1 — the tag source enumerated at completion time
  - extdir sibling rule           # install.sh symlink → `weave` finds the repo's extensions/ (zero config)

NO CHANGES TO:
  - any .go source (main.go, internal/*) — parseArgs() already freezes the matrix.
  - go.mod / go.sum / .gitignore (completions/ is NOT ignored — it IS committed).
  - install.sh (owned by the parallel P1.M6.T2.S1; it only PRINTS a pointer to this task).
  - README.md (Mode B install text is P1.M6.T4.S1; this task ships Mode A header comments only).
  - PRD.md / tasks.json / prd_snapshot.md (read-only).
```

## Validation Loop

> All commands below were RUN against the skilldozer originals (with the rename
> map + the line-1 SC2207 disable) and PASS. Toolchain present: `shellcheck`
> 0.11.0, `bash`, `zsh` (/usr/bin/zsh), `fish` (/usr/bin/fish). See
> `research/completions_reference_and_verifications.md` §4.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd ~/projects/weave

# bash — syntax + lint (CLEAN with the file-level SC2207 disable)
bash -n completions/weave.bash            # expect: no output, exit 0
shellcheck completions/weave.bash         # expect: exit 0, ZERO output (clean)
head -1 completions/weave.bash            # expect: # shellcheck disable=SC2207

# zsh — syntax (autoload function; no shebang — first line is #compdef weave)
zsh -n completions/_weave                 # expect: no output, exit 0
head -1 completions/_weave                # expect: #compdef weave

# fish — parse + register (the `complete` directives just register; no execution)
fish -c 'source completions/weave.fish'   # expect: no output, exit 0
# NOTE: do NOT gate on `fish_indent --check weave.fish` — that is a FORMATTING
# check (exits 1 on the verbatim port); it is NOT a syntax check.

# Confirm completions/ is tracked-able (NOT gitignored — only /weave binary is)
git check-ignore completions/weave.bash && echo "FAIL: gitignored" || echo "completions tracked-able OK"
grep -qx '/weave' .gitignore && echo ".gitignore ignores the binary, not completions/ OK"

# Expected: bash/zsh/fish all clean; first lines correct; completions/ not ignored.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Completion files are SHELL scripts, not Go — there is no Go unit test for them
# (and none is needed). The "unit test" IS Level 1 (lint/parse) + Level 3 (functional).

# Confirm the binary's own Go tests are unaffected (this task changes no .go files):
go test ./...
# Expected: all green.
```

### Level 3: Integration / Functional Testing (the verification deliverable)

Build weave into a TEMP bin and prepend it to PATH so the completions' `weave
--relative --all` call resolves to the repo's `extensions/` (via the sibling rule,
zero config). Flag/subcommand completion does NOT need the binary (hardcoded);
tag completion DOES.

```bash
set -e
cd ~/projects/weave
WORK=$(mktemp -d)
go build -o "$WORK/bin/weave" .

# --- bash functional (COMP_WORDS) ---
PATH="$WORK/bin:$PATH" bash -c '
source completions/weave.bash
COMP_WORDS=(weave --v); COMP_CWORD=1; _weave_completion
echo "flags(--v): ${COMPREPLY[*]}"                      # expect: --version
COMP_WORDS=(weave); COMP_CWORD=1; _weave_completion
echo "first positional has check/init:" $(echo "${COMPREPLY[*]}" | tr " " "\n" | grep -Eo "^(check|init)$" | tr "\n" " ")
COMP_WORDS=(weave check); COMP_CWORD=2; _weave_completion
echo "after check: [${COMPREPLY[*]}]"                   # expect: [] (suppressed)
COMP_WORDS=(weave init); COMP_CWORD=2; _weave_completion
echo "after init: [${COMPREPLY[*]}]"                    # expect: [] (suppressed)
COMP_WORDS=(weave --search ""); COMP_CWORD=2; _weave_completion
echo "after --search: [${COMPREPLY[*]}]"                # expect: [] (free-text)
'
# Expected: --version ; "check init" ; [] ; [] ; []

# --- fish functional (complete -C returns the completions for a cmdline) ---
PATH="$WORK/bin:$PATH" fish -c 'source completions/weave.fish; complete -C "weave --"' | grep -q -- "--help" && echo "fish flags OK"
PATH="$WORK/bin:$PATH" fish -c 'source completions/weave.fish; complete -C "weave "' | grep -Eq '^(check|init)$' && echo "fish first-positional OK"
PATH="$WORK/bin:$PATH" fish -c 'source completions/weave.fish; complete -C "weave "' | grep -q 'example' && echo "fish tag (example) OK"
# Expected: all three OK lines (example = the tag from extensions/example.ts).

# --- zsh: syntax covered by Level 1; interactive functional is MANUAL ---
#   cp completions/_weave ~/.zsh/completions/_weave   (or a dir on fpath)
#   autoload -U compinit && compinit
#   weave <TAB>          → tags + check + init
#   weave --<TAB>        → flags
#   weave check <TAB>    → nothing (suppressed)
# Document these manual steps in the PR; do not block CI on zsh interactivity.

rm -rf "$WORK"
# Expected: every assertion prints its OK line (all VERIFIED on the skilldozer originals).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Confirm the flag matrix in all three files matches main.go parseArgs() EXACTLY
# (the LOCKSTEP guarantee). Every flag below must appear in each file:
cd ~/projects/weave
for f in completions/weave.bash completions/_weave completions/weave.fish; do
  echo "== $f =="
  for flag in --version --help --path --list --all --file --relative --no-color --search --store; do
    grep -q -- "$flag" "$f" && echo "  $flag present" || echo "  $flag MISSING (FAIL)"
  done
  for sub in check init; do
    grep -qw -- "$sub" "$f" && echo "  $sub present" || echo "  $sub MISSING (FAIL)"
  done
done
# Expected: every flag + check + init present in all three files.

# Confirm no short forms exist for --relative / --no-color (§6.2) in the fish matrix:
grep -E 'l (relative|no-color)' completions/weave.fish | grep -v -- '-s ' && echo "no short forms for relative/no-color OK"

# Confirm the --search/--store inverse knob in fish (--search no -r; --store WITH -r):
grep -- '--store' completions/weave.fish | grep -q -- ' -r' && echo "--store has -r OK"
! { grep -E "l search.*-r|search.* -r" completions/weave.fish | grep -qv '^#'; } && echo "--search has NO -r OK"

# Expected: lockstep matrix confirmed; inverse knob correct.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `bash -n completions/weave.bash` → exit 0; `shellcheck completions/weave.bash` → exit 0 (clean, line-1 SC2207 disable).
- [ ] `zsh -n completions/_weave` → exit 0; first line `#compdef weave`.
- [ ] `fish -c 'source completions/weave.fish'` → exit 0.
- [ ] `go test ./...` stays green (no .go files changed).

### Feature Validation (the §14 gate)

- [ ] `completions/weave.bash`, `completions/_weave`, `completions/weave.fish` exist (NEW dir + 3 files).
- [ ] bash: flags complete on `-`; `check`/`init` as first positional; empty after `check`/`init`; empty after `--search`.
- [ ] fish: `complete -C "weave --"` lists flags; `complete -C "weave "` lists tags + `check` + `init`.
- [ ] zsh: `zsh -n` clean; interactive `weave <TAB>` / `weave --<TAB>` / `weave check <TAB>` verified manually.
- [ ] Flag matrix in all three == `main.go parseArgs()` (incl. `--store`, `check`, `init` — Level 4 confirms).
- [ ] NO file completion anywhere except `--store`'s directory value.

### Code Quality Validation

- [ ] All three are the verified port of skilldozer's completions (rename map applied; no "improvements").
- [ ] `--store`, `check`, `init` retained in all three (CONTRACT AMBIGUITY resolution honored).
- [ ] bash: `_init_completion` fallback retained; `COMPREPLY=($(compgen …))` idiom kept (SC2207 disabled, not rewritten).
- [ ] fish: `--search` has NO `-r`; `--store` HAS `-r` (the inverse knob).
- [ ] All three: tags from `weave --relative --all 2>/dev/null`; errors swallowed; no file completion.
- [ ] All three: header install instructions present (Mode A).
- [ ] No source-code (.go) change; no go.mod/.gitignore/install.sh/README/PRD.md/tasks.json change.

### Documentation & Deployment

- [ ] Each file's header comment has shell-specific install instructions (Mode A).
- [ ] README §3 install cross-reference is left to P1.M6.T4.S1 (Mode B); this task ships headers only.
- [ ] install.sh is NOT modified (parallel task P1.M6.T2.S1 owns it; it only PRINTS a pointer here).
- [ ] The CONTRACT AMBIGUITY (`--store`/`init` retained) and the SC2207 / fish-`-r` gotchas are recorded.

---

## Anti-Patterns to Avoid

- ❌ Don't strip `--store`/`check`/`init` to match the item's incomplete parenthetical
  list — the governing meta-principles ("verbatim port" + "lockstep with main.go
  parseArgs()") require all three; omitting `init` leaves `weave in<TAB>` dead.
- ❌ Don't rewrite `COMPREPLY=($(compgen …))` with `mapfile` to "fix" SC2207 — it is
  the canonical bash-completion idiom; add the file-level disable directive instead.
- ❌ Don't drop the `_init_completion` fallback — without it, minimal Linux / macOS
  default bash silently offers NOTHING (the helper package is absent).
- ❌ Don't pass `-r` to fish's `--search/-s`, or omit it from `--store` — `-r` is an
  INVERSE knob that bypasses the global `-f` (no-files). `--search`→no `-r` (free-text→nothing);
  `--store`→`-r` (dir completion). Getting this backwards leaks file completion.
- ❌ Don't use `weave --all` for tags — that prints ABSOLUTE paths. Use
  `weave --relative --all` (relTags) so the menu offers short tags.
- ❌ Don't drop the `2>/dev/null` on the tag call — a missing/broken weave must
  degrade to "no tags", not spew stderr into the completion menu.
- ❌ Don't add file completion (`compgen -f` / `_files` / drop the `-f`) anywhere
  except `--store`'s value slot — weave takes tags/flags, not paths.
- ❌ Don't modify install.sh, README.md, or any `.go` file — install.sh is owned by
  the parallel task (prints a pointer only); README is P1.M6.T4.S1; parseArgs()
  already freezes the matrix.
- ❌ Don't gate fish on `fish_indent --check` — that is a FORMATTING check (exits 1
  on the verbatim port), not syntax. Gate on `fish -c 'source …'`.
- ❌ Don't offer `check`/`init` as anything but the FIRST positional, or after they
  are already seen — they are exclusive (§6.3: +tags → exit 2); offering them
  again invites a guaranteed error.
