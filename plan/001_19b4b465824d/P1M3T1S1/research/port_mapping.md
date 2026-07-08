# Port mapping — skilldozer/internal/resolve → weave/internal/resolve

## Source (verbatim)
`/home/dustin/projects/skilldozer/internal/resolve/resolve.go` — read in full.
Companion test: `/home/dustin/projects/skilldozer/internal/resolve/resolve_test.go` — read in full.

## Type change (the ONLY semantic change)
`discover.Skill` → `discover.Extension` (defined in `internal/discover/extension.go`).

Field mapping for resolution logic (all that `Resolve` touches):
| skilldozer `Skill` field | weave `Extension` field | notes |
|---|---|---|
| `RelTag`  | `RelTag`  | same — slash-normalized, `.ts`/`.js` stripped for files |
| `Name`    | `Name`    | same — `""` if absent (PRD §7.3) |
| `Aliases` | `Aliases` | same — `[]string` via toStringSlice |
| `Dir`/`SourceFile` | `Path`/`EntryFile` | NOT touched by Resolve; Result.Extension just carries them through |

The `Result` struct field renames `Skill` → `Extension`. All else identical.

## Import path change
`github.com/dabstractor/skilldozer/internal/discover`
→ `github.com/dabstractor/weave/internal/discover`

(module = `github.com/dabstractor/weave`, confirmed in `go.mod`).

## Noun swap in error messages
Per the item contract: error strings say "extension", not "skill":
- `UnknownError.Error()`:  `unknown extension tag %q`  (was `unknown skill tag %q`)
- `AmbiguousError.Error()`: `ambiguous extension tag %q matches: <candidates>` (was `ambiguous skill tag %q matches: ...`)

Doc comments: `skill`/`skills` → `extension`/`extensions` throughout. `frontmatter name` → `package.json name` (PRD §7.3 — weave has no frontmatter; Name comes from package.json).

## Verbatim-port helpers (no logic change)
- `collectMatches(skills []discover.Skill, pred func(discover.Skill) bool) []discover.Skill`
  → `collectMatches(exts []discover.Extension, pred func(discover.Extension) bool) []discover.Extension`
- `basename(relTag string) string` — UNCHANGED (pure string op on slash-normalized tags).
- `sortedRelTags(skills []discover.Skill) []string`
  → `sortedRelTags(exts []discover.Extension) []string`

## 4-step precedence (PRD §7.2) — identical to skilldozer
1. Exact RelTag (`Canonical`) — unique, no ambiguity.
2. Basename of RelTag (`Basename`) — >1 ⇒ AmbiguousError.
3. Name (skip `Name==""`) — >1 ⇒ AmbiguousError.
4. Aliases — >1 ⇒ AmbiguousError.
5. Nothing ⇒ UnknownError.

SHORT-CIRCUIT: an ambiguity at any step returns immediately; later steps are NOT consulted.

## Signature (consumed by main.go tag loop — P1.M3.T2.S1)
```go
func Resolve(tag string, exts []discover.Extension) (Result, error)
```
Pure: no fs, no globals, no I/O. `main.go` supplies `exts` from `discover.Index(dir)`.

## Test fixture mapping (skilldozer test → weave test)
skilldozer `exampleSkills`:
```go
{Dir:"/repo/skills/foo", RelTag:"foo", Name:"foo-helper", SourceFile:"/repo/skills/foo/SKILL.md"}
{Dir:"/repo/skills/writing/reddit", RelTag:"writing/reddit", Name:"reddit-poster", Aliases:[]string{"social"}, SourceFile:".../SKILL.md"}
```
weave `exampleExts` (uses `Path`/`EntryFile`, keeps RelTag/Name/Aliases identical for the same coverage):
```go
{Path:"/repo/extensions/gate.ts", EntryFile:"/repo/extensions/gate.ts", RelTag:"gate", Kind:"file"}
{Path:"/repo/extensions/writing/reddit-poster.ts", EntryFile:".../reddit-poster.ts", RelTag:"writing/reddit-poster", Name:"reddit-poster", Aliases:[]string{"social"}}
```
This mirrors the PRD §7.1 worked example (gate.ts, writing/reddit-poster.ts, summarizer dir) and exercises all 4 steps + ambiguity + unknown.

## Validation approach
Existing codebase uses stdlib `testing` only (no testify). Pattern from `internal/discover/*_test.go`: table-driven, `t.Run` subtests, plain `==`/`reflect.DeepEqual` asserts, helper funcs. The skilldozer resolve_test.go ports nearly verbatim with the noun swap; we adopt it and extend with a `summarizer` package.json-name fixture to hit the PRD §7.2 example `@my-org/summarizer` → summarizer dir explicitly.

`go test ./internal/resolve/... -v` and `go test -race ./...` are the gates (consistent with sibling packages).
