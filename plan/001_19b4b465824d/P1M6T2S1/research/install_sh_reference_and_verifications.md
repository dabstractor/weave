# Research — P1.M6.T2.S1 `install.sh`

**TL;DR**: The deliverable is a near-mechanical **port of skilldozer's `install.sh`** with
the renames `skilldozer→weave`, `SKILLDOZER_INSTALL_BIN→weave_INSTALL_BIN`,
`skills→extensions`, `§8.2→§8.3`, `P1.M6.T15.S1→P1.M6.T3.S1`. Every validation command in
this PRP was RUN against a ported copy in a temp clone; all pass. **This file is the
evidence trail.**

---

## 1. THE reference script: skilldozer `install.sh` (local, read-only ground truth)

- **Path**: `/home/dustin/projects/skilldozer/install.sh` (4051 bytes, `#!/usr/bin/env bash`,
  `set -euo pipefail`, executable).
- **Status**: shellcheck-CLEAN (`shellcheck` 0.11.0 → exit 0, zero output). It is the
  battle-tested implementation weave §12.1 says to "mirror."
- **It already implements all 7 PRD §12.1 steps** in order:
  1. `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; cd "$SCRIPT_DIR"`
  2. `if ! command -v go …; then <stderr install hint>; exit 1; fi`
  3. `go build -trimpath -ldflags "-s -w -X main.version=$(git describe … || echo dev)" -o skilldozer .`
  4. target pick: `$SKILLDOZER_INSTALL_BIN` → `$HOME/.local/bin` (if `-d` or `$HOME -w`)
     → `/usr/local/bin` (if `-w`) → error+exact-sudo-hint
  5. `ln -sfn "$SCRIPT_DIR/skilldozer" "$TARGET/skilldozer"` (SYMLINK, `-sfn`, ABSOLUTE)
  6. PATH check via `case ":${PATH:-}:" in *":$TARGET:"*) ;;` then PRINT rc line for
     bash/zsh/fish (never auto-edit)
  7. verify via ABSOLUTE symlink path `"$TARGET/skilldozer" --version` + `example`

### Rename map (skilldozer → weave) — exhaustive
| skilldozer literal | weave literal | where |
|---|---|---|
| `skilldozer` (binary name) | `weave` | `-o`, symlink src/dst, verify cmds, echo strings |
| `SKILLDOZER_INSTALL_BIN` | `weave_INSTALL_BIN` | env override, `${weave_INSTALL_BIN:-}` |
| `skills` (sibling dir) | `extensions` | (only in COMMENT text; install.sh does not name it) |
| `§8.2 sibling-of-binary skill resolution` | `§8.3 sibling-of-binary (extension) resolution` | header comment |
| `P1.M6.T15.S1` (completions task ref) | `P1.M6.T3.S1` | trailing comment |
| `🚀 skilldozer install` | `🚀 weave install` | banner echo |
| `mcpeepants QUICK_INSTALL.sh` provenance | drop (weave mirrors skilldozer, not mcpeepants) | header comment |

### Two weave-specific deltas vs the skilldozer original
1. **Header comment**: rewrite to say "mirrors skilldozer `install.sh` (PRD §12.1)" and
   reference §8.3 (weave) not §8.2 (skilldozer). Keep the rationale: "never copies — the
   symlink is load-bearing for §8.3 sibling-of-binary resolution."
2. **Completions pointer**: skilldozer says "see task P1.M6.T15.S1"; weave says "see task
   P1.M6.T3.S1" (the actual completions subtask id in THIS plan). Everything else is
   verbatim (modulo the rename map).

---

## 2. Codebase facts that make the port work (all VERIFIED by reading the real files)

### `go build … -X main.version=…` — the linker var EXISTS and is a `var`, not `const`
- `main.go:43`: `var version = "dev"` (package-level **var**, package `main`).
  Comment L32-42: ldflags `-X main.version=…` overrides it; the linker symbol path is
  `main.version` (NOT the module import path). The default `"dev"` is what `go run` and
  plain `go build` print. So install.sh's ldflags string is correct AS-IS.
- `main.go:487`: `fmt.Fprintf(stdout, "weave %s\n", version)` — prints `weave <version>\n`.
  Lowercase `weave`, single space, newline. Verified: `weave dev` / `weave v1.2.3`.

### The sibling-of-binary rule — WHY the symlink is load-bearing
- `internal/extdir/extdir.go` `resolveSiblingFromExe` (rule 3):
  `EvalSymlinks(os.Executable())` → `filepath.Dir(real)` → `candidate = repoDir/extensions`
  → wins iff `os.Stat(candidate)` IS an existing directory. **No entry-count check** for
  rule 3 — `extensions/` merely existing as a dir is enough.
- `findSibling` → `Find()` priority: env → config → **sibling** → walk-up → ErrNotFound.
  So the sibling rule only wins when no env/config points elsewhere (the install.sh TEST
  must use a clean env — see Gotcha §6 below).
- `EvalSymlinks` is REDUNDANT-but-harmless on Linux (`/proc/self/exe` already resolves)
  and REQUIRED on macOS. **Do not remove it** from extdir (already correct; install.sh
  relies on it). Source: `architecture/verified_symlink_resolution.md` (skilldozer's
  empirical test) + weave extdir.go doc comment.

### go.mod / module / .gitignore (VERIFIED)
- `go.mod`: `module github.com/dabstractor/weave` / `go 1.25`. Zero deps, no go.sum.
  → `go build -o weave .` needs ONLY the Go toolchain (confirmed: builds clean).
- `.gitignore`: ignores `/weave` (the BUILT BINARY) — so `install.sh` itself is committed
  and tracked, while the `weave` binary it builds is not. Do NOT add install.sh to gitignore.

### Toolchain on PATH (VERIFIED)
- `go`: `/usr/bin/go`, go1.26.4 linux/amd64. `git describe --tags --always` works (returns
  `56cafe4`; no tags yet, so a real install prints `weave 56cafe4` until a tag is cut).

---

## 3. Empirical validation (RUN, not theorized) — temp clone at /tmp/weave-install-prp-test

A ported copy of install.sh (full renames applied) was placed in a temp clone containing
the real weave Go source + a STUB `extensions/example.ts` (simulating P1.M6.T1.S1's output),
then exercised. **ALL pass:**

| Check | Command | Result |
|---|---|---|
| shellcheck | `shellcheck install.sh` | **exit 0, zero output** (CLEAN) |
| syntax | `bash -n install.sh` | OK |
| build+symlink | `weave_INSTALL_BIN=/tmp/bin ./install.sh` | exit 0; `Linked: …/weave -> …/weave` |
| version via symlink | `$BIN/weave --version` | `weave dev` (non-git clone) |
| **sibling rule** (THE load-bearing test) | `$BIN/weave --path` | `…/extensions (found via sibling of binary)` ✅ |
| tag resolve via symlink | `$BIN/weave example` | `…/extensions/example.ts` ✅ |
| idempotency | re-run install.sh ×2 | exit 0; symlink refreshed, NOT nested (`ln -sfn` confirmed) |
| PATH-absent branch | `$TARGET` not on PATH | prints `Add to ~/.zshrc: export PATH="…:$PATH"` |

### Version-stamping matrix (the `$(git describe …)` ldflags) — VERIFIED
| Clone state | `weave --version` output |
|---|---|
| non-git dir (git absent/not-a-repo) | `weave dev` (the `\|\| echo dev` fallback) |
| git repo, tag `v1.2.3` exactly at HEAD | `weave v1.2.3` |
| git repo, HEAD 1 commit ahead of `v1.2.3` | `weave v1.2.3-1-gd501377` (git describe standard form) |

→ The `$(git describe …)` **must expand INSIDE the double-quoted `-ldflags` string** (do NOT
escape the `$`; skilldozer gotcha #4, confirmed). `|| echo dev` fires only outside a repo.

---

## 4. External research (bash best practices — authoritative URLs)

From `researcher` subagent (knowledge-based; canonical links):

- **`ln -sfn` vs `ln -sf`**: `-n`/`--no-dereference` treats a symlink-to-dir dest as a
  file so `-f` replaces the LINK, not dereferences into the dir. Without `-n`, `ln -sf
  newtarget existing-symlink-to-dir` creates a STRAY link INSIDE the dir and leaves the
  link pointing at the OLD target (verified-nested-trap in idempotency test conceptually).
  Ref: GNU coreutils `ln` invocation —
  https://www.gnu.org/software/coreutils/manual/html_node/ln-invocation.html#ln-invocation
- **`set -euo pipefail`**: `-e` errexit (suppressed in `if`/`||`/`&&` test position — see
  https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html);
  `-u` nounset (use `${VAR:-}` for unset: https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html);
  `-o pipefail` (rightmost-failing status: https://www.gnu.org/software/bash/manual/html_node/Pipelines.html).
  Canonical `-e` gotchas: https://mywiki.wooledge.org/BashFAQ/105
- **`command -v go`**: POSIX (https://pubs.opengroup.org/onlinepubs/9699919799/utilities/command.html);
  `which` is non-POSIX/non-portable (ShellCheck SC2230).
- **fish PATH**: `fish_add_path <dir>` is idiomatic
  (https://fishshell.com/docs/current/cmds/fish_add_path.html); bash/zsh use
  `export PATH="$dir:$PATH"` in `~/.bashrc`/`~/.zshrc`. Convention: PRINT only, never
  auto-edit rc files (intrusive + duplicates on re-run).
- **go build flags**: `-trimpath` (reproducible, strips paths from panics);
  `-s` (strip symbol table) / `-w` (strip DWARF) — `~20-30%` smaller, pclntab preserved so
  panic traces still work; `-X importpath.name=value` sets a package-level `string` var at
  link time (const/int won't work — weave's is a `var`, correct).
  Refs: https://pkg.go.dev/cmd/go , https://pkg.go.dev/cmd/link ; local `go help build`.
- **PATH-present idiom**: `case ":${PATH:-}:" in *":$TARGET:"*)` — colon padding gives
  element-boundary match (avoids `/bin` matching `/usr/bin`). `${PATH:-}` safe under `-u`.
- **shellcheck**: https://www.shellcheck.net/ , https://github.com/koalaman/shellcheck ,
  wiki https://github.com/koalaman/shellcheck/wiki . Codes that fire on install scripts:
  SC2086 (quote), SC2155 (declare+assign separately), SC2230 (use command -v), SC2181
  (if cmd; not $?). The ported script triggers NONE (clean exit 0).

---

## 5. Dependencies / scope boundaries

- **P1.M6.T1.S1 (parallel, being implemented)**: creates `extensions/example.ts`. The
  sibling rule (rule 3) wins iff `extensions/` EXISTS as a directory. So install.sh's
  verify step (`weave example`) and the load-bearing `weave --path` test REQUIRE
  P1.M6.T1.S1 to have landed `extensions/`. **If P1.M6.T1.S1 is not yet on disk, create a
  stub `extensions/` for the install.sh test** (the rule only needs the dir to exist), but
  the SHIPPED repo must contain the real `extensions/example.ts`.
- **P1.M6.T3.S1 (completions)**: install.sh does NOT install completions (matches PRD §14
  deferral + skilldozer). It only PRINTS a pointer to the completions task at the end.
- **P1.M6.T4.S1 (README) / P1.M6.T5.S1 (doc sync)**: install instructions also appear in
  README §3 (Mode B final task). install.sh itself is the Mode A self-documenting deliverable
  (echo statements guide the user). Do NOT write README in this task.
- **NO source-code (.go) change required** for install.sh: the binary already builds, the
  version var exists, the sibling rule exists. install.sh is a STANDALONE bash asset at the
  repo root.

## 6. The #1 false-failure gotcha (inherited from P1.M6.T1.S1)

The sibling rule is rule **3** — env (rule 1) and config (rule 2) WIN FIRST. A stray
`~/.config/weave/config.yaml` (from any prior `weave init`) or `weave_EXTENSIONS_DIR` in
the test shell makes `weave --path` resolve to the WRONG dir (the config store), and
`weave example` may not resolve to the repo's `extensions/example.ts`. This looks like an
install.sh bug but is purely environmental. **FIX for testing**: run the verify under a
clean env (unset both env vars + a temp/isolated HOME, or the explicit
`weave_EXTENSIONS_DIR="$SCRIPT_DIR/extensions"` override). The PRP bakes this into its
validation commands.
