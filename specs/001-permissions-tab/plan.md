# Implementation Plan: Permissions Tab

**Branch**: `001-permissions-tab` | **Date**: 2026-04-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-permissions-tab/spec.md`

## Summary

Add a third "Permissions" tab to cc-setup that provides a unified view of all Claude Code permissions (built-in tools, bash patterns, MCP tools) with profile-based presets and permission mode management. Profiles (YAML files) serve as one-time initializers that populate permissions and set the permission mode. The tab supports scope switching (user/project) with inherited permission visibility, guided permission adding, and MCP tool classification via annotations and heuristics.

## Technical Context

**Language/Version**: Go 1.25.0
**Primary Dependencies**: bubbletea v1.3.6, bubbles v0.21.1, huh v0.8.0, lipgloss v1.1.0, cobra v1.10.2, go-sdk (MCP) v1.3.1, gopkg.in/yaml.v3 (new dependency)
**Storage**: JSON files (settings.local.json, settings.json), YAML files (profiles)
**Testing**: go test
**Target Platform**: macOS/Linux CLI
**Project Type**: CLI tool
**Constraints**: Must fit existing TUI patterns, preserve all unknown JSON keys when writing config

## Constitution Check

*No constitution defined (template only). No gates to check.*

## Project Structure

### Documentation (this feature)

```text
specs/001-permissions-tab/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
├── manage.go            # MODIFY: Add tabPermissions enum, tab switching, banner rendering
├── permissions.go       # NEW: Permissions tab TUI model (permissionsModel, delegate, key map)
├── tools.go             # EXISTING: Tool permissions drill-down (unchanged)
└── plugins.go           # EXISTING: Plugin tab components (reference pattern)

internal/config/
├── claude.go            # MODIFY: Add ReadAllPermissions, WriteAllPermissions, ReadPermissionMode, WritePermissionMode
├── profiles.go          # NEW: Profile loading, built-in profile management, YAML parsing, MCP classification
└── plugins.go           # EXISTING: settings.json reading (reference for PluginSettingsPath)

internal/display/
└── display.go           # MODIFY: Add permission-specific styles (amber for custom, indicators)
```

**Structure Decision**: Single project layout, extending existing cmd/ and internal/ packages. No new packages needed.

## Phase 0: Research

### R1: YAML Library Selection

**Decision**: Use `gopkg.in/yaml.v3` for profile parsing.
**Rationale**: Standard Go YAML library, well-maintained, supports struct tags matching JSON patterns already used.
**Alternatives**: `sigs.k8s.io/yaml` (heavier, Kubernetes-focused), `github.com/pelletier/go-toml` (TOML instead of YAML, rejected per spec).

### R2: Permission Mode Storage

**Decision**: Write `permissions.defaultMode` to `settings.json` (not `settings.local.json`).
**Rationale**: Claude Code reads `permissions.defaultMode` from `settings.json`. The existing `PluginSettingsPath(scope)` function already resolves the correct path for both user and project scope. The raw JSON map preservation pattern used by `WriteEnabledPlugins` can be reused.
**Key finding**: The current `ReadToolPermissions`/`WriteToolPermissions` in claude.go work with `settings.local.json` for `permissions.allow/deny`. Permission mode is a separate field in `settings.json`. Both files need to be written on save.

### R3: All-Permissions Reading

**Decision**: Create a new `ReadAllPermissions(scope)` function that reads the full `permissions.allow` array from `settings.local.json` (not just MCP-scoped entries).
**Rationale**: Existing `ReadToolPermissions` filters by server name. The Permissions tab needs the unfiltered list to display built-in tools, bash patterns, and MCP tools together.

### R4: Profile Application and MCP Discovery

**Decision**: MCP tool discovery is lazy (on profile apply only). The existing `mcpclient.ListTools()` function and `toolCache` (sync.Map) in cmd/tools.go can be reused.
**Rationale**: Per clarification, discovery happens on demand to avoid unnecessary network calls. The tool cache prevents redundant discoveries within a session.

### R5: Built-in Profile Embedding

**Decision**: Embed built-in profile YAML content as Go string constants in `internal/config/profiles.go` using `go:embed` or raw strings. Write them to disk at `~/.config/cc-setup/profiles/` on first run or when outdated.
**Rationale**: Ensures built-in profiles ship with the binary. The underscore prefix convention (`_readonly-yolo.yaml`, `_full-yolo.yaml`) distinguishes them from user profiles.

## Phase 1: Design

### Data Flow

```
Profile YAML files           settings.local.json          settings.json
~/.config/cc-setup/profiles/  ~/.claude/ or .claude/        ~/.claude/ or .claude/
                               permissions.allow: [...]     permissions.defaultMode: "..."
                               permissions.deny: [...]      enabledPlugins: {...}
        │                           │                            │
        ▼                           ▼                            ▼
   LoadProfiles()            ReadAllPermissions()         ReadPermissionMode()
        │                           │                            │
        └───────────────────────────┴────────────────────────────┘
                                    │
                                    ▼
                          permissionsModel (TUI)
                            - permList (list.Model)
                            - checked map[string]bool
                            - permSource map[string]string  ("profile"/"custom"/"inherited")
                            - mode string
                            - profile string (transient)
                                    │
                                    ▼
                         WriteAllPermissions()  +  WritePermissionMode()
                                    │
                                    ▼
                          settings.local.json  +  settings.json
```

### Permission Item Model

Each item in the permissions list is a `permissionItem`:
- `key`: The permission string as stored (e.g., `Read`, `Bash(git:*)`, `mcp__filesystem__read_file`)
- `category`: One of `builtin`, `bash`, `mcp`
- `serverName`: For MCP items, the server name (empty for builtin/bash)
- `hint`: Classification indicator (`[read-only]`, `[destructive]`, `[heuristic]`, empty)
- `source`: `"profile"`, `"custom"`, or `"inherited"` (in-memory, for coloring)

### Profile Application Flow

1. User presses `p`, sees profile list
2. If custom changes exist, confirmation dialog: "Reset N changes? [Apply] [Cancel]"
3. On apply:
   a. Clear all permissions and source tracking
   b. Set permission mode from profile
   c. Add profile's `permissions.allow` entries (source: "profile")
   d. Add profile's `permissions.bash` entries as `Bash(<pattern>)` (source: "profile")
   e. For MCP: discover tools from enabled servers (lazy, with loading indicator)
   f. Classify each MCP tool (annotations > heuristic > profile overrides)
   g. Add approved MCP tools as `mcp__<server>__<tool>` (source: "profile")
4. User can then toggle/add/delete (changes marked as "custom")
5. On save: write expanded list to settings.local.json, write mode to settings.json

### MCP Tool Classification Logic

```
classifyTool(tool ToolInfo, profile Profile) -> (approved bool, hint string)
  if profile.MCP.Include contains tool pattern -> (true, "")
  if profile.MCP.Exclude contains tool pattern -> (false, "")
  if profile.MCP.UseAnnotations && tool.HasAnnotations:
    if tool.ReadOnlyHint -> (true, "[read-only]")
    if tool.DestructiveHint -> (false, "[destructive]")
  // Heuristic fallback
  for each prefix in profile.MCP.Heuristic.SafePrefixes:
    if tool.Name starts with prefix -> (true, "[heuristic]")
  for each prefix in profile.MCP.Heuristic.UnsafePrefixes:
    if tool.Name starts with prefix -> (false, "[heuristic]")
  // Unknown: default to not approved
  return (false, "")
```

### Key Bindings (Permissions Tab)

| Key | Action | Implementation |
|-----|--------|----------------|
| `space`/`x` | Toggle permission | Flip checked map, set source to "custom" |
| `a` | Add permission | Exit TUI, run guided input flow, re-enter TUI |
| `d` | Delete permission | Remove from list and checked map |
| `p` | Profile selector | Exit TUI, run huh form selector, re-enter TUI |
| `m` | Mode selector | Exit TUI, run huh form selector, re-enter TUI |
| `s` | Save | Exit TUI, write to disk, re-enter TUI |
| `tab` | Switch tab | Handled by parent manageModel |
| `u`/`P` | Switch scope | Reload from disk for new scope |
| `/` | Filter | BubbleTea list built-in filtering |

### Save Logic

On save, two files are written:
1. **settings.local.json** (via `WriteAllPermissions`): All checked permissions go into `permissions.allow`. Unchecked permissions are removed. Other keys (deny, ask, disabledMcpjsonServers) are preserved.
2. **settings.json** (via `WritePermissionMode`): `permissions.defaultMode` is set. Other keys (enabledPlugins, etc.) are preserved.

### Integration with manage.go

The `manageModel` struct gets:
- New `tabPermissions` enum value (after `tabPlugins`)
- New `permList list.Model` and `permKeys permissionsKeyMap` fields
- New `permChecked map[string]bool` and `permSource map[string]string`
- New `permMode string` field
- Tab switching extended to cycle through three tabs
- Banner rendering updated for three tab pills
- `View()` routes to permissions rendering when `tab == tabPermissions`

## Implementation Tasks

### Task 1: Add YAML dependency and profile data types (P1)
- Add `gopkg.in/yaml.v3` to go.mod
- Define `Profile` struct in `internal/config/profiles.go` matching schema v1
- Define `ProfileMCP`, `ProfileHeuristic` sub-structs
- Write unit tests for YAML parsing

### Task 2: Profile loading and built-in profile management (P1)
- Embed built-in profile content (`_readonly-yolo.yaml`, `_full-yolo.yaml`) as Go constants
- `EnsureBuiltinProfiles()`: write/overwrite underscore-prefixed files to `~/.config/cc-setup/profiles/`
- `LoadProfiles()`: scan directory, parse all `.yaml` files, return slice of Profile
- `LoadProfile(name)`: load single profile by name
- Handle malformed YAML gracefully (skip with error)
- Write unit tests

### Task 3: Permission reading/writing for all permission types (P1)
- `ReadAllPermissions(scope)`: read full `permissions.allow` array from settings.local.json
- `WriteAllPermissions(scope, permissions)`: write full permissions.allow array, preserving other keys
- `ReadPermissionMode(scope)`: read `permissions.defaultMode` from settings.json
- `WritePermissionMode(scope, mode)`: write `permissions.defaultMode` to settings.json, preserving other keys
- Write unit tests

### Task 4: MCP tool classification logic (P2)
- `ClassifyTool(tool ToolInfo, profile Profile) -> (approved bool, hint string)`
- Implements: explicit include/exclude > annotations > heuristic > default deny
- `ClassifyServerTools(tools []ToolInfo, profile Profile) -> []ClassifiedTool`
- Write unit tests for each classification path

### Task 5: Permissions tab TUI model (P1)
- Define `permissionItem` struct (key, category, serverName, hint, source)
- Define `permissionsKeyMap` with all key bindings
- Define `permissionCheckboxDelegate` for rendering with source-based coloring
- `buildPermissionItems()`: construct list from ReadAllPermissions + enabled MCP servers
- Toggle, dirty-checking, scope switching logic
- Integrate into `manageModel`: add `tabPermissions`, extend tab switching, update banner for three tabs

### Task 6: Profile selector flow (P1)
- Profile picker using `huh` form (list of profiles + "None")
- Confirmation dialog when custom changes would be reset
- Profile application: clear permissions, set mode, add profile entries, classify MCP tools
- MCP tool discovery with loading indicator (reuse toolCache)
- Source tracking: mark all applied permissions as "profile"

### Task 7: Permission mode selector (P1)
- Mode picker using `huh` form (default, acceptEdits, auto, bypassPermissions)
- Warning display when bypassPermissions is selected
- Mode persisted on save via WritePermissionMode

### Task 8: Add permission guided flow (P2)
- Category selector (Built-in tool, Bash pattern, MCP tool)
- Built-in tool: list selector with known tools
- Bash pattern: text input with curated suggestions list
- MCP tool: server selector, then tool discovery and selection
- Added permissions marked as "custom" source

### Task 9: Scope switching and inherited permissions (P2)
- On scope switch: reload permissions from disk for new scope
- In project scope: load user-scope permissions as read-only "inherited" items
- Inherited items rendered dimmed, not toggleable (toggling creates project override)
- Override logic: toggling inherited item creates a project-scope entry marked "custom"

### Task 10: Save flow (P1)
- Write permissions to settings.local.json via WriteAllPermissions
- Write mode to settings.json via WritePermissionMode
- Sync autoApprove in central config via existing UpdateAutoApprove
- Dirty checking: compare current state to initial state
- Quit confirmation dialog with unsaved changes summary

### Task 11: Display styles and visual polish (P2)
- Amber/yellow style for "custom" source permissions
- Teal style for "profile" source permissions
- Dimmed style for "inherited" permissions
- Classification indicators ([read-only], [destructive], [heuristic])
- bypassPermissions warning banner
- Section headers (Built-in Tools, Bash Patterns, MCP: servername)

### Task 12: Integration testing (P2)
- End-to-end test: load permissions, toggle, save, verify file
- Profile application test: apply profile, verify permissions and mode
- Scope switching test: verify inherited permissions display
- Edge case tests: malformed profiles, unreachable servers, empty state
