# Brainstorm: Permissions Subcommand

**Date:** 2026-07-09
**Status:** active

## Problem Framing

The `--full-yolo` flag on the root command is a one-off action that bypasses the TUI entirely. It feels tacked on and doesn't scale to other permission operations. The CLI needs a proper `permissions` subcommand that supports both interactive (TUI) and headless use cases for managing Claude Code permission profiles.

## Approaches Considered

### A: Flat subcommands under `perm`

`perm apply`, `perm show`, `perm reset` as subcommands. Bare `perm` shows help.

- Pros: Simple, discoverable, each command does one thing
- Cons: `perm apply yolo` is 3 words for the most common operation; `perm` alone does nothing useful

### B: `perm` opens TUI, subcommands for headless

Bare `perm` opens the TUI directly on the Permissions tab. Subcommands (`apply`, `show`, `reset`) are headless equivalents for scripting and automation.

- Pros: TUI shortcut is natural, consistent with root `cc-setup` launching TUI, clean interactive/headless separation
- Cons: Slightly more complex routing (bare command vs subcommand)

### C: Mixed shortcuts and verbs

Built-in profiles get direct shortcuts (`perm yolo`), custom profiles use `apply`. Mixed noun/verb subcommands.

- Pros: `cc-setup perm yolo` is the fastest path
- Cons: Inconsistent subcommand styles (verbs vs nouns), confusing whether `yolo` is a subcommand or argument

## Decision

**Approach B: `perm` opens TUI, subcommands for headless.**

Cleanest separation of concerns. Bare command = interactive, subcommands = headless/scriptable. Consistent with the root command pattern where bare `cc-setup` opens the TUI.

## Key Requirements

- New `permissions` subcommand with `perm` alias
- `cc-setup perm` (bare): opens TUI directly on Permissions tab
- `cc-setup perm apply <profile>`: headless profile application with slug matching (`yolo` -> "Full YOLO", `readonly` -> "Read-Only YOLO", exact name for custom)
- `cc-setup perm show`: display current permissions, mode, and source files
- `cc-setup perm reset`: clear all permissions back to defaults
- `--scope project|user` flag on all subcommands (default: `project`)
- Remove the `--full-yolo` flag from the root command

## Open Questions

- `perm show` output format: table? JSON option? What exactly to display?
- `perm reset`: should it require confirmation, or is `--force` sufficient?
- Should `perm apply` print what it did (like `runSavePermissions` currently does), or be silent?
