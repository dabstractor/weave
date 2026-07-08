package discover

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// NOTE: writeFile(t, dir, name, content) is reused from jsdoc_test.go (S2) — it
// writes content to dir/name, creating dir, and returns the full path. It is the
// JSDoc analog of extension_test.go's writePackageJSON. Reusing it avoids a
// redeclaration and keeps the package's test helpers DRY.

// walkClassified is the mini-Index: the T3 WalkDir skeleton lived in the test
// file until T3 (P1.M2.T3) implements it for real in index.go. It PROVES the
// classify-then-descend rule end-to-end by driving a real on-disk walk with
// classifyFile/classifyDir and applying the SkipDir rule.
//
// SkipDir semantics (research/walkdir_skipdir_semantics.md): SkipDir on a DIR
// skips its subtree (used to prune recognized extension dirs); SkipDir on a
// FILE skips SIBLINGS (which is why the file branch returns nil, NOT SkipDir).
func walkClassified(root string) []Extension {
	var result []Extension
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entry, keep walking
		}
		if d.IsDir() {
			ext, isExt, descend := classifyDir(root, path)
			if isExt {
				result = append(result, *ext)
			}
			if !descend {
				return filepath.SkipDir // prune subtree (load-bearing recursion rule)
			}
			return nil // plain dir → descend
		}
		ext, ok := classifyFile(root, path)
		if ok {
			result = append(result, *ext)
		}
		return nil // NOT SkipDir (SkipDir on a file skips siblings)
	})
	sort.Slice(result, func(i, j int) bool { return result[i].RelTag < result[j].RelTag })
	return result
}

// --- isExtensionFile (table-driven; mirrors extdir.isExtensionFile's rule) ---

func TestIsExtensionFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"gate.ts", true},
		{"gate.js", true},
		{"reddit-poster.ts", true},
		{"index.ts", false}, // dir-entry marker, handled by classifyDir
		{"index.js", false}, // dir-entry marker, handled by classifyDir
		{"readme.md", false},
		{"package.json", false},
		{"gate.txt", false},
		{"index.tsx", false}, // not .ts/.js
		{".ts", true},        // edge: a file named ".ts" — base is ".ts", HasSuffix true, != index.ts
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isExtensionFile(c.name); got != c.want {
				t.Errorf("isExtensionFile(%q)=%v; want %v", c.name, got, c.want)
			}
		})
	}
}

// --- classifyFile ---

// TestClassifyFileSingleFile: a root-level gate.ts with a JSDoc classifies as
// a single-file extension with RelTag="gate", Kind="file", Path==EntryFile.
func TestClassifyFileSingleFile(t *testing.T) {
	root := t.TempDir()
	path := writeFile(t, root, "gate.ts", "/** Gate extension. */\nexport default function () {}\n")

	ext, ok := classifyFile(root, path)
	if !ok {
		t.Fatal("ok=false; want true (gate.ts is a single-file extension)")
	}
	if ext == nil {
		t.Fatal("ext=nil; want non-nil")
	}
	if ext.RelTag != "gate" {
		t.Errorf("RelTag=%q; want gate", ext.RelTag)
	}
	if ext.Kind != "file" {
		t.Errorf("Kind=%q; want file", ext.Kind)
	}
	if ext.Path != path {
		t.Errorf("Path=%q; want %q", ext.Path, path)
	}
	if ext.EntryFile != path {
		t.Errorf("EntryFile=%q; want %q (Path==EntryFile for a file extension)", ext.EntryFile, path)
	}
	if ext.Description != "Gate extension." {
		t.Errorf("Description=%q; want 'Gate extension.' (from JSDoc)", ext.Description)
	}
	if ext.HasPackageJSON {
		t.Error("HasPackageJSON=true; want false (no package.json in root)")
	}
}

// TestClassifyFileRejectsIndexTS: index.ts is a dir extension's entry marker,
// handled by classifyDir on the PARENT dir — classifyFile returns (nil, false).
func TestClassifyFileRejectsIndexTS(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	path := writeFile(t, sub, "index.ts", "/** dir ext. */\n")
	if _, ok := classifyFile(root, path); ok {
		t.Error("ok=true; want false (index.ts is a dir entry, not a file entry)")
	}
}

// TestClassifyFileRejectsNonTSJS: a non-.ts/.js file is not an extension.
func TestClassifyFileRejectsNonTSJS(t *testing.T) {
	root := t.TempDir()
	path := writeFile(t, root, "readme.md", "# readme\n")
	if _, ok := classifyFile(root, path); ok {
		t.Error("ok=true; want false (readme.md is not .ts/.js)")
	}
}

// TestClassifyFileNestedRelTag: a file in a nested category dir gets a
// slash-normalized relTag with the .ts suffix stripped. Asserts NO backslash
// (proves filepath.ToSlash ran).
func TestClassifyFileNestedRelTag(t *testing.T) {
	root := t.TempDir()
	writing := filepath.Join(root, "writing")
	path := writeFile(t, writing, "reddit-poster.ts", "/** Post to reddit. */\n")

	ext, ok := classifyFile(root, path)
	if !ok {
		t.Fatal("ok=false; want true")
	}
	if ext.RelTag != "writing/reddit-poster" {
		t.Errorf("RelTag=%q; want 'writing/reddit-poster'", ext.RelTag)
	}
	if strings.Contains(ext.RelTag, "\\") {
		t.Errorf("RelTag=%q contains a backslash; OS separators must be normalized to '/'", ext.RelTag)
	}
}

// TestClassifyFileJSDocOnlyDescription: a single-file extension with NO
// package.json gets its description from JSDoc (the fallback source). Proves
// ExtractJSDoc (S2) feeds BuildExtension (S1) via classifyFile (T2).
func TestClassifyFileJSDocOnlyDescription(t *testing.T) {
	root := t.TempDir()
	path := writeFile(t, root, "gate.ts", "/** Gate desc. */\nexport default function () {}\n")

	ext, ok := classifyFile(root, path)
	if !ok {
		t.Fatal("ok=false; want true")
	}
	if ext.Description != "Gate desc." {
		t.Errorf("Description=%q; want 'Gate desc.' (from JSDoc, no package.json)", ext.Description)
	}
	if ext.HasPackageJSON {
		t.Error("HasPackageJSON=true; want false (no package.json in root)")
	}
}

// --- classifyDir ---

// TestClassifyDirPackage: a dir with package.json (pi.extensions → existing
// src/index.ts) classifies as a package extension with shouldDescend=false.
func TestClassifyDirPackage(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "summarizer")
	writePackageJSON(t, dir, `{
	  "name": "@o/sum",
	  "description": "d",
	  "pi": { "extensions": ["./src/index.ts"] }
	}`)
	entryFile := writeFile(t, filepath.Join(dir, "src"), "index.ts", "/** pkg entry. */\n")

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true (package extension)")
	}
	if descend {
		t.Error("descend=true; want false (load-bearing: recognized extensions are NOT descended)")
	}
	if ext == nil {
		t.Fatal("ext=nil; want non-nil")
	}
	if ext.Kind != "package" {
		t.Errorf("Kind=%q; want package", ext.Kind)
	}
	if ext.EntryFile != entryFile {
		t.Errorf("EntryFile=%q; want %q", ext.EntryFile, entryFile)
	}
	if ext.RelTag != "summarizer" {
		t.Errorf("RelTag=%q; want summarizer", ext.RelTag)
	}
	if ext.Name != "@o/sum" {
		t.Errorf("Name=%q; want @o/sum", ext.Name)
	}
}

// TestClassifyDirPackageNonExistentEntry: a package.json whose pi.extensions
// names a NON-existent file does NOT qualify as a package (PRD §7.1: "≥1
// existing entry"). With no index.ts/index.js, it falls through to plain dir.
func TestClassifyDirPackageNonExistentEntry(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "broken")
	writePackageJSON(t, dir, `{ "pi": { "extensions": ["./missing.ts"] } }`)
	// NOTE: no ./missing.ts, no index.ts, no index.js.

	ext, isExt, descend := classifyDir(root, dir)
	if isExt {
		t.Error("isExt=true; want false (pi.extensions names a non-existent file)")
	}
	if ext != nil {
		t.Errorf("ext=%v; want nil", ext)
	}
	if !descend {
		t.Error("descend=false; want true (falls through to plain dir)")
	}
}

// TestClassifyDirIndexTS: a dir with index.ts (+ an internal helper that must
// NOT be double-counted) classifies as a dir extension with shouldDescend=false.
func TestClassifyDirIndexTS(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "git-checkpoint")
	writeFile(t, dir, "index.ts", "/** checkpoint ext. */\n")
	writeFile(t, dir, "utils.ts", "// internal helper — must NOT be a separate entry\n")

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true (dir extension with index.ts)")
	}
	if descend {
		t.Error("descend=true; want false (load-bearing)")
	}
	if ext.Kind != "dir" {
		t.Errorf("Kind=%q; want dir", ext.Kind)
	}
	if !strings.HasSuffix(ext.EntryFile, "git-checkpoint/index.ts") {
		t.Errorf("EntryFile=%q; want suffix 'git-checkpoint/index.ts'", ext.EntryFile)
	}
	if ext.RelTag != "git-checkpoint" {
		t.Errorf("RelTag=%q; want git-checkpoint", ext.RelTag)
	}
}

// TestClassifyDirIndexJS: a dir with index.js classifies as a dir extension.
func TestClassifyDirIndexJS(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "x")
	writeFile(t, dir, "index.js", "/** js dir ext. */\n")

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true")
	}
	if descend {
		t.Error("descend=true; want false (load-bearing)")
	}
	if ext.Kind != "dir" {
		t.Errorf("Kind=%q; want dir", ext.Kind)
	}
	if !strings.HasSuffix(ext.EntryFile, "index.js") {
		t.Errorf("EntryFile=%q; want suffix 'index.js'", ext.EntryFile)
	}
}

// TestClassifyDirPlain: a dir with no package.json / index.* is a plain
// category directory → descend.
func TestClassifyDirPlain(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "category")
	// Only a non-entry file (readme.md) — NOT an extension.
	writeFile(t, dir, "readme.md", "# category\n")

	ext, isExt, descend := classifyDir(root, dir)
	if isExt {
		t.Error("isExt=true; want false (plain category dir)")
	}
	if ext != nil {
		t.Errorf("ext=%v; want nil", ext)
	}
	if !descend {
		t.Error("descend=false; want true (plain dir is descended)")
	}
}

// TestClassifyDirPrecedencePackageBeatsIndexTS: a dir with BOTH a qualifying
// package.json (pi.extensions → existing file) AND index.ts is a PACKAGE
// (case a wins). Pins pi's loader precedence (pi_extension_facts.md §4).
func TestClassifyDirPrecedencePackageBeatsIndexTS(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "both")
	writePackageJSON(t, dir, `{ "pi": { "extensions": ["./src/index.ts"] } }`)
	writeFile(t, filepath.Join(dir, "src"), "index.ts", "/** pkg entry. */\n")
	writeFile(t, dir, "index.ts", "/** dir entry. */\n") // also exists

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true")
	}
	if descend {
		t.Error("descend=true; want false (load-bearing)")
	}
	if ext.Kind != "package" {
		t.Errorf("Kind=%q; want package (package.json beats index.ts)", ext.Kind)
	}
	// EntryFile must be the package's pi.extensions entry, NOT index.ts.
	if !strings.HasSuffix(ext.EntryFile, "src/index.ts") {
		t.Errorf("EntryFile=%q; want the package.json pi.extensions entry (src/index.ts), not index.ts", ext.EntryFile)
	}
}

// TestClassifyDirIndexTSPrecedenceOverIndexJS: a dir with BOTH index.ts and
// index.js is a DIR with index.ts (case b beats c).
func TestClassifyDirIndexTSPrecedenceOverIndexJS(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "x")
	writeFile(t, dir, "index.ts", "/** ts. */\n")
	writeFile(t, dir, "index.js", "/** js. */\n")

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true")
	}
	if descend {
		t.Error("descend=true; want false")
	}
	if !strings.HasSuffix(ext.EntryFile, "index.ts") {
		t.Errorf("EntryFile=%q; want suffix 'index.ts' (b beats c)", ext.EntryFile)
	}
}

// --- integration: classifyDir composes S1 + S2 for the richest case ---

// TestClassifyDirPackageFullMetadata: a PRD-§10 package.json (name, description,
// keywords, weave.aliases, weave.category, pi.extensions) + src/index.ts with a
// JSDoc. classifyDir must return a fully-populated *Extension. Proves
// parsePackageJSON (S1) → classifyDir (T2) → BuildExtension (S1) with
// ExtractJSDoc (S2) fallback, all wired correctly. The description-fallback
// priority (pkg.Description wins over jsdoc) is S1's concern; this test
// confirms T2 passes the right values through.
func TestClassifyDirPackageFullMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "summarizer")
	writePackageJSON(t, dir, `{
	  "name": "@my-org/summarizer",
	  "description": "Summarize conversation history your way.",
	  "keywords": ["compaction", "summary", "context"],
	  "pi": { "extensions": ["./src/index.ts"] },
	  "weave": {
	    "aliases": ["summarise", "compact-helper"],
	    "category": "context"
	  }
	}`)
	entryFile := writeFile(t, filepath.Join(dir, "src"), "index.ts", "/** fallback jsdoc. */\n")

	ext, isExt, descend := classifyDir(root, dir)
	if !isExt {
		t.Fatal("isExt=false; want true")
	}
	if descend {
		t.Error("descend=true; want false (load-bearing)")
	}
	if ext.Kind != "package" {
		t.Errorf("Kind=%q; want package", ext.Kind)
	}
	if ext.EntryFile != entryFile {
		t.Errorf("EntryFile=%q; want %q (the package.json pi.extensions entry)", ext.EntryFile, entryFile)
	}
	if ext.Name != "@my-org/summarizer" {
		t.Errorf("Name=%q; want @my-org/summarizer (from pkg)", ext.Name)
	}
	// pkg.Description is non-empty → it wins over the JSDoc fallback.
	if ext.Description != "Summarize conversation history your way." {
		t.Errorf("Description=%q; want the package.json description (pkg wins over jsdoc)", ext.Description)
	}
	if !strEq(ext.Keywords, []string{"compaction", "summary", "context"}) {
		t.Errorf("Keywords=%v; want [compaction summary context]", ext.Keywords)
	}
	if ext.Category != "context" {
		t.Errorf("Category=%q; want context", ext.Category)
	}
	if !strEq(ext.Aliases, []string{"summarise", "compact-helper"}) {
		t.Errorf("Aliases=%v; want [summarise compact-helper]", ext.Aliases)
	}
	if !ext.HasPackageJSON {
		t.Error("HasPackageJSON=false; want true")
	}
}

// --- THE ACCEPTANCE TEST: PRD §7.1 worked example, end-to-end via mini-walk ---

// TestWorkedExample builds the EXACT PRD §7.1 tree and asserts the classify-
// then-descend walk produces EXACTLY 5 entries with the correct relTags and
// kinds. This is the load-bearing recursion rule proven end-to-end:
//   - git-checkpoint/utils.ts is NOT a separate entry (git-checkpoint is a dir,
//     not descended → utils.ts never visited).
//   - summarizer/src/index.ts is NOT a separate entry (summarizer is a package,
//     not descended → src/index.ts never visited).
//   - git-checkpoint/index.ts is NOT a separate FILE entry (git-checkpoint is a
//     dir, not descended → index.ts never visited as a file).
func TestWorkedExample(t *testing.T) {
	root := t.TempDir()

	// gate.ts — single-file extension at the root.
	writeFile(t, root, "gate.ts", "/** Gate. */\n")

	// git-checkpoint/ — dir extension (index.ts + utils.ts). utils.ts must NOT
	// appear as a separate entry (the load-bearing recursion rule).
	writeFile(t, filepath.Join(root, "git-checkpoint"), "index.ts", "/** Checkpoint. */\n")
	writeFile(t, filepath.Join(root, "git-checkpoint"), "utils.ts", "// internal helper\n")

	// summarizer/ — package extension (package.json + src/index.ts). src/index.ts
	// must NOT appear as a separate entry.
	writePackageJSON(t, filepath.Join(root, "summarizer"), `{
	  "name": "@o/summarizer",
	  "description": "Summarize.",
	  "pi": { "extensions": ["./src/index.ts"] }
	}`)
	writeFile(t, filepath.Join(root, "summarizer", "src"), "index.ts", "/** pkg entry. */\n")

	// writing/ — plain category dir containing a single-file extension.
	writeFile(t, filepath.Join(root, "writing"), "reddit-poster.ts", "/** Post to reddit. */\n")

	// platform/linux/ — nested dir extension.
	writeFile(t, filepath.Join(root, "platform", "linux"), "index.ts", "/** Linux platform. */\n")

	got := walkClassified(root)

	// EXACTLY 5 entries.
	if len(got) != 5 {
		t.Fatalf("len(got)=%d; want 5. Entries: %+v", len(got), relTags(got))
	}

	wantRelTags := []string{
		"gate",
		"git-checkpoint",
		"platform/linux",
		"summarizer",
		"writing/reddit-poster",
	}
	gotRelTags := relTags(got)
	for i, w := range wantRelTags {
		if gotRelTags[i] != w {
			t.Errorf("RelTags[%d]=%q; want %q (full: %v)", i, gotRelTags[i], w, gotRelTags)
		}
	}

	// Kinds per relTag.
	byTag := make(map[string]Extension, len(got))
	for _, e := range got {
		byTag[e.RelTag] = e
	}
	wantKinds := map[string]string{
		"gate":                  "file",
		"git-checkpoint":        "dir",
		"platform/linux":        "dir",
		"summarizer":            "package",
		"writing/reddit-poster": "file",
	}
	for tag, wantKind := range wantKinds {
		e, ok := byTag[tag]
		if !ok {
			t.Errorf("missing entry for relTag %q", tag)
			continue
		}
		if e.Kind != wantKind {
			t.Errorf("Kind(%q)=%q; want %q", tag, e.Kind, wantKind)
		}
	}

	// The load-bearing recursion rule: these MUST NOT appear as separate entries.
	forbidden := []string{
		"git-checkpoint/utils",    // inside git-checkpoint (dir, not descended)
		"git-checkpoint/utils.ts", // (unstripped form, just in case)
		"git-checkpoint/index",    // git-checkpoint's entry marker, not a file entry
		"summarizer/src/index",    // inside summarizer (package, not descended)
		"summarizer/src/index.ts",
	}
	for _, bad := range forbidden {
		if _, ok := byTag[bad]; ok {
			t.Errorf("load-bearing recursion rule violated: %q appeared as a separate entry (it must be pruned by SkipDir)", bad)
		}
	}
}

// relTags returns the sorted relTags of a slice of Extensions (helper for
// TestWorkedExample's diagnostics).
func relTags(exts []Extension) []string {
	out := make([]string, len(exts))
	for i, e := range exts {
		out[i] = e.RelTag
	}
	return out
}
