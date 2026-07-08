# Task for scout

Exhaustively scan /home/dustin/projects/weave/main_test.go for EVERY test (function name + line number + exact assertion) that will BREAK or need to change when weave's run() precedence ladder is updated to the M5.T1.S1 contract.

NEW precedence (run() top): (1) c.help -> usage to STDOUT exit 0; (2) c.version -> version exit 0; (3) c.unknownFlag != "" -> 'weave: unknown flag X' to stderr exit 2; (4) exclusivityError(c) -> exit 2; (5) c.init; (6) mode dispatch (path/list/search/check/all/tags); (7) NO recognized mode -> usage to STDERR exit 1.

CURRENT behavior being changed: version is FIRST (help is currently a no-op exit 0 with empty output, not dispatched), no unknownFlag->exit2, no exclusivity, and no-args / current-final-return is `return 0` (exit 0).

I already know `TestRunNoArgsIsNoOp` asserts run(nil) == exit 0 and must flip to exit 1 + usage on stderr. Find ALL OTHERS. Specifically check every Test* func that calls run(...):
- Any test calling run([]string{...}) with an unknown dashed flag (e.g. --bogus, -z, a short bundle with an unknown char) asserting a NON-exit-2 code.
- Any test with mutually-exclusive combos (e.g. --list --search, a tag + --list, check + a tag, init + a tag, init + --list) asserting a NON-exit-2 code.
- Any test calling run([]string{"--help"}) or run([]string{"-h"}) asserting specific output/exit (help is currently a no-op exit 0 empty output).
- Any test asserting --version vs --help precedence.
- Confirm whether the parseArgs-level tests (TestParseArgs*) test run() dispatch or ONLY the parser (parser-only tests are unaffected by dispatch changes).

For EACH affected test, report: function name, line number, the exact current assertion line(s), and what it must change to (and whether it should be DELETED vs UPDATED). Read the ENTIRE main_test.go (~1509 lines). Do NOT modify files. Output one markdown report to the output path.

---
**Output:**
Write your findings to exactly this path: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M5T1S1/research/test_impact_scan.md
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