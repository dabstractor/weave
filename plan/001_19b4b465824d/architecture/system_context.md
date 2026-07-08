# System Context — weave

## What weave is

`weave` is a tiny, fast Go CLI that resolves a human-friendly **extension tag** to the
**absolute filesystem path** of a locally-stored [pi.dev](https://pi.dev) extension, so it
can be loaded into **pi** on demand:

```bash
pi -e "$(weave my-ext)" -e "$(weave my-other-ext)"
```

It is the extension analog of **skilldozer** (skill path printer for `pi --skill`) and
**mcpeepants** (MCP server config resolver). All three are centralized, on-disk catalogs
addressed by tag, surfaced through a one-liner, living deliberately OUTSIDE pi's
auto-discovery locations.

## Relationship to pi

pi can load extensions via `-e <path>` / `--extension <path>`. Despite the word "file",
`-e` accepts BOTH files and directories (pi's `resolveExtensionEntries` handles both).
`--no-extensions` / `-ne` disables auto-discovery but explicit `-e` paths still work —
verified in pi source (`resource-loader.js`):

```js
const extensionPaths = this.noExtensions
    ? cliEnabledExtensions                                    // only -e paths
    : this.mergePaths(cliEnabledExtensions, enabledExtensions); // -e paths + discovered
```

This makes the canonical `pi --no-extensions -e "$(weave <tag>)"` one-liner provably load
ONLY the resolved extension, never anything from auto-discovery.

## What weave does NOT do

- Does NOT load/execute extensions (that is pi's job via `pi -e`).
- Does NOT install/copy extensions into `~/.pi/...` or run `pi install`.
- Does NOT use Node, Python, jiti, or any runtime at run time (static Go binary).
- Does NOT maintain a catalog index (disk-discovered on every call).
- Does NOT place extensions in any pi auto-discovery location.

## The canonical contract

```
weave <tag>  →  prints exactly one absolute path to stdout (+ trailing newline)
unknown tag  →  prints NOTHING to stdout, error to stderr, exit 1
```

This guarantees `pi -e "$(weave badtag)"` fails loudly rather than passing a garbage path.

## pi extension resolution (verified from source)

pi's `loader.js` (`resolveExtensionEntries` + `discoverExtensionsInDir`):
1. If path is a **file** (`*.ts` or `*.js`) → load directly.
2. If path is a **directory**:
   - If it has `package.json` with `pi.extensions` array → load each declared entry.
   - Else if it has `index.ts` → load it (then `index.js` as fallback).
   - Else → discover `*.ts`/`*.js` files one level deep.
- `.ts` AND `.js` are both recognized (`isExtensionFile` checks both suffixes).
- pi's own discovery is one level deep. weave's catalog walk is deeper but emits paths
  that pi's resolver consumes identically.

## Import path note

The PRD's example extension (§11) imports from `@earendil-works/pi-coding-agent`.
The locally-installed npm pi uses `@mariozechner/pi-coding-agent`. Both are the same
codebase under different package names (OSS vs npm distribution). pi's loader aliases
make both resolvable. The example extension should use the import path from the PRD
(`@earendil-works/pi-coding-agent`) as specified — it is the canonical public name.
