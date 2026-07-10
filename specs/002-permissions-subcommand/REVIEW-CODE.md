# Code Review: Permissions Subcommand

**Branch**: `002-permissions-subcommand`
**Date**: 2026-07-09
**Reviewer**: Ship pipeline (automated)

## Spec Compliance Check

**Score: 16/16 (100%)**

| Requirement | Status | Evidence |
|-------------|--------|----------|
| FR-001: `permissions` cmd + `perm` alias | PASS | `permCmd` with `Aliases: []string{"perm"}` in cmd/permissions.go:24-26 |
| FR-002: bare `perm` opens TUI on Permissions tab | PASS | `RunE` calls `runManageWithTab(tabPermissions)` in cmd/permissions.go:32-34 |
| FR-003: apply with slug matching | PASS | `resolveProfile()` checks `profileSlugs` map case-insensitively, falls back to exact match (cmd/permissions.go:272-323) |
| FR-004: write via same TUI logic | PASS | `permApplyCmd` calls `runSavePermissions()` (cmd/permissions.go:105) |
| FR-005: MCP wildcard expansion | PASS | `buildProfileState()` expands `mcp__<server>__*` for global wildcard profiles (cmd/permissions.go:1012-1020) |
| FR-006: auto-consolidation | PASS | Via `runSavePermissions` which calls `consolidatePermissions()` (cmd/permissions.go:938) |
| FR-007: accept trust dialog for project | PASS | Via `runSavePermissions` which calls `AcceptTrustDialog()` for project scope (cmd/permissions.go:952) |
| FR-008: print summary | PASS | Via `runSavePermissions` output (entry count, paths, mode, consolidated entries) |
| FR-009: show with grouped output | PASS | `permShowCmd` groups by builtin/bash/mcp using `categorizePermission()` (cmd/permissions.go:139-175) |
| FR-010: reset clears permissions | PASS | `permResetCmd` calls `WriteAllPermissions(scope, [], [])` and `WritePermissionMode(scope, "")` (cmd/permissions.go:220-228) |
| FR-011: --scope flag on all subcommands | PASS | `PersistentFlags` on `permCmd` with validation in `PersistentPreRunE` (cmd/permissions.go:35-43, 256) |
| FR-012: remove --full-yolo flag | PASS | `var fullYolo bool` and `Flags().BoolVar()` removed from cmd/root.go |
| FR-013: remove runFullYolo() | PASS | Function removed, replaced by `buildProfileState()` (cmd/permissions.go:993-1025) |
| FR-014: no-arg error lists profiles | PASS | Custom args handling in `permApplyCmd.RunE` (cmd/permissions.go:83-86) |
| FR-015: bad profile error lists profiles | PASS | `resolveProfile()` returns error with available profiles list (cmd/permissions.go:302-322) |
| FR-016: auto-create built-in profiles | PASS | `resolveProfile()` calls `EnsureBuiltinProfiles()` first (cmd/permissions.go:273) |

**Gate Outcome: PASS**

## Deep Review Report

### Agent 1: Correctness Review

**Finding count**: 0 critical, 0 important

The implementation correctly:
- Reuses `runSavePermissions` for apply (same code path as TUI save)
- Reuses `buildProfileState` extracted from the former `runFullYolo()`
- Handles empty permissions gracefully in `show` and `reset`
- Validates scope with `PersistentPreRunE`
- Handles all edge cases (no args, bad profile name, nothing to reset)

### Agent 2: Architecture Review

**Finding count**: 0 critical, 0 important

Clean design:
- All new code in existing files (no file proliferation)
- `permCmd` as parent with subcommands follows cobra best practices
- `PersistentFlags` for `--scope` propagates to all subcommands correctly
- `runManageWithTab` cleanly extends `runManage` with backward compatibility
- `resolveProfile` and `buildProfileState` are properly separated helpers

### Agent 3: Security Review

**Finding count**: 0 critical, 0 important

No security concerns:
- Only reads/writes local config files (same as existing TUI)
- No user input used in file paths (scope is validated to project|user)
- No command injection vectors
- Trust dialog acceptance is properly gated on project scope

### Agent 4: Production Readiness Review

**Finding count**: 0 critical, 1 minor (informational)

- Minor: `listAvailableProfiles()` iterates over a map (non-deterministic order for slug listing). Mitigated by `sort.Strings(available)` call. No action needed.

### Agent 5: Test Coverage Review

**Finding count**: 0 critical, 1 informational

- No unit tests were added (not requested in spec). The code is validated by `go build` and manual end-to-end testing (T008, T014 in tasks.md).

### Summary

| Dimension | Critical | Important | Minor |
|-----------|----------|-----------|-------|
| Correctness | 0 | 0 | 0 |
| Architecture | 0 | 0 | 0 |
| Security | 0 | 0 | 0 |
| Production | 0 | 0 | 1 |
| Tests | 0 | 0 | 1 |
| **Total** | **0** | **0** | **2** |

**Verdict**: PASS. No critical or important findings. 2 minor informational notes, no action required.
