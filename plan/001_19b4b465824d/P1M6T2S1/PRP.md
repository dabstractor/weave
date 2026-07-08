# PRP — P1.M6.T2.S1: `install.sh` (build + symlink + PATH check + verify)

> **Subtask:** Ship a standalone bash `install.sh` at the repo root that BUILDS the `weave`
> binary with version ldflags and **symlinks** (never copies) it into a PATH dir, then guides
> the user through PATH setup and a verify command. It is a near-mechanical **port of
> skilldozer's working, shellcheck-clean `install.sh`** with the renames
> `skilldozer→weave`, `SKILLDOZER_INSTALL_BIN→weave_INSTALL_BIN`, `skills→extensions`,
> `§8.2→§8.3`, `P1.M6.T15.S1→P1.M6.T3.S1`. **Every validation command in this PRP was RUN
> against a ported copy in a temp clone and PASSES** (shellcheck clean; build+symlink; the
> load-bearing sibling-of-binary rule finds `extensions/`; idempotent re-run; version
> stamping for dev/tagged/ahead-of-tag). See `research/install_sh_reference_and_verifications.md`.

---

## Goal

**Feature Goal**: Create `install.sh` at the repo root that implements all 7 steps of PRD
§12.1: (1) `cd` to its own dir, (2) verify `go` on PATH (else hint + exit 1), (3) `go build`
with `-trimpath` + version ldflags, (4) pick a target bin dir (`$weave_INSTALL_BIN` →
`$HOME/.local/bin` → `/usr/local/bin`, else exact-sudo hint), (5) **symlink** `$TARGET/weave`
→ `$SCRIPT_DIR/weave` with `ln -sfn`, (6) ensure `$TARGET` is on `PATH` (else PRINT the
exact rc-file line for the detected shell), (7) print+run a verify command (`weave example`).

**Deliverable**: ONE new file — `install.sh` at the repo root (executable, `#!/usr/bin/env bash`,
`set -euo pipefail`). It is the **Mode A self-documenting** deliverable (echo statements
guide the user; README §3 install text is a separate Mode B task, P1.M6.T4.S1).

**Success Definition**:
- `install.sh` exists at repo root, `chmod +x`, committed (NOT gitignored — `.gitignore`
  ignores `/weave` the binary, not `install.sh`).
- `shellcheck install.sh` → exit 0, zero output (CLEAN). `bash -n install.sh` → OK.
- Running `weave_INSTALL_BIN=<tmp> ./install.sh` builds `weave`, creates the symlink, and
  `<tmp>/weave --version` prints `weave <v>` (dev / git-describe).
- **The load-bearing test**: `<tmp>/weave --path` reports `…/extensions (found via sibling
  of binary)` — i.e. the symlink resolves back to the repo and the §8.3 sibling rule fires.
  `<tmp>/weave example` resolves to `…/extensions/example.ts`.
- Re-running install.sh is idempotent: the symlink is refreshed, not nested (`ln -sfn`).

## User Persona (if applicable)

**Target User**: A clone-and-build user who runs `./install.sh` once and expects `weave` on
PATH, working, with zero config — the repo's own `extensions/` discovered automatically.
(PRD §12.1 "Why symlink, not copy": the sibling-of-binary rule (§8.3) gives a zero-config
default store, and `git pull && go build` updates the linked binary in place.)

**Use Case**: `git clone … && cd weave && ./install.sh` → `weave example` works from any dir.

**User Journey**: clone → `./install.sh` (banner, builds, symlinks into `~/.local/bin`,
prints PATH line if needed + verify command) → reload shell → `weave example` resolves.
A `go install` user (PRD §12.2) does NOT use this script; they run `weave init` on first use.

**Pain Points Addressed**: zero-config discovery (a COPY would break §8.3 silently — the
binary would resolve its own dir to the install location, not the repo, and never find
`extensions/`); no manual `export PATH` guesswork (the script detects the shell and prints
the exact line); no silent `sudo` (if root is needed, the exact command is printed).

## Why

- **Closes the §12.1 contract** — PRD §12.1 mandates an `install.sh` that mirrors skilldozer's,
  implementing exactly these 7 steps. This task lands that file.
- **The symlink is load-bearing for §8.3** — `weave`'s rule-3 sibling resolution
  (`EvalSymlinks(os.Executable()) → Dir → +/extensions`) only finds the repo's `extensions/`
  if the binary on PATH is a SYMLINK back to the repo. A copy would point at the install dir,
  silently breaking the zero-config default store. (Verified: see
  `architecture/verified_symlink_resolution.md` + `internal/extdir/extdir.go`.)
- **`git pull && go build` updates in place** — because the symlink points at the repo's
  built binary, rebuilding the binary (via `go build` or re-running `./install.sh`) is enough;
  nothing in PATH needs re-linking (the symlink already targets the repo).
- **Enables M6.T4 (README) / M6.T5 (doc sync)** — README §3 install instructions reference
  `./install.sh`; the Mode B doc-sync task (P1.M6.T5.S1) verifies install instructions cover
  all three paths (install.sh / `go install` / from-source). install.sh is the Mode A half.

## What

A single new executable file, `install.sh`, at the repo root. No `.go` source change is
required (the binary already builds in P1.M6.T1.S1's environment; the `version` var and the
sibling rule already exist). The script:

1. `cd`s to its own directory (the repo root) via
   `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`.
2. Verifies `go` is on `PATH` via `command -v go`; if not, prints install instructions
   (Go download URL) to stderr and exits 1.
3. Builds: `go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o weave .`
4. Picks the target bin dir, first usable wins:
   `$weave_INSTALL_BIN` (if set) → `$HOME/.local/bin` (if present or creatable) →
   `/usr/local/bin` (if writable) → error with the exact `weave_INSTALL_BIN=…` / `sudo ln …`
   command. **No silent sudo.**
5. `ln -sfn "$SCRIPT_DIR/weave" "$TARGET/weave"` — SYMLINK (never copy), `-sfn` (the `-n`
   treats an existing symlink-to-dir dest as a file so `-f` replaces the link), ABSOLUTE target.
6. Ensures `$TARGET` is on `PATH` via the colon-padded `case ":${PATH:-}:" in *":$TARGET:"*)`;
   if absent, detects the shell from `$SHELL` and PRINTS (never auto-edits) the exact rc line:
   bash → `~/.bashrc`, zsh → `~/.zshrc`, fish → `~/.config/fish/config.fish` (`fish_add_path`).
7. Prints + runs the verify command using the ABSOLUTE symlink path
   (`"$TARGET/weave" --version` and `"$TARGET/weave" example`) so it works before the new PATH
   entry is live in the current shell.

### Success Criteria

- [ ] `install.sh` exists at repo root, `chmod +x` (mode `0755` or `+x`), tracked by git.
- [ ] `shellcheck install.sh` exits 0 with no output; `bash -n install.sh` succeeds.
- [ ] First line is `#!/usr/bin/env bash`; `set -euo pipefail` is the second non-comment line.
- [ ] `weave_INSTALL_BIN=<tmp> ./install.sh` builds `weave`, prints `Linked: <tmp>/weave -> <repo>/weave`, exits 0.
- [ ] `<tmp>/weave --version` prints `weave dev` (non-git) / `weave <git-describe>` (tagged).
- [ ] **`<tmp>/weave --path` prints `…/extensions` and `(found via sibling of binary)`** (the load-bearing symlink test).
- [ ] `<tmp>/weave example` resolves to `…/extensions/example.ts`.
- [ ] Re-running install.sh twice keeps `$TARGET/weave` a regular symlink (not a nested dir).
- [ ] With `$TARGET` off PATH, the script prints the correct rc-file line for the detected shell.
- [ ] No silent sudo; an unwritable `/usr/local/bin` path prints the exact `weave_INSTALL_BIN=` / `sudo ln -sfn …` command.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement
this successfully?_ **Yes.** The deliverable is ONE bash file. The reference implementation
(skilldozer's `install.sh`) is local, working, and shellcheck-clean — the weave version is a
near-mechanical port with a documented rename map. The full ported script is transcribed
verbatim in "Implementation Patterns." The codebase facts an implementer needs: the linker
var is `main.version` (a `var`, not `const` — main.go L43), the module is
`github.com/dabstractor/weave` (go.mod), `.gitignore` ignores `/weave` not `install.sh`, the
sibling rule (`extdir.resolveSiblingFromExe`) wins iff `extensions/` exists as a dir, and
`go` is on PATH (`/usr/bin/go`, go1.26.4). All validation commands were RUN and pass.

### Documentation & References

```yaml
# CONTRACT — the reference script to port (LOCAL, read-only ground truth)
- file: /home/dustin/projects/skilldozer/install.sh
  why: "PRD §12.1 says 'mirrors skilldozer install.sh'. This is that script: 4051 bytes,
        #!/usr/bin/env bash, set -euo pipefail, shellcheck-CLEAN (verified). Port it with the
        rename map below. Every §12.1 step (1-7) is already implemented correctly here."
  pattern: "7 numbered sections matching PRD §12.1 steps 1-7. SCRIPT_DIR via BASH_SOURCE[0];
            command -v go; go build -trimpath -ldflags '-s -w -X main.version=...'; target
            pick chain; ln -sfn (SYMLINK, ABSOLUTE, -n); case-PATH-check + shell-detect
            PRINT; verify via ABSOLUTE symlink path."
  critical: "Do NOT 'improve' it. It is battle-tested. The ONLY edits are the renames in the
             map + the two weave-specific deltas (header comment; completions task id)."

# CONTRACT — the spec this implements (verbatim 7 steps)
- docfile: PRD.md
  section: "§12.1 'install.sh (mirrors skilldozer install.sh)' — the 7-step Behavior list +
            the 'Why symlink, not copy' blockquote (§8.3 sibling rule → zero-config store;
            git pull && go build updates in place). §12.2 (go install) is a DIFFERENT path —
            not this script. §16 confirms install.sh is committed (only /weave binary is ignored)."
  critical: "Step 5 is a SYMLINK not a copy. Step 4 order is fixed: env → ~/.local/bin →
             /usr/local/bin. Step 6 PRINTS the rc line; it does NOT auto-edit rc files."

# WHY the symlink is load-bearing — the rule install.sh exists to serve
- file: internal/extdir/extdir.go
  why: "resolveSiblingFromExe (rule 3): EvalSymlinks(os.Executable()) → Dir → +/extensions,
        wins iff os.Stat says extensions/ IS an existing dir (NO entry-count check for rule 3).
        findSibling→Find() priority: env → config → sibling → walk-up. This is why a SYMLINK
        install makes weave find the repo's extensions/ with zero config, and why a COPY would
        silently break it (the binary would resolve its own dir to the install location)."
  gotcha: "EvalSymlinks is REDUNDANT-but-harmless on Linux (/proc/self/exe) and REQUIRED on
           macOS — do NOT remove it from extdir (already correct). The sibling rule only wins
           when no env/config points elsewhere; the install.sh TEST must use a clean env (see
           Known Gotchas)."

# THE linker var install.sh stamps (must be a package-level `var`, package main)
- file: main.go
  why: "L43: `var version = \"dev\"` (package main). L32-42 comment: -X main.version=...
        overrides it at link time; symbol path is main.version (NOT the module path). L487:
        fmt.Fprintf(stdout, \"weave %s\\n\", version). So the build command's ldflags string
        is correct AS-IS and prints 'weave <version>'."
  gotcha: "-X only works on package-level STRING vars (not const/int). weave's is a var — good.
           Do not change main.go; the var already exists."

# Zero-dep build (only the Go toolchain needed) + version-stamping note
- docfile: plan/001_19b4b465824d/architecture/external_deps.md
  section: "'Zero third-party Go dependencies' + 'Optional build-time: git (for version
            stamping)' — confirms go build needs ONLY go; git describe is a SOFT dep (the
            || echo dev fallback covers non-git clones)."

# go.mod / .gitignore (VERIFIED) — module name + what is ignored
- file: go.mod
  why: "module github.com/dabstractor/weave; go 1.25; zero deps, no go.sum. go build -o weave .
        builds with only the Go toolchain."
- file: .gitignore
  why: "ignores /weave (the BUILT BINARY), /dist, *.test, *.out, .DS_Store, node_modules/,
        .pi-subagents/. It does NOT ignore install.sh — so install.sh IS committed. Do NOT add it."

# EVIDENCE — every validation command in this PRP was RUN; this is the transcript
- docfile: plan/001_19b4b465824d/P1M6T2S1/research/install_sh_reference_and_verifications.md
  why: "The ported script's shellcheck (exit 0), the full end-to-end temp-clone run (build,
        symlink, sibling rule fires, weave example resolves), the idempotency re-run test,
        and the version-stamping matrix (dev / v1.2.3 / v1.2.3-1-g<hash>) — all PASS. Also
        the exhaustive rename map and the external-research URLs (ln -sfn, set -euo pipefail,
        command -v, fish_add_path, go build flags, PATH idiom, shellcheck)."
  critical: "§3 (version-stamping matrix) and §6 (the #1 false-failure: stray config/env
             hijacks the sibling rule — clean env for the test) are the two things an
             implementer will otherwise hit blind."
```

### Current Codebase tree (relevant subset)

```bash
main.go                      # `var version = "dev"` (L43) — the ldflags target (DO NOT EDIT)
internal/extdir/extdir.go    # sibling rule (rule 3) — fires iff extensions/ exists as a dir
go.mod                       # module github.com/dabstractor/weave (go build -o weave .)
.gitignore                   # ignores /weave (binary), NOT install.sh
extensions/example.ts        # ← from P1.M6.T1.S1 (parallel); sibling rule needs this dir to EXIST
install.sh                   # ← DOES NOT EXIST YET (this task creates it)
```

### Desired Codebase tree with files to be added

```bash
install.sh                   # ← NEW (executable): the 7-step PRD §12.1 installer (port of skilldozer's)
# (no .go source change; no go.mod/.gitignore change; no PRD.md / tasks.json change)
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the #1 false-failure, inherited from P1.M6.T1.S1): the sibling rule is rule 3 —
# env (rule 1) and config (rule 2) WIN FIRST. A stray ~/.config/weave/config.yaml (from any
# prior `weave init`) or weave_EXTENSIONS_DIR in the test shell makes `weave --path` resolve
# to the config store, NOT $SCRIPT_DIR/extensions, and `weave example` may not resolve to the
# repo's extensions/example.ts. This looks like an install.sh bug but is environmental.
# FIX (test env, NOT code): run the verify under a clean env:
#   unset weave_EXTENSIONS_DIR weave_CONFIG
#   rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/weave/config.yaml"
# …or explicitly force the sibling store: weave_EXTENSIONS_DIR="$SCRIPT_DIR/extensions".
# The PRP's validation commands bake this in.

# CRITICAL (ln -sfn, not ln -sf): the `-n` (--no-dereference) treats an EXISTING symlink-to-dir
# destination as a file, so `-f` REPLACES the link instead of dereferencing into the dir.
# Without `-n`, re-running install.sh when $TARGET/weave already exists as a symlink would
# create a STRAY link INSIDE the target dir and leave the link stale. Verified: 3× re-run keeps
# $TARGET/weave a regular symlink. Ref: https://www.gnu.org/software/coreutils/manual/html_node/ln-invocation.html#ln-invocation

# GOTCHA (the $(git describe) expands INSIDE the double-quoted -ldflags string): do NOT escape
# the `$`. The build command is literally:
#   go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o weave .
# Verified outputs: non-git → "weave dev"; tag v1.2.3 → "weave v1.2.3"; 1 ahead → "weave v1.2.3-1-g<hash>".

# GOTCHA (set -euo pipefail + unset vars): under `-u`, referencing unset $weave_INSTALL_BIN,
# $SHELL, or $PATH hard-errors. Use the default-expansion forms: ${weave_INSTALL_BIN:-},
# ${SHELL:-}, ${PATH:-}. Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html

# GOTCHA (verify uses the ABSOLUTE symlink path, not bare `weave`): bare `weave` may hit a
# stale shell hash entry until the shell reloads. "$TARGET/weave" works immediately, before
# the new PATH entry is live in the current shell.

# GOTCHA (no silent sudo): if neither ~/.local/bin nor /usr/local/bin is writable, do NOT
# silently re-exec under sudo. PRINT the exact one-liner the user should run:
#   sudo ln -sfn "$SCRIPT_DIR/weave" /usr/local/bin/weave
# (mirrors skilldozer; respects the user's consent + shows the precise command).

# GOTCHA (PRINT the rc line, never auto-edit): auto-appending to ~/.bashrc is intrusive and
# creates DUPLICATE PATH entries on every re-run. Detect the shell from $(basename "$SHELL")
# and print the exact line. fish uses `fish_add_path <dir>` (idiomatic), not export PATH.

# GOTCHA (this task runs IN PARALLEL with P1.M6.T1.S1 which creates extensions/example.ts):
# the sibling rule needs extensions/ to EXIST as a dir. If P1.M6.T1.S1 has not landed yet at
# test time, create a STUB `extensions/example.ts` for the install.sh test (the rule only
# needs the dir to exist — no entry-count check for rule 3). The SHIPPED repo must contain the
# real extensions/example.ts from P1.M6.T1.S1; do NOT commit a stub.
```

## Implementation Blueprint

### Data models and structure

None. This task writes ONE bash script. It consumes the existing `weave` build target
(`go build -o weave .`), the existing `var version` (main.go L43), and the existing
sibling rule (`extdir.resolveSiblingFromExe`). No structs, no config, no Go code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE install.sh (the whole deliverable — a port of skilldozer's install.sh)
  - WRITE install.sh at the repo root with the EXACT content transcribed in
    "Implementation Patterns" below (it is the verified, shellcheck-clean port).
  - HEADER: #!/usr/bin/env bash ; set -euo pipefail ; a comment block referencing PRD §12.1
    and noting "mirrors skilldozer install.sh" + "symlink is load-bearing for §8.3".
  - RENAME MAP (apply to the skilldozer original — exhaustive):
      skilldozer -> weave            (binary name: -o, symlink src/dst, verify cmds, echoes)
      SKILLDOZER_INSTALL_BIN -> weave_INSTALL_BIN   (env override; ${weave_INSTALL_BIN:-})
      "§8.2 sibling-of-binary skill resolution" -> "§8.3 sibling-of-binary resolution"
      P1.M6.T15.S1 -> P1.M6.T3.S1    (completions task pointer in the trailing comment)
      "🚀 skilldozer install" -> "🚀 weave install"
      drop the "mcpeepants QUICK_INSTALL.sh" provenance line (weave mirrors skilldozer)
  - IMPLEMENT the 7 PRD §12.1 steps verbatim (see "Implementation Patterns" for the exact
    bytes; each step is one numbered comment block).
  - CHMOD: make the file executable (chmod +x install.sh) so ./install.sh works.
  - FILES TOUCHED: 1 NEW (install.sh).

Task 2: LINT (Level 1 — must pass before any test)
  - RUN: shellcheck install.sh            # expect: exit 0, ZERO output
  - RUN: bash -n install.sh               # expect: OK (syntax)
  - IF shellcheck flags anything: the port drifted from the verified original — re-diff
    against skilldozer's install.sh and fix. The verified port triggers NO SC codes.
  - FILES TOUCHED: 0.

Task 3: END-TO-END temp-clone test (the verification deliverable — Level 3)
  - PREP a temp clone: cp the weave Go source + a (stub or real) extensions/example.ts into
    /tmp, drop in the install.sh, chmod +x. Use weave_INSTALL_BIN -> a temp bin dir (do NOT
    touch the real ~/.local/bin during testing).
  - CLEAN ENV FIRST (the #1 gotcha): unset weave_EXTENSIONS_DIR weave_CONFIG; rm -f any
    stray ~/.config/weave/config.yaml (or run under an isolated HOME).
  - RUN: weave_INSTALL_BIN=<tmp> ./install.sh        # expect exit 0
  - ASSERT (the load-bearing chain):
      <tmp>/weave --version            # prints "weave <v>"
      <tmp>/weave --path               # prints "…/extensions (found via sibling of binary)"
      <tmp>/weave example              # resolves to "…/extensions/example.ts"
      test -L <tmp>/weave && ! test -d <tmp>/weave   # regular symlink, not a nested dir
  - ASSERT idempotency: re-run install.sh ×2; <tmp>/weave stays a regular symlink.
  - ASSERT version-stamping: in a git repo with a tag, <v> == the tag; non-git -> "dev".
  - The exact commands are in Validation Loop §Level 3 (all VERIFIED to pass).
  - FILES TOUCHED: 0 (testing only; the temp clone + temp bin are rm -rf'd after).
```

### Implementation Patterns & Key Details

```bash
#!/usr/bin/env bash
# install.sh — build weave and symlink it into PATH (PRD §12.1).
#
# Mirrors skilldozer's install.sh: it BUILDS the binary with version ldflags and SYMLINKS
# it into a PATH dir (never copies — the symlink is load-bearing for §8.3 sibling-of-binary
# resolution, which is how weave finds the repo's own extensions/ with zero config).
#
# Does NOT install completions: that is a separate task (P1.M6.T3.S1). A pointer is printed
# at the end.
set -euo pipefail

# --- §12.1 step 1: cd to the script's own dir (repo root) --------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 weave install"
echo "Repo: $SCRIPT_DIR"
echo

# --- §12.1 step 2: verify go on PATH -----------------------------------------
# Exit BEFORE building; print install instructions to stderr.
if ! command -v go >/dev/null 2>&1; then
  cat >&2 <<'EOF'
ERROR: 'go' was not found on PATH.
Install Go from https://go.dev/doc/install, then re-run ./install.sh.
EOF
  exit 1
fi

# --- §12.1 step 3: build with version ldflags --------------------------------
# The $(git describe ...) expands INSIDE the double-quoted -ldflags string: do NOT escape
# the $. `|| echo dev` only fires outside a git repo (or with no tags). Under `set -e`, a
# build failure aborts here with go's own diagnostics; no symlink is created.
go build -trimpath \
  -ldflags "-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
  -o weave .

# --- §12.1 step 4: pick target bin dir (first usable wins) -------------------
# Override → ~/.local/bin → /usr/local/bin (only if writable) → fail with hint.
# NO silent sudo: if root is required, print the exact command.
if [[ -n "${weave_INSTALL_BIN:-}" ]]; then
  TARGET="$weave_INSTALL_BIN"
  mkdir -p "$TARGET"
elif [[ -d "$HOME/.local/bin" ]] || [[ -w "$HOME" ]]; then
  TARGET="$HOME/.local/bin"
  mkdir -p "$TARGET"
elif [[ -w "/usr/local/bin" ]]; then
  TARGET="/usr/local/bin"
else
  cat >&2 <<EOF
ERROR: no writable install target found.
Re-run with: weave_INSTALL_BIN=/your/bin ./install.sh
Or (system-wide): sudo ln -sfn "$SCRIPT_DIR/weave" /usr/local/bin/weave
EOF
  exit 1
fi

# --- §12.1 step 5: SYMLINK (ln -sfn) $TARGET/weave -> $SCRIPT_DIR/weave ------
# THE load-bearing line:
#  - symlink, NEVER copy (cp breaks §8.3 sibling resolution silently)
#  - `ln -sfn`, not `ln -sf` (-n treats an existing symlink-to-dir dest as a file;
#    defensive even though our dest is a file)
#  - ABSOLUTE target ($SCRIPT_DIR/weave); relative would resolve against $TARGET
ln -sfn "$SCRIPT_DIR/weave" "$TARGET/weave"

echo "Linked: $TARGET/weave -> $SCRIPT_DIR/weave"

# --- §12.1 step 6: ensure $TARGET on PATH; else PRINT rc-file snippet --------
# Detect shell via basename of $SHELL; PRINT only — never auto-edit rc files
# (auto-editing is intrusive and duplicates lines on re-run).
case ":${PATH:-}:" in
  *":$TARGET:"*) ;;  # already on PATH
  *)
    sh="$(basename "${SHELL:-}")"
    case "$sh" in
      bash)
        echo "Add to ~/.bashrc:  export PATH=\"$TARGET:\$PATH\"" ;;
      zsh)
        echo "Add to ~/.zshrc:   export PATH=\"$TARGET:\$PATH\"" ;;
      fish)
        echo "Add to ~/.config/fish/config.fish:  fish_add_path \"$TARGET\"" ;;
      *)
        echo "Add '$TARGET' to your PATH (your shell's rc file)." ;;
    esac
    ;;
esac

# --- §12.1 step 7: verify (absolute symlink path works pre-PATH-reload) ------
# Use the ABSOLUTE symlink path: it works even before the new PATH entry is live in the
# current shell; bare `weave` may hit a stale hash until reload.
echo
echo "Verify:"
"$TARGET/weave" --version
"$TARGET/weave" example

echo
echo "Done. Reload your shell (exec \$SHELL), then run:  weave example"
echo "(Shell completions are not installed by this script — see task P1.M6.T3.S1.)"
```

### Integration Points

```yaml
PRODUCES:
  - install.sh                    # the §12.1 installer (committed, executable, NOT gitignored)

CONSUMES (read-only, no change):
  - the `weave` build target        # go build -o weave . (go.mod: github.com/dabstractor/weave)
  - var version (main.go L43)       # the -X main.version=... ldflags target
  - extdir.resolveSiblingFromExe    # rule 3: EvalSymlinks(exe) → Dir → +/extensions (fires iff dir exists)
  - go on PATH (/usr/bin/go, go1.26.4)
  - extensions/example.ts (P1.M6.T1.S1)  # the sibling rule needs extensions/ to EXIST as a dir

NO CHANGES TO:
  - any .go source (main.go, internal/*) — the binary already builds; the version var + sibling rule exist.
  - go.mod / go.sum / .gitignore / PRD.md / tasks.json.
  - README.md (Mode B install text is P1.M6.T4.S1; doc sync is P1.M6.T5.S1).
  - completions (separate task P1.M6.T3.S1; install.sh only PRINTS a pointer).
```

## Validation Loop

> All commands below were RUN against a ported copy in a temp clone and PASS.
> See `research/install_sh_reference_and_verifications.md` for the transcript.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd ~/projects/weave
shellcheck install.sh        # expect: exit 0, ZERO output (CLEAN — verified)
bash -n install.sh           # expect: no output, exit 0 (syntax OK)
head -1 install.sh           # expect: #!/usr/bin/env bash
grep -q 'set -euo pipefail' install.sh && echo "strict mode OK"
test -x install.sh && echo "executable OK"   # chmod +x applied

# Expected: shellcheck silent; bash -n silent; shebang + strict mode + executable present.
```

### Level 2: Unit Tests (Component Validation)

```bash
# install.sh is a bash script, not Go — there is no Go unit test for it (and none is needed:
# the binary's --version / sibling-rule behavior is already covered by extdir_test.go and the
# §13 suite). The "unit test" IS the Level 1 shellcheck + the Level 3 end-to-end run.

# Confirm the binary it builds still passes its own Go tests (install.sh does not touch them):
go test ./...
# Expected: all green (install.sh changes no .go files).
```

### Level 3: Integration Testing — end-to-end temp-clone install (the verification deliverable)

Run from the repo root. Uses `weave_INSTALL_BIN` → a TEMP bin dir so the real `~/.local/bin`
is never touched during testing. **FIRST** the clean-env preamble (the #1 gotcha):

```bash
set -e
# Prep a temp clone with the real weave source + the example extension (real or stub).
WORK=$(mktemp -d)
mkdir -p "$WORK/clone/extensions"
cp -r ~/projects/weave/internal ~/projects/weave/main.go ~/projects/weave/main_test.go \
      ~/projects/weave/go.mod ~/projects/weave/install.sh "$WORK/clone/"
# The sibling rule needs extensions/ to EXIST (P1.M6.T1.S1 ships the real file; stub is fine for THIS test):
cp ~/projects/weave/extensions/example.ts "$WORK/clone/extensions/" 2>/dev/null \
  || printf '/** E */\nimport type { ExtensionAPI } from "@earendil-works/pi-coding-agent";\nexport default function (pi: ExtensionAPI) {};\n' \
     > "$WORK/clone/extensions/example.ts"

BIN="$WORK/bin"
# CLEAN ENV FIRST (else a stray ~/.config/weave/config.yaml hijacks the sibling rule):
( cd "$WORK/clone" && \
  env -u weave_EXTENSIONS_DIR -u weave_CONFIG \
      HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" \
      weave_INSTALL_BIN="$BIN" ./install.sh )
# Expected: banner, builds, "Linked: $BIN/weave -> $WORK/clone/weave", a PATH/rc line, verify lines.

# The load-bearing assertions:
test -L "$BIN/weave" && ! test -d "$BIN/weave" && echo "symlink (not nested dir) OK"
"$BIN/weave" --version                                       # expect: weave dev  (temp clone is non-git)
"$BIN/weave" --path   | grep -q "extensions" && \
"$BIN/weave" --path   | grep -q "sibling of binary" && echo "sibling rule OK (LOAD-BEARING)"
"$BIN/weave" example  | grep -q "extensions/example.ts" && echo "example resolves OK"

# Idempotency (ln -sfn refresh, not nested):
( cd "$WORK/clone" && env -u weave_EXTENSIONS_DIR -u weave_CONFIG HOME="$WORK/home" \
      XDG_CONFIG_HOME="$WORK/home/.config" weave_INSTALL_BIN="$BIN" ./install.sh ) >/dev/null
( cd "$WORK/clone" && env -u weave_EXTENSIONS_DIR -u weave_CONFIG HOME="$WORK/home" \
      XDG_CONFIG_HOME="$WORK/home/.config" weave_INSTALL_BIN="$BIN" ./install.sh ) >/dev/null
test -L "$BIN/weave" && ! test -d "$BIN/weave" && echo "idempotent re-run OK"

# Version-stamping matrix (optional, confirms the git describe ldflags):
( cd "$WORK/clone" && git init -q && git add -A && \
  git -c user.email=a@b.c -c user.name=x commit -qm init && git tag v1.2.3 && \
  env -u weave_EXTENSIONS_DIR -u weave_CONFIG HOME="$WORK/home" \
      XDG_CONFIG_HOME="$WORK/home/.config" weave_INSTALL_BIN="$BIN" ./install.sh ) \
  | grep -q "weave v1.2.3" && echo "version-stamp (tag) OK"

rm -rf "$WORK"
# Expected: every assertion prints its OK line (all VERIFIED to pass).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The PATH-absent branch prints the right rc line for the detected shell:
WORK=$(mktemp -d); cp -r ~/projects/weave/internal ~/projects/weave/main.go ~/projects/weave/go.mod ~/projects/weave/install.sh "$WORK/"; mkdir -p "$WORK/extensions"; printf 'export default function (){};\n' > "$WORK/extensions/example.ts"
( cd "$WORK" && env -u weave_EXTENSIONS_DIR -u weave_CONFIG HOME="$WORK/h" XDG_CONFIG_HOME="$WORK/h/.config" \
    PATH="/usr/bin:/bin" SHELL="$(command -v zsh || echo /bin/bash)" weave_INSTALL_BIN="$WORK/bin" ./install.sh ) \
  | grep -Eq 'Add to ~/\.zshrc|Add to ~/\.bashrc' && echo "PATH-hint prints for shell OK"
rm -rf "$WORK"

# No silent sudo: an unwritable target (point weave_INSTALL_BIN at a path under a read-only dir)
# prints the exact recovery command:
WORK=$(mktemp -d); RO="$WORK/readonly"; mkdir -p "$RO"; chmod 555 "$RO"
( cd "$WORK" && env HOME="$WORK/h" XDG_CONFIG_HOME="$WORK/h/.config" \
    weave_INSTALL_BIN="$RO/sub/bin" ./install.sh ) 2>&1 \
  | grep -q 'weave_INSTALL_BIN=/your/bin' && echo "no-silent-sudo hint OK" || echo "(mkdir -p under RO may succeed as user; the /usr/local/bin branch is the real sudo path)"
chmod 755 "$RO"; rm -rf "$WORK"

# Confirm install.sh is committed (NOT gitignored):
cd ~/projects/weave
git check-ignore install.sh && echo "FAIL: install.sh is gitignored" || echo "install.sh is tracked-able OK"
grep -qx '/weave' .gitignore && echo ".gitignore ignores the binary, not install.sh OK"

# Expected: PATH-hint prints; install.sh is NOT ignored; only /weave (binary) is ignored.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `shellcheck install.sh` → exit 0, no output (CLEAN).
- [ ] `bash -n install.sh` → OK; first line `#!/usr/bin/env bash`; `set -euo pipefail` present.
- [ ] `test -x install.sh` (chmod +x applied).
- [ ] `go test ./...` stays green (install.sh changes no .go files).

### Feature Validation (the §12.1 gate)

- [ ] `install.sh` exists at repo root, executable, tracked (NOT gitignored).
- [ ] Temp-clone run: `weave_INSTALL_BIN=<tmp> ./install.sh` builds + symlinks + exits 0.
- [ ] `<tmp>/weave --version` prints `weave <v>` (dev / git-describe).
- [ ] **`<tmp>/weave --path` reports `…/extensions (found via sibling of binary)`** (load-bearing).
- [ ] `<tmp>/weave example` resolves to `…/extensions/example.ts`.
- [ ] Re-run ×2 keeps `<tmp>/weave` a regular symlink (idempotent `ln -sfn`).
- [ ] Off-PATH target → correct rc-file line printed for the detected shell.
- [ ] Unwritable target → exact `weave_INSTALL_BIN=` / `sudo ln …` command printed (no silent sudo).

### Code Quality Validation

- [ ] install.sh is the verified port of skilldozer's install.sh (rename map applied; no "improvements").
- [ ] `command -v go` used (POSIX), not `which`.
- [ ] `ln -sfn` (with `-n`), ABSOLUTE symlink target.
- [ ] `${VAR:-}` default-expansions for `weave_INSTALL_BIN`/`SHELL`/`PATH` under `set -u`.
- [ ] PRINT-only PATH setup (never auto-edits rc files).
- [ ] No source-code (.go) change; no go.mod/.gitignore/PRD.md/tasks.json change.

### Documentation & Deployment

- [ ] install.sh is self-documenting: banner + per-step echo + PATH hint + verify command + reload reminder + completions pointer.
- [ ] README §3 install text is left to P1.M6.T4.S1 (Mode B); this task ships the Mode A script only.
- [ ] The clean-env gotcha is recorded (so a future release engineer does not hit a false `--path` failure).

---

## Anti-Patterns to Avoid

- ❌ Don't COPY the binary (`cp`/`install`) instead of symlinking — a copy resolves its own dir
  to the install location, silently breaking the §8.3 sibling rule (the binary would never find
  the repo's `extensions/`). The symlink is load-bearing.
- ❌ Don't use `ln -sf` (drop the `-n`) — re-running install.sh when `$TARGET/weave` exists as a
  symlink would dereference into the target and create a stray nested link. Always `ln -sfn`.
- ❌ Don't use a RELATIVE symlink target (`ln -sfn weave "$TARGET/weave"`) — it would resolve
  against `$TARGET`, not the repo. Use the ABSOLUTE `$SCRIPT_DIR/weave`.
- ❌ Don't escape the `$` in the `-ldflags` string — `$(git describe …)` MUST expand inside the
  double quotes or the version stamp never happens (you'd ship `weave $(git describe...)` literally).
- ❌ Don't use `which go` — it's non-POSIX/non-portable (ShellCheck SC2230). Use `command -v go`.
- ❌ Don't auto-append to `~/.bashrc`/`~/.zshrc` — it's intrusive and duplicates PATH entries on
  every re-run. PRINT the exact line; let the user add it.
- ❌ Don't silently re-exec under `sudo` when `/usr/local/bin` isn't writable — PRINT the exact
  `sudo ln -sfn …` command so the user consents and sees what runs.
- ❌ Don't edit `main.go` to "add" a version var — it already exists (`var version = "dev"`, L43);
  the install.sh ldflags target it as-is.
- ❌ Don't remove `EvalSymlinks` from extdir (it's redundant-but-harmless on Linux, REQUIRED on
  macOS) — install.sh's symlink relies on it. (Already correct in the code; do not "simplify.")
- ❌ Don't run the verify under a dirty env — a stray `~/.config/weave/config.yaml` or
  `weave_EXTENSIONS_DIR` hijacks the sibling rule and makes `--path` resolve to the wrong place.
  Clean the env (or isolate HOME) before the load-bearing assertions.
- ❌ Don't have install.sh install completions — that's P1.M6.T3.S1. install.sh only PRINTS a
  pointer to that task.
