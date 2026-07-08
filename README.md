# weave

Standalone extension loader for pi. Resolves an extension tag to an absolute
path for `pi -e`.

## Why

`weave` gives you a centralized, on-disk catalog of pi extensions addressed by
short tags. The catalog lives **deliberately outside** every directory pi scans,
so extensions never enter your context automatically. They load **only on
demand** when you pass an explicit `-e`:

```bash
pi -e "$(weave example)"
```

If a tag is unknown, `weave` prints nothing and exits 1, so the `$(...)` fails
loudly instead of handing pi an empty path.

## Install

Three paths. `./install.sh` is recommended.

**A. `./install.sh` (recommended)**

Builds the binary with version info and symlinks it into `~/.local/bin`
(or `$weave_INSTALL_BIN`, or `/usr/local/bin` if that is what is writable):

```bash
./install.sh
```

The install **symlinks** rather than copies. That matters: `weave` resolves its
own executable path back through the symlink, which is how it finds the
adjacent `extensions/` directory with no env var.

**B. `go install`**

```bash
go install github.com/dabstractor/weave@latest
```

`go install` lands the binary in `$(go env GOPATH)/bin`. On first use, run
`weave init` (see First run, below). It creates the store and writes the
config. No clone required, and no `weave_EXTENSIONS_DIR` needed for normal use.

**C. From source**

```bash
go build -o weave . && ./weave example
```

or build, then symlink it yourself:

```bash
go build -o weave .
ln -sfn "$PWD/weave" ~/.local/bin/weave
```

Run `./weave example` from the repo, or use the symlink from anywhere.

### First run

Whichever install path you used, run `weave init` once:

```bash
weave init
```

It prompts for the directory where `weave` should keep your extensions
(defaulting to `$XDG_DATA_HOME/weave/extensions`, or the current directory if it
already looks like an extension store), creates it, seeds an `example.ts`
template if it is empty, and writes the config pointing at it. For scripts and
CI, skip the prompt:

```bash
weave init /path/to/store      # positional
weave init --store /path/to/store
```

On success, the configured store path is the **first line on stdout**, so
`STORE="$(weave init --store /path | head -1)"` works in scripts. The rest of
the stdout is the post-setup `check` report; the seeded/adopted status and the
discovery rule go to stderr.

`weave` does **not** expand `~` itself. If you want a home-relative path,
leave it unquoted so your shell expands it before `weave` sees it
(for example `weave init --store ~/extensions`), or pass an absolute path.
A quoted `~/...` or a `~` typed at the prompt is taken literally, because
`weave` only absolutizes the value it receives.

## Shell completions

`weave` ships dynamic completions for bash, zsh, and fish. Tag completion is
not a static list: the shell calls `weave --list` at completion time and takes
the TAG column, so it never goes stale as you add extensions. (The TAG column
holds the canonical resolvable tag — `example` for a single-file extension,
with the `.ts`/`.js` suffix stripped — whereas `weave --relative --all` prints
relative *paths* like `example.ts`, which do not resolve as tags.)

**bash** (one of):

```bash
source /path/to/weave/completions/weave.bash
cp completions/weave.bash ~/.local/share/bash-completion/completions/weave
cp completions/weave.bash /etc/bash_completion.d/weave
```

**zsh** (one of):

```bash
cp completions/_weave ~/.zsh/completions/_weave
cp completions/_weave /usr/local/share/zsh/site-functions/_weave
```

then ensure this is in your `.zshrc`:

```bash
autoload -U compinit && compinit
```

**fish**:

```bash
cp completions/weave.fish ~/.config/fish/completions/weave.fish
```

`install.sh` does not install completions automatically; copy the file you
want as shown above.

## Usage

The canonical one-liner, first:

```bash
pi -e "$(weave example)"
```

To load **only** weave extensions and none of pi's auto-discovered ones, pass
pi's `--no-extensions` (short `-ne`); explicit `-e` paths still load:

```bash
# Only the extensions weave resolves; skip pi's own discovery
pi --no-extensions -e "$(weave example)"
```

Everything else, commented:

```bash
# Resolve a tag to an absolute path (default: the resolvable path)
weave example                       # -> /.../extensions/example.ts

# Print the entry file path instead (-f / --file)
weave -f example

# Load several extensions into pi in one command
pi -e "$(weave writing/reddit)" -e "$(weave example)"

# Resolve multiple tags at once (one absolute path per line, input order)
weave example writing/reddit

# Human-readable catalog and substring search
weave --list
weave --search reddit            # matches tag / name / description / keywords / aliases / category

# Print every extension path, sorted by tag
weave --all

# Validate every extension on disk
weave check

# Where is the resolved extensions directory? (its discovery rule prints to stderr)
weave --path                        # -> /.../extensions (stderr: found via sibling of binary)

# Print paths relative to the extensions directory instead of absolute
weave --relative example

# Disable ANSI color even on a TTY (for --list / --search tables)
weave --no-color --list

# Version is the git-describe value (dynamic, not a fixed string)
weave --version

# Short flags combine (-af) and long flags accept --flag=value (--search=reddit)
```

**Error contract.** An unknown tag prints **nothing** to stdout and exits 1
(the error goes to stderr only). That is why
`pi -e "$(weave badtag)"` fails loudly instead of loading nothing. When
multiple tags are given, any unresolved tag causes nothing to be printed and
exit 1, so `pi` never sees a partial result.

The `--path`, `--list`, `--search`, and `--all` modes are mutually exclusive:
combining any two exits 2, as does combining a tag with any of them (a tag
resolves one path; those modes inspect the whole store). Bare `weave <tag>`
never prompts: if unconfigured, it prints `weave is not configured; run \`weave init\`` to stderr, writes nothing to stdout, and exits 1.

`weave --help` lists every flag.

## How extensions are organized

Extensions live in the `extensions/` directory at the store root. An extension
is one of three kinds:

- a single **file**: a `*.ts` or `*.js` file whose base name is not
  `index.ts` or `index.js`
- a **directory**: a directory that directly contains `index.ts` or `index.js`
- a **package**: a directory with a `package.json` whose `pi.extensions` array
  names at least one existing entry

During discovery, `weave` skips `node_modules/` and `.git/` directories, and
any file or directory whose name starts with `.` (hidden entries). So an
`npm install` at the store root will not pollute the catalog, and a stray
`.secret.ts` is ignored.

The canonical **tag** is the entry's path **relative to `extensions/`**, with
`/` separators, and a trailing `.ts`/`.js` stripped for single files. It is
**not** the `package.json` `name`.

```text
extensions/gate.ts                    -> tag gate
extensions/writing/reddit-poster.ts   -> tag writing/reddit-poster
extensions/git-checkpoint/ (dir)      -> tag git-checkpoint
extensions/summarizer/ (package)      -> tag summarizer
```

Once a directory is recognized as a directory or package extension, `weave`
does not descend into it (its internal `.ts` files are one unit). Only plain
directories (category folders) are descended.

Tag resolution tries, in order:

1. the exact canonical tag (`writing/reddit-poster`)
2. the final path segment / basename (`reddit-poster`)
3. the `package.json` `name`
4. a declared alias (see `weave.aliases`)
5. else: unknown

So `weave gate`, `weave writing/reddit-poster`, `weave reddit-poster` (if
unique), and `weave @my-org/summarizer` (matching a `package.json` `name`)
all resolve.

**Reserved tag names.** `check` and `init` are subcommand names, so they
never resolve as tags: `weave check` runs validation and `weave init` runs
first-run setup. An extension whose canonical tag collides is still fully
usable, just not via that one tag: it appears in `--list` and `--all`, and
resolves by a nested path, by its `package.json` `name`, or by a declared
alias.

## Adding an extension

Drop a `.ts` file or a directory under `extensions/`.

For a single file, a minimal template with a leading JSDoc block (`weave`
pulls the description from the leading `/** ... */` when there is no
`package.json`):

```typescript
/**
 * One or two sentences describing what the extension does.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  /* ... */
}
```

For a directory or package, an optional `package.json` carries metadata:

```json
{
  "name": "my-extension",
  "description": "One or two sentences describing what the extension does.",
  "keywords": ["example", "demo"],
  "weave": {
    "aliases": ["demo"],
    "category": "meta"
  }
}
```

- `name`: a resolution fallback and the `--list` NAME column.
- `description`: the `--list` DESCRIPTION column (the JSDoc block is used when
  absent).
- `keywords`, `weave.category`, `weave.aliases`: feed `--search` and
  resolution. Unknown keys are ignored.

`extensions/example.ts` is a copy-pasteable template; start from it.

When you are done, validate everything on disk:

```bash
weave check
```

Output:

```text
OK    example (example)
1 extensions, 0 errors, 0 warnings
```

## How `weave` finds the store

`weave` locates `extensions/` by this priority (first hit wins):

1. **`weave_EXTENSIONS_DIR` env var**: override; if set and an existing dir,
   use it (symlinked paths are resolved). Lets CI, tests, and temporary
   redirects win without editing the config.
2. **Config file `store`**: the primary, set by `weave init`. The config
   lives at `$XDG_CONFIG_HOME/weave/config.yaml` (becomes
   `~/.config/weave/config.yaml`); override the file path with
   `weave_CONFIG=<file>` (handy for tests and multiple profiles). Minimal
   valid file:

   ```yaml
   store: /home/you/extensions
   ```

   A missing or unreadable config is treated as "not yet configured" and falls
   through to the rules below. Never a hard error.
3. **Sibling of the running binary** (symlink-aware: `os.Executable()` plus
   `EvalSymlinks()`): still lets a clone-and-build dev workflow work with no
   config. This is the rule a `./install.sh` symlink install relies on; a
   copy would break it silently.
4. **Walk up from `cwd`**: for `go run` and dev.
5. **None**: unconfigured. `weave` prints
   `weave is not configured; run \`weave init\`` to stderr, writes nothing to
   stdout, and exits 1.

`weave --path` reports the winning directory on stdout and the matching rule
on stderr, one of `weave_EXTENSIONS_DIR`, `config file`, `sibling of binary`,
or `ancestor of cwd`. The stderr label matters when `weave_EXTENSIONS_DIR` is
typo'd: a bad value is silently ignored and discovery falls through to a
lower rule, so the `--path` label is the only way to tell the env var was
skipped.

## Constraints

`weave` is deliberately a thin path printer.

- **No catalog index.** There is no `extensions.json` and no manifest
  enumerating extensions. The catalog is always walked from disk on each call.
  A settings config file (the store location, written by `weave init`) is
  expected and fine; the rule is only that catalog data already on disk is
  never duplicated into a sidecar.
- **Never auto-discovered by pi.** The extensions store does **not** live in
  any directory pi scans. It is never:
  - `~/.pi/agent/extensions`
  - a project `.pi/extensions`
  - a `node_modules` package
  - a `package.json` with a `pi.extensions` field consumed by pi's own
    discovery
  - any path pi would auto-discover
- **Loaded only via `-e`.** An extension enters your context only when you ask
  for it explicitly: `pi -e "$(weave <tag>)"`.
- **`weave` only ever prints paths.** It never copies or installs extensions
  into `~/.pi/...`. Where the path points is up to you.
- **Zero runtime dependencies.** Build-time needs Go; the resulting binary
  stands alone.

Licensed under the MIT License.
