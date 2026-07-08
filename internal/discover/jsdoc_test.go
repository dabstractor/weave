package discover

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes content to dir/name for tests and returns the full path. It
// is the JSDoc analog of extension_test.go's writePackageJSON (defined there
// because that file landed first); it is defined HERE because jsdoc_test.go
// writes arbitrary entry files (.ts/.js), not package.json. MkdirAll ensures
// dir exists.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %q: %v", p, err)
	}
	return p
}

// TestStripStarPrefix pins the JSDoc continuation-line cleaner's contract in
// isolation, so ExtractJSDoc failures localize faster. Cases come from the PRP
// (P1.M2.T1.S2) Task 5 spec.
func TestStripStarPrefix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"star-space", "* foo", "foo"},
		{"star-no-space", "*foo", "foo"},
		{"star-only", "*", ""},
		{"ws-star-space", "   * foo", "foo"},
		{"tab-star", "\t*foo", "foo"},
		{"no-star", "foo", "foo"},
		{"ws-no-star", "  foo", "foo"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripStarPrefix(c.in); got != c.want {
				t.Errorf("stripStarPrefix(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

// TestExtractJSDoc covers the 8 item-contract cases plus the /*** guard, CRLF
// robustness, and the extra edge cases from PRP Task 6. Each case writes a temp
// file and asserts ExtractJSDoc(path) == want.
func TestExtractJSDoc(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}

	cases := []struct {
		name    string
		content string // written verbatim (BOM cases handled separately)
		want    string
	}{
		// 1. Multi-line block.
		{"multiline", "/**\n * Line one.\n * Line two.\n */\ncode();\n", "Line one. Line two."},
		// 2. Single-line block.
		{"single-line", "/** desc */\ncode();\n", "desc"},
		// 3. No JSDoc — code first.
		{"no-jsdoc-code", "export function f() {}\n", ""},
		// 4. Line comment first.
		{"line-comment", "// a comment\ncode();\n", ""},
		// 5. Plain /* block (single star) — NOT a JSDoc opener.
		{"plain-block", "/* plain block */\ncode();\n", ""},
		// 6. BOM-prefixed JSDoc (BOM added in the test body).
		{"bom-prefixed", "/**\n * desc\n */\n", "desc"},
		// 7. Leading blank lines before the opener.
		{"leading-blanks", "\n\n  \n/**\n * desc\n */\n", "desc"},
		// 8. Unclosed JSDoc (no "*/").
		{"unclosed", "/**\n * desc\n", ""},
		// 9. Triple-star /*** — NOT a JSDoc opener (jsdoc.fyi).
		{"triple-star", "/***\n * desc\n */\n", ""},
		// 10. CRLF line endings.
		{"crlf", "/**\r\n * desc\r\n */\r\n", "desc"},
		// 11. Single-line, no spaces inside.
		{"single-line-no-space", "/**desc*/\n", "desc"},
		// 13. Empty file.
		{"empty-file", "", ""},
		// 14. JSDoc with a blank star-only inner line (dropped).
		{"jsdoc-blank-inner", "/**\n * a\n *\n * b\n */\n", "a b"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			name := c.name + ".ts"
			var p string
			if c.name == "bom-prefixed" {
				p = writeFileBytes(t, dir, name, append(bom, []byte(c.content)...))
			} else {
				p = writeFile(t, dir, name, c.content)
			}
			if got := ExtractJSDoc(p); got != c.want {
				t.Errorf("ExtractJSDoc(%q) = %q; want %q", p, got, c.want)
			}
		})
	}

	// 12. Missing file — ExtractJSDoc returns "" without panicking.
	t.Run("missing-file", func(t *testing.T) {
		got := ExtractJSDoc(filepath.Join(t.TempDir(), "does-not-exist.ts"))
		if got != "" {
			t.Errorf("ExtractJSDoc(missing) = %q; want \"\"", got)
		}
	})
}

// writeFileBytes writes raw bytes (used for the BOM case) and returns the path.
func writeFileBytes(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("WriteFile %q: %v", p, err)
	}
	return p
}

// TestExtractJSDocFeedsBuildExtensionFallback is the Level-3 integration test:
// it proves the S1+S2 contract end-to-end. A single-file extension (no
// package.json, hasPkg=false) with a leading JSDoc gets its Description from the
// JSDoc via BuildExtension's fallback chain (pkg.Description empty → jsdocDesc
// wins). This mirrors exactly what T3 Index will do.
func TestExtractJSDocFeedsBuildExtensionFallback(t *testing.T) {
	dir := t.TempDir()
	entry := writeFile(t, dir, "gate.ts",
		"/**\n * Gate function.\n * Second line.\n */\nexport function gate() {}\n")

	jsdoc := ExtractJSDoc(entry)
	if jsdoc != "Gate function. Second line." {
		t.Fatalf("ExtractJSDoc = %q; want \"Gate function. Second line.\"", jsdoc)
	}

	// hasPkg=false, pkg=zero → BuildExtension falls back to jsdocDesc.
	ext := BuildExtension(entry, entry, "gate", "file", PackageJSON{}, false, jsdoc)

	if ext.Description != "Gate function. Second line." {
		t.Errorf("Description = %q; want \"Gate function. Second line.\" (JSDoc fallback)",
			ext.Description)
	}
	if ext.HasPackageJSON {
		t.Errorf("HasPackageJSON = true; want false (single-file extension)")
	}
}
