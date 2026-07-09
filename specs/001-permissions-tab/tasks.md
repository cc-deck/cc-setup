# Tasks: Permissions Tab

**Input**: Design documents from `/specs/001-permissions-tab/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add new dependency and create foundational data types

- [x] T001 Add `gopkg.in/yaml.v3` dependency to go.mod via `go get gopkg.in/yaml.v3`
- [x] T002 [P] Define Profile, ProfilePermissions, ProfileMCP, ProfileHeuristic, and ClassifiedTool structs with YAML/JSON tags in internal/config/profiles.go
- [x] T003 [P] Define permissionItem struct (key, category, serverName, hint, source) in cmd/permissions.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Config reading/writing functions that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Implement `ReadAllPermissions(scope)` to read full `permissions.allow` array from settings.local.json in internal/config/claude.go
- [x] T005 Implement `WriteAllPermissions(scope, permissions)` to write full permissions.allow array (preserving other keys) in internal/config/claude.go
- [x] T006 [P] Implement `ReadPermissionMode(scope)` to read `permissions.defaultMode` from settings.json in internal/config/claude.go
- [x] T007 [P] Implement `WritePermissionMode(scope, mode)` to write `permissions.defaultMode` to settings.json (preserving other keys) in internal/config/claude.go
- [x] T008 Implement `categorizePermission(key)` function returning category (builtin/bash/mcp) and serverName in cmd/permissions.go
- [x] T009 Add permission-specific display styles (amber for custom, teal for profile, dimmed for inherited) in internal/display/display.go

**Checkpoint**: Foundation ready, user story implementation can begin

---

## Phase 3: User Story 1 - View and Toggle Permissions (Priority: P1) MVP

**Goal**: Users can open a Permissions tab, see all current permissions grouped by category, toggle them on/off, change the permission mode, and save changes.

**Independent Test**: Open cc-setup, switch to the Permissions tab, toggle a permission, save, verify the change appears in `settings.local.json`.

### Implementation for User Story 1

- [x] T010 [US1] Add `tabPermissions` enum value to manageTab const block and extend tab count in cmd/manage.go
- [x] T011 [US1] Define `permissionsKeyMap` struct with key bindings (space, a, d, p, m, s, u/P, /) in cmd/permissions.go
- [x] T012 [US1] Define `permissionCheckboxDelegate` implementing list.ItemDelegate with source-based coloring in cmd/permissions.go
- [x] T013 [US1] Implement `buildPermissionItems(scope)` to construct list items from ReadAllPermissions + enabled MCP servers in cmd/permissions.go
- [x] T014 [US1] Add permissions fields to `manageModel` struct (permList, permChecked, permSource, permMode, permDirty, permScope) in cmd/manage.go
- [x] T015 [US1] Initialize permissions list in `manageModel` Init/constructor, loading permissions for default scope in cmd/manage.go
- [x] T016 [US1] Extend tab switching logic (tab key handler) to cycle through three tabs (Servers, Plugins, Permissions) in cmd/manage.go
- [x] T017 [US1] Update banner rendering to display three tab pills instead of two in cmd/manage.go
- [x] T018 [US1] Implement permissions View() rendering (mode display at top, categorized list with section headers) in cmd/permissions.go
- [x] T019 [US1] Implement space/x key handler for toggling permission checked state and marking source as "custom" in cmd/permissions.go
- [x] T020 [US1] Route Update() and View() to permissions handlers when tab == tabPermissions in cmd/manage.go
- [x] T021 [US1] Refactor scope switching in manage.go Update() to be tab-aware: when tab == tabPermissions, skip `p` for ScopeProject (freeing it for profile picker) and call permissions-specific reload instead of reloadCheckedState/reloadPluginCheckedState in cmd/manage.go
- [x] T022 [US1] Implement `reloadPermissions()` method on manageModel to clear and reload permission list, checked state, and sources from disk for current scope in cmd/permissions.go
- [x] T023 [US1] Implement mode picker using huh form (default, acceptEdits, auto, bypassPermissions) triggered by `m` key in cmd/permissions.go
- [x] T024 [US1] Implement `d` key handler to delete/remove a permission from the list and checked map in cmd/permissions.go
- [x] T025 [US1] Implement save flow (`s` key): write permissions via WriteAllPermissions, write mode via WritePermissionMode, sync autoApprove via UpdateAutoApprove in cmd/permissions.go
- [x] T026 [US1] Implement dirty checking (compare current state to initial state) and quit confirmation dialog with unsaved changes summary in cmd/permissions.go

**Checkpoint**: User Story 1 is fully functional and testable independently

---

## Phase 4: User Story 2 - Apply a Profile to Initialize Permissions (Priority: P1)

**Goal**: Users can select a profile from a picker to populate the permission list with preset permissions and mode, then fine-tune before saving.

**Independent Test**: Open Permissions tab, press `p`, select "Read-only YOLO", verify permissions and mode are populated, optionally adjust, save.

### Implementation for User Story 2

- [x] T027 [US2] Embed built-in profile YAML content for `_readonly-yolo.yaml` and `_full-yolo.yaml` as Go constants in internal/config/profiles.go
- [x] T028 [US2] Implement `EnsureBuiltinProfiles()` to write/overwrite underscore-prefixed files to `~/.config/cc-setup/profiles/` (creating directory via os.MkdirAll if needed) in internal/config/profiles.go
- [x] T029 [US2] Implement `LoadProfiles()` to scan profiles directory, parse all .yaml files, return slice of Profile in internal/config/profiles.go
- [x] T030 [US2] Implement `LoadProfile(name)` to load a single profile by name in internal/config/profiles.go
- [x] T031 [US2] Implement profile picker using huh form (list of profiles + "None") in cmd/permissions.go
- [x] T032 [US2] Implement confirmation dialog when custom changes exist ("Reset N changes? [Apply] [Cancel]") in cmd/permissions.go
- [x] T033 [US2] Implement profile application logic: clear permissions, set mode, add allow entries, add bash patterns as `Bash(<pattern>)`, mark source as "profile" in cmd/permissions.go
- [x] T034 [US2] Implement `p` key handler to exit TUI, run profile picker, apply selection, re-enter TUI in cmd/permissions.go
- [x] T035 [US2] Display bypassPermissions warning banner when mode is set to bypassPermissions in cmd/permissions.go
- [x] T036 [US2] Call `EnsureBuiltinProfiles()` during manage command initialization in cmd/manage.go

**Checkpoint**: User Story 2 is fully functional and testable independently

---

## Phase 5: User Story 4 - MCP Tool Classification During Profile Application (Priority: P2)

**Goal**: MCP tools are automatically classified as safe/unsafe based on annotations and heuristics during profile application.

**Independent Test**: Apply "Read-only YOLO" with an MCP server having annotated tools, verify classification indicators.

### Implementation for User Story 4

- [x] T037 [US4] Implement `ClassifyTool(tool, profile)` returning (approved bool, hint string) with include/exclude > annotations > heuristic > default deny logic in internal/config/profiles.go
- [x] T038 [US4] Implement `ClassifyServerTools(tools, profile)` returning []ClassifiedTool in internal/config/profiles.go
- [x] T039 [US4] Integrate MCP tool discovery into profile application flow: discover tools from enabled servers (reuse toolCache), classify, add approved tools as `mcp__<server>__<tool>` in cmd/permissions.go
- [x] T040 [US4] Add loading indicator during MCP tool discovery and handle unreachable servers (skip with warning) in cmd/permissions.go
- [x] T041 [US4] Display classification indicators ([read-only], [destructive], [heuristic]) on MCP tool items in cmd/permissions.go

**Checkpoint**: MCP classification working during profile application

---

## Phase 6: User Story 3 - Add New Permissions with Guided Input (Priority: P2)

**Goal**: Users can add custom permissions via a guided flow with category selection, syntax help, and suggestions.

**Independent Test**: Press `a`, select "Bash pattern", see suggestions, type a pattern, confirm, verify it appears in the list.

### Implementation for User Story 3

- [x] T042 [US3] Implement category selector huh form (Built-in tool, Bash pattern, MCP tool) triggered by `a` key in cmd/permissions.go
- [x] T043 [US3] Implement built-in tool selector (list of known tools: Read, Write, Edit, Glob, Grep, WebFetch, WebSearch, Bash) in cmd/permissions.go
- [x] T044 [US3] Implement bash pattern input with curated suggestions list (git:*, npm test, cargo test:*, kubectl:*, make:*) and syntax help in cmd/permissions.go
- [x] T045 [US3] Implement MCP tool add flow: server selector then tool discovery and selection for the chosen server in cmd/permissions.go
- [x] T046 [US3] Mark added permissions as "custom" source with amber visual treatment in cmd/permissions.go

**Checkpoint**: Guided add flow working for all three categories

---

## Phase 7: User Story 5 - Scope Switching on Permissions Tab (Priority: P2)

**Goal**: Users can switch between user and project scope, with inherited permissions shown as dimmed read-only items.

**Independent Test**: Open Permissions tab in user scope, switch to project scope, verify inherited items appear dimmed, add project-specific permission, save, verify it lands in project-scoped settings.

### Implementation for User Story 5

- [x] T047 [US5] Implement scope switching (`u`/`.` key handler) to call reloadPermissions() for new scope in cmd/permissions.go
- [x] T048 [US5] In project scope, load user-scope permissions as read-only "inherited" items (dimmed, not toggleable) in cmd/permissions.go
- [x] T049 [US5] Implement override logic: toggling an inherited item creates a project-scope entry marked "custom" in cmd/permissions.go

**Checkpoint**: Scope switching with inherited permissions working

---

## Phase 8: User Story 6 - Built-in Profile Management Across Upgrades (Priority: P3)

**Goal**: Built-in profiles (underscore-prefixed) are updated on cc-setup upgrades without affecting user-created profiles.

**Independent Test**: Place modified `_readonly-yolo.yaml`, run cc-setup, verify overwritten. Place custom profile, verify untouched.

### Implementation for User Story 6

- [x] T050 [US6] Add version comparison logic to `EnsureBuiltinProfiles()` to overwrite outdated built-in profiles in internal/config/profiles.go
- [x] T051 [US6] Handle schema_version migration for older user profiles (warn if migration not possible) in internal/config/profiles.go

**Checkpoint**: Profile upgrade logic complete

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Visual polish, edge cases, and integration validation

- [x] T052 [P] Implement `/` filter support using bubbletea list built-in filtering in cmd/permissions.go
- [x] T053 Verify JSON key preservation when writing settings.local.json and settings.json (deny, ask, disabledMcpjsonServers, enabledPlugins)
- [x] T054 Handle edge cases: malformed profiles (skip with error), empty permission state, no MCP servers enabled
- [x] T055 Verify tab navigation works correctly across all three tabs with proper focus management

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001 for yaml.v3)
- **US1 (Phase 3)**: Depends on Phase 2 completion
- **US2 (Phase 4)**: Depends on US1 (tab-aware scope handler and permissions TUI must exist)
- **US4 (Phase 5)**: Depends on US2 (profile application flow exists)
- **US3 (Phase 6)**: Depends on US1 (permissions list exists)
- **US5 (Phase 7)**: Depends on US1 (reloadPermissions and basic scope exist)
- **US6 (Phase 8)**: Depends on US2 (profile management exists)
- **Polish (Phase 9)**: Depends on all desired user stories

### Within Each User Story

- Config/model code before TUI code
- TUI structure before key handlers
- Key handlers before save/persistence

### Parallel Opportunities

- T002 and T003 can run in parallel (different files)
- T004 and T006 can run in parallel (read settings.local.json / read settings.json)
- T005 and T007 can run in parallel (write settings.local.json / write settings.json)
- US3 and US5 can start in parallel after US1 completes (different concerns)

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup (add yaml.v3, define structs)
2. Complete Phase 2: Foundational (config read/write, styles)
3. Complete Phase 3: US1 (view, toggle, save, mode, tab-aware scope handling)
4. Complete Phase 4: US2 (profiles, application, bypassPermissions warning)
5. **STOP and VALIDATE**: Test view/toggle/profile/save flow end-to-end

### Incremental Delivery

1. Setup + Foundational -> Core config layer ready
2. US1 -> View, toggle, save, mode selection (MVP foundation)
3. US2 -> Profile support (key differentiator)
4. US4 -> MCP classification (makes profiles practical)
5. US3 -> Guided add flow (full customization)
6. US5 -> Scope switching (user/project)
7. US6 -> Upgrade management (long-term)
8. Polish -> Edge cases, filter support

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story should be independently testable after its phase completes
- Commit after each task or logical group
- Existing patterns to reference: cmd/plugins.go (tab switching), cmd/tools.go (MCP discovery), internal/config/plugins.go (settings.json read/write)
- Key binding note: `p` is profile picker in permissions tab, scope uses `.` (toggle) and `u` (user). The tab-aware scope handler (T021) ensures `p` is not intercepted by the global ScopeProject handler when permissions tab is active.
