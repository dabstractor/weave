# Architecture Mapping — skilldozer → weave

This document maps each skilldozer package to its weave equivalent, identifying what
ports verbatim, what adapts, and what is new.

## 1. `internal/config` — Settings file

### skilldozer
- Uses `gopkg.in/yaml.v3` for `File{Store string}` Marshal/Unmarshal.
- `Path()` resolves `$SKILLDOZER_CONFIG` or `$XDG_CONFIG_HOME/skilldozer/config.yaml`.
- `DefaultStore()` resolves `$XDG_DATA_HOME/skilldozer/skills` or `~/.local/share/skilldozer/skills`.
- `Load(path)` returns raw `os.ReadFile` error (so callers can `errors.Is(err, fs.ErrNotExist)`).

### weave (ADAPT)
- **No yaml.v3.** Hand-roll a trivial parser for the one key `store:`:
  - Read file, split lines, find a line matching `^store:\s*(.+)$` (after trimming).
  - `Save` writes exactly `store: <value>\n` (MkdirAll parent, WriteFile 0o644).
  - Unknown keys ignored (room to grow).
  - Broken syntax is NOT a hard error for Load (treated as "not configured" → fall through).
    Actually: a missing/unreadable config falls through. A present-but-unparseable config
    is ALSO treated as fall-through (unlike skilldozer where broken YAML is a hard error) —
    because our hand-rolled parser is so simple (one regex per line) that "unparseable"
    really just means "no `store:` line found", which is the same as "not configured."
- `Path()`: `$weave_CONFIG` or `$XDG_CONFIG_HOME/weave/config.yaml`. **Note env var is
  lowercase** `weave_CONFIG` per PRD §8.1 (Go `os.Getenv("weave_CONFIG")`).
- `DefaultStore()`: `$XDG_DATA_HOME/weave/extensions` or `~/.local/share/weave/extensions`.
- Same `Load` error convention (return raw read error for `errors.Is` checks).

### Key file: `internal/config/config.go`
```go
type File struct { Store string }
func Load(path string) (File, error)   // hand-rolled line scan
func Save(path string, f File) error   // write "store: <value>\n"
func Path() (string, error)            // $weave_CONFIG or XDG default
func DefaultStore() (string, error)    // $XDG_DATA_HOME/weave/extensions
```

## 2. `internal/extdir` — Locate the extensions dir (was `skillsdir`)

### skilldozer → weave (PORT NEARLY VERBATIM)
- **Same 5-rule priority order** (PRD §8.3):
  1. `weave_EXTENSIONS_DIR` env var (was `SKILLDOZER_SKILLS_DIR`)
  2. Config file `store` key
  3. Sibling of running binary (symlink-aware: `os.Executable()` + `filepath.EvalSymlinks()`)
  4. Walk up from cwd
  5. None → `ErrNotFound`
- **Port `resolveSiblingFromExe` verbatim** — only the sibling dir name changes: `extensions`
  instead of `skills`.
- **Port `findWalkUpAncestor` verbatim** — only the predicate changes.
- **Predicate change:** skilldozer uses `HasSkillMD(dir)` (walks for `SKILL.md`).
  weave uses `HasExtensionEntry(dir)` — walks for ANY extension entry at any depth:
  a `*.ts`/`*.js` file (not `index.*`), OR a dir containing `index.ts`/`index.js`,
  OR a dir with `package.json` containing `pi.extensions`. This is the "is this dir a
  store" predicate used by both walk-up resolution AND `weave init` cwd auto-detect.
- **`Source` type and `String()` labels** port verbatim with env-var name updated.

### Key file: `internal/extdir/extdir.go`
```go
type Source int  // SourceEnv, SourceConfig, SourceSibling, SourceWalkUp
func Find() (dir string, src Source, err error)
func HasExtensionEntry(dir string) bool  // exported for init's cwd auto-detect
```

## 3. `internal/discover` — Walk, classify, extract metadata

### skilldozer → weave (REWRITE)
This is where the most new logic lives. skilldozer's discovery is simple: walk for
`SKILL.md` files, parse YAML frontmatter. weave must classify entries as file/dir/package
and apply the classify-then-descend recursion rule.

### 3a. `extension.go` — Extension struct + metadata extraction (was `skill.go`)

**Extension struct** (replaces `Skill`):
```go
type Extension struct {
    Path           string  // resolvable path (file or dir) — DEFAULT output
    EntryFile      string  // the .ts/.js file pi loads — --file output
    RelTag         string  // canonical tag (path relative to extensions/, .ts/.js stripped for files)
    Kind           string  // "file" | "dir" | "package"
    Name           string  // package.json name, else ""
    Description    string  // package.json description, else leading JSDoc, else ""
    Keywords       []string // package.json keywords
    Category       string  // package.json weave.category
    Aliases        []string // package.json weave.aliases
    HasPackageJSON bool    // whether a package.json was read
}
```

**Metadata extraction** (`BuildExtension` or equivalent):
- Parse `package.json` via `encoding/json` into a struct with standard fields + namespaced
  `Weave` sub-struct (`Aliases`, `Category`) + `Pi` sub-struct (`Extensions`).
- Be lenient: wrong-typed fields coerced or ignored (non-array `keywords` → `[]`).
- `toStringSlice` helper from skilldozer ports directly (normalizes `[]any` → `[]string`).
  Actually with `encoding/json`, arrays unmarshal into `[]any` (via `json.Unmarshal` into
  `interface{}`) or `[]string` (via typed struct field). Use typed struct fields where
  possible; fall back to `[]any` normalization for nested `weave.aliases`.

### 3b. JSDoc extractor (NEW — replaces `ParseFrontmatter`)

**`ExtractJSDoc(path string) string`** — precise, bounded, stdlib-only:
1. Read `entryFile`, strip leading UTF-8 BOM (`[]byte{0xEF, 0xBB, 0xBF}`).
2. Skip leading whitespace and blank lines.
3. If next chars are exactly `/**` (JSDoc block open):
   - Find closing `*/`.
   - Take lines between.
   - For each line: strip optional leading whitespace + single `*` + one optional space.
   - Drop empty lines. Join survivors with single space. Trim.
4. If no closing `*/`, or first non-whitespace token is NOT `/**` → return `""`.
5. Single-line `/** ... */` works: content between `/**` and `*/`, trimmed.

### 3c. `discover.go` — Classify-then-descend walk logic (NEW)

**`classifyEntry(dir, name, isDir) (entry *Extension, shouldDescend bool)`**:
- If it's a **file** with `.ts`/`.js` extension and base name is NOT `index.ts`/`index.js`:
  → single-file extension. `entry.Path = file`, `entry.EntryFile = file`, `entry.Kind = "file"`.
  `relTag` = relative path with `.ts`/`.js` stripped.
- If it's a **directory**:
  - Check for `package.json` with `pi.extensions` array naming ≥1 existing entry:
    → package extension. `entry.Path = dir`, `entry.EntryFile = first existing pi.extensions entry`,
    `entry.Kind = "package"`.
  - Else check for `index.ts` or `index.js`:
    → dir extension. `entry.Path = dir`, `entry.EntryFile = index file`, `entry.Kind = "dir"`.
  - Else: plain directory → `shouldDescend = true` (recurse into it).
- When a dir is recognized as an extension → `shouldDescend = false` (DO NOT recurse).

**Recursion rule (load-bearing — prevents double-counting):**
When a directory is recognized as a dir/package extension, do NOT descend into it.
Its internal `.ts` files are implementation details of one unit. Only descend into
plain directories (category folders that may contain nested entries).

### 3d. `index.go` — Index() walk + sort (ADAPT)

- Use `filepath.WalkDir` but with **custom descend logic** (WalkDir's default descends
  into every dir; weave must skip recognized extension dirs). Implementation:
  - Walk with a recursive helper rather than WalkDir, OR use WalkDir with
    `filepath.SkipDir` returned when an entry dir is recognized.
  - Actually: WalkDir visits every entry. For each dir entry, classify it FIRST; if it's
    an extension, emit the entry and return `filepath.SkipDir` to prevent descent. If it's
    a plain dir, return nil (WalkDir descends naturally). For file entries, classify them.
- Stat-guard the root before walking (skilldozer pattern).
- Sort by `RelTag` (same as skilldozer).
- Empty store → nil slice, nil error (same as skilldozer).

## 4. `internal/resolve` — Tag → extension (PORT NEARLY VERBATIM)

### skilldozer → weave
The 4-step precedence is identical (§7.2):
1. Exact canonical tag (`RelTag`)
2. Basename (final `/`-component of `RelTag`)
3. `package.json` `name` (was frontmatter `name`)
4. Declared alias (`weave.aliases`, was `metadata.aliases`)

Port `Resolve()`, `UnknownError`, `AmbiguousError`, `collectMatches`, `basename`,
`sortedRelTags` verbatim. Only the type changes: `discover.Skill` → `discover.Extension`.

## 5. `internal/ui` — Table formatting (PORT NEARLY VERBATIM)

Port `PrintList`, `padRight`, `displayWidth`, `wrapWords`, ANSI constants verbatim.
- Column descriptions change: `HasFM` check for "(missing)" → `Description == ""` check
  for "(none)". PRD §7.3: empty description renders as `(none)` in `--list`.
- `descWrapWidth` stays at 40.

## 6. `internal/search` — Substring search (PORT NEARLY VERBATIM)

Port `Search()` and `matches()` verbatim. Same 6 searchable fields (tag, name, description,
keywords, aliases, category). Type changes: `discover.Skill` → `discover.Extension`.

## 7. `internal/check` — Validation (ADAPT)

### weave check rules (§9)
- **ERROR**: duplicate canonical `relTag` across entries.
- **ERROR**: a dir/package entry's `entryFile` does not exist on disk.
- **ERROR**: `package.json` present but unparseable as JSON.
- **WARN**: package extension with non-empty `dependencies` but no `node_modules/`.
- **WARN**: entry has no description (neither package.json description nor JSDoc).
- **WARN**: top-level non-entry directory contains zero discoverable entries.

Port the `Report`/`SkillReport`/`Finding`/`Severity` structure from skilldozer.
The validation logic is different (no frontmatter/name rules; instead relTag dups,
entryFile existence, package.json validity, deps-installed, description presence).

## 8. `main.go` — Entrypoint (ADAPT)

Port the overall structure: `main()`, `run()`, `parseArgs()`, `config` struct,
`exclusivityError()`, `skillPath()` (→ `extensionPath()`), `isTerminal()`, `stdinIsTerminal()`,
`readPrompt()`, `chooseStore()`, `resolveStore()`, `setupStore()`, `runInit()`.

Key changes:
- `config` struct: same fields, different binary name in usage text.
- `extensionPath()`: applies `--file` (→ `EntryFile`) and `--relative` modifiers.
  Default = `Path` (the resolvable path). `--file` = `EntryFile`.
- `exampleSkillTemplate` → `exampleExtensionTemplate`: the §11 example.ts body as a
  string constant.
- `setupStore`: seeds `example.ts` (not `example/SKILL.md`) into an empty store.
- Env var names: `weave_EXTENSIONS_DIR`, `weave_CONFIG`, `weave_INSTALL_BIN`.
- Usage text: updated for `weave` and `pi -e`.
