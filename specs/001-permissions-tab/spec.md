# Feature Specification: Permissions Tab

**Feature Branch**: `001-permissions-tab`
**Created**: 2026-04-16
**Status**: Draft
**Input**: User description: "Add a third tab that manages permissions (user and project wide), with YOLO mode profiles for unattended operation and read-only YOLO for non-invasive tasks, fitting into the existing UX to make research-like tasks as continuous as possible."

## User Scenarios & Testing

### User Story 1 - View and Toggle Permissions (Priority: P1)

A user opens cc-setup and navigates to the Permissions tab to see the current permission mode and all currently allowed permissions across built-in tools, bash patterns, and MCP tools for enabled servers. They can toggle individual permissions on or off, change the permission mode, then save the changes.

**Why this priority**: This is the foundation of the feature. Without the ability to view and edit permissions, nothing else works. It delivers immediate value by replacing manual JSON editing.

**Independent Test**: Open cc-setup, switch to the Permissions tab, toggle a permission, save, and verify the change appears in `settings.local.json`.

**Acceptance Scenarios**:

1. **Given** cc-setup is running with existing permissions in `settings.local.json`, **When** the user presses `tab` to navigate to the Permissions tab, **Then** the current permission mode is displayed at the top and all current permissions are displayed in a flat list grouped by category (Built-in Tools, Bash Patterns, MCP servers) with checked/unchecked state matching the file.
2. **Given** the Permissions tab is active, **When** the user presses `space` on a permission, **Then** the permission toggles between checked and unchecked.
3. **Given** the user has toggled permissions, **When** they press `s`, **Then** the changes (including permission mode) are written to `settings.local.json` and `settings.json` for the current scope and a confirmation is shown.
4. **Given** the Permissions tab is active with project scope selected, **When** the user views the list, **Then** user-scope permissions that are inherited appear as dimmed read-only items.
5. **Given** the Permissions tab is active, **When** the user presses `m`, **Then** a mode selector appears with the available permission modes (default, acceptEdits, auto, bypassPermissions) and the current mode is highlighted.

---

### User Story 2 - Apply a Profile to Initialize Permissions (Priority: P1)

A user wants to quickly set up a "Read-only YOLO" permission set for a research session. They select a profile from the profile picker, which populates the permission list with the profile's defined permissions. The user can then fine-tune before saving.

**Why this priority**: Profiles are the key differentiator that makes this feature more than just a settings editor. They enable the primary use case of quickly switching between permission modes for different workflows.

**Independent Test**: Open cc-setup, go to the Permissions tab, press `p`, select "Read-only YOLO", verify the permission list is populated with the expected read-only permissions and the permission mode is set to the profile's mode, optionally adjust, then save.

**Acceptance Scenarios**:

1. **Given** the Permissions tab is active, **When** the user presses `p`, **Then** a list of available profiles is shown (including "None" and all profiles from `~/.config/cc-setup/profiles/`). Selecting "None" clears all permissions and resets the permission mode to `default`.
2. **Given** the profile picker is open, **When** the user selects "Read-only YOLO", **Then** the permission list is populated with the profile's defined permissions (Read, Glob, Grep, WebFetch, WebSearch, safe bash patterns, and read-only MCP tools) and the permission mode is set to the profile's defined mode (e.g., `acceptEdits`).
3. **Given** the profile picker is open, **When** the user selects "Full YOLO", **Then** all permissions are enabled and the permission mode is set to `bypassPermissions`.
4. **Given** a profile has been applied, **When** the user views the permission list, **Then** profile-derived permissions are visually distinct (teal) from any user-customized permissions (amber/yellow).
5. **Given** a profile has been applied and the user has made custom changes, **When** they switch to a different profile, **Then** a confirmation dialog warns that N custom changes will be reset and asks for confirmation.
6. **Given** a profile is applied, **When** the user adds or toggles a permission manually, **Then** that permission is marked as "custom" with the distinct visual treatment.
7. **Given** a profile that sets `bypassPermissions` mode is applied, **When** the user views the tab, **Then** a prominent warning is displayed indicating that all safety checks are disabled.

---

### User Story 3 - Add New Permissions with Guided Input (Priority: P2)

A user wants to add a specific bash pattern permission that is not covered by any profile. They use the add flow which guides them through selecting a category and entering the permission with syntax help and suggestions.

**Why this priority**: Adding custom permissions extends the utility beyond profiles and gives users full control. It is a natural follow-up to the core view/toggle and profile features.

**Independent Test**: Open cc-setup, go to the Permissions tab, press `a`, select "Bash pattern", see suggestions and syntax help, type a pattern, confirm, and verify it appears in the list.

**Acceptance Scenarios**:

1. **Given** the Permissions tab is active, **When** the user presses `a`, **Then** a category selector appears with options: Built-in tool, Bash pattern, MCP tool.
2. **Given** "Built-in tool" is selected, **When** the selector opens, **Then** a list of known tools is shown (Read, Write, Edit, Glob, Grep, WebFetch, WebSearch, Bash) and the user can pick one.
3. **Given** "Bash pattern" is selected, **When** the input opens, **Then** a curated list of common safe patterns is shown as suggestions (e.g., `git:*`, `npm test`, `cargo test:*`, `kubectl:*`, `make:*`) and the user can select one or type a custom pattern.
4. **Given** "MCP tool" is selected, **When** the selector opens, **Then** a list of enabled MCP servers is shown, followed by a tool list for the chosen server (discovered via MCP protocol).

---

### User Story 4 - MCP Tool Classification During Profile Application (Priority: P2)

When a profile is applied, MCP tools from enabled servers are automatically classified as safe or unsafe based on server-provided annotations and name-based heuristics. The user sees the classification reasoning and can override.

**Why this priority**: Correct MCP tool classification makes profiles practical for the primary use case. Without it, users would need to manually decide on every MCP tool.

**Independent Test**: Apply "Read-only YOLO" profile with an MCP server that has tools with and without annotations. Verify annotated tools are classified by annotation, unannotated tools by name heuristic, and the classification indicator is shown.

**Acceptance Scenarios**:

1. **Given** a profile with `mcp.use_annotations: true` is applied, **When** an MCP tool has `ReadOnlyHint: true`, **Then** the tool is auto-approved and shown with a `[read-only]` indicator.
2. **Given** a profile is applied, **When** an MCP tool has `DestructiveHint: true`, **Then** the tool is not auto-approved and shown with a `[destructive]` indicator.
3. **Given** a profile is applied, **When** an MCP tool has no annotations, **Then** the heuristic classifies it based on name prefix and shows a `[heuristic]` indicator.
4. **Given** a tool was classified by heuristic, **When** the user toggles it, **Then** the tool's state changes and its source changes to "custom."

---

### User Story 5 - Scope Switching on Permissions Tab (Priority: P2)

A user switches between user and project scope on the Permissions tab to view and edit permissions at different levels. Project scope shows inherited user-scope permissions as read-only and allows project-specific overrides.

**Why this priority**: Scope support is needed for the permissions tab to match the existing UX pattern and serve the primary workflow where users set global defaults (user scope) and project-specific overrides (project scope).

**Independent Test**: Open Permissions tab, verify user scope permissions, switch to project scope, verify inherited items appear dimmed, add a project-specific permission, save, verify it lands in the project-scoped settings file.

**Acceptance Scenarios**:

1. **Given** the Permissions tab is in user scope, **When** the user presses the project scope key, **Then** the view switches to show project-scoped permissions with inherited user permissions shown as dimmed read-only items.
2. **Given** project scope is active, **When** the user toggles an inherited permission, **Then** a project-level override is created (adding or removing the permission at project scope), and the item's visual treatment changes from "inherited" to "custom."
3. **Given** project scope is active, **When** the user adds a new permission, **Then** it is saved to the project-scoped settings file.

---

### User Story 6 - Built-in Profile Management Across Upgrades (Priority: P3)

When cc-setup is upgraded, built-in profiles (underscore-prefixed files) are updated to reflect new defaults without affecting user-created profiles.

**Why this priority**: Important for long-term maintainability but not needed for initial release. Users can manually update profile files if needed.

**Independent Test**: Place a modified `_readonly-yolo.yaml` in the profiles directory, run an upgraded cc-setup, verify the file is overwritten with the new defaults. Place a custom `my-profile.yaml`, verify it is untouched after upgrade.

**Acceptance Scenarios**:

1. **Given** cc-setup is upgraded, **When** it starts, **Then** built-in profiles (underscore-prefixed) are overwritten with the latest defaults.
2. **Given** a user-created profile exists, **When** cc-setup is upgraded, **Then** the user profile is not modified.
3. **Given** a user profile has an older `schema_version`, **When** cc-setup loads it, **Then** it migrates the schema gracefully (or warns if migration is not possible).

---

### Edge Cases

- What happens when an MCP server is unreachable during profile application? MCP tool discovery happens lazily at profile application time with a loading indicator. Unreachable servers are skipped with a warning, and profile application continues for all other servers.
- What happens when the profiles directory does not exist? cc-setup creates it on first run and writes the built-in profiles.
- What happens when a profile YAML file is malformed? The profile is skipped with an error message, other profiles remain available.
- What happens when the permission list is very long (many MCP servers with many tools)? The flat list with section headers supports scrolling and filtering via `/`.
- What happens when the user saves without changes? No write operation occurs, similar to the existing dirty-checking pattern.

## Requirements

### Functional Requirements

- **FR-001**: System MUST display a third tab labeled "Permissions" in the tab bar alongside "Servers" and "Plugins."
- **FR-002**: System MUST show the current permission mode and all current permissions from `settings.local.json` in a categorized flat list (Built-in Tools, Bash Patterns, MCP tools per enabled server).
- **FR-003**: Users MUST be able to toggle individual permissions on and off via the space key.
- **FR-004**: Users MUST be able to save permission changes to `settings.local.json` for the current scope.
- **FR-005**: System MUST load permission profiles from `~/.config/cc-setup/profiles/` (YAML files).
- **FR-006**: Users MUST be able to select a profile to populate the permission list as a one-time preset action.
- **FR-007**: System MUST visually distinguish profile-derived permissions (teal) from user-customized permissions (amber/yellow).
- **FR-008**: System MUST show a confirmation dialog when switching profiles would reset custom permission changes.
- **FR-009**: System MUST classify MCP tools using server-provided annotations (ReadOnlyHint, DestructiveHint) as primary source.
- **FR-010**: System MUST fall back to name-based heuristics when MCP tool annotations are absent, using configurable safe/unsafe prefix lists.
- **FR-011**: System MUST show classification indicators on MCP tools (`[read-only]`, `[destructive]`, `[heuristic]`).
- **FR-012**: Users MUST be able to add new permissions via a guided flow with category selection, syntax help, and curated suggestions.
- **FR-013**: Users MUST be able to delete permissions from the list. Deleting a permission removes it from the allow list (equivalent to unchecking). For profile-derived permissions, delete and uncheck behave identically.
- **FR-014**: System MUST support scope switching (user/project) on the Permissions tab, matching the existing scope toggle pattern.
- **FR-015**: System MUST show inherited user-scope permissions as dimmed read-only items when in project scope.
- **FR-016**: System MUST only show MCP tools for servers that are enabled in the current scope.
- **FR-017**: System MUST ship two built-in profiles: "Read-only YOLO" (non-destructive operations only, `acceptEdits` mode) and "Full YOLO" (all permissions, `bypassPermissions` mode).
- **FR-018**: System MUST update built-in profiles (underscore-prefixed files) on cc-setup upgrades without touching user-created profiles.
- **FR-019**: System MUST support filtering the permission list via the `/` key.
- **FR-020**: System MUST persist only expanded permission entries to `settings.local.json`, not profile references.
- **FR-021**: Users MUST be able to view and change the Claude Code permission mode (default, acceptEdits, auto, bypassPermissions) via the `m` key.
- **FR-022**: System MUST persist the permission mode to `settings.json` under `permissions.defaultMode` for the current scope.
- **FR-023**: Profiles MUST be able to define a permission mode that is applied alongside the permission entries when the profile is selected.
- **FR-024**: System MUST display a prominent warning when the permission mode is set to `bypassPermissions`, indicating that all safety checks are disabled.

### Key Entities

- **Permission**: A single allow entry (e.g., `Read`, `Bash(git:*)`, `mcp__server__tool`) that maps to Claude Code's `permissions.allow` list in `settings.local.json`.
- **Permission Mode**: Claude Code's execution mode that controls how permission checks are handled at runtime. Modes include `default` (ask for everything), `acceptEdits` (auto-approve file edits), `auto` (AI classifier decides), and `bypassPermissions` (skip all checks). Stored in `settings.json` under `permissions.defaultMode`.
- **Profile**: A YAML file defining a preset collection of permissions, MCP classification rules, and a permission mode. Has a schema version, name, description, and permission definitions.
- **Permission Source**: Tracks whether a permission was derived from a profile application or customized by the user, for visual distinction purposes. This is an in-memory concept, not persisted.
- **Tool Classification**: The determination of whether an MCP tool is safe or unsafe, based on annotations, heuristics, or explicit profile overrides.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can switch from no permissions to a full read-only permission set in under 10 seconds (select profile, save).
- **SC-002**: Users can view and understand all active permissions across built-in tools, bash, and MCP servers from a single screen.
- **SC-003**: Users can add a custom permission in under 30 seconds using the guided input flow.
- **SC-004**: Profile application correctly classifies 90%+ of MCP tools without manual intervention (for servers that provide annotations).
- **SC-005**: Switching between user and project scope on the Permissions tab takes under 2 seconds with inherited permissions clearly visible.
- **SC-006**: Built-in profile upgrades do not affect user-created profiles across cc-setup version updates.
- **SC-007**: Applying "Full YOLO" profile results in zero interactive permission prompts during a Claude Code session, including compound command safety checks.

## Clarifications

### Session 2026-04-16

- Q: What should "None" in the profile picker do to existing permissions? → A: "None" clears all permissions and resets mode to `default` (clean slate).
- Q: When should MCP tool discovery happen during profile application? → A: Lazily when a profile is applied (on demand), with a loading indicator and failed servers skipped with a warning.

## Assumptions

- Users have at most a handful of MCP servers enabled at a time (typical: 2-5), keeping the flat list manageable without collapsible sections.
- The existing `settings.local.json` format for `permissions.allow` and `permissions.deny` is stable and will not change in future Claude Code versions.
- YAML is an acceptable format for profile definitions since users are comfortable editing YAML files directly or via Claude.
- The existing MCP client library (`mcpclient.ListTools()`) can discover tools from all enabled servers reliably enough for profile application.
- Profile files are small enough that loading all profiles into memory at startup is not a performance concern.
- The existing tab switching, scope switching, and confirmation dialog patterns in the TUI are reusable for the Permissions tab without major refactoring.
