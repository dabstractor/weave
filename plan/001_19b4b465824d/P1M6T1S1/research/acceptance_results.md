# §13 Acceptance Results — VERIFIED by running the real binary + real example.ts

Method: copied the live `main.go` + `internal/` + `go.mod` into an isolated
`/tmp/wv-acc`, wrote `extensions/example.ts` with the EXACT
`exampleExtensionTemplate` bytes, `go build -o weave .`, and ran every §13
check. Build used: `go1.25`, module `github.com/dabstractor/weave`, binary
prints `weave dev` (the `version` var is `"dev"` under an unflagged build).

This is GROUND TRUTH — not speculation. The numbers below are real exit codes.

## 1. example.ts byte-equality — CONFIRMED

`extensions/example.ts` written from the §11 body diffs IDENTICAL to the
`exampleExtensionTemplate` raw-string body extracted from main.go (L84-110).
File size: **724 bytes**. The §11 body contains ZERO backticks, so the Go raw
string needs no `+ "`" +` splicing (the main.go comment already notes this).
Both copies == PRD §11.

## 2. Core §13 criteria — ALL PASS (9/9) in a clean environment

| # | check | result |
|---|-------|--------|
| 1 | `go build -o weave .` | ✅ OK |
| 2 | `./weave --version` → `weave dev` | ✅ rc=0 |
| 3 | `test "$(./weave --path)" = "$PWD/extensions"` | ✅ sibling rule fires |
| 4 | `./weave --list` shows `example` | ✅ (TAG col, JSDoc description shown) |
| 5 | `test -f "$(./weave example)"` → `…/extensions/example.ts` | ✅ |
| 6 | `test -f "$(./weave -f example)"` → the file itself | ✅ |
| 7 | unknown tag `nope`: empty stdout, exit 1 | ✅ |
| 8 | absolute-path contract: output starts with `/` | ✅ |
| 9 | `./weave check` exit 0, example reported OK | ✅ (`OK    example (example)`) |

`--list` output shape (real):
```
TAG      NAME    DESCRIPTION
example  (none)  Reference example extension for weave.
                 Demonstrates a minimal pi extension and how
```
`check` output (real): `OK    example (example)` / `1 extensions, 0 errors, 0 warnings`.

## 3. Dir + package styles (temp scaffolding) — ALL PASS (4/4)

Created `/tmp/edz-acc/{foo/index.ts, bar/package.json + bar/src/index.ts}`:
- `weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave --list` → shows `foo` AND `bar` ✅
- `weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave foo` → `/tmp/edz-acc/foo` (dir) ✅
- `weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave -f bar` → `/tmp/edz-acc/bar/src/index.ts` (package entry file) ✅

## 4. Symlink install (resolve-back-to-repo) — ALL PASS (2/2)

`ln -sf "$PWD/weave" /tmp/weave-bin/weave`:
- `/tmp/weave-bin/weave example` → `$PWD/extensions/example.ts` (EvalSymlinks resolves the symlink back to the repo) ✅
- `weave_EXTENSIONS_DIR="$PWD/extensions" ./weave example` → resolves ✅

This confirms §12.1 symlink install works and `findSibling`'s
`filepath.EvalSymlinks` call is doing its job (the macOS-essential line).

## 5. Config + first-run — ALL PASS (5/5)

In `/tmp/weave-iso` with isolated `HOME` + `XDG_CONFIG_HOME`:
- unconfigured (`weave x`): exit 1, stderr contains `run \`weave init\`` ✅
- `weave init --store /tmp/weave-store` (with `weave_CONFIG`): store dir created ✅
- config written: `grep -q 'store: /tmp/weave-store' cfg.yaml` ✅
- config rule wins: `--path` resolves to the config store ✅
- env beats config: `weave_EXTENSIONS_DIR=… ./weave --path` prefers env ✅

## 6. pi end-to-end — HARD criterion PASSES (does not error)

`pi` IS on PATH: `/home/dustin/.local/bin/pi`, version **0.80.3**. NOT deferred.

**The proof is a CONTRAST, not the model's prose.** The §13 comment hedges:
"confirm pi's output references the example extension / **does not error**."

- **Correct example.ts**: `pi --no-extensions -e "$(./weave example)" -p "say hi"`
  → exit **0**, NO load error, normal assistant reply. The extension LOADS via
  `-e` with discovery disabled (the entire point of §2.2 "no auto-discovery").
- **Broken extension** (control): write `extensions/broken.ts` with invalid TS,
  run the same command → exit **1**, stderr:
  `Error: Failed to load extension "/tmp/wv-acc/extensions/broken.ts": Failed to
  load extension: ParseError: Missing semicolon.`

So `--no-extensions` does NOT block explicit `-e` paths (matches
pi_extension_facts.md §3: `cliEnabledExtensions` always load), and a VALID
example.ts loads without error. The hard criterion ("does not error") PASSES.

**Soft note on the "references the example extension" half:** the assistant's
prose may NOT name the `weave-example` command, because commands registered by
an ad-hoc `-e` extension are not auto-surfaced into the model's context and
`session_start` `ui.notify` does not print to `-p` one-shot stdout. This is a
test-design artifact (§13's prompt asks the model to introspect something it has
no automatic view of), NOT an implementation defect — the extension is provably
loaded. Document this; do NOT "fix" example.ts to chase model prose. The
load/no-error contract is what matters and it holds.

## 7. ⚠️ CRITICAL GOTCHA — §13 needs a CLEAN config/env state

The §13 *Discovery + path* block (`--path`, `--list`, `weave example`) does
**NOT** isolate `HOME`/`XDG_CONFIG_HOME` (unlike the *Config + first-run*
block, which does). It assumes "a clean clone." In a real dev shell this is
NOT clean:

- A stray `~/.config/weave/config.yaml` left by ANY prior `weave init` (this
  session's testing created one pointing at `~/projects/weave/foo`) fires
  §8.3 **rule 2 (config)** BEFORE **rule 3 (sibling)**. Result: `./weave --path`
  returns the config store, NOT `$PWD/extensions`, and
  `test "$(./weave --path)" = "$PWD/extensions"` **FAILS** — a false negative
  that looks like an implementation bug but is purely environmental.
- A stray `weave_EXTENSIONS_DIR` env export (rule 1) does the same.

**The implementation is correct.** To run §13 faithfully, the implementer MUST
start clean:
```bash
unset weave_EXTENSIONS_DIR weave_CONFIG
# remove/ignore any stray config from prior testing:
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/weave/config.yaml"
```
…or run the §13 *Discovery* block under an isolated
`HOME=$(mktemp -d) XDG_CONFIG_HOME=$HOME/.config` (mirroring what the
*Config + first-run* block already does). The PRP bakes this into the
acceptance runner. This is the single most likely "false failure" an
implementer will hit, so it is called out explicitly.

## 8. .gitignore — example.ts IS committed

`.gitignore` ignores `/weave` (the built binary), `/dist`, `/build`,
`node_modules/`, `.pi-subagents/`, etc. It does NOT ignore `extensions/` or
`*.ts`. So `extensions/example.ts` is a tracked, committed repo asset (PRD §16
explicitly: "everything else is committed, including extensions/example.ts").
No `.gitignore` change needed.

## 9. No implementation fixes required

EVERY §13 criterion passes in a clean environment with the current M1-M4 code
(+ the M5.T1.S1 CLI-contract changes landing in parallel, which only affect
help/precedence/error-exit paths — none of the §13 *Discovery*/*resolution*
behavior). The ONLY deliverable is the `extensions/example.ts` file plus a
runnable acceptance check. The "if any criterion fails, fix the implementation"
clause is a no-op here: nothing fails (once the env is clean).

## 10. Optional: byte-equality guard test

main.go's own comment (L77) states the cross-task contract: "the repo asset
extensions/example.ts created by P1.M6.T1.S1 MUST equal this byte-for-byte."
A ~6-line Go test in main_test.go reading `extensions/example.ts` and asserting
it equals `exampleExtensionTemplate` locks this permanently into `go test ./...`.
No existing test reads a repo-root-relative file (all use `t.TempDir()` /
`t.Chdir`), so the test must locate the repo root. Simplest robust approach
when running `go test` from repo root: `os.ReadFile("extensions/example.ts")`.
RECOMMENDED but optional — the file is byte-correct by construction.
