# Port Mapping — skilldozer `internal/search` → weave `internal/search`

## Source of truth
`/home/dustin/projects/skilldozer/internal/search/search.go` (read in full during research).
This is a **near-verbatim port** with ONE change: the element type.

## Type swap (the ONLY semantic change)
| skilldozer | weave |
|---|---|
| `discover.Skill` | `discover.Extension` |
| `s.HasFM` | `s.HasPackageJSON` |

`Skill.HasFM` and `Extension.HasPackageJSON` are semantic equivalents:
both signal "was the catalog metadata file read?" For weave the metadata
file is `package.json`; for skilldozer it was YAML frontmatter in `SKILL.md`.

**IMPORTANT — HasFM/HasPackageJSON is NOT used by `matches()` or `Search()`.**
Neither function reads the bool. It only matters for tests that want to
build "metadata-less" extensions (e.g. the `TestSearchNoPackageJSONStillMatchesByTag`
case). The test helper must set it correctly so the intent is documented,
but the production code does not branch on it.

## Field name mapping (all 6 searchable fields are IDENTICAL)
| skilldozer `Skill` field | weave `Extension` field |
|---|---|
| `RelTag` | `RelTag` ✓ same |
| `Name` | `Name` ✓ same |
| `Description` | `Description` ✓ same |
| `Keywords` | `Keywords` ✓ same |
| `Aliases` | `Aliases` ✓ same |
| `Category` | `Category` ✓ same |

No field renames. The 6-field scope is identical to PRD §6.1 `--search`:
"tag, `package.json` name/description/keywords, the leading-JSDoc
description, `weave.aliases`, and `weave.category`". Note the leading-JSDoc
description is already folded into `Extension.Description` by
`BuildExtension` (the description fallback chain), so search sees a single
`Description` field regardless of source.

## Import path swap
| skilldozer | weave |
|---|---|
| `github.com/dabstractor/skilldozer/internal/discover` | `github.com/dabstractor/weave/internal/discover` |

## What ports VERBATIM (no edits)
- `package search` declaration.
- `import ("strings" + discover)`.
- `func Search(query string, skills []discover.Skill) []discover.Skill` —
  the lowercasing-once, preserve-input-order, empty-matches-all logic.
- `func matches(q string, s discover.Skill) bool` — the 6-field short-circuit
  cascade, keywords-tested-INDIVIDUALLY, aliases-tested-INDIVIDUALLY.
- All doc comments (do noun-swap `skill`→`extension` in prose; see below).

## Doc comment noun swaps (prose only; logic untouched)
- `discover.Skill` → `discover.Extension`
- `skill` → `extension`
- `skilldozer --search` → `weave --search`
- `frontmatter name` → `package.json name`
- `metadata.keyword` / `metadata.alias` / `metadata.category` →
  `package.json keywords` / `weave.aliases` / `weave.category`
- `HasFM==false` → `HasPackageJSON==false`
- §10 references → §6.1 (the weave PRD section that defines `--search`).
  Weave's PRD does NOT have a §10; `--search` is defined in §6.1 and the
  six fields in §7.1/§7.3. Replace the skilldozer §10 citations with §6.1.

## The boundary-safety invariant (CRITICAL — do not change)
Keywords AND aliases are tested **INDIVIDUALLY**, never joined. skilldozer's
`TestSearchKeywordSubstringNotJoinBoundary` proves it: query `"wriocial"`
must NOT match keywords `["writing","social"]`. Joining would create false
positives across boundaries. Port both `for _, kw := range s.Keywords` and
`for _, a := range s.Aliases` loops EXACTLY as-is.

## Empty-query semantics (do not add a special case)
`strings.Contains(hay, "") == true` always, so an empty query matches every
extension. The PRD carves out NO special case. Do NOT add `if query == ""`
guard. skilldozer's `TestSearchEmptyQueryMatchesAll` pins this.

## Preserve-input-order (do not re-sort)
`discover.Index` already sorts `[]Extension` by `RelTag`. `Search` iterates
in order and appends matches → output stays sorted. `ui.PrintList` does not
re-sort. Do NOT sort inside `Search`.

## Test helper port
skilldozer's `sk(tag, name, desc string, keywords ...string) discover.Skill`
ports as `mkExt(tag, name, desc string, keywords ...string) discover.Extension`
with `HasFM: true` → `HasPackageJSON: true`. Aliases/Category tests use
inline `discover.Extension{...}` literals (copy those literally with the
`HasFM:` → `HasPackageJSON:` swap).

## Test file: port EVERY test function
skilldozer `internal/search/search_test.go` has 16 test functions. Port ALL
of them with the type/noun/field swap. The skilldozer `Issue 4 fix` comment
on `TestSearchMatchesCategoryAndAliases` documents the inversion of the old
wrong-behavior test — keep the correct behavior and the explanatory comment
(updated to cite PRD §6.1 / §7.3 instead of §10).
