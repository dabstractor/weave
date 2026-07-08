# Research: P1.M2.T5.S1 (--list mode dispatch in run())

## 1. Port source (skilldozer main.go `if c.list` branch)

Verified verbatim from `/home/dustin/projects/skilldozer/main.go` lines ~478-505:

```go
if c.list {
	dir, _, err := skillsdir.Find()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	skills, err := discover.Index(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(skills) == 0 {
		fmt.Fprintln(stderr, "no skills found in "+dir)
		return 1
	}
	ui.PrintList(stdout, skills, isTerminal(stdout) && !c.noColor)
	return 0
}
```

### The two deltas (weave):
1. `skills` → `exts` (local var rename; TYPE already []Extension in weave's discover).
2. `"no skills found in "+dir` → `"no extensions found in "+dir` (noun swap; `+dir` suffix kept).

Everything else ports byte-for-byte. **Critical**: `src` is discarded (`dir, _, err`) — `--list` does NOT print the `(found via ...)` label (that's `--path`-only).

## 2. Function signatures consumed (all PRESENT in weave)

- `extdir.Find() (dir string, src Source, err error)` — `internal/extdir/extdir.go`. Returns `("", 0, ErrNotFound)` when unconfigured. `ErrNotFound.Error()` = `"weave is not configured; run \`weave init\`"` (backticks literal).
- `discover.Index(extensionsDir string) ([]Extension, error)` — `internal/discover/index.go`. Returns nil slice + nil error for empty store; nil + err for missing/non-dir root. Sorted by RelTag.
- `ui.PrintList(w io.Writer, exts []discover.Extension, useColor bool)` — `internal/ui/ui.go` (parallel P1.M2.T4.S1, treat as PRESENT). Empty slice → early return (prints nothing).

## 3. Test fixtures: writeSkillTree → writeExtTree

skilldozer's `writeSkillTree` writes `<tag>/SKILL.md` (markdown + YAML frontmatter). weave single-file extensions are `.ts` FILES (PRD §7.1). Ported helper writes `<tag>.ts` with a `/** ... */` JSDoc block (ExtractJSDoc requires the two-star opener). See PRP "Known Gotchas" for the exact code.

## 4. Test patterns ported (skilldozer main_test.go)

- `withTerminal(t, isTTY)` — lines ~18-24, pure plumbing, port verbatim. Swaps package-level `isTerminal` var; NOT t.Parallel-safe.
- `writeSkillTree` — lines ~41-54, adapt to `.ts`/JSDoc (see above).
- Six `TestRunList*` tests — lines ~315-430:
  - TestRunListSuccess (non-TTY → no ANSI)
  - TestRunListShortFlag (`-l`)
  - TestRunListNoSkillsExit1 → TestRunListNoExtensionsExit1 (empty store, exit 1)
  - TestRunListSkillsDirUnresolvableExit1 → TestRunListUnresolvableExit1 (unconfigured, exit 1, "weave init")
  - TestRunListNoColorFlagSuppressesANSI (TTY forced + --no-color → no ANSI)
  - TestRunListColorWhenTTY (TTY forced → ANSI bold/cyan/reset)

## 5. CRITICAL: TestRunUndispatchedModeIsNoOp must be modified

`/home/dustin/projects/weave/main_test.go` currently contains:
```go
func TestRunUndispatchedModeIsNoOp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--list"}, &out, &errOut)  // NOW WRONG
	if code != 0 { ... }
	if out.Len() != 0 { ... }
}
```
This was written in P1.M1.T4.S1 when `--list` was parsed-but-undispatched. NOW `--list` has real behavior. Fix: swap `"--list"` → `"--all"` (still undispatched until M3.T2.S1). If missed, `go test ./...` fails.

## 6. Insertion point in run()

After the `if c.path { ... }` block (ending `return 0` + `}`), before the `// 3) All other parsed modes are no-ops` comment. Renumber fallthrough `// 3)` → `// 4)`, drop "M2 adds --list" stale clause.

## 7. Imports

main.go currently imports: fmt, io, os, strings + `internal/extdir`. ADD `internal/discover` (before extdir) and `internal/ui` (after extdir). Both are NEW (run() did not call discover or ui before).

main_test.go currently imports: bytes, path/filepath, strings, testing. ADD `os` (for os.WriteFile in writeExtTree).
