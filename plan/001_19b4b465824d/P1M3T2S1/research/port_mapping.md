# Port mapping: skilldozer main.go → weave main.go (P1.M3.T2.S1)

This is a targeted port of **only the tag-resolution / `--all` / modifier plumbing**
from skilldozer's main.go into weave's main.go. It is NOT a wholesale port — weave's
main.go already has its own parseArgs, run() (with --version/--path/--list), version,
isTerminal. We only ADD three things:

1. `extensionPath()` — the weave analog of skilldozer's `skillPath()`.
2. The `--all` branch in `run()`.
3. The tag-resolution branch in `run()`.

## The ONE semantic difference (the load-bearing one)

### skilldozer `skillPath(s discover.Skill, skillsDir string, c config) string`
```go
p := s.Dir          // default: the skill DIRECTORY
if c.file {
    p = s.SourceFile  // --file: s.Dir + "/SKILL.md"  (ALWAYS the SKILL.md)
}
if c.relative { ... filepath.Rel(skillsDir, p) ... }
```
In skilldozer: `Dir` is ALWAYS a directory; `SourceFile` is ALWAYS `Dir+"/SKILL.md"`.
So `--file` is a trivial swap of "dir" → "dir/SKILL.md".

### weave `extensionPath(ext discover.Extension, extensionsDir string, c config) string`
```go
p := ext.Path          // default: resolvable path (file for files, dir for dirs)
if c.file {
    p = ext.EntryFile    // --file: the .ts/.js pi LOADS (file | index.ts | pkg entry)
}
if c.relative { ... filepath.Rel(extensionsDir, p) ... }
```
In weave: `Path` is the **resolvable path** — a FILE for single-file extensions,
a DIRECTORY for dir/package extensions (PRD §7.1). `EntryFile` is the `.ts`/`.js`
file pi loads (the file itself for files, `index.ts`/`index.js` for dirs, the first
existing `pi.extensions` entry for packages). They are DIFFERENT concepts:

| Kind     | Path (default)              | EntryFile (--file)                         |
|----------|-----------------------------|--------------------------------------------|
| file     | `…/gate.ts`                 | `…/gate.ts` (SAME — no-op for --file)      |
| dir      | `…/git-checkpoint`          | `…/git-checkpoint/index.ts`                |
| package  | `…/summarizer`              | `…/summarizer/src/index.ts` (pi.extensions)|

This is the ONLY structural change from skilldozer. The `filepath.Rel` logic,
the modifier-combine logic (`--file --relative`), and the err-guard-then-fallback
pattern are all UNCHANGED. The key correctness property: `extensionPath` does NOT
care which Kind it is — it just reads `ext.Path` or `ext.EntryFile`, both already
populated by `discover.Index` → `classifyFile`/`classifyDir` → `BuildExtension`.
No `switch ext.Kind` is needed (or wanted — the fields already encode the answer).

## Reference: skilldozer's skillPath (verbatim, for diffing)

File: `/home/dustin/projects/skilldozer/main.go`, func `skillPath`.
Read in full during research. The ONLY edits are:
- `s discover.Skill` → `ext discover.Extension`
- `s.Dir` → `ext.Path`
- `s.SourceFile` → `ext.EntryFile`
- param name `skillsDir` → `extensionsDir` (cosmetic; match the codebase noun)
- doc comment: "skill directory" → "resolvable path"; "SKILL.md" → "entry file"

## Reference: skilldozer's run() branches to port

### `--all` branch (port verbatim, swap Discover + skillPath)
```go
if c.all {
    dir, _, err := skillsdir.Find()   // weave: extdir.Find()
    if err != nil { fmt.Fprintln(stderr, err); return 1 }
    skills, err := discover.Index(dir)
    if err != nil { fmt.Fprintln(stderr, err); return 1 }
    for _, s := range skills {
        fmt.Fprintln(stdout, skillPath(s, dir, c))   // → extensionPath(ext, dir, c)
    }
    return 0   // exit 0 even for empty store (PRD §6.1)
}
```
Weave edits: `skillsdir.Find()` → `extdir.Find()`; `skills` → `exts`;
`skillPath(s, dir, c)` → `extensionPath(ext, dir, c)`. NOTHING else changes.

### tag-resolution branch (port verbatim, swap Discover + resolve + skillPath)
```go
if len(c.tags) > 0 {
    dir, _, err := skillsdir.Find()   // weave: extdir.Find()
    if err != nil { fmt.Fprintln(stderr, err); return 1 }
    skills, err := discover.Index(dir)
    if err != nil { fmt.Fprintln(stderr, err); return 1 }
    paths := make([]string, 0, len(c.tags))   // buffered; flushed only if all resolve
    hadErr := false
    for _, tag := range c.tags {
        res, rerr := resolve.Resolve(tag, skills)   // → resolve.Resolve(tag, exts)
        if rerr != nil {
            fmt.Fprintln(stderr, rerr)   // one error line per problem tag (verbatim)
            hadErr = true
            continue
        }
        paths = append(paths, skillPath(res.Skill, dir, c))   // → extensionPath(res.Extension, dir, c)
    }
    if hadErr { return 1 }   // buffered paths NEVER written → stdout empty (§6.4)
    for _, p := range paths { fmt.Fprintln(stdout, p) }   // one path per line, input order
    return 0
}
```
Weave edits: `skillsdir.Find()` → `extdir.Find()`; `skills` → `exts`;
`resolve.Resolve(tag, skills)` stays (the call signature is identical — Resolve
takes `[]discover.Extension` in weave, which is what Index returns);
`res.Skill` → `res.Extension` (the P1.M3.T1.S1 contract: `Result.Extension`).
The `import "github.com/dabstractor/weave/internal/resolve"` is NEW (weave main.go
does not yet import it). `filepath` import is NEW (for extensionPath's filepath.Rel).

## PLACEMENT in weave's run()

Weave's run() currently dispatches: version → path → list → `return 0` (no-op tail).
The new branches go BEFORE the final `return 0`, in this order (so --all is checked
before tags — matches skilldozer's order and the §6.3 precedence the M5 exclusivity
check will build on):
1. `if c.all { ... }`        ← ADD
2. `if len(c.tags) > 0 { ... }` ← ADD
3. `return 0`                ← existing no-op tail (becomes the "no recognized mode"
   path; M5.T1.S1 turns it into usage→stderr→exit 1)

## ATOMICITY — the §6.4 contract (unchanged from skilldozer)

The loop resolves ALL tags, buffering paths. On ANY error:
- print one stderr line per problem tag (`fmt.Fprintln(stderr, rerr)` — the typed
  error's `.Error()` is already the right text: `unknown extension tag "foo"` or
  `ambiguous extension tag "x" matches: a, b`),
- set `hadErr = true`,
- CONTINUE the loop (so every problem tag gets its own line),
- after the loop: `if hadErr { return 1 }` — the buffered `paths` slice is NEVER
  written to stdout, so stdout stays EMPTY.

This is why `paths` is buffered (not printed inline): a later failing tag must not
leak earlier successes to stdout. `pi -e "$(weave good bad)"` must capture an EMPTY
string and see exit 1, not a partial path.

## Import additions to weave main.go

Current imports: `fmt`, `io`, `os`, `strings`, `internal/discover`, `internal/extdir`, `internal/ui`.
ADD:
- `path/filepath` (extensionPath uses filepath.Rel)
- `github.com/dabstractor/weave/internal/resolve`

`internal/ui` is already imported (M2.T5.S1). Do NOT remove it.
