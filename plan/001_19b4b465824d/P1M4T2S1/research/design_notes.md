# P1.M4.T2.S1 Design Notes

## Source material read
- `internal/discover/extension.go` (Extension struct + parsePackageJSON 3-valued return + BuildExtension)
- `internal/discover/discover.go` (classifyFile/classifyDir, fileExists, the package-vs-dir precedence)
- `internal/discover/index.go` (Index walk + classify-then-descend + SkipDir pruning)
- `internal/check/check.go` + `check_test.go` (skilldozer — Report/Severity/Finding/SkillReport structure to port)
- `architecture/architecture_mapping.md §7` (ADAPT directive: port structure, change rules)
- PRD §9, §7.1, §7.3 (authoritative rules)
- `main.go` (consumer; check dispatch lands in M4.T3.S1, NOT here)

## Key design decisions

### 1. Port the TYPE STRUCTURE, not the RULES (architecture_mapping §7)
skilldozer's check.go validates Agent Skills frontmatter rules (name charset,
64-char limit, 1024-char description, dup NAME scan). weave has NONE of those.
weave's rules (PRD §9) are entirely different:

| weave §9 rule | Level | How to detect |
|---|---|---|
| duplicate canonical relTag across entries | ERROR | global scan over []Extension.RelTag |
| dir/package entry's entryFile does not exist on disk | ERROR | os.Stat(ext.EntryFile) per ext |
| package.json present but unparseable JSON | ERROR | re-parse ext's DIR via parsePackageJSON, check err != nil with hasPkg==true |
| package ext with non-empty deps but no node_modules/ | WARN | re-parse, check len(pkg.Dependencies)>0 AND no node_modules/ in ext.Path (dir) |
| entry has no description | WARN | ext.Description == "" (already folds pkg desc OR JSDoc) |
| top-level non-entry dir with zero discoverable entries | WARN | requires a FS walk — see below |

So: `Severity`, `String()`, `Finding`, `ExtensionReport` (was SkillReport), `Report`, `HasErrors()` port verbatim.
The `Check()` body and `checkFields`/`appendDupFindings` helpers are REWRITTEN.

### 2. Re-parse to distinguish "unparseable" from "missing" (item desc §1)
This mirrors skilldozer's re-parse of frontmatter. discover.Index DROPS
parsePackageJSON's error (lenient — classifyFile/classifyDir pass `_`). So check
must call `discover.ParsePackageJSON(ext's dir)` again to recover the
unparseable-vs-missing distinction. The 3-valued return is exactly what we need:
- (pkg{}, false, nil) → no package.json → no error (normal for file exts)
- (pkg{}, true, err) → EXISTS but unparseable → ERROR
- (pkg, true, nil) → success → use pkg for deps check

PROBLEM: `parsePackageJSON` is UNEXPORTED. The item desc explicitly says:
"discover.parsePackageJSON (needs to be accessible — export it or re-parse in check)".
The cleanest option is to EXPORT it: rename `parsePackageJSON` → `ParsePackageJSON`,
and export `packageJSON` (or expose a thin accessor). BUT that couples check to
discover's internal type. Two viable approaches:

**Option A (chosen):** Export a small helper on discover that does the re-parse
AND returns the fields check needs, WITHOUT exposing the raw packageJSON type.
E.g. `discover.ParsePackageJSONCheck(dir string) (hasDepsWithoutNodeModules bool,
unparseable bool)`. Too bespoke; leaks the §9 logic into discover.

**Option B (cleaner, chosen):** Export `ParsePackageJSON` (rename) and the
`PackageJSON` type (rename struct + fields capitalized). check imports it and
reads `pkg.Dependencies` + the error directly. This is a SMALL edit to
discover/extension.go (capitalize 4 symbols) but it makes the dependency
explicit and auditable. The item desc sanctions "export it".

DECISION: Option B — export `discover.ParsePackageJSON` + `discover.PackageJSON`.
This is a 1-line-per-symbol change in the already-Complete discover package
(P1.M2.T1.S1), with no behavior change. check then reads `.Dependencies` and
treats the err return directly. This also keeps the §9 decision logic INSIDE
check (where it belongs), not smuggled into discover.

For the `node_modules/` check: `ext.Kind == "package"` AND dir is `ext.Path`
(package/dir Path is the directory). Check `filepath.Join(ext.Path, "node_modules")`
via os.Stat + IsDir. Only fires for package kind (PRD §9: "a PACKAGE extension's
package.json declares non-empty dependencies but the entry dir has no node_modules").

### 3. Empty category folder (Pass 2 WARN) — the hard one
This is the ONE rule that CANNOT be derived from `[]Extension` alone. Index
prunes resolvable dir subtrees and only emits Extension entries. A top-level
directory like `extensions/empty/` (no entries at any depth) contributes ZERO
entries to []Extension — so check, given only []Extension, has no idea `empty/`
exists. skilldozer does NOT have an analogous rule (its §9 only scans skills).

The item desc says Pass 2 is a "global check". But the input is
`func Check(exts []discover.Extension) Report` per the item desc — and that
signature alone cannot detect empty folders.

RESOLUTION: Check needs the EXTENSIONS DIR path too, to do its own walk for the
empty-folder rule. Two options:

**Option X:** Change signature to `Check(dir string, exts []discover.Extension) Report`.
main passes both. check does a WalkDir over dir to find top-level non-entry dirs
containing zero entries at any depth, OR'ing those findings into the report.

**Option Y:** Keep `Check(exts) Report` and SKIP the empty-folder rule (it would
need a separate function). This violates the item desc which lists the rule.

The item desc is explicit: "Pass 2 (global checks): ... Top-level non-entry
directory with zero discoverable entries at any depth → WARN 'empty category
folder: <dir>'." So Option X is required.

DECISION: Signature `func Check(dir string, exts []discover.Extension) Report`.
This differs from skilldozer's `Check(skills)` but is necessary. The empty-folder
detection does its OWN WalkDir over dir (cheap — small tree), counting entries
per top-level subdir using the SAME classify-then-descend semantics as Index.
A top-level subdir is "empty" if walking it (with the pruning rule) yields zero
Extension entries. This re-uses discover's exported `Index` recursively per
top-level child, OR replicates the walk inline. Simplest: walk each top-level
child dir with `discover.Index(child)`; if it returns (nil/empty, nil) AND the
child is not itself a resolvable extension entry → WARN.

Actually even simpler and matches the rule precisely: for each top-level entry
of dir, if it is a plain (non-extension) directory, run discover.Index on that
subdir; if zero entries come back → it's an empty category folder → WARN.

Note: a top-level dir that IS a resolvable extension (has index.ts or a
qualifying package.json) is NOT "empty" — it's a real extension. Only PLAIN
category dirs that resolve to zero entries at any depth are flagged.

### 4. Severity/String/Finding/ExtensionReport/Report/HasErrors — verbatim port
Rename `SkillReport` → `ExtensionReport`, field `Skill` → `Extension`. Everything
else (Severity iota, String switch, Finding{Level,Message}, Report{BySkill→ByExt
or keep BySkill name? rename to ByExt, Errors, Warnings}, HasErrors) ports with
noun swaps only. I'll rename `BySkill` → `ByExt` for clarity (the field name is
internal; main consumes it in M4.T3.S1 which isn't built yet, so no rename
conflict).

### 5. Output formatting is NOT in scope
PRD §9 output format ("OK <relTag>", summary line) is rendered by main in
M4.T3.S1, NOT here. check.Check returns a structured Report; main renders it.
This matches skilldozer's split (check returns Report; main renders).

### 6. parsePackageJSON signature after export
Current: `parsePackageJSON(dir string) (pkg packageJSON, hasPkg bool, err error)`.
After export: `ParsePackageJSON(dir string) (pkg PackageJSON, hasPkg bool, err error)`.
check calls it on `ext.Path`'s directory. For file-kind exts, the dir is
`filepath.Dir(ext.Path)`. For dir/package exts, the dir IS `ext.Path`. To keep
it uniform: check should call ParsePackageJSON on the ENTRY's directory.

For a file ext, the package.json (if any) lives next to the file → Dir(EntryFile).
For a dir/package ext, package.json lives in ext.Path (the dir) → ext.Path == Dir(EntryFile)? 
For package: EntryFile is `ext.Path/src/index.ts`, so Dir(EntryFile) is NOT ext.Path.
For dir: EntryFile is `ext.Path/index.ts`, Dir(EntryFile) == ext.Path.

To be consistent with how discover BUILT the ext, check must re-parse the SAME
dir discover parsed: for file exts, Dir(file); for dir/package, the dir itself
(ext.Path). So check parses `ext.Path` for dir/package kinds and
`filepath.Dir(ext.EntryFile)` for file kind. Simpler: derive the package dir as
ext.Path if Kind != "file" else Dir(ext.Path). (For a file ext, Path==EntryFile==the file.)

## Test plan (port skilldozer's check_test.go structure, swap rules)
- clean store (all OK, 0/0)
- duplicate relTag across two entries → ERROR each
- missing entryFile (point ext.EntryFile at nonexistent path) → ERROR
- unparseable package.json (write broken JSON) → ERROR
- deps without node_modules (package ext, package.json deps, no node_modules/) → WARN
- no description (file ext, no pkg, no JSDoc) → WARN
- empty category dir (top-level plain dir, zero entries) → WARN
- empty input Check("", nil) → clean, no panic
- node_modules present with deps → clean (no WARN)

Tests use t.TempDir() + real files (mirrors skilldozer's mkSkill helper → mkExt).
