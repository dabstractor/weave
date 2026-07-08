# Task for scout

In the weave codebase at /home/dustin/projects/weave, I need to confirm the consumer contract for the `discover.Index()` function I'm about to implement.

Please investigate:

1. Read /home/dustin/projects/weave/main.go and identify ALL places where `discover.Index()` would be called (it's not called yet — main.go currently only does --version and --path). Confirm that main.go imports neither `discover` nor `extdir.Find()` together yet (extdir.Find is used for --path mode). List the run() dispatch structure so I understand where Index() will later be wired.

2. Check the architecture mapping at /home/dustin/projects/weave/plan/001_19b4b465824d/architecture/architecture_mapping.md §3d — confirm the exact Index() signature and behavior spec.

3. Read /home/dustin/projects/weave/internal/discover/extension.go and confirm the exact `Extension` struct field names and `BuildExtension` signature that Index() will populate via the classify functions.

4. Check whether there are ANY existing tests in the discover package that exercise a full walk (grep for WalkDir in /home/dustin/projects/weave/internal/discover/). I expect NONE since Index doesn't exist yet, but confirm.

5. Verify go.mod has module path `github.com/dabstractor/weave` and go 1.25 (so I know import paths and that t.Chdir / t.TempDir are available).

Return a concise summary with:
- The exact consumer wiring points (line numbers in main.go where Index will later be called)
- Confirmation of the Extension struct fields and BuildExtension signature
- Confirmation that no walk tests exist yet
- The go.mod module path and version

---
**Output:**
Write your findings to exactly this path: /home/dustin/projects/weave/.pi-subagents/artifacts/outputs/1c458943/context.md
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