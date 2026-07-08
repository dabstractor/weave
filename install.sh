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
