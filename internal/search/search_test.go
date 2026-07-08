package search

import (
	"testing"

	"github.com/dabstractor/weave/internal/discover"
)

// mkExt builds one discover.Extension with the searchable fields set
// (HasPackageJSON true so all fields are "real"). Mirrors the convention used in
// sibling test files but lets keywords be set. Use len() on Keywords (never a
// nil check) per the discover contract.
func mkExt(tag, name, desc string, keywords ...string) discover.Extension {
	return discover.Extension{
		RelTag:         tag,
		Name:           name,
		Description:    desc,
		Keywords:       keywords,
		HasPackageJSON: true,
	}
}

func TestSearchMatchByTag(t *testing.T) {
	in := []discover.Extension{mkExt("writing/reddit", "rp", "d")}
	out := Search("writing/reddit", in)
	if len(out) != 1 || out[0].RelTag != "writing/reddit" {
		t.Errorf("exact tag match failed: got %+v", out)
	}
}

func TestSearchMatchByTagSubstring(t *testing.T) {
	in := []discover.Extension{mkExt("writing/reddit", "rp", "d")}
	out := Search("redd", in)
	if len(out) != 1 {
		t.Errorf("tag substring 'redd' should match writing/reddit: got %+v", out)
	}
}

func TestSearchMatchByBasenameAsSubstring(t *testing.T) {
	in := []discover.Extension{mkExt("writing/reddit", "rp", "d")}
	out := Search("reddit", in) // basename is part of the relTag string
	if len(out) != 1 {
		t.Errorf("'reddit' substring of relTag should match: got %+v", out)
	}
}

func TestSearchMatchByName(t *testing.T) {
	in := []discover.Extension{mkExt("a", "reddit-poster", "d")}
	out := Search("poster", in)
	if len(out) != 1 {
		t.Errorf("name substring 'poster' should match: got %+v", out)
	}
}

func TestSearchMatchByDescription(t *testing.T) {
	in := []discover.Extension{mkExt("a", "n", "Posts messages to social media")}
	out := Search("social", in)
	if len(out) != 1 {
		t.Errorf("description substring 'social' should match: got %+v", out)
	}
}

func TestSearchMatchByKeyword(t *testing.T) {
	in := []discover.Extension{mkExt("a", "n", "d", "writing", "social")}
	out := Search("soc", in)
	if len(out) != 1 {
		t.Errorf("keyword substring 'soc' should match keyword 'social': got %+v", out)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	in := []discover.Extension{mkExt("Reddit", "Name", "Desc")}
	for _, q := range []string{"reddit", "REDDIT", "rEdDiT", "name", "DESC"} {
		if out := Search(q, in); len(out) != 1 {
			t.Errorf("case-insensitive query %q should match; got %+v", q, out)
		}
	}
}

func TestSearchNoMatchReturnsEmpty(t *testing.T) {
	in := []discover.Extension{mkExt("a", "n", "d", "k")}
	if out := Search("zzznotfound", in); len(out) != 0 {
		t.Errorf("no-match query should return empty slice; got %+v", out)
	}
}

func TestSearchEmptyQueryMatchesAll(t *testing.T) {
	in := []discover.Extension{mkExt("a", "n", "d"), mkExt("b", "m", "e")}
	if out := Search("", in); len(out) != 2 {
		t.Errorf("empty query should match all; got %d", len(out))
	}
}

func TestSearchPreservesInputOrder(t *testing.T) {
	in := []discover.Extension{
		mkExt("zebra", "n", "match"),
		mkExt("apple", "n", "match"),
		mkExt("mango", "n", "unrelated"),
	}
	out := Search("match", in) // matches zebra + apple by description, in that order
	if len(out) != 2 || out[0].RelTag != "zebra" || out[1].RelTag != "apple" {
		t.Errorf("order not preserved: got %+v", out)
	}
}

func TestSearchMultipleMatchesAllReturned(t *testing.T) {
	in := []discover.Extension{
		mkExt("a", "x", "common"),
		mkExt("b", "x", "unrelated"),
		mkExt("c", "common", "y"),
	}
	out := Search("common", in)
	if len(out) != 2 {
		t.Errorf("expected 2 matches across desc+name; got %d: %+v", len(out), out)
	}
}

func TestSearchNoPackageJSONStillMatchesByTag(t *testing.T) {
	// HasPackageJSON false => Name/Description empty, Keywords nil, but RelTag present.
	in := []discover.Extension{{RelTag: "bare-extension", HasPackageJSON: false}}
	out := Search("bare", in)
	if len(out) != 1 {
		t.Errorf("metadata-less extension must still match by tag; got %+v", out)
	}
}

func TestSearchMatchesCategoryAndAliases(t *testing.T) {
	// PRD §6.1/§7.3 state weave.aliases/weave.category feed `weave --search` — so
	// aliases and category ARE searched. This makes --search consistent with
	// resolve, which resolves by alias (§7.2 step 4). Issue 4 fix: inverts the old
	// TestSearchDoesNotMatchCategoryOrAliases that encoded the wrong behavior.
	withAliases := []discover.Extension{
		{RelTag: "x", Name: "n", Description: "d", Aliases: []string{"secret-alias"}, HasPackageJSON: true},
	}
	if out := Search("secret-alias", withAliases); len(out) != 1 {
		t.Errorf("search must match weave.aliases: query %q got %+v", "secret-alias", out)
	}
	withCategory := []discover.Extension{
		{RelTag: "x", Name: "n", Description: "d", Category: "secret-cat", HasPackageJSON: true},
	}
	if out := Search("secret-cat", withCategory); len(out) != 1 {
		t.Errorf("search must match weave.category: query %q got %+v", "secret-cat", out)
	}
}

func TestSearchKeywordSubstringNotJoinBoundary(t *testing.T) {
	// Keywords are matched INDIVIDUALLY, not joined — so a query spanning a
	// boundary between two keywords must NOT match. "wriocial" is not a substring
	// of "writing" nor of "social" individually.
	in := []discover.Extension{mkExt("a", "n", "d", "writing", "social")}
	if out := Search("wriocial", in); len(out) != 0 {
		t.Errorf("keyword-boundary query must not match (keywords searched individually): got %+v", out)
	}
}

func TestSearchNilInput(t *testing.T) {
	if out := Search("x", nil); len(out) != 0 {
		t.Errorf("nil input should yield empty; got %+v", out)
	}
}

func TestSearchReturnsDistinctResults(t *testing.T) {
	// An extension matching in MULTIPLE fields (e.g. tag AND description) is
	// returned ONCE, not duplicated.
	in := []discover.Extension{mkExt("match", "n", "match")}
	out := Search("match", in)
	if len(out) != 1 {
		t.Errorf("multi-field match should return the extension once; got %d", len(out))
	}
}
