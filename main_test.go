package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// unsetExtEnv neutralizes the two env-based extdir rules (weave_EXTENSIONS_DIR
// and weave_CONFIG, both LOWERCASE) so Find() falls through to the cwd/sibling
// rules. Required for hermetic --path failure + precedence tests: without it, a
// dev machine's real config (~/.config/weave/config.yaml) leaks a real dir and
// the test prints it instead of ErrNotFound.
//
// It points BOTH env vars at non-existent ghost paths in a fresh temp dir so
// rule 1 (env var) and rule 2 (config file) deterministically miss. t.Setenv
// restores the original values on cleanup automatically — no manual cleanup.
//
// Do NOT call t.Parallel() on a test that uses unsetExtEnv: t.Setenv mutates
// process-global env and forbids t.Parallel.
func unsetExtEnv(t *testing.T) {
	t.Helper()
	t.Setenv("weave_EXTENSIONS_DIR", filepath.Join(t.TempDir(), "no-ext")) // rule 1 misses (dir doesn't exist)
	t.Setenv("weave_CONFIG", filepath.Join(t.TempDir(), "no-config.yaml")) // rule 2 misses (file doesn't exist) — LOWERCASE
}

// --- parseArgs matrix ---

func TestParseArgsEmpty(t *testing.T) {
	c := parseArgs(nil)
	if c.version || c.path {
		t.Errorf("parseArgs(nil): version=%v path=%v; want both false", c.version, c.path)
	}
}

func TestParseArgsVersionLong(t *testing.T) {
	c := parseArgs([]string{"--version"})
	if !c.version || c.path {
		t.Errorf("parseArgs(--version): version=%v path=%v; want true,false", c.version, c.path)
	}
}

func TestParseArgsVersionShort(t *testing.T) {
	c := parseArgs([]string{"-v"})
	if !c.version {
		t.Errorf("parseArgs(-v): version=false; want true")
	}
}

func TestParseArgsPathLong(t *testing.T) {
	c := parseArgs([]string{"--path"})
	if !c.path || c.version {
		t.Errorf("parseArgs(--path): path=%v version=%v; want true,false", c.path, c.version)
	}
}

func TestParseArgsPathShort(t *testing.T) {
	c := parseArgs([]string{"-p"})
	if !c.path {
		t.Errorf("parseArgs(-p): path=false; want true")
	}
}

// Flags may appear in any order (PRD §6); both long+short forms recognized.
func TestParseArgsAnyOrderBothForms(t *testing.T) {
	c := parseArgs([]string{"-p", "--version"})
	if !c.version || !c.path {
		t.Errorf("parseArgs(-p --version): version=%v path=%v; want true,true", c.version, c.path)
	}
}

// Unknown dashed flags are captured into c.unknownFlag (the exit-2 behavior lands
// in M5.T1.S1; this milestone only PARSES them). The FIRST unknown offender wins;
// non-dashed positionals are still captured as <tag>s. `check`/`init` are
// RESERVED subcommands and are NOT captured as tags, so they are deliberately
// excluded from this positional-capture test.
func TestParseArgsUnknownFlagCaptured(t *testing.T) {
	c := parseArgs([]string{"--frobnicate", "sometag", "othertag"})
	if c.version || c.path {
		t.Errorf("parseArgs(unknown): version=%v path=%v; want both false", c.version, c.path)
	}
	if c.unknownFlag != "--frobnicate" {
		t.Errorf("unknownFlag=%q; want --frobnicate (first unknown captured)", c.unknownFlag)
	}
	// Non-dashed positionals are captured as tags; the dashed --frobnicate is excluded.
	if len(c.tags) != 2 || c.tags[0] != "sometag" || c.tags[1] != "othertag" {
		t.Errorf("parseArgs tags=%v; want [sometag othertag] (positionals captured)", c.tags)
	}
}

func TestParseArgsListLong(t *testing.T) {
	c := parseArgs([]string{"--list"})
	if !c.list || c.version || c.path {
		t.Errorf("parseArgs(--list): list=%v; want true (others false)", c.list)
	}
}

func TestParseArgsListShort(t *testing.T) {
	c := parseArgs([]string{"-l"})
	if !c.list {
		t.Errorf("parseArgs(-l): list=false; want true")
	}
}

func TestParseArgsNoColor(t *testing.T) {
	c := parseArgs([]string{"--no-color"})
	if !c.noColor {
		t.Errorf("parseArgs(--no-color): noColor=false; want true")
	}
}

// --- parseArgs: positional <tag> capture ---

// Positional <tag> args (non-dashed tokens) are captured in INPUT order (PRD §6.1).
func TestParseArgsCapturesTagsInOrder(t *testing.T) {
	c := parseArgs([]string{"foo", "writing/reddit"})
	if len(c.tags) != 2 || c.tags[0] != "foo" || c.tags[1] != "writing/reddit" {
		t.Errorf("tags=%v; want [foo writing/reddit] in input order", c.tags)
	}
}

// Dashed unknowns are NOT tags; only the positional is captured. The FIRST
// unknown offender wins (--frobnicate before -x).
func TestParseArgsDashedUnknownNotATag(t *testing.T) {
	c := parseArgs([]string{"--frobnicate", "real-tag", "-x"})
	if len(c.tags) != 1 || c.tags[0] != "real-tag" {
		t.Errorf("tags=%v; want [real-tag] (dashed tokens excluded)", c.tags)
	}
	if c.unknownFlag != "--frobnicate" {
		t.Errorf("unknownFlag=%q; want --frobnicate (first of two unknowns wins)", c.unknownFlag)
	}
}

// Tags and recognized flags may interleave (PRD §6: flags appear in any order).
func TestParseArgsTagsAndFlagsInterleave(t *testing.T) {
	c := parseArgs([]string{"--no-color", "a", "-l", "b"})
	if !c.list || !c.noColor || len(c.tags) != 2 || c.tags[0] != "a" || c.tags[1] != "b" {
		t.Errorf("config=%+v; want list+noColor true and tags=[a b]", c)
	}
}

// --- parseArgs: modifiers --file/-f, --relative, --all/-a ---

// --file/-f sets c.file (long and short forms, PRD §6.2).
func TestParseArgsFileLong(t *testing.T) {
	c := parseArgs([]string{"--file"})
	if !c.file {
		t.Errorf("parseArgs(--file): file=false; want true")
	}
}

func TestParseArgsFileShort(t *testing.T) {
	c := parseArgs([]string{"-f"})
	if !c.file {
		t.Errorf("parseArgs(-f): file=false; want true")
	}
}

// --relative has NO short form (PRD §6.2 lists only the long form).
func TestParseArgsRelativeLong(t *testing.T) {
	c := parseArgs([]string{"--relative"})
	if !c.relative {
		t.Errorf("parseArgs(--relative): relative=false; want true")
	}
}

// --all/-a sets c.all (long and short forms, PRD §6.1).
func TestParseArgsAllLong(t *testing.T) {
	c := parseArgs([]string{"--all"})
	if !c.all {
		t.Errorf("parseArgs(--all): all=false; want true")
	}
}

func TestParseArgsAllShort(t *testing.T) {
	c := parseArgs([]string{"-a"})
	if !c.all {
		t.Errorf("parseArgs(-a): all=false; want true")
	}
}

// Modifiers may interleave with tags and other flags (PRD §6 any order).
func TestParseArgsModifiersInterleave(t *testing.T) {
	c := parseArgs([]string{"-f", "example", "--relative"})
	if !c.file || !c.relative || len(c.tags) != 1 || c.tags[0] != "example" {
		t.Errorf("config=%+v; want file+relative true and tags=[example]", c)
	}
}

// --- parseArgs: --search/-s value flag ---

// --search <q> sets searchMode=true and captures the query; the value is NOT a tag.
func TestParseArgsSearchLong(t *testing.T) {
	c := parseArgs([]string{"--search", "reddit"})
	if !c.searchMode || c.searchQ != "reddit" {
		t.Errorf("parseArgs(--search reddit): mode=%v q=%q; want true,reddit", c.searchMode, c.searchQ)
	}
	if len(c.tags) != 0 {
		t.Errorf("--search value leaked into tags: %v", c.tags)
	}
}

// -s <q> short form behaves identically.
func TestParseArgsSearchShort(t *testing.T) {
	c := parseArgs([]string{"-s", "reddit"})
	if !c.searchMode || c.searchQ != "reddit" {
		t.Errorf("parseArgs(-s reddit): mode=%v q=%q; want true,reddit", c.searchMode, c.searchQ)
	}
}

// --search with NO following value (last token) -> searchMode stays false.
func TestParseArgsSearchNoValueStaysInactive(t *testing.T) {
	c := parseArgs([]string{"--search"})
	if c.searchMode {
		t.Errorf("parseArgs(--search) with no value: searchMode=true; want false (no value consumed)")
	}
}

// --search consumes exactly ONE value; a following positional is captured as a tag.
func TestParseArgsSearchConsumesOneValue(t *testing.T) {
	c := parseArgs([]string{"--search", "q", "sometag"})
	if !c.searchMode || c.searchQ != "q" {
		t.Errorf("search not captured: mode=%v q=%q", c.searchMode, c.searchQ)
	}
	if len(c.tags) != 1 || c.tags[0] != "sometag" {
		t.Errorf("tags=%v; want [sometag] (the token after the search value)", c.tags)
	}
}

// --search=<q> '='-form captures the value after '='.
func TestParseArgsSearchEqualsForm(t *testing.T) {
	c := parseArgs([]string{"--search=reddit"})
	if !c.searchMode || c.searchQ != "reddit" {
		t.Errorf("parseArgs(--search=reddit): mode=%v q=%q; want true,reddit", c.searchMode, c.searchQ)
	}
}

// --- parseArgs: short bundles (-vpl, -sfoo, -ls foo) ---

// A bool short bundle sets every flag in it (here -vpl -> version+path+list).
func TestParseArgsShortBundleBools(t *testing.T) {
	c := parseArgs([]string{"-vpl"})
	if !c.version || !c.path || !c.list {
		t.Errorf("parseArgs(-vpl): version=%v path=%v list=%v; want true,true,true", c.version, c.path, c.list)
	}
}

// -s with an embedded value ("-sfoo") captures "foo" as the search query.
func TestParseArgsShortBundleSearchEmbedded(t *testing.T) {
	c := parseArgs([]string{"-sfoo"})
	if !c.searchMode || c.searchQ != "foo" {
		t.Errorf("parseArgs(-sfoo): mode=%v q=%q; want true,foo", c.searchMode, c.searchQ)
	}
}

// -ls <q>: bool flags before 's' are set, and 's' consumes the NEXT argv token.
func TestParseArgsShortBundleSearchNextToken(t *testing.T) {
	c := parseArgs([]string{"-ls", "foo"})
	if !c.list {
		t.Errorf("parseArgs(-ls foo): list=false; want true (bool before s)")
	}
	if !c.searchMode || c.searchQ != "foo" {
		t.Errorf("parseArgs(-ls foo): mode=%v q=%q; want true,foo (next token)", c.searchMode, c.searchQ)
	}
}

// An unknown char in a bundle REJECTS the whole bundle: NOTHING is applied, and
// the whole bundle is captured as unknownFlag (two-phase validate-then-commit).
func TestParseArgsShortBundleUnknownCharRejectsAll(t *testing.T) {
	c := parseArgs([]string{"-vz"})
	if c.version {
		t.Errorf("parseArgs(-vz): version=true; want false (bundle rejected wholesale, nothing applied)")
	}
	if c.unknownFlag != "-vz" {
		t.Errorf("parseArgs(-vz): unknownFlag=%q; want -vz (whole bundle captured)", c.unknownFlag)
	}
}

// --- parseArgs: `check` subcommand ---

// The bare token "check" selects the check subcommand and is NOT captured as a tag.
func TestParseArgsCheckSubcommand(t *testing.T) {
	c := parseArgs([]string{"check"})
	if !c.check {
		t.Errorf("parseArgs(check): check=false; want true")
	}
	if len(c.tags) != 0 {
		t.Errorf("parseArgs(check): tags=%v; want empty ('check' is a subcommand, not a tag)", c.tags)
	}
}

// `check` is recognized even when it follows a flag (--no-color check).
func TestParseArgsCheckAfterFlag(t *testing.T) {
	c := parseArgs([]string{"--no-color", "check"})
	if !c.check {
		t.Errorf("parseArgs(--no-color check): check=false; want true")
	}
	if !c.noColor {
		t.Errorf("parseArgs(--no-color check): noColor=false; want true (flag still parsed)")
	}
}

// --- parseArgs: `init` subcommand + `--store` ---

// `init` alone is a RESERVED subcommand (like `check`): sets c.init and is NOT
// captured as a tag.
func TestParseArgsInitSubcommand(t *testing.T) {
	c := parseArgs([]string{"init"})
	if !c.init {
		t.Errorf("parseArgs(init): init=false; want true")
	}
	if len(c.tags) != 0 {
		t.Errorf("parseArgs(init): tags=%v; want empty ('init' is a subcommand, not a tag)", c.tags)
	}
	if c.initStore != "" {
		t.Errorf("parseArgs(init): initStore=%q; want empty", c.initStore)
	}
}

// `init <dir>` captures the positional <dir> into c.initStore (NOT into tags).
func TestParseArgsInitPositionalDir(t *testing.T) {
	c := parseArgs([]string{"init", "/tmp/x"})
	if !c.init {
		t.Errorf("init not set")
	}
	if c.initStore != "/tmp/x" {
		t.Errorf("initStore=%q; want /tmp/x", c.initStore)
	}
	if len(c.tags) != 0 {
		t.Errorf("tags=%v; want empty (dir consumed as store, not a tag)", c.tags)
	}
}

// `init --store <dir>` long form: --store fills initStore (init already set).
func TestParseArgsInitStoreLongForm(t *testing.T) {
	c := parseArgs([]string{"init", "--store", "/tmp/x"})
	if !c.init {
		t.Errorf("init not set")
	}
	if c.initStore != "/tmp/x" {
		t.Errorf("initStore=%q; want /tmp/x", c.initStore)
	}
	if len(c.tags) != 0 {
		t.Errorf("tags=%v; want empty", c.tags)
	}
}

// `init --store=<dir>` '='-form: --store fills initStore.
func TestParseArgsInitStoreEqualsForm(t *testing.T) {
	c := parseArgs([]string{"init", "--store=/tmp/x"})
	if !c.init {
		t.Errorf("init not set")
	}
	if c.initStore != "/tmp/x" {
		t.Errorf("initStore=%q; want /tmp/x", c.initStore)
	}
}

// `--store <dir>` with NO `init` token still implies init.
func TestParseArgsStoreWithoutInitToken(t *testing.T) {
	c := parseArgs([]string{"--store", "/tmp/x"})
	if !c.init {
		t.Errorf("--store should set init=true; got false")
	}
	if c.initStore != "/tmp/x" {
		t.Errorf("initStore=%q; want /tmp/x", c.initStore)
	}
	if len(c.tags) != 0 {
		t.Errorf("tags=%v; want empty", c.tags)
	}
}

// --- run: --version / -v ---

func TestRunVersionPrintsWeaveVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--version): code=%d; want 0", code)
	}
	want := "weave " + version + "\n" // version == "dev" under `go test` (no ldflags)
	if got := out.String(); got != want {
		t.Errorf("run(--version) stdout=%q; want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--version) stderr=%q; want empty", errOut.String())
	}
}

func TestRunVersionShortFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"-v"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-v): code=%d; want 0", code)
	}
	if !strings.HasPrefix(out.String(), "weave ") {
		t.Errorf("run(-v) stdout=%q; want 'weave <version>\\n'", out.String())
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Errorf("run(-v) stdout=%q; want trailing newline", out.String())
	}
}

// --- run: --path / -p ---

// --path success: weave_EXTENSIONS_DIR set to an existing dir -> rule 1 wins,
// Find() returns that dir, printed byte-exact to stdout, exit 0. The source
// label "(found via weave_EXTENSIONS_DIR)" goes to stderr (Issue 1).
func TestRunPathSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("weave_EXTENSIONS_DIR", dir) // rule 1 wins deterministically
	var out, errOut bytes.Buffer
	code := run([]string{"--path"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--path) success: code=%d; want 0", code)
	}
	// Find() cleans the env value via filepath.Abs, so compare to the cleaned form.
	want := filepath.Clean(dir) + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(--path) stdout=%q; want %q (byte-exact dir + newline)", got, want)
	}
	if got, want := errOut.String(), "(found via weave_EXTENSIONS_DIR)\n"; got != want {
		t.Errorf("run(--path) success stderr=%q; want %q (Issue 1 source label)", got, want)
	}
}

func TestRunPathShortFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-p"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-p): code=%d; want 0", code)
	}
	if got := out.String(); got != filepath.Clean(dir)+"\n" {
		t.Errorf("run(-p) stdout=%q; want %q", got, filepath.Clean(dir)+"\n")
	}
	if got, want := errOut.String(), "(found via weave_EXTENSIONS_DIR)\n"; got != want {
		t.Errorf("run(-p) stderr=%q; want %q (Issue 1 source label)", got, want)
	}
}

// Issue 1 (QA): --path must report which §8 rule won to stderr, while stdout
// stays byte-exact so the §13 `test "$(weave --path)" = ...` gate still passes.
// The env case is deterministic; sibling/walk-up are covered by extdir's
// TestSourceString.
func TestRunPathReportsSourceLabel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("weave_EXTENSIONS_DIR", dir) // rule 1 wins -> SourceEnv
	var out, errOut bytes.Buffer
	if code := run([]string{"--path"}, &out, &errOut); code != 0 {
		t.Fatalf("run(--path): code=%d; want 0", code)
	}
	// stdout: byte-exact dir + newline (§13 contract preserved).
	if got, want := out.String(), filepath.Clean(dir)+"\n"; got != want {
		t.Errorf("--path stdout=%q; want %q", got, want)
	}
	// stderr: the SourceEnv label, exactly, nothing else.
	if got, want := errOut.String(), "(found via weave_EXTENSIONS_DIR)\n"; got != want {
		t.Errorf("--path stderr=%q; want %q", got, want)
	}
}

// --path failure: env unset + cwd in an empty temp tree -> all §8.3 rules
// miss -> Find() returns ErrNotFound. Assert: exit 1, stdout EMPTY, stderr has
// the one-line fix (run `weave init`). Empty stdout is the §6.4 contract that
// makes `pi -e "$(weave bad)"` fail loudly.
func TestRunPathFailureErrNotFound(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // empty tree -> rule 3 (sibling) + rule 4 (walk-up) miss
	var out, errOut bytes.Buffer
	code := run([]string{"--path"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--path) failure: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(--path) failure stdout=%q; want EMPTY (§6.4: print nothing on failure)", out.String())
	}
	msg := errOut.String()
	for _, want := range []string{"run", "weave init"} {
		if !strings.Contains(msg, want) {
			t.Errorf("run(--path) failure stderr=%q; missing substring %q", msg, want)
		}
	}
}

// --- run: precedence ---

// --version takes precedence over --path (PRD §6.3): version printed, Find()
// never called, exit 0 — even though the extensions dir is unresolvable here.
func TestRunVersionPrecedenceOverPath(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // would make --path fail, but --version wins first
	var out, errOut bytes.Buffer
	code := run([]string{"--path", "--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--path --version): code=%d; want 0 (version precedence)", code)
	}
	want := "weave " + version + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(--path --version) stdout=%q; want %q (version, not path)", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--path --version) stderr=%q; want empty", errOut.String())
	}
}

// --- run: no-op modes (this milestone) ---

// A parsed mode that is NOT yet dispatched (e.g. --list, <tag>) is a no-op in
// this milestone: exit 0, no output. M2/M3/M4/M5 add the dispatch branches.
// This pins the "parser complete, dispatch trimmed" contract so a later
// milestone that accidentally changes the no-op shape is caught.
func TestRunUndispatchedModeIsNoOp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)
	if code != 0 {
		t.Errorf("run(--list): code=%d; want 0 (no-op until M2.T5.S1)", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(--list) stdout=%q; want empty (no-op)", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--list) stderr=%q; want empty (no-op)", errOut.String())
	}
}

// No args at all is also a no-op in this milestone (the no-args → usage → exit 1
// path lands in M5.T1.S1). A bare `weave` currently exits 0.
func TestRunNoArgsIsNoOp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(nil, &out, &errOut)
	if code != 0 {
		t.Errorf("run(nil): code=%d; want 0 (no-args → usage lands in M5.T1.S1)", code)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("run(nil) produced output; want none (no-op): stdout=%q stderr=%q", out.String(), errOut.String())
	}
}
