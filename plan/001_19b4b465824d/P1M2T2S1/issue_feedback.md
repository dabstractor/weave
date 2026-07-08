# Implementation Issue Feedback (Attempt 1/3)

The previous implementation attempt encountered an issue that requires re-planning.

## Issue Details

P1.M2.T2.S1 is fully and correctly implemented: discover.go (5 functions: fileExists, relTagForDir, isExtensionFile, classifyFile, classifyDir) and discover_test.go (15 tests) are created exactly per the PRP. All 4 validation levels pass for MY deliverables — gofmt clean, go build/vet exit 0, all 15 T2 tests pass (including TestWorkedExample, the PRD §7.1 acceptance test proving the load-bearing classify-then-descend recursion rule end-to-end: exactly 5 entries gate/git-checkpoint/summarizer/writing/reddit-poster/platform/linux, with utils.ts and src/index.ts correctly NOT double-counted), race-clean, stdlib-only imports, no forbidden imports, exported API surface correct (T2 symbols all unexported), and the duplicated isExtensionFile/fileExists predicates verified byte-for-byte identical to extdir's copies. HOWEVER, the gate 'go test ./internal/discover/... passes ALL tests (S1+S2+T2)' cannot fully close due to ONE pre-existing failure that is NOT in my code: S1's TestParsePackageJSONLenientWrongTypes (internal/discover/extension_test.go:184, committed in 6acaed8) fails because Go's encoding/json hard-errors when a JSON scalar like \

## Full Agent Output

The implementation is complete and correct. Let me provide the final report.

## Summary

I implemented **P1.M2.T2.S1** (`classifyEntry` + classify-then-descend recursion rules) exactly per the PRP.

### Deliverables (2 new files)
- **`internal/discover/discover.go`** (157 lines): the 3 specified functions (`isExtensionFile`, `classifyFile`, `classifyDir`) plus 2 package-private helpers (`fileExists`, `relTagForDir`). Stdlib-only imports (`os`, `path/filepath`, `strings`). No walk logic (T3 owns that), no package doc comment (S1 owns it), no `encoding/json` (calls S1's `parsePackageJSON`), no `extdir` import (predicates re-implemented locally and verified byte-for-byte identical to extdir's copies).
- **`internal/discover/discover_test.go`** (505 lines, 15 test functions): all PRP-required unit tests, the two Level-3 integration tests, and **`TestWorkedExample`** — the PRD §7.1 acceptance test that builds the exact tree (`gate.ts`, `git-checkpoint/`, `summarizer/`, `writing/reddit-poster.ts`, `platform/linux/`) and asserts exactly 5 entries with correct relTags/kinds, proving the load-bearing recursion rule (`utils.ts` and `src/index.ts` are NOT double-counted).

### Validation gates passed
- **Level 1 (syntax/style):** `gofmt -l` clean, `go build ./...` exit 0, `go vet ./...` exit 0, no forbidden imports, no `require`/`go.sum`, no walk tokens in discover.go, all 3 signatures present, package-doc count = 0.
- **Level 2 (unit):** all 15 T2 tests pass; all 3 S2 tests pass. Race-clean.
- **Level 3 (integration):** `TestClassifyDirPackageFullMetadata` + `TestClassifyFileJSDocOnlyDescription` pass (T2 correctly composes S1's `parsePackageJSON`/`BuildExtension` + S2's `ExtractJSDoc`).
- **Level 4 (domain):** `TestWorkedExample` passes; exported API surface is exactly S1's `Extension`/`BuildExtension` + S2's `ExtractJSDoc` (all T2 symbols correctly unexported); `discover.isExtensionFile`/`fileExists` verified identical to extdir's copies.

### One pre-existing blocker (NOT introduced by this subtask)
A single test fails in the discover package: **S1's `TestParsePackageJSONLenientWrongTypes`** (in `extension_test.go`, committed in `6acaed8`). This is a documented S1 defect — see `plan/001_19b4b465824d/P1M2T1S1/issue_feedback.md`, where the S1 implementer halted and reported that the test's premise is empirically false: Go's `encoding/json` hard-errors when a JSON scalar (e.g. `"keywords": "notarray"`) is unmarshaled into a `[]any` field, exactly as it does for `[]string`. The S1 code was committed with the failing test anyway.

This failure is **outside my PRP's scope** ("No other files change" — my deliverables are `discover.go` + `discover_test.go` only), and **independent of my work** — my T2 functions compose `parsePackageJSON`/`BuildExtension`/`ExtractJSDoc` whose working contracts I verified directly, and all 15 of my tests pass. Fixing S1 would require editing `extension.go`/`extension_test.go` (a "Complete" subtask's contract), which my PRP forbids. The S1 `issue_feedback.md` already documents the two viable fixes (type array fields as `any`, or use `json.RawMessage` + manual decode) for whoever owns the S1 PRP to apply.

```json
{
   "result": "issue",
   "message": "P1.M2.T2.S1 is fully and correctly implemented: discover.go (5 functions: fileExists, relTagForDir, isExtensionFile, classifyFile, classifyDir) and discover_test.go (15 tests) are created exactly per the PRP. All 4 validation levels pass for MY deliverables — gofmt clean, go build/vet exit 0, all 15 T2 tests pass (including TestWorkedExample, the PRD §7.1 acceptance test proving the load-bearing classify-then-descend recursion rule end-to-end: exactly 5 entries gate/git-checkpoint/summarizer/writing/reddit-poster/platform/linux, with utils.ts and src/index.ts correctly NOT double-counted), race-clean, stdlib-only imports, no forbidden imports, exported API surface correct (T2 symbols all unexported), and the duplicated isExtensionFile/fileExists predicates verified byte-for-byte identical to extdir's copies. HOWEVER, the gate 'go test ./internal/discover/... passes ALL tests (S1+S2+T2)' cannot fully close due to ONE pre-existing failure that is NOT in my code: S1's TestParsePackageJSONLenientWrongTypes (internal/discover/extension_test.go:184, committed in 6acaed8) fails because Go's encoding/json hard-errors when a JSON scalar like \"keywords\":\"notarray\" is unmarshaled into a []any field — contradicting the test's premise that []any makes encoding/json fully lenient. This is a documented S1 defect: the S1 implementer halted and reported it in plan/001_19b4b465824d/P1M2T1S1/issue_feedback.md, but the code was committed with the failing test intact. My PRP explicitly says 'No other files change' and scopes my deliverables to discover.go + discover_test.go only, so fixing S1's extension.go/extension_test.go (a Complete subtask's contract) is outside my authority. S1's other 10 tests pass, S2's 3 tests pass, all 15 of my T2 tests pass, and config/extdir/main all pass. The S1 issue_feedback.md already documents two viable fixes (type array fields as `any`, or use json.RawMessage + manual decode) for the S1 PRP owner to apply. My T2 implementation is correct, complete, and ready to be consumed by T3's Index() walk."
}
```

## Instructions

Review this feedback and create a revised PRP that addresses the issue.
If the issue is fundamentally impossible to resolve, output `"result": "fail"` with an explanation.
