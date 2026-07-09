# Research: Permissions Tab

## R1: YAML Library Selection

- **Decision**: `gopkg.in/yaml.v3`
- **Rationale**: Standard Go YAML library, stable API, struct tags match JSON patterns
- **Alternatives**: `sigs.k8s.io/yaml` (too heavy), TOML (rejected in spec)

## R2: Permission Mode Storage Location

- **Decision**: Write `permissions.defaultMode` to `settings.json` (via `PluginSettingsPath`)
- **Rationale**: Claude Code reads permission mode from `settings.json`, not `settings.local.json`. The existing path resolution and JSON preservation pattern from `WriteEnabledPlugins` can be reused.
- **Key detail**: `settings.local.json` stores `permissions.allow`/`deny` arrays. `settings.json` stores `permissions.defaultMode` and `enabledPlugins`. Two different files, both need updating on save.

## R3: All-Permissions Reading

- **Decision**: New `ReadAllPermissions(scope)` returns the raw `permissions.allow` array
- **Rationale**: Existing `ReadToolPermissions(scope, serverName)` filters by MCP server prefix. The Permissions tab needs all entries (built-in tools, bash patterns, MCP tools) unfiltered.
- **Implementation**: Same JSON reading pattern as existing functions, just skip the `mcp__` prefix filter.

## R4: MCP Tool Discovery Timing

- **Decision**: Lazy discovery on profile application, not on tab open
- **Rationale**: Avoids unnecessary network calls. Users may only view/edit static permissions without using profiles.
- **Implementation**: Reuse `mcpclient.ListTools()` and `toolCache` (sync.Map in cmd/tools.go) for caching within session.

## R5: Built-in Profile Embedding

- **Decision**: Embed YAML as Go string constants, write to disk on startup
- **Rationale**: Binary ships profiles without external file dependencies. Underscore prefix convention makes ownership clear.
- **Implementation**: `go:embed` or raw string constants in profiles.go. `EnsureBuiltinProfiles()` called during manage command init.

## R6: Permission Categories from String Format

Claude Code permission strings have predictable formats:
- **Built-in tools**: Single word: `Read`, `Write`, `Edit`, `Glob`, `Grep`, `WebFetch`, `WebSearch`
- **Built-in tools with args**: `WebFetch(*)`, `WebSearch(*)`
- **Bash patterns**: `Bash(command)`, `Bash(command:*)`, `Bash(*)`
- **MCP tools**: `mcp__<serverName>__<toolName>`, `mcp__<serverName>__*`

Categorization logic:
- Starts with `mcp__` -> MCP tool (extract server name from second segment)
- Starts with `Bash(` -> Bash pattern
- Everything else -> Built-in tool
