# External Dependencies — weave

## Zero third-party Go dependencies

weave has **zero** third-party Go dependencies. This is strictly simpler than skilldozer,
which required `gopkg.in/yaml.v3` for SKILL.md YAML frontmatter parsing.

### Why zero deps is achievable

| Need | skilldozer solution | weave solution |
|------|---------------------|----------------|
| Config file (`store:` key) | `yaml.v3` Marshal/Unmarshal | Hand-rolled ~10-line line scanner: split lines, regex `^store:\s*(.+)$` |
| Extension metadata | `yaml.v3` for YAML frontmatter | `encoding/json` (stdlib) for `package.json` |
| Description extraction | YAML `description:` field | Hand-rolled ~20-line JSDoc `/** ... */` scanner |
| Tag normalization | `filepath.ToSlash` (stdlib) | Same |
| Table formatting | `fmt`, `strings`, `unicode/utf8` (stdlib) | Same |
| ANSI color | String literals `\x1b[...m` (stdlib) | Same |

### No `go.sum`

Because there are no dependencies, there is no `go.sum` file. `go.mod` has only the
`module` and `go` directives. A clean clone builds with just the Go toolchain.

## stdlib packages used (all in Go standard library)

| Package | Used for |
|---------|----------|
| `os` | Env vars, file I/O, `os.Executable()`, `os.Stdin`/`os.Stdout`/`os.Stderr`, `os.Stat` |
| `path/filepath` | Path manipulation, `WalkDir`, `EvalSymlinks`, `Abs`, `Rel`, `ToSlash`, `Dir`, `Join`, `Clean` |
| `encoding/json` | `package.json` parsing into struct |
| `fmt` | Output formatting, `Errorf` |
| `strings` | String manipulation, `Contains`, `ToLowerCase`, `TrimSpace`, `Split`, `Join`, `HasPrefix` |
| `sort` | Sorting `[]Extension` by `RelTag`, sorting candidates |
| `io` | `io.Writer`/`io.Reader` interfaces, `io.EOF` |
| `io/fs` | `fs.DirEntry` for `WalkDir` callback |
| `bufio` | `bufio.Reader` for interactive prompt in `weave init` |
| `unicode/utf8` | `RuneCountInString` for display-width table padding |
| `errors` | `errors.New`, sentinel errors |
| `regexp` | Config line parsing (optional; simple string ops suffice) |

## Build-time dependency: Go toolchain

Only the Go compiler is needed at build time. Minimum Go: latest two stable releases
(set `go 1.25` in the `go.mod` directive). Verified toolchain: `go1.26.4 linux/amd64`.

## No runtime dependencies

weave is a statically-linked Go binary. At run time it needs:
- No Node.js, Python, jiti, or any interpreter.
- No shared libraries (CGO disabled by default).
- Only read access to the filesystem (extensions store, config file).

## Optional build-time: git (for version stamping)

`install.sh` uses `git describe --tags --always` to stamp the version:
```bash
go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)"
```
If git is absent or the directory is not a git repo, the fallback `dev` is used. This is
a soft dependency, not a hard requirement.
