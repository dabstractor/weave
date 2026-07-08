# Test Impact Scan — M5.T1.S1 run() precedence ladder

Scanned the **entire** `main_test.go` (1508 lines, 78 `Test*` funcs, 37 `run(...)` call sites).
Method: enumerated every `run([]string{...})` argv combo, cross-checked each against the 7-step
new precedence contract, and verified parseArgs-level tests never reach `run()`.

## TL;DR — only ONE existing test breaks

**`TestRunNoArgsIsNoOp` (line 554) is the only test that will FAIL / must change.**

Everything else either still passes under the new ladder, or it is an UNTESTED GAP
(new behavior with no existing `run()` coverage — see "Gaps" below). No `run()`-level test
currently asserts non-exit-2 codes for unknown flags or mutually-exclusive combos, because
**no such `run()` call exists in the file.**

---

## 1. BREAKING test — must UPDATE (the one known case)

### `TestRunNoArgsIsNoOp` — `main_test.go:554-563` → UPDATE (flip exit code + add stderr usage)

```go
554: func TestRunNoArgsIsNoOp(t *testing.T) {
555: 	var out, errStore bytes.Buffer
556: 	code := run(nil, &out, &errStore)
557: 	if code != 0 {
558: 		t.Errorf("run(nil): code=%d; want 0 (no-args → usage lands in M5.T1.S1)", code)
559: 	}
560: 	if out.Len() != 0 || errStore.Len() != 0 {
561: 		t.Errorf("run(nil) produced output; want none (no-op): stdout=%q stderr=%q", out.String(), errStore.String())
562: 	}
563: }
```
*(line numbers are exact; note `errStore` is the local stderr buffer name in this func, the
field is `errOut` elsewhere)*

**Why it breaks:** new contract step (7) — NO recognized mode → print usage to **STDERR**,
**exit 1**. Current code returns `0` and prints nothing. This is the final-return / no-args
fall-through being changed.

**Required change:**
- Line 557-559 assertion: flip `code != 0` → `code != 1`; update want message to `want 1 (no-args → usage on stderr, exit 1)`.
- Line 560-562 assertion: split into two checks —
  - `out.Len() != 0` → **stdout still empty** (usage goes to stderr, not stdout).
  - `errStore.String()` must now **contain usage** (e.g. `strings.Contains(errStore.String(), "usage")` or check for `Usage:\nweave` / `Usage: weave` depending on the implemented usage string). It is no longer empty.
- **Rename recommended** (optional, cosmetic): `TestRunNoArgsIsNoOp` → `TestRunNoArgsPrintsUsageExit1`. The current name ("IsNoOp") becomes a lie under the new contract.

**Verdict: UPDATE (not delete).** The test is still valuable; it just inverts from exit-0/no-op to exit-1/usage-on-stderr.

---

## 2. Tests that STILL PASS under the new ladder (verified, no change needed)

All of these omit `--help`, omit unknown dashed flags, and omit mutually-exclusive mode combos,
so the new steps (1) help, (3) unknownFlag, (4) exclusivityError never fire for them.

### Version dispatch & precedence — `run()` level (all still pass)
New ladder puts **help (1) → version (2)**. None of these pass `--help`, so version still wins
over path/tag/all (modes are step 6). Behavior unchanged.

| Func | Line | argv | Why unaffected |
|---|---|---|---|
| `TestRunVersionPrintsWeaveVersion` | 415 | `{"--version"}` | no help flag → version step (2) → exit 0, `weave dev\n` on stdout, empty stderr. ✅ |
| `TestRunVersionShortFlag` | 430 | `{"-v"}` | same as above. ✅ |
| `TestRunVersionPrecedenceOverPath` | 531 | `{"--path", "--version"}` | no help → version (2) beats path (6) → exit 0. ✅ |
| `TestRunVersionPrecedenceOverTag` | 892 | `{"example", "--version"}` | no help → version (2) beats tag mode (6). ✅ |
| `TestRunVersionPrecedenceOverAll` | 1165 | `{"--all", "--version"}` | no help → version (2) beats --all (6). ✅ |

> ⚠️ **Naming/documentation staleness (NOT a break):** the three `*VersionPrecedenceOver*`
> names + their `// --version takes precedence...` comments now overstate — under the new
> ladder **help** precedes version. The test *bodies* still pass, so no code change is
> required, but the names/comments are now imprecise. Optional cleanup only.

### Mode-dispatch tests — all unaffected
Every `run()` call below has exactly one mode (or a mode + modifiers only), no help/version/
unknown/exclusive flag, so it lands cleanly on step (6) mode dispatch unchanged:

- Path: `TestRunPathSuccess` 449, `TestRunPathShortFlag` 467, `TestRunPathReportsSourceLabel` 487, `TestRunPathFailureErrNotFound` 508
- List: `TestRunListSuccess` 570, `TestRunListShortFlag` 596, `TestRunListNoExtensionsExit1` 614, `TestRunListUnresolvableExit1` 631, `TestRunListNoColorFlagSuppressesANSI` 650, `TestRunListColorWhenTTY` 668
- Tags: `TestRunTagSingleResolvesToPath` 742, `TestRunTagMultipleInInputOrder` 762, `TestRunTagAtomicityUnknownPrintsNothing` 785, `TestRunTagAllFailMultipleErrorLines` 802, `TestRunTagDuplicateArgResolvesTwice` 820, `TestRunTagAmbiguousListsCandidates` 837, `TestRunTagUnresolvable` 861, `TestRunTagPathIsAbsolute` 878, `TestRunTagFileOnSingleFileIsNoOp` 911, `TestRunTagFileOnDirExtPrintsIndexTS` 932, `TestRunTagFileOnPkgExtPrintsPiExtensionsEntry` 954, `TestRunTagRelativePrintsRelativePath` 974, `TestRunTagFileRelativeCombine` 992, `TestRunTagFileAtomicity` 1007
- All: `TestRunAllPrintsAllSorted` 1024, `TestRunAllShortFlag` 1049, `TestRunAllFilePrintsAllEntryFiles` 1067, `TestRunAllRelativePrintsAllRelative` 1111, `TestRunAllEmptyStoreExit0` 1134, `TestRunAllUnresolvable` 1148
- Search: `TestRunSearchMatch` 1181, `TestRunSearchShortFlag` 1205, `TestRunSearchNoMatchExit1` 1219, `TestRunSearchEmptyQueryMatchesAll` 1236, `TestRunSearchUnresolvableExit1` 1253
- Check: `TestRunCheckClean` 1272, `TestRunCheckEmptyStoreClean` 1294, `TestRunCheckWithErrorExit1` 1310, `TestRunCheckWithWarningExit0` 1343, `TestRunCheckUnresolvableExit1` 1365

> Note on tag-mode `run([]string{"nope1","nope2"})` (806) and `{"example","nope"}` (789, 1011):
> these are **non-dashed positionals** captured as `<tag>`s, NOT dashed unknowns. They exercise
> tag-resolution atomicity (§6.4 → exit 1). They do **not** touch the new `unknownFlag != "" → exit 2`
> step (that step only applies to dashed args). Unaffected. ✅

> Note on `{"--list","--no-color"}` (657), `{"--all","--file"}` (1095), `{"--all","--relative"}`
> (1115), `{"-f","--relative","reddit-poster"}` (996): `--no-color`, `--file`, `--relative` are
> **modifiers**, not modes — no exclusivity conflict. Unaffected. ✅

---

## 3. parseArgs-level tests — UNAFFECTED (confirmed parser-only)

Every `TestParseArgs*` func calls `parseArgs(...)` directly and **never** calls `run(...)`.
They assert how the parser *captures* flags (including `c.unknownFlag`, `c.help`-equivalent if
present, `c.init`), not how `run()` *dispatches* on them. Dispatch changes in `run()` cannot
break them.

Verified parser-only (all call `parseArgs`, none call `run`):
`TestParseArgsEmpty` 69, `*VersionLong` 76, `*VersionShort` 83, `*PathLong` 90, `*PathShort` 97,
`*AnyOrderBothForms` 105, `*UnknownFlagCaptured` 117, `*ListLong` 131, `*ListShort` 138,
`*NoColor` 145, `*CapturesTagsInOrder` 155, `*DashedUnknownNotATag` 164, `*TagsAndFlagsInterleave` 175,
`*FileLong` 185, `*FileShort` 192, `*RelativeLong` 200, `*AllLong` 208, `*AllShort` 215,
`*ModifiersInterleave` 223, `*SearchLong` 233, `*SearchShort` 244, `*SearchNoValueStaysInactive` 252,
`*SearchConsumesOneValue` 260, `*SearchEqualsForm` 271, `*ShortBundleBools` 281,
`*ShortBundleSearchEmbedded` 289, `*ShortBundleSearchNextToken` 297, `*ShortBundleUnknownCharRejectsAll` 309,
`*CheckSubcommand` 322, `*CheckAfterFlag` 333, `*InitSubcommand` 347, `*InitPositionalDir` 361,
`*InitStoreLongForm` 375, `*InitStoreEqualsForm` 389, `*StoreWithoutInitToken` 400.

In particular these unknown-flag cases are **parser** tests and remain green:
- `TestParseArgsUnknownFlagCaptured` 117 — asserts `c.unknownFlag == "--frobnicate"` (parser capture). No run() call.
- `TestParseArgsDashedUnknownNotATag` 164 — asserts first-of-two unknowns captured. No run() call.
- `TestParseArgsShortBundleUnknownCharRejectsAll` 309 — asserts `-vz` rejected wholesale, `c.unknownFlag == "-vz"`, `version == false`. No run() call.

These will NOT break, but they also do NOT cover the new step (3) `unknownFlag != "" → exit 2`
*dispatch* behavior. That dispatch is untested (see Gaps).

---

## 4. GAPS — new-contract behavior with NO existing `run()` coverage (add, don't change)

Exhaustive scan of all 31 distinct `run([]string{...})` argv combos confirms **none** exercise
the new top-of-ladder branches. These new behaviors are currently UNTESTED at the `run()` level:

1. **`--help` / `-h` dispatch (step 1)** — no `run([]string{"--help"})` or `run([]string{"-h"})`
   exists. Help is currently a complete no-op (not dispatched, exit 0, empty output). New contract:
   usage → **stdout**, exit 0. **Need a new test** asserting stdout contains usage, exit 0, stderr empty.
2. **`--help` precedence over `--version` (step 1 beats step 2)** — no `run([]string{"--help","--version"})`
   exists. Need a new test asserting help wins (usage on stdout, NOT the version line).
3. **unknown dashed flag → exit 2 (step 3)** — no `run()` call passes `--bogus`/`-z`-bundle/`--frobnicate`.
   The parser captures it; the dispatch (`unknown flag X` → stderr, exit 2) is untested. Need new tests.
4. **mutually-exclusive combo → exit 2 (step 4)** — no `run()` call passes any exclusive pair
   (`--list --search`, tag+`--list`, check+tag, init+tag, init+`--list`). Need new tests for `exclusivityError(c)`.
5. **`init` dispatch via `run()` (step 5)** — all `init` tests are parser-only (`TestParseArgsInit*` 347-415).
   No `run([]string{"init", ...})` exists. Need a new test if init dispatch lands in this slice.

---

## 5. Non-run() helpers — unaffected (no dispatch)
`TestChooseStore*` (1414-1484) test store selection; `TestReadPromptFormatsDefAndTrims` (1484)
tests prompt formatting. Neither touches `run()` precedence. Unaffected.

---

## Start Here
**`main_test.go:554` (`TestRunNoArgsIsNoOp`)** — the single existing test that must be edited:
flip exit code 0→1, assert usage lands on stderr, keep stdout empty. Then author the 5 new
gap tests in section 4 above.