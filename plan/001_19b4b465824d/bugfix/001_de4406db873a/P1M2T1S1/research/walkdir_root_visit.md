# WalkDir root-visit behavior (verified empirically)

Goal: prove that `filepath.WalkDir(root, cb)` passes back the EXACT `root` string
it was given as the callback's `path` argument for the root visit — so the
`if path == root` guard in `discover.Index` fires exactly once (for the root)
and never for a subdirectory.

This is the load-bearing assumption behind the P1.M2.T1.S1 fix (Issue 3:
root-level `index.ts` collapsing the store). If `path` were cleaned, rewritten,
or differed from `root` for the root visit, the guard would either never fire
(bug persists) or fire for the wrong directory.

## The test program

```go
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	// Reproduce the bug's store shape: index.ts at root + a subdir extension.
	root, _ := os.MkdirTemp("", "weave-root-skip")
	defer os.RemoveAll(root)
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "index.ts"), []byte("// root idx\n"), 0o644)
	os.WriteFile(filepath.Join(root, "sub", "x.ts"), []byte("// sub\n"), 0o644)

	// Simulate what Index does: filepath.Abs then WalkDir.
	absRoot, _ := filepath.Abs(root)
	fmt.Printf("absRoot = %q\n", absRoot)

	rootVisits := 0
	exactMatch := false
	filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			fmt.Printf("DIR visit: path=%q  path==absRoot? %v\n", path, path == absRoot)
			if path == absRoot {
				exactMatch = true
			}
		}
		if d.IsDir() && path == absRoot {
			rootVisits++
		}
		return nil
	})
	fmt.Printf("\nroot visited %d time(s); exact match (path == root) ever true? %v\n",
		rootVisits, exactMatch)
}
```

## Verbatim output (go1.25, Linux)

```
absRoot = "/tmp/weave-root-skip809361331"
DIR visit: path="/tmp/weave-root-skip809361331"  path==absRoot? true
DIR visit: path="/tmp/weave-root-skip809361331/sub"  path==absRoot? false

root visited 1 time(s); exact match (path == root) ever true? true
```

## Findings

1. **The root is visited exactly once**, as a directory, and it is the FIRST
   directory visit. This matches the `filepath.WalkDir` documentation: "The
   first path is always the root." (Note: the doc says WalkDir visits root
   even if the callback returns SkipDir at root — no, actually, returning
   SkipDir at root is the BUG, it prunes the whole tree. The root IS visited.)

2. **`path == root` is `true` for the root visit and `false` for every
   subdirectory.** The string passed back as `path` for the root is byte-
   identical to the `root` argument WalkDir was invoked with (after
   `filepath.Abs` cleaned it). WalkDir does NOT re-clean, rewrite, or resolve
   the root path before handing it to the callback.

3. **The fix's guard is sound and complete.** `if path == root { return nil }`
   placed at the top of `if d.IsDir()` intercepts exactly the single root
   visit and forces descent (`return nil`), preventing `classifyDir(root, root)`
   from ever running. For every other directory, the guard is `false` and the
   existing classifyDir logic runs unchanged.

## Robustness against the planned P1.M2.T3.S1 (symlink-root) fix

The fix_design §Bug 4 plans to resolve the root symlink BEFORE the walk:

```go
root, err := filepath.Abs(extensionsDir)
if resolved, err := filepath.EvalSymlinks(root); err == nil {
    root = resolved
}
// ... filepath.WalkDir(root, ...)
```

Because the P1.M2.T3.S1 fix edits the `root` variable ABOVE the `WalkDir`
invocation, WalkDir is then called with the RESOLVED root. The finding above
("path == root for the root visit") applies to whatever string WalkDir is
given — so `path == root` continues to hold for the root visit after the
symlink fix lands. The P1.M2.T1.S1 guard does NOT need modification.

CRITICAL: this invariant holds ONLY if the guard compares the raw `path` and
`root` strings. If the guard did `filepath.Clean(path) == root` or
`filepath.EvalSymlinks(path) == root`, it would re-introduce an asymmetry
(`path` would be the resolved value only if P1.M2.T3.S1 had landed; before
that, `path` would be the unresolved symlink while `root` might be resolved —
a mismatch). Comparing raw strings — both sourced from the same WalkDir
invocation — is correct today and robust to the planned `root` rewrite.
