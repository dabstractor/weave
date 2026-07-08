# Research Notes — P1.M1.T3.S1

## Scope of THIS subtask (ONLY)
- Source type + 4 constants + String() labels
- envVar const = "weave_EXTENSIONS_DIR" (lowercase)
- findEnv() (rule 1) — env resolution, NO EvalSymlinks
- HasExtensionEntry(dir) — exported predicate (init cwd-auto-detect + walk-up qualifier)
- Package `extdir` skeleton that P1.M1.T3.S2 and P1.M1.T3.S3 will extend

## NOT in scope (later subtasks)
- findConfig (rule 2) → P1.M1.T3.S2
- findSibling + resolveSiblingFromExe (rule 3) → P1.M1.T3.S2
- findWalkUp + findWalkUpAncestor (rule 4) → P1.M1.T3.S3
- Find() combiner + ErrNotFound → P1.M1.T3.S3

## Key sources
- skilldozer `internal/skillsdir/skillsdir.go` (the port origin) — read in full
- skilldozer `internal/skillsdir/skillsdir_test.go` — read the findEnv + Source + HasSkillMD tests
- architecture_mapping.md §2 — the port map (env var name, sibling dir name, predicate name)
- PRD §8.3 rule 1 + §8.2 labels + §7.1 extension-entry definition
- PRD §13 acceptance (extdir is "most failure-prone area — implement and test it first")

## What ports verbatim from skilldozer (rule-1 portions)
- `type Source int` + 4 constants in iota order: SourceEnv, SourceConfig, SourceSibling, SourceWalkUp
- `String()` switch — labels change: `"SKILLDOZER_SKILLS_DIR"` → `"weave_EXTENSIONS_DIR"`; rest identical
- `envVar` const pattern (lowercase package const; tests drive via t.Setenv)
- `findEnv()` body: os.LookupEnv → empty check → os.Stat → IsDir → filepath.Abs → return abs/SourceEnv/true; else ("",0,false)

## What is NEW for weave (HasExtensionEntry — NOT a HasSkillMD port)
The predicate changes fundamentally. skilldozer's HasSkillMD walks for one filename
(`SKILL.md`). weave walks for ANY extension entry per PRD §7.1:

An **extension entry** is:
- (a) a `*.ts` or `*.js` FILE whose base name is NOT `index.ts`/`index.js`
- (b) a DIRECTORY that directly contains `index.ts` or `index.js`
- (c) a DIRECTORY with `package.json` containing a `pi.extensions` array

"at any depth" + "first find short-circuits" → use sentinel error + WalkDir like skilldozer,
but the callback must check all three conditions.

### Implementation detail for case (c) — package.json pi.extensions
From pi_extension_facts.md §6: pi reads `package.json`, takes `pkg.pi.extensions` (array of
strings), resolves each relative to the dir, and only counts entries that exist. For the
PREDICATE (HasExtensionEntry), we need: "package.json with a `pi.extensions` array naming
≥1 existing entry" (PRD §7.1 dir/package definition).

For this subtask, the predicate only needs to know IF such an array exists naming ≥1
existing entry. We parse package.json with encoding/json (stdlib), lenient per §7.3.

### Short-circuit approach (mirrors skilldozer's errSkillMDFound)
```go
var errExtensionFound = errors.New("extension entry found")
found := false
_ = filepath.WalkDir(dir, func(path, d, err) error {
    if err != nil { return nil }
    if d.IsDir() {
        // check case (b) and (c) — does THIS dir qualify as an entry?
        if isResolvableDir(path) {   // index.ts/index.js OR package.json pi.extensions
            found = true
            return errExtensionFound
        }
        return nil  // plain dir → keep descending
    }
    // file: check case (a) — *.ts/*.js not named index.*
    if isExtensionFile(d.Name()) {  // .ts/.js and not index.ts/index.js
        found = true
        return errExtensionFound
    }
    return nil
})
return found
```

**GOTCHA — the predicate recursion rule differs from the Index() classify-then-descend
rule (P1.M2.T2).** For the PREDICATE, we ONLY need a boolean: "is there ANY extension
entry anywhere under dir?" We descend into ALL dirs (including resolvable ones) because
the question is existence, not enumeration. Actually — re-reading PRD §7.1: the predicate
is "at least one extension entry at any depth". If a dir IS a resolvable entry, we've
already found one and short-circuit. If it's a plain dir, descend. So the WalkDir callback
above is correct: resolvable dir ⇒ found=true (short-circuit), plain dir ⇒ descend.

This is subtly different from Index() (P1.M2.T2.S1) which must NOT descend into resolvable
dirs. But for the predicate, once we hit a resolvable dir we return immediately anyway,
so the distinction doesn't matter for correctness — both reach the same boolean.

### isExtensionFile helper (case a)
```go
func isExtensionFile(name string) bool {
    if name == "index.ts" || name == "index.js" {
        return false
    }
    return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")
}
```

### isResolvableDir helper (cases b + c)
```go
func isResolvableDir(dir string) bool {
    // (b) index.ts or index.js directly in dir
    if fileExists(filepath.Join(dir, "index.ts")) || fileExists(filepath.Join(dir, "index.js")) {
        return true
    }
    // (c) package.json with pi.extensions array naming ≥1 existing entry
    return hasPiExtensions(dir)
}
```

### hasPiExtensions helper (case c) — minimal, lenient
```go
func hasPiExtensions(dir string) bool {
    data, err := os.ReadFile(filepath.Join(dir, "package.json"))
    if err != nil { return false }
    var pkg struct {
        Pi struct {
            Extensions []string `json:"extensions"`
        } `json:"pi"`
    }
    if json.Unmarshal(data, &pkg) != nil { return false }
    for _, e := range pkg.Pi.Extensions {
        if fileExists(filepath.Join(dir, e)) { return true }
    }
    return false
}
```
Note: encoding/json into a typed `[]string` field silently drops non-string elements
(lenient per §7.3 "wrong-typed fields coerced or ignored"). If `pi.extensions` is not
present, the slice is nil → loop is a no-op → false. Correct.

These helpers (isExtensionFile, isResolvableDir, hasPiExtensions, fileExists) are
package-internal (lowercase). They may be reused by Index()/discover later, but for THIS
subtask they live in extdir.go (or a sibling internal file) and are tested only via
HasExtensionEntry's behavior.

## Module path
`github.com/dabstractor/weave` (verified go.mod). Import path for config:
`github.com/dabstractor/weave/internal/config`. BUT this subtask does NOT import config
yet — findConfig (rule 2) is P1.M1.T3.S2. So extdir.go for S1 has NO import of config.
This is important: S1 must compile standalone WITHOUT config existing-yet-being-unimported.
Actually config WILL exist (parallel: P1.M1.T2.S1 is being implemented in parallel and is
"Ready"). But S1 doesn't need it. Keep imports minimal: errors, encoding/json, io/fs, os,
path/filepath, strings.

## findEnv details pinned by skilldozer tests
- TestFindEnvUnset: LookupEnv returns ok=false → ("",0,false)
- TestFindEnvEmpty: "" → ("",0,false)  [the `val == ""` guard]
- TestFindEnvExistingDir: t.TempDir() → found, abs, SourceEnv
- TestFindEnvNonexistent: stat fails → false
- TestFindEnvRegularFile: stat ok but !IsDir → false
- TestFindEnvRelativePathAbsolutized: "." → filepath.Abs(".") → found
- TestFindEnvDoesNotResolveSymlinks: symlink-to-dir → returns the SYMLINK path (absolutized),
  NOT the resolved target. os.Stat follows the symlink (so IsDir=true), but we do NOT call
  EvalSymlinks. Pinned contract.

## HasExtensionEntry test cases (port/adapt from HasSkillMD tests)
- TestHasExtensionEntryFoundNestedFile: skills/a/b/foo.ts → true (case a, nested)
- TestHasExtensionEntryFoundShallowFile: skills/foo.ts → true (case a)
- TestHasExtensionEntryFoundIndexDir: skills/pkg/ with index.ts → true (case b)
- TestHasExtensionEntryFoundPackageJSON: skills/pkg/ with package.json{pi.extensions:["./src/index.ts"]} + src/index.ts → true (case c)
- TestHasExtensionEntryPackageJSONNoExistingEntry: package.json{pi.extensions:["./missing.ts"]} → false (case c requires ≥1 existing)
- TestHasExtensionEntryEmptyDir: empty dir → false
- TestHasExtensionEntryOnlyNonEntryFiles: only README.md → false
- TestHasExtensionEntryOnlyIndexFileAtRoot: index.ts at root of dir → false (index.* is NOT a single-file entry; but the ROOT dir containing index.ts IS a resolvable dir → case b → TRUE). 
  **CAREFUL**: HasExtensionEntry(root) where root/index.ts exists → case (b) applies to root itself → TRUE. This matches PRD §7.1 (a dir containing index.ts is an entry).
- TestHasExtensionEntryIgnoresIndexAsFile: a standalone index.ts NOT making a dir entry — but per §7.1 index.ts/index.js as a FILE is explicitly excluded from case (a). If root has ONLY index.ts and nothing else, is root a resolvable dir (case b)? YES — root contains index.ts. So TRUE. If a SUBDIR has only index.ts, that subdir is an entry → TRUE.
- TestSourceString: 4 labels + "unknown" for out-of-range
