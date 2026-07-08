# P1.M6.T5.S1 — Verified ground truth & drift findings

> Purpose: the frozen facts the doc-sweep auditor checks the README against, plus
> the drifts already found during PRP research. Every fact was verified by
> reading source AND running the built binary. The auditor should re-confirm the
> drifts independently (reproduction commands below) before fixing.

## 0. State at research time

- `README.md` ALREADY EXISTS at repo root (11116 bytes, written by the parallel
  task P1.M6.T4.S1). It is HIGHLY accurate. This task is a SURGICAL verification
  + fix pass, not a rewrite.
- All implementation subtasks P1.M1-P1.M6 are complete/landing. The binary builds
  and `go test ./...` is green (no source touched in research).
- `go.mod` module path: `github.com/dabstractor/weave` (so
  `go install github.com/dabstractor/weave@latest` is correct).

## 1. Frozen CLI matrix (source of truth: main.go usageText L118-168 + config struct)

Long form / short form / value-taking / reserved-subcommand, verified against
`weave --help` and parseArgs:

| Token            | Kind            | Notes |
|------------------|-----------------|-------|
| `<tag> [...]`    | positional     | one abs path per line, input order |
| `--all` / `-a`   | mode            | every path, sorted by tag; exit 0 even empty |
| `--list` / `-l`  | mode            | table TAG/NAME/DESCRIPTION; exit 1 if empty |
| `--search`/`-s`  | mode+value      | substring over 6 fields; exit 1 if no match |
| `check`          | reserved subcmd | validate; exit 1 if any ERROR |
| `init` `[<dir>]` | reserved subcmd | first-run setup |
| `--store` `<dir>`| value (init)    | implies init; no short form |
| `--path` / `-p`  | mode            | dir→stdout, rule→stderr |
| `--file` / `-f`  | modifier        | entry file path; combines with tag/--all |
| `--relative`     | modifier        | NO short form; paths rel to extensions dir |
| `--no-color`     | modifier        | NO short form; disables ANSI on TTY |
| `--help` / `-h`  | mode            | usage to STDOUT, exit 0; wins over everything |
| `--version`/`-v` | mode            | `weave <version>\n` |

Exit codes: `0` success/help/version | `1` unresolved/empty/unconfigured |
`2` unknown flag / mutually-exclusive modes. (main.go run() L1.1/1.2.)

## 2. Env vars (verified by grep, all LOWERCASE, case-sensitive on Linux)

| Env var                 | Source                                      | Effect |
|-------------------------|---------------------------------------------|--------|
| `weave_EXTENSIONS_DIR`  | internal/extdir/extdir.go L74 `envVar`      | rule-1 store override |
| `weave_CONFIG`          | internal/config/config.go L108 `configEnv`  | config-file path override |
| `weave_INSTALL_BIN`     | install.sh `${weave_INSTALL_BIN:-}`         | install symlink target override |
| `XDG_DATA_HOME`         | config.go DefaultStore                      | default store base |
| `XDG_CONFIG_HOME`       | config.go Path (via os.UserConfigDir)       | config home |

`--path` stderr rule labels (extdir Source.String(), EXACT strings):
`weave_EXTENSIONS_DIR`, `config file`, `sibling of binary`, `ancestor of cwd`.

## 3. Canonical outputs (verified by running the binary)

- `weave --version` → `weave dev` (ldflags override at build).
- `weave --path` → dir to stdout; `(found via <label>)` to stderr.
- `weave check` per-line: `%-5s %s (%s)\n` → e.g. `OK    example (example)`
  (OK + 3 spaces padding + 1 separator). Summary: `%d extensions, %d errors, %d warnings\n`
  → e.g. `1 extensions, 0 errors, 0 warnings` (plural even for 1).
- `weave init`: store path = FIRST stdout line, then check report on stdout;
  `Seeded ...`/`Adopted ...` + `(found via ...)` on stderr.
- Error contract: unknown tag → NOTHING on stdout, error to stderr, exit 1.
- License: MIT (LICENSE, Copyright (c) 2026 Dustin Schultz).

## 4. DRIFTS FOUND (verify + fix)

### DRIFT 1 (HIGH) — README's `~` expansion claim is FALSE

README "First run" says: *"A leading `~` (or a bare `~`) in a typed answer or a
`--store`/positional path expands to your home directory."*

Implementation (main.go resolveStore L1015): `abs, err := filepath.Abs(store)`.
`filepath.Abs` does NOT expand `~`. Reproduction (run in research):

```
weave init --store '/tmp/wvtilde/~/x'   # created a LITERAL dir named '~'
ls /tmp/wvtilde/  # -> shows a directory literally called '~'
```

So a quoted path or an interactive prompt answer containing `~` is taken LITERALLY.

Resolution options (item rule: "fix the doc, NOT the impl, unless the impl is wrong"):
- (A) RECOMMENDED — fix the README: drop the false claim. State weave does NOT
  expand `~` itself; the caller's shell expands an UNquoted `~/path` before weave
  sees it. (Go convention; PRD §8.2 does not mandate tilde expansion.)
- (B) only if parity-with-skilldozer is required AND verified: add tilde
  expansion in resolveStore (an impl fix; permitted because the impl would then
  be "wrong" vs a documented+desired behavior). Verify skilldozer actually does it
  before choosing (B).

### DRIFT 2 (MED) — `--no-extensions` pattern NOT mentioned in README

Item contract: "verify the canonical one-liner is `pi -e "$(weave <tag>)"` AND
the `--no-extensions` pattern is mentioned." The canonical one-liner IS present
and correct. The `--no-extensions` pattern is NOT mentioned anywhere in the
current README.

Resolution: confirm the exact pi flag (the contract names `--no-extensions`;
verify against `pi --help` / pi docs) and add a short Usage note/example showing
how to load ONLY weave extensions and none of pi's auto-discovered ones, e.g.
`pi --no-extensions -e "$(weave example)"`. If the real flag differs, document
the actual flag.

### DRIFT 3 (LOW/MED) — .gitignore is a SUPERSET of PRD §16

PRD §16 (exact, line 497-507):
```
/weave
/dist
*.test
*.out
.DS_Store
```
plus note: "/weave ignores the locally-built binary; everything else is committed,
including extensions/example.ts."

Current .gitignore has all 5 required entries (correct) PLUS extras:
`/build`, `node_modules/`, `vendor/`, `.env`, `.env.*`, `.idea/`, `.vscode/`,
`*.swp`, `*~`, `.pi-subagents/`.

- All 5 required entries present + correctly spelled. PASS on the required set.
- The extras ignore deps/secrets/IDE/tooling artifacts — NONE of which PRD §16
  wants committed. So they do not violate "everything else is committed."
- `/build` is unused (no /build dir exists).

Resolution (judgment call): keep the 5 required (present). KEEP the protective
extras (node_modules/, .env*, .pi-subagents/, etc.) because removing them risks
committing secrets and the .pi-subagents artifacts dir. TRIM the unused `/build`
for tidiness. Document the deviation from literal "exactly" with this reasoning.
(Only strip to exactly 5 if the owner demands strict spec compliance AND the
repo is guarded another way; recommend against.)

## 5. NON-drifts confirmed clean (the "also check these" list)

- No em dash in README (period used in one-liner). PASS.
- No `skilldozer` reference in README / example.ts / completions. The ONE
  `skilldozer` mention is `install.sh:4` ("Mirrors skilldozer's install.sh"), a
  dev comment SANCTIONED by PRD §12.1 ("install.sh (mirrors skilldozer
  install.sh)") — not user-facing prose. PASS (confirm interpretation).
- No `yaml.v3` in any user-facing doc. PASS.
- example.ts == main.go exampleExtensionTemplate == PRD §11 (byte-for-byte). PASS.
- Constraints section covers all 4 §15.8 points. PASS.
- All §6.1 commands + §6.2 modifiers present in README. PASS.
- All 3 env vars documented. PASS.
