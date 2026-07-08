# Bug 2 (Issue 2) — ExtractJSDoc degenerate `/**/` overlap: research notes

## The bug (reproduced by trace, no binary run needed)
Input file content `/**/\n`. ExtractJSDoc reaches the single-line branch
(`case i == startIdx && i == closeIdx`) with `line == "/**/"`.

Current code:
```go
rest := strings.TrimPrefix(line, "/**")   // TrimPrefix("/**/", "/**") == "/"
if c := strings.Index(rest, "*/"); c >= 0 { // Index("/", "*/") == -1
    rest = rest[:c]
}
line = rest                                // line == "/"  ← WRONG
```

**Root cause (overlap):** in `/**/`, the opener `/**` occupies indices 0–2 and
the closer `*/` occupies indices 2–3 — they SHARE index 2 (the middle `*`).
`TrimPrefix(line, "/**")` consumes indices 0–2, destroying the closer's `*`,
leaving the lone `/`. `Index("/", "*/") == -1`, so no cut happens and the stray
`/` survives as the "content". It then flows through `stripStarPrefix` (no `*`
prefix → unchanged) and `TrimSpace` → `"/"` is returned as the description.

## Why all OTHER empty forms work
- `/** */` → opener `/​**​` (0–2), then ` ` (3), then `*/` (4–5). No overlap.
  `TrimPrefix` → `" */"`, `Index(...,"*/") == 1`, cut → `" "`, stripStarPrefix
  + TrimSpace → `""`. Correct.
- `/**\n*/` → multi-line: opener line `/​**` has no `*/`, closeIdx = next line.
  Hits the `case i == closeIdx` branch (cut at `*/`), content empty. Correct.
- `/***/` → REJECTED at step 5 (`HasPrefix(line, "/***")` → return `""`). Not a
  bug; the `/***/` triple-star case is already correct and must stay untouched.

So the bug is NARROW: only the single-line branch, only when `*/` starts at
index ≤ 3 on the opener line (i.e. the opener and closer overlap). The single
degenerate input is `/**/` (c == 2).

## The fix (item CONTRACT, position-based on the ORIGINAL line)
```go
case i == startIdx && i == closeIdx:
    // Single-line block: find "*/" on the ORIGINAL line BEFORE extracting
    // content, so the opener/closer overlap in degenerate blocks (e.g. "/**/"
    // has "*/" at index 2, within the opener's first 3 chars) is handled.
    c := strings.Index(line, "*/")
    if c <= 3 {
        // "*/" starts at or before the opener "/**" ends (index 3 exclusive)
        // -> opener and closer overlap -> empty content.
        line = ""
    } else {
        line = line[3:c] // content strictly between "/**" and "*/"
    }
```

### Verified traces (python, byte indices match Go string indexing for ASCII)
| input          | c=Index(line,"*/") | branch      | line after | stripStarPrefix | TrimSpace | want |
|----------------|--------------------|-------------|------------|-----------------|-----------|------|
| `/**/`         | 2                  | c<=3 → ""   | `""`       | `""`            | `""`      | `""` |
| `/** desc */`  | 9                  | c>3 → [3:9] | `" desc "` | `" desc "`*     | `"desc"`  | `"desc"` |
| `/**desc*/`    | 7                  | c>3 → [3:7] | `"desc"`   | `"desc"`        | `"desc"`  | `"desc"` |
| `/** */`       | 4                  | c>3 → [3:4] | `" "`      | `""` (TrimLeft then no `*`, then 1 space) | `""` | `""` |

\* `stripStarPrefix(" desc ")`: TrimLeft → `"desc "` (no leading `*` → unchanged
   by the `*`/space steps), TrimSpace → `"desc"`. CORRECT — existing case
   `single-line` (`/** desc */` → `"desc"`) continues to pass.
   `stripStarPrefix(" ")`: TrimLeft `" \t"` → `""`. Correct (dropped).

### Why this is correct and minimal
- The guard `c <= 3` (not `c < 3`) is deliberate: the opener `/**` spans indices
  0,1,2 (length 3); its exclusive end is index 3. A closer starting at index 3
  would mean input like `/**X*/` with a 0-length `X`? No — `/***/` is rejected
  earlier. The smallest valid overlap-free single-line block is `/** */`
  (`*/` at index 4). Index 3 would be `/**/`+`*`? Not reachable cleanly; `c<=3`
  (≤, inclusive of 3) is safe and matches the item contract exactly. Index 3
  cannot occur for a valid block because `/**` already occupies 0–2, so the
  earliest the `*` of a distinct `*/` could start is index 3 only if the opener
  were `/**` followed immediately by `*` — i.e. `/***/`, which step 5 rejects.
  So `c==3` is effectively unreachable; `c<=3` collapses to `c==2` in practice.
- `line[3:c]` is always in-bounds: `c` comes from `Index(line,"*/")`, so `c+2 <=
  len(line)`; and `c > 3` guarantees `3 < c`, so `line[3:c]` is a valid
  non-negative-length slice with `c <= len(line)`.

## Test additions (TDD)
Add to the `cases` slice in `TestExtractJSDoc` (internal/discover/jsdoc_test.go):
```go
// 15. Empty single-line JSDoc, degenerate opener/closer overlap (Issue 2).
{"empty-block-degenerate", "/**/\n", ""},
// 16. Empty single-line JSDoc with a space (regression guard — already worked).
{"empty-block-space", "/** */\n", ""},
```
All 14 existing cases (incl. `single-line` and `single-line-no-space`) MUST
continue to pass unchanged.

## Doc comment [Mode A]
Replace the current 1-line comment on the single-line branch with the longer
comment shown in the fix block above (explains WHY we find `*/` on the original
line first: opener/closer overlap in degenerate blocks). No README/user-doc
change for this subtask (the M4.T1 doc sync owns user-facing prose).

## Scope boundaries (what NOT to touch)
- Do NOT modify step 5 (the `/***/` triple-star guard) — it is already correct.
- Do NOT modify the multi-line branches (`case i == startIdx`, `case i ==
  closeIdx`) — the bug is single-line only.
- Do NOT change `ExtractJSDoc`'s signature or any other function.
- Do NOT touch `stripStarPrefix`, `BuildExtension`, or any other file.
- P1.M2.T3.S1 (parallel) edits `internal/discover/index.go` only — NO overlap.

## Validation
`go test ./internal/discover/... -v` (covers TestExtractJSDoc, the new cases,
and the integration test TestExtractJSDocFeedsBuildExtensionFallback). Then
`go build ./...`, `go vet ./...`, `go test -race ./...` for the whole-repo gate.
No new deps; `strings` already imported.
