# Codebase State — weave

## Current repo state (greenfield)

The `~/projects/weave` repo is greenfield. It contains only:
- `PRD.md` — the complete specification (read-only)
- `.git/` — initialized git repo
- `plan/` — this plan directory

No Go code, no `go.mod`, no source files exist yet. Everything is built from scratch.

## Go toolchain

```bash
$ go version
go1.26.4 linux/amd64   # (verify at implementation time)
```

Module path: `github.com/dabstractor/weave`. Minimum Go: latest two stable releases
(set `go 1.25` in `go.mod` directive; the toolchain is newer so this is safe).

## Zero third-party dependencies

Unlike skilldozer (which depends on `gopkg.in/yaml.v3` for SKILL.md frontmatter),
weave has **zero** third-party Go dependencies. Metadata extraction uses:
- `encoding/json` (stdlib) for `package.json`
- A ~20-line hand-rolled string scanner for leading JSDoc comment extraction
- A ~10-line hand-rolled YAML reader for the one-key config file (`store: <path>`)

This means: **no `go.sum`**, no `go get`, no supply chain. `go build` works from a
clean clone with just the Go toolchain.

## Reference project: skilldozer (`~/projects/skilldozer`)

The PRD is explicitly "a near-clone of the Skilldozer PRD." The skilldozer codebase
is the primary architectural reference. Its structure:

```
skilldozer/
├── main.go                    # entrypoint: arg parsing (hand-rolled switch), dispatch
├── internal/
│   ├── skillsdir/skillsdir.go # locate skills/ dir (§8 priority order) — PORT NEARLY VERBATIM
│   ├── config/config.go       # settings file (yaml.v3) — REWRITE as hand-rolled YAML
│   ├── discover/
│   │   ├── discover.go        # ParseFrontmatter (YAML) — REPLACE with JSDoc+JSON
│   │   ├── skill.go           # Skill struct + BuildSkill — ADAPT to Extension
│   │   └── index.go           # WalkDir scan + sort — REWRITE classify-then-descend
│   ├── resolve/resolve.go     # tag → skill (§7.2 precedence) — PORT NEARLY VERBATIM
│   ├── search/search.go       # substring search — PORT NEARLY VERBATIM
│   ├── check/check.go         # validation report — ADAPT validation rules
│   └── ui/ui.go               # table formatting — PORT NEARLY VERBATIM
├── install.sh                 # build + symlink into PATH — PORT with renames
├── completions/               # bash/zsh/fish — PORT with renames
├── README.md                  # user docs — REWRITE for extensions
├── LICENSE                    # MIT
├── go.mod / go.sum            # module github.com/dabstractor/skilldozer + yaml.v3
└── skills/example/SKILL.md    # the one example skill
```

## Reference project: mcpeepants (`~/projects/mcpeepants`)

Secondary reference for CLI style (banner, shell detection, usage format). mcpeepants is
a bash script (`get-server-config.sh`), not Go, so it informs style/UX, not architecture.

## Key deltas: skilldozer → weave

| Aspect | skilldozer | weave |
|--------|-----------|-------|
| Unit of discovery | Directory containing `SKILL.md` | File (`*.ts`/`*.js`, not `index.*`) OR resolvable dir (`index.*` or `package.json`+`pi.extensions`) |
| Walk rule | Every dir with `SKILL.md` is a skill | **Classify-then-descend**: recognize dir/package extensions and DON'T recurse into them; only recurse into plain (non-extension) dirs |
| Metadata source | YAML frontmatter in `SKILL.md` | `package.json` (JSON) + leading JSDoc comment in entry file |
| Metadata dependency | `gopkg.in/yaml.v3` | None (stdlib `encoding/json` + hand-rolled JSDoc scanner) |
| Config file parser | `yaml.v3` Marshal/Unmarshal | Hand-rolled ~10-line YAML reader for `store: <value>` |
| Default output | Skill directory path | Resolvable path (file for files, dir for dirs) |
| `--file` output | `SKILL.md` path | Entry `.ts`/`.js` file path (the file pi loads) |
| Canonical tag | Dir path relative to skills/ (e.g. `writing/reddit`) | Path relative to extensions/, `.ts`/`.js` stripped for single files (e.g. `gate`, `writing/reddit-poster`) |
| Env var prefix | `SKILLDOZER_` | `weave_` (lowercase: `weave_EXTENSIONS_DIR`, `weave_CONFIG`) |
| Config dir | `$XDG_CONFIG_HOME/skilldozer/config.yaml` | `$XDG_CONFIG_HOME/weave/config.yaml` |
| Example asset | `skills/example/SKILL.md` | `extensions/example.ts` (single-file) |
| Store sibling dir | `skills/` | `extensions/` |
| Extension struct fields | `Dir`, `SourceFile` (= Dir + "/SKILL.md") | `Path` (resolvable), `EntryFile` (what pi loads), `Kind` (file/dir/package) |
| `check` rules | Frontmatter validity, name rules, dup names | Dup relTags, missing entryFile, unparseable package.json, deps-not-installed WARN, no-description WARN, empty-category-dir WARN |
