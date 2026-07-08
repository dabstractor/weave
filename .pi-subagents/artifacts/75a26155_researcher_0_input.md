# Task for researcher

Research the EXACT semantics of Go's `path/filepath.WalkDir` for implementing a "classify-then-descend" directory walk. I need PRECISE documented answers (cite pkg.go.dev path/filepath) to:

1. When a WalkDir callback returns `filepath.SkipDir` for a DIRECTORY entry, does WalkDir skip that directory's ENTIRE subtree (all descendants) AND continue with the next sibling entry? Confirm exact behavior.

2. Is the root directory passed to the callback BEFORE its children? Can the callback classify and SkipDir the root itself?

3. When the callback returns SkipDir for a FILE entry, what happens?

4. Confirm: WalkDir does NOT follow symlinks by default.

5. `fs.DirEntry.IsDir()` reliability for file vs dir distinction.

6. Traversal order: lexical by filename within each directory? Confirm.

7. `filepath.Rel(basepath, targpath)` — what does it return when targpath is under basepath? And `filepath.ToSlash` — replaces OS separator with `/`?

Give a concise (under 400 words) summary with exact documented behaviors and URLs. This is for a Go walker that must NOT recurse into directories recognized as extension units (only descend into "plain" category folders), implemented via WalkDir + SkipDir.

---
**Output:**
Write your findings to exactly this path: /home/dustin/projects/weave/.pi-subagents/artifacts/outputs/75a26155/research.md
This path is authoritative for this run.
Ignore any other output filename or output path mentioned elsewhere, including output destinations in the base agent prompt, system prompt, or task instructions.

## Acceptance Contract
Acceptance level: checked
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Implement the requested change without widening scope

Required evidence: changed-files, tests-added, commands-run, residual-risks, no-staged-files

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```