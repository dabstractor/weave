# Verified Facts — P1.M5.T1.S1 (`--help` usage + run() precedence + exclusivity + no-args)

Consolidated load-bearing facts for the PRP. All line numbers are from the CURRENT
working tree (M4.T4.S2 already merged: `exampleExtensionTemplate` L71/84, `chooseStore`
L777, `resolveStore` L821, `setupStore` L862, `runInit` L899, init dispatch L411-423).

---

## §1 — The two files under edit, and what is ALREADY there

- `main.go` (798 lines): has `version` var (L43), `isTerminal` var, full `parseArgs`
  (captures `c.help`, `c.version`, `c.unknownFlag`, `c.init`, `c.initStore`, `c.check`,
  `c.tags`, all modifiers since M1.T4.S1), and `run()` whose CURRENT ladder is:
  - L403-410 `// 1) --version` → `if c.version { fmt.Fprintf(stdout, "weave %s\n", version); return 0 }`
  - L411-423 `// 1.5) weave init dispatch` → `if c.init { return runInit(c, stdout, stderr) }`
  - L424 `// 2) --path` … (path, list, search, check, all, tags)
  - L640-643 `// 8) All other parsed modes are no-ops …` → `return 0`  ← THIS IS THE LINE TO CHANGE
- `main_test.go` (1509 lines): 78 `Test*` funcs. The ONLY existing test that BREAKS under
  the new ladder is `TestRunNoArgsIsNoOp` (L554-563). All other run()-level tests pass
  unchanged (verified by scout). parseArgs-level tests are unaffected.

**There is NO `usageText`, NO `usage()`, NO `c.help`/`unknownFlag`/`exclusivityError`/
no-args-stderr dispatch in `run()` yet. This task lands EXACTLY those.**

## §2 — weave has NO `storeMissingValue` (CRITICAL delta vs skilldozer)

`grep -rn storeMissingValue main.go main_test.go` → CONFIRMED none. skilldozer's run()
has a step 3.5 (`c.storeMissingValue → exit 2`); **weave does NOT**. weave's `parseArgs`
deliberately defers "--store needs a value" repo-wide (its `--store` case comment: "no
exit-2 'needs argument' here; the codebase defers that repo-wide"). So weave's precedence
ladder is ONE STEP SHORTER than skilldozer's:

```
skilldozer: help → version → unknownFlag → storeMissingValue → exclusivity → init → dispatch → no-args
weave:      help → version → unknownFlag → exclusivity → init → dispatch → no-args   (no storeMissingValue)
```

Do NOT port skilldozer's `TestRun*StoreNoValue*` family — weave has no signal to assert on.

## §3 — run() precedence ladder to implement (7 steps; the new top + new bottom)

Insert into `main.go` `run()`:

1. **help** — INSERT a NEW block BEFORE the `// 1) --version` comment (L403):
   `if c.help { fmt.Fprint(stdout, usage()); return 0 }` (usage → STDOUT, exit 0). Help is
   the "help wins" tiebreak (PRD §6.3): beats version AND unknownFlag AND exclusivity.
2. **version** — ALREADY PRESENT (L406-410), stays as-is.
3. **unknownFlag** — INSERT a NEW block AFTER the version block's `}`, BEFORE `// 1.5) init` (L411):
   `if c.unknownFlag != "" { fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag); return 2 }`
   (PRD §6 header "Unknown flags ⇒ error + exit 2"; stdout stays EMPTY for §6.4 $(...) safety).
4. **exclusivity** — INSERT a NEW block AFTER unknownFlag, BEFORE `// 1.5) init`:
   `if bad, msg := exclusivityError(c); bad { fmt.Fprintln(stderr, msg); return 2 }`
   (PRD §6.3; checked AFTER unknownFlag so `--bogus --list` reports the unknown flag first).
5. **init** — ALREADY PRESENT (L420-422), stays where it is; exclusivity now correctly GATES it.
6. **mode dispatch** — UNCHANGED (path/list/search/check/all/tags branches, L424+).
7. **no recognized mode** — CHANGE the final `return 0` (L643) to:
   `fmt.Fprint(stderr, usage()); return 1` (PRD §6.3: usage to STDERR, exit 1).

The insert anchors are the EXACT comment lines `// 1) --version` and `// 1.5) weave init
dispatch` — stable, unique strings in the file.

## §4 — `exclusivityError` (byte-identical to skilldozer except the `weave:` prefix)

`func exclusivityError(c config) (bad bool, msg string)` — six families, checked IN ORDER
(first hit wins; first message returned). Port from skilldozer main.go L722-770 verbatim,
swap the `skilldozer:` message prefix → `weave:`:

```
(a) n := count of {c.path, c.list, c.searchMode, c.all} set; if n >= 2 →
        "weave: listing modes --path/--list/--search/--all are mutually exclusive"
(b) if len(c.tags) > 0 && (c.path || c.list || c.searchMode || c.all) →
        "weave: tags cannot be combined with --path/--list/--search/--all"
(c) if c.check && len(c.tags) > 0 →
        "weave: 'check' cannot be combined with tag arguments"
(d) if c.check && (c.path || c.list || c.searchMode || c.all) →
        "weave: 'check' cannot be combined with --path/--list/--search/--all"
(e) if c.init && len(c.tags) > 0 →
        "weave: 'init' cannot be combined with tag arguments"
(f) if c.init && (c.check || c.list || c.searchMode || c.all || c.path) →
        "weave: 'init' cannot be combined with --list/--search/--all/--path/check"
else return false, ""
```

**RECONCILING family (b) with the item description:** the item's (b) literally reads
"tags + (list/searchMode/all)" — omitting `--path`. But PRD §6 header mandates the CLI
contract is **byte-identical to skilldozer's**, and skilldozer's Issue 3 deliberately
ADDED tags+path to avoid silently dropping a stray tag on `weave foo --path` (it would
otherwise print the dir and discard `foo` with no error). The item's (b) is an oversight
— families (d) and (f) in the SAME item description DO include `--path`, confirming path
belongs in the set. **Decision: include `--path` in (b)** (match skilldozer / PRD
byte-identity). The item's stated tests (tag --list, check tag) still pass; we ADD
coverage for tag --path. Documented as a CRITICAL decision in the PRP.

**Modifiers `--file`/`--relative`/`--no-color` NEVER trigger exclusivity** — they combine
with a single mode (e.g. `--all --file`, `--list --no-color`). Confirmed by the existing
tests `TestRunListNoColorFlagSuppressesANSI`, `TestRunAllFilePrintsAllEntryFiles`, etc.

## §5 — `usageText` + `usage()` (the user-facing help)

`const usageText = <raw string literal>` placed at package scope near the other consts
(`version` L43 / `exampleExtensionTemplate` L84). `func usage() string { return usageText }`
wraps it so every print site is uniform (`fmt.Fprint(w, usage())`). Same text → STDOUT for
`--help` (exit 0) and → STDERR for no-args (exit 1); only destination + exit differ.

### Structure (mirror skilldozer main.go L52-97, adapt for weave):
- **Tagline** (line 1): `weave — manifest-free extension path printer`
  (the milestone title is "weave v1.0 — Manifest-free extension path printer"; em-dash `—`
  U+2014, NOT hyphen-minus — matches skilldozer's tagline character).
- **Description** (line 3): `Resolve extension tags to on-disk extension paths (manifest-free).`
- **USAGE:** header + 9 invocation lines (2-space indent): `weave <tag>…`, `--all`, `--list`,
  `--search <query>`, `check`, `init [<dir>]`, `--path`, `--help`, `--version`.
- **EXAMPLES:** header + 9 lines. The canonical one-liner is `pi -e "$(weave example)"`
  and `pi -e "$(weave writing/reddit)"` (NOT `pi --skill`). `-f example` → "print the entry
  file path". Comments aligned with skilldozer's spacing.
- **OPTIONS:** header + 13 lines, two-column, 2-space indent. Option-spec field
  left-justified width 16 + 3-space gap → descriptions begin at column 21. FIX the latent
  skilldozer alignment bug: skilldozer's `init [<dir>]` and `--store <dir>` rows start at
  column 20 (one space short); weave pads them to column 21 like every other row.
- **Exit codes:** trailing line: `0 success/help/version | 1 unresolved/no extensions/unresolvable dir | 2 unknown flag / mutually-exclusive modes`
- **Trailing newline:** EXACTLY one `\n` after the Exit codes line (closing backtick on its
  own line). No trailing blank line.

### The `--file` row (the ONE semantic delta): `Print the entry file path instead of the resolvable path (modifier)` — the entry file (.ts/.js pi loads: file itself, index.ts, or first pi.extensions entry). NOT SKILL.md.

## §6 — The single test to UPDATE + the new tests to ADD

### UPDATE (1): `TestRunNoArgsIsNoOp` (main_test.go L554-563)
Flip from "exit 0, no output" to "exit 1, usage on STDERR, stdout EMPTY". Rename to
`TestRunNoArgsPrintsUsageExit1` (the name "IsNoOp" becomes a lie). New body:
```go
var out, errOut bytes.Buffer
code := run(nil, &out, &errOut)
if code != 1 { t.Errorf("run(nil): code=%d; want 1 (no-args → usage on stderr, exit 1)", code) }
if out.Len() != 0 { t.Errorf("run(nil) stdout=%q; want EMPTY (usage goes to stderr)", out.String()) }
if !strings.Contains(errOut.String(), "USAGE") { t.Errorf("run(nil) stderr=%q; want the usage block", errOut.String()) }
```

### ADD (~12 new tests) under a `// --- run: CLI contract — help / precedence / unknown / exclusivity / no-args (M5.T1.S1) ---` header. Port the SHAPE from skilldozer main_test.go; weave deltas = binary name, the `weave:` prefix, the `pi -e` one-liner. Coverage (each asserts code + which stream + empty-stdout discipline):
1. `TestRunHelpToStdoutExit0` — `--help` → exit 0, stdout has USAGE/EXAMPLES/OPTIONS + `pi -e "$(weave example)"`, no ANSI, stderr empty.
2. `TestRunHelpShortFlag` — `-h` → exit 0, stdout has USAGE.
3. `TestRunHelpBeatsVersion` — `--help --version` → exit 0, stdout IS usage (does NOT contain `weave dev`).
4. `TestRunHelpBeatsUnknownFlag` — `--help --bogus` → exit 0, usage on stdout, stderr empty (help wins over unknown).
5. `TestRunVersionBeatsUnknownFlag` — `--version --bogus` → exit 0, stdout == `weave dev\n`, stderr empty (version beats unknown).
6. `TestRunUnknownFlagExit2` — `--frobnicate` → exit 2, stdout empty, stderr == `weave: unknown flag '--frobnicate'\n`.
7. `TestRunUnknownShortFlagExit2` — `-z` → exit 2, stdout empty, stderr == `weave: unknown flag '-z'\n`.
8. `TestRunExclusivityListAndSearch` — `--list --search foo` → exit 2, stdout empty, stderr has "mutually exclusive".
9. `TestRunExclusivityTagsAndPath` — `foo --path` → exit 2, stdout empty, stderr has "cannot be combined" (the (b)+path case).
10. `TestRunExclusivityTagsAndList` — `foo --list` → exit 2, stdout empty, stderr has "cannot be combined".
11. `TestRunExclusivityCheckAndTags` — `check foo` → exit 2, stdout empty, stderr has "check".
12. `TestRunExclusivityInitAndList` — `init --list` → exit 2, stdout empty, stderr has "init".
13. `TestRunExclusivityInitAndStrayTag` — `init foo` → exit 2, stdout empty, stderr has "tag".
14. `TestRunModifiersOnlyNoMode` — `--no-color` alone → exit 1, stdout empty, stderr has USAGE (modifiers are not a mode).
(OPTIONAL) `TestExclusivityErrorPredicate` — call `exclusivityError` directly over a config table, incl. the modifier-doesn't-count cases (all+file, list+noColor, path+relative → bad=false). Port from skilldozer's predicate test.

None of the exclusivity/unknown/help tests need a store fixture or env setup — they all exit
BEFORE `extdir.Find`, so `unsetExtEnv`/`t.Chdir` are not required (no dir resolution runs).

## §7 — Validation commands (verified working in this repo)

```bash
go build ./...                              # compile
go vet ./...                                # vet
go test ./... -v                            # full suite
go test -run 'TestRunHelp|TestRunUnknown|TestRunExclusivity|TestRunNoArgs|TestRunVersionBeats|TestRunModifiersOnly' -v   # targeted
gofmt -l main.go main_test.go               # empty output expected
```
Manual (build + §13-shaped smoke):
```bash
go build -o /tmp/weave . && /tmp/weave --help | head        # usage to stdout
/tmp/weave --bogus; echo "exit=$?"                           # expect exit=2
/tmp/weave; echo "exit=$?"                                   # no-args: expect exit=1, usage on stderr
/tmp/weave --list --search x; echo "exit=$?"                # expect exit=2
```

## §8 — Scope discipline (what NOT to touch)

- NO new files, NO new imports (everything used — `fmt`, `io`, `os` — is already imported).
- NO `internal/*` change. NO `go.mod`/`go.sum` change. NO `parseArgs` change (already
  captures every flag since M1.T4.S1). NO `storeMissingValue` (deliberately deferred).
- NO PRD.md / tasks.json edits.
- The existing `TestRunVersionPrecedenceOver*` tests (Path/Tag/All) still PASS but their
  NAMES/comments now overstate ("version takes precedence over everything") since help now
  precedes version. Cosmetic; OPTIONAL non-blocking cleanup (do not let it expand scope).
