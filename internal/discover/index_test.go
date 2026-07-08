package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// index_test.go (P1.M2.T3.S1) drives the PUBLIC Index() through the same
// scenarios discover_test.go's walkClassified already proved, PLUS the four
// additions that make Index a real public API: the stat-guard (missing root,
// root-is-file), the nil-on-empty contract, the absolute-output contract for
// relative input, and the by-RelTag sort. It is white-box (package discover)
// so it reuses writeFile (jsdoc_test.go), writePackageJSON + strEq
// (extension_test.go), and relTags (discover_test.go) WITHOUT redeclaring them.

// --- stat-guard: the load-bearing additions over walkClassified ---

// TestIndexMissingRoot: a non-existent root must surface an error, NOT be
// swallowed as (nil, nil). Without the os.Stat guard BEFORE WalkDir, WalkDir
// feeds the root's lstat error to the callback, and the per-entry
// `if err != nil { return nil }` hides it. The guard pins PRD §6.4 error
// semantics (critical for `$(...)` use).
func TestIndexMissingRoot(t *testing.T) {
	_, err := Index(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("err=nil; want an error (missing root must propagate after the Stat guard)")
	}
}

// TestIndexRootIsFile: a root that is a regular file (not a directory) must
// surface an error. The guard's IsDir check enforces "extensions dir is a dir".
func TestIndexRootIsFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	_, err = Index(f.Name())
	if err == nil {
		t.Fatal("err=nil; want an error (root must be a directory)")
	}
}

// --- nil-on-empty contract (§3d) ---

// TestIndexEmptyDir: an existing empty dir yields a zero-length slice and nil
// error (callers test with len(), e.g. --list exits 1 "if no extensions found").
func TestIndexEmptyDir(t *testing.T) {
	got, err := Index(t.TempDir()) // exists, empty
	if err != nil {
		t.Fatalf("err=%v; want nil (empty dir is not an error)", err)
	}
	if len(got) != 0 {
		t.Errorf("len=%d; want 0 (empty tree → no extensions)", len(got))
	}
}

// TestIndexNilOnEmpty pins the nil-vs-empty distinction: an empty store yields
// a NIL slice (not a non-nil empty slice). `var result []Extension` with zero
// appends is nil; pre-allocating make([]Extension, 0) would fail this test.
// Matters for any caller doing `if exts == nil`.
func TestIndexNilOnEmpty(t *testing.T) {
	got, _ := Index(t.TempDir())
	if got != nil {
		t.Errorf("got=%v; want nil (empty store → nil slice, §3d contract)", got)
	}
}

// --- sort by RelTag (still required: WalkDir is lexical by FILENAME) ---

// TestIndexSortedByRelTag proves the result is sorted by RelTag ascending, NOT
// by walk-visit order. The fixtures are ordered so FILENAME order ≠ RELTAG
// order, so a missing sort would produce a different sequence. WalkDir visits
// lexical by filename within each dir; the sort key is the full relTag, so
// nested entries (mango/beta vs zebra) need reordering.
func TestIndexSortedByRelTag(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "zebra.ts", "/** z. */\n")                        // tag "zebra"
	writeFile(t, root, "apple.ts", "/** a. */\n")                        // tag "apple"
	writeFile(t, filepath.Join(root, "mango"), "fig.ts", "/** f. */\n")  // tag "mango/fig"
	writeFile(t, filepath.Join(root, "mango"), "beta.ts", "/** b. */\n") // tag "mango/beta"
	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	tags := relTags(got)
	want := []string{"apple", "mango/beta", "mango/fig", "zebra"}
	if !strEq(tags, want) {
		t.Errorf("order=%v; want %v (lexicographic by RelTag)", tags, want)
	}
}

// --- absolute-output contract (PRD §6.1, §13) ---

// TestIndexRelativeInputStillAbsolute pins that a RELATIVE extensionsDir still
// yields absolute Path/EntryFile: Index calls filepath.Abs FIRST, so the
// classifiers compute relTag against the absolute root and produce absolute
// paths. t.Chdir (Go 1.24+; go.mod is 1.25) scopes the cwd change.
func TestIndexRelativeInputStillAbsolute(t *testing.T) {
	absRoot := t.TempDir()
	writeFile(t, absRoot, "example.ts", "/** ex. */\n")
	parent := filepath.Dir(absRoot)
	t.Chdir(parent) // Go 1.24+; restored on test completion
	rel, err := filepath.Rel(parent, absRoot)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Index(rel) // RELATIVE input
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d; want 1", len(got))
	}
	if !filepath.IsAbs(got[0].Path) {
		t.Errorf("Path=%q is RELATIVE; want absolute (relative input must still abs-ify)", got[0].Path)
	}
	if !filepath.IsAbs(got[0].EntryFile) {
		t.Errorf("EntryFile=%q is RELATIVE; want absolute", got[0].EntryFile)
	}
	if got[0].RelTag != "example" {
		t.Errorf("RelTag=%q; want example", got[0].RelTag)
	}
}

// --- THE ACCEPTANCE TEST: PRD §7.1 worked example via the PUBLIC Index() ---

// TestIndexWorkedExample builds the EXACT PRD §7.1 tree and asserts Index
// produces EXACTLY 5 entries with the correct relTags and kinds. This is the
// same tree as discover_test.go's TestWorkedExample (which drives walkClassified),
// but run through the PUBLIC Index() — proving Index == walkClassified + guard.
// The load-bearing recursion rule is pinned: git-checkpoint/utils.ts,
// git-checkpoint/index.ts, and summarizer/src/index.ts do NOT appear as
// separate entries (recognized dir/package extensions are pruned by SkipDir).
func TestIndexWorkedExample(t *testing.T) {
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

	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v; want nil (valid tree)", err)
	}

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

	// Build a lookup map for kind + forbidden-entry checks.
	byTag := make(map[string]Extension, len(got))
	for _, e := range got {
		byTag[e.RelTag] = e
	}

	// Kinds per relTag.
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

	// Absolute-output contract: every Path and EntryFile must be absolute.
	for _, e := range got {
		if !filepath.IsAbs(e.Path) {
			t.Errorf("Path=%q is RELATIVE; want absolute (PRD §6.1)", e.Path)
		}
		if !filepath.IsAbs(e.EntryFile) {
			t.Errorf("EntryFile=%q is RELATIVE; want absolute (PRD §6.1)", e.EntryFile)
		}
	}
}

// --- stray files / non-entry subdirs are ignored ---

// TestIndexIgnoresStrayFiles: README.md (non-.ts/.js), draft.txt (in a plain
// subdir), and gate.ts.bak (non-.ts/.js) are all ignored by classifyFile. The
// "notes" plain dir with no entries contributes nothing. Only real/gate is
// discovered.
func TestIndexIgnoresStrayFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real"), "gate.ts", "/** real. */\n")
	// Distractions that must NOT be treated as extensions:
	writeFile(t, root, "README.md", "# hi\n")
	writeFile(t, filepath.Join(root, "notes"), "draft.txt", "draft\n")
	writeFile(t, root, "gate.ts.bak", "bak\n")
	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(got) != 1 || got[0].RelTag != "real/gate" {
		t.Fatalf("got=%v; want exactly one extension 'real/gate' (stray files/subdirs ignored)", relTags(got))
	}
}

// TestIndexRootIndexTSDoesNotCollapse reproduces Issue 3: a stray index.ts at the
// store ROOT must NOT collapse the whole store into a single "." extension.
// Before the root-skip guard, classifyDir ran on the root (which contains
// index.ts → case c), computed relTag = filepath.Rel(root, root) = ".", and
// returned SkipDir — pruning the ENTIRE subtree so every real extension
// vanished. After the guard, the root is always descended and sub/x is
// discovered normally.
func TestIndexRootIndexTSDoesNotCollapse(t *testing.T) {
	root := t.TempDir()
	// A stray index.ts at the ROOT (the footgun). Must NOT make the root an extension.
	writeFile(t, root, "index.ts", "/** root scratch file. */\n")
	// A real extension nested one level down. Must survive.
	writeFile(t, filepath.Join(root, "sub"), "x.ts", "/** sub extension. */\n")

	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v; want nil (valid store; root index.ts is not an error)", err)
	}

	// Exactly ONE entry — the real extension. NOT zero (whole-store collapse) and
	// NOT one-with-tag-"." (the collapsed-store signature).
	if len(got) != 1 {
		t.Fatalf("len(got)=%d; want 1 (root index.ts must not collapse the store). entries=%v",
			len(got), relTags(got))
	}
	if got[0].RelTag != "sub/x" {
		t.Errorf("RelTag=%q; want 'sub/x' (the real extension under the root)", got[0].RelTag)
	}

	// The bug's signature: an entry tagged ".". Guard against it explicitly so a
	// future regression is caught with a clear message, not just a wrong count.
	for _, e := range got {
		if e.RelTag == "." {
			t.Errorf("found a '.' entry (the root-collapse signature); entries=%v", relTags(got))
		}
	}
}

// --- PRD Issue 6: node_modules, .git, and hidden entries are skipped (P1.M2.T2.S1) ---

// TestIndexSkipsNodeModules reproduces Issue 6: a top-level node_modules/ (from
// `npm install` at the store root) must NOT contribute its nested packages as
// extensions. The WalkDir callback prunes node_modules via filepath.SkipDir.
func TestIndexSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "myext.ts", "/** my ext. */\n")
	// A nested npm package — must NOT become an extension.
	writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "index.js",
		"/** dep. */\nexport default function(){}\n")
	writeFile(t, filepath.Join(root, "node_modules", "somepkg"), "package.json",
		`{"name":"somepkg"}`)
	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	tags := relTags(got)
	if len(got) != 1 || got[0].RelTag != "myext" {
		t.Fatalf("got=%v; want exactly one extension 'myext' (node_modules pruned)", tags)
	}
	// Guard against the spurious entry by name.
	for _, tag := range tags {
		if strings.HasPrefix(tag, "node_modules/") {
			t.Errorf("node_modules leaked into catalog: %q (must be pruned)", tag)
		}
	}
}

// TestIndexSkipsHiddenFile reproduces Issue 6: a hidden file (.secret.ts) must
// NOT become an extension. The file-skip returns nil (NOT SkipDir), so the
// sibling myext.ts is STILL discovered.
func TestIndexSkipsHiddenFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "myext.ts", "/** my ext. */\n")
	writeFile(t, root, ".secret.ts", "/** secret. */\nexport default function(){}\n")
	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	tags := relTags(got)
	if len(got) != 1 || got[0].RelTag != "myext" {
		t.Fatalf("got=%v; want exactly one extension 'myext' (.secret.ts skipped, myext.ts kept)", tags)
	}
	for _, tag := range tags {
		if strings.HasPrefix(tag, ".") {
			t.Errorf("hidden file leaked into catalog: %q", tag)
		}
	}
}

// TestIndexSkipsGitDir reproduces Issue 6: a .git/ directory (git internals)
// must NOT contribute its artifacts as extensions. Pruned via SkipDir.
func TestIndexSkipsGitDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "myext.ts", "/** my ext. */\n")
	// A .ts-like artifact inside .git/ — must NOT become an extension.
	writeFile(t, filepath.Join(root, ".git", "hooks"), "some.ts", "// git hook\n")
	got, err := Index(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	tags := relTags(got)
	if len(got) != 1 || got[0].RelTag != "myext" {
		t.Fatalf("got=%v; want exactly one extension 'myext' (.git pruned)", tags)
	}
	for _, tag := range tags {
		if strings.HasPrefix(tag, ".git/") || strings.HasPrefix(tag, ".git") {
			t.Errorf(".git leaked into catalog: %q", tag)
		}
	}
}
