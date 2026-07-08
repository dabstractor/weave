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
