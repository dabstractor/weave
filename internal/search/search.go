// Package search filters a []discover.Extension by a case-insensitive substring
// query over the six fields PRD §6.1 enriches for `weave --search`: the canonical
// tag, the package.json name, the description, each keyword, each alias, and the
// weave.category. It is a PURE function over []discover.Extension: no filesystem,
// no globals, no I/O — main (P1.M4.T3.S1) supplies the index from discover.Index
// and passes the filtered (still-sorted) slice to ui.PrintList for the "same
// table format as --list" rendering (PRD §6.1).
//
// It mirrors internal/resolve (also a pure matching function over
// []discover.Extension, in its own package with its own _test.go) so the two
// matching concerns — precise tag resolution (resolve) and fuzzy catalog search
// (search) — stay isolated, independently unit-testable, and out of the thin
// main dispatcher.
package search

import (
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// Search returns every extension in exts for which query is a case-insensitive
// substring of ANY of six fields: RelTag (the canonical tag), Name (package.json
// name), Description (package.json description OR the leading JSDoc, already
// folded into Extension.Description by BuildExtension), any Keyword, any Alias,
// or Category (weave.category) (PRD §6.1, §7.1, §7.3). Input order is preserved:
// discover.Index sorts []Extension by RelTag, and ui.PrintList does NOT re-sort,
// so the filtered slice is displayed already-sorted.
//
// An empty query matches EVERY extension: strings.Contains(hay, "") is always
// true, so `weave --search ""` behaves like `weave --list` (exit 1 only if the
// store is empty). This is the natural substring semantics; the PRD carves out
// no special case for an empty query.
//
// An extension with no package.json (HasPackageJSON==false) has Name=="" and
// Description=="" and nil Keywords, but its RelTag is always present — so it is
// still discoverable by searching its tag, matching how resolve lets a
// metadata-less extension resolve by basename (PRD §7.1). Only RelTag is
// searchable for such an extension.
func Search(query string, exts []discover.Extension) []discover.Extension {
	q := strings.ToLower(query) // lowercase the query ONCE, not per field
	var matched []discover.Extension
	for _, e := range exts {
		if matches(q, e) {
			matched = append(matched, e)
		}
	}
	return matched
}

// matches reports whether the already-lowercased query q is a case-insensitive
// substring of any searchable field of e. q is lowercased once by the caller
// (Search); each field is lowercased lazily inside Contains.
//
// Field scope is SIX fields: RelTag, Name, Description, each Keyword, each
// Alias, and Category. PRD §6.1/§7.3 state --search covers tag, package.json
// name/description/keywords, the leading-JSDoc description, weave.aliases, and
// weave.category — so aliases and category ARE searched. This makes --search
// consistent with resolve, which resolves by alias (§7.2 step 4). Aliases are
// matched INDIVIDUALLY (see the Keywords note below) for the same
// boundary-safety reason.
//
// Keywords are tested INDIVIDUALLY (not strings.Join'd): a query spanning a
// boundary between two keywords must not match (joining would create false
// positives like "wri"+"ocial" => "wriocial" across "writing","social").
func matches(q string, e discover.Extension) bool {
	if strings.Contains(strings.ToLower(e.RelTag), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), q) {
		return true
	}
	for _, kw := range e.Keywords {
		if strings.Contains(strings.ToLower(kw), q) {
			return true
		}
	}
	// Aliases (weave.aliases) — matched INDIVIDUALLY, same boundary-safety
	// as Keywords: a query spanning two aliases must not match. PRD §6.1/§7.3
	// say aliases feed --search; this also makes --search consistent with
	// resolve, which resolves by alias (§7.2 step 4).
	for _, a := range e.Aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	// Category (weave.category) — a single scalar field (PRD §6.1/§7.3).
	if strings.Contains(strings.ToLower(e.Category), q) {
		return true
	}
	return false
}
