package extdir

import (
	"os"
	"path/filepath"
	"testing"
)

// unsetEnvVar removes envVar for the duration of the test and restores the
// prior state on cleanup. Needed because t.Setenv can only set, never unset.
// Do NOT call t.Parallel() in any test that uses this or t.Setenv — mutates env.
//
// ALSO neutralizes the config-file rule (PRD §8.3 rule 2): points weave_CONFIG
// at a non-existent path so findConfig deterministically misses in all-miss /
// env-only tests. Without this, a machine with a real config
// (~/.config/weave/config.yaml, or weave_CONFIG set) would make the new
// findConfig tests that expect a miss leak a real dir. Mirrors skilldozer's
// unsetEnvVar (which does the same for SKILLDOZER_CONFIG).
func unsetEnvVar(tb testing.TB) {
	tb.Helper()
	prev, had := os.LookupEnv(envVar)
	if err := os.Unsetenv(envVar); err != nil {
		tb.Fatalf("unsetenv %s: %v", envVar, err)
	}
	tb.Cleanup(func() {
		if had {
			_ = os.Setenv(envVar, prev)
		} else {
			_ = os.Unsetenv(envVar)
		}
	})
	// NEW in S2: neutralize the config rule (PRD §8.3 rule 2). weave_CONFIG is
	// LOWERCASE per config.go const configEnv; os.Setenv is case-sensitive on Linux.
	cfgGhost := filepath.Join(tb.TempDir(), "no-config.yaml")
	prevCfg, hadCfg := os.LookupEnv("weave_CONFIG")
	if err := os.Setenv("weave_CONFIG", cfgGhost); err != nil {
		tb.Fatalf("setenv weave_CONFIG: %v", err)
	}
	tb.Cleanup(func() {
		if hadCfg {
			_ = os.Setenv("weave_CONFIG", prevCfg)
		} else {
			_ = os.Unsetenv("weave_CONFIG")
		}
	})
}

// makeFakeBinary creates a regular file at dir/name to stand in for a compiled
// binary. EvalSymlinks + os.Stat(Join(dir,"extensions")) do not require a real
// ELF executable, so a 1-byte file is sufficient (skilldozer research §5:
// verified_facts.md §5). Used by the resolveSiblingFromExe tests.
func makeFakeBinary(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake binary %s: %v", p, err)
	}
	return p
}

// writeCfg writes content to a temp config.yaml, sets weave_CONFIG to it, and
// returns (cfgPath, cfgDir). Helper for the findConfig tests. Mirrors the
// writeCfg idiom from skilldozer and internal/config/config_test.go.
func writeCfg(t *testing.T, content string) (cfgPath, cfgDir string) {
	t.Helper()
	cfgDir = t.TempDir()
	cfgPath = filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", cfgPath, err)
	}
	t.Setenv("weave_CONFIG", cfgPath)
	return cfgPath, cfgDir
}

func TestSourceString(t *testing.T) {
	cases := []struct {
		src  Source
		want string
	}{
		{SourceEnv, "weave_EXTENSIONS_DIR"},
		{SourceConfig, "config file"},
		{SourceSibling, "sibling of binary"},
		{SourceWalkUp, "ancestor of cwd"},
		{Source(-1), "unknown"}, // out-of-range -> default
		{Source(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.src.String(); got != c.want {
			t.Errorf("Source(%d).String() = %q, want %q", c.src, got, c.want)
		}
	}
}

// Rule 1: env unset -> not found (fall through, no error).
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvUnset(t *testing.T) {
	unsetEnvVar(t)
	dir, src, found := findEnv()
	if found {
		t.Errorf("findEnv() env unset: got found=true dir=%q src=%v; want found=false", dir, src)
	}
}

// Rule 1: env set to "" -> not found (treated as no usable dir).
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvEmpty(t *testing.T) {
	t.Setenv(envVar, "")
	dir, src, found := findEnv()
	if found {
		t.Errorf("findEnv() env empty: got found=true dir=%q src=%v; want found=false", dir, src)
	}
}

// Rule 1: env set to an existing directory (absolute temp dir) -> found, abs path, SourceEnv.
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvExistingDir(t *testing.T) {
	dir := t.TempDir() // already absolute + clean
	t.Setenv(envVar, dir)
	got, src, found := findEnv()
	if !found {
		t.Fatalf("findEnv() existing dir: found=false; want true")
	}
	if src != SourceEnv {
		t.Errorf("findEnv() existing dir: src=%v; want SourceEnv", src)
	}
	if want := filepath.Clean(dir); got != want {
		t.Errorf("findEnv() existing dir: dir=%q; want %q", got, want)
	}
}

// Rule 1: env set to a path that does not exist -> not found (fall through).
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvNonexistent(t *testing.T) {
	t.Setenv(envVar, filepath.Join(t.TempDir(), "does-not-exist"))
	dir, src, found := findEnv()
	if found {
		t.Errorf("findEnv() nonexistent: got found=true dir=%q src=%v; want found=false", dir, src)
	}
}

// Rule 1: env set to a regular file (not a directory) -> not found (fall through).
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvRegularFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envVar, f)
	dir, src, found := findEnv()
	if found {
		t.Errorf("findEnv() regular file: got found=true dir=%q src=%v; want found=false", dir, src)
	}
}

// Rule 1: env set to a RELATIVE existing dir (".") -> found, absolutized via filepath.Abs.
// Proves the filepath.Abs (relative->absolute) path without chdir or cwd pollution.
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvRelativePathAbsolutized(t *testing.T) {
	t.Setenv(envVar, ".") // "." always exists and is a dir (the test's cwd)
	got, src, found := findEnv()
	if !found {
		t.Fatalf("findEnv() relative '.': found=false; want true")
	}
	if src != SourceEnv {
		t.Errorf("findEnv() relative '.': src=%v; want SourceEnv", src)
	}
	want, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("findEnv() relative '.': dir=%q; want absolutized %q", got, want)
	}
}

// Rule 1 CONTRACT: the env path must NOT be passed through EvalSymlinks.
// If weave_EXTENSIONS_DIR points at a symlink-to-a-dir, findEnv must return the
// symlink path (made absolute/clean), NOT the resolved target. The user points
// exactly where they want.
// Do NOT call t.Parallel() — mutates env.
func TestFindEnvDoesNotResolveSymlinks(t *testing.T) {
	realDir := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "link-to-ext")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}
	t.Setenv(envVar, link)
	got, src, found := findEnv()
	if !found {
		t.Fatalf("findEnv() symlink-to-dir: found=false; want true (os.Stat follows the symlink)")
	}
	if src != SourceEnv {
		t.Errorf("findEnv() symlink-to-dir: src=%v; want SourceEnv", src)
	}
	if got == realDir {
		t.Errorf("findEnv() symlink-to-dir: dir=%q == resolved target; must NOT EvalSymlinks the env path", got)
	}
	want, err := filepath.Abs(link)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("findEnv() symlink-to-dir: dir=%q; want symlink path (absolutized) %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// HasExtensionEntry tests (NEW for weave — skilldozer has HasSkillMD analogs but
// the recognition logic differs: weave recognizes 3 entry kinds per PRD §7.1).
// ---------------------------------------------------------------------------

// Case (a): a single-file .ts extension nested at depth → true.
func TestHasExtensionEntryFoundNestedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "foo.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(nested foo.ts): got false; want true (WalkDir recurses, case a)")
	}
}

// Case (a): a single-file .ts extension at shallow depth → true.
func TestHasExtensionEntryFoundShallowFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gate.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(shallow gate.ts): got false; want true (case a)")
	}
}

// Case (a) variant: a single-file .js extension → true.
func TestHasExtensionEntryFoundShallowJSFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gate.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(shallow gate.js): got false; want true (case a, .js variant)")
	}
}

// Case (b): a directory directly containing index.ts → true.
func TestHasExtensionEntryFoundJSDir(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "index.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(pkg/index.ts): got false; want true (case b)")
	}
}

// Case (b) variant: a directory directly containing index.js → true.
func TestHasExtensionEntryFoundJSIndexJs(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "index.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(pkg/index.js): got false; want true (case b, .js variant)")
	}
}

// Case (c): a directory with package.json whose pi.extensions names ≥1 EXISTING entry → true.
func TestHasExtensionEntryFoundPackageJSON(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	src := filepath.Join(pkg, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "package.json"),
		[]byte(`{"name":"summarizer","pi":{"extensions":["./src/index.ts"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(package.json + existing entry): got false; want true (case c)")
	}
}

// Case (c) CONTRACT: pi.extensions naming a NON-existing entry → false (mirrors
// pi's own resolver: only entries that actually exist on disk are counted).
func TestHasExtensionEntryPackageJSONNoExistingEntry(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	// pi.extensions points at ./missing.ts, which does NOT exist on disk.
	if err := os.WriteFile(filepath.Join(pkg, "package.json"),
		[]byte(`{"name":"x","pi":{"extensions":["./missing.ts"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(package.json + missing entry): got true; want false (case c requires ≥1 existing entry)")
	}
}

// Case (c) CONTRACT: package.json with no pi field → false (and no index.* and no *.ts/*.js).
func TestHasExtensionEntryPackageJSONNoPiField(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "package.json"),
		[]byte(`{"name":"x"}`), 0o644); err != nil { // no pi field
		t.Fatal(err)
	}
	if HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(package.json, no pi): got true; want false")
	}
}

// Case (c) robustness: malformed package.json (not valid JSON) → false, not a panic.
func TestHasExtensionEntryPackageJSONMalformed(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "package.json"),
		[]byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(malformed package.json): got true; want false")
	}
}

// Negative: empty dir → false.
func TestHasExtensionEntryEmptyDir(t *testing.T) {
	if HasExtensionEntry(t.TempDir()) {
		t.Errorf("HasExtensionEntry(empty dir): got true; want false")
	}
}

// Negative: only non-entry files (e.g. README.md) → false.
func TestHasExtensionEntryOnlyNonEntryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(only README.md): got true; want false")
	}
}

// WalkDir visits root first: if root itself is resolvable (root/index.ts exists),
// HasExtensionEntry returns true immediately (case b applies to the root dir).
func TestHasExtensionEntryRootItselfIsResolvable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(root has index.ts): got false; want true (WalkDir visits root first, case b)")
	}
}

// Case (a) EXCLUSION documented: index.ts/index.js are dir markers, NOT single-file
// entries. A dir containing ONLY index.ts is still resolvable via case (b) → true.
// The case-(a) EXCLUSION of index.* matters for Index() (P1.M2.T2, which classifies
// the entry kind), NOT for this predicate: the predicate just reports "any entry
// exists", and a dir with index.ts IS an entry via case (b). This test pins that a
// subdir whose only .ts file is an index.ts still qualifies the dir via case (b).
func TestHasExtensionEntryIndexAsDirMarker(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	// pkg has ONLY index.ts (no other .ts/.js, no package.json) -> resolvable via case (b).
	if err := os.WriteFile(filepath.Join(pkg, "index.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(pkg/index.ts only): got false; want true (case b — index.ts is a dir marker)")
	}
}

// Short-circuit sanity: a deep non-entry file and a shallow entry. The predicate
// returns true (the sentinel guarantees early exit once the entry is found; this
// test is informational — the structural guarantee is the WalkDir + sentinel shape).
func TestHasExtensionEntryShortCircuits(t *testing.T) {
	root := t.TempDir()
	// Shallow entry (case a).
	if err := os.WriteFile(filepath.Join(root, "shallow.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A deep tree of non-entry files that would be wasteful to walk in full.
	deep := filepath.Join(root, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := "readme" + string(rune('a'+i)) + ".md"
		if err := os.WriteFile(filepath.Join(deep, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !HasExtensionEntry(root) {
		t.Errorf("HasExtensionEntry(shallow entry + deep non-entry tree): got false; want true (short-circuits on the shallow entry)")
	}
}

// ---------------------------------------------------------------------------
// resolveSiblingFromExe tests (rule 3 testable core). Ported from skilldozer
// with renames: "skills" -> "extensions".
// ---------------------------------------------------------------------------

// Rule 3 CONTRACT: a symlink to the binary in a DIFFERENT dir must resolve back
// to the REAL binary's repo dir's extensions/. Mirrors
// architecture/verified_symlink_resolution.md and the PRD §12.1 symlink-install
// rationale (~/.local/bin/weave -> ~/projects/weave/weave resolves back to the repo).
func TestResolveSiblingFromExeSymlinkCrossDir(t *testing.T) {
	// tempA holds the REAL binary + its sibling extensions/
	tempA := t.TempDir()
	binary := makeFakeBinary(t, tempA, "weave")
	extA := filepath.Join(tempA, "extensions")
	if err := os.Mkdir(extA, 0o755); err != nil {
		t.Fatal(err)
	}
	// tempB holds a symlink to the binary (different dir, like ~/.local/bin)
	tempB := t.TempDir()
	link := filepath.Join(tempB, "weave")
	if err := os.Symlink(binary, link); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	got, found := resolveSiblingFromExe(link)
	if !found {
		t.Fatalf("resolveSiblingFromExe(symlink): found=false; want true")
	}
	if got != extA {
		t.Errorf("resolveSiblingFromExe(symlink): dir=%q; want the REAL binary's extensions %q", got, extA)
	}
	if filepath.Dir(got) != tempA {
		t.Errorf("resolveSiblingFromExe(symlink): resolved to %q, not the real binary's dir %q", filepath.Dir(got), tempA)
	}
}

// Rule 3: direct (non-symlinked) binary with a sibling extensions/ also wins.
func TestResolveSiblingFromExeDirect(t *testing.T) {
	tempA := t.TempDir()
	binary := makeFakeBinary(t, tempA, "weave")
	extA := filepath.Join(tempA, "extensions")
	if err := os.Mkdir(extA, 0o755); err != nil {
		t.Fatal(err)
	}
	got, found := resolveSiblingFromExe(binary)
	if !found {
		t.Fatalf("resolveSiblingFromExe(direct): found=false; want true")
	}
	if got != extA {
		t.Errorf("resolveSiblingFromExe(direct): dir=%q; want %q", got, extA)
	}
}

// Rule 3: EvalSymlinks-error fallback. A non-existent exe whose parent dir DOES
// have a sibling extensions/ must still win via real=exe. (Contract: 'if err, use
// exe as fallback.') Pinned by the item contract.
func TestResolveSiblingFromExeEvalSymlinksFallback(t *testing.T) {
	tempC := t.TempDir()
	extC := filepath.Join(tempC, "extensions")
	if err := os.Mkdir(extC, 0o755); err != nil {
		t.Fatal(err)
	}
	// 'ghost' binary does not exist -> EvalSymlinks errors -> fall back to exe.
	ghost := filepath.Join(tempC, "does-not-exist-binary")
	got, found := resolveSiblingFromExe(ghost)
	if !found {
		t.Fatalf("resolveSiblingFromExe(ghost): found=false; want true (EvalSymlinks fallback to exe)")
	}
	if got != extC {
		t.Errorf("resolveSiblingFromExe(ghost): dir=%q; want %q (Dir(exe)/extensions)", got, extC)
	}
}

// Rule 3: binary exists but NO sibling extensions/ dir -> miss.
func TestResolveSiblingFromExeNoExtensionsDir(t *testing.T) {
	tempA := t.TempDir()
	binary := makeFakeBinary(t, tempA, "weave")
	// deliberately create no extensions/ sibling
	if _, found := resolveSiblingFromExe(binary); found {
		t.Errorf("resolveSiblingFromExe(no extensions): got found=true; want false")
	}
}

// Rule 3: sibling path 'extensions' is a regular FILE, not a dir -> miss (IsDir guard).
func TestResolveSiblingFromExeExtensionsIsFile(t *testing.T) {
	tempA := t.TempDir()
	binary := makeFakeBinary(t, tempA, "weave")
	if err := os.WriteFile(filepath.Join(tempA, "extensions"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found := resolveSiblingFromExe(binary); found {
		t.Errorf("resolveSiblingFromExe(extensions is file): got found=true; want false (IsDir guard)")
	}
}

// ---------------------------------------------------------------------------
// findSibling tests (rule 3 entry; os.Executable exercised).
// ---------------------------------------------------------------------------

// Smoke test: the REAL test binary runs from a temp build dir (go-buildXXX)
// that has NO sibling extensions/, so findSibling must return found=false without
// panicking. This is the only deterministic assertion possible for findSibling
// (os.Executable cannot be controlled); the symlink logic is covered by the
// resolveSiblingFromExe tests above.
func TestFindSiblingNoExtensionsNextToTestBinary(t *testing.T) {
	dir, src, found := findSibling()
	if found {
		t.Errorf("findSibling(): got found=true dir=%q src=%v; want false (test binary's dir has no sibling extensions/)", dir, src)
	}
}

// ---------------------------------------------------------------------------
// findConfig tests (PRD §8.3 rule 2 / §8.1). Ported from skilldozer with renames:
// "SKILLDOZER_CONFIG" -> "weave_CONFIG".
// ---------------------------------------------------------------------------

// Rule 2 hit: an existing store dir named by an absolute `store` -> found,
// SourceConfig, cleaned absolute dir.
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigHit(t *testing.T) {
	store := t.TempDir() // already an existing, absolute dir
	writeCfg(t, "store: "+store+"\n")
	got, src, found := findConfig()
	if !found {
		t.Fatal("findConfig existing store: found=false; want true")
	}
	if src != SourceConfig {
		t.Errorf("src=%v; want SourceConfig", src)
	}
	if want := filepath.Clean(store); got != want {
		t.Errorf("dir=%q; want cleaned %q", got, want)
	}
}

// Rule 2 miss: config file does not exist -> fall through (never a hard error).
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigMissingFile(t *testing.T) {
	t.Setenv("weave_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if dir, src, found := findConfig(); found {
		t.Errorf("findConfig missing file: got found=true dir=%q src=%v; want false", dir, src)
	}
}

// Rule 2 miss: config file has no `store` key -> fall through.
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigMissingStoreKey(t *testing.T) {
	writeCfg(t, "foo: bar\n") // no `store:` key
	if dir, src, found := findConfig(); found {
		t.Errorf("findConfig no store key: got found=true dir=%q src=%v; want false", dir, src)
	}
}

// Rule 2 miss: `store` names a dir that does not exist -> fall through.
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigStoreDirAbsent(t *testing.T) {
	writeCfg(t, "store: "+filepath.Join(t.TempDir(), "no-such-store")+"\n")
	if dir, src, found := findConfig(); found {
		t.Errorf("findConfig absent store dir: got found=true dir=%q src=%v; want false", dir, src)
	}
}

// Rule 2 (weave variant): weave's config.Load is a hand-rolled line scanner, NOT
// yaml.v3, so there is NO "malformed syntax" hard-error case — a garbage-content
// file with no "store:" line returns File{Store: ""}, nil, and findConfig falls
// through via the f.Store == "" branch. This is equivalent to MissingStoreKey for
// weave's parser shape. (skilldozer had a hard-error case via yaml.v3; weave does not.)
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigMalformedSyntax(t *testing.T) {
	writeCfg(t, "store: [unclosed\n") // no parseable `store:` value -> File{Store:""}
	if dir, src, found := findConfig(); found {
		t.Errorf("findConfig malformed syntax: got found=true dir=%q src=%v; want false (fall through, no hard error)", dir, src)
	}
}

// Rule 2 (PRD §8.1): a relative `store` is resolved against the config FILE's dir,
// not against cwd. `store: mystore` in a config at <cfgDir>/config.yaml resolves to
// <cfgDir>/mystore.
// Do NOT call t.Parallel() — mutates env.
func TestFindConfigRelativeStoreResolvedAgainstConfigDir(t *testing.T) {
	cfgPath, cfgDir := writeCfg(t, "store: mystore\n")
	store := filepath.Join(cfgDir, "mystore")
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", store, err)
	}
	got, src, found := findConfig()
	if !found {
		t.Fatal("findConfig relative store: found=false; want true")
	}
	if src != SourceConfig {
		t.Errorf("src=%v; want SourceConfig", src)
	}
	if got != store {
		t.Errorf("dir=%q; want %q (joined to filepath.Dir(%q))", got, store, cfgPath)
	}
}
