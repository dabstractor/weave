# P1.M4.T3.S1 — Research / Contract Notes

Single deliverable: two dispatch branches in `main.go run()` (`--search`, `check`).
The "gears" all exist: `search.Search`, `check.Check`, `ui.PrintList`,
`discover.Index`, `extdir.Find`. This task only WIRES them. Notes below pin the
exact contracts the implementation must satisfy.

## 1. Consumed package contracts (verified by reading source)

### extdir.Find
`func Find() (dir string, src Source, err error)` — `internal/extdir/extdir.go:291`.
On miss: `err == ErrNotFound`, `ErrNotFound.Error()` ==
`"weave is not configured; run \`weave init\`"` (extdir.go:271). The dispatch
discards `src` (only `--path` prints it).

### discover.Index
`func Index(extensionsDir string) ([]Extension, error)` — `internal/discover/index.go`.
Returns `[]Extension` sorted by RelTag. Empty store → `(nil, nil)` (test with len).

### search.Search
`func Search(query string, exts []discover.Extension) []discover.Extension` —
`internal/search/search.go:134`. Pure. Empty query matches all. Order preserved.

### ui.PrintList
`func PrintList(w io.Writer, exts []discover.Extension, useColor bool)` —
`internal/ui/ui.go`. The SAME renderer `--list` uses (PRD §6.1 "same table format").

### check.Check  ← from the P1.M4.T2.S1 PRP (CONTRACT — treat as built)
- `func Check(dir string, exts []discover.Extension) Report`  ← **dir FIRST**
- `type Report struct { ByExt []ExtensionReport; Errors int; Warnings int }`
- `type ExtensionReport struct { Extension discover.Extension; Findings []Finding }`
- `type Finding struct { Level Severity; Message string }`
- `type Severity int` + `func (s Severity) String() string` → `"OK"|"WARN"|"ERROR"` (VALUE receiver → `%s` works)
- `func (r Report) HasErrors() bool`  → `r.Errors > 0`

**FIELD NAMES (noun swaps from skilldozer):** `ByExt` (NOT BySkill),
`Extension` (NOT Skill). The rendering loop uses `er.Extension.Name`,
`er.Extension.RelTag`, `er.Findings`.

⚠ **DISCREPANCY:** the item description writes `check.Check(exts)`, but the
P1.M4.T2.S1 PRP (built in parallel) defines `check.Check(dir, exts)` because the
§9 empty-category-folder rule needs a filesystem walk. The `dir` is ALREADY in
scope from `extdir.Find()` in the same branch. **Call `check.Check(dir, exts)`.**

## 2. Dispatch insertion point + ordering (weave main.go run())

Current order (read from main.go): `version → path → list → all → tags → no-op`.
Insert `--search` and `check` BETWEEN the `--list` block and the `--all` block,
matching skilldozer's proven order `list → search → check → all → tags`:

`version → path → list → search → check → all → tags → no-op`

Renumber the trailing `// N)` comments (cosmetic; comment-only). Each branch is a
standalone `if … { … return }`, so the first-true-in-order wins when multiple
modes are set (exclusivity exit-2 lands in M5.T1.S1 — this task's branches coexist
via ordering exactly as `--list`/`--all`/`tags` already do).

## 3. Exact wording deltas vs skilldozer (load-bearing)

| skilldozer                       | weave (this task)                              |
| -------------------------------- | ---------------------------------------------- |
| `"no skills matched "+q`         | `"no extensions matched "+c.searchQ` (stderr)  |
| `"%d skills, %d errors, %d …"`   | `"%d extensions, %d errors, %d warnings"`      |
| `name == "" → "(none)"`          | `name == "" → relTagBase(RelTag)` (basename)   |
| `check.Check(skills)`            | `check.Check(dir, exts)`                       |
| `rep.BySkill` / `sr.Skill`       | `rep.ByExt` / `er.Extension`                   |

The "(name or basename)" rule is in the ITEM DESCRIPTION (not skilldozer's
"(none)"). `relTagBase` = substring after the last `/`, else the whole RelTag.
`strings` is already imported in main.go.

## 4. Summary count N

`N = len(exts)` (discovered extensions), NOT `len(rep.ByExt)`. The check PRP
appends empty-category-folder findings as SYNTHETIC ExtensionReport entries in
`ByExt`; those are folders, not extensions, and must not inflate the count.
Matches skilldozer's `len(skills)`. (Note: `%d extensions` prints "1 extensions"
— grammatically awkward but intentional, identical to skilldozer's `%d skills`.)

## 5. Test-fixture reliability (verified against discover.classifyDir source)

| Desired check finding | Fixture (under weave_EXTENSIONS_DIR root) | discover indexes it? | check flags? |
| --- | --- | --- | --- |
| **clean** (0 findings) | `example.ts` with `/** A demo */` JSDoc | yes (file) | none → OK line |
| **ERROR** unparseable pkg | `broken/package.json` = `{ not json` + `broken/index.ts` with JSDoc | **yes** — case (a) fails (broken JSON → Pi.Extensions nil), case (b) index.ts wins → dir kind | re-parse → ERROR "package.json is not valid JSON" |
| **WARN** no description | `nodesc.ts` = `export default function(){}` (NO JSDoc, no pkg) | yes (file) | WARN "no description" |
| **WARN** empty folder | empty dir `empty/` (no entries) | n/a (not an entry) | check walks top-level subdirs, Index(empty)=0 → WARN |

The ERROR fixture is reliable on Linux (NOT filesystem-dependent, unlike the
duplicate-relTag case-insensitive rule). The no-description WARN fixture gives
exit 0 (warnings never fail) — use it to test WARN rendering WITHOUT exit 1.

## 6. Existing test helpers to reuse (main_test.go)

- `writeExtTree(t, map[tag]desc)` — single-file `.ts` exts with JSDoc. Returns root.
- `sampleStore(t)` — `example` + `writing/reddit-poster`.
- `writeDirExt(t, root, tag, desc)` — `<tag>/index.ts` (dir kind).
- `writePkgExt(t, root, tag, desc)` — `<tag>/package.json` + `<tag>/src/index.ts` (package kind).
- `withTerminal(t, bool)` — override `isTerminal`.
- `unsetExtEnv(t)` + `t.Chdir(t.TempDir())` — force extdir.Find → ErrNotFound.
- `t.Setenv("weave_EXTENSIONS_DIR", dir)` — rule 1 wins deterministically.

All new tests follow the same `var out, errOut bytes.Buffer; code := run(args, &out, &errOut)` shape.

## 7. Imports

Add to main.go's import block (alphabetical, internal group):
- `"github.com/dabstractor/weave/internal/check"`  (goes FIRST — 'c' < 'd')
- `"github.com/dabstractor/weave/internal/search"` (goes between resolve and ui)

No new stdlib imports (`fmt`, `io`, `os`, `path/filepath`, `strings` all present).
