# Verified Facts — P1.M4.T4.S2 (setupStore + runInit + exampleExtensionTemplate + init dispatch)

Researched by reading the live weave tree (main.go, internal/{config,extdir,discover,check}),
the parallel sibling PRP P1.M4.T4.S1 (CONTRACT for resolveStore/chooseStore + the
`configpkg`/`bufio` imports it lands), and skilldozer's shipped reference (main.go +
main_test.go + the S2/S3 PRPs that are the exact-shape analogs of this task).

## §1 — Dependencies that are LANDED when S2 begins (assume S1 lands as specified)

S1 (parallel) appends 4 functions to main.go and is left UNCALLED; S2 is the one that
WIRES them. When S2 starts, main.go will contain:

- `func resolveStore(haveStore string) (string, error)` — the I/O wrapper S2's runInit
  calls as `resolveStore(c.initStore)`. Absolutizes via filepath.Abs; prompt → os.Stderr.
  (S1 PRP §2, "resolveStore" block.)
- `func chooseStore(...) (string, error)` — the pure decision core. S2 does NOT call it
  directly (resolveStore does). Do not reach for it.
- `func stdinIsTerminal() bool`, `func readPrompt(...)` — only reachable via resolveStore.

main.go's import block, after S1 lands, is (stdlib then internal, alphabetical):

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dabstractor/weave/internal/check"
	configpkg "github.com/dabstractor/weave/internal/config"   // ALIASED (local `config` struct at main.go ~L77)
	"github.com/dabstractor/weave/internal/discover"
	"github.com/dabstractor/weave/internal/extdir"
	"github.com/dabstractor/weave/internal/resolve"
	"github.com/dabstractor/weave/internal/search"
	"github.com/dabstractor/weave/internal/ui"
)
```

**CRITICAL: S2 needs ZERO new imports.** `exampleExtensionTemplate` is a raw string;
`setupStore` uses os/filepath/fmt/configpkg; `runInit` uses fmt/io/filepath/configpkg/
extdir/discover/check + resolveStore (S1). All already imported. Do NOT touch the import
block (S1 owns it; touching it risks a merge conflict with S1's still-landing edit).

## §2 — The consumed APIs (verified exported, with signatures)

```go
// internal/config/config.go
type File struct{ Store string }
func Load(path string) (File, error)          // for tests to round-trip the written config
func Save(path string, f File) error          // path FIRST; MkdirAll(parent)+WriteFile 0o644
func Path() (string, error)                    // $weave_CONFIG or XDG default; pure env fn
func DefaultStore() (string, error)            // XDG data home + "weave/extensions"

// internal/extdir/extdir.go
type Source int  // SourceEnv | SourceConfig | SourceSibling | SourceWalkUp
func (s Source) String() string               // "weave_EXTENSIONS_DIR"|"config file"|"sibling of binary"|"ancestor of cwd"
func Find() (dir string, src Source, err error) // returns extdir.ErrNotFound when all miss
func HasExtensionEntry(dir string) bool        // cwd-auto-detect predicate (S1 uses it, not S2)

// internal/discover/extension.go + index.go
type Extension struct {
	Path, EntryFile, RelTag, Kind, Name, Description string
	Keywords []string; Category string; Aliases []string; HasPackageJSON bool
}
func Index(extensionsDir string) ([]Extension, error)  // sorted by RelTag

// internal/check/check.go
type Severity int          // LevelOK | LevelWarn | LevelError ; .String() → "OK"|"WARN"|"ERROR"
type Finding struct{ Level Severity; Message string }
type ExtensionReport struct{ Extension discover.Extension; Findings []Finding }
type Report struct{ ByExt []ExtensionReport; Errors, Warnings int }
func (Report) HasErrors() bool
func Check(dir string, exts []discover.Extension) Report   // dir FIRST, exts SECOND (NOT skilldozer's Check(exts))
```

**The weave `check.Check` signature is `Check(dir, exts)` — dir FIRST.** This differs
from skilldozer's `check.Check(skills)` (exts only). runInit MUST call `check.Check(dir, exts)`,
not `check.Check(exts)`. (dir is needed for the §9 empty-category-folder walk.)

## §3 — run() insertion anchor (where the `if c.init` dispatch goes)

Weave's run() currently dispatches in this order: version → path → list → search → check →
all → tags → default(0). There is NO exclusivityError / unknownFlag / storeMissingValue
dispatch yet (all three are M5.T1.S1). Per the item description ("the `if c.init` dispatch
(before the normal mode ladder)"), the init branch slots in **right after the `if c.version`
block and before `if c.path`** — the "before the normal mode ladder" position. When M5
lands exclusivityError, it inserts BETWEEN version and init (so exclusivity is enforced
before init runs); that is a stable, conflict-free future edit.

Exact anchor (main.go, transcribed verbatim from the live file):

```go
	if c.version {
		fmt.Fprintf(stdout, "weave %s\n", version)
		return 0
	}

	// 2) --path (PRD §6.1/§6.4). extdir.Find locates the dir via the §8.3 rules.
```

Insert the init dispatch between `return 0` (end of version block) and the `// 2) --path`
comment. §6.3 precedence is preserved: --version still wins over init (`init --version`
prints the version).

## §4 — exampleExtensionTemplate: single raw string literal (NO backtick splicing)

The PRD §11 example.ts body was extracted and `grep -c '`'` returned **0** — zero backticks.
So `const exampleExtensionTemplate = \`…\`` works as ONE raw string literal (unlike
skilldozer's exampleSkillTemplate, which needed `+ "\`" +` splicing for 8 backticks).

Content (byte-exact, `/**` immediately after the opening backtick, `}\n` before the
closing backtick — matches skilldozer's exampleSkillTemplate framing):

```
/**
 * Reference example extension for weave. Demonstrates a minimal pi extension
 * and how weave resolves a tag to an absolute path. Registers a harmless
 * /weave-example command and greets on session start. Safe to delete once
 * you add real extensions.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("weave example extension loaded", "info");
  });

  pi.registerCommand("weave-example", {
    description: "Prove the weave example extension is loaded",
    handler: async (_args, ctx) => {
      ctx.ui.notify("Hello from the weave example extension!", "info");
    },
  });
}
```

**Cross-task contract:** the repo asset `extensions/example.ts` created by P1.M6.T1.S1
MUST equal `exampleExtensionTemplate` byte-for-byte (both == PRD §11). This PRP pins the
bytes; M6.T1.S1 must copy them verbatim.

## §5 — THE KEY WEAVE DELTA: seed `store/example.ts` (root, single file), NOT `store/example/SKILL.md`

skilldozer's setupStore seeds a SUBDIRECTORY: `MkdirAll(store/example)` then
`WriteFile(store/example/SKILL.md, exampleSkillTemplate)`. Weave seeds a SINGLE FILE at the
store ROOT: `WriteFile(store/example.ts, exampleExtensionTemplate)`. There is NO `exampleDir`
and NO second MkdirAll for a subdirectory — only the single `MkdirAll(store, 0o755)`.

Consequences for the ported tests:
- read the seeded file at `filepath.Join(store, "example.ts")`, NOT `store/example/SKILL.md`.
- "adopted, never clobber" asserts NO `example.ts` was created (not "no `example/` dir").
- The seeded example.ts is a §7.1 single-file entry → discover.Index finds it with
  RelTag "example" (trailing .ts stripped), Kind "file", description from the leading JSDoc.
  check.Check therefore reports it OK (no description-WARN, entry exists).

## §6 — THE SECOND WEAVE DELTA: runInit's check report goes to STDOUT

skilldozer's runInit renders the check report to STDERR (with a comment "the two blocks
intentionally DIVERGE — do NOT extract a shared helper" because skilldozer's standalone
`check` uses stdout). **The weave item description overrides this**: step 6 says
"discover.Index + check.Check → print check report to stdout". This is internally
consistent: weave's standalone `check` ALSO uses stdout (see main.go `if c.check` branch),
and PRD §8.2 step 5 ("Print the output of `weave --path` and `weave check`") under weave's
own conventions means dir→stdout + (found via)→stderr + check-report→stdout. So weave's
runInit reproduces its own --path and check outputs faithfully, with NO stream divergence.

Net runInit output map (per item description steps 4-6):
- STDERR: "Seeded example extension at <path>" | "Adopted existing store at <path>"; "(found via <src>)".
- STDOUT: the configured store path (one line); then the check report (per-extension OK/finding lines + the "N extensions, M errors, K warnings" summary).

This means the TestRunInit assertion for weave is the OPPOSITE of skilldozer's:
- skilldozer asserts `stdout == store+"\n"` (exactly one line) and treats a check-report
  leak onto stdout as a bug.
- weave asserts stdout CONTAINS the store path AND the check summary ("extensions,").

Exit code: 0 once create+config succeed (check findings never change init's exit code —
item description: "check findings don't change init exit code").

## §7 — Test patterns to port (from skilldozer main_test.go, adapted for weave)

Existing weave main_test.go helpers (REUSE, do not redefine): `unsetExtEnv(t)` (sets
weave_EXTENSIONS_DIR + weave_CONFIG to ghost temp paths), `writeExtTree(t, map)`,
`withTerminal(t, bool)`. Imports: bytes, io, os, path/filepath, strings, testing.

Tests to port (white-box `package main`):
1. `TestSetupStoreEmptyDirSeedsExampleAndWritesConfig` — empty store → seeded=true, reads
   `store/example.ts` == exampleExtensionTemplate EXACTLY, config round-trips store.
2. `TestSetupStoreNonEmptyDirAdoptsInPlaceAndWritesConfig` — store with a pre-existing file
   → seeded=false, file byte-intact, NO example.ts created, config still written.
3. `TestSetupStoreIdempotent` — run1 seeds, run2 adopts (example.ts byte-identical), config valid.
4. `TestSetupStoreMkdirAllFailureReturnsWrappedError` — store path is an existing file →
   (false, err), no config written.
5. `TestRunInitStoreWritesConfigCreatesStorePrintsPathExit0` — `run(["init","--store",store])`
   with weave_CONFIG=<temp cfg>, weave_EXTENSIONS_DIR="", t.Chdir(tempdir): exit 0, store
   created, config.Store==store, stdout contains store path AND "extensions," (weave puts
   the check report on stdout), stderr contains "Seeded example extension at" + "(found via".
6. `TestRunBareTagUnconfiguredNeverPrompts` — clean env + t.Chdir(tempdir), `run(["sometag"])`:
   exit 1, stderr contains the "run `weave init`" hint, stdout EMPTY (§6.4). Structural
   proof init is never reached (c.init is false for tags).

NOT ported: skilldozer's `TestRunInitStoreTildeExpandsHome` — expandHome is explicitly out
of scope for weave M4.T4 (S1 PRP §"Anti-Patterns": "Don't add expandHome"). Weave's
resolveStore absolutizes via filepath.Abs only; `~` is not expanded.

## §8 — Env var names (weave uses LOWERCASE, per config.go)

- `weave_CONFIG` (lowercase) — config-file location override.
- `weave_EXTENSIONS_DIR` (lowercase) — extensions dir override (rule 1).
Tests use `t.Setenv("weave_CONFIG", …)` and `t.Setenv("weave_EXTENSIONS_DIR", "")`.
(NOT skilldozer's `SKILLDOZER_CONFIG` / `SKILLDOZER_SKILLS_DIR` uppercase.)
