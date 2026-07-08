package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
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

// withTerminal overrides the package-level isTerminal func for one test and
// restores it on cleanup. Use it to exercise the color-enabled path through
// run() without a real terminal. NOT t.Parallel-safe (mutates package state).
func withTerminal(t *testing.T, isTTY bool) {
	t.Helper()
	prev := isTerminal
	isTerminal = func(io.Writer) bool { return isTTY }
	t.Cleanup(func() { isTerminal = prev })
}

// writeExtTree writes a temporary extensions store: one single-file <tag>.ts
// extension per layout entry, each with a leading `/** ... */` JSDoc block so
// discover.ExtractJSDoc picks up the description. weave extensions are .ts FILES
// directly under the store root (PRD §7.1), NOT <tag>/SKILL.md directories as in
// skilldozer — a directory with no index.ts/package.json is NOT recognized by
// classifyDir, so it must be a .ts FILE. The JSDoc opener is exactly `/**` (two
// stars); a `//` or `/*` (one star) comment yields Description="".
func writeExtTree(t *testing.T, layout map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for tag, desc := range layout {
		content := "/** " + desc + " */\nexport default function() {}\n"
		path := filepath.Join(root, filepath.FromSlash(tag)+".ts")
		// MkdirAll the parent so a nested tag (e.g. "writing/reddit-poster")
		// creates its category dir; os.WriteFile alone fails on a missing parent.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir parent of %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
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

// No args at all is still a no-op in this milestone (the no-args → usage → exit 1
// path lands in M5.T1.S1). A bare `weave` currently exits 0. (M3.T2.S1 dispatched
// --all and <tag>, so those are no longer no-ops — they have dedicated test
// blocks below; only truly-unrecognized argv still falls through to return 0.)
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

// --- run: --list / -l (M2.T5.S1) ---

// --list success: a populated store renders a TAG/NAME/DESCRIPTION table on
// stdout, empty stderr (NO "(found via ...)" source label — that is --path-only),
// exit 0. A non-TTY *bytes.Buffer yields plain output (no ANSI) by default.
func TestRunListSuccess(t *testing.T) {
	dir := writeExtTree(t, map[string]string{
		"example": "A demo extension.",
	})
	t.Setenv("weave_EXTENSIONS_DIR", dir) // rule 1 wins; Find returns dir, Index finds the extension
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--list): code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"TAG", "NAME", "DESCRIPTION", "example", "A demo extension."} {
		if !strings.Contains(got, want) {
			t.Errorf("run(--list) stdout missing %q:\n%s", want, got)
		}
	}
	// Default (non-TTY buffer) -> no ANSI escapes.
	if strings.Contains(got, "\x1b[") {
		t.Errorf("run(--list) on a non-TTY must not emit ANSI:\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--list) stderr=%q; want empty (no source label for --list)", errOut.String())
	}
}

// -l short flag behaves identically to --list.
func TestRunListShortFlag(t *testing.T) {
	dir := writeExtTree(t, map[string]string{
		"example": "d",
	})
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-l"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-l): code=%d; want 0", code)
	}
	if !strings.Contains(out.String(), "example") {
		t.Errorf("run(-l) stdout missing the example tag:\n%s", out.String())
	}
}

// --list with NO extensions (empty store) -> PRD §6.1 exit 1, stdout empty, a
// message to stderr. weave_EXTENSIONS_DIR pointing at an existing-but-empty dir:
// rule 1 wins (it needs only an existing dir), Index returns [], len==0 -> exit 1.
func TestRunListNoExtensionsExit1(t *testing.T) {
	t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()) // exists, no .ts -> empty catalog
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--list) empty store: code=%d; want 1 (PRD §6.1 '1 if no extensions found')", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(--list) empty store stdout=%q; want empty (only the exit-1 + stderr msg)", out.String())
	}
	if !strings.Contains(errOut.String(), "no extensions found") {
		t.Errorf("run(--list) empty store stderr=%q; want a 'no extensions found' message", errOut.String())
	}
}

// --list when the extensions dir is unresolvable -> Find() returns ErrNotFound
// -> exit 1, stdout empty, the one-line fix to stderr (same contract as --path).
func TestRunListUnresolvableExit1(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // force all §8.3 rules to miss
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--list) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(--list) unresolvable stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("run(--list) unresolvable stderr=%q; want the one-line fix", errOut.String())
	}
}

// --list with --no-color suppresses ANSI even when stdout looks like a TTY.
// Forces isTerminal=true (so color WOULD be on by default) and asserts --no-color
// still yields plain output.
func TestRunListNoColorFlagSuppressesANSI(t *testing.T) {
	dir := writeExtTree(t, map[string]string{
		"example": "d",
	})
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	withTerminal(t, true) // pretend stdout is a TTY
	var out, errOut bytes.Buffer
	code := run([]string{"--list", "--no-color"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--list --no-color): code=%d; want 0", code)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("--no-color must suppress ANSI even on a TTY:\n%s", out.String())
	}
}

// --list color path: when stdout is a TTY (forced) and --no-color is absent, the
// table carries ANSI escapes. Proves the TTY gate is wired into run().
func TestRunListColorWhenTTY(t *testing.T) {
	dir := writeExtTree(t, map[string]string{
		"example": "d",
	})
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	withTerminal(t, true)
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--list) tty: code=%d; want 0", code)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[1m") || !strings.Contains(got, "\x1b[36m") || !strings.Contains(got, "\x1b[0m") {
		t.Errorf("TTY output should contain ANSI bold/cyan/reset:\n%s", got)
	}
}

// --- run: <tag> resolution (M3.T2.S1) ---

// sampleStore builds a store with a top-level `example` and a nested
// `writing/reddit-poster`, returning the extensions dir. Both are single-file
// .ts extensions (weave's layout — NOT <tag>/SKILL.md dirs as in skilldozer),
// so the default output is the FILE path (.../example.ts, .../writing/reddit-poster.ts).
// `reddit-poster` resolves by BASENAME (the final '/'-component of its RelTag).
func sampleStore(t *testing.T) string {
	t.Helper()
	return writeExtTree(t, map[string]string{
		"example":              "A demo extension.",
		"writing/reddit-poster": "Posts to reddit.",
	})
}

// writeDirExt writes a DIR extension: <tag>/index.ts with a leading JSDoc
// description. discover.classifyDir recognizes a dir with index.ts as Kind="dir";
// its Path is the DIR, EntryFile is dir+"/index.ts". This is the kind
// writeExtTree CANNOT build (it only writes single-file .ts extensions).
func writeDirExt(t *testing.T, root, tag, desc string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(tag))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "/** " + desc + " */\nexport default function() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "index.ts"), []byte(content), 0o644); err != nil {
		t.Fatalf("write index.ts: %v", err)
	}
	return dir
}

// writePkgExt writes a PACKAGE extension: <tag>/package.json with a pi.extensions
// array naming ./src/index.ts, plus <tag>/src/index.ts with a JSDoc description.
// discover.classifyDir recognizes this as Kind="package"; its Path is the DIR,
// EntryFile is the FIRST pi.extensions entry (src/index.ts). This is the kind
// writeExtTree CANNOT build (single-file only).
func writePkgExt(t *testing.T, root, tag, desc string) string {
	t.Helper()
	pkgDir := filepath.Join(root, filepath.FromSlash(tag))
	if err := os.MkdirAll(filepath.Join(pkgDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir %s/src: %v", pkgDir, err)
	}
	pkgJSON := `{"name": "` + filepath.Base(tag) + `", "pi": {"extensions": ["./src/index.ts"]}}`
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	content := "/** " + desc + " */\nexport default function() {}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "src", "index.ts"), []byte(content), 0o644); err != nil {
		t.Fatalf("write src/index.ts: %v", err)
	}
	return pkgDir
}

// Single tag resolves to its absolute .ts FILE path on stdout, exit 0, no stderr.
// weave's default output is the FILE path (single-file kind: Path==EntryFile==the
// .ts file), NOT a directory as in skilldozer.
func TestRunTagSingleResolvesToPath(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(example): code=%d; want 0", code)
	}
	want := filepath.Join(dir, "example.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(example) stdout=%q; want %q (absolute .ts file path + newline)", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(example) stderr=%q; want empty", errOut.String())
	}
}

// Multiple tags -> one path per line, in INPUT order (not sorted), exit 0.
// `reddit-poster` resolves by basename to writing/reddit-poster; `example` by
// canonical tag.
func TestRunTagMultipleInInputOrder(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"reddit-poster", "example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(reddit-poster example): code=%d; want 0", code)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 paths; got %d: %q", len(lines), out.String())
	}
	if lines[0] != filepath.Join(dir, "writing", "reddit-poster.ts") {
		t.Errorf("lines[0]=%q; want the reddit-poster .ts (input order preserved)", lines[0])
	}
	if lines[1] != filepath.Join(dir, "example.ts") {
		t.Errorf("lines[1]=%q; want the example .ts (input order preserved)", lines[1])
	}
}

// ATOMICITY (§6.4): one unknown tag among resolvable ones -> NOTHING on stdout,
// one stderr line per problem tag, exit 1. The resolvable tag must NOT leak to
// stdout (buffered paths are discarded on any failure).
func TestRunTagAtomicityUnknownPrintsNothing(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"example", "nope"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(example nope): code=%d; want 1 (atomic failure)", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (§6.4: nothing printed on failure)", out.String())
	}
	if !strings.Contains(errOut.String(), "nope") {
		t.Errorf("stderr=%q; want an error line naming 'nope'", errOut.String())
	}
}

// All tags fail -> one stderr line per problem tag, nothing on stdout, exit 1.
func TestRunTagAllFailMultipleErrorLines(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"nope1", "nope2"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(nope1 nope2): code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	errLines := strings.Split(strings.TrimRight(errOut.String(), "\n"), "\n")
	if len(errLines) != 2 {
		t.Fatalf("want 2 stderr lines (one per problem tag); got %d: %q", len(errLines), errOut.String())
	}
}

// A tag repeated in argv resolves each time; output repeats. Not an error.
func TestRunTagDuplicateArgResolvesTwice(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"example", "example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(example example): code=%d; want 0", code)
	}
	want := strings.Repeat(filepath.Join(dir, "example.ts")+"\n", 2)
	if got := out.String(); got != want {
		t.Errorf("stdout=%q; want two identical path lines:\n%s", got, want)
	}
}

// Ambiguous tag (basename collision) -> stderr lists the candidate full tags,
// NOTHING on stdout, exit 1 (PRD §6.4). The candidates come from resolve's
// *AmbiguousError verbatim (NO "weave:" prefix).
func TestRunTagAmbiguousListsCandidates(t *testing.T) {
	dir := writeExtTree(t, map[string]string{
		"writing/reddit": "d",
		"coding/reddit":  "d",
	})
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"reddit"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(reddit) ambiguous: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (ambiguous => nothing on stdout)", out.String())
	}
	msg := errOut.String()
	for _, want := range []string{"reddit", "coding/reddit", "writing/reddit"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr=%q; missing candidate %q", msg, want)
		}
	}
}

// Extensions dir unresolvable + tags -> exit 1, nothing on stdout, the one-line
// fix on stderr (same contract as --path/--list).
func TestRunTagUnresolvable(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // all §8.3 rules miss
	var out, errOut bytes.Buffer
	code := run([]string{"example"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(example) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}

// The resolved path is ABSOLUTE (PRD §6.1 default; --relative is tested below).
func TestRunTagPathIsAbsolute(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(example): code=%d; want 0", code)
	}
	if p := strings.TrimRight(out.String(), "\n"); !filepath.IsAbs(p) {
		t.Errorf("resolved path %q is not absolute (discover.Extension.Path should be absolute)", p)
	}
}

// --version precedes tag-resolution mode even when a tag is present (PRD §6.3).
func TestRunVersionPrecedenceOverTag(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"example", "--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(example --version): code=%d; want 0 (version precedence)", code)
	}
	if got := out.String(); got != "weave "+version+"\n" {
		t.Errorf("stdout=%q; want the version line (precedence over tag mode)", got)
	}
}

// --- run: <tag> + --file/--relative modifiers (M3.T2.S1) ---

// --file on a SINGLE-FILE extension is a NO-OP: EntryFile == Path (both the .ts
// file), so the output is the SAME path as the default. This is the OPPOSITE of
// skilldozer, where --file appended /SKILL.md. PRD §6.2: for file-kind
// extensions --file prints the entry file, which IS the file itself.
func TestRunTagFileOnSingleFileIsNoOp(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-f", "example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-f example): code=%d; want 0", code)
	}
	// EntryFile == Path for single-file kind → the SAME .ts path as the default.
	want := filepath.Join(dir, "example.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(-f example) stdout=%q; want %q (--file on a single-file ext is a no-op: EntryFile==Path)", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(-f example) stderr=%q; want empty", errOut.String())
	}
}

// --file on a DIR extension prints the dir's index.ts (PRD §6.2). This is one of
// the TWO weave-specific cases skilldozer could not exercise (its SourceFile was
// always Dir+"/SKILL.md"). Here Path is the dir, EntryFile is dir+"/index.ts".
func TestRunTagFileOnDirExtPrintsIndexTS(t *testing.T) {
	root := t.TempDir()
	writeDirExt(t, root, "git-checkpoint", "Git checkpoint extension.")
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"-f", "git-checkpoint"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-f git-checkpoint): code=%d; want 0", code)
	}
	want := filepath.Join(root, "git-checkpoint", "index.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(-f git-checkpoint) stdout=%q; want %q (--file on a dir ext → index.ts)", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(-f git-checkpoint) stderr=%q; want empty", errOut.String())
	}
}

// --file on a PACKAGE extension prints the FIRST pi.extensions entry (PRD §6.2).
// This is the second weave-specific case skilldozer could not exercise. Here
// Path is the package dir, EntryFile is the first existing pi.extensions entry
// (./src/index.ts).
func TestRunTagFileOnPkgExtPrintsPiExtensionsEntry(t *testing.T) {
	root := t.TempDir()
	writePkgExt(t, root, "summarizer", "Summarizes things.")
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"-f", "summarizer"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-f summarizer): code=%d; want 0", code)
	}
	want := filepath.Join(root, "summarizer", "src", "index.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(-f summarizer) stdout=%q; want %q (--file on a package ext → first pi.extensions entry)", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("run(-f summarizer) stderr=%q; want empty", errOut.String())
	}
}

// --relative prints the .ts file path RELATIVE to the extensions dir (PRD §6.2).
// The output uses the OS path separator (filepath.Rel), so compare via FromSlash.
func TestRunTagRelativePrintsRelativePath(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--relative", "reddit-poster"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--relative reddit-poster): code=%d; want 0", code)
	}
	want := filepath.FromSlash("writing/reddit-poster.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(--relative reddit-poster) stdout=%q; want %q (relative .ts path)", got, want)
	}
}

// --file --relative COMBINE: an entry-file path RELATIVE to the extensions dir
// (PRD §6.2). On a single-file ext EntryFile==Path, so this matches --relative
// alone here; the COMBINE is exercised by the dir/package --file tests' relative
// counterparts implicitly (EntryFile differs from Path there).
func TestRunTagFileRelativeCombine(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-f", "--relative", "reddit-poster"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-f --relative reddit-poster): code=%d; want 0", code)
	}
	want := filepath.FromSlash("writing/reddit-poster.ts") + "\n"
	if got := out.String(); got != want {
		t.Errorf("run(-f --relative reddit-poster) stdout=%q; want %q (relative entry file)", got, want)
	}
}

// Modifiers must NOT break §6.4 atomicity: one bad tag -> NOTHING on stdout, exit 1.
func TestRunTagFileAtomicity(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-f", "example", "nope"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(-f example nope): code=%d; want 1 (atomic failure)", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (modifiers must not break §6.4)", out.String())
	}
}

// --- run: --all/-a (M3.T2.S1) ---

// --all prints every extension's absolute .ts FILE path, one per line, SORTED by
// canonical tag (discover.Index already sorts []Extension by RelTag). exit 0.
func TestRunAllPrintsAllSorted(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--all"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--all): code=%d; want 0", code)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 paths; got %d: %q", len(lines), out.String())
	}
	// Sorted by RelTag: "example" < "writing/reddit-poster".
	if lines[0] != filepath.Join(dir, "example.ts") {
		t.Errorf("lines[0]=%q; want example.ts (sorted)", lines[0])
	}
	if lines[1] != filepath.Join(dir, "writing", "reddit-poster.ts") {
		t.Errorf("lines[1]=%q; want writing/reddit-poster.ts (sorted)", lines[1])
	}
	if errOut.Len() != 0 {
		t.Errorf("run(--all) stderr=%q; want empty", errOut.String())
	}
}

// -a short form behaves identically to --all.
func TestRunAllShortFlag(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-a"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-a): code=%d; want 0", code)
	}
	if !strings.Contains(out.String(), filepath.Join(dir, "example.ts")) {
		t.Errorf("run(-a) stdout missing example.ts:\n%s", out.String())
	}
}

// --all --file on a MIXED store (single-file + dir ext): --all prints ext.Path
// (the file for the single-file ext, the dir for the dir ext); --all --file
// prints ext.EntryFile (the file for the single-file ext, index.ts for the dir
// ext). They DIFFER on the dir ext, so this is a MEANINGFUL coverage of the
// --file modifier across Kinds (not a tautological no-op assertion).
func TestRunAllFilePrintsAllEntryFiles(t *testing.T) {
	root := t.TempDir()
	// A single-file ext: example.ts (Path==EntryFile==example.ts).
	if err := os.WriteFile(filepath.Join(root, "example.ts"), []byte("/** A demo extension. */\nexport default function() {}\n"), 0o644); err != nil {
		t.Fatalf("write example.ts: %v", err)
	}
	// A dir ext: git-checkpoint/index.ts (Path is the dir, EntryFile is index.ts).
	writeDirExt(t, root, "git-checkpoint", "Git checkpoint extension.")
	t.Setenv("weave_EXTENSIONS_DIR", root)

	// --all (default): example.ts + git-checkpoint/ (the dir).
	var outAll, errAll bytes.Buffer
	if code := run([]string{"--all"}, &outAll, &errAll); code != 0 {
		t.Fatalf("run(--all): code=%d; want 0", code)
	}
	linesAll := strings.Split(strings.TrimRight(outAll.String(), "\n"), "\n")
	if len(linesAll) != 2 {
		t.Fatalf("--all want 2 paths; got %d: %q", len(linesAll), outAll.String())
	}
	if linesAll[0] != filepath.Join(root, "example.ts") {
		t.Errorf("--all lines[0]=%q; want example.ts", linesAll[0])
	}
	if linesAll[1] != filepath.Join(root, "git-checkpoint") {
		t.Errorf("--all lines[1]=%q; want the git-checkpoint DIR (Path for dir kind)", linesAll[1])
	}

	// --all --file: example.ts + git-checkpoint/index.ts (the entry files).
	var outFile, errFile bytes.Buffer
	if code := run([]string{"--all", "--file"}, &outFile, &errFile); code != 0 {
		t.Fatalf("run(--all --file): code=%d; want 0", code)
	}
	linesFile := strings.Split(strings.TrimRight(outFile.String(), "\n"), "\n")
	if len(linesFile) != 2 {
		t.Fatalf("--all --file want 2 paths; got %d: %q", len(linesFile), outFile.String())
	}
	if linesFile[0] != filepath.Join(root, "example.ts") {
		t.Errorf("--all --file lines[0]=%q; want example.ts (EntryFile==Path for file kind)", linesFile[0])
	}
	if linesFile[1] != filepath.Join(root, "git-checkpoint", "index.ts") {
		t.Errorf("--all --file lines[1]=%q; want git-checkpoint/index.ts (EntryFile for dir kind)", linesFile[1])
	}
}

// --all --relative: every extension's path RELATIVE to the extensions dir, sorted.
func TestRunAllRelativePrintsAllRelative(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--all", "--relative"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--all --relative): code=%d; want 0", code)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 paths; got %d: %q", len(lines), out.String())
	}
	if lines[0] != filepath.FromSlash("example.ts") {
		t.Errorf("lines[0]=%q; want 'example.ts' (relative)", lines[0])
	}
	if lines[1] != filepath.FromSlash("writing/reddit-poster.ts") {
		t.Errorf("lines[1]=%q; want 'writing/reddit-poster.ts' (relative, OS-sep)", lines[1])
	}
}

// --all with an EMPTY store -> prints nothing, exit 0 (PRD §6.1: --all is ALWAYS
// exit 0, UNLIKE --list which exits 1 "if no extensions found" — --all is a
// scripting command where empty output + exit 0 is the useful shape).
func TestRunAllEmptyStoreExit0(t *testing.T) {
	t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()) // exists, no .ts files
	var out, errOut bytes.Buffer
	code := run([]string{"--all"}, &out, &errOut)
	if code != 0 {
		t.Errorf("run(--all) empty: code=%d; want 0 (PRD §6.1 --all is always 0)", code)
	}
	if out.Len() != 0 {
		t.Errorf("run(--all) empty stdout=%q; want empty", out.String())
	}
}

// --all when the extensions dir is unresolvable -> exit 1, empty stdout, the
// one-line fix (same contract as --path/--list/<tag>).
func TestRunAllUnresolvable(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // all §8.3 rules miss
	var out, errOut bytes.Buffer
	code := run([]string{"--all"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--all) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}

// --version precedes --all even when both are given (PRD §6.3).
func TestRunVersionPrecedenceOverAll(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--all", "--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--all --version): code=%d; want 0 (version precedence)", code)
	}
	if got := out.String(); got != "weave "+version+"\n" {
		t.Errorf("stdout=%q; want the version line (precedence over --all)", got)
	}
}

// --- run: --search (M4.T3.S1) ---

// --search with a match → filtered TAG table on stdout, exit 0, stderr empty.
func TestRunSearchMatch(t *testing.T) {
	dir := sampleStore(t) // example + writing/reddit-poster
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "reddit"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--search reddit): code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"TAG", "reddit-poster"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	// A non-matching extension must NOT appear.
	if strings.Contains(got, "example") {
		t.Errorf("stdout has 'example' (filter not applied):\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty", errOut.String())
	}
}

// -s short form behaves identically to --search.
func TestRunSearchShortFlag(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"-s", "example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(-s example): code=%d; want 0", code)
	}
	if !strings.Contains(out.String(), "example") {
		t.Errorf("stdout missing example:\n%s", out.String())
	}
}

// --search with NO match → exit 1, stderr "no extensions matched <q>", stdout EMPTY.
func TestRunSearchNoMatchExit1(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "zzz"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--search zzz): code=%d; want 1 (no matches)", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want EMPTY (§6.4: nothing on no-match)", out.String())
	}
	if !strings.Contains(errOut.String(), "no extensions matched zzz") {
		t.Errorf("stderr=%q; want 'no extensions matched zzz'", errOut.String())
	}
}

// --search "" matches EVERY extension (empty substring) → behaves like --list.
func TestRunSearchEmptyQueryMatchesAll(t *testing.T) {
	dir := sampleStore(t)
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"--search", ""}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(--search ''): code=%d; want 0 (empty query matches all)", code)
	}
	got := out.String()
	for _, want := range []string{"example", "reddit-poster"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q (empty query should match all):\n%s", want, got)
		}
	}
}

// --search when the dir is unresolvable → exit 1, stdout empty, one-line fix.
func TestRunSearchUnresolvableExit1(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir()) // all §8.3 rules miss
	var out, errOut bytes.Buffer
	code := run([]string{"--search", "x"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(--search x) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}

// --- run: check (M4.T3.S1) ---

// check on a clean store → one OK line per extension + summary, exit 0.
func TestRunCheckClean(t *testing.T) {
	dir := sampleStore(t) // example + writing/reddit-poster, both have JSDoc descs
	t.Setenv("weave_EXTENSIONS_DIR", dir)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(check) clean: code=%d; want 0", code)
	}
	got := out.String()
	for _, want := range []string{"OK", "example", "reddit-poster",
		"2 extensions, 0 errors, 0 warnings"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr=%q; want empty (check prints to stdout)", errOut.String())
	}
}

// check on an EMPTY store → "0 extensions, 0 errors, 0 warnings", exit 0.
// (Unlike --list which exits 1 on empty; check is validation: nothing == nothing wrong.)
func TestRunCheckEmptyStoreClean(t *testing.T) {
	t.Setenv("weave_EXTENSIONS_DIR", t.TempDir()) // exists, no entries
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Errorf("run(check) empty: code=%d; want 0 (empty store is clean)", code)
	}
	if !strings.Contains(out.String(), "0 extensions, 0 errors, 0 warnings") {
		t.Errorf("stdout=%q; want the 0/0/0 summary", out.String())
	}
}

// check with an ERROR → exit 1, the ERROR printed to STDOUT (pipeable).
// Fixture: a dir with a BROKEN package.json + an index.ts. discover indexes it
// as a dir-kind ext (case (b): broken JSON nulls Pi.Extensions, index.ts wins);
// check re-parses package.json(dir) → ERROR "package.json is not valid JSON".
func TestRunCheckWithErrorExit1(t *testing.T) {
	root := t.TempDir()
	brokenDir := filepath.Join(root, "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "package.json"),
		[]byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "index.ts"),
		[]byte("/** x */\nexport default function() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(check) with error: code=%d; want 1 (any ERROR)", code)
	}
	got := out.String()
	// The report is on STDOUT (check is a report, not a path emitter).
	if !strings.Contains(got, "ERROR") || !strings.Contains(got, "broken") {
		t.Errorf("stdout missing ERROR/broken line:\n%s", got)
	}
	if !strings.Contains(got, "1 errors") {
		t.Errorf("stdout summary missing '1 errors':\n%s", got)
	}
}

// check with only a WARN → exit 0 (warnings never change the exit code).
// Fixture: a single-file ext with NO JSDoc and no package.json → Description=""
// → check WARN "no description".
func TestRunCheckWithWarningExit0(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nodesc.ts"),
		[]byte("export default function() {}\n"), 0o644); err != nil { // NO JSDoc
		t.Fatal(err)
	}
	t.Setenv("weave_EXTENSIONS_DIR", root)
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run(check) with warning: code=%d; want 0 (warnings don't fail)", code)
	}
	got := out.String()
	if !strings.Contains(got, "WARN") || !strings.Contains(got, "nodesc") {
		t.Errorf("stdout missing WARN/nodesc line:\n%s", got)
	}
	if !strings.Contains(got, "0 errors, 1 warnings") {
		t.Errorf("stdout summary missing '0 errors, 1 warnings':\n%s", got)
	}
}

// check when the dir is unresolvable → exit 1, stdout empty, one-line fix.
func TestRunCheckUnresolvableExit1(t *testing.T) {
	unsetExtEnv(t)
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code := run([]string{"check"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("run(check) unresolvable: code=%d; want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q; want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "weave init") {
		t.Errorf("stderr=%q; want the one-line fix", errOut.String())
	}
}

// --- chooseStore (M4.T4.S1) ---

// mkdirWithExtEntry makes a temp dir that extdir.HasExtensionEntry reports as a
// store (contains a §7.1 entry at depth): tmp/sub/index.ts. cwd is a PARAMETER
// to chooseStore, so no t.Chdir is needed (unlike resolveStore which calls
// os.Getwd). Mirrors skilldozer's mkdirWithSkillMD with index.ts for the
// extension-kind predicate.
func mkdirWithExtEntry(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "index.ts"),
		[]byte("export default function() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile index.ts: %v", err)
	}
	return dir
}

// failIfCalled returns a prompt fn that fails the test if chooseStore invokes it.
// Enforces the prompt-safety guarantee (PRD §8.2): the non-interactive branches
// (`haveStore != ""` and `!isTTY`) must NEVER call the prompt fn.
func failIfCalled(t *testing.T) func(string, string) (string, error) {
	t.Helper()
	return func(label, def string) (string, error) {
		t.Errorf("chooseStore: prompt must not be called (label=%q)", label)
		return "", nil
	}
}

// OUTPUT #1: `init --store /tmp/x` ⇒ /tmp/x; prompt NEVER called.
func TestChooseStoreExplicitOverrideNoPrompt(t *testing.T) {
	got, err := chooseStore("/tmp/x", "/any/cwd", true, "/def", failIfCalled(t))
	if err != nil || got != "/tmp/x" {
		t.Errorf("chooseStore(/tmp/x,...): got (%q,%v); want (/tmp/x,nil)", got, err)
	}
}

// OUTPUT #2: cwd-with-extension + non-TTY ⇒ cwd; prompt NEVER called.
func TestChooseStoreCwdDetectNonTTY(t *testing.T) {
	cwd := mkdirWithExtEntry(t)
	got, err := chooseStore("", cwd, false, "/def", failIfCalled(t))
	if err != nil || got != cwd {
		t.Errorf("chooseStore(cwd-with-ext,non-TTY): got (%q,%v); want (%q,nil)", got, err, cwd)
	}
}

// OUTPUT #3: cwd-without-extension + non-TTY ⇒ defaultStore; prompt NEVER called.
func TestChooseStoreNoExtNonTTYUsesDefault(t *testing.T) {
	got, err := chooseStore("", t.TempDir(), false, "/def", failIfCalled(t))
	if err != nil || got != "/def" {
		t.Errorf("chooseStore(empty-cwd,non-TTY): got (%q,%v); want (/def,nil)", got, err)
	}
}

// OUTPUT #4: isTTY + prompt "" ⇒ default (cwd-without-ext so default=defaultStore).
func TestChooseStoreTTYEmptyPromptAcceptsDefault(t *testing.T) {
	prompt := func(label, def string) (string, error) { return "", nil }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err != nil || got != "/def" {
		t.Errorf("chooseStore(TTY,empty-prompt): got (%q,%v); want (/def,nil)", got, err)
	}
}

// OUTPUT #5: isTTY + prompt "/custom" ⇒ /custom (VERBATIM — no Abs in the core).
func TestChooseStoreTTYTypedPathOverrides(t *testing.T) {
	prompt := func(label, def string) (string, error) { return "/custom", nil }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err != nil || got != "/custom" {
		t.Errorf("chooseStore(TTY,typed-/custom): got (%q,%v); want (/custom,nil)", got, err)
	}
}

// The cwd-auto-detect DEFAULT is cwd even on a TTY; an empty prompt answer
// accepts that cwd default (not defaultStore). Guards against a bug where
// HasExtensionEntry is only consulted on the !isTTY branch.
func TestChooseStoreCwdDetectIsAlsoTheTTYDefault(t *testing.T) {
	cwd := mkdirWithExtEntry(t)
	prompt := func(label, def string) (string, error) {
		if def != cwd {
			t.Errorf("prompt default=%q; want cwd %q (auto-detect)", def, cwd)
		}
		return "", nil // Enter ⇒ accept the cwd default
	}
	got, err := chooseStore("", cwd, true, "/def", prompt)
	if err != nil || got != cwd {
		t.Errorf("chooseStore(cwd-with-ext,TTY,empty): got (%q,%v); want (%q,nil)", got, err, cwd)
	}
}

// A genuine (non-EOF) prompt read error is returned, not swallowed.
func TestChooseStorePropagatesPromptError(t *testing.T) {
	wantErr := errors.New("simulated read failure")
	prompt := func(label, def string) (string, error) { return "", wantErr }
	got, err := chooseStore("", t.TempDir(), true, "/def", prompt)
	if err == nil || !errors.Is(err, wantErr) {
		t.Errorf("chooseStore(prompt-error): got (%q,%v); want error wrapping %v", got, err, wantErr)
	}
}

// (Optional, 8th) readPrompt formats "%s [%s]: " and trims; empty/EOF ⇒ def.
func TestReadPromptFormatsDefAndTrims(t *testing.T) {
	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("/typed/path\n"))
	got, err := readPrompt(r, &out, "Where", "/def")
	if err != nil || got != "/typed/path" {
		t.Errorf("readPrompt(typed): got (%q,%v); want (/typed/path,nil)", got, err)
	}
	if want := "Where [/def]: "; out.String() != want {
		t.Errorf("readPrompt output=%q; want %q", out.String(), want)
	}
	// Empty line ⇒ def.
	var out2 bytes.Buffer
	r2 := bufio.NewReader(strings.NewReader("\n"))
	got2, err := readPrompt(r2, &out2, "Where", "/def")
	if err != nil || got2 != "/def" {
		t.Errorf("readPrompt(empty): got (%q,%v); want (/def,nil)", got2, err)
	}
	// EOF no text ⇒ def (not an error).
	var out3 bytes.Buffer
	r3 := bufio.NewReader(strings.NewReader(""))
	got3, err := readPrompt(r3, &out3, "Where", "/def")
	if err != nil || got3 != "/def" {
		t.Errorf("readPrompt(EOF): got (%q,%v); want (/def,nil)", got3, err)
	}
}
