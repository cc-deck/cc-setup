# Implementation Plan: Permissions Subcommand

**Branch**: `002-permissions-subcommand` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/002-permissions-subcommand/spec.md`

## Summary

Add a `permissions` subcommand (alias: `perm`) to cc-setup with three headless subcommands (`apply`, `show`, `reset`) and a bare-command TUI shortcut. This replaces the existing `--full-yolo` flag with a proper command hierarchy that supports any profile, both scopes, and non-interactive use.

## Technical Context

**Language/Version**: Go 1.25.0
**Primary Dependencies**: cobra v1.10.2, bubbletea v1.3.6, bubbles v0.21.1, lipgloss v1.1.0
**Storage**: JSON config files (settings.local.json, settings.json)
**Testing**: `go test ./...`
**Target Platform**: macOS, Linux (CLI)
**Project Type**: CLI tool
**Performance Goals**: All headless commands complete in under 2 seconds
**Constraints**: Must reuse existing config read/write functions; no new dependencies
**Scale/Scope**: 3 new subcommands, 1 modified function, 2 removed items (flag + function)

## Constitution Check

*Constitution is a blank template. No gates to evaluate.*

## Project Structure

### Documentation (this feature)

```text
specs/002-permissions-subcommand/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (from /speckit-tasks)
```

### Source Code (repository root)

```text
cmd/
├── root.go              # MODIFY: remove --full-yolo flag, add permCmd
├── permissions.go       # MODIFY: remove runFullYolo(), add perm subcommands + helpers
└── manage.go            # MODIFY: add runManageWithTab() or tab parameter to runManage()

internal/config/
├── claude.go            # EXISTING: ReadAllPermissions, WriteAllPermissions, etc.
└── profiles.go          # EXISTING: LoadProfiles, EnsureBuiltinProfiles, Profile struct
```

**Structure Decision**: All new code lives in existing files. `cmd/permissions.go` already contains the permission-related logic. The new cobra commands and `resolveProfile`/`runPermShow`/`runPermReset` functions go there. No new files needed.

## Implementation Approach

### 1. Modify `runManage` to accept a starting tab (cmd/manage.go)

Add a `startTab` parameter to `runManage()`:
- Change signature from `func runManage() error` to `func runManageWithTab(startTab manageTab) error`
- Replace `tab := tabServers` with `tab := startTab`
- Add convenience wrapper: `func runManage() error { return runManageWithTab(tabServers) }`
- Update the root command's `RunE` to call `runManage()` (unchanged behavior)

### 2. Create `permCmd` and subcommands (cmd/permissions.go)

**Parent command** (`permCmd`):
- `Use: "permissions"`, `Aliases: []string{"perm"}`
- `RunE`: calls `runManageWithTab(tabPermissions)` for bare invocation
- `PersistentFlags`: `--scope` (string, default "project")
- Registered on `rootCmd` in `init()`

**Apply subcommand** (`permApplyCmd`):
- `Use: "apply <profile>"`, `Args: cobra.ExactArgs(1)`
- Calls `resolveProfile(args[0])` to find the profile
- Builds permission state from profile (same logic as current `runFullYolo` but generalized)
- Calls `runSavePermissions(states, sources, profile.Mode, scope)`

**Show subcommand** (`permShowCmd`):
- `Use: "show"`
- Reads permissions via `config.ReadAllPermissions(scope)`, `config.ReadDenyPermissions(scope)`, `config.ReadPermissionMode(scope)`
- Groups entries by category (builtin/bash/mcp) using `categorizePermission()`
- Prints formatted output with mode, grouped entries, and file paths

**Reset subcommand** (`permResetCmd`):
- `Use: "reset"`
- Calls `config.WriteAllPermissions(scope, []string{}, []string{})` to clear allow/deny
- Calls `config.WritePermissionMode(scope, "")` to clear mode
- Prints confirmation with file paths

### 3. Add `resolveProfile` helper (cmd/permissions.go)

```
func resolveProfile(name string) (*config.Profile, error)
```

1. `EnsureBuiltinProfiles()`
2. Check `profileSlugs` map (case-insensitive via `strings.EqualFold`)
3. If slug match found, use the mapped profile name
4. Load profiles via `config.LoadProfiles()`
5. Find profile by name (exact match)
6. If not found, return error listing available profiles

### 4. Remove old `--full-yolo` infrastructure (cmd/root.go, cmd/permissions.go)

- Remove `var fullYolo bool` and `rootCmd.Flags().BoolVar(...)` from `cmd/root.go`
- Remove `runFullYolo()` from `cmd/permissions.go`
- Remove the `if fullYolo` check from root command's `RunE`

### 5. Build profile application from resolveProfile (cmd/permissions.go)

Extract the permission-state-building logic from `runFullYolo()` into a reusable `buildProfileState(profile *config.Profile)` function that returns `(states map[string]permState, sources map[string]string)`. This is called by `permApplyCmd`.

## Complexity Tracking

No constitution violations to justify.
