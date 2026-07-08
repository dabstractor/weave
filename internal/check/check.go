// Package check validates every extension in the store against the PRD §9 rules
// and returns a structured Report. It is a FUNCTION over the extensions directory
// plus the pre-sorted catalog ([]discover.Extension from discover.Index) that
// returns a Report; main's check dispatch (P1.M4.T3.S1) renders the report to
// stdout (PRD §9 output format) and maps Report.HasErrors() to the exit code.
//
// It ports the TYPE STRUCTURE from skilldozer's internal/check (Severity/String,
// Finding, SkillReport→ExtensionReport, Report, HasErrors — see architecture
// mapping §7) but REWRITES the validation logic, because weave's §9 rules
// (relTag duplicates, entryFile existence, package.json validity, deps-installed,
// description presence, empty category folders) are entirely different from
// skilldozer's Agent-Skills frontmatter/name rules. None of skilldozer's
// checkFields/appendDupFindings/validName/nameLenMax/descLenMax are ported.
//
// It mirrors the package layout of internal/resolve and internal/search (a
// function over []discover.Extension in its own internal/ package with its own
// _test.go): the validation concern stays isolated, independently unit-testable,
// and out of the thin main dispatcher.
//
// The non-obvious parts:
//
//   - The dir PARAMETER. Check(dir, exts) takes the absolute extensions
//     directory IN ADDITION to []Extension, because the §9 "empty category
//     folder" rule (a top-level non-entry directory with zero discoverable
//     entries at any depth) CANNOT be derived from []Extension alone: discover.
//     Index prunes resolvable subtrees and emits only Extension records, so an
//     empty top-level folder is invisible to exts. The dir walk recovers it.
//
//   - The re-parse. discover.Index DROPS the package.json parse error (a
//     malformed package.json still builds a resolvable entry via dir
//     classification, lenient discovery). check therefore RE-RUNS discover.
//     ParsePackageJSON on each entry's package dir to recover the
//     unparseable-vs-missing distinction — the exact re-parse the
//     extension.go doc comment already documents as check's responsibility.
//     The double parse is cheap (small files, small store) and idempotent.
package check

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dabstractor/weave/internal/discover"
)

// Severity ranks a finding. OK < WARN < ERROR. Exported so main (M4.T3.S1) can
// switch on it when rendering. OK is the implicit value for an entry with no
// findings (never carried by a Finding).
type Severity int

const (
	LevelOK Severity = iota
	LevelWarn
	LevelError
)

// String renders a Severity as the status word main left-pads ("OK", "WARN",
// "ERROR"). Ports verbatim from skilldozer (with the same default→"OK" safety for
// an out-of-range value).
func (s Severity) String() string {
	switch s {
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "OK"
	}
}

// Finding is one validation result line for a single extension. An entry with
// zero findings is OK (main emits one "OK" line); an entry with N findings emits
// N ERROR/WARN lines. Message is empty for OK (OK findings are never created).
type Finding struct {
	Level   Severity
	Message string
}

// ExtensionReport binds an extension to its findings. ByExt is in input order
// (discover.Index sorts by RelTag), so the report is deterministic. Ports
// skilldozer's SkillReport; the field Skill → Extension.
type ExtensionReport struct {
	Extension discover.Extension
	Findings  []Finding // empty => the extension is OK
}

// Report is the full check output. ByExt is in input order; Errors/Warnings are
// the totals across all findings (drive the summary line + exit code). Ports
// skilldozer's Report (field BySkill → ByExt).
type Report struct {
	ByExt    []ExtensionReport
	Errors   int
	Warnings int
}

// HasErrors reports whether any ERROR finding exists. main maps this to the exit
// code (PRD §9: exit 1 if any ERROR). WARNs never affect it.
func (r Report) HasErrors() bool { return r.Errors > 0 }

// Check validates every extension in exts against the PRD §9 rules, plus the
// empty-category-folder rule derived from a walk over dir, and returns a
// structured Report. It is the P1.M4.T2.S1 deliverable; main's check dispatch
// (M4.T3.S1) renders it and maps HasErrors() to the exit code.
//
// Algorithm (three passes):
//
//  1. PER-EXTENSION local checks: re-parse the entry's package.json (recover
//     unparseable-vs-missing via discover.ParsePackageJSON), stat the entry
//     file, check deps-without-node_modules (package extensions only), and
//     check description presence. Append one Finding per failing rule.
//  2. GLOBAL checks: a duplicate-relTag scan (any canonical tag owned by ≥2
//     entries yields one ERROR per owner, naming the other tag(s) sorted), and
//     the empty-category-folder walk over dir (every top-level subdir that
//     yields zero discover.Index entries at any depth → one WARN, stored on a
//     synthetic ExtensionReport so main renders it uniformly with the others).
//  3. TALLY errors/warnings across all findings.
//
// dir is the ABSOLUTE extensions directory (the value extdir.Find returned and
// discover.Index walked). It is required for Pass 2's empty-folder walk; an
// empty dir ("") yields no empty-folder findings (os.ReadDir on "" errors and is
// skipped), so Check("", exts) is safe.
//
// check does NOT print anything. It returns a Report; main renders it per the
// §9 output format.
func Check(dir string, exts []discover.Extension) Report {
	var rep Report
	rep.ByExt = make([]ExtensionReport, len(exts))

	// Pass 1: per-extension local checks.
	for i := range exts {
		ext := exts[i]
		rep.ByExt[i] = ExtensionReport{
			Extension: ext,
			Findings:  localFindings(ext),
		}
	}

	// Pass 2: global checks.
	appendDuplicateRelTagFindings(&rep)
	appendEmptyFolderFindings(&rep, dir)

	// Pass 3: tally.
	for i := range rep.ByExt {
		for _, f := range rep.ByExt[i].Findings {
			switch f.Level {
			case LevelError:
				rep.Errors++
			case LevelWarn:
				rep.Warnings++
			}
		}
	}
	return rep
}

// localFindings runs the four per-extension §9 checks and returns one Finding
// per failing rule. Order (matches PRD §9 listing): unparseable package.json →
// missing entryFile → deps-without-node_modules (package kind only) → no
// description. An OK extension returns nil (no findings).
func localFindings(ext discover.Extension) []Finding {
	var findings []Finding

	// Re-parse the package.json of the entry's package dir to recover the
	// unparseable-vs-missing distinction discover.Index collapses (lenient
	// discovery ignores parsePackageJSON's err; check surfaces it per §9).
	pkg, hasPkg, perr := discover.ParsePackageJSON(packageDir(ext))
	if hasPkg && perr != nil {
		// package.json EXISTS but is unparseable JSON → ERROR. The raw err is
		// NOT surfaced (the message is fixed per the item description); perr!=nil
		// is only the trigger.
		findings = append(findings, Finding{LevelError, "package.json is not valid JSON"})
	}

	// Entry file must exist on disk (defensive per §9; should not happen given
	// §7.1 classification, but catches hand-edited pi.extensions).
	if _, err := os.Stat(ext.EntryFile); err != nil {
		findings = append(findings, Finding{
			Level:   LevelError,
			Message: "entry file does not exist: " + ext.EntryFile,
		})
	}

	// deps-without-node_modules WARN — PACKAGE extensions only (PRD §9: "a
	// PACKAGE extension's package.json declares non-empty dependencies"). Fires
	// only when the package.json parsed (hasPkg && perr == nil) AND it declares
	// ≥1 dependency AND node_modules/ is absent from the package dir.
	if ext.Kind == "package" && hasPkg && perr == nil && len(pkg.Dependencies) > 0 && !nodeModulesPresent(ext) {
		findings = append(findings, Finding{LevelWarn, "dependencies declared but node_modules/ not found"})
	}

	// No description (neither package.json description nor leading JSDoc) → WARN.
	// BuildExtension already folded the priority chain into ext.Description.
	if ext.Description == "" {
		findings = append(findings, Finding{LevelWarn, "no description (neither package.json description nor leading JSDoc)"})
	}

	return findings
}

// packageDir returns the directory whose package.json discover parsed for ext.
// For dir/package kinds, ext.Path IS the dir (package.json sits IN it). For the
// file kind, ext.Path is the file itself, so package.json sits BESIDE it
// (filepath.Dir). Do NOT use filepath.Dir(ext.EntryFile) for package kinds:
// EntryFile is ext.Path/src/index.ts, so Dir(EntryFile) != ext.Path.
func packageDir(ext discover.Extension) string {
	if ext.Kind == "file" {
		return filepath.Dir(ext.Path)
	}
	return ext.Path
}

// nodeModulesPresent reports whether ext.Path/node_modules is a directory. A
// stray file named "node_modules" must NOT satisfy the check (hence IsDir).
func nodeModulesPresent(ext discover.Extension) bool {
	info, err := os.Stat(filepath.Join(ext.Path, "node_modules"))
	return err == nil && info.IsDir()
}

// appendDuplicateRelTagFindings adds a duplicate-relTag ERROR to every entry
// that shares its canonical RelTag with at least one other entry (PRD §9).
//
// In practice discover.Index prunes exact duplicate file/dir collisions, but two
// paths can still strip to the same RelTag on case-insensitive filesystems
// (foo.ts AND foo/index.ts, or two bar/ dirs); this surfaces that ambiguity. The
// "also in" list names every OTHER owner's tag (a duplicate relTag is its own
// tag, so the others list is the full owner list minus the current entry) and is
// sorted for deterministic output.
//
// It mutates rep.ByExt in place by index (the owners list carries indices so the
// match is unambiguous even when two entries share a RelTag string).
func appendDuplicateRelTagFindings(rep *Report) {
	// owners maps relTag → list of ByExt indices owning it.
	owners := map[string][]int{}
	for i := range rep.ByExt {
		tag := rep.ByExt[i].Extension.RelTag
		owners[tag] = append(owners[tag], i)
	}

	for tag, idxs := range owners {
		if len(idxs) < 2 {
			continue
		}
		// Sort the owner tags for a deterministic "also in" list. Each owner's
		// tag is `tag` itself, so the others list excludes the owner's own index.
		others := make([]string, 0, len(idxs)-1)
		for _, i := range idxs {
			others = append(others, rep.ByExt[i].Extension.RelTag)
		}
		sort.Strings(others)
		for _, i := range idxs {
			// Build this owner's "also in" list: every tag in the sorted set
			// EXCEPT occurrences belonging to this owner. Since duplicate tags
			// are identical strings, exclude exactly one occurrence (the owner's).
			also := make([]string, 0, len(others)-1)
			skipped := false
			for _, t := range others {
				if !skipped && t == rep.ByExt[i].Extension.RelTag {
					skipped = true // drop one occurrence of this owner's own tag
					continue
				}
				also = append(also, t)
			}
			rep.ByExt[i].Findings = append(rep.ByExt[i].Findings, Finding{
				Level: LevelError,
				Message: fmt.Sprintf(
					"duplicate relTag %q (also in: %s)",
					tag, strings.Join(also, ", ")),
			})
		}
	}
}

// appendEmptyFolderFindings walks the top-level children of dir and appends a
// WARN for each plain category folder that contains ZERO discoverable entries at
// any depth (PRD §9). A top-level subdir that IS a resolvable extension yields
// ≥1 discover.Index entry and is therefore NOT flagged — discover.Index is
// reused here so the resolvability semantics stay identical to Index's
// classify-then-descend rule (no duplicated logic).
//
// Each empty-folder finding is appended to rep.ByExt as a synthetic
// ExtensionReport whose Extension is zero-valued with RelTag set to the folder's
// basename, so main (M4.T3.S1) renders it uniformly with the per-extension
// findings. A dir argument of "" (or any unreadable dir) yields no findings
// (os.ReadDir errors and the loop is skipped).
func appendEmptyFolderFindings(rep *Report, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // unreadable/empty dir argument → nothing to walk
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue // only directories can be empty category folders
		}
		child := filepath.Join(dir, e.Name())
		sub, ierr := discover.Index(child)
		if ierr != nil {
			continue // an unreadable subdir is not "empty" (it errored); skip
		}
		if len(sub) == 0 {
			rep.ByExt = append(rep.ByExt, ExtensionReport{
				Extension: discover.Extension{RelTag: e.Name()},
				Findings: []Finding{{
					Level:   LevelWarn,
					Message: "empty category folder: " + e.Name(),
				}},
			})
		}
	}
}
