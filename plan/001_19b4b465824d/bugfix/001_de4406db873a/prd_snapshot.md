# Bug Fix Requirements

## Overview

Creative end-to-end QA of the `weave` implementation against PRD rev. (PRD.md).
Testing covered: all §13 acceptance gates (PASS), tag resolution §7.2 (all 4
precedence steps + ambiguity + atomicity), discovery §7.1 (file/dir/package
shapes, classify-then-descend), JSDoc extraction §7.3, `check` §9, `init` §8.2,
config/extdir §8, modifiers §6.2, arg parser §6.3 (bundles, `=` forms,
precedence), search, completions, install.sh, and the live `pi -e` load.

Overall quality is high: every PRD §13 acceptance command passes, the example
extension loads cleanly in `pi --no-extensions -e`, exit codes and the
`$(...)` stdout-empty contract are correct, and lenient `package.json` parsing
works. The issues below are real but mostly narrow; one Major bug (multi-entry
`pi.extensions`) silently breaks a PRD-supported package shape and cascades into
spurious duplicate-tag errors. The rest are edge cases and footguns.

- Total distinct scenarios exercised: ~90
- Passing: ~83
- Failing/issue-bearing: 6 documented below (1 Major, 5 Minor)

Reproductions use a freshly built binary at `/tmp/weave` (`go build -o /tmp/weave .`)
and a throwaway store via `weave_EXTENSIONS_DIR=<dir>`. No real HOME/config was
mutated (a side-effect into `~/.local/share/weave` from one interactive-init
probe was removed).

---

## Critical Issues (Must Fix)

None. Core functionality works for the common cases; all PRD §13 acceptance
gates pass; the example extension loads in `pi`.

---

## Major Issues (Should Fix)

### Issue 1: Package with multiple `pi.extensions` entries is invisible when the FIRST entry is missing

**Severity**: Major
**PRD Reference**: §7.1 (entry classification: "a `package.json` with a
`pi.extensions` array naming **≥1 existing entry**"; entryFile: "**the first
existing `pi.extensions` entry**"). Also contradicts §9 (the package never
becomes an entry, so its real problem is never reported).

**Expected Behavior**: A package whose `package.json` declares
`pi.extensions: ["./missing.ts", "./src/real.ts"]` (first entry absent, a later
entry present) MUST be classified as a package extension. The directory is the
resolvable path; `entryFile` (for `-f`) is the **first existing** entry
(`./src/real.ts`). Per §7.1: "naming ≥1 existing entry" qualifies the package.

**Actual Behavior**: `discover.classifyDir` case (a) only stat-checks
`entries[0]`. If `entries[0]` does not exist it falls through, never trying the
later entries. The directory is then treated as a plain category folder,
descended into, and the existing entry file(s) are mis-discovered as separate
single-file extensions — producing a cascade of wrong output:

- `weave <pkg>` and `weave -f <pkg>` ⇒ `unknown extension tag` (exit 1).
- `--list` does NOT show the package; instead it shows the entry files as stray
  single-file extensions under a fabricated nested tag (losing all
  `package.json` metadata: name, description, keywords).
- When two real entry files share a stem after `.ts`/`.js` stripping (e.g.
  `real.js` + `real.ts`), both collapse to the same `relTag` and `check`
  reports a spurious **duplicate relTag ERROR**, masking the real cause.

**Steps to Reproduce**:
```bash
go build -o /tmp/weave .
rm -rf /tmp/bug1 && mkdir -p /tmp/bug1/mypkg/src
printf '{"name":"mypkg","description":"My package","pi":{"extensions":["./src/MISSING.ts","./src/real.ts"]}}\n' > /tmp/bug1/mypkg/package.json
printf '/** real entry */\nexport default function(){}\n' > /tmp/bug1/mypkg/src/real.ts

weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave mypkg      # ACTUAL: unknown extension tag "mypkg" (rc=1)
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave -f mypkg   # ACTUAL: unknown (EXPECTED: /tmp/bug1/mypkg/src/real.ts)
weave_EXTENSIONS_DIR=/tmp/bug1 /tmp/weave --list     # ACTUAL: shows "mypkg/src/real", NOT package "mypkg"
```
With two existing siblings (`real.js` + `real.ts`) the fall-through additionally
yields `ERROR duplicate relTag "pkg/src/real"` from `check`.

**Aggravating factor — discovery/init inconsistency**: `extdir.hasPiExtensions`
(the predicate behind `HasExtensionEntry`, used by `weave init` cwd auto-detect
and the §8.3 walk-up qualifier) correctly ITERATES all entries and returns true
if ANY exists. So `weave init` run from inside such a package dir will "Adopt
existing store", but `weave --list` on that same adopted store then
mis-discovers it. The two code paths disagree on what a package extension is.

**Root cause**: `internal/discover/discover.go`, `classifyDir` case (a):
```go
entries := toStringSlice(pkg.Pi.Extensions)
if len(entries) > 0 {
    entryFile := filepath.Join(path, entries[0])   // <-- only entries[0]
    if fileExists(entryFile) { ... }
    // pi.extensions names a non-existent file → NOT a package; fall through.
}
```
Contrast the correct iteration already present in `internal/extdir/extdir.go`
`hasPiExtensions`.

**Suggested Fix**: In `classifyDir` case (a), iterate `entries` and select the
FIRST existing one (mirroring `hasPiExtensions` and PRD §7.1 "first existing
entry"). Only fall through if NO entry exists. Add a test for
`["./missing.ts", "./src/real.ts"]`. (The existing `TestClassifyDirPackageNonExistentEntry`
only covers the all-missing single-entry case, so this gap went undetected.)

---

## Minor Issues (Nice to Fix)

### Issue 2: Empty single-line JSDoc `/**/` extracts `/` as the description

**Severity**: Minor
**PRD Reference**: §7.3.2 ("Single-line `/** ... */` blocks work (no inner `*`
lines → the content between `/**` and `*/`, trimmed)" and "If neither yields a
description, `description = ""` (rendered as `(none)`)").

**Expected Behavior**: A file beginning with exactly `/**/` (open immediately
followed by close, no content) is a valid empty JSDoc block; its description is
`""`, rendered as `(none)` in `--list`.

**Actual Behavior**: The description is `/` (a single slash), shown in `--list`
and indexed by `--search`. The single-line handler strips the `/**` opener
(leaving `/`) and then looks for `*/` in the remainder; because the closer's `*`
was consumed as the opener's last star, no `*/` is found and the stray `/` is
kept as content. All other empty forms (`/** */`, `/**\n*/`, `/***/`) are
correct.

**Steps to Reproduce**:
```bash
rm -rf /tmp/bug2 && mkdir -p /tmp/bug2
printf '/**/\n' > /tmp/bug2/empty.ts          # exactly /**/
weave_EXTENSIONS_DIR=/tmp/bug2 /tmp/weave --list
# ACTUAL:  empty  (none)  /
# EXPECTED: empty  (none)  (none)
```

**Root cause**: `internal/discover/jsdoc.go`, `ExtractJSDoc`, single-line branch
(`i == startIdx && i == closeIdx`): `rest := strings.TrimPrefix(line, "/**")`
then `Index(rest, "*/")`. For `/**/`, `rest == "/"` contains no `*/`.

**Suggested Fix**: Detect the degenerate `/**/`/`/**<whitespace>*/` empty
single-line case (e.g. after stripping the opener, if the remainder is empty or
contains only the closing `/`, treat content as empty), or find the closing
`*/` on the original line accounting for opener/closer overlap.

---

### Issue 3: An `index.ts`/`index.js` at the store ROOT collapses the entire store into one extension tagged `.`

**Severity**: Minor
**PRD Reference**: §7.1 (classify-then-descend; the store root is conceptually a
container, not itself an extension). PRD's worked example always treats the root
as a multi-entry container.

**Expected Behavior**: A stray `index.ts` at the extensions root should not make
the whole store disappear as a single unit. At minimum its tag should not be the
bare `.` (relpath of root to itself).

**Actual Behavior**: `discover.Index` runs `classifyDir` on the root. Because
the root contains `index.ts`, case (c) classifies the ROOT directory itself as a
dir extension with `relTag = filepath.Rel(root, root) = "."`, then returns
`filepath.SkipDir` — pruning the ENTIRE store. Every other extension under the
root becomes invisible, and the store resolves to a single tag `.`.

**Steps to Reproduce**:
```bash
rm -rf /tmp/bug3 && mkdir -p /tmp/bug3/sub
printf '/** root idx */\nexport default function(){}\n' > /tmp/bug3/index.ts
printf '/** sub ext */\nexport default function(){}\n' > /tmp/bug3/sub/x.ts
weave_EXTENSIONS_DIR=/tmp/bug3 /tmp/weave --list     # ACTUAL: only "." ; sub/x invisible
weave_EXTENSIONS_DIR=/tmp/bug3 /tmp/weave sub/x       # ACTUAL: unknown extension tag "sub/x"
```

**Suggested Fix**: Treat the walk root as a container that is never itself
classified as an extension (skip classification for `path == root`, always
descend), or at least reject a `.` relTag. Low frequency in practice but a sharp
footgun for anyone using `index.ts` as a scratch file at the store root.

---

### Issue 4: A symlinked `weave_EXTENSIONS_DIR` is accepted by `--path` but yields an empty catalog

**Severity**: Minor (cross-platform relevance: on macOS `/tmp` is a symlink to
`/private/tmp`, so `weave_EXTENSIONS_DIR=/tmp/...` would silently break)

**PRD Reference**: §8.3 rule 1 ("if set and an existing dir, use it") and §8.3
("`--path` is the only way to tell which directory actually won").

**Expected Behavior**: If `weave_EXTENSIONS_DIR` names a directory (even via a
symlink), `weave` should both report it via `--path` AND discover its contents.

**Actual Behavior**: `extdir.findEnv` validates the path with `os.Stat` (which
FOLLOWS the symlink) and returns the symlink path verbatim (deliberately NOT
`EvalSymlinks`-resolved). `discover.Index` then calls `filepath.WalkDir` on that
symlink path; WalkDir `Lstat`s the root, sees `ModeSymlink` (not a dir), and
walks nothing. Result: `--path` prints the dir and exit 0, but `--list` says
"no extensions found" (exit 1) and every tag resolves to unknown — a confusing
`--path`-vs-reality mismatch.

**Steps to Reproduce**:
```bash
rm -rf /tmp/s1 /tmp/s1link && mkdir -p /tmp/s1
printf '/** ext */\nexport default function(){}\n' > /tmp/s1/foo.ts
ln -sfn /tmp/s1 /tmp/s1link
weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --path    # /tmp/s1link  (rc=0)
weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave --list    # "no extensions found" (rc=1)
weave_EXTENSIONS_DIR=/tmp/s1link /tmp/weave foo       # unknown extension tag (rc=1)
```

**Suggested Fix**: Resolve the root symlink before walking in `discover.Index`
(e.g. `filepath.EvalSymlinks(root)` for the walk root, or stat-follow and walk
the resolved target), OR `EvalSymlinks` the env value in `findEnv`. The "use it"
intent in §8.3 is functional, not literal-preservation.

---

### Issue 5: `weave check` reports a package with a missing `pi.extensions` entry as an "empty category folder" WARN, not the §9 ERROR

**Severity**: Minor
**PRD Reference**: §9 ("ERROR: a dir/package entry's `entryFile` does not exist
on disk ... catches hand-edited `package.json` `pi.extensions` pointing at a
missing file"). Note the PRD tension: §7.1 says a package.json whose
`pi.extensions` names a non-existent file is NOT a package, so the §9 ERROR is
inherently hard to reach — but §9 explicitly asks for it.

**Expected Behavior**: A `package.json` whose `pi.extensions` points at a
missing entry file should surface a clear, actionable error about the bad
`pi.extensions` path.

**Actual Behavior**: Per §7.1 the dir is not classified as a package (no
existing entry), has no `index.*`, so it is descended into as a plain category
folder. `check` then runs `discover.Index` on it, finds zero entries, and emits
`WARN empty category folder: <name>` — hiding the real cause (a broken
`pi.extensions` path) behind an unrelated-sounding warning, and at WARN
severity (exit 0) rather than ERROR (exit 1).

**Steps to Reproduce**:
```bash
rm -rf /tmp/bug5 && mkdir -p /tmp/bug5/badpkg
printf '{"name":"badpkg","description":"d","pi":{"extensions":["./missing.ts"]}}\n' > /tmp/bug5/badpkg/package.json
weave_EXTENSIONS_DIR=/tmp/bug5 /tmp/weave check
# ACTUAL:   WARN  badpkg (badpkg): empty category folder: badpkg   (exit 0)
# EXPECTED: an ERROR citing the missing pi.extensions entry         (exit 1)
```

**Suggested Fix**: In `check` (or in `classifyDir`'s fall-through), detect a
directory that has a `package.json` with a non-empty `pi.extensions` whose
entries are ALL missing, and report an ERROR naming the bad entry (rather than
letting it degrade to an empty-folder WARN). This resolves the §7.1/§9 tension
in favor of the explicit §9 requirement.

---

### Issue 6: `node_modules/`, hidden `.ts` files, and symlinked `.ts` files are discovered as catalog entries (footgun)

**Severity**: Minor (not a strict spec violation — PRD §7.1 does not require
skipping these — but a real footgun that can pollute the catalog)

**PRD Reference**: §7.1 (classify-then-descend descends into every plain dir;
`isExtensionFile` accepts any `*.ts`/`*.js` not named `index.*`). §17 only
forbids placing the STORE itself in an auto-discovery location; it does not
speak to these nested cases.

**Actual Behavior**:
- A top-level `node_modules/` (e.g. created by running `npm install` at the
  store root to share deps) is descended into; every nested package with an
  `index.js`/`index.ts` becomes an extension (e.g. tag
  `node_modules/somepkg`).
- A hidden file like `.secret.ts` becomes an extension tagged `.secret`.
- A symlinked `.ts` file (e.g. `ln -s shared.ts linked.ts`) is discovered as a
  second entry (symlinked FILES are followed; only symlinked DIRECTORIES are
  not, per WalkDir's default).

**Steps to Reproduce**:
```bash
rm -rf /tmp/bug6 && mkdir -p /tmp/bug6/node_modules/somepkg
printf '/** my ext */\nexport default function(){}\n' > /tmp/bug6/myext.ts
printf '/** dep */\nexport default function(){}\n' > /tmp/bug6/node_modules/somepkg/index.js
printf '{"name":"somepkg"}\n' > /tmp/bug6/node_modules/somepkg/package.json
weave_EXTENSIONS_DIR=/tmp/bug6 /tmp/weave --list   # shows node_modules/somepkg as an extension
```

**Suggested Fix** (optional, product judgment): consider skipping well-known
non-extension directories (`node_modules`, `.git`) and/or hidden entries during
the walk, mirroring how pi/npm treat these. Not required by the PRD; flagged for
awareness.

---

## Testing Summary

- Total distinct scenarios exercised: ~90
- Passing: ~83
- Failing / issue-bearing: 6 (1 Major, 5 Minor)
- Areas with good coverage:
  - All PRD §13 acceptance gates (build, --version, --path sibling rule,
    --list, tag resolution, -f, unknown-tag stdout-empty contract, absolute-path
    contract, check, dir/package resolution, env override, unconfigured hint,
    non-interactive init, config-vs-env precedence) — all PASS.
  - Live `pi --no-extensions -e "$(weave example)"` loads the example and
    registers `/weave-example` (exit 0).
  - Tag resolution §7.2 (canonical > basename > name > alias; ambiguity;
    atomicity of multi-tag; canonical-beats-basename precedence).
  - Error semantics & exit codes (unknown=1, ambiguous=1, unknown-flag=2,
    exclusivity=2, no-args=usage→stderr=1, help/version precedence, bundles).
  - Lenient `package.json` parsing (non-array keywords/pi.extensions coerced to
    `[]`; description priority package.json > JSDoc).
  - `check` exit codes (ERROR→1, WARN-only→0, duplicate-relTag, deps-without-
    node_modules, unparseable JSON, no-description).
  - `init` flows (cwd auto-detect, non-interactive, seed-vs-adopt, TTY gating).
  - install.sh (symlink resolves back to repo; sibling-of-binary store works;
    version ldflags applied).
- Areas needing more attention:
  - **Multi-entry `pi.extensions` handling** (Issue 1) — the only functional
    correctness gap; a PRD-supported package shape is mis-discovered.
  - JSDoc single-line degenerate `/**/` (Issue 2).
  - Root-level `index.ts` collapse and symlinked env path (Issues 3–4) — sharp
    edge cases worth defensive handling.
  - §9's "missing pi.extensions entry" ERROR is unreachable in practice
    (Issue 5) — PRD §7.1/§9 tension to resolve.
