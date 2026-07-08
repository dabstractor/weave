# Research: P1.M6.T4.S1 README.md

Verified by reading the code, running the built binary, and comparing the
skilldozer README (the mandated tone/structure model). Every fact below was
confirmed against the actual `weave` binary built from the current tree.

## 1. Deliverable + scope

- ONE file: `README.md` at repo root. No README exists yet (confirmed `ls README*`
  fails). This is PRD §5 "Mode A": the README IS the documentation deliverable.
- Self-references all CLI flags, env vars, and behaviors implemented across
  M1-M6 (per the item's DOCS note).
- MUST mirror skilldozer's README tone + structure (item RESEARCH NOTE step 1).
- MUST pass the write-tech-docs linter (item RESEARCH NOTE: "no em dashes, no
  marketing tell-words, no hedging").
- Runs in PARALLEL with P1.M6.T3.S1 (completions). Treat that PRP as a contract:
  `completions/weave.bash`, `completions/_weave`, `completions/weave.fish` will
  exist. This README only REFERENCES them (Shell completions section); it does
  not create or modify them.

## 2. The em-dash conflict (RESOLVED)

Three sources collide:

- PRD §15.1 one-liner literally contains an em dash:
  "Standalone extension loader for pi — resolves an extension tag to an absolute
  path for `pi -e`."
- The binary's own `usageText` (main.go L120) uses an em dash:
  `weave — manifest-free extension path printer`.
- BUT the item RESEARCH NOTE step 1 says: "Use the write-tech-docs skill
  conventions: no em dashes, no marketing tell-words, no hedging." And
  write-tech-docs rule #1 is "No em dashes. Not once." Plus its linter
  (`scripts/lint.sh`) FAILS on U+2014 AND on " -- " (space-dash-dash-space) in prose.
- The skilldozer README one-liner uses a PERIOD (no em dash): "Standalone skill
  loader for pi. Resolves a skill tag to an absolute path for `pi --skill`."

RESOLUTION: the writing-conventions instruction (RESEARCH NOTE + skilldozer
mirror) governs HOW; it overrides the literal em dash in §15.1. Keep the words,
swap the em dash for a period (skilldozer's exact style). Final one-liner:

    Standalone extension loader for pi. Resolves an extension tag to an absolute
    path for `pi -e`.

This is a deliberate, reasoned choice. The binary's internal usageText em dash is
the binary's concern (a Go string constant, not subject to the doc linter); the
README is a separate doc that must pass lint.sh.

## 3. Exact CLI contract (verified from main.go usageText L118-168 + binary runs)

Binary: `weave`. Module: `github.com/dabstractor/weave`, go 1.25.

Commands / flags (long + short; --relative and --no-color have NO short form):

| Invocation | stdout | exit |
|---|---|---|
| `weave <tag> [<tag>...]` | one ABSOLUTE path per line, input order | 0 all resolve; 1 if ANY fail (prints nothing) |
| `weave --all` / `-a` | every extension's resolvable path, sorted by tag | 0 |
| `weave --list` / `-l` | table TAG, NAME, DESCRIPTION | 0 (1 if none) |
| `weave --search <q>` / `-s <q>` | same table, filtered | 0; 1 if no match |
| `weave check` | `OK`/`WARN`/`ERROR` report + summary | 0 clean; 1 if any ERROR |
| `weave init [<dir>]` | store path (+ check report) | 0; 1 on error/cancel |
| `weave --path` / `-p` | resolved extensions dir | 0 (1 unresolvable) |
| `weave --help` / `-h` | usage | 0 |
| `weave --version` / `-v` | `weave <version>` | 0 |

Modifiers (combine with tags or --all): `--file`/`-f` (entry file path),
`--relative` (paths relative to extensions dir), `--no-color` (no ANSI).

Exit codes: 0 success/help/version | 1 unresolved/no extensions/unresolvable dir
| 2 unknown flag / mutually-exclusive modes. No args + no flag => usage to
STDERR, exit 1. --help/--version win over everything. Mixing a <tag> with
--list/--search/--all is exit 2.

## 4. Verified output formats (ran the built binary)

- `weave example`  => `/home/dustin/projects/weave/extensions/example.ts`
- `weave -f example` => same (single-file: entry file == file)
- `weave --relative --all` => `example.ts` (relative resolvable PATH, not the
  bare tag `example`. NOTE: differs from skilldozer, where the path IS the dir
  = the tag. For weave a single-file extension's path keeps the .ts. Document
  the weave reality, not skilldozer's.)
- `weave --version` => `weave dev` (dev when no git tags; install.sh stamps
  `$(git describe --tags --always)` via ldflags)
- `weave --path` stdout = the dir; stderr = one label, EXACTLY one of:
  `weave_EXTENSIONS_DIR`, `config file`, `sibling of binary`, `ancestor of cwd`
  (printed as `(found via <label>)`)
- `weave check` => `OK    example (example)` then `1 extensions, 0 errors, 0 warnings`
- `weave init --store /tmp/x` stdout = `/tmp/x` + the check report lines;
  stderr = `Seeded example extension at /tmp/x/example.ts` + `(found via config file)`.
  (init stdout carries BOTH the store path and the check report; the seed/rule
  status goes to stderr. Document the store path is the first stdout line.)

## 5. Env vars (all LOWERCASE, case-sensitive on Linux; verified in source)

- `weave_EXTENSIONS_DIR` — store dir override (§8.3 rule 1). internal/extdir
  const `envVar = "weave_EXTENSIONS_DIR"`.
- `weave_CONFIG` — config file path override (default
  `$XDG_CONFIG_HOME/weave/config.yaml` -> `~/.config/weave/config.yaml`).
  internal/config const `configEnv = "weave_CONFIG"`.
- `weave_INSTALL_BIN` — install target bin dir override (install.sh).
- `XDG_DATA_HOME` — default store base -> `$XDG_DATA_HOME/weave/extensions`
  (-> `~/.local/share/weave/extensions`). Default config base ->
  `$XDG_CONFIG_HOME/weave` (-> `~/.config/weave`).

Config file is one key, hand-rolled YAML reader (no yaml.v3 dep): `store: /abs/path`.

## 6. Extension discovery + tag resolution (PRD §7; document the summary)

Three extension kinds:
- single FILE: `*.ts`/`*.js` whose basename is NOT `index.ts`/`index.js`.
- DIR: a directory containing `index.ts`/`index.js`.
- PACKAGE: a directory with `package.json` whose `pi.extensions` array names
  >=1 existing entry.

Recursion rule (load-bearing): once a dir is recognized as a dir/package
extension, do NOT descend into it. Only descend into PLAIN dirs (category
folders). This prevents double-counting internal .ts files.

Tag (relTag) = path relative to the extensions dir, `/` separators, with a
trailing `.ts`/`.js` stripped for single files. Examples: `gate.ts` -> `gate`;
`writing/reddit.ts` -> `writing/reddit`; `git-checkpoint/` (dir) ->
`git-checkpoint`; `writing/snippets/` (resolvable dir) -> `writing/snippets`.

Tag resolution precedence (first match wins):
1. exact canonical tag (`writing/reddit`)
2. basename / final segment (`reddit`)
3. `package.json` `name`
4. declared alias (`weave.aliases`)
5. else unknown (nothing on stdout, exit 1)

Metadata priority: `package.json` (name/description/keywords + the non-standard
`weave.aliases`/`weave.category`) > leading JSDoc `/** ... */` block in the
entry file > empty (rendered `(none)` in --list). No new file format invented.

## 7. Store resolution priority (§8.3; document verbatim)

1. `weave_EXTENSIONS_DIR` env (if set + existing dir)
2. config file `store` (set by `weave init`)
3. sibling of the running binary (symlink-aware: os.Executable + EvalSymlinks;
   this is why install.sh symlinks, not copies)
4. walk up from cwd (dev / go run)
5. none => unconfigured: stderr `weave is not configured; run \`weave init\``,
   exit 1, nothing on stdout. Bare `weave <tag>` NEVER prompts.

## 8. install.sh (P1.M6.T2.S1, COMPLETE — read it)

Builds with version ldflags, SYMLINKS (never copies) into:
`$weave_INSTALL_BIN` -> `$HOME/.local/bin` (if present/creatable) ->
`/usr/local/bin` (if writable). Prints a PATH rc-file hint for the detected
shell if the target is not on PATH. Verifies with `weave --version` + `weave
example`. Does NOT install completions (prints a pointer to P1.M6.T3.S1).

## 9. completions (P1.M6.T3.S1 contract — will exist)

`completions/weave.bash` (function `_weave_completion`),
`completions/_weave` (zsh, `#compdef weave`), `completions/weave.fish`. Tags
derived dynamically from `weave --relative --all`. README's Shell completions
section gives the source/copy commands per shell (mirror skilldozer's section).

## 10. Skilldozer README structure (the model to mirror)

1. `# Title` + one-liner (period, no em dash)
2. `## Why`
3. `## Install` (A. ./install.sh / B. go install / C. From source) + `### First run`
4. `## Shell completions` (bash/zsh/fish source-or-copy blocks)
5. `## Usage` (canonical one-liner FIRST, then commented examples, then an
   "Error contract" paragraph)
6. `## Where skills live` -> weave analog: "How extensions are organized"
7. `## Adding a skill` -> "Adding an extension"
8. `## How `skilldozer` finds the store` -> "How `weave` finds the store"
9. `## Constraints`

PRD §15 lists the same 8 sections plus "Also include a Shell Completions
section." Follow skilldozer's actual ordering (Shell completions AFTER Install,
BEFORE Usage) since the RESEARCH NOTE says mirror skilldozer's structure.

## 11. write-tech-docs linter (the validation gate)

`bash /home/dustin/.pi/agent/skills/write-tech-docs/scripts/lint.sh README.md`.
Strips fenced code blocks + inline code FIRST, then fails on:
- em dashes U+2014 AND " -- " (space-dash-dash-space) in prose
- banned tell-words (powerful, robust, elegant, seamless, comprehensive,
  leverage, utilize, unlock, empower, streamline, elevate, delve, moreover,
  furthermore, truly, incredibly, ...) whole-word case-insensitive
- any prose paragraph over 100 words (skips headings/lists/tables/quotes/code)

Implication: flags like `--all` are safe (inline code is stripped). But prose
must never contain `word -- word` or U+2014. Run until exit 0.
