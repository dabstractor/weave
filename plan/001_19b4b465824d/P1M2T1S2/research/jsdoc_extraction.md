# JSDoc Extraction — Research Notes (P1.M2.T1.S2)

Verifies the `ExtractJSDoc(path string) string` semantics against authoritative sources.

## A. `/**` is THE precise JSDoc marker

Confirmed by multiple authoritative sources:

- **jsdoc.fyi — Writing Comments** (https://jsdoc.fyi/getting-started/comments):
  > "Comments beginning with `/*` (one star) or `/***` (three or more stars) are not parsed
  > by JSDoc. Use them freely to write private notes."

  ⇒ The opener MUST be exactly `/**` (two stars). `/*` (plain block) and `/***`+ are NOT
  JSDoc. This is the load-bearing rule for step 5 ("if the first non-blank line does NOT
  start with exactly `/**` → return `""`").

- **jsdoc.app — Getting Started** (https://jsdoc.app/about-getting-started):
  > "Each comment must start with a `/**` sequence in order to be recognized."

- **Google JavaScript Style Guide** (https://google.github.io/styleguide/jsguide.html):
  uses "JSDoc" for "both human-readable text and machine-readable annotations within
  `/** ... */`."

- **Playcode.io** (https://playcode.io/javascript/comments):
  > "`/* */` is a regular block comment. `/** */` is a JSDoc comment."

**Decision**: step 5 must check the first non-blank line STARTS WITH `/**` (prefix match),
and a `/*` (single star) or `/***` opener must yield `""`. Note: `/***` starts with `/**`
as a PREFIX, so a naive `strings.HasPrefix(line, "/**")` would WRONGLY accept `/***`. The
PRD rule says "exactly `/**`" — interpret this as: the opener is `/**` and the char after
it (if any) must NOT be `*`. For a LEADING line `/** ... */` or `/**` alone this is fine;
the only edge is `/***` which jsdoc.fyi says is NOT a JSDoc. Safest check:
  `strings.HasPrefix(line, "/**") && !strings.HasPrefix(line, "/***")`
Practically, since step 7 takes content BETWEEN `/**` and `*/`, a `/***foo*/` would extract
`*foo` — undesirable. Implement the two-prefix guard. (Single-line `/***/` has nothing
between, so it's harmless; `/*** x */` is the case to reject.)

## B. Per-line star-stripping rule

The rule "optional leading run of whitespace + single `*` + one optional space" is the
standard JSDoc line-cleaning convention:

- **Biome `useSingleJsDocAsterisk`** (https://biomejs.dev/linter/rules/use-single-js-doc-asterisk/):
  > "Enforce JSDoc comment lines to start with a single asterisk, except for the first one.
  > This rule ensures that every line in a JSDoc block ... [starts with one `*`]."

- **TSLint `jsdoc-format`** (https://palantir.github.io/tslint/rules/jsdoc-format/):
  > "each line contains an asterisk and asterisks must be aligned; each asterisk must be
  > followed by either a space or a newline (except for the [last])."

- **VS Code** auto-inserts ` * ` on Enter inside a JSDoc block (Reddit r/vscode, SO 63545917).

**Decision**: For each content line, strip: leading whitespace, then ONE `*` (if present),
then ONE optional space. The PRD rule is exact and matches the convention. A line that is
just ` *` or ` ` or empty → becomes empty after stripping → dropped per step 7.

## C. Edge cases (correct behavior, validated)

| Case | Expected `ExtractJSDoc` | Source / reasoning |
|------|------------------------|--------------------|
| Single-line `/** desc */` | `"desc"` | PRD §7.3.2 last bullet; jsdoc.app |
| Multi-line with ` * ` prefixes | lines joined w/ single space, trimmed | PRD §7.3.2 |
| `/* plain block */` (single star) | `""` | jsdoc.fyi: `/*` not parsed |
| `// line comment` first | `""` | not `/**` |
| Code first | `""` | not `/**` |
| BOM-prefixed file | BOM stripped, then normal | PRD §7.3.2 |
| Leading blank lines / whitespace | skipped, then `/**` recognized | PRD §7.3.2 |
| Unclosed `/**` (no `*/`) | `""` | PRD §7.3.2 "If there is no closing `*/`..." |
| `/*** x */` (3+ stars) | `""` | jsdoc.fyi: `/***` not parsed (guard in step 5) |

## D. Go implementation pitfalls

1. **`bytes.TrimPrefix` is a no-op when the prefix is absent** — safe to always call for BOM
   stripping. (Confirmed: same pattern used in skilldozer's `discover.go` `utf8BOM`.)

2. **CRLF line endings**: `strings.Split(data, "\n")` leaves trailing `\r` on each line.
   The PRD rule does NOT mention CRLF handling for JSDoc (unlike skilldozer's ParseFrontmatter
   which explicitly trims `\r` from `---` fence lines). For JSDoc, the per-line `*`-strip
   rule strips leading whitespace then `*` then optional space — a trailing `\r` is NOT
   leading, so it would survive into the joined description. RECOMMENDATION: after splitting
   on `\n`, also `strings.TrimRight(line, "\r")` (or `strings.TrimSpace` the final result,
   which strips trailing `\r` too). The final `Trim` in step 7 handles trailing whitespace;
   for INTERIOR lines a stray `\r` mid-description is ugly but harmless. Safest: trim `\r`
   from every line right after split (mirrors skilldozer's CRLF handling). This is a minor
   robustness add consistent with skilldozer precedent.

3. **Finding `*/` — whole-string scan vs line scan**: The rule says "scan lines (or scan the
   string) for `*/`". For a LEADING JSDoc (first non-blank line is `/**`), scanning the whole
   string with `strings.Index(s, "*/")` finds the FIRST `*/`, which is the JSDoc closer (code
   hasn't started yet, so no string-literal false positives before it). This is SAFE given the
   "first non-blank line must be `/**`" precondition. Multi-line scan is equivalent and more
   explicit; either works. Prefer the whole-string `strings.Index` approach for simplicity,
   then slice; fall back to line-based only if needed. The PRD explicitly permits either.

4. **Single-line `/** desc */`**: `strings.Index(s, "/**")` then `strings.Index(rest, "*/")`
   between them = `" desc "`, Trim → `"desc"`. No per-line `*` stripping needed (the content
   has no inner `*` lines). The general algorithm (split the between-content into lines,
   strip ` *` prefixes, drop empties, join) handles single-line as a degenerate case
   (one line, no leading `*`, no leading space to strip beyond the space already there) —
   BUT only if the strip rule is "optional" (it is). So one unified algorithm works for both.

5. **`HasPrefix(line, "/**")` on the first non-blank line**: must be the first non-blank
   line AFTER BOM strip and whitespace/blank skip. If that line is `/**` exactly or starts
   with `/**`, proceed; else return `""`.

## E. Reference implementations

- **skilldozer `ParseFrontmatter`** (the function this REPLACES) is the closest in-repo
  analog: same shape (read file → strip BOM → split lines → find fences → extract between →
  return "" on missing). Port its STRUCTURE, not its YAML logic. The BOM constant
  `utf8BOM = []byte{0xEF, 0xBB, 0xBF}` ports VERBATIM (it's already in skilldozer's
  discover.go). NOTE: skilldozer's extension.go (S1) does NOT define this constant; S2's
  jsdoc.go must define it locally (or inline `bytes.TrimPrefix(data, []byte{0xEF,0xBB,0xBF})`).

- **Go stdlib `go/doc`** extracts `//` line comments — different format (line comments, not
  block comments), NOT directly applicable, but the "leading comment, skip blanks" philosophy
  is the same.

- No third-party JSDoc parser should be imported (PRD §2 stdlib-only). The extraction is
  trivial enough for hand-rolled stdlib code (os, bytes, strings).

## Consumer contract (from S1 PRP)

`ExtractJSDoc(path)` returns a `string`. The SOLE caller (T3 Index) passes it as the
`jsdocDesc` argument to `BuildExtension(...)`, which uses it ONLY when `pkg.Description` is
empty (PRD §7.3 priority order). It is also referenced by `check` (M4.T2) for the "no
description" WARN — but check derives that from `Extension.Description == ""` (already
computed by BuildExtension), NOT by calling ExtractJSDoc directly. So the SINGLE integration
point is: `jsdocDesc := ExtractJSDoc(entryFile)` then `BuildExtension(..., jsdocDesc)`.

## Validation approach

Table-driven tests mirroring skilldozer's `TestParseFrontmatter` shape, writing temp files
via a `writeFile(t, dir, name, content)` helper (analog of S1's `writePackageJSON`). Cases:
the 8 from the item contract (multi-line, single-line, no-JSDoc code, `//` comment, `/*`
plain block, BOM-prefixed, leading blanks, unclosed) PLUS `/***` (3-star rejection) and
CRLF line endings for robustness.
