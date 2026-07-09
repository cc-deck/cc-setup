# Brainstorm: Permissions Tab for cc-setup

**Date**: 2026-04-16
**Status**: Brainstorm complete, ready for spec

## Problem Statement

Claude Code's permission system is powerful but scattered. Permissions live in `settings.local.json` (user and project scope), MCP tool permissions are managed per-server, and there's no quick way to switch between permission "modes" for different workflows. Users doing research tasks want uninterrupted operation. Users doing exploratory work want safety rails. Switching between these modes currently requires manual JSON editing or re-approving permissions one by one.

**Goal**: Make research-like tasks and other workflows as continuous as possible by providing a unified permission management UI with profile-based presets.

## Design Decisions

### D1: Permissions Tab is the single pane of glass

A third tab (alongside Servers and Plugins) that shows all Claude Code permissions in one scrollable list: built-in tools, bash patterns, and MCP tool permissions for all enabled servers. Flat list with section headers (not collapsible tree). Revisit if the list gets too large.

### D2: Profiles are presets, not live bindings

Profiles initialize the permission list but are not stored. What gets persisted is always the expanded permission set in `settings.local.json`. Selecting a profile is a one-time population action. cc-setup does not track "which profile is active."

### D3: Profile storage and ownership

- Location: `~/.config/cc-setup/profiles/`
- cc-setup owned profiles use underscore prefix: `_readonly-yolo.yaml`, `_full-yolo.yaml`
- cc-setup upgrades can overwrite underscore-prefixed files
- User-created profiles: any `.yaml` without underscore prefix, never modified by cc-setup
- Schema version field enables future schema migrations for user profiles

### D4: MCP tool classification for profiles

When a profile is applied, MCP tools are classified as safe/unsafe:

1. **Primary**: Use server-provided `ReadOnlyHint` / `DestructiveHint` annotations
2. **Fallback** (no annotations): Heuristic based on tool name prefixes
   - Safe: `list`, `get`, `read`, `search`, `describe`, `show`, `fetch`, `count`, `check`, `view`
   - Unsafe: `create`, `delete`, `update`, `write`, `execute`, `run`, `modify`, `remove`, `drop`, `send`
3. **Override**: Profile can explicitly include/exclude tool patterns

### D5: Scope and MCP server scoping

- Permissions tab respects the current scope toggle (user/project)
- MCP tools shown only for servers enabled in the current scope
- In project scope, inherited user-scope permissions shown as read-only items (similar to inherited servers on the Servers tab)

### D6: Visual distinction for permission sources

- **Profile-derived** permissions (unmodified from profile): standard teal
- **User-customized** permissions (added/changed after profile): amber/yellow
- **Inherited** (user-scope permissions visible in project scope): dimmed/read-only

### D7: Profile switching resets custom changes

Switching profiles shows a confirmation dialog: "Switching to 'Read-only YOLO' will reset N custom permission changes. [Apply] [Cancel]". Reuses the existing confirmation dialog pattern.

### D8: Coexistence with Servers tab tool permissions

Both the Servers tab drill-down and the Permissions tab can edit MCP tool permissions. They read/write the same `settings.local.json` data. No sync issues because both reflect current file state on load.
- Servers tab: server-centric deep-dive
- Permissions tab: global permission overview

## Profile Schema (v1)

```yaml
schema_version: 1
name: "Read-only YOLO"
description: "Auto-approve all non-destructive operations"
builtin: true  # cc-setup managed

# Permission mode: default, acceptEdits, auto, bypassPermissions
mode: "acceptEdits"

permissions:
  # Built-in Claude Code tools
  allow:
    - Read
    - Glob
    - Grep
    - WebFetch(*)
    - WebSearch
  deny: []

  # Bash command patterns
  bash:
    - "git status"
    - "git log:*"
    - "git diff:*"
    - "git branch"
    - "ls:*"
    - "pwd"
    - "which:*"
    - "cat:*"
    - "head:*"
    - "tail:*"
    - "wc:*"
    - "find:*"
    - "tree:*"
    - "rg:*"
    - "cargo test:*"
    - "go test:*"
    - "npm test"
    - "make:*"
    - "python --version"
    - "node --version"
    - "uv:*"

  # MCP tool classification rules
  mcp:
    use_annotations: true
    heuristic:
      safe_prefixes:
        - list
        - get
        - read
        - search
        - describe
        - show
        - fetch
        - count
        - check
        - view
      unsafe_prefixes:
        - create
        - delete
        - update
        - write
        - execute
        - run
        - modify
        - remove
        - drop
        - send
    include: []
    exclude: []
```

## UX Layout

```
+----------------------------------------------------------+
| [Servers] [Plugins] [Permissions]       [Project] [User]  |
| ~/.claude/settings.local.json                             |
+----------------------------------------------------------+
|  Profile: [None v]  (p to change)                         |
|-----------------------------------------------------------+
|  Built-in Tools                                           |
|  ----------------                                         |
|  [x] Read                                    (profile)    |
|  [x] Glob                                    (profile)    |
|  [x] Grep                                    (profile)    |
|  [ ] Write                                                |
|  [ ] Edit                                                 |
|  [x] WebFetch(*)                             (profile)    |
|  [x] WebSearch                               (profile)    |
|                                                           |
|  Bash Patterns                                            |
|  -------------                                            |
|  [x] git status                              (profile)    |
|  [x] git log:*                               (profile)    |
|  [x] npm test                                (custom)     |
|                                                           |
|  MCP: filesystem (3/7 tools)                              |
|  --------------------------                               |
|  [x] read_file              [read-only]      (profile)    |
|  [x] list_directory         [read-only]      (profile)    |
|  [ ] write_file             [destructive]                 |
|  [x] search_files           [heuristic]      (profile)    |
|                                                           |
|  MCP: github (5/12 tools)                                 |
|  -------------------------                                |
|  ...                                                      |
+----------------------------------------------------------+
|  space toggle  a add  d delete  p profile  s save         |
+----------------------------------------------------------+
```

## Key Bindings

| Key | Action |
|-----|--------|
| `space` | Toggle permission on/off |
| `a` | Add permission (category selector, then input with proposals) |
| `d` | Delete selected permission |
| `p` | Open profile selector |
| `s` | Save to `settings.local.json` |
| `tab` | Switch tabs |
| `u`/`P` | Switch scope |
| `/` | Filter permissions list |

## Adding Permissions

When user presses `a`:
1. Category selector: `Built-in tool`, `Bash pattern`, `MCP tool`
2. Per category:
   - **Built-in tools**: selectable list of known tools (Read, Write, Edit, Glob, Grep, WebFetch, WebSearch, Bash)
   - **Bash patterns**: free text input with syntax hint, plus curated proposals of common safe patterns (git:\*, npm test, cargo test:\*, go test:\*, make:\*, python:\*, kubectl:\*, etc.)
   - **MCP tools**: server selector, then tool list (reuses existing tool discovery from `mcpclient.ListTools()`)

## Built-in Profiles

### Read-only YOLO (`_readonly-yolo.yaml`)
- Permission mode: `acceptEdits` (auto-approves file reads and common filesystem operations)
- All read-only built-in tools (Read, Glob, Grep, WebFetch, WebSearch)
- Curated safe bash patterns (git read commands, ls, cat, test runners, version checks)
- MCP tools: annotation-based + heuristic fallback, only safe tools

### Full YOLO (`_full-yolo.yaml`)
- Permission mode: `bypassPermissions` (skips all safety checks including compound command prompts)
- All permissions: `["*"]`
- All bash: `["*"]`
- All MCP tools: include `["*"]`

## Out of Scope

- Deny-list management in the TUI (profiles can express deny, UI focuses on allow-listing)
- Profile editor in the TUI (profiles are text files)
- Auto-detection of "which profile matches current permissions"
- Collapsible tree UI (start flat, revisit if needed)

## Implementation Notes

- Reuse existing `ReadToolPermissions` / `WriteToolPermissions` from `config/claude.go`
- Reuse existing `ListTools` from `internal/mcp/mcpclient` for MCP tool discovery
- Profile YAML parsing via `gopkg.in/yaml.v3` (already available or trivial to add)
- New `internal/config/profiles.go` for profile loading, built-in profile management, schema validation
- New `cmd/permissions.go` for the TUI model (mirrors structure of `manage.go` and `tools.go`)
- Tab enum extension: `tabServers`, `tabPlugins`, `tabPermissions`
