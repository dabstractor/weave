# Task for scout

Two-part extraction from the skilldozer reference at /home/dustin/projects/skilldozer/.

PART A - Extract the EXACT skilldozer M5 test patterns to port to weave. Read /home/dustin/projects/skilldozer/main_test.go and find all tests for: --help (exit 0, usage on stdout), --help/--version precedence (which wins), unknown flag (exit 2, 'unknown flag' on stderr, empty stdout), exclusivity combos (2+ listing modes; tags+mode; check+tags; check+mode; init+tags; init+mode - each exit 2), and no-args (exit 1, usage on stderr, empty stdout). For each give: function name, the run() args it passes, and the exact assertion lines (code/exit/stream). Note which port DIRECTLY to weave and which need a weave delta (binary 'weave' not 'skilldozer'; 'extension' not 'skill'; pi -e one-liner; NO storeMissingValue step so skip any --store-no-value tests).

PART B - Read /home/dustin/projects/skilldozer/main.go usageText constant (around lines 52-97) and report its EXACT structure: the sections (tagline, USAGE:, EXAMPLES:, OPTIONS:, Exit codes: trailing line), the OPTIONS column alignment width, the trailing newline handling. Then describe precisely how to ADAPT it to weave: binary 'weave', tagline 'manifest-free extension path printer', the canonical one-liner is `pi -e "$(weave <tag>)"` (NOT pi --skill), list every weave PRD 6.1 command and 6.2 modifier with weave semantics (--file prints the ENTRY FILE not SKILL.md; store dir is 'extensions'). Do NOT write any files other than the report - output one markdown report to the output path with the usageText blueprint AND the test extraction.

---
**Output:**
Write your findings to exactly this path: /home/dustin/projects/weave/plan/001_19b4b465824d/P1M5T1S1/research/skilldozer_reference.md
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