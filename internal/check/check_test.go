package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dabstractor/weave/internal/discover"
)

// mkFileExt writes a single-file extension (root/<relTag>.ts with an optional
// leading JSDoc) and returns the discover.Extension the way discover.Index
// would: write the file, Index the root, return the matching entry. relTag uses
// '/' separators (cross-platform via filepath.FromSlash). Each extension lives
// under a shared root so duplicate-relTag tests can collide two of them.
//
// This mirrors skilldozer's mkSkill (write a temp file, build via the real
// discover helpers so check sees realistic Extension values).
func mkFileExt(t *testing.T, root, relTag, body string) discover.Extension {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relTag)+".ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return findExt(t, root, relTag)
}

// mkDirExt writes a dir extension (root/<relTag>/index.ts) and returns the
// discover.Extension from discover.Index.
func mkDirExt(t *testing.T, root, relTag, body string) discover.Extension {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relTag))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.ts"), []byte(body), 0o644); err != nil {
		t.Fatalf("write index.ts: %v", err)
	}
	return findExt(t, root, relTag)
}

// mkPackageExt writes a package extension: root/<relTag>/package.json +
// root/<relTag>/src/index.ts (pi.extensions pointing at ./src/index.ts). pkgJSON
// is written verbatim. Returns the discover.Extension from discover.Index.
func mkPackageExt(t *testing.T, root, relTag, pkgJSON, indexBody string) discover.Extension {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relTag))
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", src, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.ts"), []byte(indexBody), 0o644); err != nil {
		t.Fatalf("write src/index.ts: %v", err)
	}
	return findExt(t, root, relTag)
}

// findExt Indexes root and returns the entry whose RelTag matches.
func findExt(t *testing.T, root, relTag string) discover.Extension {
	t.Helper()
	exts, err := discover.Index(root)
	if err != nil {
		t.Fatalf("discover.Index(%s): %v", root, err)
	}
	for _, e := range exts {
		if e.RelTag == relTag {
			return e
		}
	}
	t.Fatalf("extension %q not discovered under %s", relTag, root)
	return discover.Extension{}
}

// finding searches rep for a finding whose message contains substr; returns it
// and whether it was found.
func finding(rep Report, substr string) (Finding, bool) {
	for _, r := range rep.ByExt {
		for _, f := range r.Findings {
			if strings.Contains(f.Message, substr) {
				return f, true
			}
		}
	}
	return Finding{}, false
}

// TestCheckCleanStore: a valid file extension (with a JSDoc description) AND a
// valid package extension (with a package.json description) → 0 errors, 0
// warnings, HasErrors false.
func TestCheckCleanStore(t *testing.T) {
	root := t.TempDir()
	fileExt := mkFileExt(t, root, "gate", "/** a clean file extension */\nexport function x() {}\n")
	pkgExt := mkPackageExt(t, root, "summarizer",
		`{"name":"summarizer","description":"A package.","pi":{"extensions":["./src/index.ts"]}}`,
		"export function s() {}\n")
	exts := []discover.Extension{fileExt, pkgExt}

	rep := Check(root, exts)

	if rep.Errors != 0 || rep.Warnings != 0 {
		t.Errorf("clean store: Errors=%d Warnings=%d; want 0,0. Findings: %+v", rep.Errors, rep.Warnings, rep.ByExt)
	}
	if rep.HasErrors() {
		t.Errorf("clean store: HasErrors=true; want false")
	}
	if len(rep.ByExt) != 2 {
		t.Errorf("ByExt len=%d; want 2", len(rep.ByExt))
	}
	for _, r := range rep.ByExt {
		if len(r.Findings) != 0 {
			t.Errorf("clean extension %q should have 0 findings; got %+v", r.Extension.RelTag, r.Findings)
		}
	}
}

// TestCheckDuplicateRelTag: two entries with the same RelTag → 2 ERRORs, each
// naming the other. (Two index.ts under differently-cased dirs on a
// case-insensitive FS can collide; here we force the collision by building two
// Extensions with the same RelTag via a crafted tree.)
func TestCheckDuplicateRelTag(t *testing.T) {
	root := t.TempDir()
	// Two dir extensions whose RelTags we force equal by building Extensions
	// directly (discover.Index would not normally yield two identical RelTags;
	// we synthesize them to exercise the rule).
	a := mkDirExt(t, root, "alpha", "/** alpha */\n")
	b := mkDirExt(t, root, "beta", "/** beta */\n")
	// Force the collision by overriding RelTag on one of them.
	b.RelTag = a.RelTag // both now "alpha"
	exts := []discover.Extension{a, b}

	rep := Check(root, exts)

	if rep.Errors != 2 {
		t.Errorf("duplicate relTag → Errors=%d; want 2. Findings: %+v", rep.Errors, rep.ByExt)
	}
	if !rep.HasErrors() {
		t.Errorf("HasErrors=false; want true (2 ERRORs)")
	}
	// Each owner's dup ERROR must name the other's tag.
	for _, r := range rep.ByExt {
		if f, ok := finding(Report{ByExt: []ExtensionReport{r}}, "duplicate relTag"); !ok || f.Level != LevelError {
			t.Errorf("owner %q missing a duplicate relTag ERROR; got %+v", r.Extension.RelTag, r.Findings)
		} else if !strings.Contains(f.Message, "alpha") {
			t.Errorf("dup message should name the shared tag; got %q", f.Message)
		}
	}
}

// TestCheckMissingEntryFile: an extension whose EntryFile stat fails → 1 ERROR
// 'entry file does not exist'. Built by Index-then-delete to keep the Extension
// realistic while forcing the stat failure.
func TestCheckMissingEntryFile(t *testing.T) {
	root := t.TempDir()
	ext := mkFileExt(t, root, "gone", "/** desc */\nexport function g() {}\n")
	// Delete the entry file after discovery so Check's stat fails.
	if err := os.Remove(ext.EntryFile); err != nil {
		t.Fatalf("remove entry file %s: %v", ext.EntryFile, err)
	}

	rep := Check(root, []discover.Extension{ext})

	if rep.Errors != 1 {
		t.Errorf("missing entryFile → Errors=%d; want 1. Findings: %+v", rep.Errors, rep.ByExt)
	}
	f, ok := finding(rep, "entry file does not exist")
	if !ok || f.Level != LevelError {
		t.Errorf("missing 'entry file does not exist' ERROR; got %+v", rep.ByExt[0].Findings)
	}
	if !strings.Contains(f.Message, ext.EntryFile) {
		t.Errorf("missing-entryFile message should name the path; got %q", f.Message)
	}
}

// TestCheckUnparseablePackageJSON: a package.json with broken JSON → 1 ERROR
// 'package.json is not valid JSON'. The fixture is a PACKAGE extension: its dir
// contains a broken package.json, so classifyDir's case (b) binds the entry to
// the package root (kind="package"); check then re-parses that root and
// recovers the unparseable error. The index.ts at the top level is the
// best-effort entry file (brokenPackageEntryFile falls back to it).
func TestCheckUnparseablePackageJSON(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash("broken"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// index.ts at the top level → best-effort entry file for the broken-JSON
	// package (case b of classifyDir uses index.ts/index.js at the root when the
	// package.json is unparseable). The dir is classified as kind="package"
	// because a broken package.json signals package intent.
	if err := os.WriteFile(filepath.Join(dir, "index.ts"), []byte("/** d */\nexport function b() {}\n"), 0o644); err != nil {
		t.Fatalf("write index.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{ not valid json`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	ext := findExt(t, root, "broken")
	if ext.Kind != "package" {
		t.Fatalf("expected package kind (broken pkg binds to package root); got %q", ext.Kind)
	}

	rep := Check(root, []discover.Extension{ext})

	if rep.Errors != 1 {
		t.Errorf("unparseable package.json → Errors=%d; want 1. Findings: %+v", rep.Errors, rep.ByExt)
	}
	f, ok := finding(rep, "package.json is not valid JSON")
	if !ok || f.Level != LevelError {
		t.Errorf("missing 'package.json is not valid JSON' ERROR; got %+v", rep.ByExt[0].Findings)
	}
}

// TestCheckUnparseablePackageJSONNestedEntry: the BUG 3 regression — a package
// extension whose entry lives in a subdir (src/index.ts) with NO root index.ts,
// and a broken package.json at the package root. discover must bind the entry
// to the package root (tag = the dir basename, NOT "<dir>/src"), and check must
// surface the §9 ERROR (exit 1). Previously the dir was misclassified as a
// plain category folder, src/ discovered as a stray dir extension, and the
// broken package.json at the root was never inspected.
func TestCheckUnparseablePackageJSONNestedEntry(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash("p2"))
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.ts"), []byte("export function s() {}\n"), 0o644); err != nil {
		t.Fatalf("write src/index.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{ this is not valid json\n`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	exts, err := discover.Index(root)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(exts) != 1 {
		t.Fatalf("expected 1 extension bound to the package root; got %d: %+v", len(exts), exts)
	}
	if exts[0].RelTag != "p2" {
		t.Errorf("RelTag=%q; want p2 (bound to package root, not %q/src)", exts[0].RelTag, exts[0].RelTag)
	}
	if exts[0].Kind != "package" {
		t.Errorf("Kind=%q; want package", exts[0].Kind)
	}

	rep := Check(root, exts)
	if !rep.HasErrors() {
		t.Errorf("broken package.json with nested entry → no ERROR; want §9 'not valid JSON' ERROR. Findings: %+v", rep.ByExt)
	}
	if f, ok := finding(rep, "package.json is not valid JSON"); !ok || f.Level != LevelError {
		t.Errorf("missing 'package.json is not valid JSON' ERROR; got %+v", rep.ByExt[0].Findings)
	}
}

// TestCheckDepsWithoutNodeModules: a package ext whose package.json declares
// non-empty dependencies AND has no node_modules/ → 1 WARN.
func TestCheckDepsWithoutNodeModules(t *testing.T) {
	root := t.TempDir()
	ext := mkPackageExt(t, root, "summarizer",
		`{"name":"summarizer","description":"d","pi":{"extensions":["./src/index.ts"]},"dependencies":{"zod":"^3.0.0"}}`,
		"export function s() {}\n")

	rep := Check(root, []discover.Extension{ext})

	if rep.Warnings != 1 || rep.Errors != 0 {
		t.Errorf("deps-no-node_modules → Warnings=%d Errors=%d; want 1,0. Findings: %+v", rep.Warnings, rep.Errors, rep.ByExt)
	}
	if f, ok := finding(rep, "dependencies declared but node_modules"); !ok || f.Level != LevelWarn {
		t.Errorf("missing deps-without-node_modules WARN; got %+v", rep.ByExt[0].Findings)
	}
}

// TestCheckDepsWithNodeModulesOK: same as above but node_modules/ present → no
// deps WARN.
func TestCheckDepsWithNodeModulesOK(t *testing.T) {
	root := t.TempDir()
	ext := mkPackageExt(t, root, "summarizer",
		`{"name":"summarizer","description":"d","pi":{"extensions":["./src/index.ts"]},"dependencies":{"zod":"^3.0.0"}}`,
		"export function s() {}\n")
	// Create node_modules/ in the package dir.
	if err := os.MkdirAll(filepath.Join(ext.Path, "node_modules"), 0o755); err != nil {
		t.Fatalf("MkdirAll node_modules: %v", err)
	}

	rep := Check(root, []discover.Extension{ext})

	if f, ok := finding(rep, "dependencies declared"); ok {
		t.Errorf("node_modules present → should NOT warn; got %q", f.Message)
	}
	if rep.Errors != 0 || rep.Warnings != 0 {
		t.Errorf("deps-with-node_modules → Errors=%d Warnings=%d; want 0,0. Findings: %+v", rep.Errors, rep.Warnings, rep.ByExt)
	}
}

// TestCheckDepsNoWarnForDirKind: a DIR extension (not package) with a package
// json that has deps must NOT fire the deps WARN (PRD §9 scopes it to package
// extensions). Uses a dir extension that happens to have a package.json.
func TestCheckDepsNoWarnForDirKind(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dirwithpkg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// index.ts → dir-kind extension (classifyDir case b wins because package.json
	// has NO pi.extensions, so it is not a package extension).
	if err := os.WriteFile(filepath.Join(dir, "index.ts"), []byte("/** d */\nexport function x() {}\n"), 0o644); err != nil {
		t.Fatalf("write index.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"d","description":"d","dependencies":{"zod":"^3"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	ext := findExt(t, root, "dirwithpkg")
	if ext.Kind != "dir" {
		t.Fatalf("expected dir kind; got %q", ext.Kind)
	}

	rep := Check(root, []discover.Extension{ext})

	if f, ok := finding(rep, "dependencies declared"); ok {
		t.Errorf("dir-kind extension must NOT fire deps WARN; got %q", f.Message)
	}
}

// TestCheckNoDescription: a file extension with no package.json and no JSDoc →
// 1 WARN 'no description'.
func TestCheckNoDescription(t *testing.T) {
	root := t.TempDir()
	// No leading JSDoc → BuildExtension folds to "" description.
	ext := mkFileExt(t, root, "bare", "export function b() {}\n")

	rep := Check(root, []discover.Extension{ext})

	if rep.Warnings != 1 || rep.Errors != 0 {
		t.Errorf("no-description → Warnings=%d Errors=%d; want 1,0. Findings: %+v", rep.Warnings, rep.Errors, rep.ByExt)
	}
	if f, ok := finding(rep, "no description"); !ok || f.Level != LevelWarn {
		t.Errorf("missing 'no description' WARN; got %+v", rep.ByExt[0].Findings)
	}
}

// TestCheckDescriptionFromJSDoc: a file extension with a leading JSDoc but no
// package.json → no description WARN (JSDoc satisfies it).
func TestCheckDescriptionFromJSDoc(t *testing.T) {
	root := t.TempDir()
	ext := mkFileExt(t, root, "withjsdoc", "/** A nice description. */\nexport function w() {}\n")

	rep := Check(root, []discover.Extension{ext})

	if f, ok := finding(rep, "no description"); ok {
		t.Errorf("JSDoc should satisfy description; got WARN %q", f.Message)
	}
}

// TestCheckEmptyCategoryFolder: a top-level plain subdir with zero discoverable
// entries at any depth → 1 WARN 'empty category folder'.
func TestCheckEmptyCategoryFolder(t *testing.T) {
	root := t.TempDir()
	// A real extension at the top level (so the store is not "empty").
	ext := mkFileExt(t, root, "gate", "/** d */\nexport function g() {}\n")
	// A plain empty category folder (no entries anywhere inside it).
	if err := os.MkdirAll(filepath.Join(root, "abandoned"), 0o755); err != nil {
		t.Fatalf("MkdirAll abandoned: %v", err)
	}

	rep := Check(root, []discover.Extension{ext})

	if rep.Warnings != 1 || rep.Errors != 0 {
		t.Errorf("empty folder → Warnings=%d Errors=%d; want 1,0. Findings: %+v", rep.Warnings, rep.Errors, rep.ByExt)
	}
	f, ok := finding(rep, "empty category folder")
	if !ok || f.Level != LevelWarn {
		t.Errorf("missing 'empty category folder' WARN; got %+v", rep.ByExt)
	}
	if !strings.Contains(f.Message, "abandoned") {
		t.Errorf("empty-folder WARN should name the folder; got %q", f.Message)
	}
}

// TestCheckNestedExtensionFolderNotFlagged: a top-level subdir that IS a
// resolvable extension (has index.ts) is NOT an empty folder (discover.Index
// yields ≥1 entry for it).
func TestCheckNestedExtensionFolderNotFlagged(t *testing.T) {
	root := t.TempDir()
	// A dir extension under root → resolvable, so NOT empty.
	mkDirExt(t, root, "git-checkpoint", "/** d */\nexport function g() {}\n")
	// Build the exts slice the way main would: Index the whole root.
	exts, err := discover.Index(root)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	rep := Check(root, exts)

	if f, ok := finding(rep, "empty category folder"); ok {
		t.Errorf("a resolvable extension dir must not be flagged empty; got %q", f.Message)
	}
}

// TestCheckEmptyInputClean: Check(dir, nil) → 0/0, HasErrors false, no panic.
func TestCheckEmptyInputClean(t *testing.T) {
	root := t.TempDir()
	rep := Check(root, nil)

	if rep.Errors != 0 || rep.Warnings != 0 || len(rep.ByExt) != 0 {
		t.Errorf("Check(dir, nil) should be empty clean report; got %+v", rep)
	}
	if rep.HasErrors() {
		t.Errorf("Check(dir, nil).HasErrors()=true; want false")
	}
}

// TestCheckEmptyDirNoEmptyFolderFinding: an extensions dir that is completely
// empty (no extensions AND no subdirs) yields 0 findings (no empty-folder WARN,
// because there are no top-level subdirs to walk).
func TestCheckEmptyDirNoEmptyFolderFinding(t *testing.T) {
	root := t.TempDir() // empty
	rep := Check(root, nil)

	if rep.Errors != 0 || rep.Warnings != 0 {
		t.Errorf("empty dir → Errors=%d Warnings=%d; want 0,0", rep.Errors, rep.Warnings)
	}
}
