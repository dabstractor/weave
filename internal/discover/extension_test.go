package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writePackageJSON writes content to dir/package.json for tests; the parse path
// under test is ParsePackageJSON(dir). It is the weave analog of skilldozer's
// writeSkill (defined in discover_test.go there); it is defined HERE because no
// sibling test file exists yet in this package. MkdirAll ensures dir exists.
func writePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile package.json in %q: %v", dir, err)
	}
}

// strEq compares two string slices by length + elements (nil and []string{} are
// both "empty" here; callers that care about nil-vs-empty assert len() directly).
// Ported verbatim from skilldozer's skill_test.go.
func strEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- toStringSlice (table-driven; ported verbatim from skilldozer) ---

func TestToStringSlice(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"any-slice-of-strings", []any{"a", "b"}, []string{"a", "b"}},
		{"any-slice-skips-non-strings", []any{"a", 2, "b", 3.14, true}, []string{"a", "b"}},
		{"any-slice-empty", []any{}, []string{}}, // present-but-empty -> non-nil empty (len 0)
		{"string-slice-passthrough", []string{"x", "y"}, []string{"x", "y"}},
		// PRD §7.3: a non-array value where an array is expected coerces to []
		// ("a non-array keywords ⇒ []"). A scalar string is therefore dropped,
		// NOT wrapped into [s]. lenientAnySlice already does this at decode time;
		// toStringSlice mirrors it for literal struct construction.
		{"single-string-dropped", "solo", nil},
		{"int", 42, nil},
		{"map", map[string]any{"a": 1}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toStringSlice(c.in)
			// Compare by length + element (the meaningful contract). This treats
			// nil and []string{} as equal (both len 0), matching the documented
			// "callers use len()" rule rather than pinning nil-vs-empty.
			if len(got) != len(c.want) {
				t.Fatalf("%s: len(got)=%d; want %d (got=%#v want=%#v)", c.name, len(got), len(c.want), got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("%s: got[%d]=%q; want %q", c.name, i, got[i], c.want[i])
				}
			}
		})
	}
}

// --- ParsePackageJSON ---

// TestParsePackageJSONSuccess: a full PRD-§10 package.json round-trips through
// ParsePackageJSON with every field populated and hasPkg=true, err=nil.
func TestParsePackageJSONSuccess(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
	  "name": "@my-org/summarizer",
	  "description": "Summarize conversation history your way.",
	  "keywords": ["compaction", "summary", "context"],
	  "pi": { "extensions": ["./src/index.ts"] },
	  "weave": {
	    "aliases": ["summarise", "compact-helper"],
	    "category": "context"
	  },
	  "dependencies": { "zod": "^3.0.0" }
	}`)

	pkg, hasPkg, err := ParsePackageJSON(dir)
	if err != nil {
		t.Fatalf("ParsePackageJSON err=%v; want nil", err)
	}
	if !hasPkg {
		t.Fatal("hasPkg=false; want true (package.json present and valid)")
	}
	if name, _ := pkg.Name.(string); name != "@my-org/summarizer" {
		t.Errorf("Name=%q; want @my-org/summarizer", name)
	}
	if desc, _ := pkg.Description.(string); desc != "Summarize conversation history your way." {
		t.Errorf("Description=%q; want the full description", desc)
	}
	// Keywords is []any — normalize for comparison.
	if !strEq(toStringSlice(pkg.Keywords), []string{"compaction", "summary", "context"}) {
		t.Errorf("Keywords=%v; want [compaction summary context]", pkg.Keywords)
	}
	if !strEq(toStringSlice(pkg.Weave.Aliases), []string{"summarise", "compact-helper"}) {
		t.Errorf("Weave.Aliases=%v; want [summarise compact-helper]", pkg.Weave.Aliases)
	}
	if cat, _ := pkg.Weave.Category.(string); cat != "context" {
		t.Errorf("Weave.Category=%q; want context", cat)
	}
	if !strEq(toStringSlice(pkg.Pi.Extensions), []string{"./src/index.ts"}) {
		t.Errorf("Pi.Extensions=%v; want [./src/index.ts]", pkg.Pi.Extensions)
	}
	if pkg.Dependencies["zod"] != "^3.0.0" {
		t.Errorf("Dependencies[zod]=%q; want ^3.0.0", pkg.Dependencies["zod"])
	}
}

// TestParsePackageJSONMissing: a dir with NO package.json yields the
// (PackageJSON{}, false, nil) 3-valued shape — a missing file is NOT an error.
func TestParsePackageJSONMissing(t *testing.T) {
	dir := t.TempDir() // exists, but empty — no package.json
	pkg, hasPkg, err := ParsePackageJSON(dir)
	if err != nil {
		t.Fatalf("err=%v; want nil (missing file is not an error)", err)
	}
	if hasPkg {
		t.Error("hasPkg=true; want false (no package.json present)")
	}
	if !reflect.DeepEqual(pkg, PackageJSON{}) {
		t.Errorf("pkg=%#v; want PackageJSON{} (zero value)", pkg)
	}
}

// TestParsePackageJSONMalformed: a package.json that exists but is invalid JSON
// yields (PackageJSON{}, true, parseError) — hasPkg=true signals "we tried and
// failed". check (§9 ERROR) consumes this. err is asserted non-nil but NOT
// type-pinned (json errors are an implementation detail).
func TestParsePackageJSONMalformed(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "x"`) // truncated, invalid JSON
	pkg, hasPkg, err := ParsePackageJSON(dir)
	if err == nil {
		t.Fatal("err=nil; want a parse error (truncated JSON)")
	}
	if !hasPkg {
		t.Error("hasPkg=false; want true (package.json exists but unparseable)")
	}
	if !reflect.DeepEqual(pkg, PackageJSON{}) {
		t.Errorf("pkg=%#v; want PackageJSON{} (unmarshal failed before any field set)", pkg)
	}
}

// TestParsePackageJSONUnknownKeysIgnored: encoding/json's default ignores
// unknown keys (no DisallowUnknownFields). "bogus" is simply absent.
func TestParsePackageJSONUnknownKeysIgnored(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"x","bogus":1,"weave":{"aliases":["a","b"]}}`)
	pkg, hasPkg, err := ParsePackageJSON(dir)
	if err != nil {
		t.Fatalf("err=%v; want nil (unknown keys must NOT error)", err)
	}
	if !hasPkg {
		t.Error("hasPkg=false; want true")
	}
	if name, _ := pkg.Name.(string); name != "x" {
		t.Errorf("Name=%q; want x", name)
	}
	if !strEq(toStringSlice(pkg.Weave.Aliases), []string{"a", "b"}) {
		t.Errorf("Weave.Aliases=%v; want [a b]", pkg.Weave.Aliases)
	}
	// "bogus" has no field to check — it is silently ignored by the struct tags.
}

// TestParsePackageJSONLenientWrongTypes — THE CRITICAL LENIENCY TEST. Every
// array/scalar field is WRONG-TYPED. PRD §7.3 demands this parse with err==nil
// (coerced, not errored). A typed []string/string struct would FAIL here; the
// []any/any typing makes it pass. If this test fails, a PackageJSON field is
// typed []string/string and must be changed to []any/any.
func TestParsePackageJSONLenientWrongTypes(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
	  "name": 123,
	  "description": ["not", "a", "string"],
	  "keywords": "notarray",
	  "weave": {
	    "aliases": ["a", 2, "b"],
	    "category": 456
	  }
	}`)
	pkg, hasPkg, err := ParsePackageJSON(dir)
	// LOAD-BEARING ASSERTION: err==nil. A typed []string/string struct would
	// return "json: cannot unmarshal X into Go struct field ... of type Y".
	if err != nil {
		t.Fatalf("err=%v; want nil (PRD §7.3: wrong-typed fields COERCED, not errored)", err)
	}
	if !hasPkg {
		t.Error("hasPkg=false; want true (parse succeeded)")
	}

	// name is a number (any) → comma-ok .(string) in BuildExtension yields "".
	if _, ok := pkg.Name.(string); ok {
		t.Errorf("pkg.Name=%#v; expected non-string (number), which BuildExtension drops to \"\"", pkg.Name)
	}

	// keywords is a JSON string "notarray" into a []any field. PRD §7.3 mandates
	// "a non-array keywords ⇒ []". Whatever encoding/json places in pkg.Keywords
	// for a scalar value, the assertion is on the NORMALIZED output: len 0.
	// (Verified empirically: a JSON scalar into []any does not error the parse;
	// toStringSlice then yields len 0 via its default branch, since encoding/json
	// does not produce a Go []any from a JSON scalar.)
	kw := toStringSlice(pkg.Keywords)
	if len(kw) != 0 {
		t.Errorf("toStringSlice(Keywords)=%v; want len 0 (PRD §7.3: non-array keywords ⇒ [])", kw)
	}

	// aliases is mixed ["a", 2, "b"] → toStringSlice drops the number → ["a","b"].
	if got := toStringSlice(pkg.Weave.Aliases); !strEq(got, []string{"a", "b"}) {
		t.Errorf("toStringSlice(Weave.Aliases)=%v; want [a b] (non-string 2 dropped)", got)
	}

	// category is a number (any) → comma-ok .(string) yields "".
	if _, ok := pkg.Weave.Category.(string); ok {
		t.Errorf("pkg.Weave.Category=%#v; expected non-string (number), dropped to \"\" by BuildExtension", pkg.Weave.Category)
	}
}

// --- BuildExtension ---

// TestBuildExtensionFull: all fields populated, hasPkg=true, jsdocDesc="".
// Mirrors skilldozer TestBuildSkillFull's assertion style.
func TestBuildExtensionFull(t *testing.T) {
	pkg := PackageJSON{
		Name:        any("example"),
		Description: any("An example."),
		Keywords:    []any{"example", "demo", "weave"},
		Weave: weaveBlock{
			Aliases:  []any{"ex", "demo-ext"},
			Category: any("meta"),
		},
	}
	e := BuildExtension("/store/example", "/store/example/index.ts", "example", "dir", pkg, true, "")
	if e.Path != "/store/example" {
		t.Errorf("Path=%q; want /store/example", e.Path)
	}
	if e.EntryFile != "/store/example/index.ts" {
		t.Errorf("EntryFile=%q; want /store/example/index.ts", e.EntryFile)
	}
	if e.RelTag != "example" {
		t.Errorf("RelTag=%q; want example", e.RelTag)
	}
	if e.Kind != "dir" {
		t.Errorf("Kind=%q; want dir", e.Kind)
	}
	if e.Name != "example" {
		t.Errorf("Name=%q; want example", e.Name)
	}
	if e.Description != "An example." {
		t.Errorf("Description=%q; want 'An example.'", e.Description)
	}
	if !strEq(e.Keywords, []string{"example", "demo", "weave"}) {
		t.Errorf("Keywords=%v; want [example demo weave]", e.Keywords)
	}
	if e.Category != "meta" {
		t.Errorf("Category=%q; want meta", e.Category)
	}
	if !strEq(e.Aliases, []string{"ex", "demo-ext"}) {
		t.Errorf("Aliases=%v; want [ex demo-ext]", e.Aliases)
	}
	if !e.HasPackageJSON {
		t.Error("HasPackageJSON=false; want true")
	}
}

// TestBuildExtensionNoPackageJSON: pkg=PackageJSON{}, hasPkg=false, jsdocDesc="".
// BuildExtension is TOTAL — no panic, empty metadata, but location fields still
// set from args (a single-file extension with no package.json still resolves by tag).
func TestBuildExtensionNoPackageJSON(t *testing.T) {
	e := BuildExtension("/store/gate.ts", "/store/gate.ts", "gate", "file", PackageJSON{}, false, "")
	if e.Path != "/store/gate.ts" {
		t.Errorf("Path=%q; want /store/gate.ts", e.Path)
	}
	if e.EntryFile != "/store/gate.ts" {
		t.Errorf("EntryFile=%q; want /store/gate.ts", e.EntryFile)
	}
	if e.RelTag != "gate" {
		t.Errorf("RelTag=%q; want gate", e.RelTag)
	}
	if e.Kind != "file" {
		t.Errorf("Kind=%q; want file", e.Kind)
	}
	if e.Name != "" || e.Description != "" || e.Category != "" {
		t.Errorf("Name=%q Description=%q Category=%q; want all empty", e.Name, e.Description, e.Category)
	}
	if len(e.Keywords) != 0 || len(e.Aliases) != 0 {
		t.Errorf("Keywords=%v Aliases=%v; want len 0", e.Keywords, e.Aliases)
	}
	if e.HasPackageJSON {
		t.Error("HasPackageJSON=true; want false")
	}
}

// TestBuildExtensionDescriptionFallback: PRD §7.3 priority order —
// package.json description FIRST, else jsdocDesc, else "".
func TestBuildExtensionDescriptionFallback(t *testing.T) {
	cases := []struct {
		name      string
		pkgDesc   any // package.json description (any-typed field)
		jsdocDesc string
		want      string
	}{
		{"pkg wins", "from pkg", "from jsdoc", "from pkg"},
		{"jsdoc when pkg absent", nil, "from jsdoc", "from jsdoc"},
		{"empty when both absent", nil, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := PackageJSON{Description: c.pkgDesc}
			e := BuildExtension("/p", "/p/index.ts", "t", "dir", pkg, true, c.jsdocDesc)
			if e.Description != c.want {
				t.Errorf("Description=%q; want %q (pkgDesc=%#v jsdoc=%q)", e.Description, c.want, c.pkgDesc, c.jsdocDesc)
			}
		})
	}
}

// TestBuildExtensionHasPackageJSONFromArg: pass hasPkg=true with pkg=PackageJSON{}
// and assert HasPackageJSON==true. Proves HasPackageJSON is the ARGUMENT, not
// re-derived from pkg's contents (the inverse of TestBuildExtensionNoPackageJSON).
func TestBuildExtensionHasPackageJSONFromArg(t *testing.T) {
	e := BuildExtension("/p", "/p/index.ts", "t", "package", PackageJSON{}, true, "")
	if !e.HasPackageJSON {
		t.Error("HasPackageJSON=false; want true (must come from the hasPkg arg, not pkg contents)")
	}
}

// TestBuildExtensionEndToEnd: write a real PRD-§10 package.json, parse it, then
// BuildExtension. Proves ParsePackageJSON → BuildExtension round-trip against a
// real file (mirrors skilldozer TestBuildSkillEndToEnd: ParseFrontmatter→BuildSkill).
func TestBuildExtensionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
	  "name": "@my-org/summarizer",
	  "description": "Summarize conversation history your way.",
	  "keywords": ["compaction", "summary", "context"],
	  "pi": { "extensions": ["./src/index.ts"] },
	  "weave": {
	    "aliases": ["summarise", "compact-helper"],
	    "category": "context"
	  },
	  "dependencies": { "zod": "^3.0.0" }
	}`)

	pkg, hasPkg, err := ParsePackageJSON(dir)
	if err != nil {
		t.Fatalf("ParsePackageJSON: %v", err)
	}
	if !hasPkg {
		t.Fatal("hasPkg=false; want true")
	}

	entryFile := filepath.Join(dir, "src", "index.ts")
	e := BuildExtension(dir, entryFile, "summarizer", "package", pkg, hasPkg, "fallback jsdoc")

	if e.Path != dir {
		t.Errorf("Path=%q; want %q", e.Path, dir)
	}
	if e.EntryFile != entryFile {
		t.Errorf("EntryFile=%q; want %q", e.EntryFile, entryFile)
	}
	if e.RelTag != "summarizer" {
		t.Errorf("RelTag=%q; want summarizer", e.RelTag)
	}
	if e.Kind != "package" {
		t.Errorf("Kind=%q; want package", e.Kind)
	}
	if e.Name != "@my-org/summarizer" {
		t.Errorf("Name=%q; want @my-org/summarizer", e.Name)
	}
	// pkg.Description is non-empty → it wins over "fallback jsdoc".
	if e.Description != "Summarize conversation history your way." {
		t.Errorf("Description=%q; want the package.json description (pkg wins over jsdoc)", e.Description)
	}
	if !strEq(e.Keywords, []string{"compaction", "summary", "context"}) {
		t.Errorf("Keywords=%v; want [compaction summary context]", e.Keywords)
	}
	if e.Category != "context" {
		t.Errorf("Category=%q; want context", e.Category)
	}
	if !strEq(e.Aliases, []string{"summarise", "compact-helper"}) {
		t.Errorf("Aliases=%v; want [summarise compact-helper]", e.Aliases)
	}
	if !e.HasPackageJSON {
		t.Error("HasPackageJSON=false; want true")
	}
}
