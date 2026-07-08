# encoding/json leniency behavior (verified empirically)

Goal: pin the EXACT stdlib `encoding/json` behavior for `parsePackageJSON`, so the
PRP can specify a struct + parsing strategy that satisfies PRD §7.3's leniency
mandate ("unknown keys ignored; wrong-typed fields coerced or ignored").

## Verbatim test output (go1.25, stdlib `encoding/json`)

### Case 1 — wrong-typed array field (string where []string expected)
```go
var s struct{ Keywords []string `json:"keywords"` }
json.Unmarshal([]byte(`{"keywords":"notarray"}`), &s)
// err = json: cannot unmarshal string into Go struct field .keywords of type []string
// s.Keywords = []
```
**FINDING: stdlib returns a HARD ERROR for a wrong-typed field. It does NOT silently
coerce `"notarray"` to `[]`. The contract phrase "a non-array `keywords` ⇒ `[]`"
is NOT satisfied by a plain typed `[]string` field + `json.Unmarshal` — the whole
unmarshal aborts and every other field stays zero.**

### Case 2 — nested []string under a sub-struct (works when well-typed)
```go
var s struct{ Weave struct{ Aliases []string `json:"aliases"`; Category string `json:"category"` } `json:"weave"` }
json.Unmarshal([]byte(`{"weave":{"aliases":["a","b"],"category":"ctx"}}`), &s)
// err = <nil>, Aliases=[a b], Category=ctx
```

### Case 3 — mixed []any elements into a []string field
```go
var s struct{ Weave struct{ Aliases []string `json:"aliases"` } `json:"weave"` }
json.Unmarshal([]byte(`{"weave":{"aliases":["a", 2, "b"]}}`), &s)
// err = json: cannot unmarshal number into Go struct field .weave.aliases of type string
// Aliases = [a (gap) b]  — partial fill, then error aborts the rest of the object
```
**FINDING: one bad element in an otherwise-string array ALSO hard-errors.**

### Case 4 — map[string]string dependencies (works)
```go
var s struct{ Dependencies map[string]string `json:"dependencies"` }
json.Unmarshal([]byte(`{"dependencies":{"zod":"^3.0.0"}}`), &s)
// err = <nil>, deps = map[zod:^3.0.0]
```

### Case 5 — empty input
```go
json.Unmarshal([]byte(``), &s)
// err = unexpected end of JSON input
```

### Case 6 — non-object top-level (array)
```go
json.Unmarshal([]byte(`[]`), &s)
// err = json: cannot unmarshal array into Go value of type struct{...}
```

### Case 7 — unknown fields (default behavior)
```go
dec := json.NewDecoder(strings.NewReader(`{"name":"x","bogus":1,"weave":{"aliases":["a","b"]}}`))
dec.Decode(&s)  // NO DisallowUnknownFields()
// err = <nil>, name=x, aliases=[a b]   — "bogus" silently ignored ✓
```

### Case 8 — the LENIENT strategy: []any fields + toStringSlice
```go
var s struct{ Weave struct{ Aliases []any `json:"aliases"`; Category any `json:"category"` } `json:"weave"` }
json.Unmarshal([]byte(`{"weave":{"aliases":["a", 2, "b"],"category": 123}}`), &s)
// err = <nil>, Aliases=[a 2 b], Category=123
// → then toStringSlice([a 2 b]) = [a b]   (drops the non-string "2")
// → category asserted as string → "" (number dropped)
```
**FINDING: typing the array fields as `[]any` (not `[]string`) makes json.Unmarshal
LENIENT — mixed types parse without error, and the existing `toStringSlice` helper
normalizes to []string exactly as skilldozer does for yaml.v3's []any. This is the
strategy that ACTUALLY achieves PRD §7.3 leniency for arrays.**

## Design conclusion for parsePackageJSON

The `packageJSON` struct fields for ARRAYS (Keywords, Weave.Aliases, Pi.Extensions)
MUST be typed as `[]any`, NOT `[]string`, so that a wrong-typed or mixed-type
field does not abort the entire unmarshal. Then `toStringSlice` (ported from
skilldozer's skill.go) normalizes `[]any` → `[]string`, dropping non-string
elements — exactly the PRD §7.3 "wrong-typed fields coerced or ignored" behavior.

Scalar fields (Name, Description, Weave.Category) as `string` are FINE: a wrong-typed
scalar (e.g. `"category": 123`) still errors the unmarshal. To be fully lenient on
scalars too, type them as `any` and assert `.(string)`. The PRD §7.3 examples only
mention array coercion, but for symmetry and to fully honor "coerced or ignored",
the BuildExtension step should read scalars defensively. The simplest robust approach:
type scalars as `any` in packageJSON, assert to string in BuildExtension (comma-ok
falls back to "").

**parsePackageJSON return contract** (per item contract):
- file missing (os.IsNotExist): return `(packageJSON{}, false, nil)` — NOT an error.
- JSON parse error (truly malformed JSON, or a type mismatch we chose not to suppress):
  return `(packageJSON{}, true, parseError)` — the bool signals "a package.json WAS
  read but unparseable", which check (M4.T2) reports as an ERROR.
- success: return `(pkg, true, nil)`.

With []any array fields + any scalar fields, the ONLY remaining hard-error cases are
truly malformed JSON (syntax errors) — which is precisely what check (§9 ERROR) wants
to flag. Wrong-typed VALUES are absorbed by the []any/any typing. This is the correct
leniency boundary.
