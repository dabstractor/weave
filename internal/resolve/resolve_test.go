package resolve

import (
	"errors"
	"testing"

	"github.com/dabstractor/weave/internal/discover"
)

// exampleExts mirrors the PRD §7.2 example setup: a single-file extension `gate`,
// a nested extension `writing/reddit-poster` (whose package.json name is
// `reddit-poster` and which declares alias `social`), and a package extension
// `summarizer` (whose package.json name is `@my-org/summarizer`, exercising the
// pure name fallback that basename cannot reach). Only RelTag/Name/Aliases
// influence resolution; Path/EntryFile are filled with realistic absolute paths
// so a returned Result.Extension is usable by main. reddit-poster is given an
// alias "social" so the example fixture also exercises the alias step without
// needing a second fixture.
var exampleExts = []discover.Extension{
	{
		RelTag:    "gate",
		Kind:      "file",
		Name:      "",
		Path:      "/repo/extensions/gate.ts",
		EntryFile: "/repo/extensions/gate.ts",
	},
	{
		RelTag:    "writing/reddit-poster",
		Kind:      "file",
		Name:      "reddit-poster",
		Aliases:   []string{"social"},
		Path:      "/repo/extensions/writing/reddit-poster.ts",
		EntryFile: "/repo/extensions/writing/reddit-poster.ts",
	},
	{
		RelTag:    "summarizer",
		Kind:      "package",
		Name:      "@my-org/summarizer",
		Path:      "/repo/extensions/summarizer",
		EntryFile: "/repo/extensions/summarizer/src/index.ts",
	},
}

// TestResolveExamples is THE PRD §7.2 examples table (the item's required test),
// plus the alias step on the same fixture. Each row asserts both the resolved
// RelTag and the MatchKind. Note `reddit-poster` is BOTH a basename (of
// `writing/reddit-poster`) AND a package.json name — basename (step 2) wins over
// name (step 3), so the assertion is Basename; the pure name fallback is covered
// by `@my-org/summarizer`.
func TestResolveExamples(t *testing.T) {
	cases := []struct {
		tag       string
		wantRel   string
		wantMatch MatchKind
	}{
		{"gate", "gate", Canonical},                                   // exact RelTag
		{"writing/reddit-poster", "writing/reddit-poster", Canonical}, // exact RelTag (nested)
		{"reddit-poster", "writing/reddit-poster", Basename},          // basename, unambiguous (beats Name)
		{"@my-org/summarizer", "summarizer", Name},                    // package.json name fallback
		{"social", "writing/reddit-poster", Alias},                    // declared alias
	}
	for _, c := range cases {
		got, err := Resolve(c.tag, exampleExts)
		if err != nil {
			t.Errorf("Resolve(%q): err=%v; want nil", c.tag, err)
			continue
		}
		if got.Match != c.wantMatch {
			t.Errorf("Resolve(%q): Match=%v; want %v", c.tag, got.Match, c.wantMatch)
		}
		if got.Extension.RelTag != c.wantRel {
			t.Errorf("Resolve(%q): RelTag=%q; want %q", c.tag, got.Extension.RelTag, c.wantRel)
		}
	}
}

// TestResolveAmbiguous exercises the >1-match case at each of steps 2/3/4. Input
// is deliberately passed in REVERSE sorted order to prove sortedRelTags sorts the
// Candidates regardless of input ordering.
func TestResolveAmbiguous(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		// exts listed REVERSE-sorted so Candidates sorting is observable.
		exts []discover.Extension
		want []string // expected sorted Candidates
	}{
		{
			name: "basename",
			tag:  "reddit",
			exts: []discover.Extension{
				{RelTag: "writing/reddit", Name: "a"},
				{RelTag: "coding/reddit", Name: "b"},
			},
			want: []string{"coding/reddit", "writing/reddit"},
		},
		{
			name: "name",
			tag:  "dup",
			exts: []discover.Extension{
				{RelTag: "beta", Name: "dup"},
				{RelTag: "alpha", Name: "dup"},
			},
			want: []string{"alpha", "beta"},
		},
		{
			name: "alias",
			tag:  "shared",
			exts: []discover.Extension{
				{RelTag: "beta", Aliases: []string{"shared"}},
				{RelTag: "alpha", Aliases: []string{"shared"}},
			},
			want: []string{"alpha", "beta"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Resolve(c.tag, c.exts)
			if err == nil {
				t.Fatalf("Resolve(%q) [%s]: err=nil res=%+v; want *AmbiguousError", c.tag, c.name, res)
			}
			ae, ok := err.(*AmbiguousError)
			if !ok {
				t.Fatalf("Resolve(%q) [%s]: err type=%T; want *AmbiguousError", c.tag, c.name, err)
			}
			if ae.Tag != c.tag {
				t.Errorf("Tag=%q; want %q", ae.Tag, c.tag)
			}
			if len(ae.Candidates) != len(c.want) {
				t.Fatalf("Candidates=%v; want %v", ae.Candidates, c.want)
			}
			for i, want := range c.want {
				if ae.Candidates[i] != want {
					t.Errorf("Candidates[%d]=%q; want %q (full=%v)", i, ae.Candidates[i], want, ae.Candidates)
				}
			}
		})
	}
}

// TestResolveUnknown: a tag matching nothing, and an empty/nil index, both yield
// *UnknownError{Tag: tag}. No panic on nil/empty input.
func TestResolveUnknown(t *testing.T) {
	// Unknown tag against the example index.
	res, err := Resolve("nope", exampleExts)
	if err == nil {
		t.Fatalf("Resolve(nope): err=nil res=%+v; want *UnknownError", res)
	}
	ue, ok := err.(*UnknownError)
	if !ok {
		t.Fatalf("Resolve(nope): err type=%T; want *UnknownError", err)
	}
	if ue.Tag != "nope" {
		t.Errorf("Tag=%q; want nope", ue.Tag)
	}

	// Empty index ⇒ unknown (range over nil/empty is a no-op).
	if _, err := Resolve("anything", nil); err == nil {
		t.Fatal("Resolve(anything, nil): err=nil; want *UnknownError")
	}
	if _, err := Resolve("anything", []discover.Extension{}); err == nil {
		t.Fatal("Resolve(anything, []): err=nil; want *UnknownError")
	}
}

// TestResolvePrecedence: first-match-wins. A tag that matches at an EARLIER step
// must resolve there even if it would also match a later step.
func TestResolvePrecedence(t *testing.T) {
	// Canonical beats Name: ext A has RelTag "x"; ext B has Name "x".
	// tag "x" must resolve to A at step 1 (Canonical), NOT B at step 3 (Name).
	exts := []discover.Extension{
		{RelTag: "x", Name: "a-name", Path: "/s/x"},
		{RelTag: "y", Name: "x"},
	}
	got, err := Resolve("x", exts)
	if err != nil {
		t.Fatalf("Resolve(x) precedence: err=%v; want nil", err)
	}
	if got.Match != Canonical {
		t.Errorf("Match=%v; want Canonical (step 1 beats step 3)", got.Match)
	}
	if got.Extension.RelTag != "x" {
		t.Errorf("RelTag=%q; want x", got.Extension.RelTag)
	}

	// Canonical beats Basename: a top-level ext "gate" — tag "gate" is BOTH the
	// exact RelTag (step 1) AND its own basename (step 2). Step 1 must win.
	got2, err := Resolve("gate", exampleExts)
	if err != nil || got2.Match != Canonical {
		t.Errorf("Resolve(gate): match=%v err=%v; want Canonical (step 1 beats step 2)", got2.Match, err)
	}
}

// TestResolveEmptyTagGuard: an extension with Name=="" (no package.json name)
// must NOT match step 3, so a degenerate empty tag ("") yields *UnknownError,
// not a Name hit.
func TestResolveEmptyTagGuard(t *testing.T) {
	exts := []discover.Extension{
		{RelTag: "nofm", Name: ""}, // no package.json → Name empty
	}
	res, err := Resolve("", exts)
	if err == nil {
		t.Fatalf("Resolve(\"\"): err=nil res=%+v; want *UnknownError (empty Name must not match)", res)
	}
	if _, ok := err.(*UnknownError); !ok {
		t.Fatalf("Resolve(\"\"): err type=%T; want *UnknownError", err)
	}

	// Sanity: a NON-empty tag still resolves normally on the same fixture by basename.
	if _, err := Resolve("nofm", exts); err != nil {
		t.Errorf("Resolve(nofm): err=%v; want nil (basename match)", err)
	}
}

// TestResolveDuplicateAliasCountedOnce: an extension whose Aliases lists the
// same tag twice still counts as ONE match (collectMatches appends each
// extension at most once), so a single such extension resolves cleanly rather
// than being misread as ambiguous.
func TestResolveDuplicateAliasCountedOnce(t *testing.T) {
	exts := []discover.Extension{
		{RelTag: "alpha", Aliases: []string{"dup", "dup"}},
	}
	got, err := Resolve("dup", exts)
	if err != nil {
		t.Fatalf("Resolve(dup): err=%v; want nil (duplicate alias counts once)", err)
	}
	if got.Match != Alias || got.Extension.RelTag != "alpha" {
		t.Errorf("Resolve(dup): match=%v rel=%q; want Alias/alpha", got.Match, got.Extension.RelTag)
	}
}

// TestResolveErrorsAs: the typed errors must be extractable via errors.As — the
// contract main (P1.M3.T2.S1) relies on to branch on error type and read
// Candidates.
func TestResolveErrorsAs(t *testing.T) {
	ambig := []discover.Extension{
		{RelTag: "writing/reddit"},
		{RelTag: "coding/reddit"},
	}
	_, err := Resolve("reddit", ambig)

	var ae *AmbiguousError
	if !errors.As(err, &ae) {
		t.Fatalf("errors.As(*AmbiguousError)=false for %T; want true", err)
	}
	if ae.Tag != "reddit" || len(ae.Candidates) != 2 {
		t.Errorf("extracted AmbiguousError=%+v; want Tag=reddit, 2 candidates", ae)
	}

	_, err = Resolve("nope", exampleExts)
	var ue *UnknownError
	if !errors.As(err, &ue) {
		t.Fatalf("errors.As(*UnknownError)=false for %T; want true", err)
	}
	if ue.Tag != "nope" {
		t.Errorf("extracted UnknownError.Tag=%q; want nope", ue.Tag)
	}

	// Negative: an UnknownError must NOT masquerade as an AmbiguousError.
	_, err = Resolve("nope", exampleExts)
	var wrong *AmbiguousError
	if errors.As(err, &wrong) {
		t.Error("errors.As(*AmbiguousError)=true on an UnknownError; want false")
	}
}

// TestErrorMessages: exact .Error() text (we own the format strings). main may
// reformat for §6.4, but the package's own rendering must be stable.
func TestErrorMessages(t *testing.T) {
	if got := (&UnknownError{Tag: "foo"}).Error(); got != `unknown extension tag "foo"` {
		t.Errorf("UnknownError.Error()=%q; want %q", got, `unknown extension tag "foo"`)
	}
	got := (&AmbiguousError{Tag: "reddit", Candidates: []string{"coding/reddit", "writing/reddit"}}).Error()
	want := `ambiguous extension tag "reddit" matches: coding/reddit, writing/reddit`
	if got != want {
		t.Errorf("AmbiguousError.Error()=%q; want %q", got, want)
	}
}

// TestMatchKindString: each constant renders, and an out-of-range value renders
// as "unknown".
func TestMatchKindString(t *testing.T) {
	cases := []struct {
		m    MatchKind
		want string
	}{
		{Canonical, "canonical"},
		{Basename, "basename"},
		{Name, "name"},
		{Alias, "alias"},
		{MatchKind(-1), "unknown"},
		{MatchKind(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("MatchKind(%d).String()=%q; want %q", c.m, got, c.want)
		}
	}
}

// TestBasename: the private slash-split helper (covers the no-slash and
// multi-level cases directly, independent of Resolve).
func TestBasename(t *testing.T) {
	cases := []struct{ relTag, want string }{
		{"writing/reddit-poster", "reddit-poster"},
		{"gate", "gate"}, // no slash → whole string
		{"a/b/c", "c"},   // multi-level → last
		{"", ""},         // degenerate
	}
	for _, c := range cases {
		if got := basename(c.relTag); got != c.want {
			t.Errorf("basename(%q)=%q; want %q", c.relTag, got, c.want)
		}
	}
}
