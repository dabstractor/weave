# Skilldozer Reference — M5.T1.S1 Extraction & Weave Adaptation

Source of truth: `/home/dustin/projects/skilldozer/main.go` and `/home/dustin/projects/skilldozer/main_test.go`.
Target weave state verified: `main.go` has `var version = "dev"` (line 43), `parseArgs` (with `unknownFlag`, `help` fields wired), but **no** `usageText` constant, **no** `usage()` fn, **no** `if c.help`/`unknownFlag`/`exclusivityError`/no-args-stderr dispatch in `run()`, and **no** `storeMissingValue` field anywhere. This task lands exactly those.

Global weave deltas applied to EVERY test below:
- Binary name `weave` (not `skilldozer`).
- Unit noun `extension` (not `skill`); store dir `extensions` (not `skills`).
- Canonical one-liner is `pi -e "$(weave <tag>)"` (NOT `pi --skill "$(skilldozer <tag>)"`).
- `--file` prints the **ENTRY FILE** (the `.ts`/`.js` pi loads) — NOT `SKILL.md`.
- weave has **NO `storeMissingValue` step** → every `--store`-no-value / `init --store`-no-value test is **SKIPPED** (see §A.6).
- `version` resolves to `"dev"` under `go test` (no ldflags) in BOTH repos.

---

# PART A — Test Pattern Extraction (skilldozer → weave)

All tests call `run(args, &out, &errOut)` with `var out, errOut bytes.Buffer`. Convention: assert exit code with `t.Fatalf`, then assert stream contents with `t.Errorf`.

## A.1 — `--help` → exit 0, usage on STDOUT, empty stderr

**`TestRunHelpToStdoutExit0`** (`main_test.go:1619`) — `run([]string{"--help"}, ...)`
```go
code := run([]string{"--help"}, &out, &errOut)
// exit
if code != 0 { t.Fatalf("run(--help): code=%d; want 0", code) }
// stdout: section headers + canonical one-liner, PLAIN (no ANSI)
got := out.String()
for _, want := range []string{"USAGE:", "EXAMPLES:", "OPTIONS:", `pi --skill "$(skilldozer example)"`} {
    if !strings.Contains(got, want) { ... }
}
if strings.Contains(got, "\x1b[") { ... }   // help must be PLAIN
// stderr: empty
if errOut.Len() != 0 { ... }
```
**`TestRunHelpShortFlag`** (`main_test.go:1639`) — `run([]string{"-h"}, ...)` → `code==0`, stdout contains `"USAGE:"`.

**Weave delta (A.1):** the one-liner assertion changes — the expected substring becomes `pi -e "$(weave example)"`. The `USAGE:`/`EXAMPLES:`/`OPTIONS:` substring checks port **directly** (binary-agnostic headers). The PLAIN-no-ANSI and empty-stderr checks port **directly**.

## A.2 — `--help` vs `--version` precedence (help wins)

**`TestRunHelpBeatsVersion`** (`main_test.go:1652`) — `run([]string{"--help", "--version"}, ...)`
```go
if code != 0 { t.Fatalf(...) }
// stdout must NOT contain the version line -> help won
if strings.Contains(out.String(), "skilldozer "+version) { ... }
// stdout must BE the help block
if !strings.Contains(out.String(), "USAGE:") { ... }
```
**Weave delta (A.2):** `"skilldozer "+version` → `"weave "+version`. Logic ports **directly**.

Related precedence tests (not strictly required by the task brief, but form the precedence chain `help → version → unknownFlag`; all port directly with binary-name swap):
- **`TestRunHelpBeatsUnknownFlag`** (`main_test.go:1716`) — `run([]string{"--help","--bogus"})` → `code==0`, stdout contains `"USAGE:"`, stderr empty.
- **`TestRunVersionBeatsUnknownFlag`** (`main_test.go:1748`) — `run([]string{"--version","--bogus"})` → `code==0`, stdout == `"skilldozer "+version+"\n"` (**delta:** `"weave "+version+"\n"`), stderr empty.

## A.3 — Unknown flag → exit 2, `'unknown flag'` on stderr, empty stdout

**`TestRunDefaultUnknownFlag`** (`main_test.go:798`, lives under the "no-args" cluster) — `run([]string{"--frobnicate"}, ...)`
```go
code := run([]string{"--frobnicate"}, &out, &errOut)
if code != 2 { t.Fatalf("run(--frobnicate): code=%d; want 2", code) }
// stdout: EMPTY
if out.Len() != 0 { t.Errorf("stdout=%q; want EMPTY (§6.4)", out.String()) }
// stderr: EXACT line
want := "skilldozer: unknown flag '--frobnicate'\n"
if got := errOut.String(); got != want { t.Errorf("stderr=%q; want %q", got, want) }
```
**`TestRunUnknownShortFlagExit2`** (`main_test.go:1716` cluster) — `run([]string{"-z"}, ...)` → `code==2`, stdout empty, stderr == `"skilldozer: unknown flag '-z'\n"`.

**Weave delta (A.3):** the exact stderr string changes prefix `skilldozer:` → `weave:`:
- `"weave: unknown flag '--frobnicate'\n"`
- `"weave: unknown flag '-z'\n"`
This is the exact line weave's `run()` must emit (note the format `"weave: unknown flag '%s'\n"`).

## A.4 — Exclusivity combos → exit 2, empty stdout

All call `run(args, &out, &errOut)` and assert the **same triad**: `code==2`, `out.Len()==0` (empty stdout), and a substring on stderr. None need a store fixture/env (exclusivity runs before `extdir.Find`).

### A.4.a — 2+ listing modes (Issue 6)
| Function | line | run() args | stderr substring asserted |
|---|---|---|---|
| `TestRunExclusivityListAndSearch` | 2259 | `{"--list","--search","foo"}` | `"mutually exclusive"` |
| `TestRunExclusivityAllAndList` | 2274 | `{"--all","--list"}` | `"mutually exclusive"` |
| `TestRunExclusivityPathAndList` | 2289 | `{"--path","--list"}` | `"mutually exclusive"` |
| `TestRunExclusivityListingModePairs` | 2305 | all 6 pairs table | `"mutually exclusive"` |

The pairs table (`main_test.go:2305`) exhaustively covers: `{"--path","--list"}`, `{"--path","--search","x"}`, `{"--path","--all"}`, `{"--list","--search","x"}`, `{"--list","--all"}`, `{"--search","x","--all"}`.

A predicate-level companion (calls `exclusivityError` directly, not `run`): **`TestExclusivityErrorListingModes`** (`main_test.go:2201`) — a table over `config{...}` structs asserting `bad` and `strings.Contains(msg,"mutually exclusive")`, including the modifier-doesn't-count cases (`all+file`, `list+noColor`, `path+relative` → `bad=false`).

### A.4.b — tags + a mode (Issue 3)
| Function | line | run() args | stderr substring |
|---|---|---|---|
| `TestRunExclusivityTagsAndList` | 1733 | `{"foo","--list"}` | `"cannot be combined"` |
| `TestRunExclusivityTagsAndSearch` | 1748 | `{"foo","--search","q"}` | (empty-stdout only) |
| `TestRunExclusivityTagsAndAll` | 1760 | `{"foo","--all"}` | (empty-stdout only) |
| `TestRunExclusivityTagsAndPath` | 1775 | `{"foo","--path"}` | `"cannot be combined"` |
| `TestRunExclusivityPathAndTag` | 1790 | `{"--path","foo"}` | `"cannot be combined"` |

Predicate companion: **`TestExclusivityErrorTagsAndPath`** (`main_test.go:2241`) → `exclusivityError(config{tags:[]string{"foo"},path:true})` asserts `bad==true`, msg contains `"tags cannot be combined"` and `"--path"`.

### A.4.c — check + tags
| Function | line | run() args | stderr substring |
|---|---|---|---|
| `TestRunExclusivityCheckAndTags` | 1805 | `{"check","foo"}` | `"check"` |

### A.4.d — check + a mode
| Function | line | run() args | stderr substring |
|---|---|---|---|
| `TestRunExclusivityCheckAndList` | 1820 | `{"check","--list"}` | (empty-stdout only) |
| `TestRunExclusivityCheckAndPath` | 1835 | `{"check","--path"}` | `"check"` AND `"--path"` |

### A.4.e — init + tags
| Function | line | run() args | stderr substring |
|---|---|---|---|
| `TestRunExclusivityInitAndStrayTag` | 1926 | `{"init","foo","bar"}` | `"tag"` |
| `TestRunExclusivityInitInit` | 1944 | `{"init","init"}` | `"init"` (+ asserts config file NOT written) |

### A.4.f — init + a mode
| Function | line | run() args | stderr substring |
|---|---|---|---|
| `TestRunExclusivityInitAndList` | 1855 | `{"init","--list"}` | `"init"` |
| `TestRunExclusivityInitAndPath` | 1870 | `{"init","--path"}` | `"init"` |
| `TestRunExclusivityInitAndSearch` | 1901 | `{"init","--search","q"}` | (empty-stdout only) |
| `TestRunExclusivityInitAndAll` | 1913 | `{"init","--all"}` | (empty-stdout only) |
| `TestRunExclusivityInitAndCheck` | 1886 | `{"init","check"}` | `"init"` |

**Weave delta (A.4):** these port **directly**. The asserted substrings (`"cannot be combined"`, `"mutually exclusive"`, `"check"`, `"init"`, `"tag"`, `"--path"`) are all **binary-agnostic**, so they pass unchanged. The exact messages weave's `exclusivityError` must produce (prefix swap `skilldozer:` → `weave:`), copied from `main.go` `exclusivityError`:
```
weave: listing modes --path/--list/--search/--all are mutually exclusive
weave: tags cannot be combined with --path/--list/--search/--all
weave: 'check' cannot be combined with tag arguments
weave: 'check' cannot be combined with --path/--list/--search/--all
weave: 'init' cannot be combined with tag arguments
weave: 'init' cannot be combined with --list/--search/--all/--path/check
```
**weave `exclusivityError` checklist** (order matters, first-hit wins): (1) count of `{path,list,searchMode,all}` ≥ 2; (2) `tags` + any of those; (3) `check` + `tags`; (4) `check` + any mode; (5) `init` + `tags`; (6) `init` + any of `{check,list,searchMode,all,path}`. Modifiers (`file`,`relative`,`noColor`) never trigger it.

## A.5 — No-args → exit 1, usage on STDERR, empty stdout

**`TestRunDefaultNoArgs`** (`main_test.go:779`) — `run(nil, &out, &errOut)`
```go
code := run(nil, &out, &errOut)
if code != 1 { t.Errorf("run(nil): code=%d; want 1", code) }
// stdout: EMPTY
if out.Len() != 0 { t.Errorf("run(nil) stdout=%q; want EMPTY", out.String()) }
// stderr: the USAGE block (same text as --help, different destination)
if !strings.Contains(errOut.String(), "USAGE") { t.Errorf(...) }
```
**`TestRunModifiersOnlyNoMode`** (`main_test.go:1697`) — `run([]string{"--no-color"}, ...)` → same triad (`code==1`, stdout empty, stderr contains `"USAGE"`).

**Weave delta (A.5):** ports **directly** — the `"USAGE"` substring and the code/stream contract are binary-agnostic. The SAME `usageText` constant is printed to **stdout** for `--help` (exit 0) and to **stderr** for no-args (exit 1); only destination + exit differ.

## A.6 — SKIPPED (weave has no storeMissingValue)

The following skilldozer tests assert `--store`/`--store=`/`init --store` presented with **no value** → exit 2 before init dispatch. weave has **no `storeMissingValue` field and no step 3.5 guard**, so **port NONE of these**:
- `TestRunInitStoreNoValueExits2` (`main_test.go`), `TestRunStoreEqualsEmptyExits2`, `TestRunStoreBareNoValueExits2`, `TestRunInitStoreNoValueDoesNotWriteConfig`
- parse-level: `TestParseArgsInitStoreLongFormNoValueSetsSignal`, `TestParseArgsInitStoreEqualsFormEmptyValueSetsSignal`, `TestParseArgsStoreNoValueNoInitTokenSetsSignal`

> Open question for the parent: weave still parses `init --store <dir>` and `--store=<dir>` (the value-present forms) — those tests *do* port. Only the **missing-value** family is skipped. Confirm whether weave wants *any* exit-2 behavior for a bare trailing `--store`; current weave `parseArgs` does not set a missing-value signal, so a bare `--store` currently falls to the no-args/usage path.

## Port-direct vs. delta summary (PART A)

| Pattern | Ports directly | Weave delta |
|---|---|---|
| `--help` exit 0 + STDOUT + empty stderr + no-ANSI | ✅ | one-liner substring → `pi -e "$(weave example)"` |
| help-beats-version | ✅ | expected NON-version line uses `"weave "+version` |
| unknown flag exit 2 + exact stderr + empty stdout | ✅ structure | stderr prefix `skilldozer:` → `weave:` |
| all exclusivity (A.4.a–f) | ✅ (substrings binary-agnostic) | none — but weave `exclusivityError` must emit the `weave:`-prefixed messages above |
| no-args exit 1 + STDERR usage + empty stdout | ✅ | none |
| store-missing-value exit 2 | ❌ SKIP | weave has no `storeMissingValue` |

---

# PART B — usageText Blueprint & Weave Adaptation

## B.1 — EXACT skilldozer structure (measured byte-exact via `cat -A`)

Source: `main.go:52-97` (raw backtick string literal; the closing backtick is on its own line so the string ends with exactly **one** trailing `\n`, no trailing blank line).

```
skilldozer — skill path printer
<blank>
Resolve skill tags to on-disk skill directory paths (manifest-free).
<blank>
USAGE:
  skilldozer <tag> [<tag>...]
  skilldozer --all
  skilldozer --list
  skilldozer --search <query>
  skilldozer check
  skilldozer init [<dir>]
  skilldozer --path
  skilldozer --help
  skilldozer --version
<blank>
EXAMPLES:
  pi --skill "$(skilldozer example)"
  pi --skill "$(skilldozer writing/reddit)"
  skilldozer example reddit          # one absolute path per line, input order
  skilldozer -f example              # print the SKILL.md path
  skilldozer --relative --all        # every skill path, relative to the skills dir
  skilldozer --list                  # human-readable catalog
  skilldozer --search reddit         # substring search over tag/name/description/keywords/aliases/category
  skilldozer check                   # validate every skill on disk
  skilldozer init --store <dir>     # non-interactive first-run setup
<blank>
OPTIONS:
  <tag> [<tag>...]   Resolve tags to skill directory paths (one absolute path per line)
  --all, -a          Print every skill's directory path, sorted by tag
  --list, -l         Human-readable catalog (TAG, NAME, DESCRIPTION)
  --search <q>, -s   Substring search over tag / name / description / keywords / aliases / category
  check              Validate every skill on disk (report OK / WARN / ERROR)
  init [<dir>]      First-run setup: pick/create the skills store and write the config
  --store <dir>     Non-interactive store path for init
  --path, -p         Print the resolved skills directory (discovery rule printed to stderr)
  --file, -f         Print the SKILL.md path instead of the directory (modifier)
  --relative         Print paths relative to the skills directory (modifier)
  --no-color         Disable ANSI color even on a TTY (modifier)
  --help, -h         Show this help message
  --version, -v      Print the skilldozer version
<blank>
Exit codes: 0 success/help/version | 1 unresolved/no skills/unresolvable dir | 2 unknown flag / mutually-exclusive modes
```

### Section anatomy (top to bottom)
1. **Tagline** (line 1): `skilldozer — skill path printer` — note the em-dash `—` (U+2014, UTF-8 `e2 80 94`), NOT a hyphen-minus.
2. **Description** (line 3): one sentence ending `(manifest-free).`
3. **`USAGE:`** header + 9 invocation lines, each 2-space indented. Order: `<tag>` → `--all` → `--list` → `--search` → `check` → `init` → `--path` → `--help` → `--version`.
4. **`EXAMPLES:`** header + 9 example lines, 2-space indented. Comments are aligned with padding spaces before `#`.
5. **`OPTIONS:`** header + 13 option lines, 2-space indented, two-column.
6. **Trailing line** (no header): `Exit codes: 0 … | 1 … | 2 …`.

### OPTIONS column alignment (measured)
- **2-space indent**; option-spec field **left-justified to width 16** (longest specs are `<tag> [<tag>...]` and `--search <q>, -s`, both 16 chars); then a **fixed 3-space gap**; descriptions begin at **column 21**.
- **LATENT BUG in skilldozer:** the `init [<dir>]` row (spec=12) and `--store <dir>` row (spec=13) have ONE space too few — their descriptions start at column 20, not 21. Correct padding would be 7 spaces for `init [<dir>]` and 6 spaces for `--store <dir>`. **Weave should fix this** (see B.2).

### Trailing-newline handling
The const ends at the `Exit codes:` line's `\n` immediately before the closing backtick — **exactly one trailing newline, no trailing blank line**. Printed verbatim via `fmt.Fprint(w, usage())`. The same text is emitted to stdout (`--help`, exit 0) and to stderr (no-args, exit 1); only destination differs.

---

## B.2 — Weave adaptation (the `usageText` to write)

Rules applied: binary `weave`; noun `extension`; store dir `extensions`; one-liner `pi -e "$(weave <tag>)"`; `--file` = ENTRY FILE not SKILL.md; **OPTIONS alignment fixed** so every description starts at column 21; "skills" → "extensions" in the exit-codes line.

### Tagline decision
Skilldozer splits the task's requested tagline across two lines: `skilldozer — skill path printer` (line 1) + `(manifest-free)` (line 3). The task brief literally requests tagline **"manifest-free extension path printer"**. Two faithful options:
- **(Recommended — structural parity)** keep the split: line 1 `weave — extension path printer`, line 3 ends `(manifest-free)`.
- **(Literal brief wording)** line 1 `weave — manifest-free extension path printer` (drop manifest-free from line 3, or keep both).

Below uses the recommended parity version. If the literal wording is preferred, swap only line 1.

### Proposed weave `usageText` (column-aligned, trailing single `\n`)

```
weave — extension path printer

Resolve extension tags to on-disk extension paths (manifest-free).

USAGE:
  weave <tag> [<tag>...]
  weave --all
  weave --list
  weave --search <query>
  weave check
  weave init [<dir>]
  weave --path
  weave --help
  weave --version

EXAMPLES:
  pi -e "$(weave example)"
  pi -e "$(weave writing/reddit)"
  weave example reddit          # one absolute path per line, input order
  weave -f example              # print the entry file path
  weave --relative --all        # every extension path, relative to the extensions dir
  weave --list                  # human-readable catalog
  weave --search reddit         # substring search over tag/name/description/keywords/aliases/category
  weave check                   # validate every extension on disk
  weave init --store <dir>      # non-interactive first-run setup

OPTIONS:
  <tag> [<tag>...]   Resolve tags to extension paths (one absolute path per line)
  --all, -a          Print every extension's path, sorted by tag
  --list, -l         Human-readable catalog (TAG, NAME, DESCRIPTION)
  --search <q>, -s   Substring search over tag / name / description / keywords / aliases / category
  check              Validate every extension on disk (report OK / WARN / ERROR)
  init [<dir>]       First-run setup: pick/create the extensions store and write the config
  --store <dir>      Non-interactive store path for init
  --path, -p         Print the resolved extensions directory (discovery rule printed to stderr)
  --file, -f         Print the entry file path instead of the resolvable path (modifier)
  --relative         Print paths relative to the extensions directory (modifier)
  --no-color         Disable ANSI color even on a TTY (modifier)
  --help, -h         Show this help message
  --version, -v      Print the weave version

Exit codes: 0 success/help/version | 1 unresolved/no extensions/unresolvable dir | 2 unknown flag / mutually-exclusive modes
```

### Weave PRD §6.1 commands & §6.2 modifiers (with weave semantics) — what the OPTIONS/USAGE rows must advertise

From `plan/001_19b4b465824d/prd_snapshot.md` §6.1 (lines 127-137) and §6.2 (lines 141-145). Contract is byte-identical to skilldozer **except `--file` semantics**.

**§6.1 Commands** (9):
| Invocation | stdout | exit |
|---|---|---|
| `weave <tag> [<tag>...]` | one **absolute** resolvable path per line, input order | `0` all resolve; `1` if any fail (nothing printed) |
| `weave --all` / `-a` | every extension's resolvable path, sorted by tag | always `0` |
| `weave --list` / `-l` | table TAG/NAME/DESCRIPTION | `0`; `1` if no extensions found |
| `weave --search <q>` / `-s <q>` | same table, filtered (case-insensitive substring over tag, `package.json` name/description/keywords, leading-JSDoc description, `weave.aliases`, `weave.category`) | `0`; `1` if no matches |
| `weave check` | report: OK lines + WARN/ERROR | `0` clean; `1` if any ERROR |
| `weave init` (non-interactive `init <dir>` / `init --store <dir>`) | configured store path | `0`; `1` on error/cancel |
| `weave --path` / `-p` | absolute extensions dir (+ found-via label to stderr) | `0`; `1` if unresolvable |
| `weave --help` / `-h` | help text to **stdout** | `0` |
| `weave --version` / `-v` | `weave <version>` single line | `0` |

**§6.2 Modifiers** (3) — combine with `<tag>` resolution or `--all` only:
| Flag | Weave-specific effect |
|---|---|
| `--file` / `-f` | Print the **ENTRY FILE** path (the `.ts`/`.js` pi loads): the file itself for single-file extensions; `index.ts`/`index.js` for dir extensions; the first existing `pi.extensions` entry for package extensions. **NOT SKILL.md.** This is the one structural delta vs skilldozer. |
| `--no-color` | Disable ANSI even on a TTY. |
| `--relative` | Print paths relative to the **extensions** dir (default absolute). |

### weave `run()` dispatch order this task must insert (matches skilldozer `main.go`)
Current weave `run()` checks only `c.version` → `c.init` → modes. Insert in this precedence order:
1. `if c.help { fmt.Fprint(stdout, usage()); return 0 }` — **before** version (help-wins tiebreak).
2. `if c.version { fmt.Fprintf(stdout, "weave %s\n", version); return 0 }` (already present).
3. `if c.unknownFlag != "" { fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag); return 2 }` — after version, before exclusivity.
4. **(NO step 3.5 / storeMissingValue — weave skips this.)**
5. `if bad, msg := exclusivityError(c); bad { fmt.Fprintln(stderr, msg); return 2 }` — before the init/mode dispatch.
6. Move `if c.init { return runInit(...) }` to **after** exclusivity (it currently sits before `--path`; exclusivity must gate it).
7. At the very end (no mode matched): `fmt.Fprint(stderr, usage()); return 1`.

---

## Start Here
Open `/home/dustin/projects/skilldozer/main.go` lines 52-97 (the `usageText` const) and lines ~430-560 (the `run()` precedence ladder + `exclusivityError`). Then port into `/home/dustin/projects/weave/main.go`: add the `usageText` const (B.2 above) + `usage()` fn, insert the `help`/`unknownFlag`/`exclusivityError`/no-args branches into `run()`, and add the `main_test.go` tests from PART A.

## Supervisor coordination
None required — all facts resolved from source. One non-blocking open question surfaced (whether weave wants any exit-2 for a bare trailing `--store`): noted in §A.6 for the parent; does not block this extraction.