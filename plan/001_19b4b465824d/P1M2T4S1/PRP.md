# PRP — P1.M2.T4.S1: PrintList + padRight + wrapWords + displayWidth

## Goal

**Feature Goal**: Create the `internal/ui` package — a PURE table formatter that renders a `[]discover.Extension` as the `TAG / NAME / DESCRIPTION` catalog table consumed by `main.go`'s `--list` and `--search` modes (PRD §6.1). It is a near-verbatim port of skilldozer's `internal/ui/ui.go` (read in full during research): the ONLY deltas are (a) the input type changes from `[]discover.Skill` to `[]discover.Extension`, and (b) the empty-description sentinel changes from `(missing)` (driven by `!HasFM || desc==""`) to `(none)` (driven by `TrimSpace(desc)==""`), per PRD §7.3 item 3. Everything else — ANSI constants, `descWrapWidth=40`, `PrintList` body, `displayWidth`, `padRight`, `wrapWords`, the `paint` closure, the row/width math, the header, the continuation-line logic — ports byte-for-byte.

**Deliverable**: TWO new files. (1) `internal/ui/ui.go` (`package ui`) containing the package doc comment, four ANSI/width constants, and four functions: `PrintList(w io.Writer, exts []discover.Extension, useColor bool)`, `displayWidth(s string) int`, `padRight(s string, n int) string`, `wrapWords(s string, width int) []string`. (2) `internal/ui/ui_test.go` (`package ui`) porting skilldozer's `ui_test.go` and adding two weave-specific tests. No other files change. go.mod gains NO `require` (stdlib only: `fmt`, `io`, `strings`, `unicode/utf8`, plus the cross-package `github.com/dabstractor/weave/internal/discover`).

**Success Definition**:
- `go build ./...` exits 0 (new `internal/ui` package compiles; imports `internal/discover` which is PRESENT from P1.M2.T1.S1 + P1.M2.T2.S1 + P1.M2.T3.S1).
- `go vet ./...` exits 0 (clean).
- `go test ./internal/ui/... -v` passes ALL tests.
- `go test -race ./...` passes.
- The `--list` table format matches PRD §6.1: header row `TAG`, `NAME`, `DESCRIPTION` (bold when colored); data rows with TAG (cyan when colored), NAME (or `(none)`), DESCRIPTION (or `(none)`, word-wrapped to 40 columns, continuation lines leave TAG/NAME blank).
- Empty description renders as `(none)` (PRD §7.3 item 3) — NOT `(missing)` (skilldozer's sentinel) and NOT blank.
- Empty NAME renders as `(none)` (unchanged from skilldozer).
- An empty `[]Extension` prints nothing (the early return; `main.go` owns the "exit 1 if no extensions found" decision in P1.M2.T5.S1).
- Color is opt-in via `useColor`; when false, output contains ZERO `\x1b` bytes (critical for `$(...)`, pipes, log files, and deterministic tests — PRD §6.4).
- Multi-byte tags (e.g. `café`: 4 runes / 5 bytes) align correctly because column math uses `displayWidth` (rune count), NOT `len` (byte count). Pinned by `TestPrintListColumnsAlignedForMultibyte`.
- `ui.go` imports ONLY stdlib + `internal/discover`. NO third-party deps. go.mod still has no `require` block; no `go.sum`.

## User Persona (if applicable)

**Target User**: weave CLI users (transitively, via `--list`/`--search`) and, directly, `main.go` (P1.M2.T5.S1 wires `--list`; P1.M4.T3.S1 wires `--search`). This subtask is an internal package with no CLI surface yet — it exposes `ui.PrintList` for the next milestone to call.

**Use Case**: `main.go`'s `run()` resolves the extensions dir (`extdir.Find`), builds the catalog (`discover.Index` → `[]Extension`), and — for `--list`/`--search` — calls `ui.PrintList(stdout, exts, useColor)`. `useColor` is `isTerminal(os.Stdout) && !noColor`. `PrintList` is the last step: it renders the already-sorted slice as a human-readable table. For `--search` (P1.M4.T1.S1), the filtered (still-sorted) slice is passed to the same `PrintList` (PRD §6.1: "same table format as `--list`").

**User Journey**: N/A (internal). End-user journey (`weave --list`) is assembled in P1.M2.T5.S1; this subtask is the rendering gear.

**Pain Points Addressed**:
- Catalog readability: raw `[]Extension` is unusable for humans; the aligned, wrapped table makes TAG/NAME/DESCRIPTION scannable.
- Pipe/determinism safety: color MUST be opt-in so `weave --list | grep` and `$(weave --list)` see clean text (PRD §6.4 error-semantics note: "critical for `$(...)` use" extends to color contamination).
- Unicode alignment: a naive `len()`-based column math would misalign any tag/name containing a multi-byte rune; `displayWidth` (rune count) fixes the common case (accents, em-dashes, smart quotes) without a wide-width table dep.

## Why

- **PRD §6.1 contract surface**: `--list` is defined as "Table: `TAG`, `NAME`, `DESCRIPTION` (wrapped)" and `--search` as "Same table format as `--list`, filtered." This subtask IS that table format. Without it, `main.go` cannot ship `--list` (P1.M2.T5.S1 is blocked on `ui.PrintList`).
- **Near-verbatim port minimizes risk**: skilldozer's `internal/ui/ui.go` is a PROVEN, tested implementation (its `ui_test.go` covers empty input, color on/off, the `(none)`/`(missing)` sentinels, folded-newline trimming, long-description wrapping, input-order preservation, cross-row column alignment, and the multi-byte alignment regression). Porting it verbatim (with the two documented deltas) inherits all that coverage. Writing from scratch would re-litigate the padding/wrap/color-ordering decisions skilldozer already settled.
- **The `(none)` sentinel is the load-bearing weave change**: PRD §7.3 item 3 explicitly states `description = ""` is "rendered as `(none)` in `--list`." skilldozer's `(missing)` (which fired on `!HasFM`) has NO weave analog — `Extension` has no `HasFM` field, and PRD §7.3 says empty desc → `(none)` regardless of source (package.json OR JSDoc). The predicate simplifies to `TrimSpace(desc) == ""`.
- **Color isolation for `$(...)` safety**: PRD §6.4's "critical for `$(...)` use" applies to PATH output AND to list output that may be grepped. Keeping color purely opt-in (caller decides via `useColor`) and applying it ONLY to the header (bold) and TAG column (cyan) — never NAME/DESCRIPTION — means the searchable text columns are always byte-clean.
- **Pre-compute widths from PLAIN content**: skilldozer computes column widths from the UNCOLORED strings, then applies `padRight` to the plain string BEFORE `paint`. This keeps the invisible ANSI bytes out of the width math. Port this ordering verbatim — it is the non-obvious trick that makes colored output align identically to uncolored output.

## What

A NEW `internal/ui/ui.go` (`package ui`) — a port of `/home/dustin/projects/skilldozer/internal/ui/ui.go` (READ-ONLY reference, read in full during research). The package contains:

### Package doc comment (port + adapt wording)
```go
// Package ui renders the human-readable extension catalog table for weave's
// --list and --search modes (PRD §6.1). It is a PURE formatter: it takes a
// []discover.Extension (already discovered and sorted by the caller —
// discover.Index sorts by RelTag) and writes a TAG/NAME/DESCRIPTION table.
// Color is opt-in via a useColor parameter, so the caller (main) owns the TTY /
// --no-color decision and unit tests are fully deterministic (no real terminal
// required).
//
// This is the P1.M2.T4.S1 deliverable. main.go wires `--list`/`-l` (P1.M2.T5.S1)
// and `--no-color` (PRD §6.2) to call PrintList; --search (P1.M4.T1.S1) reuses
// PrintList with a filtered slice (PRD §6.1: "same table format as --list").
package ui
```

### Constants (port VERBATIM)
```go
const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
)

const descWrapWidth = 40
```
Port the rationale comment on `descWrapWidth` verbatim (the "no TIOCGWINSZ / golang.org/x/term — PRD hard constraint forbids the dep" reasoning).

### `func PrintList(w io.Writer, exts []discover.Extension, useColor bool)`

Port skilldozer's `PrintList` body VERBATIM with exactly these edits (see research/skilldozer_ui_port_diff.md for the line-by-line):

1. **Signature**: `skills []discover.Skill` → `exts []discover.Extension`. Loop var `s` → `ext` (or `e`).
2. **Loop body** — the `desc` computation (THE load-bearing change):
```go
// Empty/blank description -> "(none)" (PRD §7.3 item 3: "description = "" ...
// rendered as `(none)` in `--list`"). TrimSpace normalizes whitespace-only
// descriptions AND any trailing newline so it does not inject a blank line
// into the wrap. weave has no HasFM analog — the only signal is "is the
// description empty after trim?".
desc := strings.TrimSpace(ext.Description)
if desc == "" {
	desc = "(none)"
}
```
   Compare to skilldozer's `if !s.HasFM || desc == "" { desc = "(missing)" }` — weave DROPS the `!s.HasFM` clause (no such field) and changes the sentinel string.
3. The NAME `"(none)"` rule ports unchanged: `if name == "" { name = "(none)" }`.
4. The column-width growth (`if displayWidth(ext.RelTag) > tagW`) ports unchanged.
5. The `paint` closure, `sep = "  "`, `tagPad`/`namePad`, the bold header `fmt.Fprintf`, the per-row loop, `wrapWords(r.desc, descWrapWidth)`, the first-line render (TAG cyan via `paint(ansiCyan, padRight(r.tag, tagW))`, NAME via plain `padRight`), and the continuation-line render (blank TAG/NAME via `tagPad`/`namePad`) ALL port verbatim.

### `func displayWidth(s string) int`
```go
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}
```
Port the doc comment verbatim, INCLUDING the "KNOWN LIMITATION: wide CJK runes that render two cells wide are still counted as one; a full East-Asian width table would be needed for that, deliberately avoided to keep weave dependency-free (PRD §2 hard constraint)." Do NOT "fix" this — it is a documented, deliberate trade-off.

### `func padRight(s string, n int) string`
Port verbatim: early-return if `displayWidth(s) >= n`; else `s + strings.Repeat(" ", n-displayWidth(s))`. Port the doc comment verbatim (the "applied to PLAIN text before paint so ANSI escapes stay out of the width math" note is load-bearing).

### `func wrapWords(s string, width int) []string`
Port verbatim: `strings.Fields(s)`; empty → `[]string{""}`; greedy pack with `displayWidth(cur)+1+displayWidth(word) <= width`; long word on its own line (no split); final `lines = append(lines, cur)`. Port the doc comment verbatim.

### A NEW `internal/ui/ui_test.go` (`package ui`, white-box so it can call the unexported `displayWidth`/`padRight`/`wrapWords` and read the `ansiBold`/`ansiCyan`/`ansiReset` constants). Port skilldozer's `ui_test.go` with these edits:

1. **Import path**: `github.com/dabstractor/skilldozer/internal/discover` → `github.com/dabstractor/weave/internal/discover`.
2. **`mk` helper**: drop the `fm bool` param — `func mk(tag, name, desc string) discover.Extension { return discover.Extension{RelTag: tag, Name: name, Description: desc} }`. Update every call site (remove the trailing `true`/`false`).
3. **Sentinel tests MERGE**: skilldozer has `TestPrintListEmptyDescriptionShowsMissing` and `TestPrintListNoFrontmatterShowsMissing`. weave has only `TestPrintListEmptyDescriptionShowsNone` (empty `desc` → `(none)`). Drop the "no frontmatter" test — `Extension` has no frontmatter concept.
4. **Assertion strings**: every `"(missing)"` → `"(none)"`.
5. **Port the alignment helpers** `colOf` (byte column) and `runeCol` (rune column) VERBATIM — they are pure string math, test-only. `runeCol` is the one that catches the byte-vs-rune width bug; port `TestPrintListColumnsAlignedForMultibyte` verbatim (it uses `runeCol`).
6. **ADD weave-specific tests**:
   - `TestPrintListTrimsDescription` — a `Description` with leading/trailing whitespace renders trimmed (no leading spaces in the cell, no blank wrap line).
   - `TestPrintListJSDocOnlyDescriptionNotNone` — an `Extension` whose `Description` is non-empty (simulating a JSDoc-sourced desc on a single-file extension with no package.json) does NOT render `(none)` — proves the sentinel fires ONLY on empty, not on "no package.json."

### Success Criteria

- [ ] `internal/ui/ui.go` exists with `package ui`, the four constants, and the four functions with the exact signatures above.
- [ ] `PrintList`'s ONLY deltas from skilldozer are: (a) `[]discover.Extension` input, (b) `desc := strings.TrimSpace(ext.Description); if desc == "" { desc = "(none)" }`. No `HasFM` reference. No `(missing)` string anywhere.
- [ ] `PrintList(nil-exts, false)` writes nothing (early return).
- [ ] Empty `Name` → `(none)`; empty/whitespace `Description` → `(none)`.
- [ ] `useColor == false` → output contains ZERO `\x1b` bytes (grep `"\x1b"` → no match).
- [ ] `useColor == true` → output contains `ansiBold`, `ansiCyan`, AND `ansiReset` (header bold, TAG cyan, reset after every colored run).
- [ ] DESCRIPTION column is word-wrapped to `descWrapWidth` (40); every wrapped line fits within 40 columns (measured from the DESCRIPTION column start); long words go on their own line (not split).
- [ ] Continuation lines (wrapped DESCRIPTION) leave TAG and NAME cells blank (spaces) so columns stay aligned.
- [ ] Column widths are computed from PLAIN content; multi-byte tags (e.g. `café`) align under the header at the same DISPLAY (rune) column as ASCII rows. Pinned by `TestPrintListColumnsAlignedForMultibyte`.
- [ ] `PrintList` does NOT re-sort its input (it renders in slice order; `discover.Index` already sorted). Pinned by `TestPrintListPreservesInputOrder`.
- [ ] `displayWidth("café")==4`, `displayWidth("—")==1`; `padRight("café",5)=="café "`; `wrapWords("café bar",8)==["café bar"]`. Pinned by the three unit tests.
- [ ] `ui.go` imports ONLY `fmt`, `io`, `strings`, `unicode/utf8`, `github.com/dabstractor/weave/internal/discover`. NO third-party. NO `internal/config`, `internal/extdir`.
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/ui/...`, `go test -race ./...`, `go test ./...` all pass.
- [ ] go.mod has no `require` block; no `go.sum` exists.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_ **Yes.** This PRP names: the authoritative port source (`/home/dustin/projects/skilldozer/internal/ui/ui.go`, read in full, quoted in research/skilldozer_ui_port_diff.md line-by-line); the authoritative spec (PRD §6.1 table contract + §7.3 item 3 `(none)` rule, quoted); the input type contract (`discover.Extension` struct, read in full from `internal/discover/extension.go`, with the explicit "NO HasFM field" callout); the exact two edits (type swap + sentinel); the exact test patterns to port (skilldozer's `ui_test.go`, read in full); the multi-byte alignment regression and its `runeCol` helper; and the two weave-specific tests to add. The package is pure stdlib + one cross-package import that is already PRESENT. No external library docs needed.

### Documentation & References

```yaml
- file: /home/dustin/projects/skilldozer/internal/ui/ui.go  (READ-ONLY — the port source)
  why: THIS IS THE IMPLEMENTATION. Read it completely and port it. It contains the
        package doc comment, the four constants (ansiReset/ansiBold/ansiCyan,
        descWrapWidth), PrintList (the paint closure, the row/width computation,
        the header, the per-row loop, the continuation-line logic), displayWidth,
        padRight, and wrapWords. weave's ui.go is this file with TWO edits.
  pattern: |
    Port VERBATIM. The two edits:
    (1) Signature: []discover.Skill -> []discover.Extension (import path
        github.com/dabstractor/skilldozer/... -> github.com/dabstractor/weave/...).
    (2) The desc cell computation:
        skilldozer:  desc := strings.TrimSpace(s.Description)
                       if !s.HasFM || desc == "" { desc = "(missing)" }
        weave:       desc := strings.TrimSpace(ext.Description)
                       if desc == "" { desc = "(none)" }
        (Drop the !s.HasFM clause — Extension has no HasFM field. Change sentinel
        string (missing) -> (none) per PRD §7.3 item 3.)
    Everything else — the paint closure, padRight-before-paint ordering (keeps
    ANSI bytes out of width math), sep="  ", tagPad/namePad, bold header, cyan TAG,
    wrapWords call, continuation lines — ports byte-for-byte.
  critical: |
    The NON-OBVIOUS trick to preserve: compute column widths from PLAIN (uncolored)
    content, apply padRight to the PLAIN string, THEN wrap in paint(). If you pad
    AFTER painting, the ANSI escape bytes inflate len() and break alignment (and
    the colored vs uncolored tables would differ). skilldozer does this correctly;
    copy the ordering EXACTLY. Also preserve the early `if len(exts) == 0 { return }`
    — main.go owns the exit-1 decision (P1.M2.T5.S1), PrintList just renders.

- file: /home/dustin/projects/skilldozer/internal/ui/ui_test.go  (READ-ONLY — the test source)
  why: Port these tests. They cover: empty input, single row no-color, color emits
        ANSI, no-color has no ANSI, empty name -> (none), empty desc -> (missing)
        (CHANGE to (none)), folded-newline trim, long-description wrap, input-order
        preservation, cross-row column alignment, displayWidth unit cases,
        padRight multibyte, wrapWords multibyte, and the multi-byte column
        alignment regression (TestPrintListColumnsAlignedForMultibyte using runeCol).
  pattern: |
    Port colOf (byte column) and runeCol (rune column) helpers VERBATIM — they are
    pure string math, test-only. Port the mk helper but DROP the fm bool param:
      func mk(tag, name, desc string) discover.Extension {
          return discover.Extension{RelTag: tag, Name: name, Description: desc}
      }
    Merge the two skilldozer "(missing)" tests into ONE weave "(none)" test
    (TestPrintListEmptyDescriptionShowsNone). Change import path. Then ADD
    TestPrintListTrimsDescription + TestPrintListJSDocOnlyDescriptionNotNone.
  critical: |
    runeCol is the helper that catches the byte-vs-rune width bug. byte offsets are
    uniform under byte-padding (blind to the bug) and actually DIFFER under
    rune-padding. So TestPrintListColumnsAlignedForMultibyte MUST use runeCol, NOT
    colOf. Port it verbatim with its comment explaining why.

- file: /home/dustin/projects/weave/internal/discover/extension.go  (P1.M2.T1.S1, PRESENT — read in full)
  why: The Extension struct PrintList consumes. CONFIRMED fields:
        type Extension struct {
            Path, EntryFile, RelTag, Kind, Name, Description string
            Keywords, Aliases []string
            Category string
            HasPackageJSON bool
        }
        PrintList reads ONLY RelTag, Name, Description. It does NOT read Path,
        EntryFile, Kind, Keywords, Aliases, Category, or HasPackageJSON.
  critical: |
    Extension has NO HasFM field. skilldozer's Skill.HasFM (a bool from YAML
    frontmatter parsing) has no weave analog. The description sentinel MUST be
    `TrimSpace(ext.Description) == ""` — do NOT invent a HasFM check, do NOT use
    HasPackageJSON as a proxy (PRD §7.3: an extension with a package.json but
    empty description still renders (none); an extension with only a JSDoc desc
    renders the desc, NOT (none)). This is the single most important behavioral
    difference from skilldozer.

- file: /home/dustin/projects/weave/PRD.md
  why: Authoritative spec. §6.1 defines the --list/--search table contract
       ("Table: TAG, NAME, DESCRIPTION (wrapped)"; "--search ... Same table format
       as --list, filtered"). §7.3 item 3 is the (none) rule: "description = ""
       ... rendered as `(none)` in `--list`". §6.2 defines --no-color. §6.4
       "critical for $(...) use" extends to color contamination (color must be
       opt-in). §2 hard constraint: stdlib-only (forbids golang.org/x/term for
       terminal-width detection — hence fixed descWrapWidth=40).
  critical: |
    PRD §7.3 item 3 VERBATIM: "If neither [package.json description nor JSDoc]
    yields a description, `description = ""` (rendered as `(none)` in `--list`)."
    This is the source of the (none) sentinel and the reason the predicate is just
    `desc == ""` (no HasFM). §2: "No third-party dependencies" — do NOT add
    golang.org/x/term, go-runewidth, or anything else; the rune-count width
    approximation and the fixed wrap width are deliberate.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md
  why: §5 is THIS subtask's spec. Verbatim: "Port `PrintList`, `padRight`,
       `displayWidth`, `wrapWords`, ANSI constants verbatim. Column descriptions
       change: `HasFM` check for '(missing)' -> `Description == ""` check for
       '(none)'. PRD §7.3: empty description renders as `(none)` in `--list`.
       `descWrapWidth` stays at 40."
  critical: |
    §5 confirms the port is VERBATIM except the one sentinel change. It names
    exactly the four functions + ANSI constants to port and pins descWrapWidth=40.

- file: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M2T3S1/PRP.md  (parallel predecessor — treat as PRESENT)
  why: Defines discover.Index(dir) ([]Extension, error) — the function whose output
        PrintList consumes. Index returns []Extension SORTED by RelTag and with
        absolute Path/EntryFile. PrintList RELIES on the sort (it does NOT sort);
        P1.M2.T3.S1's TestIndexSortedByRelTag pins that contract. PrintList does
        not care about Path/EntryFile absoluteness (it only renders RelTag/Name/
        Description), so the absolute-output contract is irrelevant here, but the
        SORT contract is load-bearing for "input order preserved" semantics.
  critical: |
    PrintList renders in SLICE ORDER. main.go MUST pass a pre-sorted slice
    (discover.Index sorts; search.Search must preserve order when filtering —
    P1.M4.T1.S1's concern, not here). Do NOT add a sort inside PrintList — it
    would mask a caller bug and violate the "pure formatter" contract.

- file: /home/dustin/projects/weave/go.mod
  why: Module `github.com/dabstractor/weave`, `go 1.25`. ui.go imports are stdlib
        (fmt, io, strings, unicode/utf8) + one internal package
        (github.com/dabstractor/weave/internal/discover). go.mod gains NO require.
  critical: Adding any third-party dep would violate PRD §2 (stdlib-only).
```

### Current Codebase tree

```bash
# After P1.M1 (Complete) + M2.T1.S1 (Complete) + M2.T1.S2 (Complete) + M2.T2.S1 (parallel, PRESENT) + M2.T3.S1 (parallel, PRESENT).
# THIS subtask ADDS internal/ui/ui.go + internal/ui/ui_test.go.
$ cd /home/dustin/projects/weave && find . -name '*.go' -not -path './.git/*' -not -path './.pi-subagents/*' | sort
./internal/config/config.go            # P1.M1.T2.S1 (Complete)
./internal/config/config_test.go
./internal/extdir/extdir.go            # P1.M1.T3.S1+S2+S3 (Complete)
./internal/extdir/extdir_test.go
./internal/discover/extension.go       # P1.M2.T1.S1 (Complete — PRESENT; Extension struct)
./internal/discover/extension_test.go  # P1.M2.T1.S1 (Complete — PRESENT)
./internal/discover/jsdoc.go           # P1.M2.T1.S2 (Complete — PRESENT)
./internal/discover/jsdoc_test.go      # P1.M2.T1.S2 (Complete — PRESENT)
./internal/discover/discover.go        # P1.M2.T2.S1 (parallel — PRESENT; classify*)
./internal/discover/discover_test.go   # P1.M2.T2.S1 (parallel — PRESENT)
./internal/discover/index.go           # P1.M2.T3.S1 (parallel — PRESENT; Index)
./internal/discover/index_test.go      # P1.M2.T3.S1 (parallel — PRESENT)
./main.go                              # P1.M1.T4.S1 (Complete — does NOT yet import ui)
./main_test.go
# THIS subtask ADDS:
#   ./internal/ui/ui.go        (PrintList, displayWidth, padRight, wrapWords, ANSI constants)
#   ./internal/ui/ui_test.go   (ported + 2 new tests)
```

### Desired Codebase tree with files to be added

```bash
weave/
├── internal/
│   ├── config/                    # UNCHANGED
│   ├── extdir/                    # UNCHANGED
│   ├── discover/                  # S1+S2+T2+T3 PRESENT (Extension struct is the input type)
│   │   ├── extension.go           # PRESENT — Extension struct (RelTag, Name, Description, ...)
│   │   ├── jsdoc.go               # PRESENT
│   │   ├── discover.go            # PRESENT
│   │   └── index.go               # PRESENT — Index() []Extension (sorted by RelTag)
│   └── ui/                        # NEW package (this subtask)
│       ├── ui.go                  # NEW — PrintList, displayWidth, padRight, wrapWords, constants
│       └── ui_test.go             # NEW — ported tests + TestPrintListTrimsDescription + TestPrintListJSDocOnlyDescriptionNotNone
├── main.go                        # UNCHANGED (P1.M2.T5.S1 wires --list → ui.PrintList later)
├── go.mod                         # UNCHANGED (no new require; stdlib + internal/discover only)
└── ...                            # everything else unchanged
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Extension has NO HasFM field): skilldozer's Skill.HasFM drove the
// (missing) sentinel. weave's Extension has HasPackageJSON, NOT HasFM. PRD §7.3
// item 3 says empty description -> "(none)" regardless of source. So the
// predicate is JUST `strings.TrimSpace(ext.Description) == ""`. Do NOT use
// HasPackageJSON as a proxy: an extension with a package.json but empty desc
// still renders (none); an extension with only a JSDoc desc renders the desc,
// NOT (none). This is THE behavioral delta from skilldozer.

// CRITICAL (padRight BEFORE paint, never after): compute column widths from PLAIN
// (uncolored) content, apply padRight to the PLAIN string, THEN wrap in paint().
// If you pad AFTER painting, the ANSI escape bytes (\x1b[36m ... \x1b[0m = 9+
// invisible bytes per cyan cell) inflate len() and misalign columns — AND the
// colored table would differ from the uncolored one. skilldozer does this
// correctly; copy the ordering EXACTLY. The paint closure returns `code + s +
// ansiReset` only when useColor is true, else passthrough — so the SAME padRight
// call works for both modes.

// CRITICAL (displayWidth uses RUNE count, not byte len): utf8.RuneCountInString,
// NOT len(). A multi-byte rune like é (2 bytes, 1 rune/column) or — (3 bytes,
// 1 rune/column) must count as ONE column for padding/wrapping. Using len()
// would over-pad multi-byte cells and misalign columns. Pinned by
// TestPadRightMultibyte + TestWrapWordsMultibyte + TestPrintListColumnsAlignedForMultibyte.
// KNOWN LIMITATION (document, do NOT fix): wide CJK runes (2 display cells) are
// undercounted as 1. A full East-Asian width table needs a dep; PRD §2 forbids
// it. Port skilldozer's limitation comment verbatim.

// CRITICAL (color is OPT-IN; default must be byte-clean): useColor == false must
// produce ZERO \x1b bytes. PRD §6.4 "critical for $(...) use" means list output
// that gets grepped/piped must be clean text. The paint closure is a passthrough
// when useColor is false — no ANSI bytes are emitted. main.go (P1.M2.T5.S1)
// computes useColor = isTerminal(os.Stdout) && !noColor; PrintList just honors
// the flag. Pin with TestPrintListNoColorHasNoANSI.

// CRITICAL (PrintList does NOT sort): it renders in slice order. discover.Index
// (P1.M2.T3.S1) sorts by RelTag; search.Search (P1.M4.T1.S1) must preserve order
// when filtering. Adding a sort inside PrintList would mask caller bugs and
// violate the "pure formatter" contract. Pin with TestPrintListPreservesInputOrder
// (pass zebra-then-apple; assert zebra row renders first).

// CRITICAL (empty slice prints NOTHING, not a header): `if len(exts) == 0
// { return }` — early return BEFORE writing the header. main.go (P1.M2.T5.S1)
// owns the "exit 1 if no extensions found" decision; PrintList is defensive, not
// authoritative. Pin with TestPrintListEmptyPrintsNothing (assert buf.Len()==0).

// CRITICAL (wrapWords empty -> []string{""}, NOT []string{}): a zero-length
// return would make `descLines[0]` panic. The `[]string{""}` contract means
// callers can ALWAYS index [0] for the first description line. Port this
// verbatim. strings.Fields collapses runs of spaces and drops leading/trailing
// whitespace, so `""` and `"   "` both yield []string{""}.

// CRITICAL (wrapWords does NOT split long words): a single word longer than
// width goes on its OWN line (the `default:` branch starts a new line with the
// word, even if the word itself exceeds width). This is intentional — splitting
// mid-word would corrupt identifiers/URLs. Port verbatim.

// GOTCHA (imports: fmt, io, strings, unicode/utf8, internal/discover): do NOT
// import os (PrintList takes an io.Writer, it does not touch the filesystem),
// encoding/json (no parsing here — Extension is already built), or any terminal-
// detection library (useColor is a parameter). The discover import is the ONLY
// cross-package dep.

// GOTCHA (package doc comment lives in ui.go): there is exactly one file in the
// ui package (plus its test), so the package doc comment goes at the top of
// ui.go, ABOVE `package ui`. Keep it (godoc uses it). Adapt skilldozer's wording:
// "skill catalog" -> "extension catalog", "skilldozer" -> "weave", update the
// cross-references to weave's task IDs (P1.M2.T5.S1 for --list wiring,
// P1.M4.T1.S1 for --search).

// GOTCHA (continuation lines leave TAG AND NAME blank): when a description wraps
// to multiple lines, lines 2..N render with tagPad (spaces, width tagW) + sep +
// namePad (spaces, width nameW) + sep + wrappedLine. This keeps the DESCRIPTION
// column aligned across the whole wrapped block. Port the continuation loop
// verbatim — it precomputes tagPad/namePad once before the row loop.

// GOTCHA (the header has NO trailing padRight on DESCRIPTION): the last column
// (DESCRIPTION) is painted bold but NOT padded — there's nothing to its right.
// padRight("DESCRIPTION", someWidth) would inject trailing spaces. skilldozer
// paints the bare "DESCRIPTION" string. Copy that (the Fprintf for the header
// uses paint(ansiBold, "DESCRIPTION"), not padRight(...)).
```

## Implementation Blueprint

### Data models and structure

NO new data models. This subtask CONSUMES `discover.Extension` (PRESENT from P1.M2.T1.S1) and produces formatted text on an `io.Writer`. It adds only four functions and four constants.

```go
// The complete symbol set of ui.go:
const (
    ansiReset    = "\x1b[0m"
    ansiBold     = "\x1b[1m"
    ansiCyan     = "\x1b[36m"
)
const descWrapWidth = 40

func PrintList(w io.Writer, exts []discover.Extension, useColor bool)
func displayWidth(s string) int
func padRight(s string, n int) string
func wrapWords(s string, width int) []string
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE ui.go skeleton + package doc + imports + constants
  - CREATE /home/dustin/projects/weave/internal/ui/ui.go with `package ui`.
  - PACKAGE DOC COMMENT (port skilldozer's, adapt wording — see "What" section):
    "Package ui renders the human-readable extension catalog table for weave's
    --list and --search modes (PRD §6.1)..." Update task cross-refs:
    --list wiring is P1.M2.T5.S1; --search is P1.M4.T1.S1.
  - IMPORTS (exact set, alphabetical): fmt, io, strings, unicode/utf8,
    github.com/dabstractor/weave/internal/discover.
    (Do NOT import os, encoding/json, or any terminal-detection lib.)
  - CONSTANTS (port VERBATIM):
        const (
            ansiReset = "\x1b[0m"
            ansiBold  = "\x1b[1m"
            ansiCyan  = "\x1b[36m"
        )
        const descWrapWidth = 40
  - PORT the descWrapWidth rationale comment VERBATIM (the "no TIOCGWINSZ /
    golang.org/x/term — PRD hard constraint forbids the dep" reasoning; change
    "skilldozer" -> "weave" and "PRD §4/§7.3" -> "PRD §2" for the dep constraint).

Task 2: IMPLEMENT displayWidth + padRight + wrapWords (port VERBATIM)
  - These three helpers port from skilldozer ui.go BYTE-FOR-BYTE (they reference
    no Skill/Extension type, so the type swap does not touch them).
  - displayWidth: `return utf8.RuneCountInString(s)`. Port the doc comment
    INCLUDING the "KNOWN LIMITATION: wide CJK runes counted as one; deliberate,
    PRD §2 forbids the width-table dep" note.
  - padRight: early-return if `displayWidth(s) >= n`; else
    `return s + strings.Repeat(" ", n-displayWidth(s))`. Port the doc comment
    (the "applied to PLAIN text before paint" note is load-bearing).
  - wrapWords: port the full body — strings.Fields; empty -> []string{""};
    greedy pack; long word on own line; final append. Port the doc comment.
  - NOTE: define these BEFORE PrintList or AFTER — Go does not care about order
    within a package. skilldozer defines them AFTER PrintList; match that for
    diff-friendliness.

Task 3: IMPLEMENT PrintList (port + the TWO edits)
  - SIGNATURE: `func PrintList(w io.Writer, exts []discover.Extension, useColor bool)`.
  - PORT the comprehensive doc comment, adapting: "skills" -> "extensions",
    "Skill.RelTag" -> "Extension.RelTag", the DESCRIPTION rule to:
    "Extension.Description, or '(none)' when the description is empty/blank
    (PRD §7.3 item 3)." Drop any mention of HasFM.
  - BODY (port skilldozer's PrintList VERBATIM with these edits):
    1. Early return: `if len(exts) == 0 { return }`.
    2. Width init: `tagW := len("TAG"); nameW := len("NAME")`.
    3. cells struct + rows slice: `rows := make([]cells, len(exts))`.
    4. LOOP `for i, ext := range exts`:
       - NAME: `name := ext.Name; if name == "" { name = "(none)" }`.
       - DESC (THE EDIT):
             desc := strings.TrimSpace(ext.Description)
             if desc == "" {
                 desc = "(none)"
             }
         (Compare skilldozer: `if !s.HasFM || desc == "" { desc = "(missing)" }`.
          Drop the HasFM clause; change sentinel to (none).)
       - Width growth: `if displayWidth(ext.RelTag) > tagW { tagW = ... }`;
         `if displayWidth(name) > nameW { nameW = ... }`.
       - `rows[i] = cells{ext.RelTag, name, desc}`.
    5. paint closure (VERBATIM): `paint := func(code, s string) string { if !useColor { return s }; return code + s + ansiReset }`.
    6. `const sep = "  "; tagPad := strings.Repeat(" ", tagW); namePad := strings.Repeat(" ", nameW)`.
    7. HEADER (VERBATIM): Fprintf with paint(ansiBold, padRight("TAG", tagW)),
       sep, paint(ansiBold, padRight("NAME", nameW)), sep, paint(ansiBold, "DESCRIPTION").
       (NOTE: "DESCRIPTION" is NOT padRight'd — bare string inside paint.)
    8. ROW LOOP `for _, r := range rows`:
       - `descLines := wrapWords(r.desc, descWrapWidth)`.
       - First line: Fprintf with paint(ansiCyan, padRight(r.tag, tagW)), sep,
         padRight(r.name, nameW), sep, descLines[0].
       - Continuation: `for _, line := range descLines[1:]` Fprintf with
         tagPad, sep, namePad, sep, line.
  - NOTE: the padRight-before-paint ordering in steps 7-8 is load-bearing
    (see GOTCHA). Copy skilldozer's Fprintf format strings EXACTLY.

Task 4: CREATE ui_test.go — helpers + empty/single/color tests (port)
  - CREATE /home/dustin/projects/weave/internal/ui/ui_test.go with `package ui`.
  - IMPORTS: bytes, strings, testing, unicode/utf8,
    github.com/dabstractor/weave/internal/discover.
  - mk helper (DROP the fm param):
        func mk(tag, name, desc string) discover.Extension {
            return discover.Extension{RelTag: tag, Name: name, Description: desc}
        }
  - colOf + runeCol helpers: port VERBATIM (pure string math).
  - PORT: TestPrintListEmptyPrintsNothing (PrintList(&buf, nil, false); buf.Len()==0).
  - PORT: TestPrintListSingleNoColor (contains TAG/NAME/DESCRIPTION/example/
    "A demo skill."; no "\x1b["; header precedes data).
  - PORT: TestPrintListColorEmitsANSI (useColor=true; contains ansiBold, ansiCyan,
    ansiReset).
  - PORT: TestPrintListNoColorHasNoANSI (useColor=false; no "\x1b").
  - PORT: TestPrintListMissingNameShowsNone (empty Name -> "(none)").

Task 5: CREATE ui_test.go — the (none) sentinel test (port + merge)
  - PORT + MERGE skilldozer's two tests into ONE:
        func TestPrintListEmptyDescriptionShowsNone(t *testing.T) {
            var buf bytes.Buffer
            PrintList(&buf, []discover.Extension{mk("a", "a", "")}, false)
            if !strings.Contains(buf.String(), "(none)") {
                t.Errorf("empty description should render (none):\n%s", buf.String())
            }
        }
  - NOTE: skilldozer has a SEPARATE TestPrintListNoFrontmatterShowsMissing. weave
    has NO frontmatter concept — an Extension is just {RelTag, Name, Description}.
    Empty Description -> (none) is the ONLY case. Do NOT port the frontmatter test.

Task 6: CREATE ui_test.go — wrap/trim/order/alignment tests (port)
  - PORT: TestPrintListTrimsFoldedScalarNewline -> rename to
    TestPrintListTrimsDescription. Use a Description with a trailing newline
    ("has trailing newline\n"); assert the text appears AND no "\n\n" (blank line)
    in output. (weave's Description can carry a trailing newline if a JSDoc or
    package.json desc has one; TrimSpace normalizes it.)
  - PORT: TestPrintListWrapsLongDescription (long desc; assert >= 3 lines; every
    wrapped line fits within descWrapWidth measured from descCol; all words
    survive in the joined output). Use a weave-flavored long description.
  - PORT: TestPrintListPreservesInputOrder (zebra then apple; assert zebra row
    before apple row — PrintList does NOT sort).
  - PORT: TestPrintListColumnsAlignedAcrossRows (two rows with differing tag/name
    widths; assert every description starts at descCol; NAME aligned under NAME
    header). Uses colOf.

Task 7: CREATE ui_test.go — multibyte regression tests (port)
  - PORT: TestDisplayWidth (café=4, —=1, a—b=3, ascii=5, ""=0).
  - PORT: TestPadRightMultibyte (café,5 -> "café "; éé,4 -> "éé  "; ascii,3 ->
    "ascii"; "",3 -> "   ").
  - PORT: TestWrapWordsMultibyte (wrapWords("café bar",8) == ["café bar"]).
  - PORT: TestPrintListColumnsAlignedForMultibyte (café + ascii rows; use runeCol
    to assert every description at the same DISPLAY column as DESCRIPTION header;
    NAME aligned under NAME header). Uses runeCol — NOT colOf (see GOTCHA).

Task 8: CREATE ui_test.go — weave-specific tests (NEW)
  - ADD TestPrintListTrimsDescription (leading/trailing whitespace):
        desc := "   padded description with spaces   "
        PrintList(&buf, []discover.Extension{mk("x", "x", desc)}, false)
        out := buf.String()
        // The rendered description is trimmed (no leading spaces in the cell).
        // Find the line containing the description and assert it has no run of
        // 2+ leading spaces after the DESCRIPTION column separator.
        if !strings.Contains(out, "padded description with spaces") {
            t.Errorf("trimmed desc text missing:\n%s", out)
        }
    (TrimSpace is called on ext.Description before the empty check AND before
    wrapWords, so leading/trailing whitespace is gone. wrapWords then re-joins
    internal spaces normally.)
  - ADD TestPrintListJSDocOnlyDescriptionNotNone:
        // An extension whose Description comes from JSDoc (no package.json, but
        // BuildExtension filled Description from the JSDoc fallback). The desc is
        // non-empty, so it must NOT render (none).
        PrintList(&buf, []discover.Extension{mk("gate", "", "Gate the flow.")}, false)
        out := buf.String()
        if strings.Contains(out, "(none)") {
            // Wait — Name is "" here, so "(none)" WILL appear for the NAME cell.
            // To isolate the DESCRIPTION check, give it a Name:
        }
        // CORRECTED: give a Name so only the desc is under test.
        PrintList(&buf, []discover.Extension{mk("gate", "gate", "Gate the flow.")}, false)
        out = buf.String()
        if strings.Contains(out, "(none)") {
            t.Errorf("non-empty JSDoc-sourced desc must not render (none):\n%s", out)
        }
        if !strings.Contains(out, "Gate the flow.") {
            t.Errorf("desc text missing:\n%s", out)
        }
  - NOTE: the corrected version of TestPrintListJSDocOnlyDescriptionNotNone must
    supply a non-empty Name, otherwise the NAME cell's (none) would false-positive
    the assertion. (This subtlety is the reason the test exists — it proves the
    (none) sentinel is per-cell, not per-row.)

Task 9: VALIDATE build, vet, test, deps
  - RUN: cd /home/dustin/projects/weave && go build ./...                    # expect exit 0
  - RUN: go vet ./...                                                        # expect exit 0, clean
  - RUN: go test ./internal/ui/... -v                                        # expect ALL PASS
  - RUN: go test ./... -v                                                    # expect ALL PASS
  - RUN: go test -race ./...                                                 # expect no data races
  - RUN: gofmt -l internal/ui/ui.go internal/ui/ui_test.go                   # expect no output
  - RUN: grep -q "HasFM\|(missing)" ./internal/ui/ui.go && echo "FAIL: HasFM/(missing) must not appear" || echo "OK"
  - RUN: grep -q "(none)" ./internal/ui/ui.go && echo "OK (sentinel present)" || echo "FAIL: (none) missing"
  - RUN: grep -qE "golang.org/x/term|go-runewidth|github.com/!.*ui" ./internal/ui/ui.go && echo "FAIL: forbidden third-party dep" || echo "OK"
  - RUN: grep -q "^require" go.mod && echo FAIL || echo OK                   # expect OK
  - RUN: test ! -f go.sum && echo OK || echo FAIL                            # expect OK
  - RUN: ! grep -qE "os\.|encoding/json" ./internal/ui/ui.go && echo "no os/json imports (correct)" || echo "FAIL"
```

### Implementation Patterns & Key Details

```go
// The COMPLETE ui.go (≈120 lines). Port skilldozer's ui.go; the ONLY non-cosmetic
// edits are the signature type and the desc cell (marked <<EDIT>>).

package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/dabstractor/weave/internal/discover"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
)

const descWrapWidth = 40

// PrintList writes the TAG/NAME/DESCRIPTION catalog table for extensions to w.
// [port skilldozer's doc comment; adapt "skills"->"extensions", drop HasFM refs]
func PrintList(w io.Writer, exts []discover.Extension, useColor bool) {  // <<EDIT: Skill->Extension>>
	if len(exts) == 0 {
		return
	}

	tagW := len("TAG")
	nameW := len("NAME")
	type cells struct{ tag, name, desc string }
	rows := make([]cells, len(exts))
	for i, ext := range exts {                                            // <<EDIT: s->ext>>
		name := ext.Name
		if name == "" {
			name = "(none)"
		}
		// <<EDIT: the load-bearing change>> ---------------------------
		desc := strings.TrimSpace(ext.Description)
		if desc == "" {
			desc = "(none)"
		}
		// (skilldozer had: if !s.HasFM || desc == "" { desc = "(missing)" })
		// -------------------------------------------------------------
		if displayWidth(ext.RelTag) > tagW {
			tagW = displayWidth(ext.RelTag)
		}
		if displayWidth(name) > nameW {
			nameW = displayWidth(name)
		}
		rows[i] = cells{ext.RelTag, name, desc}
	}

	paint := func(code, s string) string {
		if !useColor {
			return s
		}
		return code + s + ansiReset
	}

	const sep = "  "
	tagPad := strings.Repeat(" ", tagW)
	namePad := strings.Repeat(" ", nameW)

	fmt.Fprintf(w, "%s%s%s%s%s\n",
		paint(ansiBold, padRight("TAG", tagW)),
		sep,
		paint(ansiBold, padRight("NAME", nameW)),
		sep,
		paint(ansiBold, "DESCRIPTION"),
	)

	for _, r := range rows {
		descLines := wrapWords(r.desc, descWrapWidth)
		fmt.Fprintf(w, "%s%s%s%s%s\n",
			paint(ansiCyan, padRight(r.tag, tagW)),
			sep,
			padRight(r.name, nameW),
			sep,
			descLines[0],
		)
		for _, line := range descLines[1:] {
			fmt.Fprintf(w, "%s%s%s%s%s\n", tagPad, sep, namePad, sep, line)
		}
	}
}

func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

func padRight(s string, n int) string {
	if displayWidth(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-displayWidth(s))
}

func wrapWords(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, len(words))
	cur := ""
	for _, word := range words {
		switch {
		case cur == "":
			cur = word
		case displayWidth(cur)+1+displayWidth(word) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	lines = append(lines, cur)
	return lines
}
```

### Integration Points

```yaml
PACKAGE BOUNDARY:
  - ui.go is `package ui` — a NEW package. It imports
    github.com/dabstractor/weave/internal/discover (PRESENT from P1.M2.T1.S1).
  - PrintList is the ONLY export downstream code calls. displayWidth/padRight/
    wrapWords are unexported helpers (tested white-box via ui_test.go in the
    same package).

MAIN.GO (FUTURE — NOT this subtask):
  - P1.M2.T5.S1 (--list):  exts, err := discover.Index(dir); if len(exts)==0 {
                              exit 1 "no extensions found" }
                            ui.PrintList(os.Stdout, exts, isTerminal(os.Stdout) && !noColor)
  - P1.M4.T1.S1 (--search): matches := search.Search(exts, q); if len(matches)==0 {
                              exit 1 }
                            ui.PrintList(os.Stdout, matches, isTerminal(os.Stdout) && !noColor)
  - This subtask does NOT touch main.go. It only provides PrintList.

CONFIG:
  - none. PrintList takes an io.Writer and a bool. The color decision (TTY detect,
    --no-color flag) is the caller's job (main.go).

DATABASE:
  - none.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating ui.go + ui_test.go — fix before proceeding.
cd /home/dustin/projects/weave
gofmt -l internal/ui/ui.go internal/ui/ui_test.go   # expect no output
go vet ./internal/ui/...                              # expect clean

# Project-wide validation
go build ./...                                        # expect exit 0
go vet ./...                                          # expect exit 0, clean

# Expected: Zero errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the ui package.
cd /home/dustin/projects/weave
go test ./internal/ui/... -v                          # expect ALL PASS
go test ./internal/ui/... -run 'TestPrintList' -v     # just PrintList tests
go test ./internal/ui/... -run 'TestDisplayWidth|TestPadRight|TestWrapWords' -v  # helpers

# Targeted: the load-bearing (none) sentinel (the weave-specific behavior)
go test ./internal/ui/... -run 'TestPrintListEmptyDescriptionShowsNone|TestPrintListJSDocOnlyDescriptionNotNone' -v

# Targeted: the multibyte alignment regression (the byte-vs-rune width bug)
go test ./internal/ui/... -run 'Multibyte' -v

# Race detector + full suite
go test -race ./...
go test ./... -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
```

### Level 3: Integration Testing (System Validation)

```bash
# Confirm the package builds and PrintList is callable from outside the package.
cd /home/dustin/projects/weave
# (ui.PrintList is not yet wired into main.go — P1.M2.T5.S1 does that. This
# subtask is verified via the white-box ui_test.go in Level 2. The integration
# check here is a manual smoke test proving the rendered table matches PRD §6.1.)

# Smoke-test PrintList directly via a tiny throwaway program:
cat > /tmp/ui_smoke_test.go <<'EOF'
package main
import (
    "os"
    "github.com/dabstractor/weave/internal/discover"
    "github.com/dabstractor/weave/internal/ui"
)
func main() {
    exts := []discover.Extension{
        {RelTag: "gate", Name: "gate", Description: "Gate the flow."},
        {RelTag: "writing/reddit-poster", Name: "reddit", Description: ""},
        {RelTag: "summarizer", Name: "", Description: "Summarize documents and conversations."},
    }
    ui.PrintList(os.Stdout, exts, false)
    println("---")
    ui.PrintList(os.Stdout, exts, true)
}
EOF
go run /tmp/ui_smoke_test.go
# Expected: a TAG/NAME/DESCRIPTION table; writing/reddit-poster row shows (none)
# for the empty description; summarizer row shows (none) for the empty name;
# the colored version has cyan tags + bold header; the plain version has no escapes.
rm -f /tmp/ui_smoke_test.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Visual alignment check (manual): pipe through cat -A to confirm column alignment
# and that no-color output has zero escape bytes.
cd /home/dustin/projects/weave
cat > /tmp/ui_align_check.go <<'EOF'
package main
import (
    "os"
    "github.com/dabstractor/weave/internal/discover"
    "github.com/dabstractor/weave/internal/ui"
)
func main() {
    exts := []discover.Extension{
        {RelTag: "café", Name: "cafe-name", Description: "café — skill"},
        {RelTag: "ascii", Name: "ascii-name", Description: "ascii skill"},
        {RelTag: "x", Name: "x", Description: "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore"},
    }
    ui.PrintList(os.Stdout, exts, false)
}
EOF
go run /tmp/ui_align_check.go | cat -A
# Expected: every DESCRIPTION starts at the same column (the café row's multi-byte
# tag does NOT shift it); the long description wraps at <=40 cols with continuation
# lines leaving TAG/NAME blank; cat -A shows NO $-prefixed escape bytes (^[[).
rm -f /tmp/ui_align_check.go
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `cd /home/dustin/projects/weave && go test ./... -v`
- [ ] `go test -race ./...` passes (no data races).
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/ui/ui.go internal/ui/ui_test.go` (no output)
- [ ] go.mod has no `require` block; no `go.sum` exists.

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] `PrintList(nil, false)` writes nothing (TestPrintListEmptyPrintsNothing).
- [ ] Empty Name → `(none)`; empty/whitespace Description → `(none)` (TestPrintListEmptyDescriptionShowsNone).
- [ ] Non-empty Description (JSDoc-sourced) does NOT render `(none)` (TestPrintListJSDocOnlyDescriptionNotNone).
- [ ] Description with leading/trailing whitespace renders trimmed (TestPrintListTrimsDescription).
- [ ] `useColor == false` → zero `\x1b` bytes (TestPrintListNoColorHasNoANSI).
- [ ] `useColor == true` → header bold, TAG cyan, reset after every run (TestPrintListColorEmitsANSI).
- [ ] DESCRIPTION wraps at ≤40 cols; continuation lines leave TAG/NAME blank (TestPrintListWrapsLongDescription).
- [ ] Multi-byte tags align under the header (TestPrintListColumnsAlignedForMultibyte).
- [ ] Input order preserved, NOT re-sorted (TestPrintListPreservesInputOrder).

### Code Quality Validation

- [ ] `ui.go` imports ONLY `fmt`, `io`, `strings`, `unicode/utf8`, `internal/discover`.
- [ ] NO `HasFM` reference; NO `(missing)` string anywhere (grep confirms).
- [ ] `(none)` sentinel present (grep confirms).
- [ ] No third-party deps (no golang.org/x/term, no go-runewidth).
- [ ] padRight applied BEFORE paint (ANSI bytes stay out of width math).
- [ ] displayWidth uses utf8.RuneCountInString (not len).
- [ ] wrapWords empty → `[]string{""}` (never zero-length).
- [ ] Package doc comment present and accurate (weave wording, correct task cross-refs).

### Documentation & Deployment

- [ ] PrintList has a comprehensive doc comment (column rules, color policy, empty-slice behavior).
- [ ] displayWidth/padRight/wrapWords doc comments port skilldozer's (including the CJK limitation note).
- [ ] No environment variables introduced (PrintList takes params, not env).

---

## Anti-Patterns to Avoid

- ❌ Don't invent a `HasFM`-like check — `Extension` has no such field; PRD §7.3 says empty desc → `(none)`, full stop.
- ❌ Don't use `HasPackageJSON` as a proxy for the description sentinel — an extension with a package.json but empty description STILL renders `(none)`; an extension with only a JSDoc desc renders the desc.
- ❌ Don't render `(missing)` — that's skilldozer's sentinel; weave's is `(none)` (PRD §7.3 item 3).
- ❌ Don't pad after painting — ANSI escape bytes corrupt the width math; pad the PLAIN string, then paint.
- ❌ Don't use `len()` for column width — multi-byte runes misalign; use `displayWidth` (rune count).
- ❌ Don't add a terminal-width detection dep — PRD §2 forbids non-stdlib; `descWrapWidth=40` is fixed and deliberate.
- ❌ Don't sort inside `PrintList` — it's a pure formatter; `discover.Index` sorts, `search.Search` preserves order.
- ❌ Don't write a header for an empty slice — early-return on `len(exts)==0`; `main.go` owns the exit-1 decision.
- ❌ Don't split long words in `wrapWords` — put them on their own line (splitting corrupts identifiers/URLs).
- ❌ Don't return `[]string{}` from `wrapWords("")` — callers index `[0]`; return `[]string{""}`.
- ❌ Don't import `os` or `encoding/json` — `PrintList` takes an `io.Writer` and a pre-built `[]Extension`; it touches neither the filesystem nor JSON.
- ❌ Don't port skilldozer's `TestPrintListNoFrontmatterShowsMissing` — weave has no frontmatter concept; merge into the single `(none)` test.
