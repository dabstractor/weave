# PRP — P1.M6.T4.S1: `README.md` (full §15 outline)

> **Subtask:** Write `README.md` at the repo root following PRD §15's 8-section
> outline plus a Shell Completions section. This is PRD §5 **Mode A**: the
> README *is* the documentation deliverable, self-referencing every CLI flag,
> env var, and behavior shipped across M1-M6. **Mirror skilldozer's README**
> (`/home/dustin/projects/skilldozer/README.md`) for tone and structure, and
> follow the **write-tech-docs** skill conventions. Every CLI/env fact in this
> PRP was **verified by building the binary and running it**; see
> `research/verified_facts_and_skilldozer_mirror.md`.

---

## Goal

**Feature Goal**: Produce a single `README.md` at `/home/dustin/projects/weave/README.md`
that mirrors skilldozer's README structure and tone, covers PRD §15's eight
sections plus Shell Completions, and is accurate to the **actual** `weave` binary
behavior (so the later doc-sync task P1.M6.T5.S1 passes without edits).

**Deliverable**: ONE new file, `README.md`, at the repo root. No other file is
created or modified.

**Success Definition**:
- `README.md` exists at repo root (none exists today).
- It passes the write-tech-docs linter: `bash scripts/lint.sh README.md` exits 0.
- It contains all eight PRD §15 sections + a Shell Completions section, in
  skilldozer's ordering.
- Every flag, env var, exit code, and output format it documents matches the
  verified binary behavior in this PRP (no invented commands).
- `go test ./...` stays green (no `.go` files touched).

## User Persona (if applicable)

**Target User**: a pi user who keeps a personal collection of extensions and wants
to load them into pi on demand by short tag.

**Use Case**: install weave once, drop `.ts` files under a store dir, then run
`pi -e "$(weave my-ext)"`. The README is the first thing they read to do that.

**User Journey**: read the one-liner + Why -> Install -> `weave init` -> drop a
file -> `pi -e "$(weave tag)"`. The README front-loads that path.

**Pain Points Addressed**: pi has many official discovery locations; weave gives
one centralized store that is deliberately NOT auto-discovered, loaded only via
`-e`. The README explains why that matters and how to use it.

## Why

- **Closes the §15 contract** — PRD §15 mandates the README with these sections;
  this is the doc deliverable for the whole milestone.
- **Accuracy gate for P1.M6.T5.S1** — the doc-sync task verifies "README matches
  implementation." A README that invents flags or misstates env vars fails that
  task. This PRP hands the writer every verified fact so the README is right the
  first pass.
- **Self-contained install** — `go install` users have no clone; the README is
  their only manual. It must cover `weave init`, the store concept, and
  completions without pointing at repo-only files.
- **Runs in parallel with P1.M6.T3.S1 (completions)** — this PRP treats that PRP
  as a contract: `completions/weave.bash`, `completions/_weave`,
  `completions/weave.fish` will exist. The README only references them.

## What

A `README.md` that mirrors skilldozer's structure, adapted to weave's file/dir
extension unit and `pi -e` loading. Plain markdown. No em dashes, no marketing
tell-words, no hedging (write-tech-docs rules; enforced by the linter).

### Success Criteria

- [ ] `README.md` at repo root exists (1 NEW file).
- [ ] `bash scripts/lint.sh README.md` exits 0 (no em dashes, no tell-words, no
      100-word prose paragraphs).
- [ ] All eight §15 sections present + a Shell Completions section, in this order:
      Title+one-liner, Why, Install (+First run), Shell completions, Usage,
      How extensions are organized, Adding an extension, How `weave` finds the
      store, Constraints.
- [ ] Documents every flag in the matrix below (long + short forms), every env
      var (`weave_CONFIG`, `weave_EXTENSIONS_DIR`, `weave_INSTALL_BIN`,
      `XDG_DATA_HOME`), the exit codes, and the `pi -e "$(weave tag)"` contract.
- [ ] The canonical install one-liners run as written (`./install.sh`,
      `go install github.com/dabstractor/weave@latest`, `weave init`).
- [ ] `go test ./...` still green (no source touched).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed
to implement this successfully?_ **Yes.** The deliverable is one markdown file.
The structure model (skilldozer's README) is local and read-only. Every CLI
flag, env var, exit code, and output format is transcribed below, verified by
running the built binary. The tone + formatting rules are pinned by the
write-tech-docs skill and its linter. No guessing required.

### Documentation & References

```yaml
# CONTRACT — the structure + tone model to mirror (item RESEARCH NOTE step 1)
- file: /home/dustin/projects/skilldozer/README.md
  why: "PRD §15 = 'Mirror the skilldozer README's tone and structure.' This is
        the working reference: section order, the one-liner-with-period (not em
        dash), the Install A/B/C + First-run shape, the Shell completions block,
        the Usage 'canonical one-liner first, then commented examples, then an
        Error-contract paragraph' shape, and the Constraints bullets."
  pattern: "Read it end to end before writing. Port section SHAPE and tone, swap
            the nouns: skilldozer->weave, skill->extension, --skill->-e,
            SKILL.md->entry file (.ts/.js), skills/->extensions/."
  critical: "skilldozer's one-liner uses a PERIOD, not an em dash. Do likewise.
             See 'The em-dash decision' below. Do NOT copy skilldozer's
             tag/path conflation: a weave single-file extension's path keeps
             the .ts (weave --relative --all prints example.ts, not the bare
             tag example). Document weave's reality."

# CONTRACT — the writing rules (item RESEARCH NOTE step 1) + the gate
- file: /home/dustin/.pi/agent/skills/write-tech-docs/SKILL.md
  why: "Hard rules the README must obey: no em dashes (not once), no marketing
        tell-words (powerful/robust/elegant/seamless/comprehensive/leverage/
        utilize/unlock/empower/streamline/elevate/delve/moreover/furthermore/
        truly/incredibly/...), no hedging/formulaic transitions, no narrating
        the codebase, no prose paragraph over ~100 words. Front-load the answer;
        imperative for steps; second person; specific over general."
  critical: "Run the linter (next entry) and fix every hit. The skill's README
             checklist applies: first 1-2 sentences say what it is + who it is
             for; Why before How; install copy-pasteable and tested; usage common
             case first; features one line each; license stated."

# CONTRACT — the validation gate (run until exit 0)
- file: /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh
  why: "Enforces the write-tech-docs rules on the doc. Strips fenced code blocks
        + inline code FIRST, then fails on: em dashes (U+2014) AND ' -- '
        (space-dash-dash-space) in prose; banned tell-words (whole-word, ci); any
        prose paragraph over 100 words."
  critical: "Flags like --all are SAFE (inline code is stripped before checking).
             But prose must never contain 'word -- word' or U+2014. 'license
             stated plainly' is fine. Run: bash scripts/lint.sh README.md"

# CONTRACT — the frozen CLI matrix + canonical usage text (verified)
- file: main.go
  why: "usageText (L118-168) is the authoritative flag list + descriptions + exit
        codes. parseArgs() (L214-357) freezes the matrix. version var (L43) is
        stamped by install.sh ldflags. The README's flag list MUST match this
        exactly."
  pattern: "Mirror the flag set, short forms, and the 'mutually exclusive modes
            => exit 2' + 'no args => stderr, exit 1' rules."
  critical: "--relative and --no-color have NO short forms. check and init are
             RESERVED positional subcommands (they never resolve as tags)."

# CONTRACT — exact env var names (lowercase, case-sensitive on Linux)
- file: internal/config/config.go
  why: "configEnv = 'weave_CONFIG' (L108); default store base uses XDG_DATA_HOME
        (L159). Documents the config-file path + the one-key YAML store."
- file: internal/extdir/extdir.go
  why: "envVar = 'weave_EXTENSIONS_DIR' (L74). Source labels (L58-64): exactly
        'weave_EXTENSIONS_DIR', 'config file', 'sibling of binary', 'ancestor of
        cwd'. --path prints the dir to stdout and '(found via <label>)' to stderr."

# CONTRACT — install.sh (P1.M6.T2.S1, COMPLETE)
- file: install.sh
  why: "Documents the three install facts the README's Install section states:
        builds with version ldflags, SYMLINKS (never copies) into
        $weave_INSTALL_BIN / ~/.local/bin / /usr/local/bin, prints a PATH hint,
        verifies with weave --version + weave example, does NOT install
        completions (prints a pointer)."

# CONTRACT — completions (P1.M6.T3.S1, parallel — treat as contract)
- docfile: plan/001_19b4b465824d/P1M6T3S1/PRP.md
  why: "Defines the three completion files the README's Shell completions section
        references: completions/weave.bash, completions/_weave, completions/weave.fish.
        Tags derive from weave --relative --all. README only REFERENCES these
        paths + the per-shell source/copy commands; it does not create them."

# CONTRACT — the spec this implements (read-only)
- docfile: PRD.md
  section: "§15 (README outline, 8 sections + completions), §1 (goal/why),
            §2 (constraints), §6 (CLI contract), §7 (discovery + tag resolution),
            §8 (store location), §9 (check), §10 (metadata conventions),
            §11 (example extension), §12 (install)."
  critical: "§15.1's one-liner literally contains an em dash, but the RESEARCH
             NOTE + write-tech-docs override it. See 'The em-dash decision'."

# EVIDENCE — every fact below was verified by running the built binary
- docfile: plan/001_19b4b465824d/P1M6T4S1/research/verified_facts_and_skilldozer_mirror.md
  why: "Verified CLI outputs, env vars, exit codes, label strings, check/init
        formats, the em-dash resolution, and the skilldozer section map."

# EXTERNAL — what `pi -e` does (so the Why + Usage ring true)
- url: https://pi.dev
  why: "pi loads an extension from an absolute path via `pi -e <path>`. weave's
        whole job is to turn a tag into that path. The README states this plainly."
```

### Current Codebase tree (relevant subset)

```bash
main.go                      # usageText (L118-168) = the frozen flag matrix; version var (L43)
internal/config/config.go    # weave_CONFIG, one-key YAML store, XDG defaults
internal/extdir/extdir.go    # weave_EXTENSIONS_DIR + the four --path labels
internal/check/check.go      # `weave check` OK/WARN/ERROR report + summary
extensions/example.ts        # the one shipped example (P1.M6.T1.S1, COMPLETE)
install.sh                   # symlink installer (P1.M6.T2.S1, COMPLETE)
completions/                 # ← created by P1.M6.T3.S1 (parallel): weave.bash/_weave/weave.fish
README.md                    # ← DOES NOT EXIST YET (this task creates it)
LICENSE                      # exists; README should name the license
```

### Desired Codebase tree with files to be added

```bash
README.md                    # ← NEW: full §15 README, mirrors skilldozer, lint-clean
# (no other file created or modified)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL — the em-dash decision (RESOLVED): PRD §15.1's one-liner contains
     an em dash AND the binary's usageText (main.go L120) does too. BUT the item
     RESEARCH NOTE step 1 says "Use the write-tech-docs skill conventions: no em
     dashes", write-tech-docs rule #1 is "No em dashes. Not once.", and the
     linter FAILS on U+2014. skilldozer's one-liner uses a PERIOD. RESOLUTION:
     keep the words, use a period: "Standalone extension loader for pi. Resolves
     an extension tag to an absolute path for `pi -e`." The binary's internal
     em dash is its own concern (Go string constant, not the doc); the README
     is a doc that must pass lint.sh. This is deliberate, not an oversight. -->

<!-- CRITICAL — the linter also flags " -- " (space-dash-dash-space) in PROSE.
     Inline code is stripped first, so flags like `--all` in backticks are safe.
     But never write "word -- word" or "go install -- first-class" in prose. Use
     a period, colon, or parentheses. -->

<!-- CRITICAL — weave is NOT a drop-in for skilldozer's tag/path wording.
     skilldozer's `--all` prints skill DIRECTORIES, and a skill dir's relative
     path IS its tag, so "the path is the tag" is true there. For weave, a
     single-file extension's resolvable PATH keeps the .ts: `weave --relative
     --all` prints `example.ts`, while the TAG is `example`. Document the weave
     reality in Usage and in How extensions are organized. Do not copy
     skilldozer's "the tag is the directory" framing verbatim. -->

<!-- CRITICAL — env var names are LOWERCASE and case-sensitive on Linux:
     `weave_CONFIG`, `weave_EXTENSIONS_DIR`, `weave_INSTALL_BIN`. Do NOT write
     them as WEAVE_CONFIG etc. Verified in internal/config and internal/extdir. -->

<!-- GOTCHA — `weave init` stdout carries BOTH the store path AND the check
     report; stderr carries the seed/rule status. So "the configured store path
     is the first stdout line" is the precise, scriptable claim. Do not promise
     "init prints only the store path to stdout" (that is skilldozer's wording;
     weave also emits the check report on stdout). -->

<!-- GOTCHA — bare `weave <tag>` NEVER prompts. If unconfigured it prints to
     stderr exactly `weave is not configured; run \`weave init\``, exits 1, and
     prints nothing to stdout. State this in Usage (Error contract) and in How
     weave finds the store: it is why `pi -e "$(weave x)"` fails loudly instead
     of hanging. -->

<!-- GOTCHA — install.sh does NOT install completions (it prints a pointer to
     P1.M6.T3.S1). The README's Shell completions section gives the manual
     source/copy commands. Do not claim install.sh sets up completions. -->

<!-- GOTCHA — this task runs in PARALLEL with P1.M6.T3.S1 (completions). Do NOT
     create or edit completions/* ; only reference the three paths the sibling
     PRP defines. No write conflict because you touch only README.md. -->

<!-- STYLE — no prose paragraph over 100 words (the linter fails on it). Keep
     paragraphs to ~4 sentences. Use lists/tables/code blocks for anything
     literal. One job per section. Specific versions/paths, not "latest". -->
```

## Implementation Blueprint

### Data models and structure

None. One markdown file. The "model" is the section outline below (mirrors
skilldozer + PRD §15). Write it top to bottom in this order.

### The em-dash decision (apply throughout)

The one-liner, the tagline, and ALL prose use a period / colon / parentheses
instead of an em dash or " -- ". Example applied:

> Standalone extension loader for pi. Resolves an extension tag to an absolute
> path for `pi -e`.

Rationale (record in your head, do not put in the README): the item RESEARCH
NOTE + write-tech-docs skill + skilldozer mirror all override the literal em
dash in PRD §15.1. The linter enforces it.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: WRITE README.md — Title + one-liner + Why
  - FIRST LINE: `# weave` (heading), then a blank line, then the one-liner as a
    plain paragraph (NOT a heading): "Standalone extension loader for pi. Resolves
    an extension tag to an absolute path for `pi -e`." (period, NO em dash).
  - `## Why` (one short paragraph + a code block): weave gives a centralized,
    on-disk catalog of pi extensions addressed by short tags. The catalog lives
    DELIBERATELY outside every directory pi scans, so extensions never enter your
    context automatically. They load ONLY on demand via `pi -e`:
        pi -e "$(weave example)"
    If a tag is unknown, weave prints nothing and exits 1, so the `$(...)` fails
    loudly instead of handing pi an empty path. (Mirror skilldozer's Why shape.)
  - FILES TOUCHED: 1 NEW (README.md). Create the file.

Task 2: APPEND `## Install` (three paths) + `### First run`
  - Lead one line: "Three paths. `./install.sh` is recommended."
  - **A. `./install.sh` (recommended)** — a `./install.sh` fenced block, then one
    short paragraph: it builds the binary with version info and SYMLINKS it into
    `~/.local/bin` (or `$weave_INSTALL_BIN`, or `/usr/local/bin` if that is what
    is writable). Stress the symlink (not copy): weave resolves its own
    executable path back through the symlink, which is how it finds the adjacent
    `extensions/` dir with no env var. (Mirror skilldozer's A block + paragraph.)
  - **B. `go install`** — `go install github.com/dabstractor/weave@latest` in a
    block. One paragraph: lands in `$(go env GOPATH)/bin`. On first use run
    `weave init` (creates the store, writes the config). No clone, no
    `weave_EXTENSIONS_DIR` needed for normal use.
  - **C. From source** — `go build -o weave . && ./weave example` plus the
    optional self-symlink (`ln -sfn "$PWD/weave" ~/.local/bin/weave`). Run
    `./weave example` from the repo, or use the symlink from anywhere.
  - `### First run` — `weave init` in a block, then the flow: it prompts for the
    dir where weave should keep your extensions (default
    `$XDG_DATA_HOME/weave/extensions`, or the cwd if it already looks like a
    store), creates it, seeds an `example.ts` template if empty, writes the
    config pointing at it. Non-interactive forms:
        weave init /path/to/store
        weave init --store /path/to/store
    State the scriptable contract precisely: the configured store path is the
    FIRST line on stdout (the check report also prints to stdout; the
    seeded/adopted status and the discovery rule go to stderr). A leading `~`
    expands to your home dir. (Mirror skilldozer's First run; adapt the stdout
    detail to weave's reality per the gotcha.)
  - FILES TOUCHED: 0 (appending to the file created in Task 1).

Task 3: APPEND `## Shell completions`
  - One intro paragraph: weave ships dynamic completions for bash, zsh, and fish.
    Tag completion is not a static list; the shell calls `weave --relative --all`
    at completion time, so it never goes stale as you add extensions.
  - NOTE line: install.sh does not install completions; copy the file you want.
  - **bash** (one of): `source /path/to/weave/completions/weave.bash`;
    `cp completions/weave.bash ~/.local/share/bash-completion/completions/weave`;
    `cp completions/weave.bash /etc/bash_completion.d/weave`.
  - **zsh** (one of): `cp completions/_weave ~/.zsh/completions/_weave`;
    `cp completions/_weave /usr/local/share/zsh/site-functions/_weave`; then
    `autoload -U compinit && compinit` in `.zshrc`.
  - **fish**: `cp completions/weave.fish ~/.config/fish/completions/weave.fish`.
  - (Mirror skilldozer's Shell completions block; rename files to weave's three.)
  - FILES TOUCHED: 0.

Task 4: APPEND `## Usage` (canonical one-liner FIRST, then commented examples,
        then an Error contract paragraph)
  - Canonical one-liner in its own fenced block FIRST:
        pi -e "$(weave example)"
  - Then one big commented code block mirroring skilldozer's, covering (use the
    EXACT flags; keep comments short, imperative, no tell-words):
        # Resolve a tag to an absolute path (default: the resolvable path)
        weave example                       # -> /.../extensions/example.ts
        # Print the entry file path instead (-f / --file)
        weave -f example
        # Load several extensions into pi in one command
        pi -e "$(weave writing/reddit)" -e "$(weave example)"
        # Resolve multiple tags at once (one path per line, input order)
        weave example writing/reddit
        # Human-readable catalog and substring search
        weave --list
        weave --search reddit
        # Print every extension path, sorted by tag
        weave --all
        # Validate every extension on disk
        weave check
        # Where is the resolved extensions directory? (rule prints to stderr)
        weave --path
        # Print paths relative to the extensions directory instead of absolute
        weave --relative example
        # Disable ANSI color even on a TTY (for --list / --search tables)
        weave --no-color --list
        # Version is the git-describe value (dynamic, not a fixed string)
        weave --version
        # Short flags combine (-af) and long flags accept --flag=value (--search=reddit)
    NOTE: the comment text `# -> /.../...` uses an ASCII arrow, not an em dash.
  - Then an **Error contract** paragraph (mirror skilldozer's): an unknown tag
    prints NOTHING to stdout and exits 1 (the error goes to stderr only), so
    `pi -e "$(weave badtag)"` fails loudly instead of loading nothing. When
    multiple tags are given, any unresolved tag causes nothing to be printed and
    exit 1, so pi never sees a partial result. `--path`, `--list`, `--search`,
    and `--all` are mutually exclusive: combining any two exits 2, as does
    combining a tag with any of them. Bare `weave <tag>` never prompts: if
    unconfigured, it prints `weave is not configured; run \`weave init\`` to
    stderr, writes nothing to stdout, and exits 1. `weave --help` lists every flag.
  - FILES TOUCHED: 0.

Task 5: APPEND `## How extensions are organized`
  - One paragraph: extensions live in the `extensions/` directory at the store
    root. An extension is ONE of three kinds (use a list):
      - a single FILE: a `*.ts` or `*.js` file whose base name is NOT `index.ts`/`index.js`
      - a DIR: a directory that directly contains `index.ts` or `index.js`
      - a PACKAGE: a directory with a `package.json` whose `pi.extensions` array
        names at least one existing entry
  - The canonical **tag** is the entry's path relative to `extensions/`, with `/`
    separators, and a trailing `.ts`/`.js` STRIPPED for single files. It is NOT
    the `package.json` `name`. A worked example in a fenced block (port skilldozer's):
        extensions/gate.ts                    -> tag gate
        extensions/writing/reddit-poster.ts   -> tag writing/reddit-poster
        extensions/git-checkpoint/ (dir)      -> tag git-checkpoint
        extensions/summarizer/ (package)      -> tag summarizer
  - Recursion rule (one sentence): once a directory is recognized as a dir or
    package extension, weave does NOT descend into it (its internal `.ts` files
    are one unit); only plain directories (category folders) are descended.
  - Tag resolution tries, in order (numbered list, mirror skilldozer's):
      1. the exact canonical tag (`writing/reddit-poster`)
      2. the final path segment / basename (`reddit-poster`)
      3. the `package.json` `name`
      4. a declared alias (see `weave.aliases`)
      5. else: unknown
    One line: so `weave gate`, `weave writing/reddit-poster`, `weave
    reddit-poster` (if unique), and `weave @my-org/summarizer` (matching a
    `package.json` name) all resolve.
  - Reserved-tag note (port skilldozer's): `check` and `init` are subcommand
    names, so they never resolve as tags. `weave check` runs validation; `weave
    init` runs first-run setup. An extension whose tag collides is still fully
    usable via a nested path, its `package.json` name, or an alias; it appears in
    `--list` and `--all`.
  - FILES TOUCHED: 0.

Task 6: APPEND `## Adding an extension`
  - One line: drop a `.ts` file or a directory under `extensions/`.
  - For a single file, show a minimal template with a leading JSDoc block (weave
    pulls the description from the leading `/** ... */` when there is no
    `package.json`). Example block (mirrors extensions/example.ts shape):
        /**
         * One or two sentences describing what the extension does.
         */
        import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
        export default function (pi: ExtensionAPI) { /* ... */ }
  - For a dir/package, optional `package.json` for metadata: show the standard
    fields `name`, `description`, `keywords`, plus the non-standard `weave`
    object (`weave.aliases`, `weave.category`). One line each: `name` is a
    resolution fallback + the `--list` NAME column; `description` is the `--list`
    DESCRIPTION column (JSDoc used when absent); `keywords`/`weave.category`/
    `weave.aliases` feed `--search` and resolution. Unknown keys are ignored.
  - Close: `extensions/example.ts` is a copy-pasteable template; start from it.
    When done, validate: `weave check` (block):
        weave check
        # OK    example (example)
        # 1 extensions, 0 errors, 0 warnings
  - FILES TOUCHED: 0.

Task 7: APPEND `## How `weave` finds the store`
  - One line: weave locates `extensions/` by this priority (first hit wins).
    Numbered list (mirror skilldozer's exactly, rename env vars):
      1. `weave_EXTENSIONS_DIR` env var: override; if set and an existing dir, use it.
      2. Config file `store`: the primary, set by `weave init`. The config lives
         at `$XDG_CONFIG_HOME/weave/config.yaml` (-> `~/.config/weave/config.yaml`);
         override the file path with `weave_CONFIG=<file>`. Minimal valid file:
             store: /home/you/extensions
         A missing/unreadable config is "not yet configured" and falls through, never an error.
      3. Sibling of the running binary (symlink-aware: `os.Executable()` plus
         `EvalSymlinks`): lets a clone-and-build dev workflow work with no config.
         This is the rule a `./install.sh` symlink relies on; a copy breaks it.
      4. Walk up from `cwd`: for `go run` / dev.
      5. None: unconfigured. weave prints `weave is not configured; run \`weave init\``
         to stderr, writes nothing to stdout, exits 1.
  - `weave --path` reports the winning directory on stdout and the matching rule
    on stderr, one of: `weave_EXTENSIONS_DIR`, `config file`, `sibling of binary`,
    `ancestor of cwd`. One sentence on why the stderr label matters: a typo'd
    `weave_EXTENSIONS_DIR` is silently ignored and discovery falls through, so
    `--path` is the only way to tell the env var was skipped.
  - FILES TOUCHED: 0.

Task 8: APPEND `## Constraints`
  - Mirror skilldozer's bullet list, adapted. Bullets:
      - No catalog index. There is no `extensions.json`; the catalog is always
        walked from disk on each call. A settings config file (the store
        location, written by `weave init`) is expected and fine; catalog data
        already on disk is never duplicated into a sidecar.
      - Never auto-discovered by pi. The store does NOT live in any directory pi
        scans. It is never: `~/.pi/agent/extensions`, a project `.pi/extensions`,
        a `node_modules` package, a `package.json` with a `pi.extensions` field
        consumed by pi's own discovery, or any path pi would auto-discover.
      - Loaded only via `-e`. An extension enters your context only when you ask:
        `pi -e "$(weave <tag>)"`.
      - weave only ever prints paths. It never copies or installs extensions into
        `~/.pi/...`; where the path points is up to you.
      - Zero runtime dependencies. Build-time needs Go; the binary stands alone.
  - Optional final line: state the license (the repo has a LICENSE file). Keep it
    one line, plain (e.g. "Licensed under the MIT License." if LICENSE is MIT;
    verify by reading LICENSE).
  - FILES TOUCHED: 0.

Task 9: LINT (the gate) + manual accuracy sweep
  - RUN: bash /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh README.md
  - EXPECT: exit 0. If it reports hits: em dash -> swap for period/colon/" -- "
    for a period; tell-word -> cut or replace with evidence; >100-word paragraph
    -> split into a list or two shorter paragraphs. Re-run until exit 0.
  - MANUAL ACCURACY SWEEP (so P1.M6.T5.S1 passes): re-read the README against the
    verified matrix in research/verified_facts_and_skilldozer_mirror.md §3-5.
    Confirm every flag/env var/exit code/output matches. Confirm no em dashes
    survived anywhere (the linter catches them, but double-check headings/tables,
    which the linter also scans after stripping code).
  - RUN: go test ./...   (EXPECT green; this task changes no .go files.)
  - FILES TOUCHED: 0.
```

### Implementation Patterns & Key Details

```markdown
<!-- One-liner pattern (period, NOT em dash; mirrors skilldozer): -->
# weave

Standalone extension loader for pi. Resolves an extension tag to an absolute
path for `pi -e`.

<!-- Why pattern: fact + the failure-loud contract in a code block. -->
## Why

weave gives you a centralized, on-disk catalog of pi extensions addressed by
short tags. The catalog lives deliberately outside every directory pi scans,
so extensions never enter your context automatically. They load only on demand:

    pi -e "$(weave example)"

If a tag is unknown, weave prints nothing and exits 1, so the `$(...)` fails
loudly instead of handing pi an empty path.

<!-- Usage: canonical one-liner FIRST, then a commented block, then the Error
     contract paragraph. Comments use ASCII `#` and `->`, never em dashes. -->
<!-- Install: three labelled sub-blocks (A/B/C) + a First run sub-section.
     Stress the symlink (load-bearing for sibling-of-binary resolution). -->
<!-- Store section: the five-rule priority list verbatim; rename env vars to
     weave_EXTENSIONS_DIR / weave_CONFIG. -->
<!-- Constraints: bullet list, no index file, never auto-discovered, -e only,
     prints paths only, zero runtime deps. -->
```

### Integration Points

```yaml
PRODUCES:
  - README.md                     # the doc deliverable (Mode A)

CONSUMES (read-only reference; no change):
  - the skilldozer README         # /home/dustin/projects/skilldozer/README.md (structure + tone model)
  - write-tech-docs SKILL.md      # the writing rules
  - scripts/lint.sh               # the validation gate
  - main.go usageText             # the frozen flag matrix (L118-168)
  - internal/config, internal/extdir  # env var names + --path labels
  - install.sh                    # install facts (P1.M6.T2.S1, COMPLETE)
  - extensions/example.ts         # the shipped template (P1.M6.T1.S1, COMPLETE)
  - completions/*                 # the three files (P1.M6.T3.S1, parallel CONTRACT)

NO CHANGES TO:
  - any .go source, go.mod, .gitignore, install.sh, completions/*, LICENSE,
    PRD.md, tasks.json, prd_snapshot.md. This task writes ONLY README.md.
```

## Validation Loop

### Level 1: Syntax & Style (the mandatory gate)

```bash
cd ~/projects/weave

# THE gate: the write-tech-docs linter. Strips code/inline-code, then fails on
# em dashes (U+2014), " -- " in prose, banned tell-words, and >100-word paragraphs.
bash /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh README.md
# Expected: exit 0, zero hits. Fix every hit and re-run until clean.

# Belt-and-suspenders em-dash sweep (the linter already covers this; confirm
# headings/tables too, since the linter strips code but scans the rest):
! grep -nP '\x{2014}' README.md && echo "no em dashes OK"
# Expected: "no em dashes OK".

# Markdown sanity (a parser should accept it; not a hard gate, but catch typos):
# (only if a markdown linter is installed; skip silently otherwise)
command -v markdownlint >/dev/null && markdownlint README.md || true

# Expected: linter exit 0; no em dashes; markdown parses.
```

### Level 2: Unit Tests (Component Validation)

```bash
# README is prose; there is no Go unit test for it. Confirm the binary's own
# tests are unaffected (this task changes no .go files):
cd ~/projects/weave
go test ./...
# Expected: all green.
```

### Level 3: Integration / Manual Accuracy Sweep (so P1.M6.T5.S1 passes)

```bash
cd ~/projects/weave

# Build the binary the README documents, then confirm every command the README
# shows actually behaves as written. (Commands copied from the README's blocks.)
go build -o /tmp/weave-readme .

# Every flag the README lists must be a real flag (usageText parity). For each
# flag token the README mentions in prose/code, confirm it appears in --help:
/tmp/weave-readme --help > /tmp/w_help.txt
for f in --all -a --list -l --search -s --file -f --relative --no-color \
         --path -p --help -h --version -v --store check init; do
  grep -qw -- "$f" /tmp/w_help.txt || echo "README references $f but --help lacks it (CHECK)"
done
# Expected: no CHECK lines (every referenced flag is real).

# The canonical one-liner from the README must resolve a real extension:
/tmp/weave-readme example        # -> /.../extensions/example.ts, exit 0

# init non-interactive form from the README must work and print the store path:
/tmp/weave-readme init --store /tmp/wv-readme-store 2>/dev/null | head -1
# Expected: /tmp/wv-readme-store (first stdout line).

# --path label strings must match what the README's store section lists:
/tmp/weave-readme --path 2>&1 1>/dev/null   # one of the four labels

# Env var names in the README must match the source (lowercase):
grep -rhoE 'weave_[A-Z_]+' README.md | sort -u   # should be only weave_CONFIG,
                                                 # weave_EXTENSIONS_DIR, weave_INSTALL_BIN

rm -f /tmp/weave-readme /tmp/w_help.txt; rm -rf /tmp/wv-readme-store
# Expected: every command behaves as the README claims.
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd ~/projects/weave

# Structure parity with skilldozer: the same section headings (renamed) in the
# same order. Confirm the eight §15 sections + Shell completions are present:
for h in "## Why" "## Install" "### First run" "## Shell completions" "## Usage" \
         "## How extensions are organized" "## Adding an extension" \
         "## How \`weave\` finds the store" "## Constraints"; do
  grep -qF "$h" README.md && echo "OK: $h" || echo "MISSING: $h"
done
# Expected: every heading present.

# License is stated (write-tech-docs README checklist). Verify LICENSE type first:
head -1 LICENSE
grep -qi 'license' README.md && echo "license stated OK" || echo "state the license"

# No invented commands: every fenced `weave ...` / `pi -e ...` line is real
# (Level 3 already ran the canonical ones; this is a reminder to eyeball the rest).

# Expected: structure parity, license stated, no invented commands.
```

## Final Validation Checklist

### Technical Validation

- [ ] `bash scripts/lint.sh README.md` exits 0 (no em dashes, no tell-words, no
      long prose paragraphs).
- [ ] `! grep -nP '\x{2014}' README.md` (no em dashes anywhere).
- [ ] `go test ./...` stays green (no `.go` files changed).

### Feature Validation

- [ ] `README.md` exists at repo root (1 NEW file; none existed before).
- [ ] All eight §15 sections + Shell completions present, in skilldozer's order.
- [ ] Every flag/env var/exit code matches the verified matrix (Level 3 confirms).
- [ ] `weave example`, `weave init --store <dir>`, `weave --path` behave as documented.
- [ ] Canonical install one-liners (`./install.sh`,
      `go install github.com/dabstractor/weave@latest`, `weave init`) are real.
- [ ] The em-dash decision is applied (period in the one-liner, no em dashes).

### Code Quality Validation

- [ ] Mirrors skilldozer's structure and tone (section order, one-liner shape,
      Install A/B/C + First run, Usage canonical-first + Error contract).
- [ ] Documents weave's reality, not skilldozer's tag/path conflation (a
      single-file extension's `--relative` path keeps the `.ts`).
- [ ] Env var names lowercase and exact (`weave_CONFIG`,
      `weave_EXTENSIONS_DIR`, `weave_INSTALL_BIN`).
- [ ] No marketing tell-words, no hedging, no narrating the codebase.
- [ ] No prose paragraph over ~100 words; lists/tables/code used for literals.

### Documentation & Deployment

- [ ] Self-references every CLI flag, env var, and behavior (Mode A).
- [ ] install.sh is described accurately (symlink, not copy; no completions install).
- [ ] Completions section references the three `completions/*` paths (P1.M6.T3.S1
      contract) with per-shell source/copy commands.
- [ ] License stated plainly.
- [ ] Accurate enough that P1.M6.T5.S1 (doc sync) needs no README edits.

---

## Anti-Patterns to Avoid

- ❌ Don't use an em dash (in the one-liner or anywhere). The RESEARCH NOTE +
  write-tech-docs + skilldozer mirror all override PRD §15.1's literal em dash.
  Use a period. The linter fails on U+2014 and on " -- " in prose.
- ❌ Don't copy skilldozer's "the tag IS the directory / the path" framing. For
  weave, a single-file extension's `--relative` path keeps the `.ts`; the TAG is
  the `.ts`-stripped name. Document weave's reality or P1.M6.T5.S1 will flag it.
- ❌ Don't write env vars uppercase (`WEAVE_CONFIG`). They are lowercase and
  case-sensitive on Linux: `weave_CONFIG`, `weave_EXTENSIONS_DIR`,
  `weave_INSTALL_BIN`.
- ❌ Don't claim `weave init` prints "only" the store path to stdout. It also
  prints the check report on stdout; the store path is the FIRST line. State it
  precisely.
- ❌ Don't claim install.sh installs completions. It does not (it prints a
  pointer). The README's Shell completions section gives manual source/copy steps.
- ❌ Don't invent flags, paths, or commands. Every fenced command must be real;
  Level 3 confirms. If unsure, run it against the built binary first.
- ❌ Don't create or edit any file other than README.md (parallel task P1.M6.T3.S1
  owns completions/*; install.sh is owned by P1.M6.T2.S1).
- ❌ Don't narrate the codebase. The reader has the code. Document what it is, why
  it exists, how to use it, and the gotchas. Then stop.
- ❌ Don't write marketing prose. "powerful/seamless/robust/comprehensive" get cut
  by the linter. Replace adjectives with a command or a measured fact.
- ❌ Don't skip the linter. It is the gate. Run it, fix hits, re-run until exit 0.
