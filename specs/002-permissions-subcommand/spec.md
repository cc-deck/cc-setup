# Feature Specification: Permissions Subcommand

**Feature Branch**: `002-permissions-subcommand`
**Created**: 2026-07-09
**Status**: Draft
**Input**: User description: "Permissions subcommand with TUI shortcut, headless apply/show/reset, scope flag, and slug-based profile matching"

## User Scenarios & Testing

### User Story 1 - Apply a Permission Profile Headlessly (Priority: P1)

A user (or automation script) wants to quickly apply the "Full YOLO" permission profile to a project without opening the TUI. They run `cc-setup perm apply yolo` and the profile is applied to the project scope, with a confirmation message showing what was written and where.

**Why this priority**: This is the primary use case driving the feature. Headless profile application enables scripting, CI/CD integration, and quick setup without interactive sessions.

**Independent Test**: Can be fully tested by running `cc-setup perm apply yolo` in a project directory and verifying that `settings.local.json` and `settings.json` contain the expected permissions and mode.

**Acceptance Scenarios**:

1. **Given** a project directory with MCP servers configured, **When** running `cc-setup perm apply yolo`, **Then** the Full YOLO profile is applied to project scope: `settings.local.json` contains all allowed tools and MCP wildcards, `settings.json` has `bypassPermissions` mode, and a success message is printed.
2. **Given** a project directory, **When** running `cc-setup perm apply readonly`, **Then** the Read-Only YOLO profile is applied to project scope with `acceptEdits` mode.
3. **Given** a custom profile "My Custom" exists in `~/.config/cc-setup/profiles/`, **When** running `cc-setup perm apply "My Custom"`, **Then** the custom profile is applied by exact name match.
4. **Given** no profiles directory exists, **When** running `cc-setup perm apply yolo`, **Then** built-in profiles are auto-created before applying.
5. **Given** a project directory, **When** running `cc-setup perm apply yolo --scope user`, **Then** the profile is applied to user scope (`~/.claude/settings.local.json`).

---

### User Story 2 - Show Current Permissions (Priority: P2)

A user wants to see what permissions are currently configured for a scope without opening the TUI. They run `cc-setup perm show` and get a summary of the current state.

**Why this priority**: Understanding current state is essential for debugging permission issues and verifying that profiles were applied correctly.

**Independent Test**: Can be fully tested by configuring known permissions, running `cc-setup perm show`, and verifying the output matches expectations.

**Acceptance Scenarios**:

1. **Given** permissions have been configured in project scope, **When** running `cc-setup perm show`, **Then** the output shows the permission mode, allowed tools, denied tools, and the file paths where they are stored.
2. **Given** no permissions have been configured in project scope, **When** running `cc-setup perm show`, **Then** the output indicates no permissions are configured and shows the default mode.
3. **Given** permissions exist in both user and project scope, **When** running `cc-setup perm show --scope user`, **Then** only user-scope permissions are displayed.

---

### User Story 3 - Open TUI on Permissions Tab (Priority: P2)

A user wants to jump directly to the Permissions tab of the TUI without navigating from the Servers tab. They run `cc-setup perm` (bare, no subcommand) and the TUI opens with the Permissions tab active.

**Why this priority**: Provides a convenient shortcut for interactive permission management, consistent with the root command pattern.

**Independent Test**: Can be tested by running `cc-setup perm` and verifying the TUI launches with the Permissions tab selected.

**Acceptance Scenarios**:

1. **Given** servers are configured, **When** running `cc-setup perm` with no subcommand, **Then** the TUI opens with the Permissions tab active instead of the Servers tab.
2. **Given** no servers are configured, **When** running `cc-setup perm`, **Then** the empty-state message is shown (same as `cc-setup` with no servers).

---

### User Story 4 - Reset Permissions to Defaults (Priority: P3)

A user wants to clear all configured permissions and reset to a clean state. They run `cc-setup perm reset` and all permission entries and mode settings are removed from the scope's config files.

**Why this priority**: Useful for troubleshooting and starting fresh, but less frequent than apply or show.

**Independent Test**: Can be tested by configuring permissions, running `cc-setup perm reset`, and verifying the config files no longer contain permission entries.

**Acceptance Scenarios**:

1. **Given** permissions are configured in project scope, **When** running `cc-setup perm reset`, **Then** the `permissions.allow` and `permissions.deny` arrays are cleared from `settings.local.json`, the `permissions.defaultMode` is removed from `settings.json`, and a confirmation message is printed.
2. **Given** no permissions are configured, **When** running `cc-setup perm reset`, **Then** a message indicates there is nothing to reset.
3. **Given** permissions in project scope, **When** running `cc-setup perm reset --scope user`, **Then** only user-scope permissions are cleared.

---

### Edge Cases

- What happens when a profile name doesn't match any slug or exact name? An error message listing available profiles is shown.
- What happens when `perm apply` is run with no profile argument? An error message showing usage and available profiles is shown.
- What happens when `perm show` finds permissions in both scopes? Only the requested scope is shown (project by default).
- What happens when `perm reset` is run on a scope that has other settings in the config files? Only permission-related keys are removed; other settings (e.g., `enabledPlugins`) are preserved.
- What happens with an invalid `--scope` value? An error message is shown listing valid values (`project`, `user`).

## Requirements

### Functional Requirements

- **FR-001**: System MUST provide a `permissions` subcommand on the root command with `perm` as an alias.
- **FR-002**: Running `cc-setup perm` with no subcommand MUST open the TUI with the Permissions tab active.
- **FR-003**: `cc-setup perm apply <profile>` MUST apply a named profile headlessly, matching built-in slugs case-insensitively (`yolo` -> "Full YOLO", `readonly` -> "Read-only YOLO") and custom profiles by exact name.
- **FR-004**: `cc-setup perm apply` MUST write permissions to `settings.local.json` and the mode to `settings.json` for the target scope, using the same logic as the TUI save path (`runSavePermissions`).
- **FR-005**: `cc-setup perm apply` MUST expand MCP wildcards for all configured servers when the profile uses a global wildcard include.
- **FR-006**: `cc-setup perm apply` MUST run auto-consolidation on permissions before writing (same as TUI).
- **FR-007**: `cc-setup perm apply` MUST accept the trust dialog for project scope (same as TUI).
- **FR-008**: `cc-setup perm apply` MUST print a summary of what was written: entry count, file paths, mode, and any consolidated entries.
- **FR-009**: `cc-setup perm show` MUST display the current permission mode, allowed tools (grouped by builtin/bash/mcp), denied tools, and the file paths for the target scope in human-readable plain text format.
- **FR-010**: `cc-setup perm reset` MUST clear `permissions.allow`, `permissions.deny` from `settings.local.json` and `permissions.defaultMode` from `settings.json` for the target scope, preserving all other settings in those files.
- **FR-011**: All subcommands (`apply`, `show`, `reset`) MUST accept a `--scope` flag with values `project` (default) or `user`.
- **FR-012**: The existing `--full-yolo` flag on the root command MUST be removed.
- **FR-013**: The existing `runFullYolo()` function MUST be removed and its logic subsumed by `perm apply`.
- **FR-014**: `cc-setup perm apply` with no argument MUST show an error with usage and list available profiles.
- **FR-015**: `cc-setup perm apply` with an unrecognized profile name MUST show an error listing available profiles.
- **FR-016**: Built-in profiles MUST be auto-created (via `EnsureBuiltinProfiles`) before any profile operation.

### Key Entities

- **Profile**: A named set of permission rules (mode, allow list, deny list, bash patterns, MCP rules) loaded from YAML files in `~/.config/cc-setup/profiles/`.
- **Scope**: The target configuration scope, either `project` (current directory's `.claude/`) or `user` (`~/.claude/`).
- **Slug**: A short, case-insensitive alias for built-in profiles (e.g., `yolo` for "Full YOLO").

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can apply any built-in profile with a single command (`cc-setup perm apply yolo`) completing in under 2 seconds.
- **SC-002**: `cc-setup perm show` provides enough information for a user to understand their current permission state without opening any config files.
- **SC-003**: `cc-setup perm reset` fully clears permission state without affecting other configuration (plugins, MCP servers).
- **SC-004**: All headless commands (`apply`, `show`, `reset`) work in non-interactive environments (CI, scripts, SSH sessions without TTY).
- **SC-005**: The `perm` alias reduces typing by 60% compared to the full `permissions` subcommand name.

## Clarifications

### Session 2026-07-09

- Q: Should `perm show` support structured output (e.g., `--json`)? → A: No, human-readable plain text only for v1. Output is categorized by builtin/bash/mcp, matching the TUI grouping.
- Q: Should `perm reset` require user confirmation before clearing? → A: No confirmation required. The action is intentional, low-risk (permissions can be re-applied instantly), and requiring confirmation would break non-interactive use (SC-004).

## Assumptions

- The existing `runSavePermissions` function and `config.WriteAllPermissions`/`config.WritePermissionMode` functions are stable and will be reused without modification.
- The existing `config.LoadProfiles` and `config.EnsureBuiltinProfiles` functions handle profile file management.
- The TUI already supports starting on a specific tab (or can be modified to accept a starting tab parameter with minimal effort).
- Only two built-in slugs are needed: `yolo` and `readonly`. Custom profiles use exact name matching.
- The `--scope` flag follows the same pattern as the TUI scope selector (project vs user).
