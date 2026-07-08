// Package discover scans the on-disk extensions/ tree and extracts extension
// metadata (PRD §7.1, §7.3). This file (P1.M2.T1.S1) implements the Extension
// data model and its constructors: the Extension struct (the typed record
// resolve/search/check/ui consume), the packageJSON unmarshal target,
// parsePackageJSON (read+parse a dir's package.json leniently), BuildExtension
// (assemble an Extension from parsed data + walk-derived location info + a JSDoc
// fallback description), and the toStringSlice []any→[]string normalizer. The
// classify-then-descend walk (T2), Index (T3), and JSDoc extractor (S2) are
// LATER subtasks in this package.
package discover

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// toStringSlice normalizes a metadata value (from an encoding/json []any-typed
// struct field) into []string.
//
// encoding/json unmarshals JSON arrays into []any (when the target field is
// typed []any), NEVER []string. This helper asserts []any → []string so the
// typed Extension fields are convenient for resolve/search. Behavior (verified;
// see research/json_leniency.md):
//   - nil           -> nil
//   - []any         -> []string, with NON-STRING elements silently skipped
//     (lenient: a stray number in `keywords` is dropped, matching
//     the "wrong-typed fields coerced or ignored" leniency of PRD §7.3)
//   - []string      -> returned as-is (defensive; encoding/json never produces this)
//   - single string -> []string{s} (lenient: a string where an array was expected)
//   - anything else -> nil
//
// A present-but-empty array ([]any{}) yields an empty non-nil []string; an
// absent field yields nil. Both have len 0 → callers MUST test with len(), not
// a nil check. (Ported verbatim from skilldozer's skill.go — it is pure and
// source-format-agnostic; only the []any source differs: yaml.v3 there,
// encoding/json here.)
func toStringSlice(v any) []string {
	switch s := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return s
	case string:
		return []string{s}
	default:
		return nil
	}
}

// Extension is a resolved on-disk extension (PRD §7.1). Index() (T3) returns a
// []Extension; resolve.Resolve (M3.T1) matches tags against it; search.Search
// (M4.T1) filters it; check.Check (M4.T2) validates it; ui.PrintList (M2.T4)
// renders it.
//
// It is BUILT by BuildExtension, never unmarshaled, so it carries NO json tags
// (unlike packageJSON below, which is the unmarshal target). Putting
// `json:"..."` tags on Extension is a category error — it is assembled
// field-by-field by BuildExtension.
//
// Field semantics (PRD §7.1 §7.3):
//   - Path:           the resolvable path — the file for single-file extensions,
//     the directory for dir/package extensions. The DEFAULT output (what
//     `weave <tag>` prints).
//   - EntryFile:      the .ts/.js file pi loads — the file itself (single-file),
//     index.ts/index.js (dir), or the first existing pi.extensions entry
//     (package). The --file output.
//   - RelTag:         the canonical tag — the entry path relative to the
//     extensions dir, OS separators normalized to '/', with a trailing
//     .ts/.js stripped for single-file entries (e.g. "writing/reddit-poster").
//     Resolution and --list key off this.
//   - Kind:           "file" | "dir" | "package" (derived by T2's classifier).
//     Aids diagnostics and check.
//   - Name:           package.json name, else "" (resolution fallback #3,
//     --list NAME column).
//   - Description:    package.json description if present, ELSE the leading
//     JSDoc (supplied by the caller via jsdocDesc), ELSE "". Rendered as
//     "(none)" in --list when empty.
//   - Keywords:       package.json keywords ([]string; nil if absent/non-array).
//     Feeds --search.
//   - Category:       package.json weave.category, else "". Feeds --search.
//   - Aliases:        package.json weave.aliases ([]string; nil if absent).
//     Resolution fallback #4.
//   - HasPackageJSON: whether a package.json was read for this entry. Set from
//     the BuildExtension hasPkg argument (NOT re-derived).
//
// Aliases/Keywords nil-vs-empty: an ABSENT field yields nil; a PRESENT-but-empty
// array yields a non-nil empty slice. Both have len 0 → callers MUST test with
// len(), not a nil check (inherited from toStringSlice).
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
	HasPackageJSON bool
}

// packageJSON is the encoding/json unmarshal target for a directory's
// package.json (PRD §10). It carries json tags (unlike Extension, which is
// BUILT, never unmarshaled).
//
// LOAD-BEARING TYPING DECISION (see research/json_leniency.md): the array
// fields (Keywords, Weave.Aliases, Pi.Extensions) are typed []any and the scalar
// fields (Name, Description, Weave.Category) are typed any — NOT []string /
// string. stdlib encoding/json HARD-ERRORS the ENTIRE unmarshal when a typed
// []string/string field meets a wrong-typed JSON value (verified: a string
// "notarray" into a []string Keywords aborts the parse and leaves every field
// zero). That violates PRD §7.3's leniency mandate ("a non-array keywords ⇒
// []"). Typing as []any/any makes json.Unmarshal LENIENT: a wrong-typed value
// is absorbed (mixed ["a",2,"b"] parses with err==nil), and BuildExtension
// normalizes via toStringSlice (drops non-strings) + comma-ok .(string) asserts
// (drops non-strings to ""). This is the ONE place this package deliberately
// diverges from extdir.hasPiExtensions, which can use []string because a wrong
// type there just means "return false" (acceptable for that predicate).
//
// Unknown keys are ignored by encoding/json's default (no
// DisallowUnknownFields) — also a PRD §7.3 requirement. Dependencies is typed
// map[string]string (maps are lenient by default for present keys; BuildExtension
// does not consume it but check M4.T2 may).
type packageJSON struct {
	Name         any               `json:"name"`
	Description  any               `json:"description"`
	Keywords     []any             `json:"keywords"`
	Pi           piBlock           `json:"pi"`
	Weave        weaveBlock        `json:"weave"`
	Dependencies map[string]string `json:"dependencies"`
}

// piBlock is the namespaced `pi` object in package.json (what pi loads).
// Extensions is []any for leniency (though BuildExtension does not consume it —
// T2's classifier uses it via isResolvableDir/hasPiExtensions).
type piBlock struct {
	Extensions []any `json:"extensions"`
}

// weaveBlock is the namespaced `weave` catalog object in package.json (read by
// weave only, never by pi). Aliases is []any (toStringSlice drops non-strings);
// Category is any (comma-ok .(string) drops non-strings to "").
type weaveBlock struct {
	Aliases  []any `json:"aliases"`
	Category any   `json:"category"`
}

// parsePackageJSON reads and parses the package.json in dir. It returns a
// 3-valued result that distinguishes the cases the §9 check (M4.T2) needs:
//
//   - (packageJSON{}, false, nil): NO package.json file (os.ReadFile failed —
//     file does not exist, or any other read error such as a permission
//     failure). Normal for single-file extensions; check does NOT flag this.
//     Collapsing ALL read errors (including non-NotExist) to hasPkg=false keeps
//     the contract clean: hasPkg and err never contradict (a perm error with
//     hasPkg=true,err!=nil would be ambiguous). The §9 check only cares about
//     the "exists but unparseable" case below.
//   - (packageJSON{}, true, err): package.json EXISTS but is unparseable JSON
//     (truly malformed syntax — wrong-typed VALUES are absorbed by the
//     []any/any typing and do not reach this branch). check (§9 ERROR) reports
//     this. hasPkg=true signals "a package.json was read but unparseable".
//     The raw json.Unmarshal error is returned verbatim (NOT wrapped) so check
//     can inspect/report it.
//   - (pkg, true, nil): success. The parsed fields are populated; unknown keys
//     ignored, wrong-typed array/scalar fields coerced (leniency, PRD §7.3).
//
// The caller (T3 Index) passes hasPkg straight through to BuildExtension.
func parsePackageJSON(dir string) (pkg packageJSON, hasPkg bool, err error) {
	data, rerr := os.ReadFile(filepath.Join(dir, "package.json"))
	if rerr != nil {
		// Any read failure (incl. fs.ErrNotExist AND permission errors) → treat
		// as "no package.json". This keeps the 3-valued shape contradiction-free
		// (hasPkg=false always pairs with err=nil here). errors.Is is imported
		// for this branch; if a future revision wants to surface perm errors,
		// branch explicitly on errors.Is(rerr, fs.ErrNotExist).
		_ = errors.Is(rerr, fs.ErrNotExist) // documents intent; current policy collapses all read errors
		return packageJSON{}, false, nil
	}
	if uerr := json.Unmarshal(data, &pkg); uerr != nil {
		return packageJSON{}, true, uerr // exists but unparseable → hasPkg=true, err set
	}
	return pkg, true, nil
}

// BuildExtension assembles an Extension from walk-derived location info (path,
// entryFile, relTag, kind — produced by T2's classifier), the parsed package.json
// (pkg from parsePackageJSON), whether a package.json was read (hasPkg, from
// parsePackageJSON's second return), and a JSDoc fallback description (jsdocDesc,
// supplied by the caller after S2's ExtractJSDoc runs on the entry file).
//
// It is the boundary between parsePackageJSON (this subtask, S1) + ExtractJSDoc
// (S2) and the Index walk (T3). T3 calls it once per discovered entry:
//
//	pkg, hasPkg, perr := parsePackageJSON(dir)
//	// T3 decides how to surface perr (malformed JSON) to check (M4); BuildExtension
//	// itself never errors — it works on any packageJSON, including packageJSON{}
//	// (HasPackageJSON=false) from a no-package.json single-file extension.
//	jsdoc := ExtractJSDoc(entryFile) // S2 — this subtask does NOT parse JSDoc
//	e := BuildExtension(path, entryFile, relTag, kind, pkg, hasPkg, jsdoc)
//
// It is TOTAL: no error return, no panic — even when pkg is packageJSON{} (a
// missing package.json yields empty metadata + HasPackageJSON = hasPkg). The
// comma-ok type assertions on the any-typed scalar fields are nil-safe and
// wrong-typed-safe (a number-valued Description drops to "", not a panic).
//
// Description fallback chain (PRD §7.3): package.json description FIRST, else the
// leading JSDoc (jsdocDesc), else "". This is the load-bearing difference from
// skilldozer's BuildSkill (which copies fm.Description verbatim — YAML is its
// only source); weave has TWO sources with a priority order, and the fallback
// lives here so it is unit-testable in isolation by passing a synthetic jsdocDesc.
//
// HasPackageJSON comes from the hasPkg ARGUMENT verbatim — do NOT re-derive it
// (e.g. "pkg != packageJSON{}"); the caller already knows from parsePackageJSON
// whether a file was found, and re-deriving would couple BuildExtension to
// parsePackageJSON's zero-value representation.
func BuildExtension(path, entryFile, relTag, kind string, pkg packageJSON, hasPkg bool, jsdocDesc string) Extension {
	// Description fallback chain (PRD §7.3): package.json description FIRST,
	// else JSDoc, else "".
	desc, _ := pkg.Description.(string) // comma-ok: non-string/absent → "", false
	if desc == "" {
		desc = jsdocDesc
	}
	name, _ := pkg.Name.(string)               // comma-ok leniency on the any-typed scalar
	category, _ := pkg.Weave.Category.(string) // comma-ok: a number category → ""
	return Extension{
		Path:           path,
		EntryFile:      entryFile,
		RelTag:         relTag,
		Kind:           kind,
		Name:           name,
		Description:    desc,
		Keywords:       toStringSlice(pkg.Keywords), // []any → []string, drops non-strings
		Category:       category,
		Aliases:        toStringSlice(pkg.Weave.Aliases),
		HasPackageJSON: hasPkg, // from the ARGUMENT, not re-derived
	}
}
