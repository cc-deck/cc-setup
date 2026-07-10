# Data Model: Permissions Subcommand

## Entities

### Profile (existing, no changes)

Loaded from YAML files in `~/.config/cc-setup/profiles/`. Represents a named set of permission rules.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Display name (e.g., "Full YOLO") |
| Description | string | Human-readable description |
| Builtin | bool | Whether this is a built-in profile |
| Mode | string | Permission mode (e.g., "bypassPermissions", "acceptEdits") |
| Permissions | ProfilePermissions | Allow/deny/bash/mcp rules |

### ProfileSlugs (new, static mapping)

Maps short aliases to profile names for built-in profiles.

| Slug | Profile Name |
|------|-------------|
| `yolo` | Full YOLO |
| `readonly` | Read-only YOLO |

### Scope (conceptual, no struct)

Determines which config files are read/written:

| Scope | settings.local.json | settings.json |
|-------|-------------------|--------------|
| `project` | `.claude/settings.local.json` | `.claude/settings.json` |
| `user` | `~/.claude/settings.local.json` | `~/.claude/settings.json` |

## Relationships

```
permCmd (cobra.Command)
├── applyCmd (cobra.Command)
│   └── resolveProfile(name) → Profile
│       ├── slug match (profileSlugs map)
│       └── exact name match (LoadProfiles)
├── showCmd (cobra.Command)
│   └── reads from scope's config files
└── resetCmd (cobra.Command)
    └── writes empty values to scope's config files

All subcommands share:
  --scope flag (project|user, default: project)
```

## State Transitions

No state machines. All operations are stateless reads or writes to config files.
