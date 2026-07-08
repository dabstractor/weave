# README.md section-by-section review (post-fix)

Confirms exactly what changes, what stays, and where each edit lands.
All line numbers are against the current README.md (the pre-fix version,
11,865 bytes).

## The two bug fixes with user-facing README impact

### Issue 6 (P1.M2.T2.S1, COMPLETE) — primary addition
**Behavior:** `discover.Index` now skips `node_modules/`, `.git/`, and any
file/dir whose base name starts with `.` (hidden entries) during the walk.
- DIRS in `skipDirs` or hidden → `filepath.SkipDir` (prune the subtree).
- Hidden FILES → `return nil` (skip the one file; NOT SkipDir, which would
  prune siblings because `.` sorts first lexically).

**README gap:** The "How extensions are organized" section (lines ~204-233)
does NOT currently mention ANY skip rules. A reader has no way to know that
`node_modules/`, `.git/`, and `.foo.ts` are excluded. This is the primary
documentation addition.

**Exact current text** (lines 204-216):
```
## How extensions are organized

Extensions live in the `extensions/` directory at the store root. An extension
is one of three kinds:

- a single **file**: a `*.ts` or `*.js` file whose base name is not
  `index.ts` or `index.js`
- a **directory**: a directory that directly contains `index.ts` or `index.js`
- a **package**: a directory with a `package.json` whose `pi.extensions` array
  names at least one existing entry
```

**Insertion point:** After the three-kinds bullet list (after the "names at
least one existing entry" line, ~line 209) AND before the canonical-tag
paragraph. A short bullet or 1-2 sentence note fits naturally here — it
describes what the walk ignores, which belongs right after what the walk
recognizes.

### Issue 4 (P1.M2.T3.S1, COMPLETE) — optional clarification
**Behavior:** `discover.Index` now resolves the walk root via
`filepath.EvalSymlinks` before walking. A symlinked `weave_EXTENSIONS_DIR`
(now resolved) yields the real catalog instead of an empty one.

**README current state:** The env-var rule (#1, lines 300-303) does NOT
mention symlinks at all — it just says "if set and an existing dir, use it."
Before the fix, a symlinked dir was "accepted by --path but yields an empty
catalog" (the bug). After the fix, it works correctly.

**Is a doc note needed?** The item CONTRACT says: "the description is still
accurate, optionally note that symlinks are followed." The existing description
("if set and an existing dir, use it") is now ACCURATE (the symlink IS an
existing dir, and it now works). A note is OPTIONAL but helpful — a reader who
symlinks their store (e.g. `ln -s /mnt/shared/extensions ~/extensions`) should
know it works. A short parenthetical is the right weight: "symlinks are
followed" or "symlinked paths are resolved." Keep it to one clause.

## What does NOT change (confirm, do not edit)

### Issue 1 (P1.M1.T1.S1, COMPLETE) — package kind description
**Current README (line 208-209):** "a **package**: a directory with a
`package.json` whose `pi.extensions` array names at least one existing entry."

**Post-fix classifyDir:** Now iterates ALL `pi.extensions` entries and picks
the first EXISTING one (was: only checked `entries[0]`). The README's "names
at least one existing entry" was ALWAYS correct — the code was wrong, not the
docs. **No change needed.** The item CONTRACT confirms this explicitly.

### Issues 2, 3, 5 — internal, no README surface
- Issue 2 (JSDoc `/**/`): internal to `ExtractJSDoc`. No user-facing README
  claim to update. The "Adding an extension" section's JSDoc example
  (`/** ... */`) is unaffected.
- Issue 3 (root-level `index.ts`): internal to `Index`'s WalkDir root guard.
  No README claim about root-level `index.ts` exists.
- Issue 5 (check all-missing pi.extensions → ERROR): internal to `check`. The
  README's `weave check` example (lines 286-293) shows a clean store; it does
  not enumerate every ERROR/WARN case. No change needed.

The item CONTRACT is explicit: "Do NOT add documentation for Bugs 2, 3, 5."

## Other README sections — verify consistency (no edits expected)

### "How weave finds the store" (lines ~296-330)
- Rule #3 (sibling of binary) ALREADY says "symlink-aware: `os.Executable()`
  plus `EvalSymlinks()`" — accurate, no change.
- Rule #4 (walk up from cwd) — unaffected.
- Rule #5 (unconfigured) — unaffected.
- The `--path` note (lines ~325-330) says `--path` shows the original
  (possibly symlinked) path. This is STILL accurate after Issue 4: `--path`
  uses `extdir.Find()` directly (NOT `discover.Index`), so it shows the
  symlink path; `Index` resolves it internally. **No change needed** — but
  note the subtle distinction: `--path` shows the symlink, `--list`/`<tag>`
  resolve it. The README does not need to spell this out; the existing
  `--path` description is correct.

### "Adding an extension" (lines ~235-285)
- JSDoc example unaffected by Issue 2 (the fix handles the degenerate empty
  case; normal `/** ... */` blocks were never broken).
- `package.json` example unaffected by Issue 1.
- No skip rules belong here (they belong in "How extensions are organized").

### "Constraints" (lines ~332-end)
- Line 347: "a `node_modules` package" appears in the "never auto-discovered
  by pi" list. This is about pi's discovery, NOT weave's. It is accurate and
  unrelated to Issue 6 (Issue 6 is about weave skipping node_modules WITHIN
  its own store). **No change needed** — but be careful not to confuse the
  two: the "Constraints" `node_modules` is about where the STORE must not
  live; the new "How extensions are organized" note is about what weave
  skips INSIDE the store.

## Summary of edits

1. **PRIMARY** — "How extensions are organized" (after the three-kinds bullet,
   ~line 209): add a 1-2 sentence note that `node_modules/`, `.git/`, and
   hidden entries (names starting with `.`) are skipped during discovery.
2. **OPTIONAL** — "How weave finds the store", rule #1 (env var, ~line 302):
   add a short parenthetical that symlinked paths are followed/resolved.

Everything else is a confirm-no-change.
