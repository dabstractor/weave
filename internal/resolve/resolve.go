// Package resolve maps a user-supplied tag to a single extension using the PRD
// §7.2 precedence. It is a PURE function over []discover.Extension: no
// filesystem, no global state, no I/O — it takes the already-built index as a
// parameter and main (P1.M3.T2.S1) supplies it from discover.Index().
//
// The precedence (first match wins; a later step is consulted ONLY if every
// earlier step produced nothing) is, in order:
//
//  1. Canonical — tag == extension.RelTag (case-sensitive). RelTag is unique per
//     entry, so at most one hit.
//  2. Basename  — tag == the final '/'-component of extension.RelTag (e.g.
//     "reddit-poster" matches "writing/reddit-poster"). >1 hit ⇒ *AmbiguousError.
//  3. Name      — tag == extension.Name (the package.json name). >1 ⇒ *AmbiguousError.
//  4. Alias     — tag appears in extension.Aliases (weave.aliases). >1 ⇒ *Ambiguous.
//  5. otherwise — *UnknownError.
//
// An ambiguity at any step SHORT-CIRCUITS: *AmbiguousError is returned immediately
// and later steps are NOT consulted (a looser match cannot rescue an ambiguity;
// the caller must see the candidates per PRD §6.4).
//
// AmbiguousError.Candidates is the SORTED list of the matching extensions'
// RelTags — sorted here so the error is deterministic regardless of how the
// caller ordered the input slice (PRD §6.4 wants stable stderr for scripting).
package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// MatchKind identifies which §7.2 step resolved a tag. Its zero value is not a
// valid success; Resolve always sets it on the success path. Exported so callers
// can switch on it (e.g. --list/debug could annotate "reddit-poster (basename)").
type MatchKind int

const (
	// Canonical means tag == extension.RelTag (step 1, exact canonical tag).
	Canonical MatchKind = iota
	// Basename means tag == the final '/'-component of extension.RelTag (step 2).
	Basename
	// Name means tag == extension.Name, the package.json name (step 3).
	Name
	// Alias means tag appeared in extension.Aliases (step 4).
	Alias
)

// String renders a MatchKind for logs/debug. An out-of-range value renders as
// "unknown".
func (m MatchKind) String() string {
	switch m {
	case Canonical:
		return "canonical"
	case Basename:
		return "basename"
	case Name:
		return "name"
	case Alias:
		return "alias"
	default:
		return "unknown"
	}
}

// Result is the outcome of resolving one tag. The zero value Result{} is NOT a
// valid success: Resolve returns it only together with a non-nil error.
type Result struct {
	Extension discover.Extension
	Match     MatchKind
}

// UnknownError is returned by Resolve when no §7.2 step matched the tag. main
// prints it to stderr and exits 1 (PRD §6.4).
type UnknownError struct {
	Tag string
}

// Error implements error. No prefix (main adds program context).
func (e *UnknownError) Error() string {
	return fmt.Sprintf("unknown extension tag %q", e.Tag)
}

// AmbiguousError is returned when a short tag matched >1 extension at the SAME
// precedence step. Candidates is the sorted list of the matching extensions'
// RelTags (the full canonical tags the user can use to disambiguate). main lists
// them on stderr and exits 1 (PRD §6.4).
type AmbiguousError struct {
	Tag        string
	Candidates []string
}

// Error implements error. Candidates are joined with ", " for a readable line.
func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous extension tag %q matches: %s", e.Tag, strings.Join(e.Candidates, ", "))
}

// Resolve applies the PRD §7.2 precedence to tag against exts and returns the
// single matching extension, or a typed error (*UnknownError / *AmbiguousError).
//
// It is pure: it does not touch the filesystem or mutate exts. It consults each
// precedence step only if every earlier step produced no match. An ambiguity at
// any step returns *AmbiguousError immediately (later steps are NOT consulted).
//
// Field-level gotcha: step 3 (Name) and step 4 (Alias) only consider an
// extension whose relevant field is non-empty. An extension with no package.json
// name (Name=="") is never matched by name, and an empty alias never matches;
// this prevents a degenerate empty tag (or a missing-name extension) from
// spuriously resolving. RelTag and its basename are always non-empty for a real
// extension, so steps 1–2 need no guard.
func Resolve(tag string, exts []discover.Extension) (Result, error) {
	// Step 1 — exact canonical tag. RelTag is unique per entry ⇒ at most one.
	// First (only) match wins; no ambiguity is possible at this step.
	for _, e := range exts {
		if e.RelTag == tag {
			return Result{Extension: e, Match: Canonical}, nil
		}
	}

	// Step 2 — basename (final '/'-component of RelTag).
	if m := collectMatches(exts, func(e discover.Extension) bool {
		return basename(e.RelTag) == tag
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Basename}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}

	// Step 3 — package.json name (skip extensions with no name: a missing name
	// is not a matchable name, and this guards against an empty tag matching
	// Name=="").
	if m := collectMatches(exts, func(e discover.Extension) bool {
		return e.Name != "" && e.Name == tag
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Name}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}

	// Step 4 — declared alias.
	if m := collectMatches(exts, func(e discover.Extension) bool {
		for _, a := range e.Aliases {
			if a == tag {
				return true
			}
		}
		return false
	}); len(m) == 1 {
		return Result{Extension: m[0], Match: Alias}, nil
	} else if len(m) > 1 {
		return Result{}, &AmbiguousError{Tag: tag, Candidates: sortedRelTags(m)}
	}

	// Step 5 — nothing matched.
	return Result{}, &UnknownError{Tag: tag}
}

// collectMatches returns every extension for which pred returns true, in input
// order. It is the shared collection loop for steps 2–4 (step 1 is
// exact-and-unique, so it is inlined in Resolve). Each extension appears at most
// once: pred is a property of the extension, so it is true or false, never
// "twice".
func collectMatches(exts []discover.Extension, pred func(discover.Extension) bool) []discover.Extension {
	var hit []discover.Extension
	for _, e := range exts {
		if pred(e) {
			hit = append(hit, e)
		}
	}
	return hit
}

// basename returns the final '/'-component of a slash-normalized relTag (e.g.
// "writing/reddit-poster" → "reddit-poster"). relTag is always slash-normalized
// by discover (filepath.ToSlash), so splitting on '/' is correct on every
// platform and no OS-separator handling is needed here. A tag with no '/' is its
// own basename. Uses strings.LastIndex (zero-alloc) rather than path.Base to
// stay faithful to the item's "split on /, take last element" and avoid
// importing "path".
func basename(relTag string) string {
	if i := strings.LastIndex(relTag, "/"); i >= 0 {
		return relTag[i+1:]
	}
	return relTag
}

// sortedRelTags returns the RelTags of exts, sorted ascending. Used for
// AmbiguousError.Candidates so the error is deterministic regardless of the
// input slice order (PRD §6.4 wants stable stderr for scripting).
func sortedRelTags(exts []discover.Extension) []string {
	tags := make([]string, len(exts))
	for i, e := range exts {
		tags[i] = e.RelTag
	}
	sort.Strings(tags)
	return tags
}
