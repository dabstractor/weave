# skilldozer `internal/ui/ui.go` → weave `internal/ui/ui.go` Port Diff

This is a **verbatim port**. skilldozer's ui.go (6922 bytes, read in full) is the
authoritative source. weave's version differs in exactly TWO places. Everything
else — ANSI constants, descWrapWidth, PrintList signature/shape, displayWidth,
padRight, wrapWords, the `paint` closure, the row/width computation, the header
rendering, the continuation-line logic — is byte-for-byte identical.

## Source (READ-ONLY port reference)

`/home/dustin/projects/skilldozer/internal/ui/ui.go` — read completely during
P1.M2.T4.S1 research. The corresponding test file
`/home/dustin/projects/skilldozer/internal/ui/ui_test.go` (10207 bytes) is the
authoritative test pattern source (port every test, adapting the `mk` helper and
the sentinel assertions).

## The TWO (and only two) changes

### Change 1 — input type: `[]discover.Skill` → `[]discover.Extension`

skilldozer (ui.go, PrintList signature):
```go
func PrintList(w io.Writer, skills []discover.Skill, useColor bool) {
```
weave:
```go
func PrintList(w io.Writer, exts []discover.Extension, useColor bool) {
```

Consequence: the import path changes from `github.com/dabstractor/skilldozer/internal/discover`
to `github.com/dabstractor/weave/internal/discover`. The loop variable changes
from `s` (Skill) to `e`/`ext` (Extension). The fields read are:
- `s.RelTag` → `e.RelTag`  (identical field name, identical semantics — /-normalized tag)
- `s.Name`   → `e.Name`    (identical — package.json name, else "")
- `s.Description` → `e.Description`  (identical — package.json desc else JSDoc else "")
- `s.HasFM`  → **DELETED** (Extension has NO HasFM field; see Change 2)

CONFIRMED from `/home/dustin/projects/weave/internal/discover/extension.go`
(P1.M2.T1.S1, PRESENT — read in full):
```go
type Extension struct {
    Path           string
    EntryFile      string
    RelTag         string
    Kind           string
    Name           string
    Description    string
    Keywords       []string
    Category       string
    Aliases        []string
    HasPackageJSON bool   // <-- NOT HasFM; NOT used by ui
}
```

### Change 2 — description sentinel: `(missing)` → `(none)`; predicate simplifies

skilldozer (ui.go, the description cell computation):
```go
// HasFM==false OR blank description -> "(missing)". A folded-scalar
// description may carry a trailing newline (discover.go contract);
// TrimSpace normalizes it so it does not inject a blank line into the wrap.
desc := strings.TrimSpace(s.Description)
if !s.HasFM || desc == "" {
    desc = "(missing)"
}
```

weave — the WHOLE change:
```go
// Empty/blank description -> "(none)" (PRD §7.3 item 3: "description = ""
// (rendered as `(none)` in `--list`)"). TrimSpace normalizes whitespace-only
// descriptions AND any trailing newline so it does not inject a blank line
// into the wrap. weave has no HasFM analog — the only signal is "is the
// description empty after trim?".
desc := strings.TrimSpace(e.Description)
if desc == "" {
    desc = "(none)"
}
```

Two edits in one:
1. Remove `!s.HasFM ||` (Extension has no HasFM; PRD §7.3 says empty desc → "(none)",
   regardless of whether the extension has a package.json or only JSDoc).
2. `"(missing)"` → `"(none)"` (PRD §7.3 item 3 verbatim: rendered as `(none)`).

That is the COMPLETE port. No other line of ui.go changes.

## What ports VERBATIM (no edits)

- Package comment — UPDATE: "skill catalog" → "extension catalog", "skilldozer"
  → "weave", "PRD §6.1" stays, "--search (P1.M4.T9)" → "--search (P1.M4.T1)",
  "--list/-l and --no-color (P1.M2.T6)" → "--list/-l and --no-color (P1.M2.T5)".
  (Doc-comment wording; cosmetic but required for accuracy. The SUBTASK label in
  the comment: skilldozer says "P1.M2.T6.S1 deliverable"; weave's task tree has
  this as "P1.M2.T4.S1".)
- Imports: `fmt`, `io`, `strings`, `unicode/utf8`, `discover` (path changes).
- ANSI constants: `ansiReset = "\x1b[0m"`, `ansiBold = "\x1b[1m"`, `ansiCyan = "\x1b[36m"`.
- `const descWrapWidth = 40`.
- PrintList body: empty-slice early return; column-width computation from PLAIN
  content (tagW from `len("TAG")`, nameW from `len("NAME")`, grown by displayWidth);
  the `cells` struct + `rows` slice; the `paint` closure; `sep = "  "`;
  `tagPad`/`namePad`; the bold header `fmt.Fprintf`; the per-row loop with
  `wrapWords(r.desc, descWrapWidth)`; first-line rendering (TAG cyan via paint,
  padRight); continuation lines (blank TAG/NAME cells via tagPad/namePad).
- `displayWidth(s string) int` — `return utf8.RuneCountInString(s)`.
- `padRight(s string, n int) string` — early return if `displayWidth(s) >= n`,
  else `s + strings.Repeat(" ", n-displayWidth(s))`.
- `wrapWords(s string, width int) []string` — `strings.Fields`; empty→`[]string{""}`;
  greedy pack with `displayWidth(cur)+1+displayWidth(word) <= width`; long word on
  own line; final append.

## KNOWN LIMITATIONS inherited verbatim (document, do not "fix")

- `displayWidth` counts runes as 1 column each. Wide CJK runes (2 display cells)
  are undercounted. Deliberate: a full East-Asian width table would need a dep,
  and PRD §2 (hard constraint) forbids non-stdlib deps. The limitation is
  documented in the skilldozer doc comment — port that comment verbatim.
- `descWrapWidth = 40` is fixed; no terminal-width detection (would need
  TIOCGWINSZ ioctl or golang.org/x/term — forbidden dep). Port the rationale
  comment verbatim.

## Test port (skilldozer ui_test.go → weave ui_test.go)

Port every test. Changes:
- `mk(tag, name, desc string, fm bool) discover.Skill` →
  `mk(tag, name, desc string) discover.Extension` (drop the `fm` param — no HasFM).
- `TestPrintListEmptyDescriptionShowsMissing` + `TestPrintListNoFrontmatterShowsMissing`
  → MERGE into `TestPrintListEmptyDescriptionShowsNone`: empty description → "(none)".
  (There is no "no frontmatter" concept for weave — an extension without a
  package.json description that also has no JSDoc simply has `Description == ""`.)
- All "(missing)" assertions → "(none)".
- Import path: `github.com/dabstractor/skilldozer/internal/discover` →
  `github.com/dabstractor/weave/internal/discover`.
- ADD (weave-specific): `TestPrintListTrimsDescription` — a description with
  leading/trailing whitespace still renders trimmed (the TrimSpace call).
- ADD: `TestPrintListJSDocOnlyDescriptionNotNone` — an extension whose Description
  comes from JSDoc (non-empty) does NOT render "(none)" (proves the sentinel only
  fires on empty, not on "no package.json").

## What does NOT port (scope boundary)

- `runeCol`/`colOf` helpers in ui_test.go DO port (they are test-only, used by
  alignment tests) — port verbatim, they are pure string math.
- main.go wiring (`--list`/`--no-color` → PrintList) is P1.M2.T5.S1, NOT this
  subtask. This subtask only creates the `ui` package.
