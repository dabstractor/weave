package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes content to a temp config.yaml and returns its path. Each
// fixture lives in its own t.TempDir() so they never collide. Mirrors the
// writeConfig helper in skilldozer's config_test.go.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// --- Save/Load round trip (contract-required) ---

// TestSaveLoadRoundTrip locks the round-trip Save->Load equality: a realistic
// Store value survives a Save then a Load unchanged.
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	want := "/home/u/extensions"
	if err := Save(path, File{Store: want}); err != nil {
		t.Fatalf("Save: err=%v; want nil", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: err=%v; want nil", err)
	}
	if got.Store != want {
		t.Errorf("round-trip Store=%q; want %q", got.Store, want)
	}
}

// TestLoadIgnoresUnknownKeys locks "unknown keys ignored": extra keys (version,
// colors) are silently dropped by the hand-rolled scanner and Store is still
// populated, with no error.
func TestLoadIgnoresUnknownKeys(t *testing.T) {
	path := writeConfig(t, "store: /abs\nversion: 3\ncolors: red\n")
	f, err := Load(path)
	if err != nil {
		t.Fatalf("unknown keys: err=%v; want nil (ignored)", err)
	}
	if f.Store != "/abs" {
		t.Errorf("Store=%q; want /abs (unknown keys must not block it)", f.Store)
	}
}

// TestLoadMissingFileIsErrNotExist locks "fs.ErrNotExist returned (not masked)
// when file absent": a missing path returns an error that satisfies
// errors.Is(err, fs.ErrNotExist), so findConfig can fall through.
func TestLoadMissingFileIsErrNotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("missing file: err=nil; want an os.ReadFile error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err=%v; want an error satisfying errors.Is(fs.ErrNotExist)", err)
	}
}

// --- Hard on-disk format claim ---

// TestSaveWritesExactFormat locks Save's byte determinism: a non-empty Store is
// written as exactly "store: <value>\n" (hand-built bytes, no sorting, no BOM,
// no trailing "..."). Read back the raw bytes to verify — do not go through Load.
func TestSaveWritesExactFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, File{Store: "/x"}); err != nil {
		t.Fatalf("Save: err=%v; want nil", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: err=%v; want nil", err)
	}
	if string(out) != "store: /x\n" {
		t.Errorf("on-disk bytes=%q; want \"store: /x\\n\"", string(out))
	}
}

// --- weave delta: no store: line is NOT an error (PRD §8.1 "never a hard error") ---

// TestLoadPresentButNoStoreLineIsFileEmptyNil pins the single biggest weave
// delta from skilldozer: a present file with content but NO "store:" line is
// "not configured" (PRD §8.1) and returns File{}, nil — NOT a hard error.
// skilldozer (yaml.v3) would hard-error on broken YAML; weave never does.
func TestLoadPresentButNoStoreLineIsFileEmptyNil(t *testing.T) {
	path := writeConfig(t, "version: 3\nnotes: hello\n")
	f, err := Load(path)
	if err != nil {
		t.Fatalf("present-but-no-store: err=%v; want nil (PRD §8.1: never a hard error)", err)
	}
	if f != (File{}) {
		t.Errorf("present-but-no-store: f=%+v; want File{} (no store: line -> not configured)", f)
	}
}

// TestLoadMalformedOrJunkIsFileEmptyNil pins that the hand-rolled scanner never
// hard-errors on "unparseable" input — it just finds no "store:" line and
// returns File{}, nil. Contrast skilldozer's TestLoadMalformedYAMLIsHardError,
// which weave DROPS.
func TestLoadMalformedOrJunkIsFileEmptyNil(t *testing.T) {
	path := writeConfig(t, "this is not yaml\n@@@\n: : :\n")
	f, err := Load(path)
	if err != nil {
		t.Fatalf("junk input: err=%v; want nil (hand-rolled scanner never hard-errors)", err)
	}
	if f != (File{}) {
		t.Errorf("junk input: f=%+v; want File{} (no store: line -> not configured)", f)
	}
}

// TestLoadPicksFirstStoreLine documents first-match-wins: a file with two
// "store:" lines returns the FIRST one (the `return` inside the scan loop).
func TestLoadPicksFirstStoreLine(t *testing.T) {
	path := writeConfig(t, "store: /first\nstore: /second\n")
	f, err := Load(path)
	if err != nil {
		t.Fatalf("two store lines: err=%v; want nil", err)
	}
	if f.Store != "/first" {
		t.Errorf("two store lines: Store=%q; want \"/first\" (first match wins)", f.Store)
	}
}

// TestLoadStoreLineWithTrailingWhitespace locks "trim trailing whitespace": a
// "store:" value with trailing spaces (or a trailing CR from a Windows editor)
// is trimmed to the bare value.
func TestLoadStoreLineWithTrailingWhitespace(t *testing.T) {
	path := writeConfig(t, "store: /path   \n")
	f, err := Load(path)
	if err != nil {
		t.Fatalf("trailing whitespace: err=%v; want nil", err)
	}
	if f.Store != "/path" {
		t.Errorf("trailing whitespace: Store=%q; want \"/path\" (trailing whitespace trimmed)", f.Store)
	}
}

// --- Parent directory creation ---

// TestSaveCreatesParentDir verifies Save runs os.MkdirAll on the parent dir, so
// a nested config path whose directories do not yet exist (the common first-run
// case under $XDG_CONFIG_HOME/weave/) is created and the file lands there.
func TestSaveCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.yaml")
	if err := Save(path, File{Store: "/store"}); err != nil {
		t.Fatalf("Save with missing parent dirs: err=%v; want nil (MkdirAll should create them)", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created at %s after Save: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// Path / DefaultStore tests.
//
// Every test below mutates process env via t.Setenv, so NONE may call
// t.Parallel (mirrors skilldozer's config_test.go convention). t.Setenv cannot
// unset, but Path/DefaultStore use os.Getenv + `!=""`, so t.Setenv(var, "")
// correctly simulates "unset" for these two functions (empty == unset).
// Path/DefaultStore read ONLY env vars (no filesystem), so no temp FILES are
// needed — t.TempDir() is used only to obtain controlled ABSOLUTE env values.
// ---------------------------------------------------------------------------

// Path: a non-empty weave_CONFIG override is returned filepath.Clean'd, honored
// over $XDG_CONFIG_HOME. Proves the override branch wins.
func TestPathWeaveConfigAbsoluteOverride(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // prove override WINS over XDG
	t.Setenv(configEnv, "/abs/path/to/cfg.yaml")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() override abs: err=%v; want nil", err)
	}
	if want := filepath.Clean("/abs/path/to/cfg.yaml"); got != want {
		t.Errorf("Path() override abs: got=%q; want %q", got, want)
	}
}

// Path: a RELATIVE weave_CONFIG override is returned AS-IS (cleaned), NOT joined
// to the config home. This is THE critical no-join test (PRD §8.1 "useful for
// tests / multiple profiles"). Asserts the result is relative and contains no
// "weave" segment, proving it never touched the XDG default.
func TestPathWeaveConfigRelativeOverrideNotJoined(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(configEnv, "rel/sub/cfg.yaml")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() override rel: err=%v; want nil", err)
	}
	if want := filepath.Clean("rel/sub/cfg.yaml"); got != want {
		t.Errorf("Path() override rel: got=%q; want %q", got, want)
	}
	if filepath.IsAbs(got) {
		t.Errorf("Path() override rel: got=%q is absolute; must stay relative (NOT joined to configHome)", got)
	}
	if strings.Contains(got, "weave") {
		t.Errorf("Path() override rel: got=%q contains \"weave\"; override must NOT be joined to the XDG default", got)
	}
}

// Path: an EMPTY weave_CONFIG is equivalent to unset (os.Getenv returns "" for
// both, and the `!=""` guard treats them the same), so Path falls through to
// the os.UserConfigDir() default honoring $XDG_CONFIG_HOME.
func TestPathWeaveConfigEmptyFallsToXDG(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv(configEnv, "") // empty == unset
	xdg := t.TempDir()      // controlled absolute config home
	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() empty override: err=%v; want nil", err)
	}
	if want := filepath.Join(xdg, "weave", "config.yaml"); got != want {
		t.Errorf("Path() empty override: got=%q; want %q", got, want)
	}
}

// Path: a relative $XDG_CONFIG_HOME is rejected by os.UserConfigDir() with a
// non-nil error, and Path propagates it verbatim with an empty path. Asserts
// only err != nil (the stdlib error wording is not part of the contract).
func TestPathRejectsRelativeXDGConfigHome(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv(configEnv, "") // ensure the override does not short-circuit
	t.Setenv("XDG_CONFIG_HOME", "relative/not-abs")
	got, err := Path()
	if err == nil {
		t.Fatalf("Path() relative XDG_CONFIG_HOME: err=nil; want a non-nil error from os.UserConfigDir")
	}
	if got != "" {
		t.Errorf("Path() relative XDG_CONFIG_HOME: got=%q; want \"\" on error", got)
	}
}

// DefaultStore: an absolute $XDG_DATA_HOME is honored as the base dir.
func TestDefaultStoreAbsoluteXDGDataHome(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_DATA_HOME", "/abs/data")
	got, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore() abs XDG_DATA_HOME: err=%v; want nil", err)
	}
	if want := filepath.Join("/abs/data", "weave", "extensions"); got != want {
		t.Errorf("DefaultStore() abs XDG_DATA_HOME: got=%q; want %q", got, want)
	}
}

// DefaultStore: an EMPTY $XDG_DATA_HOME falls through to ~/.local/share.
func TestDefaultStoreEmptyXDGDataHomeFallsToHome(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_DATA_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore() empty XDG_DATA_HOME: err=%v; want nil", err)
	}
	if want := filepath.Join(home, ".local", "share", "weave", "extensions"); got != want {
		t.Errorf("DefaultStore() empty XDG_DATA_HOME: got=%q; want %q", got, want)
	}
}

// DefaultStore: a RELATIVE $XDG_DATA_HOME is invalid per the XDG spec and is
// IGNORED — the function falls back to ~/.local/share rather than producing a
// relative store path.
func TestDefaultStoreRelativeXDGDataHomeIgnored(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_DATA_HOME", "relative/data") // relative -> invalid -> ignored
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore() relative XDG_DATA_HOME: err=%v; want nil", err)
	}
	if want := filepath.Join(home, ".local", "share", "weave", "extensions"); got != want {
		t.Errorf("DefaultStore() relative XDG_DATA_HOME: got=%q; want %q (must fall back to ~/.local/share)", got, want)
	}
}

// DefaultStore: an unset/empty $HOME makes os.UserHomeDir() error, and
// DefaultStore propagates it verbatim with an empty path. (Linux-specific; PRD
// targets Linux.) Asserts only err != nil.
func TestDefaultStoreHomeUnsetErrors(t *testing.T) {
	// Do NOT call t.Parallel() — mutates env.
	t.Setenv("XDG_DATA_HOME", "") // force the HOME fallback branch
	t.Setenv("HOME", "")          // os.UserHomeDir -> error
	got, err := DefaultStore()
	if err == nil {
		t.Fatalf("DefaultStore() HOME unset: err=nil; want a non-nil error from os.UserHomeDir")
	}
	if got != "" {
		t.Errorf("DefaultStore() HOME unset: got=%q; want \"\" on error", got)
	}
}
