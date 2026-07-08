// Package discover scans the on-disk extensions/ tree and extracts extension
// metadata (PRD §7.1, §7.3). This file (P1.M2.T1.S1) implements the Extension
// data model and its constructors: the Extension struct (the typed record
// resolve/search/check/ui consume), the PackageJSON unmarshal target,
// ParsePackageJSON (read+parse a dir's package.json leniently), BuildExtension
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

// lenientAnySlice is the field type for the []-valued package.json fields
// (keywords, weave.aliases, pi.extensions). It implements json.Unmarshaler so
// that a WRONG-TYPED JSON value (a scalar like "notarray" or 123 where an array
// is expected) is COERCED to nil instead of hard-erroring the entire unmarshal.
//
// This is the load-bearing leniency hook for PRD §7.3 ("a non-array keywords ⇒
// []"). stdlib encoding/json hard-errors the WHOLE struct when a JSON scalar
// meets a typed slice field ([]any included): it returns
// "json: cannot unmarshal string into Go struct field ... of type []interface{}"
// and discards every populated field (including valid ones like pi.extensions).
// By intercepting the decode here and returning nil error for a non-array, the
// parse succeeds and the other fields are preserved.
//
// On success the underlying value is whatever encoding/json produced for the
// array: a []any holding the decoded elements (strings, numbers, bools, ...).
// toStringSlice then normalizes []any → []string, dropping non-string elements
// (so a stray number inside a valid array like ["a", 2, "b"] is dropped too).
// A non-array value yields nil here → toStringSlice → nil (len 0).
type lenientAnySlice []any

// UnmarshalJSON implements json.Unmarshaler. A JSON array decodes normally; a
// JSON null or any NON-array value (string/number/bool/object) coerces to nil
// with NO error (PRD §7.3 leniency). We hand-parse via json.Unmarshal into an
// []any first rather than letting the default decoder drive, so we can swallow
// the type mismatch ourselves.
func (s *lenientAnySlice) UnmarshalJSON(data []byte) error {
	// json.Unmarshal into []any distinguishes "JSON array" from everything else:
	// a scalar/bool/object leaves v as the decoded scalar (not []any) and returns
	// no error; a JSON null leaves v as nil. Only a real array yields []any.
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		// Truly malformed JSON at this value (unterminated, etc.) cannot be
		// coerced — propagate so ParsePackageJSON surfaces it as a parse error
		// (the §9 "unparseable" path). This is distinct from a well-formed but
		// wrong-typed value, which is the lenient case handled below.
		return err
	}
	switch arr := v.(type) {
	case []any:
		*s = arr // a real array — keep the decoded elements
	default:
		*s = nil // null / scalar / object where an array was expected → coerce (PRD §7.3)
	}
	return nil
}

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
//   - single string -> nil (a scalar where an array was expected is dropped —
//     lenientAnySlice already coerced it to nil at decode time; this branch is
//     defensive for callers that build a PackageJSON struct literally)
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
	case lenientAnySlice:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		return out
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
		return nil // scalar-where-array-expected is dropped (PRD §7.3 coerces to [])
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
// (unlike PackageJSON below, which is the unmarshal target). Putting
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

// PackageJSON is the encoding/json unmarshal target for a directory's
// package.json (PRD §10). It carries json tags (unlike Extension, which is
// BUILT, never unmarshaled).
//
// LOAD-BEARING LENIENCY DECISION (PRD §7.3): the array fields (Keywords,
// Weave.Aliases, Pi.Extensions) are typed lenientAnySlice — a custom
// json.Unmarshaler that COERCES a wrong-typed value (a scalar where an array is
// expected) to nil with NO error, instead of hard-erroring the whole unmarshal.
// stdlib encoding/json HARD-ERRORS the ENTIRE parse when a typed []any field
// meets a scalar JSON value (verified: "keywords":"notarray" aborts the parse
// and leaves EVERY field zero, including valid ones like pi.extensions). That
// violates PRD §7.3's "a non-array keywords ⇒ []" mandate. lenientAnySlice
// absorbs the wrong type; BuildExtension then normalizes via toStringSlice
// (drops non-string elements) + comma-ok .(string) asserts (drops non-strings
// to ""). This is the ONE place this package deliberately diverges from
// extdir.hasPiExtensions, which can use []string because a wrong type there
// just means "return false" (acceptable for that predicate).
//
// The scalar fields (Name, Description, Weave.Category) are typed any: a
// wrong-typed value there is absorbed by encoding/json itself and coerced to ""
// by the comma-ok asserts in BuildExtension — no custom unmarshaler needed.
//
// Unknown keys are ignored by encoding/json's default (no
// DisallowUnknownFields) — also a PRD §7.3 requirement. Dependencies is typed
// map[string]string (maps are lenient by default for present keys; BuildExtension
// does not consume it but check M4.T2 may).
type PackageJSON struct {
	Name         any               `json:"name"`
	Description  any               `json:"description"`
	Keywords     lenientAnySlice   `json:"keywords"`
	Pi           piBlock           `json:"pi"`
	Weave        weaveBlock        `json:"weave"`
	Dependencies map[string]string `json:"dependencies"`
}

// piBlock is the namespaced `pi` object in package.json (what pi loads).
// Extensions is lenientAnySlice (coerces a non-array to nil) so a wrong-typed
// pi.extensions does not break discovery of a package extension (PRD §7.3).
// BuildExtension does not consume it — classifyDir reads it via toStringSlice.
type piBlock struct {
	Extensions lenientAnySlice `json:"extensions"`
}

// weaveBlock is the namespaced `weave` catalog object in package.json (read by
// weave only, never by pi). Aliases is lenientAnySlice (coerces a non-array to
// nil; toStringSlice then drops non-strings); Category is any (comma-ok .(string)
// drops non-strings to "").
type weaveBlock struct {
	Aliases  lenientAnySlice `json:"aliases"`
	Category any             `json:"category"`
}

// ParsePackageJSON reads and parses the package.json in dir. It returns a
// 3-valued result that distinguishes the cases the §9 check (M4.T2) needs:
//
//   - (PackageJSON{}, false, nil): NO package.json file (os.ReadFile failed —
//     file does not exist, or any other read error such as a permission
//     failure). Normal for single-file extensions; check does NOT flag this.
//     Collapsing ALL read errors (including non-NotExist) to hasPkg=false keeps
//     the contract clean: hasPkg and err never contradict (a perm error with
//     hasPkg=true,err!=nil would be ambiguous). The §9 check only cares about
//     the "exists but unparseable" case below.
//   - (PackageJSON{}, true, err): package.json EXISTS but is unparseable JSON
//     (truly malformed syntax — wrong-typed VALUES are absorbed by the
//     []any/any typing and do not reach this branch). check (§9 ERROR) reports
//     this. hasPkg=true signals "a package.json was read but unparseable".
//     The raw json.Unmarshal error is returned verbatim (NOT wrapped) so check
//     can inspect/report it.
//   - (pkg, true, nil): success. The parsed fields are populated; unknown keys
//     ignored, wrong-typed array/scalar fields coerced (leniency, PRD §7.3).
//
// The caller (T3 Index) passes hasPkg straight through to BuildExtension.
func ParsePackageJSON(dir string) (pkg PackageJSON, hasPkg bool, err error) {
	data, rerr := os.ReadFile(filepath.Join(dir, "package.json"))
	if rerr != nil {
		// Any read failure (incl. fs.ErrNotExist AND permission errors) → treat
		// as "no package.json". This keeps the 3-valued shape contradiction-free
		// (hasPkg=false always pairs with err=nil here). errors.Is is imported
		// for this branch; if a future revision wants to surface perm errors,
		// branch explicitly on errors.Is(rerr, fs.ErrNotExist).
		_ = errors.Is(rerr, fs.ErrNotExist) // documents intent; current policy collapses all read errors
		return PackageJSON{}, false, nil
	}
	if uerr := json.Unmarshal(data, &pkg); uerr != nil {
		return PackageJSON{}, true, uerr // exists but unparseable → hasPkg=true, err set
	}
	return pkg, true, nil
}

// BuildExtension assembles an Extension from walk-derived location info (path,
// entryFile, relTag, kind — produced by T2's classifier), the parsed package.json
// (pkg from ParsePackageJSON), whether a package.json was read (hasPkg, from
// ParsePackageJSON's second return), and a JSDoc fallback description (jsdocDesc,
// supplied by the caller after S2's ExtractJSDoc runs on the entry file).
//
// It is the boundary between ParsePackageJSON (this subtask, S1) + ExtractJSDoc
// (S2) and the Index walk (T3). T3 calls it once per discovered entry:
//
//	pkg, hasPkg, perr := ParsePackageJSON(dir)
//	// T3 decides how to surface perr (malformed JSON) to check (M4); BuildExtension
//	// itself never errors — it works on any PackageJSON, including PackageJSON{}
//	// (HasPackageJSON=false) from a no-package.json single-file extension.
//	jsdoc := ExtractJSDoc(entryFile) // S2 — this subtask does NOT parse JSDoc
//	e := BuildExtension(path, entryFile, relTag, kind, pkg, hasPkg, jsdoc)
//
// It is TOTAL: no error return, no panic — even when pkg is PackageJSON{} (a
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
// (e.g. "pkg != PackageJSON{}"); the caller already knows from ParsePackageJSON
// whether a file was found, and re-deriving would couple BuildExtension to
// ParsePackageJSON's zero-value representation.
func BuildExtension(path, entryFile, relTag, kind string, pkg PackageJSON, hasPkg bool, jsdocDesc string) Extension {
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
