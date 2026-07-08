# Fish completion for weave.
#
# Install:
#   cp completions/weave.fish ~/.config/fish/completions/weave.fish
#
# Tags are derived DYNAMICALLY from disk by calling `weave --list` and taking
# the TAG column (weave is manifest-free, PRD §2: there is no sidecar catalog
# to read). --list is used (NOT --relative --all) because the TAG column holds
# the canonical RESOLVABLE tag — for a single-file extension the tag is
# `example` (the .ts/.js suffix stripped), whereas --relative --all prints the
# relative PATH `example.ts`, which does NOT resolve as a tag.
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
# The TAG column of `weave --list` holds the canonical resolvable tag (NOT
# --relative --all, whose relative PATH for a single-file ext keeps the .ts/.js
# suffix and does not resolve). Only DATA rows are taken (leading non-space
# first column) so wrapped DESCRIPTION continuation lines are skipped.
complete -c weave -n 'not __fish_seen_subcommand_from check init; and not __fish_prev_arg_in --search -s' \
    -a '(weave --list 2>/dev/null | awk \'NR>1 && $0 !~ /^[[:space:]]/ && NF>0 {print $1}\')' -d 'extension tag'
