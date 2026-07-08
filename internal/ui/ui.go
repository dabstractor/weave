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

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/dabstractor/weave/internal/discover"
)

// ANSI SGR escape sequences. Only emitted when useColor is true. The reset
// (ansiReset) is appended after every colored run so a single cell cannot bleed
// color into the next. All are plain byte-literal strings — no third-party dep.
const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
)

// descWrapWidth is the column width at which the DESCRIPTION cell is word-wrapped.
// weave deliberately does NOT detect terminal width: that needs a TIOCGWINSZ ioctl
// or golang.org/x/term, and PRD §2 keeps weave stdlib-only (no third-party
// dependencies). A fixed width keeps output deterministic and testable and fits a
// standard 80-column terminal alongside typical TAG/NAME widths.
const descWrapWidth = 40

// PrintList writes the TAG/NAME/DESCRIPTION catalog table for extensions to w. It
// implements PRD §6.1 `weave --list`. exts MUST already be ordered the way rows
// should appear: discover.Index sorts by RelTag; --search passes its filtered
// (still-sorted) slice. An empty slice prints nothing — main exits 1 "if no
// extensions found" before calling this (PRD §6.1, wired in P1.M2.T5.S1);
// PrintList is defensive, not authoritative.
//
// Column rules:
//   - TAG:  Extension.RelTag (the canonical '/'-normalized tag from discover).
//   - NAME: Extension.Name, or "(none)" when the name is empty.
//   - DESCRIPTION: Extension.Description, or "(none)" when the description is
//     empty/blank (PRD §7.3 item 3: "If neither [package.json description nor
//     JSDoc] yields a description, `description = ""` (rendered as `(none)` in
//     `--list`)"). weave has no HasFM analog — the only signal is "is the
//     description empty after trim?".
//
// DESCRIPTION is word-wrapped to descWrapWidth; continuation lines leave the TAG
// and NAME cells blank (spaces) so columns stay aligned.
//
// When useColor is false (non-TTY stdout, or --no-color set), output is plain
// text — exactly what `$(...)`, pipes, log files, and tests see (PRD §6.4
// "critical for `$(...)` use"). Color is applied to the header (bold) and the TAG
// column (cyan); NAME/DESCRIPTION stay default.
func PrintList(w io.Writer, exts []discover.Extension, useColor bool) {
	if len(exts) == 0 {
		return
	}

	// Compute display cells + dynamic column widths from the PLAIN content, so the
	// widths are independent of whether color is on.
	tagW := len("TAG")
	nameW := len("NAME")
	type cells struct{ tag, name, desc string }
	rows := make([]cells, len(exts))
	for i, ext := range exts {
		name := ext.Name
		if name == "" {
			name = "(none)"
		}
		// Empty/blank description -> "(none)" (PRD §7.3 item 3: "description = "" ...
		// rendered as `(none)` in `--list`"). TrimSpace normalizes whitespace-only
		// descriptions AND any trailing newline (a folded-scalar or JSDoc desc may
		// carry one) so it does not inject a blank line into the wrap. weave has no
		// HasFM analog — the only signal is "is the description empty after trim?".
		desc := strings.TrimSpace(ext.Description)
		if desc == "" {
			desc = "(none)"
		}
		if displayWidth(ext.RelTag) > tagW {
			tagW = displayWidth(ext.RelTag)
		}
		if displayWidth(name) > nameW {
			nameW = displayWidth(name)
		}
		rows[i] = cells{ext.RelTag, name, desc}
	}

	// paint wraps s in an SGR sequence + reset when color is on; otherwise it is a
	// passthrough. padRight is applied to the PLAIN string BEFORE paint so visible
	// column alignment is unaffected by the (invisible) escape bytes (their bytes
	// would otherwise corrupt the len()-based padding math).
	paint := func(code, s string) string {
		if !useColor {
			return s
		}
		return code + s + ansiReset
	}

	const sep = "  "
	tagPad := strings.Repeat(" ", tagW)
	namePad := strings.Repeat(" ", nameW)

	// Header row: bold labels.
	fmt.Fprintf(w, "%s%s%s%s%s\n",
		paint(ansiBold, padRight("TAG", tagW)),
		sep,
		paint(ansiBold, padRight("NAME", nameW)),
		sep,
		paint(ansiBold, "DESCRIPTION"),
	)

	for _, r := range rows {
		descLines := wrapWords(r.desc, descWrapWidth)
		// First description line shares the row with the TAG + NAME cells (tag cyan).
		fmt.Fprintf(w, "%s%s%s%s%s\n",
			paint(ansiCyan, padRight(r.tag, tagW)),
			sep,
			padRight(r.name, nameW),
			sep,
			descLines[0],
		)
		// Continuation lines: blank TAG/NAME cells, plain wrapped description.
		for _, line := range descLines[1:] {
			fmt.Fprintf(w, "%s%s%s%s%s\n", tagPad, sep, namePad, sep, line)
		}
	}
}

// displayWidth returns the number of display columns s occupies, approximated as
// its UTF-8 rune count. It replaces len(s) wherever a string's rendered width
// matters (column sizing, padding, word-wrap) so a multi-byte rune like é (2
// bytes, 1 rune) or — (3 bytes, 1 rune) counts as one column instead of 2–4 bytes.
// KNOWN LIMITATION: wide CJK runes that render two cells wide are still counted
// as one; a full East-Asian width table would be needed for that, deliberately
// avoided to keep weave dependency-free (PRD §2). See padRight for usage.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// padRight returns s right-padded with spaces to display width n. If s is already
// n or more columns wide it is returned unchanged (no truncation). Width is
// measured in RUNES via displayWidth (utf8.RuneCountInString), not bytes: a
// multi-byte rune like é (2 bytes, 1 column) or — (3 bytes, 1 column) renders one
// cell wide, so rune count gives correct padding for the common case (é, —, smart
// quotes, single-cell emoji). Applied to PLAIN text before paint so ANSI color
// escapes stay out of the width math (their bytes would otherwise corrupt padding).
func padRight(s string, n int) string {
	if displayWidth(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-displayWidth(s))
}

// wrapWords word-wraps s into lines of at most width characters, breaking at
// spaces. A single word longer than width is placed on its own line (not split).
// Runs of spaces collapse to one. An empty/whitespace-only s yields a single
// empty line so callers can always index [0].
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
