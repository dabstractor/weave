# Task for researcher

Research bash install-script best practices for a Go CLI installer that builds a binary and symlinks it into a PATH dir. I need AUTHORITATIVE references (URLs with section anchors where possible) and precise semantics for each point below. This feeds a PRP for an AI implementer, so be concise and factual. Do NOT write files — return the research as text.

1. `ln -sfn <target> <link>` vs `ln -sf` — what exactly does the `-n` (--no-dereference) flag do, and WHY does it matter when the destination already exists as a symlink-to-a-directory? Give the GNU coreutils `ln` documentation URL and a concrete failure example of `ln -sf` creating a nested dir.

2. `set -euo pipefail` strict mode — what each of `-e`, `-u`, `-o pipefail` does, and known GOTCHAS (`set -e` suppressed inside `if`/`||`/`&&`; `${VAR:-}` needed under `-u`; command-substitution failures masked without pipefail). Give the GNU bash manual URL sections.

3. `command -v go` — why this is the POSIX-correct existence check vs `which` (non-portable, not POSIX). Give the POSIX sh spec URL for `command -v`.

4. Shell detection for PATH setup: `$(basename "$SHELL")` and the correct rc-file line for bash (`~/.bashrc`), zsh (`~/.zshrc`), fish (`~/.config/fish/config.fish` + `fish_add_path`). Confirm `fish_add_path` is the idiomatic fish command, give its docs URL. Note the convention: PRINT the line only, never auto-edit rc files (why: intrusive, duplicates on re-run).

5. `go build -trimpath -ldflags "-s -w -X main.version=<v>"` — what each flag does: `-trimpath` (reproducible builds, strips $GOPATH/paths from panics), `-s` (strip symbol table), `-w` (strip DWARF), `-X importpath.name=value` (set a string var at link time). Give `go help build` / `go tool link` / Go ldflags documentation URLs.

6. PATH-already-present detection idiom: `case ":${PATH:-}:" in *":$TARGET:"*) ;;` — explain the leading/trailing colon padding trick and why it avoids partial-match false positives (e.g. TARGET=/bin matching /usr/bin).

7. shellcheck — confirm it is the standard bash linter, give its URL (shellcheck / github), and note which SC codes commonly fire on install scripts (e.g. SC2086 unquoted vars, SC2155 declare-and-assign masking).

For each: precise semantics + authoritative URL + concrete gotcha. Be concise.

---
**Output:**
Write your findings to exactly this path: /home/dustin/projects/weave/.pi-subagents/artifacts/outputs/66106db7/research.md
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