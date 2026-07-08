# Research: Bash install-script best practices for a Go CLI installer

## Summary
Authoritative semantics for each construct the PRP needs, with stable GNU/POSIX/Go/ShellCheck references. **Note on conflict & tooling:** The task body says *"Do NOT write files — return the research as text,"* which conflicts with the runtime output-path override (write to `…/66106db7/research.md`). Per subagent guidance, the explicit no-edit instruction wins — **I have not written any file**; the research is returned as text. Additionally, this environment exposes no `web_search`/fetch/shell tool, so URLs below are canonical references drawn from authoritative knowledge; they are stable links but were not live-fetched in this run (see Gaps).

## Findings

### 1. `ln -sfn` vs `ln -sf` — why `-n` matters
GNU coreutils `ln`: [`ln` invocation](https://www.gnu.org/software/coreutils/manual/html_node/ln-invocation.html#ln-invocation).
- `-s` `--symbolic`: make a symbolic link.
- `-f` `--force`: remove existing destination files first.
- `-n` `--no-dereference`: treat `LINK_NAME` itself as a normal file **if it is a symlink to a directory** — i.e. do *not* follow it.

**The trap:** When `<link>` already exists as a *symlink pointing to a directory*, plain `ln -sf` **dereferences** `link` first, treats the pointed-to directory as the destination, and creates the new link *inside* that directory instead of replacing `link`. `-n` tells `ln` "operate on the link node, not what it points to," so `-f` removes/recreates the link itself.

**Concrete failure example (re-pointing a versioned symlink):**
```bash
$ mkdir -p versions/1.0 versions/2.0
$ ln -s versions/1.0 current            # current -> versions/1.0
$ ln -sf versions/2.0 current           # WRONG
$ ls -l current                         # current -> versions/1.0   (unchanged!)
$ ls -l versions/1.0                    # stray:  versions/1.0/2.0 -> ../2.0  (nested junk)
# Correct:
$ ln -sfn versions/2.0 current          # -n => current treated as a file; -f replaces it
$ ls -l current                         # current -> versions/2.0
```
Rule for installers: always `ln -sfn <target> <link>` when `<link>` may already be a symlink. (The equivalent long form is `--symbolic --force --no-dereference`.)

### 2. `set -euo pipefail` strict mode — semantics & gotchas
GNU Bash manual: [The Set Builtin](https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html#The-Set-Builtin), [Pipelines](https://www.gnu.org/software/bash/manual/html_node/Pipelines.html), [Shell Parameter Expansion](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html).
- `-e` (`errexit`): exit immediately if a *pipeline*/compound command returns non-zero — **except** where the command's status is consumed by a condition. The manual lists the exceptions verbatim: a command in the test of `if`/`elif`, in a `while`/`until` condition, any part of a `&&`/`||` list except the final command, or whose status is inverted by `!`. → `if go build …; then …` will **not** trigger exit on failure; the failure is the test result. See [Set Builtin `-e`](https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html).
- `-u` (`nounset`): expanding an unset variable/parameter is an error. Mitigation: default-expansion forms `${VAR:-}` / `${VAR:-default}`. A common trap is unguarded `$PATH`/`$XDG_*` under `-u` — use `"${PATH:-}"`. See [Parameter Expansion: `${parameter:-word}`](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html).
- `-o pipefail`: the pipeline's exit status is that of the **rightmost failing command**, or 0 if all succeed. Without it, `cmd1 | cmd2` returns `cmd2`'s status, so a failing `cmd1` (e.g. `go build | tee log`, or `find … | head`) is masked. See [Pipelines](https://www.gnu.org/software/bash/manual/html_node/Pipelines.html).
- **Declare/assign masking (SC2155):** `local var=$(failing_cmd)` does **not** trip `-e`, because `local`/`declare`/`readonly`/`export` is the command and its own status is 0. Fix: `local var; var=$(failing_cmd)`. (Plain `var=$(failing_cmd)` *does* propagate the substitution's status under `-e`.)
- Canonical deep-dive on `-e` surprises: [BashFAQ/105](https://mywiki.wooledge.org/BashFAQ/105) (wooledge, not GNU, but the standard reference).

### 3. `command -v go` — the POSIX-correct existence check
POSIX.1-2017 Shell & Utilities: [`command`](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/command.html).
- `command -v name` writes the pathname (or builtin/alias/function/keyword) the shell would invoke, exit status **0 if found**, non-zero otherwise — and it's a **shell builtin** present in every POSIX shell (dash, ash, bash, zsh, …).
- `which` is **not POSIX**: it's a separate external program, historically a csh script, may be absent in minimal images (e.g. some Alpine/scratch), doesn't see builtins/aliases/functions, and uses its own path logic rather than the running shell's. ShellCheck flags it as [SC2230](https://github.com/koalaman/shellcheck/wiki/SC2230).

### 4. Shell detection & PATH rc-line (print, don't edit)
- Detection: `"${SHELL##*/}"` or `basename "$SHELL"` yields the login-shell name (`bash`/`zsh`/`fish`). Caveat: `$SHELL` is the *login* shell, not necessarily the currently-running shell; accept this as a best-effort hint.
- rc targets and lines:
  - **bash** → `~/.bashrc` (interactive non-login; also consider `~/.bash_profile` for logins): `export PATH="$HOME/.local/bin:$PATH"`
  - **zsh** → `~/.zshrc`: `export PATH="$HOME/.local/bin:$PATH"` (or `path+=("$HOME/.local/bin")`)
  - **fish** → `~/.config/fish/config.fish`; the idiomatic command is **`fish_add_path`** ([docs](https://fishshell.com/docs/current/cmds/fish_add_path.html)), which appends to `$fish_user_paths` and, by default, stores it as a **universal variable** persisted across sessions — so often no config-file edit is needed at all. Example: `fish_add_path ~/.local/bin` (add `-U` to force universal, `-g` for global).
- **Convention: print the suggested line, never auto-append to rc files.** Reasons: (a) intrusive/surprising to modify a user's dotfiles without consent; (b) naive appends create **duplicate PATH entries on every re-run**; (c) rc location/sourcing varies by OS, distro, and personal setup (e.g. `.profile` vs `.bash_profile` vs `.zshenv`), so the "right" file is unknowable; (d) the user may source a framework that manages PATH itself.

### 5. `go build` flags: `-trimpath`, `-ldflags "-s -w -X …"`
Docs: [`go build`](https://pkg.go.dev/cmd/go) (see "Build flags", `-trimpath`), [`cmd/link`](https://pkg.go.dev/cmd/link) (linker flags `-s`, `-w`, `-X`). Local: `go help build`, `go tool link -help`.
- **`-trimpath`** (build flag): strips all filesystem paths from the compiled binary; absolute paths are rewritten to the module's versioned import path (`example.com/mod@v1.0.0/file.go`) or `GOPATH`. Yields **reproducible builds** and prevents leaking `$HOME`/`$GOPATH`/absolute paths in panic traces and embedded file info. *Caveat:* reproducibility also needs a pinned Go toolchain + tagged module versions.
- **`-s`** (linker): omit the **symbol table**. Shrinks the binary.
- **`-w`** (linker): omit the **DWARF debug information** (`.debug_*` sections; what delve/gdb need). Shrinks further. `-s -w` together commonly cut ~20–30%.
- **Gotcha — what survives:** Go's **pclntab** (program-counter/line table) is *not* removed by `-s`/`-w`, so runtime panic stack traces (function names + line numbers via `runtime.Caller`) still work. You only lose external/ELF symbols and DWARF.
- **`-X importpath.name=value`** (linker): set a string variable at link time. Works on any **package-level `string`** variable; the link-time value **overrides** any initializer. Must be `importpath.name`, value after the first `=`. *Caveat:* only `string` vars; `const`, ints, or function-initialized vars can't be set this way. Split the var out: `var version = "dev"` → `-X main.version=1.2.3`.

### 6. PATH-already-present detection idiom
```bash
case ":${PATH:-}:" in
  *":$TARGET:"*) echo "present" ;;   # already in PATH
  *)               echo "absent" ;;
esac
```
**The colon-padding trick:** `PATH` is colon-delimited, but a bare substring search `case "$PATH" in *"$TARGET"*)` matches **partial path elements** — e.g. searching for `/bin` would falsely match inside `/usr/bin`, `/usr/local/bin`, or `/sbin`. By wrapping `PATH` in leading+trailing colons (`:/usr/bin:/bin:`) **and** searching for `:$TARGET:` (colons on both sides of the needle), you guarantee an **element-boundary match**: `/bin` only matches a standalone `:/bin:`, never `/usr/bin`. The padding also correctly matches `$TARGET` when it sits at the **start or end** of `PATH`. `${PATH:-}` keeps it safe under `set -u`. Caveat: assumes `$TARGET` contains no `:` itself (PATH elements never do).

### 7. shellcheck — standard linter & common codes
**shellcheck** is the de-facto static analyzer for POSIX sh/bash scripts: [shellcheck.net](https://www.shellcheck.net/), [github.com/koalaman/shellcheck](https://github.com/koalaman/shellcheck), [wiki (SC code reference)](https://github.com/koalaman/shellcheck/wiki).
Codes that fire frequently on install scripts:
- [SC2086](https://github.com/koalaman/shellcheck/wiki/SC2086): *"Double quote to prevent globbing and word splitting"* — unquoted `$VAR` → `"$VAR"`.
- [SC2155](https://github.com/koalaman/shellcheck/wiki/SC2155): *"Declare and assign separately to avoid masking return values"* — `local var=$(cmd)` → `local var; var=$(cmd)` (see #2).
- [SC2230](https://github.com/koalaman/shellcheck/wiki/SC2230): *"`which` is non-standard. Use `command -v`"* (see #3).
- [SC2046](https://github.com/koalaman/shellcheck/wiki/SC2046): quote `$(...)` to prevent word splitting.
- [SC1090](https://github.com/koalaman/shellcheck/wiki/SC1090) / [SC1091](https://github.com/koalaman/shellcheck/wiki/SC1091): can't follow a dynamic/non-constant `source` (e.g. sourcing `/etc/os-release`) — usually a permitted `# shellcheck source=/etc/os-release` or `disable` directive.
- [SC2181](https://github.com/koalaman/shellcheck/wiki/SC2181): *"Check exit code directly with `if cmd;`, not indirectly with `$?`."*
- [SC2312](https://github.com/koalaman/shellcheck/wiki/SC2312): *"Consider invoking this command directly"* (e.g. `$(command -v foo)` → just `foo` when possible).
- [SC2004](https://github.com/koalaman/shellcheck/wiki/SC2004) / [SC2006](https://github.com/koalaman/shellcheck/wiki/SC2006): legacy `$[]` / backticks.

## Sources
- Kept: GNU coreutils [`ln` invocation](https://www.gnu.org/software/coreutils/manual/html_node/ln-invocation.html#ln-invocation) — authoritative `-s/-f/-n` semantics.
- Kept: GNU Bash manual — [The Set Builtin](https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html), [Pipelines](https://www.gnu.org/software/bash/manual/html_node/Pipelines.html), [Parameter Expansion](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) — `-e`/`-u`/`pipefail`/`${VAR:-}`.
- Kept: [BashFAQ/105](https://mywiki.wooledge.org/BashFAQ/105) — the canonical `-e` gotchas reference.
- Kept: POSIX.1-2017 [`command`](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/command.html) — `command -v` is standardized; `which` is not.
- Kept: [`fish_add_path`](https://fishshell.com/docs/current/cmds/fish_add_path.html) — idiomatic fish PATH management.
- Kept: [`go build`](https://pkg.go.dev/cmd/go) and [`cmd/link`](https://pkg.go.dev/cmd/link) — `-trimpath`, `-s`, `-w`, `-X`.
- Kept: [shellcheck](https://www.shellcheck.net/) / [repo](https://github.com/koalaman/shellcheck) / [wiki](https://github.com/koalaman/shellcheck/wiki) + individual SC pages.
- Dropped: generic "bash strict mode" blog posts — superseded by the primary GNU/POSIX/ShellCheck references above.

## Gaps
- **No live URL verification:** this subagent has no `web_search`/fetch/`exec` tool, so every URL was produced from authoritative knowledge, not fetched. The links are stable canonical docs, but the implementer should click-verify the exact section anchors (esp. any `#…` fragments) and confirm the Go `cmd/link` flag wording for the installed Go version (`go tool link -help` / `go help build` are the local source of truth and should be preferred at build time).
- `-s` vs `-w` exact help-string wording varies slightly across Go versions (e.g. "symbol table" vs "symbol table and debug information"); semantics (strip symbol table / strip DWARF, pclntab preserved) are stable.
- POSIX.1-2024 supersedes 2017; the `command -v` definition is unchanged, but a newer `onlinepubs` URL can be used if desired.
- Could not empirically reproduce the `ln -sf` nested-dir example in this run (no shell tool) — the trace is from known coreutils behavior; recommend the implementer reproduce it once to lock in the regression test.

## Supervisor coordination
No decision needed. Noted: explicit task instruction *"Do NOT write files — return the research as text"* overrides the runtime output-path override; I returned text and wrote nothing. Also flagged: no web/shell tooling is available to this subagent, so URLs are unverified (research is knowledge-based, complete, and accurate to canonical docs).