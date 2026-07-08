// Command weave resolves extension tags to on-disk extension paths.
//
// main.go is the entrypoint: it parses argv, applies PRD §6 precedence
// (--version/--help win over everything), and dispatches to the matching mode.
// Wired so far (grown milestone by milestone): --version/--path (M1.T4).
// Every other §6 flag is added by later milestones (M2 --list, M3 <tag>/
// --file/--all, M4 --search/check/init, M5 --help + exit codes). The arg parser
// is intentionally a small hand-rolled switch (not Go's `flag` package) so the
// full §6 matrix — subcommands like `check`, positional <tag> args, long+short
// aliases, and §6.3 mutual exclusivity — can be expressed cleanly. The parser
// is built ONCE, fully, in this milestone; later milestones add dispatch
// branches in run() only, never parser changes.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dabstractor/weave/internal/check"
	configpkg "github.com/dabstractor/weave/internal/config"
	"github.com/dabstractor/weave/internal/discover"
	"github.com/dabstractor/weave/internal/extdir"
	"github.com/dabstractor/weave/internal/resolve"
	"github.com/dabstractor/weave/internal/search"
	"github.com/dabstractor/weave/internal/ui"
)

// version is the weave version string, printed by `weave --version`. It is
// overridden at BUILD time via ldflags (PRD §12.1 build command):
//
//	go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o weave .
//
// The default "dev" is used by `go run` and plain `go build` (no ldflags).
//
// IMPORTANT: this MUST be a package-level var, not a const. `-X main.version=...`
// rewrites a package-scope string var at link time; it cannot override a const
// (compile error) or a function-local. Because this file is `package main`, the
// linker symbol path is `main.version` (NOT the module import path).
var version = "dev"

// isTerminal reports whether w is an interactive terminal (a character device).
// It decides whether --list/--search emit ANSI color by default (PRD §6.2: color
// is on for a TTY unless --no-color is set). It type-asserts w to *os.File and
// checks the ModeCharDevice bit, so a *bytes.Buffer (tests) or a pipe/redirect
// correctly yields false -> no color, keeping output deterministic and pipe-safe.
//
// It is a package var so tests can override it to exercise the color-enabled path
// through run() without a real terminal. NOT safe for t.Parallel (mutates package
// state); the repo convention is no t.Parallel() on such tests anyway.
//
// NOTE: this milestone (P1.M1.T4.S1) does NOT yet call isTerminal — --version and
// --path do not use color. It is declared now so later milestones (M2 --list
// color) can use it without re-touching the top of main.go. Go does not error on
// unused package-level vars (only unused locals and imports).
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

// exampleExtensionTemplate is the body of the one shipped example extension
// (PRD §11), written verbatim into an EMPTY store's example.ts by setupStore
// during `weave init` (PRD §8.2 step 3). It is a compiled-in STRING CONSTANT —
// NOT a go:embed (PRD §17: "nothing about the user's collection is compiled
// in" — the example is weave's own demo, the only collection that IS compiled
// in; a real user's store is never embedded). CROSS-TASK CONTRACT: the repo
// asset extensions/example.ts created by P1.M6.T1.S1 MUST equal this
// byte-for-byte (both == PRD §11).
//
// Single raw-string literal: the §11 body has ZERO backticks (verified), so NO
// `+ "`" +` splicing is needed (contrast skilldozer's exampleSkillTemplate,
// whose body has 8 backticks and so splices). A future editor MUST re-verify
// zero backticks after any edit to this body, or switch to the splice form.
const exampleExtensionTemplate = `/**
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
`

// usageText is the full --help / no-args usage block (PRD §6.1, §6.3). It is
// byte-identical in STRUCTURE to skilldozer's (PRD §6 header: the CLI contract
// is byte-identical to skilldozer's except --file), with weave nouns: binary
// `weave`, `extension`, the canonical `pi -e "$(weave <tag>)"` one-liner,
// --file = ENTRY FILE (the .ts/.js file pi loads: file itself, index.ts, or
// the first pi.extensions entry — NOT SKILL.md), and `extensions` dir. PLAIN
// (no ANSI): `weave --help | grep` must work and tests use non-TTY buffers.
// The SAME text is printed to STDOUT for --help (exit 0) and to STDERR for the
// no-args default (exit 1) — only the destination + exit code differ.
//
// Column-alignment FIX: skilldozer's OPTIONS table has a latent bug where the
// `init [<dir>]` and `--store <dir>` rows start their descriptions at column 20
// (one space short of the other rows' column 21). weave pads EVERY OPTIONS row
// so descriptions begin at column 21 (option-spec field left-justified width 16,
// then a fixed 3-space gap).
//
// Single raw-string literal: the body contains an em-dash `—` (U+2014) in the
// tagline (NOT a hyphen-minus — matches skilldozer's tagline) and ZERO
// backticks, so NO `+ "`" +` splicing is needed. The closing backtick is on
// its OWN line so the string ends with exactly one trailing `\n` (no trailing
// blank line). A future editor MUST re-verify zero backticks after any edit.
const usageText = `weave — manifest-free extension path printer

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
`

// usage returns the help block. A tiny indirection over the constant so every
// print site is uniform (`fmt.Fprint(w, usage())`), whether the destination is
// stdout (--help, exit 0) or stderr (no-args default, exit 1).
func usage() string { return usageText }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// config holds the parsed CLI flags (PRD §6.1/§6.2 matrix). This subtask
// (P1.M1.T4.S1) lands the FULL matrix; only --version and --path are DISPATCHED
// in run() so far (M2/M3/M4/M5 add dispatch branches). Every field below is
// populated by parseArgs regardless of whether run() acts on it yet — that is the
// critical design decision that lets later milestones add dispatch logic without
// touching the parser.
type config struct {
	version     bool     // --version / -v : print "weave <version>" and exit 0
	help        bool     // --help / -h    : print usage to STDOUT and exit 0 (§6.1, §6.3 "help wins" tiebreak) — dispatched in M5.T1.S1
	path        bool     // --path / -p    : print resolved extensions dir and exit 0/1 — DISPATCHED HERE
	list        bool     // --list / -l    : print the human-readable catalog table (§6.1) — M2.T5.S1
	all         bool     // --all / -a     : print every extension's path, one per line (§6.1) — M3.T2.S1
	file        bool     // --file / -f    : print the entry file path instead of the dir path (§6.2) — M3.T2.S1
	relative    bool     // --relative     : print paths relative to the extensions dir, not absolute (§6.2) — M3.T2.S1
	noColor     bool     // --no-color     : disable ANSI color even on a TTY (§6.2) — M2.T5.S1
	searchMode  bool     // --search <q>/-s : substring search over tag/name/description/keywords/aliases/category (§10) — M4.T3.S1
	searchQ     string   // the --search query value (consumed from the token after --search/-s)
	check       bool     // `weave check` subcommand: validate every extension in the store (§9) — M4.T3.S1
	init        bool     // `weave init [<dir>]` first-run setup (PRD §8.2); also set by `--store <dir>` (which implies init) — M4.T4.S2
	initStore   string   // non-interactive store path: `init <dir>` positional or `--store <dir>` / `--store=<dir>`; empty ⇒ auto-detect
	tags        []string // positional <tag> args (PRD §6.1 `weave <tag> [<tag>...]`); resolved in run — M3.T2.S1
	unknownFlag string   // first unknown dashed token, "" if none (§6 header → exit 2) — M5.T1.S1
}

// parseArgs scans argv tokens and fills a config. Flags may appear in any order
// (PRD §6). Long forms use POSIX double-dash; short forms a single dash. Unknown
// dashed flags are tolerated for now (captured into c.unknownFlag but run() does
// not yet act on them); the full unknown-flag -> exit 2 behavior and §6.3
// mutual-exclusivity land in P1.M5.T1.S1.
//
// To add a flag in a later milestone: append a `case "--name", "-n": cfg.name =
// true` (or capture the next arg for value-taking flags like --search <q>).
// parseArgs already understands the ENTIRE §6.1/§6.2 matrix now; later
// milestones add dispatch branches in run(), never parser changes.
func parseArgs(args []string) config {
	var c config
	// Index-based loop (not range) so a value-taking flag (--search <q>) can
	// CONSUME the following token via i++ without it also being captured as a tag.
	// PRD §6.1/§6.2: --search/-s take exactly one value; every other flag is a bool.
	for i := 0; i < len(args); i++ {
		a := args[i]

		// Issue 5 (decisions.md §D5): normalize combined / '='-bearing tokens
		// BEFORE the exact-match switch so POSIX forms work. Each branch ends in
		// `continue`; the switch below still handles the original exact-token forms
		// (--version, -v, --search <q>, check, bare tags, and unknowns like -x).

		// (a) Long flag with '=': --flag=value. Split on the FIRST '='; bool flags
		// ignore the value (--version=x == --version), --search takes it as the
		// query, an unknown name is an unknown flag (the whole token is reported).
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			eq := strings.IndexByte(a, '=')
			name, val := a[:eq], a[eq+1:]
			switch name {
			case "--version":
				c.version = true
			case "--help":
				c.help = true
			case "--path":
				c.path = true
			case "--list":
				c.list = true
			case "--all":
				c.all = true
			case "--file":
				c.file = true
			case "--relative":
				c.relative = true
			case "--no-color":
				c.noColor = true
			case "--search":
				c.searchMode = true
				c.searchQ = val
			case "--store":
				// `--store=<dir>`: non-interactive store path for init (PRD §8.2). Mirrors
				// --search's '='-form; implies init mode (c.init=true). No short form.
				c.init = true
				c.initStore = val
			default:
				if c.unknownFlag == "" {
					c.unknownFlag = a
				}
			}
			continue
		}

		// (b) Short bundle: -xyz (single '-', not "--", len > 2). Expand into the
		// individual short flags; -s (value-taking) may consume the next token.
		// len-2 shorts ("-v", "-s", ...) and "--..." longs fall through to the switch.
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			if consumeNext, _ := expandShortBundle(&c, a, args, i); consumeNext {
				i++ // -s took its value from the next argv token
			}
			continue
		}

		switch a {
		case "--version", "-v":
			c.version = true
		case "--help", "-h":
			// --help takes precedence over everything else except itself (PRD §6.3
			// "help wins" tiebreak: checked FIRST in run, before --version). Help is
			// emitted PLAIN to stdout, exit 0. Dispatched in M5.T1.S1; parsed here.
			c.help = true
		case "--path", "-p":
			c.path = true
		case "--list", "-l":
			c.list = true
		case "--all", "-a":
			c.all = true
		case "--file", "-f":
			c.file = true
		case "--relative":
			c.relative = true
		case "--no-color":
			c.noColor = true
		case "--search", "-s":
			// Value-taking flag: consume the NEXT token verbatim as the query. The
			// value is NOT appended to c.tags (i++ skips it), and it never reaches
			// the default branch, so a dashed value (e.g. `--search -x` → query
			// "-x") is NOT mistaken for an unknown flag. If --search is the LAST
			// token (no value follows) searchMode stays false and the call falls
			// through to the no-recognized-mode default.
			if i+1 < len(args) {
				c.searchMode = true
				c.searchQ = args[i+1]
				i++
			}
		case "check":
			// `weave check` subcommand (PRD §9). `check` is a RESERVED positional
			// token: it selects validation mode and is NOT captured as a tag. An
			// extension literally tagged `check` cannot be resolved via `weave check`
			// (subcommand names are reserved, as in any CLI). Captured ANYWHERE in
			// argv (so `--no-color check` still selects check); run()'s
			// exclusivity check (M5) rejects check+tags / check+mode with exit 2. A
			// nested extension `writing/check` still resolves: this case matches
			// only the EXACT token "check".
			c.check = true
		case "--store":
			// `--store <dir>`: non-interactive store path for init (PRD §8.2). Mirrors
			// --search's next-token capture; implies init mode (c.init=true). No
			// short form. If it is the LAST token (no value follows) init stays
			// unset — mirrors --search-no-value (no exit-2 "needs argument" here;
			// the codebase defers that repo-wide).
			if i+1 < len(args) {
				c.init = true
				c.initStore = args[i+1]
				i++
			}
		case "init":
			// `weave init [<dir>]` first-run setup (PRD §8.2). `init` is a RESERVED
			// positional token (like `check`): it selects init mode and is NOT
			// captured as a tag. If the NEXT token is a positional <dir> (not a
			// dashed flag AND not a reserved subcommand check/init), capture it into
			// c.initStore and skip it (i++) — the `init <dir>` form. A following
			// flag (`init --store …`) or subcommand (`init check`) is left for its
			// own case so exclusivity can flag the conflict. GOTCHA: a store
			// literally named `check`/`init` must be passed via `--store`.
			c.init = true
			if i+1 < len(args) {
				next := args[i+1]
				if !strings.HasPrefix(next, "-") && next != "check" && next != "init" {
					c.initStore = next
					i++
				}
			}
		default:
			// Positional <tag> (PRD §6.1 `weave <tag> [<tag>...]`): a token that
			// does NOT start with '-' is a tag, captured here and resolved in run.
			// A dashed token NOT in the known set is an unknown flag (PRD §6 header:
			// exit 2): capture the FIRST offender for run() to report. Do NOT collect
			// a slice of unknowns — one loud error is the §6 contract.
			if strings.HasPrefix(a, "-") {
				if c.unknownFlag == "" {
					c.unknownFlag = a
				}
			} else {
				c.tags = append(c.tags, a)
			}
		}
	}
	return c
}

// expandShortBundle parses a combined short-flag token `a` (e.g. "-vh", "-pl",
// "-sfoo", "-ls") and applies the resulting flags to *c. It implements Issue 5's
// short-bundle normalization (decisions.md §D5). The caller has already guaranteed
// `a` is bundle-shaped: a single leading '-', not "--", and len(a) > 2.
//
// Semantics (PRD §6 short forms; the short set is exactly v h p l a f s):
//   - v/h/p/l/a/f are BOOL flags; each sets its config field.
//   - s is the VALUE-TAKING flag (--search): once seen, the rest of the body is
//     the query (e.g. "-sfoo" -> "foo"); if the rest is empty, the NEXT argv
//     token is consumed as the query (e.g. "-ls foo" -> list + query "foo"), and
//     the caller advances i (returns consumeNext=true). If no value is available
//     at all (empty rest AND no next arg), searchMode stays false — mirroring the
//     bare "-s"-with-no-value rule in the main switch.
//   - any char that is NEITHER a bool flag NOR the leading 's' is UNKNOWN: the
//     WHOLE bundle is rejected — c.unknownFlag is set to `a` and NOTHING is
//     applied. This two-phase (validate-then-commit) design is REQUIRED because
//     run() checks unknownFlag AFTER version/help (M5): a leaked `version=true`
//     from a partial "-vz" would make run() print the version (exit 0) and mask
//     the unknown-char error.
//
// Returns (consumeNext, ok). ok is always true for a bundle-shaped token (it was
// handled, validly or as-unknown). consumeNext=true tells the caller to i++ (the
// -s value came from the next argv token).
func expandShortBundle(c *config, a string, args []string, i int) (consumeNext, ok bool) {
	body := a[1:] // strip the single leading '-'

	// Phase 1 — validate. Walk bool flags left-to-right; the FIRST non-bool char
	// must be 's' (then the rest is the query) or it is unknown. Record where 's'
	// sits (sIdx) so Phase 2 knows where flags end and the query begins.
	sIdx := -1
	for j := 0; j < len(body); j++ {
		ch := body[j]
		if ch == 's' {
			sIdx = j
			break // 's' ends flag parsing; body[j+1:] is the query
		}
		switch ch {
		case 'v', 'h', 'p', 'l', 'a', 'f':
			// valid bool short flag (validated here; applied in Phase 2)
		default:
			// Unknown char: reject the WHOLE bundle. Commit nothing (two-phase).
			if c.unknownFlag == "" {
				c.unknownFlag = a
			}
			return false, true
		}
	}

	// Phase 2 — commit the bool flags in [0, sIdx) (or the whole body if no 's').
	end := len(body)
	if sIdx >= 0 {
		end = sIdx
	}
	for j := 0; j < end; j++ {
		switch body[j] {
		case 'v':
			c.version = true
		case 'h':
			c.help = true
		case 'p':
			c.path = true
		case 'l':
			c.list = true
		case 'a':
			c.all = true
		case 'f':
			c.file = true
		}
	}

	// Handle the value-taking 's' if it was present.
	if sIdx >= 0 {
		remainder := body[sIdx+1:]
		switch {
		case remainder != "":
			c.searchMode = true
			c.searchQ = remainder // value embedded in the bundle ("-sfoo")
		case i+1 < len(args):
			c.searchMode = true
			c.searchQ = args[i+1] // value is the next argv token ("-ls foo")
			return true, true     // caller advances i
		default:
			// 's' seen but no value anywhere: mirror the bare "-s"-no-value rule
			// (searchMode stays false). The bool flags before it remain set.
		}
	}
	return false, true
}

// run is the testable dispatcher. It returns the process exit code so main() can
// call os.Exit(run(...)) without tests ever invoking os.Exit. stdout/stderr are
// injected so tests capture output via *bytes.Buffer instead of the real streams.
//
// This milestone (P1.M1.T4.S1) dispatches ONLY --version and --path; every other
// parsed mode is a no-op (returns 0). The parser is ALREADY complete — later
// milestones add dispatch branches HERE, never parser changes:
//   - M2.T5.S1 adds the --list branch.
//   - M3.T2.S1 adds <tag>/--file/--all/--relative dispatch.
//   - M4.T3.S1 adds --search/check; M4.T4.S2 adds init.
//   - M5.T1.S1 adds --help, the unknownFlag → exit 2 branch, exclusivity, and the
//     no-args → usage → exit 1 path.
//
// Precedence (PRD §6.3): --version takes precedence over everything except
// --help (which lands in M5). So --version is the highest-precedence dispatch
// in THIS milestone: with both --version and --path set, version is printed and
// extdir.Find is never called (exit 0 even when the dir is unresolvable).
func run(args []string, stdout, stderr io.Writer) int {
	c := parseArgs(args)

	// 0) --help / -h (PRD §6.3 "help wins"): takes precedence over EVERYTHING,
	//    including --version and unknown flags. Usage to STDOUT, exit 0. Help is
	//    PLAIN (no ANSI) so `weave --help | grep` works and tests on non-TTY
	//    buffers are deterministic. Help wins the tiebreaks: --help --version
	//    -> usage (not the version line); --help --bogus -> usage (no error).
	if c.help {
		fmt.Fprint(stdout, usage())
		return 0
	}

	// 1) --version (PRD §6.3: precedes everything except --help, which is M5).
	//    Printed byte-exact: "weave <version>\n" — lowercase "weave", single
	//    space, the version var, newline. NOT "Weave", NOT "weave v%s".
	if c.version {
		fmt.Fprintf(stdout, "weave %s\n", version)
		return 0
	}

	// 1.1) Unknown dashed flag -> exit 2 (PRD §6 header "Unknown flags => error +
	//      exit 2"). stdout stays EMPTY (PRD §6.4: $(...) safety so
	//      `pi -e "$(weave --bogus)"` fails loudly with an empty command
	//      substitution instead of passing a garbage path). Checked AFTER
	//      --help/--version so those still win (the item's stated precedence:
	//      `--version --bogus` -> version wins, exit 0); checked BEFORE
	//      exclusivity so `--bogus --list` reports the unknown flag first (both
	//      exit 2; the unknown flag is the more fundamental error).
	if c.unknownFlag != "" {
		fmt.Fprintf(stderr, "weave: unknown flag '%s'\n", c.unknownFlag)
		return 2
	}

	// 1.2) Mode mutual exclusivity -> exit 2 (PRD §6.3). A pure predicate over
	//      config (no I/O): it reads only mode fields, never touches the store.
	//      GATES the init dispatch (1.5) and the mode ladder below: any forbidden
	//      combination exits before runInit/extdir.Find/Index ever run, so no
	//      partial output ever reaches stdout. Modifiers (--file/--relative/
	//      --no-color) NEVER trigger it — they combine with a single mode, so
	//      exclusivityError excludes them from its checks. stdout stays EMPTY.
	if bad, msg := exclusivityError(c); bad {
		fmt.Fprintln(stderr, msg)
		return 2
	}

	// 1.5) `weave init` dispatch (PRD §8.2). init is exclusive; until M5.T1.S1
	//      lands exclusivityError, init is placed right after --version (the
	//      highest-precedence dispatch present) and before the normal mode ladder.
	//      runInit orchestrates resolveStore → configpkg.Path → setupStore, then
	//      prints the --path rendering (dir→stdout, found-via→stderr) + the check
	//      report (→stdout, mirroring the standalone check) per §8.2 step 5. The
	//      bare-tag path never reaches here, so tag resolution never prompts
	//      (§6.4/§8.2 prompt-safety: stdin access is confined to resolveStore,
	//      which is called only here).
	if c.init {
		return runInit(c, stdout, stderr)
	}

	// 2) --path (PRD §6.1/§6.4). extdir.Find locates the dir via the §8.3 rules.
	//    On a miss err is extdir.ErrNotFound whose .Error() is the one-line fix
	//    `weave is not configured; run \`weave init\``. Print it verbatim to stderr
	//    (NO prefix, NO wrapping — fmt.Fprintln(stderr, err) prints err.Error()+"\n"),
	//    keep stdout EMPTY (§6.4: $(...) safety so `pi -e "$(weave x)"` fails
	//    loudly), exit 1. On success: dir→stdout byte-exact (the §13
	//    `test "$(weave --path)" = ...` gate captures stdout only), and the
	//    Issue-1 source label → stderr (NEVER stdout — it would break that gate).
	if c.path {
		dir, src, err := extdir.Find()
		if err != nil {
			fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline
			return 1
		}
		fmt.Fprintln(stdout, dir)                    // byte-exact dir + newline
		fmt.Fprintf(stderr, "(found via %s)\n", src) // Issue 1 source label
		return 0
	}

	// 3) --list / -l (PRD §6.1). The FIRST end-to-end wiring of the
	//    Find() -> discover.Index() -> ui.PrintList() data flow. Exit 1 on any
	//    failure path. This branch does NOT consult c.file / c.relative (--list
	//    prints a TABLE, not paths — PRD §6.2 header: modifiers combine with tag
	//    resolution or --all). Exclusivity (--list + --file, etc.) is M5.T1.S1.
	if c.list {
		// PRD §6.1 `weave --list`: resolve the store, build the index, render the
		// table. Find locates the dir via the §8.3 rules; Index walks it and
		// returns []Extension sorted by RelTag; PrintList renders TAG/NAME/DESCRIPTION.
		dir, _, err := extdir.Find() // src (2nd return) DISCARDED: --list does NOT print "(found via ...)" (that's --path-only)
		if err != nil {
			fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline (PRD §6.4/§8.2)
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		if len(exts) == 0 {
			// PRD §6.1: --list exits 1 "if no extensions found". Message to stderr so
			// stdout stays clean for any consumer. The +dir suffix is a debugging aid
			// (which dir was found-but-empty).
			fmt.Fprintln(stderr, "no extensions found in "+dir)
			return 1
		}
		// Color only when stdout is a TTY AND --no-color was not given (PRD §6.2).
		// A *bytes.Buffer (tests) / pipe / file is not a TTY -> plain output.
		ui.PrintList(stdout, exts, isTerminal(stdout) && !c.noColor)
		return 0
	}

	// 4) --search / -s <q> (PRD §6.1). Filters the index to extensions where <q>
	//    is a case-insensitive substring of the tag, package.json
	//    name/description/keywords, weave.aliases, or weave.category
	//    (internal/search), then renders the SAME table as --list via ui.PrintList
	//    (PRD §6.1: "same table format as --list, filtered"). The filtered slice
	//    keeps discover.Index's RelTag sort. Exit 0 with the table on matches;
	//    exit 1 (stderr message, EMPTY stdout) when nothing matches (PRD §6.1:
	//    "1 if no matches"). --no-color / TTY color gating is shared with --list;
	//    --file/--relative do NOT apply (search prints a TABLE, not paths — PRD
	//    §6.2: modifiers combine with tag resolution or --all only).
	if c.searchMode {
		dir, _, err := extdir.Find() // src DISCARDED: --search does NOT print "(found via ...)"
		if err != nil {
			fmt.Fprintln(stderr, err) // one-line fix (PRD §6.4/§8); stdout stays empty
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		matched := search.Search(c.searchQ, exts)
		if len(matched) == 0 {
			// PRD §6.1: exit 1 "if no matches". Mirror --list's "no extensions found"
			// convention: message to stderr, stdout stays clean.
			fmt.Fprintln(stderr, "no extensions matched "+c.searchQ)
			return 1
		}
		ui.PrintList(stdout, matched, isTerminal(stdout) && !c.noColor)
		return 0
	}

	// 5) `weave check` subcommand (PRD §9). Validates every extension in the store
	//    and prints a report: one OK line per clean extension, one line per finding
	//    (ERROR/WARN), ending with a "N extensions, M errors, K warnings" summary.
	//    Exit 0 if there are no ERRORs, 1 if there are any (WARNs never change the
	//    exit code, so `if weave check; then …` works as a gate). An empty store is
	//    clean (0 extensions, 0 errors, 0 warnings) → exit 0 (unlike --list, which
	//    exits 1 on empty).
	//
	//    check is a REPORT, not a path emitter: it ALWAYS prints its full findings
	//    to STDOUT (pipeable to less/grep, like eslint/ruff/govet) and signals
	//    pass/fail via the exit code. It is NOT subject to §6.4's "nothing on
	//    stdout on failure" — that contract is for tag/path emitters used inside
	//    $(...); check never participates in command substitution.
	//
	//    check.Check(dir, exts) takes the extensions DIR first because the §9
	//    empty-category-folder rule needs a filesystem walk (it cannot be derived
	//    from []Extension alone — discover.Index prunes empty subtrees). `dir` is
	//    already in scope from extdir.Find above. --file/--relative/--no-color do
	//    NOT apply (status report, not paths/table).
	if c.check {
		dir, _, err := extdir.Find() // src DISCARDED: check does NOT print "(found via ...)"
		if err != nil {
			fmt.Fprintln(stderr, err) // one-line fix (PRD §6.4/§8); stdout stays empty
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		rep := check.Check(dir, exts)
		// Render: status word left-padded to width 5 (OK/WARN/ERROR align); a clean
		// extension gets ONE OK line, a problem extension gets ONE line PER finding.
		// Name falls back to the BASENAME of RelTag when package.json name is empty
		// (a single-file or metadata-less extension has no name) — NOT "(none)": the
		// item description pins "<name or basename>".
		for _, er := range rep.ByExt {
			name := er.Extension.Name
			if name == "" {
				name = relTagBase(er.Extension.RelTag)
			}
			if len(er.Findings) == 0 {
				fmt.Fprintf(stdout, "%-5s %s (%s)\n", "OK", er.Extension.RelTag, name)
				continue
			}
			for _, f := range er.Findings {
				fmt.Fprintf(stdout, "%-5s %s (%s): %s\n", f.Level, er.Extension.RelTag, name, f.Message)
			}
		}
		// N = len(exts): the count of discovered EXTENSIONS. rep.ByExt may include
		// synthetic empty-folder entries (§9), which are NOT extensions, so do NOT
		// use len(rep.ByExt) for the count.
		fmt.Fprintf(stdout, "%d extensions, %d errors, %d warnings\n", len(exts), rep.Errors, rep.Warnings)
		if rep.HasErrors() {
			return 1
		}
		return 0
	}

	// 6) --all / -a (PRD §6.1). Print every extension's path, one per line,
	//    SORTED by canonical tag (discover.Index already sorts []Extension by
	//    RelTag, so this just walks the index in order). Exit 0 even for an EMPTY
	//    store (PRD §6.1 `--all` is ALWAYS exit 0, UNLIKE --list which exits 1 "if
	//    no extensions found" — --all is a scripting command where empty output +
	//    exit 0 is the useful shape). The --file/--relative modifiers apply here
	//    too (PRD §6.2 header: "combine with tag resolution or --all"), via the
	//    shared extensionPath() helper. This branch does NOT consult --list's
	//    "empty ⇒ exit 1" check.
	if c.all {
		dir, _, err := extdir.Find() // src (2nd return) DISCARDED: only --path prints it
		if err != nil {
			fmt.Fprintln(stderr, err) // ErrNotFound.Error() verbatim + newline (PRD §6.4/§8)
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		for _, e := range exts {
			fmt.Fprintln(stdout, extensionPath(e, dir, c)) // absolute path by default; --file/--relative apply
		}
		return 0
	}

	// 7) Tag-resolution mode: `weave <tag> [<tag>...]` (PRD §6.1). Resolves each
	//    tag to its extension path and prints one path per line, in INPUT order.
	//
	//    ATOMICITY (PRD §6.4 — the critical-for-$(...) contract): resolve EVERY
	//    tag first, buffering the resulting paths; if ANY tag fails
	//    (unknown/ambiguous), print one error line per problem tag to stderr,
	//    print NOTHING to stdout, and exit 1. The buffered paths are flushed ONLY
	//    when the whole invocation is known-good. This makes `pi -e "$(weave bad)"`
	//    fail loudly (empty $(...), exit 1) instead of passing a partial or garbage
	//    path. Each error is printed verbatim from resolve's typed errors —
	//    *UnknownError names the tag, *AmbiguousError lists the candidate full tags
	//    (NO "weave:" prefix, matching the extdir.ErrNotFound convention used by
	//    --path/--list). The default output is ext.Path (the resolvable path:
	//    file/dir/package); --file swaps to ext.EntryFile and --relative makes it
	//    relative to the extensions dir (applied by extensionPath, PRD §6.2).
	if len(c.tags) > 0 {
		dir, _, err := extdir.Find() // src DISCARDED: tag mode does NOT print "(found via ...)"
		if err != nil {
			fmt.Fprintln(stderr, err) // one-line fix verbatim (PRD §6.4/§8); stdout stays empty
			return 1
		}
		exts, err := discover.Index(dir)
		if err != nil {
			fmt.Fprintln(stderr, err) // e.g. dir vanished between Find and Index
			return 1
		}
		paths := make([]string, 0, len(c.tags)) // buffered; flushed ONLY if all resolve
		hadErr := false
		for _, tag := range c.tags {
			res, rerr := resolve.Resolve(tag, exts)
			if rerr != nil {
				fmt.Fprintln(stderr, rerr) // one error line per problem tag (verbatim; NO "weave:" prefix)
				hadErr = true
				continue
			}
			// extensionPath applies --file (EntryFile vs Path) and --relative (Rel to
			// extensions dir); default is the absolute resolvable path (PRD §6.1/§6.2).
			paths = append(paths, extensionPath(res.Extension, dir, c))
		}
		if hadErr {
			return 1 // paths buffered but NEVER written → stdout empty (§6.4)
		}
		for _, p := range paths {
			fmt.Fprintln(stdout, p) // one path per line, input order (NOT sorted)
		}
		return 0
	}

	// No recognized mode -> usage to STDERR, exit 1 (PRD §6.3: parity with
	// get-server-config.sh / skilldozer). Covers BOTH truly-no-args (`weave`)
	// and modifiers-only (`weave --no-color`): modifiers are NOT a mode, they
	// combine with one, so if weave was asked to DO nothing, show usage. stdout
	// stays EMPTY (PRD §6.4) so `$(weave)` in command substitution never sees
	// garbage — a wrapper script notices the non-zero exit and the empty $(...).
	fmt.Fprint(stderr, usage())
	return 1
}

// extensionPath returns the path to print for a resolved extension, applying the
// PRD §6.2 --file and --relative modifiers. It is the shared formatter used by
// BOTH the <tag>-resolution loop and the --all loop, so the modifiers behave
// identically in the two modes (PRD §6.2 header: "combine with tag resolution
// or --all").
//
// Precedence of effects:
//   - default (neither flag): ext.Path — the ABSOLUTE resolvable path (the file
//     for single-file extensions, the directory for dir/package extensions).
//     PRD §6.1.
//   - --file:                 ext.EntryFile — the ABSOLUTE .ts/.js file pi loads
//     (the file itself for single-file, index.ts for dir, the first
//     pi.extensions entry for package). PRD §6.2.
//   - --relative:             the chosen path rewritten to be RELATIVE to the
//     extensions dir (PRD §6.2 "machine-local convenience").
//   - --file --relative:      they COMBINE — an entry-file path relative to the
//     extensions dir (e.g. "writing/reddit-poster" or "git-checkpoint/index.ts").
//
// Kind-independence (load-bearing): extensionPath does NOT switch on ext.Kind.
// ext.Path and ext.EntryFile are ALREADY populated per Kind by discover.Index
// (file → Path==EntryFile==the file; dir → Path is the dir, EntryFile is
// dir+"/index.ts"; package → Path is the dir, EntryFile is the first existing
// pi.extensions entry). Reading whichever field the flags select is the ENTIRE
// logic; adding a `switch ext.Kind` is an anti-pattern.
//
// filepath.Rel cannot fail in practice here: both arguments are ABSOLUTE
// (ext.Path/ext.EntryFile are set absolute by discover.Index; extensionsDir is
// absolute from extdir.Find), and ext.Path/ext.EntryFile are always UNDER
// extensionsDir (they were discovered by walking it), so a clean relative path
// always exists. The err guard is defensive only: on a (theoretical) Rel failure
// weave falls back to the absolute path, which is still a correct, usable answer
// rather than crashing.
func extensionPath(ext discover.Extension, extensionsDir string, c config) string {
	p := ext.Path // default: absolute resolvable path (PRD §6.1/§6.2)
	if c.file {
		p = ext.EntryFile // --file: the .ts/.js entry file pi loads (PRD §6.2)
	}
	if c.relative {
		if rel, err := filepath.Rel(extensionsDir, p); err == nil {
			p = rel // --relative: path relative to the extensions dir
		}
	}
	return p
}

// relTagBase returns the final '/'-component of a canonical RelTag, used as the
// display-name fallback in `weave check` when an extension has no package.json
// name (a single-file or metadata-less extension). "writing/reddit-poster" →
// "reddit-poster"; "example" → "example". It mirrors resolve's basename
// resolution (PRD §7.2) so the "(<name or basename>)" parenthetical in the
// check report is the SAME short name a user would type.
func relTagBase(relTag string) string {
	if i := strings.LastIndex(relTag, "/"); i >= 0 {
		return relTag[i+1:]
	}
	return relTag
}

// exclusivityError reports whether c combines modes that PRD §6.3 forbids,
// returning a one-line stderr message describing the first violation found.
// It is a PURE predicate over config (no I/O, no side effects), so it is
// directly unit-testable and runs before any dir resolution. Ported verbatim
// from skilldozer main.go exclusivityError, with the `skilldozer:` message
// prefix swapped to `weave:` (PRD §6 header byte-identity mandate).
//
// Six families, checked IN ORDER (first hit wins, first message returned):
//
//	(a) 2+ listing modes (path/list/search/all)
//	(b) tags + an inspection mode (path/list/search/all)
//	(c) check + tags
//	(d) check + a listing mode
//	(e) init + tags
//	(f) init + a mode (check/list/search/all/path)
//
// FAMILY (b) NOTE: the subtask description's (b) literally omits --path, but
// PRD §6 mandates the contract is byte-identical to skilldozer (whose Issue 3
// deliberately ADDED tags+path to avoid silently dropping a stray tag on
// `weave foo --path` — without it, --path would print the dir and discard `foo`
// with no error), and the item's own families (d) and (f) DO include --path,
// confirming path belongs in the set. So (b) checks the FULL set
// {path,list,searchMode,all}.
//
// Modifiers --file/--relative/--no-color NEVER trigger exclusivity: they combine
// with a single mode (e.g. `--all --file`, `--list --no-color`) and so are
// excluded from every check below.
func exclusivityError(c config) (bad bool, msg string) {
	// (a) PRD §6.3: any 2+ of the listing modes are mutually exclusive.
	n := 0
	for _, b := range []bool{c.path, c.list, c.searchMode, c.all} {
		if b {
			n++
		}
	}
	if n >= 2 {
		return true, "weave: listing modes --path/--list/--search/--all are mutually exclusive"
	}
	hasTags := len(c.tags) > 0
	// (b) tags + an inspection mode (PRD §6.3; +path per byte-identity — see doc comment).
	if hasTags && (c.path || c.list || c.searchMode || c.all) {
		return true, "weave: tags cannot be combined with --path/--list/--search/--all"
	}
	// (c) check + tags.
	if c.check && hasTags {
		return true, "weave: 'check' cannot be combined with tag arguments"
	}
	// (d) check + a listing mode.
	if c.check && (c.path || c.list || c.searchMode || c.all) {
		return true, "weave: 'check' cannot be combined with --path/--list/--search/--all"
	}
	// (e)+(f) init is its own exclusive mode (PRD §6.3 / §8.2): rejects stray
	//        tags AND modes. Both checked under c.init so the message matches the
	//        offending field (tags vs modes), not a generic init-vs-anything error.
	if c.init {
		if hasTags {
			return true, "weave: 'init' cannot be combined with tag arguments"
		}
		if c.check || c.list || c.searchMode || c.all || c.path {
			return true, "weave: 'init' cannot be combined with --list/--search/--all/--path/check"
		}
	}
	return false, ""
}

// --- init store selection (M4.T4.S1) ---

// stdinIsTerminal reports whether os.Stdin is an interactive terminal (a
// character device), used by resolveStore to gate init's interactive prompt
// (PRD §8.2 "Prompt safety": prompt ONLY on a real TTY). It stats os.Stdin and
// checks the os.ModeCharDevice bit — the same stdlib heuristic the package-level
// isTerminal var uses for stdout color gating, but on a DIFFERENT stream.
//
// It is a plain FUNCTION, NOT a package var: the contract's test seam is
// chooseStore's isTTY PARAMETER (injected per-call), not a global override.
// (Contrast isTerminal, which IS a var because run() has no isTTY parameter.)
//
// Known harmless caveat: /dev/null is also a char device, so stdinIsTerminal()
// reports true for `weave init < /dev/null`, but a read there yields immediate
// EOF and readPrompt returns the default (never blocks). No golang.org/x/term
// (the ModeCharDevice heuristic is the established repo pattern).
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPrompt prints the prompt (label, with [def] in brackets) to w, reads one
// line from r, and returns the trimmed answer — or def when the user presses
// Enter (empty line) or sends EOF on an otherwise-empty line. A genuine read
// error (non-EOF) is returned. Used by init's interactive prompt (PRD §8.2).
//
// bufio.Reader.ReadString('\n') is preferred over bufio.Scanner (Scanner is
// line-oriented but awkward for a single interactive read; ReadString returns
// (line, error) where error == io.EOF if no newline precedes end-of-input — and
// a bare EOF with empty text means "accept default", NOT a hard error, so
// `weave init < /dev/null` and `echo | weave init` behave like "press Enter").
func readPrompt(r *bufio.Reader, w io.Writer, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	line, err := r.ReadString('\n') // includes the trailing '\n'
	if err != nil && err != io.EOF {
		return "", err
	}
	if s := strings.TrimSpace(line); s != "" {
		return s, nil
	}
	return def, nil // empty Enter OR EOF-with-no-text ⇒ accept default
}

// chooseStore resolves the store directory for `weave init` (PRD §8.2) via a
// 4-step decision that is fully independent of os.Stdin/os.Stdout/os.Getwd: the
// caller injects cwd, isTTY, the default store, and a prompt function, so the
// logic is unit-testable without a real terminal (the contract FACTORING).
//
// Resolution order (first applicable wins):
//  1. haveStore != "" — the non-interactive override from `init <dir>` or
//     `--store <dir>`. Returned VERBATIM; the prompt is NEVER called (scripts/CI).
//  2. auto-detect the default: if cwd already looks like a store (it contains at
//     least one extension entry at any depth — extdir.HasExtensionEntry, PRD §8.2
//     "detected extensions in <cwd>"), default = cwd; else default = defaultStore
//     (the $XDG_DATA_HOME/weave/extensions value from config.DefaultStore).
//  3. !isTTY and no explicit haveStore — return the auto-detected default with NO
//     prompt (scripts / CI / pipes). The prompt is NEVER called.
//  4. isTTY — prompt "Where should weave keep your extensions? [<default>]".
//     readPrompt makes empty line / EOF ⇒ default; a typed path ⇒ override.
//
// The chosen string is returned VERBATIM (it may be relative if the user typed a
// relative path); resolveStore absolutizes it via filepath.Abs. A non-nil error
// is returned ONLY on a genuine prompt read failure (a non-EOF error from the
// prompt fn); empty/EOF is "accept default", never an error.
//
// WIRED BY P1.M4.T4.S2's runInit (via resolveStore). Not yet called in run().
func chooseStore(haveStore, cwd string, isTTY bool, defaultStore string, prompt func(label, def string) (string, error)) (string, error) {
	// (1) Non-interactive override: `init <dir>` / `--store <dir>`. No prompt.
	if haveStore != "" {
		return haveStore, nil
	}
	// (2) Auto-detect the default from cwd (PRD §8.2 "detected extensions in <cwd>").
	def := defaultStore
	if extdir.HasExtensionEntry(cwd) {
		def = cwd
	}
	// (3) Off-TTY (pipe/file/CI): use the default, NO prompt (never blocks).
	if !isTTY {
		return def, nil
	}
	// (4) Interactive: prompt. Empty/EOF answer ⇒ def (the auto-detected default);
	// a typed path ⇒ override (returned verbatim). A genuine read error propagates.
	choice, err := prompt("Where should weave keep your extensions?", def)
	if err != nil {
		return "", err
	}
	if choice == "" {
		return def, nil
	}
	return choice, nil
}

// resolveStore is the I/O-bearing wrapper around chooseStore that run()'s init
// dispatch (P1.M4.T4.S2) calls. It supplies the real dependencies — os.Getwd(),
// configpkg.DefaultStore(), the os.Stdin TTY check (stdinIsTerminal), and a bufio
// prompt reader over os.Stdin/os.Stderr (readPrompt) — and returns chooseStore's
// choice ABSOLUTIZED via filepath.Abs (PRD §8.2 "absolute store path"). The ONE
// shared bufio.NewReader is created here and captured by the prompt closure so a
// future second prompt would reuse it (a fresh reader per prompt can swallow
// buffered bytes).
//
// The os.Stdin / os.Getwd access is confined to THIS function so the pure
// decision logic in chooseStore stays terminal-free and unit-testable. The prompt
// is written to STDERR (not stdout) so init's stdout stays the bare store path —
// a caller doing store="$(weave init)" must not capture the interactive prompt
// line (PRD §6.1/§6.4 spirit). A genuine cwd/default/absolutize/prompt error is
// returned wrapped; an empty or EOF prompt answer is NOT an error (readPrompt ⇒
// default).
//
// WIRED BY P1.M4.T4.S2's runInit. Not yet called in run().
func resolveStore(haveStore string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve cwd: %w", err)
	}
	def, err := configpkg.DefaultStore()
	if err != nil {
		return "", fmt.Errorf("weave init: resolve default store: %w", err)
	}
	r := bufio.NewReader(os.Stdin)
	// Prompt dialog goes to STDERR, not stdout, so it never pollutes a
	// store="$(weave init)" capture. readPrompt takes a generic io.Writer so the
	// choice is made HERE in the wrapper, not baked into the pure readPrompt.
	prompt := func(label, def string) (string, error) {
		return readPrompt(r, os.Stderr, label, def)
	}
	store, err := chooseStore(haveStore, cwd, stdinIsTerminal(), def, prompt)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(store)
	if err != nil {
		return "", fmt.Errorf("weave init: absolutize store: %w", err)
	}
	return abs, nil
}

// setupStore is the create+seed+write-config half of `weave init` (PRD §8.2
// steps 2-4); the store-CHOICE half is resolveStore (M4.T4.S1), and run()'s
// `if c.init` dispatch calls both via runInit. Both store and configPath are
// INJECTED strings (store is already absolute — resolveStore absolutized it;
// configPath is configpkg.Path()'s result from runInit), so this is directly
// unit-testable with temp paths (no wrapper layer).
//
// KEY WEAVE DELTA vs skilldozer: seed store/example.ts (a SINGLE FILE at the
// store ROOT), NOT store/example/SKILL.md in a subdirectory. There is NO
// exampleDir and NO second MkdirAll — only the one os.MkdirAll(store, 0o755).
//
// Returns (seeded, nil) on success or (false, err) on any fs failure. `seeded`
// is a SUCCESS-PATH signal (runInit prints "Seeded" vs "Adopted"); callers MUST
// check err first.
func setupStore(store, configPath string) (seeded bool, err error) {
	if err := os.MkdirAll(store, 0o755); err != nil { // (a) ensure store exists (idempotent)
		return false, fmt.Errorf("weave init: create store dir %q: %w", store, err)
	}
	entries, err := os.ReadDir(store) // (b) seed only if EMPTY (zero entries of any kind)
	if err != nil {
		return false, fmt.Errorf("weave init: read store dir %q: %w", store, err)
	}
	if len(entries) == 0 {
		// Single-file extension at the store ROOT (PRD §11/§7.1). NO example/ subdir.
		if err := os.WriteFile(filepath.Join(store, "example.ts"), []byte(exampleExtensionTemplate), 0o644); err != nil {
			return false, fmt.Errorf("weave init: seed example.ts: %w", err)
		}
		seeded = true
	}
	// (c) Non-empty: adopt in place. Do NOTHING to existing files (PRD §17). seeded stays false.
	if err := configpkg.Save(configPath, configpkg.File{Store: store}); err != nil { // (d) ALWAYS write config
		return false, fmt.Errorf("weave init: write config %q: %w", configPath, err)
	}
	return seeded, nil
}

// runInit is the `weave init` orchestrator (PRD §8.2). run()'s dispatch calls it
// when c.init is true (init is exclusive; M5's exclusivityError will enforce
// that, but is not present yet — init is placed right after --version). It
// assembles S1's resolveStore + configpkg.Path + setupStore, then reports: the
// configured store path to stdout (PRD §6.1), the `--path` "found via"
// annotation to stderr (PRD §8.2 step 5), and the check report to STDOUT (PRD
// §8.2 step 5 — weave reproduces its own `check` output; this DIVERGES from
// skilldozer, which put init's check report on stderr). Exit 0 once
// create+config succeed; check findings NEVER change init's exit code
// (best-effort report — item description: "check findings don't change init
// exit code").
//
// The bare `weave <tag>` path NEVER reaches here (c.init is false for tags),
// so tag resolution never prompts (PRD §6.4/§8.2): stdin access is confined to
// resolveStore, which is called only in this function.
func runInit(c config, stdout, stderr io.Writer) int {
	store, err := resolveStore(c.initStore) // (1) S1: choose + absolutize (haveStore!="" never blocks)
	if err != nil {
		fmt.Fprintln(stderr, err) // resolveStore wraps with "weave init: …"
		return 1
	}
	cfgPath, err := configpkg.Path() // (2) config-file location ($weave_CONFIG or XDG default)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	seeded, err := setupStore(store, cfgPath) // (3) create + seed-if-empty + write config
	if err != nil {
		fmt.Fprintln(stderr, err) // setupStore wraps with "weave init: …"
		return 1
	}
	if seeded { // (4) report (STDERR — stdout stays the store-path headline)
		fmt.Fprintf(stderr, "Seeded example extension at %s\n", filepath.Join(store, "example.ts"))
	} else {
		fmt.Fprintf(stderr, "Adopted existing store at %s\n", store)
	}
	dir, src, ferr := extdir.Find() // (5) effective store + which §8.3 rule won (AFTER setupStore)
	if ferr != nil {
		fmt.Fprintln(stderr, ferr) // should not happen (config just written); fall back to store
		dir = store
	}
	fmt.Fprintln(stdout, dir) // §6.1: stdout = the configured store path
	if ferr == nil {
		fmt.Fprintf(stderr, "(found via %s)\n", src) // mirror `weave --path`
	}
	exts, ierr := discover.Index(dir) // (6) check report on the effective store (best-effort)
	if ierr != nil {
		fmt.Fprintln(stderr, ierr)
		return 0 // setup OK; the report is best-effort
	}
	rep := check.Check(dir, exts) // DIR FIRST, exts SECOND (weave signature)
	// Render to STDOUT (mirrors the standalone `weave check` branch; diverges
	// from skilldozer, which rendered init's check report to stderr).
	for _, er := range rep.ByExt {
		name := er.Extension.Name
		if name == "" {
			name = relTagBase(er.Extension.RelTag) // reuse the existing main.go helper
		}
		if len(er.Findings) == 0 {
			fmt.Fprintf(stdout, "%-5s %s (%s)\n", "OK", er.Extension.RelTag, name)
			continue
		}
		for _, f := range er.Findings {
			fmt.Fprintf(stdout, "%-5s %s (%s): %s\n", f.Level, er.Extension.RelTag, name, f.Message)
		}
	}
	fmt.Fprintf(stdout, "%d extensions, %d errors, %d warnings\n", len(exts), rep.Errors, rep.Warnings)
	return 0 // setup succeeded; check findings do not change init's exit code
}
