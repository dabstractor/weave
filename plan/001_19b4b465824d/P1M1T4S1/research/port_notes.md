# Port Notes — P1.M1.T4.S1 (skilldozer main.go → weave main.go)

## Source of truth
- skilldozer: `/home/dustin/projects/skilldozer/main.go` (1050 lines)
- skilldozer tests: `/home/dustin/projects/skilldozer/main_test.go` (2387 lines)

## Scope of THIS subtask (per item contract §3)
Port EXACTLY these symbols from skilldozer main.go to weave main.go:

| symbol | skilldozer line | port verbatim? | notes |
|---|---|---|---|
| `package main` doc comment | 1-14 | adapt | "weave" / "extensions" / `pi -e` / milestone refs |
| imports | 16-29 | **TRIM** | only `fmt`, `io`, `os` needed for --version/--path; drop discover/resolve/ui/check/search/skillsdir/config/bufio/filepath/strings IF unused. See "Import minimization" below. |
| `var version = "dev"` | 43-52 | verbatim | change "skilldozer"→"weave" in doc + ldflags example |
| `const usageText` | 54-97 | **DEFER to M5** | NOT in this subtask (M5.T1.S1 owns --help/usage). Item contract: only --version/--path dispatch. |
| `func usage() string` | 98-99 | **DEFER to M5** | depends on usageText |
| `var isTerminal` | 109-119 | verbatim | type-asserts *os.File, ModeCharDevice |
| `func main()` | 121-123 | verbatim | `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))` |
| `type config struct` | 128-151 | verbatim | ALL §6.1/§6.2 fields incl unknownFlag, initStore |
| `func parseArgs` | 153-324 | verbatim | full flag matrix (long/short/bundle/`=`/value/reserved/positional/unknown) |
| `func expandShortBundle` | 326-406 | verbatim | two-phase validate-then-commit |
| `func run()` | 408-... | **TRIM** | only --help→DEFER, --version, --path active; everything else no-op/return 0 or falls to no-args default. See "run() dispatch scope" below. |
| `exclusivityError` | 686-746 | **DEFER to M5** | M5.T1.S1 owns exclusivity + unknown-flag exit 2 |
| `skillPath` | 747-770 | **DEFER to M3** | needs discover.Extension (M2) |
| `runInit`/`chooseStore`/`resolveStore`/`setupStore` | 822-1050 | **DEFER to M4** | init command (M4.T4) |

## Critical: item contract §3 EXACT spec
The contract pins the `config` struct fields: `version, help, path, list, all, file, relative, noColor, searchMode, searchQ, check, init, initStore, tags []string, unknownFlag string`. Port skilldozer's struct verbatim (same fields, same order, same types) — the ONLY change is the doc-comment binary name.

parseArgs: hand-rolled index-based loop, NOT flag package. Handle ALL forms:
- long: --version, --help, --path, --list, --all, --file, --relative, --no-color, --search, --store
- short: -v, -h, -p, -l, -a, -f, -s (NO short for --store/--relative/--no-color per §6.1/§6.2)
- `=`-forms: --flag=value
- short bundles: -vpl, -sfoo
- value-taking: --search <q>, --store <dir> (consume next via i++)
- reserved subcommands: check, init
- positional tags: non-dashed → c.tags
- unknown dashed → first into c.unknownFlag

run(): dispatch ONLY:
- --version → `fmt.Fprintf(stdout, "weave %s\n", version)`, return 0
- --path → `extdir.Find()`; on success `fmt.Fprintln(stdout, dir)` + `fmt.Fprintf(stderr, "(found via %s)\n", src)`, return 0; on err `fmt.Fprintln(stderr, err)`, return 1
- ALL OTHER MODES: no-ops for now (return 0 is fine since they have no behavior; later milestones add dispatch). Item contract: "All other modes are no-ops for now (added in later milestones)."
- NO --help, NO exclusivity, NO unknown-flag exit 2 (M5.T1.S1).
- parseArgs is the FULL matrix so later milestones only ADD dispatch branches in run(), NOT parser changes.

## Import minimization
After trimming run() to --version/--path + no-op stubs:
- `fmt` — needed (Fprintf/Fprintln)
- `io` — needed (run signature `io.Writer`)
- `os` — needed (main, isTerminal)
- `"github.com/dabstractor/weave/internal/extdir"` — needed (--path calls extdir.Find)
- DROP: bufio, path/filepath, strings, discover, resolve, ui, check, search, configpkg — NOT used in --version/--path dispatch.

BUT parseArgs/expandShortBundle use `strings.HasPrefix`, `strings.Contains`, `strings.IndexByte`. So **strings IS needed** for the full parser. Keep `strings`.

So imports = fmt, io, os, strings, internal/extdir.

## run() dispatch — concrete decision
Item contract: "Dispatch only --version ... and --path ... All other modes are no-ops for now."

Simplest correct shape: parse args (full matrix), then:
1. if c.version → print + return 0
2. if c.path → Find + print + return 0/1
3. else → return 0 (no-op; later milestones fill in)

NO help, NO exclusivity, NO unknown-flag → 2, NO no-args-usage. Those are M5.T1.S1.

Test contract (item §3): 
- run(["--version"]) → 0, stdout contains "weave"
- run(["--path"]) → 0, stdout contains extensions dir path (sibling-of-binary rule)
- run(["--path"]) in unconfigured env → 1

## extdir.Find() contract (from S3 PRP — treat as the dependency)
`func Find() (dir string, src Source, err error)`
- src.String() returns "weave_EXTENSIONS_DIR" / "config file" / "sibling of binary" / "ancestor of cwd"
- err on miss = `extdir.ErrNotFound` with message `weave is not configured; run \`weave init\``
- run prints `err` (the error value) to stderr via `fmt.Fprintln(stderr, err)` — Fprintln on an error prints err.Error() + newline. Verbatim from skilldozer.

## version var / ldflags
```go
var version = "dev"
```
MUST be `var` (not const) for `-X main.version=...` ldflags to work. Build command (install.sh, M6.T2):
`go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o weave .`

## isTerminal — verbatim
```go
var isTerminal = func(w io.Writer) bool {
    f, ok := w.(*os.File)
    if !ok {
        return false
    }
    fi, err := f.Stat()
    if err != nil {
        return false
    }
    return fi.Mode()&os.ModeCharDevice != 0
}
```
Item contract pins this exact form. Used by later milestones (--list color); kept now so the package compiles identically.

## Testing the --path sibling-of-binary rule
Item contract test: `run(["--path"])` returns 0 and stdout contains the extensions dir path (sibling-of-binary rule).

This means the TEST must arrange for a sibling `extensions/` dir next to the running binary. Options:
1. Build a throwaway binary into a temp dir that has `extensions/<tag>/<tag>.ts`, run it. (skilldozer's main_test.go likely does this via a test helper / TestMain.)
2. Set `weave_EXTENSIONS_DIR` env (rule 1) — simpler, hermetic. But the contract explicitly says "sibling-of-binary rule", so the test should exercise rule 3.

Look at skilldozer main_test.go for the pattern. The contract says "with sibling-of-binary rule" so port that exact test.

Need to check skilldozer main_test.go for: how does it test --path with the sibling rule? Likely builds a copy of the binary into a temp dir, or uses os.Executable() manipulation. Read it.

## parseArgs/expandShortBundle — VERBATIM port
No renames needed in the flag tokens themselves (--version stays --version; the §6 contract is byte-identical per PRD §6 header). Only doc comments change "skilldozer"→"weave", "skill"→"extension", "SKILL.md"→"entry file", `pi --skill`→`pi -e`.
