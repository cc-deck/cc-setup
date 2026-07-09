# Data Model: Permissions Tab

## Entities

### Profile

A YAML file defining a preset collection of permissions.

| Field | Type | Description |
|-------|------|-------------|
| schema_version | int | Schema version for migration (currently 1) |
| name | string | Display name (e.g., "Read-only YOLO") |
| description | string | One-line description |
| builtin | bool | True for cc-setup managed profiles |
| mode | string | Permission mode: default, acceptEdits, auto, bypassPermissions |
| permissions.allow | []string | Built-in tool permissions |
| permissions.deny | []string | Explicit deny entries |
| permissions.bash | []string | Bash command patterns |
| permissions.mcp.use_annotations | bool | Use server-provided annotations |
| permissions.mcp.heuristic.safe_prefixes | []string | Tool name prefixes considered safe |
| permissions.mcp.heuristic.unsafe_prefixes | []string | Tool name prefixes considered unsafe |
| permissions.mcp.include | []string | Explicit tool patterns to always include |
| permissions.mcp.exclude | []string | Explicit tool patterns to always exclude |

**Identity**: File path (`~/.config/cc-setup/profiles/<filename>.yaml`)
**Ownership**: Underscore prefix = cc-setup owned; no prefix = user owned

### PermissionItem (in-memory)

A single entry in the permissions list displayed on the tab.

| Field | Type | Description |
|-------|------|-------------|
| key | string | Permission string (e.g., `Read`, `Bash(git:*)`, `mcp__fs__read`) |
| category | string | `builtin`, `bash`, or `mcp` |
| serverName | string | MCP server name (empty for non-MCP) |
| hint | string | Classification indicator: `[read-only]`, `[destructive]`, `[heuristic]`, or empty |
| source | string | `profile`, `custom`, or `inherited` |

**Identity**: `key` field (unique within the list)
**Lifecycle**: Created when tab opens or profile is applied. Destroyed on tab close.

### ClassifiedTool (in-memory)

Result of MCP tool classification during profile application.

| Field | Type | Description |
|-------|------|-------------|
| name | string | Tool name from MCP server |
| approved | bool | Whether the profile considers this tool safe |
| hint | string | Classification source indicator |

## Relationships

```
Profile 1──* PermissionItem (profile application creates items)
MCP Server 1──* ToolInfo (tool discovery)
ToolInfo + Profile ──> ClassifiedTool (classification)
ClassifiedTool ──> PermissionItem (if approved)
```

## State Transitions

### Permission Source

```
(new from disk)     -> "custom" (no profile active)
(profile applied)   -> "profile"
(user toggles)      -> "custom" (was profile or custom)
(scope switch)      -> "inherited" (user-scope items in project view)
(profile "None")    -> cleared (all items removed)
```

### Permission Mode

```
"default" (initial/after "None")
    -> "acceptEdits" (Read-only YOLO profile or manual)
    -> "auto" (manual selection)
    -> "bypassPermissions" (Full YOLO profile or manual)
Any mode -> "default" (selecting "None" profile)
```

## Storage Mapping

| Concept | File | Key Path |
|---------|------|----------|
| Permission allow list | settings.local.json | `permissions.allow` |
| Permission deny list | settings.local.json | `permissions.deny` |
| Permission mode | settings.json | `permissions.defaultMode` |
| Enabled plugins | settings.json | `enabledPlugins` |
| MCP server autoApprove | central mcp.json | `servers.<name>.autoApprove` |
| Profiles | ~/.config/cc-setup/profiles/*.yaml | (entire file) |
