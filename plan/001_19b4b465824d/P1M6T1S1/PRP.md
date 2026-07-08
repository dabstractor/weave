# PRP — P1.M6.T1.S1: `extensions/example.ts` + run §13 acceptance verification

> **Subtask:** Ship the ONE repo example extension on disk and verify the FULL
> §13 acceptance suite passes. The extension body already exists as the compiled-in
> `exampleExtensionTemplate` string constant in `main.go` (L84-110, from P1.M4.T4.S2 —
> `setupStore` writes it into an empty store's `example.ts` during `weave init`). This
> subtask creates the REPO ASSET `extensions/example.ts` (the file `--list`/resolution
> demonstrate out of the box) byte-for-byte equal to that constant (both == PRD §11),
> and runs every §13 criterion against a built binary. **All criteria already pass in a
> clean environment** (verified by running the real binary + real file — see
> `research/acceptance_results.md`); the only deliverable is the file plus a runnable
> acceptance check.

---

## Goal

**Feature Goal**: Land `extensions/example.ts` as a committed, tracked repo asset whose
content is byte-for-byte identical to (a) PRD §11 and (b) the `exampleExtensionTemplate`
raw-string constant in `main.go`. Then PROVE the complete PRD §13 acceptance suite
passes against a `go build` binary — build, version, discovery/path, list, tag
resolution, `-f`, unknown-tag contract, absolute-path contract, `check`, dir/package
styles, symlink install, config/first-run, and the pi `--no-extensions -e` load.

**Deliverable**:
1. `extensions/example.ts` — a NEW file (724 bytes), content = the EXACT §11 body
   (leading JSDoc + `import type` + `export default function`). The JSDoc is what
   `weave --list` prints as the description.
2. (OPTIONAL, recommended) a ~6-line Go test in `main_test.go` —
   `TestExampleAssetEqualsTemplate` — that reads `extensions/example.ts` and asserts it
   equals `exampleExtensionTemplate`, locking the cross-task byte-equality contract into
   `go test ./...`.
3. VERIFIED acceptance: the full §13 suite run green (documented below; no implementation
   fix is required — every criterion passes once the environment is clean).

**Success Definition**:
- `extensions/example.ts` exists and `diff` against the `exampleExtensionTemplate` body
  shows zero differences.
- `git status` shows `extensions/example.ts` as a NEW tracked file (NOT gitignored —
  `.gitignore` ignores `/weave` the binary, not `*.ts`).
- `go build -o weave . && ./weave --version` prints `weave <something>`.
- Every §13 check prints its OK line (build, version, path=sibling, list shows example,
  example→file, -f example→file, unknown-tag empty-stdout/exit-1, absolute-path,
  check rc=0, dir+package styles, symlink→repo, config/first-run, pi no-error).
- `go test ./...` stays green (and, if the optional guard test is added, asserts the
  byte-equality).

## User Persona (if applicable)

**Target User**: (1) A first-time clone user who runs `./weave --list` and needs to SEE a
real extension + description so the loader is demonstrable out of the box (PRD §11:
"Ship exactly one example so `--list`/resolution are demonstrable"). (2) The release
engineer / CI that must prove the §13 contract holds before tagging. (3) A downstream
user who copies `extensions/example.ts` as the template for their own extension.

**Use Case**: `./weave --list` (see the example + its JSDoc description); `pi -e "$(./weave
example)" ...` (load it); `./weave example` (resolve the tag to an absolute path).

**User Journey**: clone → `go build -o weave .` → `./weave --list` shows `example` with
the reference description → `pi --no-extensions -e "$(./weave example)" -p "..."` loads
the extension without error. The example is explicitly "safe to delete once you add real
extensions" (its own JSDoc).

**Pain Points Addressed**: out-of-the-box demonstrability (no example ⇒ `--list` is empty
and the contract is unprovable); a copy-paste template for the dominant single-file
pattern (JSDoc description + `import type` + default-export factory).

## Why

- **Closes the §11 contract** — PRD §11 mandates exactly ONE shipped example, a single-file
  `.ts` carrying a leading JSDoc. Until this file exists on disk, `--list`/resolution have
  nothing to demonstrate in a clean clone.
- **Satisfies the cross-task byte-equality contract** — `main.go` L77: "the repo asset
  extensions/example.ts created by P1.M6.T1.S1 MUST equal this byte-for-byte (both == PRD
  §11)." Both the compiled-in seed (written by `setupStore` into an empty store) and the
  repo asset are the SAME text; this task lands the repo-asset half.
- **Proves the §13 acceptance gate** — the implementer MUST verify all §13 criteria pass.
  This task is that verification. It is the gate M6 (and thus v1.0) ships behind.
- **Enables M6.T2 (install.sh) and M6.T4 (README)** — both reference `extensions/example.ts`
  as the shipped example; install.sh's verification step is literally `weave example`.

## What

A single new file is committed: `extensions/example.ts`. No source-code (`.go`) change is
REQUIRED for acceptance (the binary already implements M1-M5). An OPTIONAL test is added to
`main_test.go` to durably guard byte-equality. Then the §13 suite is run and recorded.

### Success Criteria

- [ ] `extensions/example.ts` exists, content == §11 body == `exampleExtensionTemplate`
      (zero `diff`), 724 bytes, UTF-8, exactly one trailing newline.
- [ ] `git status --porcelain` lists `extensions/example.ts` as a new untracked-then-added
      file; it is NOT ignored by `.gitignore`.
- [ ] `go build -o weave . && echo OK` → `OK`.
- [ ] `./weave --version` → `weave dev` (or `weave <git-describe>` if built with ldflags).
- [ ] `test "$(./weave --path)" = "$PWD/extensions"` passes (sibling-of-binary rule; REQUIRES
      a clean config/env — see Known Gotchas).
- [ ] `./weave --list` shows the `example` row with the JSDoc description.
- [ ] `test -f "$(./weave example)"` and `test -f "$(./weave -f example)"` both pass.
- [ ] Unknown tag `nope`: empty stdout, exit 1.
- [ ] `./weave example` output starts with `/` (absolute-path contract).
- [ ] `./weave check` exits 0, reports `example` as OK.
- [ ] Dir + package styles resolve (temp `/tmp/edz-acc` scaffolding from §13).
- [ ] Symlink `/tmp/weave-bin/weave → $PWD/weave` still resolves `example` to
      `$PWD/extensions/example.ts`.
- [ ] Config + first-run: unconfigured hint (exit 1, `run \`weave init\``), non-interactive
      `init --store`, config-wins, env-beats-config.
- [ ] `pi --no-extensions -e "$(./weave example)" -p "…"` does NOT error (exit 0, no
      "Failed to load extension"). pi is on PATH (v0.80.3) — NOT deferred.
- [ ] (If added) `TestExampleAssetEqualsTemplate` passes inside `go test ./...`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to
implement this successfully?_ **Yes.** The entire deliverable is ONE TypeScript file whose
exact bytes are specified verbatim (PRD §11 == `exampleExtensionTemplate`, both transcribed
below). The acceptance suite is the verbatim PRD §13 shell block, augmented with the
clean-env preamble (the one gotcha). The only codebase facts an implementer needs: where
`exampleExtensionTemplate` lives (main.go L84-110), that `.gitignore` does not ignore
`extensions/` (verified), that the sibling rule needs `extensions/` to EXIST as a dir
(no entry-count check — verified in `internal/extdir/extdir.go` `resolveSiblingFromExe`),
and that pi is on PATH at `/home/dustin/.local/bin/pi` (v0.80.3). No guessing.

### Documentation & References

```yaml
# MUST READ — the verified acceptance results (ground truth: ran the real binary + real file)
- docfile: plan/001_19b4b465824d/P1M6T1S1/research/acceptance_results.md
  why: "Every §13 check was RUN against a real build + real extensions/example.ts. This
        file is the transcript: 9/9 core, 4/4 dir+package, 2/2 symlink, 5/5 config/first-run
        all PASS; the pi no-error criterion PASSES via a contrast proof (broken ext → exit 1
        + 'Failed to load extension'; valid example.ts → exit 0); and §7 is the CRITICAL
        clean-env gotcha (stray ~/.config/weave/config.yaml hijacks the sibling rule)."
  critical: "§7 (clean-env gotcha) is the #1 false-failure an implementer will hit. §6 (pi
             proof is a contrast, not model prose) prevents chasing a non-defect. §9 (no
             implementation fixes required) confirms the task is file+verify, not debug."

# CONTRACT — the file's exact bytes (PRD §11 == main.go exampleExtensionTemplate)
- docfile: PRD.md
  section: "§11 'The one shipped example extension' — the fenced ```typescript block is the
           VERBATIM content of extensions/example.ts (724 bytes, zero backticks, one
           trailing newline). §16 confirms it is committed (not gitignored). §17 forbids
           shipping >1 example."
  critical: "Copy the fenced block EXACTLY. Do not reformat, do not add/remove blank lines,
             do not change the JSDoc wording — --list prints the JSDoc verbatim and the
             byte-equality with exampleExtensionTemplate must hold."

# CONTRACT — the compiled-in twin (must equal the repo asset byte-for-byte)
- file: main.go
  why: "L71-110: exampleExtensionTemplate doc comment + raw-string constant. L77 states the
        cross-task contract. L855-873: setupStore writes this constant into an empty store's
        example.ts during `weave init` (PRD §8.2 step 3). The repo asset MUST equal this."
  pattern: "It is a `const … = \`…\`` raw string (NO embed — PRD §17 + the L74-78 comment
            explain why: 'nothing about the user's collection is compiled in'; the example is
            the one compiled-in exception, but a string constant was chosen over go:embed)."
  gotcha: "The body has ZERO backticks (verified) so no `+ \"\`\" +` splicing. If you ever
           edit the body, re-verify zero backticks or switch to the splice form."

# The sibling rule — what makes `--path`/`example` resolve in a clean clone
- file: internal/extdir/extdir.go
  why: "resolveSiblingFromExe (the rule-3 core): EvalSymlinks(exe) → repoDir → candidate =
        repoDir/extensions → wins iff os.Stat says it IS an existing directory. NOTE: rule 3
        does NOT call HasExtensionEntry — merely existing as a dir is enough. So creating
        extensions/example.ts (which creates the extensions/ dir) is sufficient for the
        sibling rule to fire. Find() order is env → config → sibling → walkup, so a stray
        config/env (rules 1-2) wins FIRST — see Known Gotchas."
  critical: "EvalSymlinks MUST stay (macOS-essential). On Linux /proc/self/exe already
             resolves the symlink, so Linux tests pass with OR without it — do not use that
             as justification to remove it."

# pi loading facts — the --no-extensions -e contract
- docfile: plan/001_19b4b465824d/architecture/pi_extension_facts.md
  section: "§2 (-e accepts files AND dirs), §3 (--no-extensions disables DISCOVERY but
            explicit -e paths STILL load — cliEnabledExtensions is always merged), §1
            (extension = default-export factory loaded via jiti; `import type` is fully
            erased so @earendil-works/pi-coding-agent never needs to resolve at runtime)."
  critical: "The example.ts uses `import type { ExtensionAPI }` — the `type` keyword erases
             the import at jiti compile time, so NO node_modules / package install is needed
             for pi to load it. This is why the pi end-to-end test works with zero npm setup."

# The acceptance suite itself (authoritative)
- docfile: PRD.md
  section: "§13 'Acceptance criteria' — the fenced ```bash block is the verbatim suite. Run
            it (with the clean-env preamble from Known Gotchas). Every line must pass."
```

### Current Codebase tree (relevant subset)

```bash
main.go                      # exampleExtensionTemplate const at L84-110 (the byte-twin)
internal/extdir/extdir.go    # sibling rule (rule 3) — fires once extensions/ exists
.gitignore                   # ignores /weave (binary), NOT extensions/*.ts
extensions/                  # ← DOES NOT EXIST YET (this task creates it + example.ts)
```

### Desired Codebase tree with files to be added

```bash
extensions/example.ts        # ← NEW (724 bytes): the §11 body, == exampleExtensionTemplate
main_test.go                 # ← OPTIONAL ADD: TestExampleAssetEqualsTemplate (~6 lines)
# (no .go source change required for acceptance; no go.mod/go.sum change; no .gitignore change)
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the #1 false failure): §13's *Discovery + path* block does NOT isolate
# HOME/XDG_CONFIG_HOME — it assumes "a clean clone." In a real dev shell a stray
# ~/.config/weave/config.yaml (created by ANY prior `weave init` — testing in THIS
# session left one pointing at ~/projects/weave/foo) fires §8.3 rule 2 (config) BEFORE
# rule 3 (sibling). Result: `./weave --path` returns the config store, NOT
# $PWD/extensions, and `test "$(./weave --path)" = "$PWD/extensions"` FAILS — a false
# negative that looks like an implementation bug but is purely environmental.
# FIX (environment, NOT code): start clean before running §13:
#   unset weave_EXTENSIONS_DIR weave_CONFIG
#   rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/weave/config.yaml"
# …or run the Discovery block under an isolated HOME=$(mktemp -d) (what §13's own
# Config+first-run block already does). The PRP's acceptance runner bakes this in.

# CRITICAL (pi proof is a CONTRAST, not model prose): `pi --no-extensions -e "$(./weave
# example)" -p "name one command it registered"` may NOT name the weave-example command
# in its answer, because commands registered by an ad-hoc -e extension are not
# auto-surfaced into the model's context and session_start ui.notify does not print to
# -p one-shot stdout. This is a test-design artifact, NOT a defect. The HARD criterion
# ("does not error") is proven by CONTRAST: a broken extension → exit 1 +
# "Failed to load extension: ParseError"; the valid example.ts → exit 0, no load error.
# Do NOT "fix" example.ts to chase model prose.

# GOTCHA: the example.ts uses `import type { ExtensionAPI } from "@earendil-works/pi-
# coding-agent"`. The `type` modifier ERASES the import at jiti compile time, so pi
# loads the file with ZERO npm/node_modules setup. If someone drops the `type` keyword,
# pi would try to resolve the package and fail. Leave `import type` exactly as written.

# GOTCHA: .gitignore ignores `/weave` (the built BINARY), `/dist`, `/build`,
# `node_modules/`, `.pi-subagents/`. It does NOT ignore `extensions/` or `*.ts`, so
# extensions/example.ts IS committed (PRD §16: "everything else is committed, including
# extensions/example.ts"). Do NOT add extensions/ to .gitignore. Do NOT add a /weave
# cleanup that would also affect the tracked file.

# GOTCHA: the sibling rule (extdir.resolveSiblingFromExe) wins iff extensions/ EXISTS as
# a directory — it does NOT require HasExtensionEntry. So merely creating
# extensions/example.ts (which mkdir's extensions/) is sufficient for rule 3 to fire and
# `--path` to return $PWD/extensions. No seeding/init step is needed for the repo itself.

# GOTCHA: this task runs IN PARALLEL with P1.M5.T1.S1 (CLI contract: --help, precedence,
# exclusivity, unknown-flag exit 2). M5.T1.S1 edits ONLY main.go + main_test.go error/
# help paths — it does NOT touch discovery/resolution/exampleExtensionTemplate. So the
# §13 Discovery/resolution/check criteria are unaffected by M5.T1.S1. If you add the
# optional guard test to main_test.go, add it in its OWN section header to avoid merge
# friction with M5.T1.S1's test block.
```

## Implementation Blueprint

### Data models and structure

None. This task writes a TypeScript asset and (optionally) one Go test function. It
consumes the existing `exampleExtensionTemplate` const (main.go L84-110) and the existing
`extensions/` sibling rule. No structs, no interfaces, no config.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE extensions/example.ts
  - WRITE the file with the EXACT §11 body (transcribed verbatim in "Implementation
    Patterns" below). Create the extensions/ directory implicitly (the write tool
    mkdir -p's parents).
  - CONTENT: leading JSDoc /** … */ (5 lines) + blank line + `import type { ExtensionAPI }
    from "@earendil-works/pi-coding-agent";` + blank line + `export default function
    (pi: ExtensionAPI) { … }` (the session_start notify + the weave-example registerCommand).
  - INVARIANT: byte-for-byte == exampleExtensionTemplate (main.go L84-110) == PRD §11.
    724 bytes, UTF-8, exactly one trailing newline, ZERO backticks.
  - VERIFY: `diff extensions/example.ts <(awk '/^const exampleExtensionTemplate = `/{f=1;sub(/^.*`/,"");print;next} f{if(/^`$/){exit}print}' main.go)`
    → empty (zero differences).
  - FILES TOUCHED: 1 NEW (extensions/example.ts).

Task 2: (OPTIONAL, recommended) ADD TestExampleAssetEqualsTemplate to main_test.go
  - APPEND a new test under a dedicated header
    `// --- example asset byte-equality (P1.M6.T1.S1) ---` (own section to avoid merge
    friction with the parallel P1.M5.T1.S1 test block).
  - BODY: read "extensions/example.ts" (cwd is the repo root under `go test .`) and assert
    string equality with exampleExtensionTemplate. Skip with t.Skip if the file is absent
    (so `go test ./...` still passes before the asset lands / in CI checkouts that exclude
    it). Body in "Implementation Patterns" below.
  - RUN: go test ./... -v -run TestExampleAssetEqualsTemplate
  - EXPECT: PASS (after Task 1). This locks the cross-task contract durably.
  - FILES TOUCHED: 1 (main_test.go, additive).

Task 3: BUILD + run the §13 acceptance suite (the verification deliverable)
  - PREP a clean environment FIRST (the #1 gotcha):
        unset weave_EXTENSIONS_DIR weave_CONFIG
        rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/weave/config.yaml"
    (or run under HOME=$(mktemp -d) XDG_CONFIG_HOME=$HOME/.config).
  - BUILD: `go build -o weave . && echo OK`
  - RUN the full §13 block (transcribed in Validation Loop §Level 3). Every check must
    print its OK line.
  - PI line: `pi --no-extensions -e "$(./weave example)" -p "briefly confirm the weave
    example extension is loaded and name one command it registered" 2>&1 | head` — assert
    exit 0 AND no "Failed to load extension" on stderr (the hard criterion). Optionally
    prove the contrast by temporarily loading a broken .ts (exit 1) — then REMOVE the
    broken file so only example.ts ships.
  - IF any criterion fails: per the item contract, FIX THE IMPLEMENTATION (not the PRD).
    Research confirms NONE fail in a clean env, so this is expected to be a no-op; a
    failure almost always means the clean-env preamble was skipped (see Known Gotchas).
  - FILES TOUCHED: 0 (verification only; the binary ./weave is gitignored).
```

### Implementation Patterns & Key Details

```typescript
// extensions/example.ts — EXACT content (== PRD §11 == main.go exampleExtensionTemplate).
// Copy verbatim. Do NOT reformat. The leading JSDoc is what `weave --list` prints as the
// description; the `import type` is erased by jiti so no npm setup is needed to load it.
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

```go
// main_test.go — OPTIONAL guard test (Task 2). Locks the cross-task byte-equality
// contract (main.go L77) into `go test ./...`. Own section header to avoid merge
// friction with the parallel P1.M5.T1.S1 test block.

// --- example asset byte-equality (P1.M6.T1.S1) ---
//
// The repo asset extensions/example.ts MUST equal the compiled-in
// exampleExtensionTemplate byte-for-byte (PRD §11 == main.go L77 contract): both are
// the one shipped example, and setupStore writes the constant into an empty store's
// example.ts, so any drift between them would mean `weave init` seeds a different
// example than the repo ships. This test makes that drift a build-time failure.
func TestExampleAssetEqualsTemplate(t *testing.T) {
	got, err := os.ReadFile("extensions/example.ts")
	if err != nil {
		t.Skipf("extensions/example.ts not present in this checkout: %v", err)
	}
	if string(got) != exampleExtensionTemplate {
		t.Errorf("extensions/example.ts drifts from exampleExtensionTemplate (PRD §11)\nwant %q\nhave %q",
			exampleExtensionTemplate, string(got))
	}
}
// NOTE: `go test .` runs with cwd = repo root (package main), so the relative path
// resolves. The t.Skip keeps `go test ./...` green in checkouts that exclude the asset.
// `os` is already imported by main_test.go (used for t.TempDir helpers elsewhere).
```

### Integration Points

```yaml
PRODUCES:
  - extensions/example.ts                 # the §11 repo asset (committed, NOT gitignored)
  - (optional) TestExampleAssetEqualsTemplate in main_test.go

CONSUMES (read-only, no change):
  - exampleExtensionTemplate (main.go L84-110)  # the byte-twin to diff against
  - extdir.Find() / resolveSiblingFromExe       # rule 3 fires once extensions/ exists
  - discover.Index() + resolve.Resolve()        # what --list / `weave example` exercise
  - check.Check()                               # what `weave check` exercises
  - pi (on PATH, v0.80.3)                       # the -e load test

NO CHANGES TO:
  - main.go source (exampleExtensionTemplate already correct; M5.T1.S1 owns the CLI edits).
  - any internal/* package.
  - go.mod / go.sum / .gitignore / PRD.md / tasks.json.
  - the set of shipped extensions (PRD §17: exactly ONE — do not add a second).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# The .ts file has no linter in this repo (no eslint/tsc configured; weave is a Go binary
# that does NOT typecheck extensions — that is pi's job at -e load time). Verify byte shape:
wc -c extensions/example.ts                 # expect 724
tail -c 1 extensions/example.ts | od -An -c # expect a single \n (one trailing newline)
grep -c '`' extensions/example.ts           # expect 0 (zero backticks)

# Byte-equality vs the compiled-in twin (the cross-task contract):
diff extensions/example.ts \
  <(awk '/^const exampleExtensionTemplate = `/{f=1;sub(/^.*`/,"");print;next} f{if(/^`$/){exit}print}' main.go) \
  && echo "byte-equal to exampleExtensionTemplate OK"

# If the optional guard test was added:
go build ./... ; go vet ./... ; gofmt -l main_test.go
go test -run TestExampleAssetEqualsTemplate -v

# Expected: 724 bytes; one trailing \n; zero backticks; diff empty; test PASS; gofmt empty.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./... -v
# Expected: all green. The extension asset itself has no unit tests (it is a TypeScript
# file consumed by pi, not by Go). The ONLY Go-side test touching it is the OPTIONAL
# TestExampleAssetEqualsTemplate (if added). discover/resolve/check tests already cover
# the BEHAVIOR (--list shows example, `weave example` resolves, check reports OK) using
# temp-dir fixtures — they are unaffected by the repo asset.
```

### Level 3: Integration Testing — the FULL §13 suite (the verification deliverable)

Run from the repo root. **FIRST** the clean-env preamble (the #1 gotcha — without it a
stray `~/.config/weave/config.yaml` makes `--path` resolve to the wrong place):

```bash
cd ~/projects/weave
unset weave_EXTENSIONS_DIR weave_CONFIG
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/weave/config.yaml"   # remove stray config from prior testing

# Build
go build -o weave . && echo OK
./weave --version                      # prints: weave dev

# Discovery + path  (sibling-of-binary rule — extensions/ now exists)
test "$(./weave --path)" = "$PWD/extensions" && echo "path=sibling OK"
./weave --list                          # shows the `example` extension (with JSDoc description)
test -f "$(./weave example)"            # resolves to a real file (…/extensions/example.ts)
test -f "$(./weave -f example)"         # -f on a single-file ext = the file itself

# Error contract: unknown tag prints nothing to stdout, exits 1
out=$(./weave nope 2>/dev/null); rc=$?
[ -z "$out" ] && [ "$rc" = "1" ] && echo "unknown-tag contract OK"

# Absolute-path contract (default)
case "$(./weave example)" in /*) echo "absolute OK";; *) echo "FAIL"; exit 1;; esac

# Validation
./weave check                           # exits 0, reports the example as OK

# End-to-end with pi (extension loads ONLY via -e, not auto-discovered).
# HARD criterion: exit 0, NO "Failed to load extension" on stderr.
pi --no-extensions -e "$(./weave example)" -p "briefly confirm the weave example extension is loaded and name one command it registered" 2>&1 | head
# Optional CONTRAST proof (then DELETE the broken file so only example.ts ships):
#   mkdir -p /tmp/edz-contrast && printf 'this is @@@ broken\n' > /tmp/edz-contrast/broken.ts
#   weave_EXTENSIONS_DIR=/tmp/edz-contrast pi --no-extensions -e /tmp/edz-contrast/broken.ts -p hi 2>&1 | head
#   # → exit 1, "Failed to load extension: ParseError"  (proves -e actually loads)
#   rm -rf /tmp/edz-contrast

# Dir + package styles resolve correctly (temp scaffolding)
rm -rf /tmp/edz-acc && mkdir -p /tmp/edz-acc/foo /tmp/edz-acc/bar/src
printf 'export default function () {}\n' > /tmp/edz-acc/foo/index.ts
printf '{"name":"@x/bar","pi":{"extensions":["./src/index.ts"]},"description":"d"}\n' > /tmp/edz-acc/bar/package.json
printf 'export default function () {}\n' > /tmp/edz-acc/bar/src/index.ts
weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave --list | grep -qw foo && weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave --list | grep -qw bar && echo "dir+package list OK"
test -d "$(weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave foo)"   && echo "foo→dir OK"
test -f "$(weave_EXTENSIONS_DIR=/tmp/edz-acc ./weave -f bar)" && echo "bar -f→file OK"
rm -rf /tmp/edz-acc

# Symlink install works (resolve-back-to-repo)
rm -rf /tmp/weave-bin && mkdir -p /tmp/weave-bin && ln -sf "$PWD/weave" /tmp/weave-bin/weave
[ "$(readlink -f "$(/tmp/weave-bin/weave example)")" = "$PWD/extensions/example.ts" ] && echo "symlink→repo OK"
weave_EXTENSIONS_DIR="$PWD/extensions" ./weave example >/dev/null && echo "env override OK"

# Config + first-run (§8) — isolated HOME so the stray-config gotcha cannot interfere
rm -rf /tmp/weave-iso && mkdir -p /tmp/weave-iso/home && cp ./weave /tmp/weave-iso/weave && cd /tmp/weave-iso
env -u weave_EXTENSIONS_DIR HOME=/tmp/weave-iso/home XDG_CONFIG_HOME=/tmp/weave-iso/home/.config \
  ./weave x 2>err; rc=$?
[ "$rc" = "1" ] && grep -q 'run `weave init`' err && echo "unconfigured-hint OK"
weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave init --store /tmp/weave-store
test -d /tmp/weave-store                                                     && echo "store created OK"
grep -q 'store: /tmp/weave-store' /tmp/weave-iso/cfg.yaml                    && echo "config written OK"
weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave --path | grep -q /tmp/weave-store      && echo "config wins OK"
weave_EXTENSIONS_DIR=/tmp/weave-store weave_CONFIG=/tmp/weave-iso/cfg.yaml ./weave --path 2>&1 | grep -q weave_EXTENSIONS_DIR && echo "env beats config OK"
cd - >/dev/null

# Expected: every check prints its OK line. (Verified — see research/acceptance_results.md.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Byte shape of the asset (no BOM, LF line endings, UTF-8 em-dashes if any):
file extensions/example.ts                       # expect: ASCII text (the body is pure ASCII)
head -c 3 extensions/example.ts | od -An -tx1   # expect: 2f 2a 2a  ("/**") — no UTF-8 BOM (ef bb bf)

# The JSDoc is the --list description (round-trip check):
desc=$(./weave --list 2>/dev/null | awk 'NR>1 && $1=="example"{f=1} f' | tr -s ' ')
echo "$desc" | grep -q "Reference example extension for weave" && echo "JSDoc→list OK"

# Exactly ONE extension ships (PRD §17 hard constraint):
[ "$(./weave --list 2>/dev/null | awk 'NR>1 && NF' | wc -l)" -ge 1 ] && echo ">=1 extension OK"
ls extensions/                                   # expect ONLY example.ts (no second extension)

# Expected: ASCII text, no BOM, JSDoc surfaces in --list, exactly one .ts under extensions/.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build -o weave .` clean; `./weave --version` prints `weave <something>`.
- [ ] `go test ./... -v` green (incl. `TestExampleAssetEqualsTemplate` if added).
- [ ] `gofmt -l main_test.go` empty (only if the optional test was added).

### Feature Validation (the §13 gate)

- [ ] `extensions/example.ts` exists, 724 bytes, byte-equal to `exampleExtensionTemplate`.
- [ ] `git status` shows it as a new tracked file (NOT gitignored).
- [ ] `test "$(./weave --path)" = "$PWD/extensions"` (clean env).
- [ ] `--list` shows `example` with the JSDoc description.
- [ ] `weave example` / `weave -f example` resolve to the real file.
- [ ] Unknown tag: empty stdout, exit 1; resolved path is absolute.
- [ ] `weave check` exit 0, example OK.
- [ ] Dir + package styles resolve; symlink → repo; config/first-run all OK.
- [ ] `pi --no-extensions -e "$(./weave example)" -p …` exit 0, no load error.

### Code Quality Validation

- [ ] `extensions/example.ts` is byte-identical to PRD §11 (no reformatting).
- [ ] `import type` preserved (jiti-erased; no npm setup needed to load).
- [ ] Exactly ONE example ships (PRD §17); no second extension added.
- [ ] The optional guard test (if added) is in its own section header (no merge friction
      with P1.M5.T1.S1) and `t.Skip`s cleanly when the asset is absent.

### Documentation & Deployment

- [ ] The example is self-documenting via its JSDoc (PRD §11 / item DOCS: "none — example
      extension is self-documenting via its JSDoc"). README §5/§6 is a separate Mode B task.
- [ ] The clean-env gotcha is recorded (so a future release engineer does not hit a false
      `--path` failure).

---

## Anti-Patterns to Avoid

- ❌ Don't reformat, "tidy," or prettify `extensions/example.ts` — it must equal §11 and the
  compiled-in constant byte-for-byte. Even a trailing-blank-line change breaks the diff guard
  and the `--list` description alignment.
- ❌ Don't drop the `type` keyword from `import type { ExtensionAPI }` — without it pi would
  try to resolve `@earendil-works/pi-coding-agent` at load time and fail with no node_modules.
- ❌ Don't run the §13 *Discovery* block without the clean-env preamble — a stray
  `~/.config/weave/config.yaml` (rules 1-2 beat rule 3) will make `--path` resolve to the
  wrong place and falsely look like an implementation bug. Clean FIRST.
- ❌ Don't treat the pi model's failure to *name* the `weave-example` command as a defect —
  ad-hoc `-e` commands are not auto-surfaced into the model's context. The criterion is
  "does not error" (exit 0, no load error), proven by the broken-extension contrast.
- ❌ Don't ship a second example, a README under `extensions/`, or any `package.json`/index
  scaffolding — PRD §17 forbids >1 example and the repo is a loader, not a library.
- ❌ Don't add `extensions/` to `.gitignore` — the asset MUST be committed (PRD §16).
- ❌ Don't edit `exampleExtensionTemplate` in main.go to "match" a hand-edited file — the
  constant is the byte source of truth (== §11); edit the FILE to match the constant, never
  the reverse, and never both independently.
- ❌ Don't add a go:embed for the example — PRD §17 + the main.go L74-78 comment deliberately
  chose a string constant over embed (and this task is about the on-disk ASSET, not how init
  seeds a store).
