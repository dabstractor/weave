# pi Extension Facts — Verified from Source

All claims verified against pi source at:
`~/.pi/agent/npm/node_modules/@mariozechner/.pi-coding-agent-e74iH7Md/dist/`

## 1. Extension definition

An extension is a TypeScript (or JavaScript) module exporting a default factory function
`(pi: ExtensionAPI) => void | Promise<void>`. Loaded via [jiti](https://github.com/unjs/jiti),
so `.ts` works without compilation.

**Verified**: `loader.js` line ~302, `loadExtension()` calls `loadExtensionModule(resolvedPath)`
which uses jiti to import the module and extract the default export.

## 2. `-e` / `--extension` flag

```
--extension, -e <path>    Load an extension file (can be used multiple times)
```

**Verified** from `pi --help` output. Despite the word "file", `-e` accepts directories too.

`loadExtension(extensionPath, cwd, ...)` calls `resolvePath(extensionPath, cwd)` which:
- Expands `~/` prefixes.
- Makes relative paths absolute against cwd.
Then passes to `loadExtensionModule(resolvedPath)`.

## 3. `--no-extensions` / `-ne` flag

```
--no-extensions, -ne    Disable extension discovery (explicit -e paths still work)
```

**Verified** from `resource-loader.js`:
```js
const extensionPaths = this.noExtensions
    ? cliEnabledExtensions
    : this.mergePaths(cliEnabledExtensions, enabledExtensions);
```

Where `cliEnabledExtensions` = paths from `-e` flags, `enabledExtensions` = auto-discovered.
With `--no-extensions`, only `-e` paths are loaded. This is the exact mechanism that makes
`pi --no-extensions -e "$(weave <tag>)"` provably load ONLY the resolved extension.

## 4. Path resolution rules (`resolveExtensionEntries`)

**Verified** from `loader.js`:

```js
function resolveExtensionEntries(dir) {
    // 1. Check package.json with "pi" field first
    const packageJsonPath = path.join(dir, "package.json");
    if (fs.existsSync(packageJsonPath)) {
        const manifest = readPiManifest(packageJsonPath);
        if (manifest?.extensions?.length) {
            const entries = [];
            for (const extPath of manifest.extensions) {
                const resolvedExtPath = path.resolve(dir, extPath);
                if (fs.existsSync(resolvedExtPath)) {
                    entries.push(resolvedExtPath);
                }
            }
            if (entries.length > 0) return entries;
        }
    }
    // 2. Check for index.ts or index.js
    const indexTs = path.join(dir, "index.ts");
    const indexJs = path.join(dir, "index.js");
    if (fs.existsSync(indexTs)) return [indexTs];
    if (fs.existsSync(indexJs)) return [indexJs];
    return null;
}
```

Key observations:
- `package.json` with `pi.extensions` is checked FIRST (before index.ts).
- Only entries that actually EXIST on disk are included.
- `index.ts` takes precedence over `index.js`.

## 5. Directory discovery (`discoverExtensionsInDir`)

**Verified** from `loader.js`:

```js
function discoverExtensionsInDir(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
        const entryPath = path.join(dir, entry.name);
        // 1. Direct files: *.ts or *.js
        if ((entry.isFile() || entry.isSymbolicLink()) && isExtensionFile(entry.name)) {
            discovered.push(entryPath);
            continue;
        }
        // 2 & 3. Subdirectories
        if (entry.isDirectory() || entry.isSymbolicLink()) {
            const entries = resolveExtensionEntries(entryPath);
            if (entries) discovered.push(...entries);
        }
    }
}
```

```js
function isExtensionFile(name) {
    return name.endsWith(".ts") || name.endsWith(".js");
}
```

Key observation: pi's own discovery is **one level deep**. It does NOT recurse into
subdirectories of subdirectories. weave's catalog walk is deeper (recursive into plain
dirs) but emits paths that pi's resolver consumes identically (via `-e <dir>` or `-e <file>`).

## 6. `readPiManifest` — package.json parsing

```js
function readPiManifest(packageJsonPath) {
    try {
        const content = fs.readFileSync(packageJsonPath, "utf-8");
        const pkg = JSON.parse(content);
        if (pkg.pi && typeof pkg.pi === "object") return pkg.pi;
        return null;
    } catch { return null; }
}
```

Silently catches parse errors (returns null). weave's `check` should be stricter:
**ERROR** on unparseable package.json (§9).

## 7. Import paths

The installed npm pi is `@mariozechner/pi-coding-agent`. The PRD's example uses
`@earendil-works/pi-coding-agent` (the OSS/GitHub distribution name). Both resolve to
the same codebase. pi's loader sets up aliases:

```js
_aliases = {
    "@mariozechner/pi-coding-agent": packageIndex,
    // ... other aliases
};
```

The PRD's `@earendil-works/pi-coding-agent` import is correct for the public/OSS
distribution. Users with the npm distribution will have both resolvable.

**Recommendation**: Use the PRD's import path (`@earendil-works/pi-coding-agent`) in the
example extension as specified. It is a `type`-only import, so it has zero runtime impact
regardless of which pi distribution is installed.

## 8. Auto-discovery locations (which weave deliberately avoids)

From `extensions.md`:
- `~/.pi/agent/extensions/*.ts` (global)
- `~/.pi/agent/extensions/*/index.ts` (global, dir extensions)
- `.pi/extensions/*.ts` (project-local)
- `.pi/extensions/*/index.ts` (project-local, dir extensions)
- `settings.json` `extensions`/`packages` arrays

weave's store must NEVER be any of these. It loads ONLY via `pi -e "$(weave <tag>)"`.
