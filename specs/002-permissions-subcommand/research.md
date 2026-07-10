# Research: Permissions Subcommand

## How does the TUI currently start on a specific tab?

**Decision**: Modify `runManage()` to accept a starting tab parameter.

**Rationale**: `runManage()` currently hardcodes `tab := tabServers` at line 1118 of `cmd/manage.go`. The function already passes `tab` to `newManageModel()` which accepts it as a parameter (line 468). Changing `runManage()` to `runManageWithTab(startTab manageTab)` (or adding a parameter) is a one-line change at the declaration plus the call site.

**Alternatives considered**:
- Global variable for start tab: rejected (couples state unnecessarily)
- New TUI entry point: rejected (duplicates the manage loop)

## How should slug matching work for built-in profiles?

**Decision**: Map slugs to profile names with a static lookup table. Case-insensitive comparison via `strings.EqualFold`.

**Rationale**: Only two built-in profiles exist. A static map is simpler and more explicit than deriving slugs from profile names algorithmically.

**Implementation**:
```go
var profileSlugs = map[string]string{
    "yolo":     "Full YOLO",
    "readonly": "Read-only YOLO",
}
```

Resolution order:
1. Check slug map (case-insensitive key match)
2. If no slug match, try exact name match against loaded profiles
3. If no match, error with available profiles listed

**Alternatives considered**:
- Algorithmic slug derivation (lowercase, strip spaces): rejected (fragile, harder to document)
- Fuzzy matching: rejected (over-engineering for 2 profiles)

## How should `perm show` format output?

**Decision**: Human-readable plain text, grouped by category. No `--json` for v1.

**Rationale**: Primary use case is a quick glance at current state. Grouping matches the TUI's visual categories. JSON output can be added later if scripting needs arise.

**Output format**:
```
Permission Mode: bypassPermissions
Scope: project

Allowed (12 entries):
  Built-in:
    Agent, Bash, Edit, Glob, Grep, Read, Skill, ToolSearch, WebFetch, WebSearch, Write
  Bash patterns:
    Bash(*)
  MCP:
    mcp__server1__*, mcp__server2__*

Denied (0 entries):
  (none)

Files:
  Permissions: .claude/settings.local.json
  Mode: .claude/settings.json
```

## How should `perm reset` clear permissions?

**Decision**: Use existing `WriteAllPermissions` with empty slices and `WritePermissionMode` with empty string.

**Rationale**: These functions already handle the JSON merge-and-preserve logic. Passing empty values clears only the permission keys while preserving `enabledPlugins` and other settings.

**Verification**: `WriteAllPermissions(scope, []string{}, []string{})` writes `{"permissions":{"allow":[],"deny":[]}}` which effectively clears. `WritePermissionMode(scope, "")` removes the `defaultMode` key.

## How should the cobra command tree be structured?

**Decision**: `permCmd` as a parent command with `applyCmd`, `showCmd`, `resetCmd` as subcommands. Bare `permCmd` (no subcommand) opens the TUI.

**Rationale**: Cobra supports this pattern natively. The parent command's `RunE` handles the bare case (TUI), while subcommands handle headless operations.

**Implementation pattern**:
```go
var permCmd = &cobra.Command{
    Use:     "permissions",
    Aliases: []string{"perm"},
    Short:   "Manage Claude Code permissions",
    RunE: func(cmd *cobra.Command, args []string) error {
        return runManageWithTab(tabPermissions)
    },
}
```

Each subcommand gets its own `--scope` flag via `PersistentFlags` on `permCmd`.
