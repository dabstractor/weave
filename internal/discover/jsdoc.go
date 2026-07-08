// (No package-level doc comment here: extension.go, P1.M2.T1.S1, owns the
// "Package discover" doc. Go uses the first package comment encountered across
// files; adding a second is ignored by godoc but is poor form.)

package discover

import (
	"bytes"
	"os"
	"strings"
)

// utf8BOM is the 3-byte UTF-8 byte-order mark (U+FEFF), ported verbatim from
// the precedent YAML-frontmatter extractor (the function this package replaces).
// Some editors prepend it to .ts/.js files; it must be stripped before "/**"
// detection, otherwise the opener reads as "\ufeff/**" and the JSDoc block is
// silently missed. bytes.TrimPrefix is a no-op when the BOM is absent, so
// stripping is always safe for BOM-free files.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripStarPrefix is the JSDoc continuation-line cleaner (PRD §7.3.2: "strip an
// optional leading run of whitespace followed by a single `*` (and one optional
// space)"). It strips, in order:
//  1. leading spaces and tabs,
//  2. a single leading "*" if present,
//  3. one optional space immediately following that "*".
//
// This is the per-line convention enforced by Biome's useSingleJsDocAsterisk and
// tslint's jsdoc-format (see research/jsdoc_extraction.md). It does NOT strip a
// trailing "\r"; callers trim CRLF before joining (ExtractJSDoc trims "\r" per
// line up front). A line with no leading "*" (e.g. the first content fragment
// after "/**" on the same line) is returned with only leading whitespace and one
// optional space stripped.
func stripStarPrefix(line string) string {
	s := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(s, "*") {
		s = s[1:]
	}
	if strings.HasPrefix(s, " ") {
		s = s[1:]
	}
	return s
}

// ExtractJSDoc reads the .ts/.js file at path and extracts the LEADING JSDoc
// block ("/** ... */") as a single-line description string (PRD §7.3.2). This is
// the metadata FALLBACK source for single-file extensions: BuildExtension
// (extension.go, S1) prefers a non-empty package.json description, and only when
// that is absent does the JSDoc description win. The sole non-test caller is T3
// Index: `jsdocDesc := ExtractJSDoc(entryFile)`.
//
// Algorithm (PRD §7.3.2, verified against jsdoc.fyi/jsdoc.app in
// research/jsdoc_extraction.md):
//  1. Read the file. On ANY os.ReadFile error, return "".
//  2. Strip a leading UTF-8 BOM if present (no-op when absent).
//  3. Split into lines on "\n"; trim a trailing "\r" from each (CRLF robustness,
//     mirroring the precedent frontmatter extractor's CRLF handling).
//  4. Skip leading blank lines; find the first non-blank line. If the file is
//     entirely blank, return "".
//  5. The first non-blank line must start with exactly "/**" AND must NOT start
//     with "/***" (jsdoc.fyi: 3+ stars are not parsed by JSDoc). A "//" line
//     comment, a "/*" plain block, code, or "/***" → return "".
//  6. Scan lines from the opener for the first line containing "*/". The opener
//     line itself may contain "*/" (single-line block). If no closer is found,
//     return "".
//  7. For each line between "/**" and "*/" (inclusive endpoints trimmed): strip
//     the "/**" opener on the first line and cut at "*/" on the last; clean each
//     line via stripStarPrefix; drop empty lines.
//  8. Join survivors with a single space and TrimSpace the result. Single-line
//     "/** desc */" blocks work as the degenerate case (one content fragment).
//
// ExtractJSDoc returns "" for everything that is not a clean LEADING JSDoc block
// (read error, wrong opener, unclosed). It never panics and never returns an
// error: the signature is `string`, not `(string, error)`, because JSDoc has no
// "malformed content" state worth surfacing — unlike the YAML-frontmatter
// extractor this function replaces, which surfaces a parse error for malformed
// YAML (see the Why section of P1.M2.T1.S2/PRP.md). JSDoc is hand-parsed with
// stdlib, not unmarshaled, so there is no malformed state to report.
func ExtractJSDoc(path string) string {
	// Step 1: read file. Any error → "".
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Step 2: strip leading UTF-8 BOM (no-op when absent).
	data = bytes.TrimPrefix(data, utf8BOM)

	// Step 3: split into lines; trim trailing "\r" per line for CRLF robustness.
	lines := strings.Split(string(data), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r")
	}

	// Step 4: find the first non-blank line.
	startIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return "" // entirely blank file
	}

	// Step 5: the first non-blank line must start with exactly "/**" and NOT
	// "/***" (jsdoc.fyi: 3+ stars are not parsed by JSDoc). HasPrefix(line,"/**")
	// alone would wrongly accept "/***" since "/***" starts with "/**".
	first := lines[startIdx]
	if !strings.HasPrefix(first, "/**") || strings.HasPrefix(first, "/***") {
		return ""
	}

	// Step 6: find the closing "*/". Because step 5 guarantees the first
	// non-blank line is "/**", the first "*/" found must be the JSDoc closer —
	// no code precedes the opener, so no string-literal "*/" false positive can
	// occur before it. The opener line itself may contain "*/" (single-line).
	closeIdx := -1
	for i := startIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "*/") {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return "" // unclosed JSDoc
	}

	// Steps 7-8: collect cleaned content lines between "/**" and "*/".
	var parts []string
	for i := startIdx; i <= closeIdx; i++ {
		line := lines[i]
		switch {
		case i == startIdx && i == closeIdx:
			// Single-line block: find "*/" on the ORIGINAL line BEFORE extracting
			// content, so the opener/closer overlap in degenerate blocks (e.g. "/**/"
			// has "*/" at index 2, within the opener's first 3 chars) is handled. For
			// "/**/" the closer sits inside the opener "/**" -> empty content.
			c := strings.Index(line, "*/")
			if c <= 3 {
				// "*/" starts at or before the opener "/**" ends (index 3 exclusive)
				// -> opener and closer overlap -> empty content.
				line = ""
			} else {
				line = line[3:c] // content strictly between "/**" and "*/"
			}
		case i == startIdx:
			// Opener line of a multi-line block: drop the "/**" prefix.
			line = strings.TrimPrefix(line, "/**")
		case i == closeIdx:
			// Closer line: cut at the first "*/".
			if c := strings.Index(line, "*/"); c >= 0 {
				line = line[:c]
			}
		}
		// Middle lines are unchanged (full line). Clean and drop empties.
		if cleaned := strings.TrimSpace(stripStarPrefix(line)); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}

	// Join survivors with a single space; trim the final result.
	return strings.TrimSpace(strings.Join(parts, " "))
}
