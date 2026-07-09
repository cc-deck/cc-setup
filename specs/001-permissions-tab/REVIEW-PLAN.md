# Plan Review: Permissions Tab

**Reviewed**: 2026-04-16 | **Spec**: spec.md | **Plan**: plan.md | **Tasks**: tasks.md

## Verdict: PASS (revised)

All five issues from the initial review have been addressed. The plan has solid coverage (24/24 FRs, 26/26 acceptance scenarios) and the critical/important gaps have been fixed.

---

## 1. Coverage Matrix

### Functional Requirements

| FR | Description | Task(s) | Status |
|----|-------------|---------|--------|
| FR-001 | Third "Permissions" tab | T010, T016, T017 | COVERED |
| FR-002 | Show mode + categorized permissions | T013, T018 | COVERED |
| FR-003 | Toggle via space | T019 | COVERED |
| FR-004 | Save to settings.local.json | T032 | COVERED |
| FR-005 | Load profiles from profiles dir | T023 | COVERED |
| FR-006 | Select profile to populate | T025, T027, T028 | COVERED |
| FR-007 | Visual distinction profile/custom | T009, T012 | COVERED |
| FR-008 | Confirmation on profile switch | T026 | COVERED |
| FR-009 | MCP classification via annotations | T035 | COVERED |
| FR-010 | Heuristic fallback | T035 | COVERED |
| FR-011 | Classification indicators | T039 | COVERED |
| FR-012 | Guided add flow | T040-T044 | COVERED |
| FR-013 | Delete permissions | T034 | COVERED |
| FR-014 | Scope switching | T045 | COVERED |
| FR-015 | Inherited dimmed read-only | T046 | COVERED |
| FR-016 | MCP tools only for enabled servers | T013, T037 | COVERED |
| FR-017 | Two built-in profiles | T021 | COVERED |
| FR-018 | Update built-in on upgrades | T048 | COVERED |
| FR-019 | Filter via / | T051 | COVERED |
| FR-020 | Persist expanded entries | T032 | COVERED |
| FR-021 | View/change mode via m | T031 | COVERED |
| FR-022 | Persist mode to settings.json | T032 | COVERED |
| FR-023 | Profiles define mode | T027 | COVERED |
| FR-024 | bypassPermissions warning | T029 | COVERED |

**Result**: 24/24 FRs covered.

### Acceptance Scenarios: 26/26 covered (all user stories mapped to tasks).

---

## 2. Red Flags

### CRITICAL #1: Key Binding Conflict (`p` key)

**Issue**: The plan assigns `p` to the profile picker in the permissions tab (T028). But `p` is already bound to `ScopeProject` as a **global** key handler in `cmd/manage.go:223`:

```go
ScopeProject: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "project scope")),
```

The scope switching handler runs globally BEFORE tab-specific handlers (`manage.go:583-609`). Pressing `p` in the permissions tab would trigger scope switching to project, not the profile picker. The profile picker would never be reachable.

**Impact**: US2 is completely broken. Profile selection cannot work as designed.

**Fix options**:
- (a) Restructure the global key handler to be tab-aware: when `tab == tabPermissions`, skip `p` for scope and let it reach the permissions handler
- (b) In the permissions tab, use `.` (already bound to ScopeToggle) for scope switching and free `p` for profiles
- (c) Use a different key for profiles (e.g., capital `P`)

**Recommendation**: Option (a). Add a task to refactor the scope switching block in `manage.go` Update() to check the active tab before matching `p`. This preserves the existing key semantics for servers/plugins while allowing `p` for profiles in the permissions tab.

### CRITICAL #2: US1 Checkpoint is False

**Issue**: Phase 3 checkpoint says "User Story 1 is fully functional and testable independently" but the save flow (T032), mode picker (T031), dirty checking (T033), and delete (T034) are in Phase 5. Without save, US1 cannot be tested because changes cannot be persisted.

**Impact**: Following the MVP strategy of "STOP and VALIDATE" at Phase 3 would fail since there's no way to verify changes landed in the settings file.

**Fix**: Move T031-T034 into Phase 3, making US1 genuinely complete at that checkpoint.

### IMPORTANT #1: Missing Global Scope Handler Refactor Task

**Issue**: The current `manage.go` Update() handles scope switching globally (lines 583-609) calling `reloadCheckedState()` and `reloadPluginCheckedState()`. For the permissions tab, scope switching needs different behavior: reload permissions from disk for the new scope, load inherited items, etc. No task addresses this architectural change.

**Impact**: Scope switching in the permissions tab would call server/plugin reload functions instead of permission-specific reload, causing the permissions list to not update.

**Fix**: Add a task in Phase 3 to refactor the global scope handler to be tab-aware, routing to a permissions-specific reload when `tab == tabPermissions`.

### IMPORTANT #2: Duplicate Task (T018 vs T050)

**Issue**: T018 says "Implement permissions View() rendering (mode display at top, categorized list with section headers)" while T050 in Polish says "Add section headers (Built-in Tools, Bash Patterns, MCP: servername) to permissions list rendering." These describe the same work.

**Fix**: Remove T050 (section headers are part of the core View implementation in T018) or clarify that T018 is a basic list and T050 adds grouping as polish.

### IMPORTANT #3: Missing ClassifiedTool Struct Definition

**Issue**: T035 and T036 reference a `ClassifiedTool` struct, but no task defines it. T002 defines Profile/ProfilePermissions/ProfileMCP/ProfileHeuristic but not ClassifiedTool.

**Fix**: Expand T002 to include ClassifiedTool struct, or add a setup task before T035.

---

## 3. Task Quality

### Format Compliance

All 54 tasks follow the required `- [ ] [TaskID] [P?] [Story?] Description with file path` format. No violations.

### Parallel Marking

Existing [P] marks are correct. Additional parallel candidates:
- T004/T006 (read settings.local.json / read settings.json, different files)
- T005/T007 (write settings.local.json / write settings.json, different files)

### Story Label Consistency

Phase 5 tasks labeled [US1] but serve both US1 and US2. Minor inconsistency.

---

## 4. Dependency Validation

All claimed dependencies are valid. One observation: the dependency "US2 runs after US1 (shares manage.go)" could be relaxed. US2's profile loading code (T021-T024) lives in internal/config/profiles.go and could start in parallel with US1's TUI work, with only the TUI integration tasks (T025-T030) needing US1 to complete first.

---

## 5. Action Items (Priority Order)

### Must Fix Before Implementation

1. **Add task**: Refactor scope key handling in `manage.go` Update() to be tab-aware. When `tab == tabPermissions`, `p` routes to profile picker, not scope. Scope uses `.` (toggle) and can still use `u` for user scope.
2. **Move T031-T034** from Phase 5 into Phase 3 so US1 is genuinely complete at its checkpoint.
3. **Expand T002**: Add `ClassifiedTool` struct definition.
4. **Add task**: Permissions-specific scope reload function (instead of `reloadCheckedState`/`reloadPluginCheckedState`).
5. **Resolve T018/T050** duplication.

### Nice to Have

6. Mark T004/T006 and T005/T007 as [P] (parallel-safe).
7. Update Phase 5 task labels from [US1] to [US1/US2].
8. Make T022 explicit about `os.MkdirAll` for profiles directory creation.

---

## 6. Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Key binding conflict breaks profile selection | Critical | Refactor scope handler to be tab-aware |
| Phase 3 checkpoint untestable without save | Critical | Move save tasks into Phase 3 |
| Two JSON files on save (atomicity) | Medium | Existing WriteToolPermissions pattern shows how. Both writes are fast, low risk of partial state. |
| MCP discovery during profile application | Medium | Reuse toolCache. T038 handles unreachable servers. |
| manage.go growing too complex | Low | Most logic in new cmd/permissions.go. Only routing and model fields added to manage.go. |
