# PRD — Weave

> **Status:** Ready for one-shot implementation. This document is the complete specification.
> **Repo:** `dabstractor/weave` (to be created; clone target `~/projects/weave`).
> **Scope of THIS task for the implementer:** build the tool, the example extension, docs, install, and completions described below. Do not change the product contract without updating this PRD.

---

## 1. Goal

A tiny, fast CLI called **`weave`** that resolves a human-friendly **extension tag** to the **absolute filesystem path** of a locally-stored [pi.dev extension](https://pi.dev), so it can be loaded into **pi** on demand:

```bash
pi -e "$(weave my-ext)" -e "$(weave my-other-ext)"
```

`weave` is to **pi extensions** what `skilldozer` is to **Agent Skills** and what `mcpeepants` (`get-server-config.sh`) is to **MCP server configs**: a centralized, on-disk catalog you address by tag, surfaced through a one-liner.

### Why it exists

pi can load extensions from many "official" discovery locations (see §3). The user wants a **single centralized store that is deliberately NOT one of those locations**, loaded **only** via the explicit `pi -e <path>` flag. `weave` is the resolver that turns a tag into that path.

This PRD is intentionally a near-clone of the **Skilldozer** PRD. The architecture, CLI contract, discovery model, config, install, completions, and acceptance gates are identical in spirit. **The one structural difference** (called out throughout) is the *unit of discovery*: a skill is always a directory containing `SKILL.md`; an extension is either a **single `.ts`/`.js` file** *or* a **directory** pi can resolve. Everything else carries over verbatim.

---

## 2. Hard constraints (non-negotiable)

1. **No catalog index — disk-discovered.** There is no `extensions.json` / index file enumerating the extension *catalog*; the set of extensions is always computed by walking the store on each call, so dropping in a file or directory makes it instantly available — no rebuild step, no index/disk drift to debug. This is specifically about the **catalog**. It does **not** prohibit a small **settings** file for things the filesystem cannot express — today, where the store lives (see §8). Catalog data already on disk is never duplicated into a sidecar; settings are not catalog data.
2. **No auto-discovery by pi.** Extensions live in a location pi does **not** scan. They load **only** through `pi -e "$(weave <tag>)"`. The store must never be `~/.pi/agent/extensions`, a project `.pi/extensions`, a `node_modules` package, a `settings.json` `extensions`/`packages` entry, or any path pi would auto-discover.
3. **`weave <tag>` prints exactly one absolute path** (to stdout, trailing newline) for a resolved extension — the canonical contract. Unknown tag ⇒ **nothing on stdout**, error to stderr, exit 1.
4. **No development of extensions beyond one example.** Ship exactly **one** example extension to prove the pipeline. The repo is a loader, not an extension library.
5. **One-shot buildable.** An implementer must be able to produce the full deliverable from this document alone, with no further questions.

---

## 3. Background — how pi extensions work (factual grounding)

Verified against pi's own docs/help and **source** (`pi --help`, `docs/extensions.md`, `dist/core/extensions/loader.js`, `dist/core/resource-loader.js`):

- An **extension** is a TypeScript (or JavaScript) module exporting a default factory function `(pi: ExtensionAPI) => void | Promise<void>`. Loaded via [jiti](https://github.com/unjs/jiti), so `.ts` works without compilation.
- `--extension <path>` / `-e <path>` **"Load an extension file (can be used multiple times)"**. Despite the word "file", **`-e` accepts a directory too** (see resolution rules below).
- `--no-extensions` / `-ne` **"Disable extension discovery (explicit -e paths still work)"**. Verified in `resource-loader.js`:
  ```js
  const extensionPaths = this.noExtensions
      ? cliEnabledExtensions                                            // only -e paths
      : this.mergePaths(cliEnabledExtensions, enabledExtensions);       // -e paths + discovered
  ```
  This is the exact analog of `--no-skills` + `--skill`, and it is what makes the canonical `pi --no-extensions -e "$(weave <tag>)"` one-liner provably load **only** the resolved extension.
- **Path resolution** (`loader.js`, `resolveExtensionEntries` + `discoverExtensionsInDir`):
  1. If the path is a **file** (`*.ts` or `*.js`) → load it directly.
  2. If the path is a **directory**:
     - If it has a `package.json` with a `pi.extensions` array → load each declared entry (resolved relative to the dir).
     - Else if it has an `index.ts` → load it (then `index.js` as fallback).
     - Else → discover `*.ts`/`*.js` files (and resolvable subdirs) **one level deep** inside it.
  - `.ts` **and** `.js` are both recognized (`isExtensionFile` checks both suffixes).
  - pi's own discovery is **one level deep** ("No recursion beyond one level. Complex packages must use package.json manifest."). `weave`'s catalog walk is deeper (§7) but emits paths that pi's resolver consumes identically.
- **No standard extension metadata.** pi synthesizes a `sourceInfo` for each extension; there is no built-in user-facing name/description field. The only conventional metadata home is a **`package.json`** (standard npm `name`/`description`/`keywords`), which pi itself reads for the `pi.extensions` manifest. `weave` builds its catalog metadata layer from `package.json` plus an optional leading JSDoc comment (§7.3) — it invents **no** new file format.
- Auto-discovery locations (which we deliberately avoid): `~/.pi/agent/extensions/*.ts`, `~/.pi/agent/extensions/*/index.ts`, `.pi/extensions/*.ts`, `.pi/extensions/*/index.ts`, plus `settings.json` `extensions`/`packages` arrays.

**Decision:** `weave` emits the **resolvable path** (the file for a single-file extension, the directory for a dir/package extension) — i.e. exactly what `pi -e` consumes. A `--file` flag forces the resolved entry `.ts`/`.js` path for callers who want it explicitly.

---

## 4. Recommended stack

**Go.** Rationale (identical to Skilldozer):

| Need | Go fit |
|---|---|
| Called inside `$(...)` many times per command → startup latency matters | Go binary starts in <5ms; Node ~50ms+ |
| Trivial install, no runtime | Single statically-linked binary; drop in `PATH` |
| Find the extensions dir relative to the binary, even through a symlink | `os.Executable()` + `filepath.EvalSymlinks()` (Linux/macOS) |
| Walk dirs, parse JSON (`package.json`) + a leading comment, format tables | stdlib `path/filepath.WalkDir`, `encoding/json`, tiny comment extractor |
| Cross-platform releases | `GOOS`/`GOARCH` matrix; `go install` / release binaries |

`weave` has **zero** third-party Go dependencies (it needs only JSON parsing and a trivial comment scan, both in the stdlib). This is strictly simpler than Skilldozer, which pulled in `gopkg.in/yaml.v3` for SKILL.md frontmatter.

Alternatives considered and **rejected**:
- **TypeScript/Node/Bun** — runtime dependency, slower cold start, install friction. (pi itself is Node, so the runtime is present, but distribution and latency are worse. There is also an aesthetic mismatch in a *loader* for pi extensions being itself a non-pi program.)
- **Rust** — equally good binary, but slower compile/more ceremony than this small CLI warrants.

> If the implementer has a strong reason to use Rust instead, the CLI contract (§6) and discovery rules (§7) stay identical; only the build steps change. **Default to Go.**

---

## 5. Target repository layout

```
weave/
├── PRD.md                  # THIS file
├── README.md              # User docs (mirror skilldozer/mcpeepants style)
├── LICENSE                # MIT (match skilldozer/mcpeepants conventions)
├── go.mod                 # module github.com/dabstractor/weave  (no dependencies)
├── .gitignore             # /weave (built binary), coverage, OS files
├── main.go                # entrypoint: arg parsing, dispatch
├── internal/
│   ├── discover/
│   │   ├── discover.go    # scan extensions dir, classify entries, parse metadata
│   │   ├── extension.go   # the Extension struct + metadata extraction (§7.3)
│   │   └── index.go       # the Index() walk + sort
│   ├── resolve/
│   │   └── resolve.go     # tag → extension resolution rules (§7.2)
│   ├── extdir/
│   │   └── extdir.go      # locate the extensions/ dir (§8 priority order)
│   └── ui/
│       └── ui.go          # --list / --search table formatting (ANSI)
├── install.sh             # build + symlink into PATH (mirrors skilldozer install.sh)
├── completions/
│   ├── weave.bash
│   ├── _weave              # zsh
│   └── weave.fish
└── extensions/
    └── example.ts         # the ONE shipped example extension (single-file)
```

`go.mod` module path: `github.com/dabstractor/weave`. Minimum Go: the latest two stable releases (set in `go.mod` `go` directive). **No `go.sum`** (no dependencies).

---

## 6. CLI contract (authoritative)

Binary name: **`weave`**. Flags use POSIX double-dash long form + single-dash short forms. Unknown flags ⇒ error + exit 2. The contract is **byte-identical to Skilldozer's** except the `--file` semantics adapt to the file/dir unit (§6.2).

### 6.1 Commands / flags

| Invocation | Behavior | stdout | exit |
|---|---|---|---|
| `weave <tag> [<tag>...]` | Resolve one or more tags to extension paths. | One **absolute** path per line, in input order. | `0` if all resolve; `1` if **any** fail (and **nothing** is printed) |
| `weave --all` / `-a` | All extensions, resolvable paths. | One absolute path per line (sorted by tag). | `0` |
| `weave --list` / `-l` | Human-readable catalog. | Table: `TAG`, `NAME`, `DESCRIPTION` (wrapped). | `0` (`1` if no extensions found) |
| `weave --search <q>` / `-s <q>` | Substring (case-insensitive) search over tag, `package.json` `name`/`description`/`keywords`, the leading-JSDoc description, `weave.aliases`, and `weave.category`. | Same table format as `--list`, filtered. | `0`; `1` if no matches |
| `weave check` | Validate every extension on disk (see §9). | Report: `OK` lines + any `WARN`/`ERROR` lines. | `0` if clean; `1` if any ERROR |
| `weave init` | First-run setup (see §8.2): prompt for the extensions store dir, create it if missing, write the config, seed a template if empty, validate. Non-interactive: `weave init <dir>` / `weave init --store <dir>`. | The configured store path. | `0` on success; `1` on error/cancel |
| `weave --path` / `-p` | Where is `weave` looking? | Absolute path of the resolved extensions dir. | `0` (`1` if unresolvable) |
| `weave --help` / `-h` | Usage. | Help text (to stdout). | `0` |
| `weave --version` / `-v` | Version. | `weave <version>` (single line). | `0` |

### 6.2 Modifiers (combine with tag resolution or `--all`)

| Flag | Effect |
|---|---|
| `--file` / `-f` | Print the **entry file** path (the `.ts`/`.js` pi loads) instead of the resolvable path. For a single-file extension this is the file itself (no-op). For a dir extension it resolves to `index.ts`/`index.js`; for a package extension to the first existing `pi.extensions` entry. E.g. `weave -f example`. |
| `--no-color` | Disable ANSI color even on a TTY. |
| `--relative` | Print paths relative to the extensions dir instead of absolute (machine-local convenience; default is absolute). |

### 6.3 Default behavior

- **No arguments and no flag** ⇒ print usage to **stderr**, exit `1` (parity with `get-server-config.sh` / skilldozer). (`weave` with just `--help` prints usage to stdout, exit 0.)
- `--help` / `--version` take precedence over everything else.
- Mixing `<tag>` with `--list`/`--search`/`--all` is an error (exit 2): these are mutually exclusive modes.

### 6.4 Error semantics (critical for `$(...)` use)

- **Any** unresolved/ambiguous tag in an `weave <tag>...` invocation ⇒ print **one** error line per problem tag to stderr, print **nothing** to stdout, exit `1`. This guarantees `pi -e "$(weave badtag)"` fails loudly rather than passing a garbage path.
- Ambiguous tag (a short name matching >1 extension) ⇒ stderr lists the candidate full tags, exit `1`.
- Extensions dir cannot be located / weave is unconfigured ⇒ stderr: `weave is not configured; run \`weave init\`` (or, if configured but the dir vanished, the concise reason + fix), exit `1`. Bare tag resolution **never** prompts (see §8.2), so `pi -e "$(weave x)"` fails loudly instead of hanging inside command substitution.

---

## 7. Extension discovery & tag resolution

### 7.1 Discovery

1. Locate the extensions dir (§8).
2. Walk it recursively with a **classify-then-descend** rule. An **extension entry** is one of:
   - A **single-file extension**: a `*.ts` or `*.js` **file** whose base name is **not** `index.ts`/`index.js` (those are directory entries, handled below). The entry path is the file.
   - A **dir/package extension**: a **directory** that is *resolvable* — i.e. it directly contains an `index.ts` or `index.js`, **or** a `package.json` with a `pi.extensions` array naming ≥1 existing entry. The entry path is the directory.
3. **Recursion rule (load-bearing — prevents double-counting):** when a directory is recognized as a dir/package extension, **do not descend into it** (its internal `.ts` files are implementation details of one unit, exactly as pi treats it). Only descend into *plain* directories (those that are NOT themselves resolvable extensions) — these are category folders that may contain nested entries. This is the extensions analog of "a `SKILL.md` makes its dir a skill, full stop."
4. For each entry, parse metadata (§7.3) and capture:
   - `path` — the resolvable path: the file for single-file extensions, the directory for dir/package extensions. **Default output.**
   - `entryFile` — the `.ts`/`.js` file pi loads: the file itself (single-file); `index.ts`/`index.js` (dir); the first existing `pi.extensions` entry (package). **`--file` output.**
   - `relTag` — the entry's path relative to the extensions dir, with OS separators normalized to `/`, **and with a trailing `.ts`/`.js` stripped if the entry is a single file**. **This is the canonical tag.** Examples: `gate.ts` → `gate`; `writing/reddit.ts` → `writing/reddit`; `git-checkpoint/` (dir) → `git-checkpoint`; `writing/snippets/` (resolvable dir) → `writing/snippets`.
   - `kind` — `file` | `dir` | `package` (derived; aids diagnostics/`check`).
   - `name` — `package.json` `name` if present, else `""`.
   - `description` — `package.json` `description` if present, else the leading JSDoc (§7.3), else `""`.
   - `keywords` — `package.json` `keywords` (array) if present, else `[]`.
   - `category` — `package.json` → `weave.category` if present, else `""`.
   - `aliases` — `package.json` → `weave.aliases` (array) if present, else `[]`.
   - `hasPackageJSON` — bool (whether a `package.json` was read for this entry).

> Because everything is read from disk, there is **no index file**. `weave` rebuilds the index on every invocation (fast: it's a directory walk of a small tree plus, per dir entry, a one-file `package.json`/entry-file read).

**Worked example** (store layout → discovered entries):

```
extensions/
├── gate.ts                       → tag "gate"            (file)
├── git-checkpoint/
│   ├── index.ts                  → tag "git-checkpoint"  (dir; NOT recursed into, so utils.ts is NOT a separate entry)
│   └── utils.ts
├── summarizer/
│   ├── package.json              → tag "summarizer"      (package; pi.extensions → ./src/index.ts; NOT recursed into)
│   └── src/index.ts
├── writing/
│   └── reddit-poster.ts          → tag "writing/reddit-poster" (file; "writing" is a plain category dir, recursed into)
└── platform/
    └── linux/                    → tag "platform/linux"  (resolvable dir, e.g. has index.ts)
```

### 7.2 Tag resolution precedence (first match wins; later steps only consulted if earlier produced nothing)

Given an input `tag`:

1. **Exact canonical tag** — equals some extension's `relTag` (case-sensitive). Direct hit ⇒ return it.
2. **Basename** — equals the final `/`-component of some extension's `relTag` (e.g. `reddit-poster` matches `writing/reddit-poster`, `gate` matches `gate`). If **>1** extension matches ⇒ ambiguous error.
3. **`package.json` `name`** — equals some extension's `name` (only entries with a non-empty `name`). If **>1** ⇒ ambiguous error.
4. **Declared alias** — appears in some extension's `weave.aliases`. If **>1** ⇒ ambiguous error.
5. Nothing ⇒ unknown-tag error.

Examples:
- `weave gate` → `…/extensions/gate.ts`
- `weave writing/reddit-poster` → `…/extensions/writing/reddit-poster.ts`
- `weave reddit-poster` → `…/extensions/writing/reddit-poster.ts` (basename, unambiguous)
- `weave @my-org/summarizer` → `…/extensions/summarizer` (by `package.json` `name`)
- `weave -f summarizer` → `…/extensions/summarizer/src/index.ts` (entry file)

### 7.3 Metadata extraction

Extensions have no spec'd frontmatter (unlike Agent Skills' `SKILL.md`). `weave` sources catalog metadata from two standard places, in priority order, and invents **no** new file format:

1. **`package.json`** (read from the extension's directory — present for dir/package extensions). Standard npm fields:
   - `name` (string) → `name`. Used as resolution fallback #3 and the `--list` NAME column.
   - `description` (string) → `description`.
   - `keywords` (array of strings) → `keywords`. Feeds `--search`.
   - Namespaced `weave` object (optional, non-standard but valid in `package.json`):
     - `weave.aliases` (array of strings) → `aliases`. Resolution fallback #4.
     - `weave.category` (string) → `category`. Feeds `--search`.
   - Parse with `encoding/json` (stdlib). Be lenient: unknown keys ignored; wrong-typed fields coerced or ignored (a non-array `keywords` ⇒ `[]`).
2. **Leading JSDoc comment** in the entry file — used as `description` **only when no `package.json` description exists**. This is the natural metadata home for single-file extensions (the dominant case) and the closest standard analog to a SKILL.md description. Extraction rule (precise, bounded, stdlib-only):
   - Read `entryFile`, strip a leading UTF-8 BOM if present.
   - Skip leading whitespace and blank lines.
   - If the next characters are exactly `/**` (a JSDoc block open), find the closing `*/`. Take the lines between. For each line, strip an optional leading run of whitespace followed by a single `*` (and one optional space). Drop empty lines. Join the survivors with a single space. Trim. That concatenation is the description.
   - If there is **no** closing `*/`, or the first non-whitespace token is **not** `/**` (e.g. it's `//`, a `/*` plain block, or code), no description is extracted from the comment.
   - Single-line `/** ... */` blocks work (no inner `*` lines → the content between `/**` and `*/`, trimmed).
3. If neither yields a description, `description = ""` (rendered as `(none)` in `--list`). The extension still resolves and loads fine — metadata is optional.

> No description length limit is enforced (extensions have no spec imposing one). A `package.json` `name` violating npm name rules is not validated as an error (it is only a fallback match key); `check` (§9) does not police `name` format.

---

## 8. Locating the extensions directory

`weave` does not assume the store lives next to the binary or inside a checkout. A small settings file records where the user keeps their extensions, written by `weave init` on first use. The store can live anywhere. **Identical to Skilldozer §8** with `extensions/` substituted for `skills/`.

### 8.1 Configuration file

- Default location: `$XDG_CONFIG_HOME/weave/config.yaml` (→ `~/.config/weave/config.yaml`). Override the file path with `weave_CONFIG=<file>` (useful for tests / multiple profiles).
- This is the **one** fixed, well-known path the binary can bootstrap from; it must not depend on the store location (chicken-and-egg).
- Format: a trivial YAML file (one key) parsed by a ~10-line hand-rolled reader (no dependency — we are not pulling in `yaml.v3` for one key). Minimal valid file:
  ```yaml
  store: /home/dustin/extensions
  ```
- Unknown keys are ignored (room to grow). A missing or unreadable config is treated as "not yet configured" and falls through to §8.3 rules 3-5 — never a hard error.

### 8.2 First-run setup — `weave init`

`weave init` is the documented first command and the **only** place weave prompts interactively.

Interactive (TTY) flow:

1. **Auto-detect from cwd first:** if the current working directory already looks like a store — it contains at least one extension entry at any depth (the store definition, §7.1) — the default store is **cwd** ("detected extensions in <cwd>"). Otherwise the default is `$XDG_DATA_HOME/weave/extensions` (→ `~/.local/share/weave/extensions`). Then prompt: "Where should weave keep your extensions? [<default>]" — Enter accepts the default, typing a path overrides.
2. `mkdir -p` the chosen dir if it does not exist.
3. If the dir is empty, seed `example.ts` as a copy-paste template (a string constant compiled into the binary — **not** `go:embed` of a directory; nothing about the user's collection is compiled in). If the dir already contains extensions, adopt it in place; never clobber or delete.
4. Write `config.yaml` (at `$weave_CONFIG` or the default location) with the absolute `store` path.
5. Print the output of `weave --path` (which rule won) and `weave check`.

Non-interactive: `weave init <dir>` or `weave init --store <dir>` (for scripts / CI). With no `<dir>`/`--store`, the same cwd-auto-detect applies — run from an extension-containing dir and it adopts that dir as the store with no prompt; run from elsewhere and it uses the XDG default. `weave_EXTENSIONS_DIR` set at runtime still bypasses the config entirely.

**Prompt safety (load-bearing):** the bare `weave <tag>` path **never** prompts. If unconfigured (every rule in §8.3 misses), it writes to stderr exactly `weave is not configured; run \`weave init\``, exits `1`, and writes **nothing** to stdout — so `pi -e "$(weave x)"` fails loudly instead of hanging inside command substitution. Any convenience auto-prompt anywhere else must be gated on `isatty(stdin)`.

### 8.3 Resolution priority (first hit wins)

1. **`weave_EXTENSIONS_DIR` env var** — override; if set and an existing dir, use it. Lets CI / tests / temporary redirects win without editing the config.
2. **Config file `store`** (§8.1) — the primary, set by `weave init`.
3. **Sibling of the running binary** (symlink-aware: `os.Executable()` + `filepath.EvalSymlinks()`) — still lets a clone-and-build dev workflow work with zero config.
4. **Walk up from `cwd`** — for `go run` / dev.
5. **None** ⇒ unconfigured: stderr one-line fix (`run \`weave init\``), exit `1`.

`weave --path` reports which rule won, on stderr, with one of the labels: `weave_EXTENSIONS_DIR`, `config file`, `sibling of binary`, `ancestor of cwd`. This matters because a bad `weave_EXTENSIONS_DIR` value is silently ignored and falls through — `--path` is the only way to tell which directory actually won. This remains the single most failure-prone area — implement and test it first (see §13 acceptance).

---

## 9. Validation — `weave check`

Walks the store and reports problems (exit `1` if any ERROR):

- ERROR: duplicate canonical `relTag` across entries (e.g. `foo.ts` **and** `foo/index.ts` both exist → both strip to tag `foo`; or two `bar/` dirs on case-insensitive filesystems). pi would load whichever `pi -e` points at, but a tag collision makes `weave`'s catalog ambiguous, so it is surfaced.
- ERROR: a dir/package entry's `entryFile` does not exist on disk (defensive; should not happen given §7.1 classification, but catches hand-edited `package.json` `pi.extensions` pointing at a missing file).
- ERROR: `package.json` present but unparseable as JSON.
- WARN: a package extension's `package.json` declares non-empty `dependencies` but the entry dir has no `node_modules/` (deps not installed → import failures at `pi -e` load time).
- WARN: an entry has no description (neither `package.json` `description` nor a leading JSDoc) — informational; still resolves.
- WARN: a top-level non-entry directory contains zero discoverable entries at any depth (a stray empty category folder).

Output format: one line per entry → `OK   <relTag> (<name or path-basename>)`; problem entries get one line per finding. Summary line at the end: `N extensions, M errors, K warnings`. `check` prints to **stdout** (pipeable) and signals via exit code; it is NOT subject to §6.4's "nothing on stdout on failure" — that contract is for path emitters used inside `$(...)`.

---

## 10. Extension directory & metadata conventions

An extension under `extensions/<tag>/` (dir/package) or `extensions/<tag>.ts` (single file):

**Single-file** (simplest, matches pi's Quick Start):
```
extensions/
└── gate.ts          # leading /** ... */ JSDoc = description; tag "gate"
```

**Directory with index.ts**:
```
extensions/
└── git-checkpoint/
    ├── index.ts     # entry; leading JSDoc optional if package.json has description
    └── utils.ts     # implementation detail, not a separate entry
```

**Package with dependencies** (richest metadata):
```
extensions/
└── summarizer/
    ├── package.json     # name, description, keywords, pi.extensions, weave.*
    ├── package-lock.json
    ├── node_modules/    # after npm install
    └── src/
        └── index.ts
```

**`package.json`** — standard npm fields plus the namespaced `weave` object (optional, for catalog enrichment only; nothing here is read by pi):
```json
{
  "name": "@my-org/summarizer",
  "description": "Summarize conversation history your way.",
  "keywords": ["compaction", "summary", "context"],
  "pi": { "extensions": ["./src/index.ts"] },
  "weave": {
    "aliases": ["summarise", "compact-helper"],
    "category": "context"
  },
  "dependencies": { "zod": "^3.0.0" }
}
```

- `pi.extensions` is the **pi** manifest field (what pi loads). `weave.aliases` / `weave.category` are **weave** catalog fields. Both coexist fine in one `package.json`.
- The canonical `relTag` is **always** derived from the on-disk path (`summarizer`), independent of `package.json` `name` — mirroring skilldozer's "relTag is canonical" philosophy. `name` is only a resolution fallback and a display column.

---

## 11. The one shipped example extension

Ship **exactly one** example so `--list`/resolution are demonstrable out of the box. It is a **single-file** extension (the dominant pi pattern) carrying a leading JSDoc so `--list` shows a real description:

`extensions/example.ts`:
```typescript
/**
 * Reference example extension for weave. Demonstrates a minimal pi extension
 * and how weave resolves a tag to an absolute path. Registers a harmless
 * /weave-example command and greets on session start. Safe to delete once
 * you add real extensions.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("weave example extension loaded", "info");
  });

  pi.registerCommand("weave-example", {
    description: "Prove the weave example extension is loaded",
    handler: async (_args, ctx) => {
      ctx.ui.notify("Hello from the weave example extension!", "info");
    },
  });
}
```

No other extensions ship in this repo.

---

## 12. Installation

### 12.1 `install.sh` (mirrors skilldozer `install.sh`)

Behavior:

1. `cd` to the script's own directory (the repo root).
2. Verify `go` is on `PATH`; if not, print install instructions and exit `1`.
3. `go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o weave .`
4. Pick a target bin dir in this order: `$weave_INSTALL_BIN` (if set) → `$HOME/.local/bin` (if present or creatable) → `/usr/local/bin` (if writable, else needs `sudo`).
5. **Symlink** (not copy) `<target>/weave` → `<repo>/weave`, so `os.Executable()` resolves back to the repo and finds `extensions/`. If a symlink already exists, refresh it.
6. Ensure the target dir is on `PATH`; if not, print the exact `export PATH=…` line for the detected shell (`~/.bashrc` / `~/.zshrc` / `~/.config/fish/config.fish`).
7. Print a verification command: `weave example`.

> **Why symlink, not copy:** the sibling-of-binary rule (§8.3) then gives a zero-config default store (the repo's own `extensions/`), and `git pull && go build` updates the linked binary in place. Clone users may run `weave init` later only if they want to relocate the store.

### 12.2 `go install`

`go install github.com/dabstractor/weave@latest` is a first-class install path: the binary is self-sufficient. It lands in `$(go env GOPATH)/bin`; on first use the user runs `weave init` (§8.2), which creates the store and writes the config. **No clone required, no `weave_EXTENSIONS_DIR` needed for normal use.**

### 12.3 Releases (optional, phase 2)

If added: a GitHub Actions workflow that builds a `linux/amd64`, `linux/arm64`, `darwin/arm64`, `darwin/amd64` matrix and publishes via `gh release`. Out of scope for the initial one-shot unless trivial.

---

## 13. Acceptance criteria (the implementer must verify all pass)

From a clean clone at `~/projects/weave`:

```bash
# Build
go build -o weave . && echo OK
./weave --version                      # prints: weave <something>

# Discovery + path
test "$(./weave --path)" = "$PWD/extensions"   # sibling-of-binary rule
./weave --list                          # shows the `example` extension
test -f "$(./weave example)"            # resolves to a real file (example.ts)
test -f "$(./weave -f example)"         # -f on a single-file ext = the file itself

# Error contract: unknown tag prints nothing to stdout, exits 1
out=$(./weave nope 2>/dev/null); rc=$?
[ -z "$out" ] && [ "$rc" = "1" ] && echo "unknown-tag contract OK"

# Absolute-path contract (default)
case "$(./weave example)" in /*) echo "absolute OK";; *) echo "FAIL"; exit 1;; esac

# Validation
./weave check                           # exits 0, reports the example as OK

# End-to-end with pi (extension loads ONLY via -e, not auto-discovered).
# --no-extensions disables discovery but explicit -e paths still load (verified in pi source).
pi --no-extensions -e "$(./weave example)" -p "briefly confirm the weave example extension is loaded and name one command it registered" 2>&1 | head
#   ↑ confirm pi's output references the example extension / does not error

# Dir + package styles resolve correctly (temp scaffolding)
mkdir -p /tmp/edz-acc/foo /tmp/edz-acc/bar/src
printf 'export default function () {}\n' > /tmp/edz-acc/foo/index.ts           # dir extension
printf '{"name":"@x/bar","pi":{"extensions":["./src/index.ts"]},"description":"d"}\n' > /tmp/edz-acc/bar/package.json
printf 'export default function () {}\n' > /tmp/edz-acc/bar/src/index.ts       # package extension
weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave --list                         # shows foo and bar
test -d "$(weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave foo)"               # dir → resolvable dir path
test -f "$(weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave -f bar)"            # package -f → bar/src/index.ts
rm -rf /tmp/edz-acc

# Symlink install works (resolve-back-to-repo)
ln -sf "$PWD/weave" /tmp/weave-bin/weave 2>/dev/null || mkdir -p /tmp/weave-bin && ln -sf "$PWD/weave" /tmp/weave-bin/weave
/tmp/weave-bin/weave example             # still resolves to $PWD/extensions/example.ts
weave_EXTENSIONS_DIR="$PWD/extensions" ./weave example   # env override works

# Config + first-run (§8)
mkdir -p /tmp/weave-iso && cp ./weave /tmp/weave-iso/weave && cd /tmp/weave-iso
# unconfigured (clean HOME, no config, no extensions sibling, no walk-up ancestor): hint + exit 1
env -u weave_EXTENSIONS_DIR HOME=/tmp/weave-iso/home XDG_CONFIG_HOME=/tmp/weave-iso/home/.config \
  ./weave x 2>err; rc=$?
[ "$rc" = 1 ] && grep -q 'run `weave init`' err && echo "unconfigured-hint OK"
# non-interactive init creates the store + writes the config
weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave init --store /tmp/weave-store
test -d /tmp/weave-store                                                    # store created
grep -q 'store: /tmp/weave-store' /tmp/weave-iso/cfg.yaml                     # config written
# config rule wins; and env still beats config
weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave --path | grep -q /tmp/weave-store
weave_EXTENSIONS_DIR=/tmp/weave-store weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave --path 2>&1 | grep -q weave_EXTENSIONS_DIR
cd - >/dev/null
```

All of the above must pass. The pi line must show the extension loaded with **`--no-extensions`** (proving we rely solely on the explicit `-e` path, never on auto-discovery).

---

## 14. Shell completions

Ship completions for bash, zsh, fish (parity with skilldozer/mcpeepants). They complete:

- Subcommands/flags after `weave ` / `weave --`.
- **Tags** by invoking `weave --all` (cheap, disk-derived) for positional completion.

Keep them simple: a function that runs `weave --all` and offers the printed paths' basename-or-relTag. Provide an `install.sh` step (already in §12) OR a short note in README to source/copy the completion file. If time-boxed, completions are the **only** deliverable that may be deferred — flag it clearly in the PR if so.

---

## 15. README.md outline

Mirror the skilldozer README's tone and structure:

1. **Title + one-liner:** "Standalone extension loader for pi — resolves an extension tag to an absolute path for `pi -e`."
2. **Why:** centralized extensions, **not** in any pi discovery location, loaded only on demand.
3. **Install:** `install.sh` (symlink) / `go install` (first-class) / from-source. First run: `weave init` (prompts for the store dir, writes the config).
4. **Usage:** the canonical `pi -e "$(weave tag)"` example, multi-extension example, `-f`, `--list`, `--search`, `--all`, `check`, `--path`.
5. **How extensions are organized:** single file (`gate.ts`), dir (`foo/index.ts`), package (`foo/package.json` + `pi.extensions`); the tag = relative path (`.ts`/`.js` stripped for files); the discovery rules (§7).
6. **Adding an extension:** drop a `.ts` file or a dir under `extensions/`; optional `package.json` for metadata; run `weave check`.
7. **How `weave` finds the store:** §8 — `weave init` writes a config pointing at the store; `weave_EXTENSIONS_DIR` overrides it; sibling / walk-up are zero-config dev fallbacks.
8. **Constraints:** no catalog index (disk-discovered); a settings config file is fine; never auto-discovered by pi; loads only via `-e`.

---

## 16. `.gitignore`

```
/weave
/dist
*.test
*.out
.DS_Store
```

(`/weave` ignores the locally-built binary; everything else is committed, including `extensions/example.ts`.)

---

## 17. Constraints & guardrails (do NOT do these)

- ❌ Do **not** add a **catalog** index/manifest (e.g. `extensions.json` enumerating extensions). The catalog is always walked from disk. A **settings** file (store location, etc.) is expected and fine — see §8; the rule is only that catalog data already on disk is never duplicated into a sidecar.
- ❌ Do **not** place extensions in any pi auto-discovery location (`~/.pi/agent/extensions`, `.pi/extensions`, a `node_modules` package, a `settings.json` `extensions`/`packages` entry). The store is loaded **only** via `-e`.
- ❌ Do **not** make `weave` install/copy extensions into `~/.pi/...` or run `pi install`. It only prints paths.
- ❌ Do **not** print anything to stdout on a failed/unknown tag resolution (breaks `pi -e "$(weave x)"`).
- ❌ Do **not** require Node, Python, jiti, or any runtime at *run* time (build-time `go` is fine). `weave` is a static binary; it does **not** load/execute extensions (that is pi's job via `pi -e`).
- ❌ Do **not** invent a new metadata file format for extensions. Metadata comes from `package.json` (standard) and the leading JSDoc comment (standard) only — see §7.3.
- ❌ Do **not** ship more than the one example extension.

---

## 18. Suggested build order (for the one-shot pass)

1. `go.mod` + `internal/extdir` + `main.go --path` → prove location resolution (§8). **Hardest part; do first.** (Port skilldozer `internal/skillsdir` nearly verbatim — only the env-var name, the sibling dir name `extensions`, and the "is this dir a store" predicate change; see §7.1 for the predicate.)
2. `internal/discover` (walk + classify + metadata parse) → `--list`. (The classify-then-descend rule in §7.1 and the JSDoc extractor in §7.3 are the new logic vs skilldozer.)
3. `internal/resolve` → `weave <tag>`, `-f`, `--all`, `--relative`.
4. `--search`, `check`.
5. `--help`/`--version`/error semantics + exit codes (§6.4).
6. Example extension + run §13 acceptance.
7. `install.sh` (symlink) + README + `.gitignore` + LICENSE.
8. Completions.

---

## 19. Decisions log (assumptions made in lieu of asking — override if you disagree)

| # | Decision | Default chosen | Rationale |
|---|---|---|---|
| 1 | Repo / binary name | **`weave`** | Sibling to `skilldozer` under `dabstractor/`; short to type inside `pi -e "$(weave …)"`; `extensiondozer` is the long-form alternative if a more explicit name is preferred. Trivially renamable before implementation (module path, env prefix, config dir, completions all derive from it). |
| 2 | Visibility | **public** | Matches mcpeepants + skilldozer + user's other repos |
| 3 | Language | **Go** | Static binary, instant startup, symlink-aware path resolution; zero third-party deps (strictly simpler than skilldozer) |
| 4 | Output unit | **resolvable path** (file for files, dir for dirs); `--file` for the entry `.ts`/`.js` | Mirrors skilldozer's "directory default, `--file` for manifest"; `pi -e` accepts both and resolves identically |
| 5 | Catalog index | **none** — walked from disk each call | Small, hand-edited catalog; an index would drift (same reasoning as skilldozer) |
| 6 | Canonical tag | relative path under `extensions/`, `.ts`/`.js` stripped for single files; basename/`name`/alias fallbacks | Inferable from disk; tolerant of common usage |
| 7 | Discovery unit | **file** (`*.ts`/`*.js`, not `index.*`) **or** resolvable dir (`index.*` or `package.json`+`pi.extensions`); recurse only into plain dirs | Matches pi's own resolution semantics exactly; avoids double-counting a dir extension's internal helper files |
| 8 | Search metadata | `package.json` `name`/`description`/`keywords` + leading JSDoc + `weave.aliases`/`weave.category` | Uses only standard metadata homes; no invented file format |
| 9 | Metadata parser | stdlib `encoding/json` + a ~20-line JSDoc extractor | No dependencies (skilldozer needed `yaml.v3` for frontmatter; extensions have none) |
| 10 | Install method | `go install` / release binary / `install.sh`; `weave init` configures the store | No clone forced; binary is self-sufficient; first run prompts for (or is told via flag/env) the store dir |
| 11 | Shipped extensions | exactly one `example.ts` (single-file) | Proves the dominant pipeline (single-file + JSDoc metadata); repo is a loader, not a library |
| 12 | Settings file | `$XDG_CONFIG_HOME/weave/config.yaml`, key `store` (trivial YAML, hand-parsed); `weave_CONFIG` overrides the path | Fixed home so the binary can bootstrap without being told; one key does not justify a YAML dependency |
| 13 | First-run UX | `weave init` prompts interactively; bare tags never prompt (any auto-prompt TTY-gated) | Protects the `pi -e "$(weave x)"` contract from hanging inside command substitution |
| 14 | Discovery order | env `weave_EXTENSIONS_DIR` → config `store` → sibling of binary → walk-up → "run `weave init`" | Env overrides config for CI/tests; heuristics kept as zero-config dev fallbacks |
| 15 | Store dir name | `extensions/` | Direct analog of skilldozer's `skills/` |
| 16 | `init` cwd auto-detect | If cwd contains any extension entry (any depth), default the store to cwd; else `$XDG_DATA_HOME/weave/extensions` | Run `weave init` inside an existing extensions dir and it adopts it in place, no typing |
| 17 | `check` deps-installed WARN | WARN when a package extension has `dependencies` but no `node_modules/` | Catches the #1 real-world `pi -e` load failure (uninstalled deps) at validation time |
