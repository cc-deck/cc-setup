# Tasks: Permissions Subcommand

**Input**: Design documents from `specs/002-permissions-subcommand/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Not explicitly requested. Test tasks omitted.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Shared infrastructure and command hierarchy

- [x] T001 Add `runManageWithTab(startTab manageTab)` function to cmd/manage.go, wrapping existing `runManage()` to accept a starting tab parameter. Update `runManage()` to call `runManageWithTab(tabServers)`.
- [x] T002 Add `profileSlugs` map and `resolveProfile(name string) (*config.Profile, error)` function to cmd/permissions.go. The map contains `{"yolo": "Full YOLO", "readonly": "Read-only YOLO"}`. The function calls `EnsureBuiltinProfiles()`, checks slugs case-insensitively, falls back to exact name match via `LoadProfiles()`, and returns an error listing available profiles if no match.
- [x] T003 Add `buildProfileState(profile *config.Profile) (map[string]permState, map[string]string)` function to cmd/permissions.go. Extract the permission-state-building logic from the existing `runFullYolo()` function: initialize builtin tools as `permAsk`, add profile allow entries, add bash patterns, expand MCP wildcards for configured servers.
- [x] T004 Create `permCmd` cobra command in cmd/permissions.go with `Use: "permissions"`, `Aliases: []string{"perm"}`, `Short: "Manage Claude Code permissions"`. Add `PersistentFlags` for `--scope` (string, default "project"). Add a `PersistentPreRunE` on `permCmd` that validates `--scope` is either `"project"` or `"user"`, returning an error listing valid values otherwise. Register `permCmd` on `rootCmd` in `init()` in cmd/root.go.

**Checkpoint**: Foundation ready. `permCmd` registered, shared helpers available.

---

## Phase 2: User Story 1 - Apply a Permission Profile Headlessly (Priority: P1)

**Goal**: Users can run `cc-setup perm apply yolo` to apply any profile headlessly.

**Independent Test**: Run `cc-setup perm apply yolo`, verify `settings.local.json` and `settings.json` contain expected permissions and mode.

### Implementation for User Story 1

- [x] T005 [US1] Create `permApplyCmd` cobra command in cmd/permissions.go with `Use: "apply <profile>"`. Use a custom `Args` validator (not `cobra.ExactArgs(1)`) that, when no argument is provided, calls `EnsureBuiltinProfiles()` and `LoadProfiles()` to list available profiles in the error message (FR-014). The `RunE` calls `resolveProfile(args[0])`, then `buildProfileState(profile)`, then `runSavePermissions(states, sources, profile.Mode, scope)` where scope comes from the `--scope` flag. Register as subcommand of `permCmd`.
- [x] T006 [US1] Remove `--full-yolo` flag: delete `var fullYolo bool`, remove `rootCmd.Flags().BoolVar(...)` from `init()`, and remove the `if fullYolo` branch from `rootCmd.RunE` in cmd/root.go.
- [x] T007 [US1] Remove `runFullYolo()` function from cmd/permissions.go (its logic is now in `buildProfileState` + `permApplyCmd`).
- [x] T008 [US1] Verify `go build ./...` succeeds and test `cc-setup perm apply yolo` end-to-end.

**Checkpoint**: `cc-setup perm apply <profile>` works with slug matching, scope flag, and summary output.

---

## Phase 3: User Story 2 - Show Current Permissions (Priority: P2)

**Goal**: Users can run `cc-setup perm show` to see current permission state.

**Independent Test**: Configure known permissions, run `cc-setup perm show`, verify output shows correct mode, tools, and file paths.

### Implementation for User Story 2

- [x] T009 [P] [US2] Create `permShowCmd` cobra command in cmd/permissions.go with `Use: "show"`. The `RunE` reads permissions via `config.ReadAllPermissions(scope)`, `config.ReadDenyPermissions(scope)`, and `config.ReadPermissionMode(scope)`. Groups allowed entries by category using `categorizePermission()`. Prints formatted output: permission mode, allowed tools (grouped by builtin/bash/mcp), denied tools, and file paths from `config.SettingsPath(scope)` and `config.PluginSettingsPath(scope)`. Register as subcommand of `permCmd`.

**Checkpoint**: `cc-setup perm show` displays current permission state.

---

## Phase 4: User Story 3 - Open TUI on Permissions Tab (Priority: P2)

**Goal**: Running `cc-setup perm` (bare) opens the TUI directly on the Permissions tab.

**Independent Test**: Run `cc-setup perm` and verify the TUI launches with the Permissions tab active.

### Implementation for User Story 3

- [x] T010 [P] [US3] Set `permCmd.RunE` to call `runManageWithTab(tabPermissions)` in cmd/permissions.go. This handles the bare `cc-setup perm` invocation (no subcommand) by opening the TUI on the Permissions tab.

**Checkpoint**: Bare `cc-setup perm` opens TUI on Permissions tab.

---

## Phase 5: User Story 4 - Reset Permissions to Defaults (Priority: P3)

**Goal**: Users can run `cc-setup perm reset` to clear all permission configuration.

**Independent Test**: Configure permissions, run `cc-setup perm reset`, verify config files no longer contain permission entries.

### Implementation for User Story 4

- [x] T011 [P] [US4] Create `permResetCmd` cobra command in cmd/permissions.go with `Use: "reset"`. The `RunE` reads current permissions to check if anything exists (for "nothing to reset" message). If permissions exist, calls `config.WriteAllPermissions(scope, []string{}, []string{})` and `config.WritePermissionMode(scope, "")`. Prints confirmation with file paths. Register as subcommand of `permCmd`.

**Checkpoint**: `cc-setup perm reset` clears permissions while preserving other settings.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup

- [x] T012 Verify `go build ./...` compiles cleanly with no warnings.
- [x] T013 Run `cc-setup perm --help` and verify subcommands (`apply`, `show`, `reset`) appear with correct descriptions and the `perm` alias works.
- [x] T014 End-to-end validation: run `cc-setup perm apply yolo`, then `cc-setup perm show`, then `cc-setup perm reset`, then `cc-setup perm show` again to verify the full cycle works.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, can start immediately
- **US1 (Phase 2)**: Depends on T001-T004 (Setup)
- **US2 (Phase 3)**: Depends on T004 (permCmd registration)
- **US3 (Phase 4)**: Depends on T001 (runManageWithTab), T004 (permCmd)
- **US4 (Phase 5)**: Depends on T004 (permCmd registration)
- **Polish (Phase 6)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Requires all Setup tasks. Core feature.
- **US2 (P2)**: Requires T004 only. Independent of US1.
- **US3 (P2)**: Requires T001 and T004. Independent of US1, US2.
- **US4 (P3)**: Requires T004 only. Independent of US1, US2, US3.

### Parallel Opportunities

- T009 (US2), T010 (US3), T011 (US4) can all run in parallel after Setup phase
- T006 and T007 can run in parallel (both are removals)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: US1 Apply (T005-T008)
3. **STOP and VALIDATE**: Test `cc-setup perm apply yolo` independently
4. The old `--full-yolo` is removed, new `perm apply` replaces it

### Incremental Delivery

1. Setup → Foundation ready
2. US1 Apply → Core headless feature works (MVP)
3. US2 Show → Users can inspect state
4. US3 TUI shortcut → Convenient interactive access
5. US4 Reset → Full lifecycle management
6. Polish → Verified end-to-end

---

## Notes

- [P] tasks = different files or independent functions, no dependencies
- All new code goes in existing files (cmd/permissions.go, cmd/root.go, cmd/manage.go)
- No new dependencies required
- Commit after each task or logical group
