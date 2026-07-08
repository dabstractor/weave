# Research Notes — P1.M1.T3.S3 (Walk-up rule 4 + Find() combiner + ErrNotFound)

## 1. Skilldozer source-of-truth ports (verbatim, with documented renames)

All verified by reading `/home/dustin/projects/skilldozer/internal/skillsdir/skillsdir.go`
(lines 218-301) and `skillsdir_test.go` (lines 352-535).

### findWalkUpAncestor — the testable ascent core (skilldozer lines 231-247)
```go
func findWalkUpAncestor(start string) (dir string, found bool) {
	cur := filepath.Clean(start)
	for {
		candidate := filepath.Join(cur, "skills")       // weave: "extensions"
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if HasSkillMD(candidate) {                  // weave: HasExtensionEntry(candidate)
				return candidate, true
			}
			// skills/ exists here but has no SKILL.md -> keep ascending.
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false // reached filesystem root, no match
		}
		cur = parent
	}
}
```
**WEAVE CHANGES (exactly two):**
1. `"skills"` → `"extensions"` (the sibling dir literal).
2. `HasSkillMD(candidate)` → `HasExtensionEntry(candidate)` (the predicate from S1, already exported).

The loop structure, the `filepath.Dir(cur)` root-detection (`parent == cur`), the skip-empty-and-continue behavior, and the "check start dir FIRST" semantics port verbatim.

### findWalkUp — rule 4 entry (skilldozer lines 256-269)
```go
func findWalkUp() (dir string, src Source, found bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, false // cwd unresolvable -> rule misses
	}
	d, ok := findWalkUpAncestor(cwd)
	if !ok {
		return "", 0, false
	}
	return d, SourceWalkUp, true
}
```
**WEAVE CHANGES: none.** Body ports verbatim. The "factored-out testable core" design is critical: `os.Getwd()` is not directly controllable from a test, so the testable ascent lives in `findWalkUpAncestor(start)`. `findWalkUp` is the thin entry calling `os.Getwd()` + delegating.

### ErrNotFound (skilldozer line 275)
```go
var ErrNotFound = errors.New("skilldozer is not configured; run `skilldozer init`")
```
**WEAVE:**
```go
var ErrNotFound = errors.New("weave is not configured; run `weave init`")
```
Exact PRD §8.2 / §6.4 user-facing string. The item contract pins it verbatim:
`errors.New("weave is not configured; run \`weave init\`")`.

### Find — the public combiner (skilldozer lines 288-301)
```go
func Find() (dir string, src Source, err error) {
	if d, s, ok := findEnv(); ok {
		return d, s, nil
	}
	if d, s, ok := findConfig(); ok {
		return d, s, nil
	}
	if d, s, ok := findSibling(); ok {
		return d, s, nil
	}
	if d, s, ok := findWalkUp(); ok {
		return d, s, nil
	}
	return "", 0, ErrNotFound
}
```
**WEAVE CHANGES: none.** Body ports verbatim. This calls all four rule helpers in priority order (findEnv rule 1 → findConfig rule 2 → findSibling rule 3 → findWalkUp rule 4) and returns the first hit as `(absDir, src, nil)`, or `("", 0, ErrNotFound)` on total miss. NEVER prompts (PRD §8.2 prompt safety).

## 2. Critical insight: ErrNotFound is EXACTLY the rule-5 miss case

PRD §8.3 lists 5 cases. Cases 1-4 are the four find* helpers. Case 5 ("None ⇒ unconfigured") is NOT a helper — it IS the `return "", 0, ErrNotFound` line at the bottom of Find(). No `findNone` function exists; the fall-through return encodes rule 5 directly.

## 3. Test patterns to port (from skilldozer_test.go)

### makeSkill helper → adapt to makeExtension
skilldozer writes `skills/<tag>/SKILL.md`. weave's extension store is `extensions/`,
and `HasExtensionEntry` recognizes a `*.ts`/`*.js` file (case a, not index.*). So the
makeExtension helper writes `extensions/<tag>/<tag>.ts`:
```go
func makeExtension(t *testing.T, dir, tag string) string {
	t.Helper()
	ext := filepath.Join(dir, "extensions")
	entry := filepath.Join(ext, tag)
	if err := os.MkdirAll(entry, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", entry, err)
	}
	if err := os.WriteFile(filepath.Join(entry, tag+".ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s.ts: %v", tag, err)
	}
	return ext
}
```
This makes `HasExtensionEntry(<dir>/extensions)` return true via case (a) — the
single-file entry kind from PRD §7.1 that S1's predicate recognizes.

### findWalkUpAncestor tests (testable core, no cwd dependency)
Port VERBATIM with `makeSkill`→`makeExtension`, `"skills"`→`"extensions"`, and the
nested-SKILL.md case becomes a nested-`*.ts` case:
- `TestFindWalkUpAncestorAtStart` — start IS the repo → `extensions` at start wins.
- `TestFindWalkUpAncestorDeep` — extensions several levels up → ascent finds it.
- `TestFindWalkUpAncestorNestedEntry` — a nested `extensions/x/y/foo.ts` counts
  (HasExtensionEntry recurses, case a).
- `TestFindWalkUpAncestorSkipsEmptyAndContinues` — `extensions/` dir that exists
  but has NO entries is SKIPPED; ascent continues to a higher ancestor that DOES
  have one. PRD §8.3 qualifies the match with "at least one extension entry."
- `TestFindWalkUpAncestorNoExtensions` — no extensions anywhere → miss.
- `TestFindWalkUpAncestorExtensionsIsFile` — `extensions` is a regular FILE → miss
  (IsDir guard).

### findWalkUp test (os.Getwd exercised via t.Chdir)
Go 1.24+ `t.Chdir` changes cwd for the test and restores on cleanup, so findWalkUp
(which calls os.Getwd) is testable without global cwd pollution. go.mod says `go 1.25`,
so t.Chdir is available.
- `TestFindWalkUpFindsAncestor` — `t.Chdir(sub)` into a subdir of a temp repo,
  findWalkUp resolves to the repo's `extensions/`, returning SourceWalkUp.

### Find tests (the public combiner)
Port VERBATIM with renames:
- `TestFindRuleEnvWins` — unsetEnvVar(t); t.Setenv(envVar, dir); Find() → SourceEnv, nil err.
- `TestFindRuleWalkUpWins` — unsetEnvVar(t); makeExtension; t.Chdir(sub); Find() → SourceWalkUp.
  (findSibling deterministically misses in a test because the test binary runs from a
  temp build dir with no sibling extensions/ — same rationale as S2's findSibling smoke.)
- `TestFindAllMissReturnsErrNotFound` — unsetEnvVar(t); t.Chdir(t.TempDir()); Find() →
  ErrNotFound via errors.Is, dir="" src=0. The walk ascends to / which has no extensions/.
- `TestErrNotFoundMessageHasFix` — asserts ErrNotFound.Error() contains "run" and
  "weave init" (the exact user-facing fix from PRD §8.2/§6.4).

### unsetEnvVar — UPGRADE in S3 (depends on S2's upgrade OR re-done here)
S2's PRP (Task 5) upgrades unsetEnvVar to also neutralize weave_CONFIG (lowercase)
so findConfig deterministically misses. **This is a SHARED dependency.** Two scenarios:
- If S2 lands FIRST: unsetEnvVar already neutralizes both envVar and weave_CONFIG.
  S3 needs NO change to unsetEnvVar — it's already correct. The Find tests inherit it.
- If S2 and S3 land in PARALLEL (which they are, per parallel_execution_context):
  both PRPs specify the same unsetEnvVar upgrade. The LAST writer wins; the helper
  body is identical either way. The S3 PRP must specify the full upgrade so S3 is
  self-contained even if S2 hasn't landed yet.

**The S3 PRP will specify the full unsetEnvVar (envVar + weave_CONFIG neutralization)
so S3 is self-contained.** If S2 already added it, the S3 implementation is a no-op
on that helper (idempotent).

## 4. Gotchas specific to S3

- **`t.Chdir` (Go 1.24+)** — go.mod declares `go 1.25`, so available. Restores cwd on
  cleanup. Required for findWalkUp and Find walk-up tests. Use `t.Chdir(sub)` not
  `os.Chdir` + manual restore.
- **findWalkUpAncestor checks the START dir FIRST** — `cur := filepath.Clean(start)`,
  then the loop body runs before any ascent. So if start itself has `extensions/` with
  entries, it wins immediately (no ascent). Pinned by TestFindWalkUpAncestorAtStart.
- **Skip-empty-and-continue** — `extensions/` existing as a dir with NO entries does
  NOT count as a match; ascent continues. This is the PRD §8.3 "at least one extension
  entry" qualifier. Pinned by TestFindWalkUpAncestorSkipsEmptyAndContinues.
- **Root detection via `parent == cur`** — `filepath.Dir(root) == root` for the
  filesystem root. This is the loop termination; do NOT use a counter or depth limit.
- **Find never errors except ErrNotFound** — the only non-nil error Find can return is
  ErrNotFound. The rule helpers return `(dir, src, bool)` (no error); Find folds
  found=false into "try next" and only ErrNotFound escapes. Pinned by TestFindAllMiss.
- **Find never prompts** — PRD §8.2 prompt safety. The bare `weave <tag>` path never
  calls init interactively. Find just returns ErrNotFound; main (P1.M1.T4) prints it
  and exits 1. Do NOT add isatty / auto-init logic to Find.
- **findSibling deterministically misses in tests** — the test binary runs from a
  temp go-build dir with no sibling `extensions/`. This means TestFindRuleWalkUpWins
  can rely on rule 3 missing and rule 4 winning. (Same rationale as S2's
  TestFindSiblingNoExtensionsNextToTestBinary.)

## 5. Scope boundaries (what S3 does NOT do)

- Does NOT modify config package (P1.M1.T2.S1, Complete).
- Does NOT add findConfig/findSibling/resolveSiblingFromExe — those are S2 (parallel).
  S3 calls them via Find() but does NOT define them. If S2 hasn't landed, S3's
  Find() won't compile until S2 lands — that's expected; S3 and S2 are a parallel
  pair that together complete T3.
- Does NOT modify main.go — main wiring is P1.M1.T4.
- Does NOT touch HasExtensionEntry or any S1 symbol — S1 is Complete and unchanged.

## 6. Build/vet/test gates (verified available)

```bash
cd /home/dustin/projects/weave
go build ./...                    # exit 0 (needs S2 landed for Find to compile)
go vet ./internal/extdir/         # exit 0
go test ./internal/extdir/ -v     # ALL pass (S1 + S2 + S3 suites)
go test -race ./internal/extdir/  # no data races
grep -q '^require' go.mod && echo FAIL || echo OK   # OK (no require)
test ! -f go.sum && echo OK || echo FAIL            # OK (no go.sum)
```

## 7. Confidence

HIGH. This is a verbatim port of skilldozer's most-tested code path (rule 4 + Find +
ErrNotFound) with two mechanical renames (`"skills"`→`"extensions"`, `HasSkillMD`→
`HasExtensionEntry`) and one string change in ErrNotFound. Every test in skilldozer
ports 1:1. The predicate (HasExtensionEntry) already exists and is tested by S1.
