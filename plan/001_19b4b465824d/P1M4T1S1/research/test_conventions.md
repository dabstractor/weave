# Test Conventions for `internal/search`

## Package layout
- `internal/search/search.go` — package `search`, the production code.
- `internal/search/search_test.go` — package `search` (white-box, same package),
  so unexported helpers like `matches` (if tested directly) are accessible.
  skilldozer's tests only exercise the exported `Search`, so white-box access
  is not strictly required, but matching skilldozer's layout (same package)
  is the safe choice.

## How sibling packages test
- `internal/resolve/resolve_test.go` builds `discover.Extension` literals
  **inline** (e.g. `{RelTag: "writing/reddit", Name: "a"}`) — no helper for
  the common case.
- `internal/discover/extension_test.go` uses inline literals too, plus a
  `writePackageJSON` file-system helper for parse tests (NOT needed here —
  search is a pure function over `[]discover.Extension`).
- skilldozer's `search_test.go` uses a tiny `sk(...)` helper to reduce
  boilerplate for the 3-positional-arg + variadic-keywords case, then inline
  literals for the alias/category cases (which need those specific fields).

**Recommendation**: follow skilldozer — a small `mkExt` helper for the common
(tag, name, desc, keywords...) shape, and inline literals where Aliases or
Category must be set. This keeps tests readable and matches the verbatim-port
intent.

## The `sk` → `mkExt` helper
```go
func mkExt(tag, name, desc string, keywords ...string) discover.Extension {
	return discover.Extension{
		RelTag:         tag,
		Name:           name,
		Description:    desc,
		Keywords:       keywords,
		HasPackageJSON: true, // documents intent; matches() does NOT read this
	}
}
```

## What `matches()` does NOT read
`matches()` reads: RelTag, Name, Description, Keywords, Aliases, Category.
It does NOT read: Path, EntryFile, Kind, HasPackageJSON. So tests can leave
those zero. The `mkExt` helper sets `HasPackageJSON: true` purely so a reader
sees the field exists and the "metadata-less" case
(`TestSearchNoPackageJSONStillMatchesByTag`) is contrastable — but production
code is indifferent to it.

## Test names to port (16 functions)
1. `TestSearchMatchByTag`
2. `TestSearchMatchByTagSubstring`
3. `TestSearchMatchByBasenameAsSubstring`
4. `TestSearchMatchByName`
5. `TestSearchMatchByDescription`
6. `TestSearchMatchByKeyword`
7. `TestSearchCaseInsensitive`
8. `TestSearchNoMatchReturnsEmpty`
9. `TestSearchEmptyQueryMatchesAll`
10. `TestSearchPreservesInputOrder`
11. `TestSearchMultipleMatchesAllReturned`
12. `TestSearchNoFrontmatterStillMatchesByTag` →
    **rename** `TestSearchNoPackageJSONStillMatchesByTag` (noun swap)
13. `TestSearchMatchesCategoryAndAliases`
14. `TestSearchKeywordSubstringNotJoinBoundary`
15. `TestSearchNilInput`
16. `TestSearchReturnsDistinctResults`

All 16 port with the `discover.Skill` → `discover.Extension` and
`HasFM:` → `HasPackageJSON:` swaps. The assertions and logic are identical.

## No `t.Parallel()`
These tests mutate no shared state, but the rest of the weave codebase does
NOT use `t.Parallel()` on unit tests of pure functions. Match the prevailing
style: no `t.Parallel()`.

## No env vars, no filesystem
`Search` is a pure function. Tests need NO `t.Setenv`, NO temp dirs, NO
`writeExtTree`. This is the simplest test file in the project.

## Validation commands
```bash
go build ./...                      # compiles the new package
go vet ./...                        # clean
go test ./internal/search/... -v    # the 16 tests, verbose
go test -race ./...                 # whole-repo race sweep
```
