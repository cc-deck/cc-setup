package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/cc-deck/cc-setup/internal/config"
	"github.com/cc-deck/cc-setup/internal/display"
	mcpclient "github.com/cc-deck/cc-setup/internal/mcp"
)

// permState represents the three possible states of a permission.
type permState int

const (
	permAsk   permState = 0 // not mentioned: Claude asks interactively
	permAllow permState = 1 // in allow list: auto-approved
	permDeny  permState = 2 // in deny list: auto-rejected
)

// allBuiltinTools is the complete list of Claude Code built-in tools.
// These are always visible in the permissions list regardless of state.
var allBuiltinTools = []string{
	"Agent", "Bash", "Edit", "Glob", "Grep",
	"Read", "Skill", "ToolSearch", "WebFetch", "WebSearch", "Write",
}

// permissionItem represents a single entry in the permissions list.
type permissionItem struct {
	key        string // Permission string: "Read", "Bash(git:*)", "mcp__server__tool"
	category   string // "builtin", "bash", or "mcp"
	serverName string // MCP server name (empty for builtin/bash)
	hint       string // Classification indicator: "[read-only]", "[destructive]", "[heuristic]", ""
	source     string // "profile", "custom", or "inherited"
}

func (i permissionItem) Title() string       { return i.key }
func (i permissionItem) Description() string { return i.hint }
func (i permissionItem) FilterValue() string { return i.key }

// categorizePermission returns the category and server name for a permission string.
func categorizePermission(key string) (category, serverName string) {
	if strings.HasPrefix(key, "mcp__") {
		parts := strings.SplitN(key, "__", 3)
		if len(parts) >= 2 {
			return "mcp", parts[1]
		}
		return "mcp", ""
	}
	if strings.HasPrefix(key, "Bash(") {
		return "bash", ""
	}
	return "builtin", ""
}

// permissionsKeyMap holds key bindings for the permissions tab.
type permissionsKeyMap struct {
	Toggle  key.Binding
	Add     key.Binding
	Delete  key.Binding
	Profile key.Binding
	Mode    key.Binding
	Save    key.Binding
}

func newPermissionsKeyMap() permissionsKeyMap {
	return permissionsKeyMap{
		Toggle:  key.NewBinding(key.WithKeys(" ", "x"), key.WithHelp("space", "toggle")),
		Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Profile: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "profile")),
		Mode:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
		Save:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save")),
	}
}

// permissionCheckboxDelegate renders permission items as single-line items with
// [x]/[ ] checkboxes and source-based coloring.
type permissionCheckboxDelegate struct {
	states  map[string]permState  // key -> permAllow/permDeny/permAsk
	sources map[string]string     // key -> "profile"/"custom"/"inherited"
	width   *int                  // pointer to terminal width for truncation
	mode    *string               // pointer to current permission mode
}

func (d permissionCheckboxDelegate) Height() int                             { return 1 }
func (d permissionCheckboxDelegate) Spacing() int                            { return 0 }
func (d permissionCheckboxDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d permissionCheckboxDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(permissionItem)
	if !ok {
		return
	}

	// Section header items have category as key prefix
	if pi.source == "header" {
		dim := lipgloss.NewStyle().Faint(true)
		fmt.Fprint(w, dim.Render("  "+pi.key))
		return
	}

	isFocused := index == m.Index()
	state := d.states[pi.key]
	source := d.sources[pi.key]
	isInherited := source == "inherited"
	isBypassed := d.mode != nil && *d.mode == "bypassPermissions"

	// Truncate long permission keys to fit terminal width
	displayKey := pi.key
	termWidth := 80
	if d.width != nil && *d.width > 0 {
		termWidth = *d.width
	}
	maxKeyLen := termWidth - 8
	if len(displayKey) > maxKeyLen && maxKeyLen > 10 {
		displayKey = displayKey[:maxKeyLen-3] + "..."
	}

	cursor := "  "
	if isFocused {
		cursor = "> "
	}

	// Three-state checkbox: [+] allow, [-] deny, [ ] ask
	var cb string
	switch state {
	case permAllow:
		cb = "[+]"
	case permDeny:
		cb = "[-]"
	default:
		cb = "[ ]"
	}

	hintSuffix := ""
	if pi.hint != "" {
		hintSuffix = " " + pi.hint
	}

	dim := lipgloss.NewStyle().Faint(true)

	// When bypassPermissions is active, dim everything (permissions are ignored)
	if isBypassed {
		line := cursor + dim.Render(cb+" "+displayKey+hintSuffix)
		fmt.Fprint(w, line)
		return
	}

	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	grey := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	mutedGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#557755"))

	if isFocused {
		cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		cyanBold := cyan.Bold(true)

		// Checkbox: green=allow, red=deny, cyan=ask, muted=inherited
		var cbStyled string
		switch {
		case isInherited && state == permAllow:
			cbStyled = mutedGreen.Bold(true).Render(cb)
		case isInherited:
			cbStyled = grey.Render(cb)
		case state == permAllow:
			cbStyled = green.Bold(true).Render(cb)
		case state == permDeny:
			cbStyled = red.Bold(true).Render(cb)
		default:
			cbStyled = cyanBold.Render(cb)
		}

		// Name: cyan bold when focused, grey for inherited
		nameStyled := cyanBold.Render(displayKey)
		if isInherited {
			nameStyled = grey.Render(displayKey)
		}

		line := cyanBold.Render(cursor) + cbStyled + " " + nameStyled + dim.Render(hintSuffix)
		fmt.Fprint(w, line)
	} else {
		// Checkbox: green=allow, red=deny, dim=ask, muted=inherited
		var cbStyled string
		switch {
		case isInherited && state == permAllow:
			cbStyled = mutedGreen.Render(cb)
		case isInherited:
			cbStyled = grey.Render(cb)
		case state == permAllow:
			cbStyled = green.Render(cb)
		case state == permDeny:
			cbStyled = red.Render(cb)
		default:
			cbStyled = dim.Render(cb)
		}

		// Name: normal, grey for inherited, red for denied
		nameStyled := displayKey
		if isInherited {
			nameStyled = grey.Render(displayKey)
		} else if state == permDeny {
			nameStyled = red.Render(displayKey)
		}

		line := cursor + cbStyled + " " + nameStyled + dim.Render(hintSuffix)
		fmt.Fprint(w, line)
	}
}

// permAction represents the action chosen from the permissions tab.
type permAction int

const (
	permActionNone permAction = iota
	permActionSave
	permActionMode
	permActionProfile
	permActionAdd
)

// buildPermissionItems constructs a flat list of permission items from the current
// scope's settings, grouped by category with section headers.
func buildPermissionItems(scope string) ([]list.Item, map[string]permState, map[string]string) {
	permissions := config.ReadAllPermissions(scope)
	denied := config.ReadDenyPermissions(scope)
	states := make(map[string]permState)
	sources := make(map[string]string)

	// All built-in tools start as "ask" (visible but not mentioned)
	for _, tool := range allBuiltinTools {
		states[tool] = permAsk
	}

	// Mark allowed permissions
	for _, perm := range permissions {
		states[perm] = permAllow
		sources[perm] = "custom"
	}

	// Mark denied permissions
	for _, perm := range denied {
		states[perm] = permDeny
		sources[perm] = "custom"
	}

	// In project scope, add inherited user-scope permissions
	if scope == "project" {
		userPerms := config.ReadAllPermissions("user")
		for _, perm := range userPerms {
			if _, exists := sources[perm]; !exists {
				states[perm] = permAllow
				sources[perm] = "inherited"
			}
		}
		userDenied := config.ReadDenyPermissions("user")
		for _, perm := range userDenied {
			if _, exists := sources[perm]; !exists {
				states[perm] = permDeny
				sources[perm] = "inherited"
			}
		}
	}

	// Categorize all permissions
	var builtinItems, bashItems []permissionItem
	mcpItems := make(map[string][]permissionItem)

	for perm, state := range states {
		// Skip built-in tools in "ask" state without a source (they'll be added separately)
		// Actually, include all - built-in tools should always show
		_ = state
		cat, server := categorizePermission(perm)
		item := permissionItem{
			key:        perm,
			category:   cat,
			serverName: server,
			source:     sources[perm],
		}

		switch cat {
		case "builtin":
			builtinItems = append(builtinItems, item)
		case "bash":
			bashItems = append(bashItems, item)
		case "mcp":
			mcpItems[server] = append(mcpItems[server], item)
		}
	}

	// Sort within categories for stable ordering
	sort.Slice(builtinItems, func(i, j int) bool { return builtinItems[i].key < builtinItems[j].key })
	sort.Slice(bashItems, func(i, j int) bool { return bashItems[i].key < bashItems[j].key })

	// Build flat list with section headers
	var items []list.Item

	if len(builtinItems) > 0 {
		items = append(items, permissionItem{key: "── Built-in Tools ──", source: "header"})
		for _, item := range builtinItems {
			items = append(items, item)
		}
	}

	if len(bashItems) > 0 {
		items = append(items, permissionItem{key: "── Bash Patterns ──", source: "header"})
		for _, item := range bashItems {
			items = append(items, item)
		}
	}

	if len(mcpItems) > 0 {
		var serverNames []string
		for name := range mcpItems {
			serverNames = append(serverNames, name)
		}
		sort.Strings(serverNames)

		for _, server := range serverNames {
			items = append(items, permissionItem{
				key:    fmt.Sprintf("── MCP: %s ──", server),
				source: "header",
			})
			serverItems := mcpItems[server]
			sort.Slice(serverItems, func(i, j int) bool { return serverItems[i].key < serverItems[j].key })
			for _, item := range serverItems {
				items = append(items, item)
			}
		}
	}

	return items, states, sources
}

// updatePermissionsTab handles key events when the permissions tab is active.
// Returns the updated model and tea.Cmd. The returned permAction signals
// that the outer loop should exit the TUI and run an inline flow.
func updatePermissionsTab(m *manageModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Skip header items for toggle/delete
	isHeader := false
	if item, ok := m.permList.SelectedItem().(permissionItem); ok {
		isHeader = item.source == "header"
	}

	switch {
	case key.Matches(msg, m.permKeys.Toggle):
		// Block toggling when bypassPermissions is active (all checks are skipped)
		if m.permMode == "bypassPermissions" {
			return *m, nil
		}
		if !isHeader {
			if item, ok := m.permList.SelectedItem().(permissionItem); ok {
				// Cycle: ask -> allow -> deny -> ask
				current := m.permStates[item.key]
				switch current {
				case permAsk:
					m.permStates[item.key] = permAllow
				case permAllow:
					m.permStates[item.key] = permDeny
				case permDeny:
					m.permStates[item.key] = permAsk
				}
				m.permSources[item.key] = "custom"
			}
		}
		return *m, nil

	case key.Matches(msg, m.permKeys.Delete):
		if !isHeader {
			if item, ok := m.permList.SelectedItem().(permissionItem); ok {
				if m.permSources[item.key] != "inherited" {
					// Built-in tools without params are always visible, reset to "ask"
					if item.category == "builtin" && !strings.Contains(item.key, "(") {
						m.permStates[item.key] = permAsk
						delete(m.permSources, item.key)
					} else {
						// Non-builtin entries (bash patterns, MCP tools) can be removed
						delete(m.permStates, item.key)
						delete(m.permSources, item.key)
						items := m.permList.Items()
						for i, li := range items {
							if pi, ok := li.(permissionItem); ok && pi.key == item.key {
								items = append(items[:i], items[i+1:]...)
								break
							}
						}
						m.permList.SetItems(items)
					}
				}
			}
		}
		return *m, nil

	case key.Matches(msg, m.permKeys.Save):
		m.permAction = permActionSave
		return *m, tea.Quit

	case key.Matches(msg, m.permKeys.Mode):
		m.permAction = permActionMode
		return *m, tea.Quit

	case key.Matches(msg, m.permKeys.Profile):
		m.permAction = permActionProfile
		return *m, tea.Quit

	case key.Matches(msg, m.permKeys.Add):
		m.permAction = permActionAdd
		return *m, tea.Quit
	}

	var cmd tea.Cmd
	m.permList, cmd = m.permList.Update(msg)
	return *m, cmd
}

// permissionsView renders the permissions tab content.
func permissionsView(m *manageModel) string {
	var b strings.Builder

	// Mode display at top
	mode := m.permMode
	if mode == "" {
		mode = "default"
	}

	modeStyle := display.StyleDim
	if mode == "bypassPermissions" {
		modeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	}
	b.WriteString("  Mode: " + modeStyle.Render(mode))

	if mode == "bypassPermissions" {
		warning := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
		b.WriteString("  " + warning.Render("WARNING: All safety checks disabled"))
	}
	b.WriteString("\n")

	b.WriteString(m.permList.View())
	return b.String()
}

// reloadPermissions clears and reloads the permission list, checked state,
// and sources from disk for the current scope. In project scope, user-scope
// permissions are included as read-only "inherited" items.
func (m *manageModel) reloadPermissions() {
	_, checked, sources := buildPermissionItems(m.scope)

	// Clear and refill checked map in place (delegate holds reference)
	for k := range m.permStates {
		delete(m.permStates, k)
	}
	for k, v := range checked {
		m.permStates[k] = v
	}

	// Clear and refill sources map in place
	for k := range m.permSources {
		delete(m.permSources, k)
	}
	for k, v := range sources {
		m.permSources[k] = v
	}

	// Clear hints
	for k := range m.permHints {
		delete(m.permHints, k)
	}

	// Rebuild the list (buildPermissionItems already handles inherited items)
	rebuildPermListFromState(m)

	m.permMode = config.ReadPermissionMode(m.scope)
	if m.permModePtr != nil {
		*m.permModePtr = m.permMode
	}

	// Snapshot for dirty checking
	m.snapshotPermState()
}

// snapshotPermState captures the current permissions state as the baseline
// for dirty-checking.
func (m *manageModel) snapshotPermState() {
	m.permInitialStates = make(map[string]permState, len(m.permStates))
	for k, v := range m.permStates {
		m.permInitialStates[k] = v
	}
	m.permInitialMode = m.permMode
}

// computePermChanges returns the list of permission changes vs the initial state.
func (m *manageModel) computePermChanges() []pendingChange {
	var changes []pendingChange
	stateLabel := map[permState]string{permAllow: "allow", permDeny: "deny", permAsk: "ask"}

	for k, current := range m.permStates {
		initial := m.permInitialStates[k]
		if current != initial {
			changes = append(changes, pendingChange{
				name:   fmt.Sprintf("%s: %s -> %s", k, stateLabel[initial], stateLabel[current]),
				action: "change",
			})
		}
	}
	// Check for entries removed from states (shouldn't happen with built-ins always present, but safety)
	for k, initial := range m.permInitialStates {
		if _, exists := m.permStates[k]; !exists && initial != permAsk {
			changes = append(changes, pendingChange{k, "remove"})
		}
	}
	if m.permMode != m.permInitialMode {
		changes = append(changes, pendingChange{
			name:   fmt.Sprintf("mode: %s -> %s", m.permInitialMode, m.permMode),
			action: "change",
		})
	}
	return changes
}

// consolidatePermissions removes redundant entries from a permission list and
// returns the cleaned list plus a list of removed entries for reporting.
// Rules:
//   - Remove Bash(# ...) entries (comment-only, no-op)
//   - Remove duplicates
//   - If Bash(*) exists, remove all other Bash(...) entries (wildcard subsumes them)
//   - If mcp__server__* exists, remove specific mcp__server__tool entries for that server
func consolidatePermissions(permissions []string) (cleaned, removed []string) {
	seen := make(map[string]bool, len(permissions))
	hasBashWildcard := false
	hasBareBash := false
	hasMcpGlobalWildcard := false
	mcpWildcards := make(map[string]bool) // server names with mcp__server__* wildcard

	// First pass: detect wildcards and bare tool entries
	for _, p := range permissions {
		if p == "Bash(*)" {
			hasBashWildcard = true
		}
		if p == "Bash" {
			hasBareBash = true
		}
		if p == "mcp__*" {
			hasMcpGlobalWildcard = true
		}
		if strings.HasPrefix(p, "mcp__") && strings.HasSuffix(p, "__*") && p != "mcp__*" {
			parts := strings.SplitN(p, "__", 3)
			if len(parts) >= 2 {
				mcpWildcards[parts[1]] = true
			}
		}
	}

	// Second pass: filter
	for _, p := range permissions {
		// Skip duplicates
		if seen[p] {
			removed = append(removed, p+" (duplicate)")
			continue
		}
		seen[p] = true

		// Remove Bash(# ...) comment-only entries
		if strings.HasPrefix(p, "Bash(#") {
			removed = append(removed, p)
			continue
		}

		// Remove junk entries (standalone shell keywords)
		if p == "Bash(done)" || p == "Bash(fi)" || p == "Bash(else)" || p == "Bash(then)" {
			removed = append(removed, p+" (junk shell keyword)")
			continue
		}

		// Bare "Bash" subsumes all Bash(pattern) entries
		if hasBareBash && strings.HasPrefix(p, "Bash(") {
			removed = append(removed, p+" (subsumed by Bash)")
			continue
		}

		// Bash(*) subsumes specific Bash patterns (if bare Bash didn't already catch them)
		if hasBashWildcard && strings.HasPrefix(p, "Bash(") && p != "Bash(*)" {
			removed = append(removed, p+" (subsumed by Bash(*))")
			continue
		}

		// mcp__* global wildcard subsumes all mcp__server__* entries
		if hasMcpGlobalWildcard && strings.HasPrefix(p, "mcp__") && p != "mcp__*" {
			removed = append(removed, p+" (subsumed by mcp__*)")
			continue
		}

		// mcp__server__* wildcard subsumes specific tools for that server
		if !hasMcpGlobalWildcard && strings.HasPrefix(p, "mcp__") && !strings.HasSuffix(p, "__*") {
			parts := strings.SplitN(p, "__", 3)
			if len(parts) >= 2 && mcpWildcards[parts[1]] {
				removed = append(removed, p+" (subsumed by mcp__"+parts[1]+"__*)")
				continue
			}
		}

		cleaned = append(cleaned, p)
	}

	return cleaned, removed
}

// runSavePermissions writes the current permission state to disk.
// The sources map is used to filter out "inherited" entries (those should
// not be written to the project-scope file since they already exist in user scope).
// Runs auto-consolidation to remove redundant entries before writing.
func runSavePermissions(states map[string]permState, sources map[string]string, mode, scope string) error {
	// Collect allowed and denied permissions, excluding inherited entries
	var permissions, denied []string
	for k, state := range states {
		if sources[k] == "inherited" {
			continue
		}
		switch state {
		case permAllow:
			permissions = append(permissions, k)
		case permDeny:
			denied = append(denied, k)
		}
		// permAsk: not mentioned in either list
	}
	sort.Strings(permissions)
	sort.Strings(denied)

	// Auto-consolidate: remove redundant entries from allow list
	permissions, removed := consolidatePermissions(permissions)

	path, err := config.WriteAllPermissions(scope, permissions, denied)
	if err != nil {
		return fmt.Errorf("failed to write permissions: %w", err)
	}

	modePath, err := config.WritePermissionMode(scope, mode)
	if err != nil {
		return fmt.Errorf("failed to write permission mode: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s permissions updated (%d entries)\n",
		display.StyleGreen.Render("Saved:"),
		len(permissions))
	if len(removed) > 0 {
		fmt.Printf("  %s removed %d redundant entries:\n",
			display.StyleYellow.Render("Cleanup:"),
			len(removed))
		for _, r := range removed {
			// Truncate long removed entries for readability
			if len(r) > 80 {
				r = r[:77] + "..."
			}
			fmt.Printf("    %s %s\n", display.StyleDim.Render("-"), r)
		}
	}
	fmt.Printf("  %s %s\n", display.StyleDim.Render("Permissions:"), path)
	fmt.Printf("  %s %s\n", display.StyleDim.Render("Mode:"), modePath)
	if mode != "" {
		fmt.Printf("  %s %s\n", display.StyleDim.Render("Permission mode:"), mode)
	}
	fmt.Println()

	return nil
}

// runModeSelector shows a huh form for selecting the permission mode.
func runModeSelector(currentMode string) (string, error) {
	if currentMode == "" {
		currentMode = "default"
	}

	var mode string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Permission Mode").
			Description("Controls how Claude Code handles permission checks at runtime").
			Options(
				huh.NewOption("default (ask for everything)", "default"),
				huh.NewOption("acceptEdits (auto-approve file edits)", "acceptEdits"),
				huh.NewOption("auto (AI classifier decides)", "auto"),
				huh.NewOption("bypassPermissions (skip ALL checks)", "bypassPermissions"),
			).
			Value(&mode),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return currentMode, handleAbort(err)
	}

	if mode == "default" {
		mode = ""
	}

	// Confirm dangerous mode
	if mode == "bypassPermissions" {
		confirmed, err := confirm("bypassPermissions disables ALL safety checks. Are you sure?", false)
		if err != nil {
			return currentMode, handleAbort(err)
		}
		if !confirmed {
			return currentMode, nil
		}
	}

	return mode, nil
}

// runProfileSelector shows a huh form for selecting a profile.
// Returns the selected profile name (empty for "None") and whether a profile was selected.
func runProfileSelector() (string, bool, error) {
	profiles, err := config.LoadProfiles()
	if err != nil {
		return "", false, fmt.Errorf("loading profiles: %w", err)
	}

	options := []huh.Option[string]{
		huh.NewOption("None (clear all permissions)", ""),
	}
	for _, p := range profiles {
		label := p.Name
		if p.Description != "" {
			label += " - " + p.Description
		}
		options = append(options, huh.NewOption(label, p.Name))
	}

	var choice string
	err = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select Profile").
			Description("Apply a permission preset").
			Options(options...).
			Value(&choice),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return "", false, handleAbort(err)
	}

	return choice, true, nil
}

// applyProfile applies a profile to the permissions state.
// It clears existing permissions and populates from the profile.
func applyProfile(m *manageModel, profileName string) error {
	// Clear all permissions
	for k := range m.permStates {
		delete(m.permStates, k)
	}
	for k := range m.permSources {
		delete(m.permSources, k)
	}
	for k := range m.permHints {
		delete(m.permHints, k)
	}

	// Re-add all built-in tools as "ask" (always visible)
	for _, tool := range allBuiltinTools {
		m.permStates[tool] = permAsk
	}

	if profileName == "" {
		// "None" selected: clear everything and reset mode
		m.permMode = ""
		if m.permModePtr != nil {
			*m.permModePtr = ""
		}
		rebuildPermListFromState(m)
		return nil
	}

	// Load the profile
	profiles, err := config.LoadProfiles()
	if err != nil {
		return fmt.Errorf("loading profiles: %w", err)
	}

	var profile *config.Profile
	for i := range profiles {
		if profiles[i].Name == profileName {
			profile = &profiles[i]
			break
		}
	}
	if profile == nil {
		return fmt.Errorf("profile %q not found", profileName)
	}

	// Set mode from profile
	m.permMode = profile.Mode
	if m.permModePtr != nil {
		*m.permModePtr = profile.Mode
	}

	// Add allow entries
	for _, perm := range profile.Permissions.Allow {
		m.permStates[perm] = permAllow
		m.permSources[perm] = "profile"
	}

	// Add bash patterns as Bash(<pattern>)
	for _, pattern := range profile.Permissions.Bash {
		key := "Bash(" + pattern + ")"
		m.permStates[key] = permAllow
		m.permSources[key] = "profile"
	}

	// MCP tool handling: if the profile uses a global wildcard include,
	// just add mcp__* instead of discovering every server's tools individually.
	hasGlobalWildcard := false
	for _, pattern := range profile.Permissions.MCP.Include {
		if pattern == "*" {
			hasGlobalWildcard = true
			break
		}
	}

	if hasGlobalWildcard {
		// Global wildcard: one entry covers all servers and tools
		m.permStates["mcp__*"] = permAllow
		m.permSources["mcp__*"] = "profile"
	} else {
		// Discover and classify MCP tools from all configured servers
		servers, _ := config.LoadServers()
		for _, serverName := range config.ServerNames(servers) {
			var tools []mcpclient.ToolInfo
			if cached, ok := toolCache.Load(serverName); ok {
				tools = cached.([]mcpclient.ToolInfo)
			} else {
				fmt.Printf("  Discovering tools for %s...\n", display.StyleCyan.Render(serverName))
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				discovered, err := mcpclient.ListTools(ctx, serverName, servers[serverName])
				cancel()
				if err != nil {
					fmt.Printf("  %s %s: %v (skipping)\n",
						display.StyleYellow.Render("Warning:"), serverName, err)
					continue
				}
				toolCache.Store(serverName, discovered)
				tools = discovered
			}

			// Classify each tool against the profile's rules
			for _, tool := range tools {
				ct := config.ClassifyTool(tool.Name, tool.ReadOnlyHint, tool.DestructiveHint, tool.HasAnnotations, *profile)
				key := "mcp__" + serverName + "__" + tool.Name
				if ct.Approved {
					m.permStates[key] = permAllow
					m.permSources[key] = "profile"
				}
				if ct.Hint != "" {
					m.permHints[key] = ct.Hint
				}
			}
		}
	}

	// Rebuild list items from the new state
	rebuildPermListFromState(m)

	return nil
}

// rebuildPermListFromState rebuilds the permission list items from the current
// checked and sources maps.
func rebuildPermListFromState(m *manageModel) {
	var builtinItems, bashItems []permissionItem
	mcpItems := make(map[string][]permissionItem)

	for perm := range m.permStates {
		cat, server := categorizePermission(perm)
		source := m.permSources[perm]
		if source == "" {
			source = "custom"
		}
		hint := m.permHints[perm]
		item := permissionItem{
			key:        perm,
			category:   cat,
			serverName: server,
			hint:       hint,
			source:     source,
		}

		switch cat {
		case "builtin":
			builtinItems = append(builtinItems, item)
		case "bash":
			bashItems = append(bashItems, item)
		case "mcp":
			mcpItems[server] = append(mcpItems[server], item)
		}
	}

	// Sort within categories
	sort.Slice(builtinItems, func(i, j int) bool { return builtinItems[i].key < builtinItems[j].key })
	sort.Slice(bashItems, func(i, j int) bool { return bashItems[i].key < bashItems[j].key })

	var items []list.Item

	if len(builtinItems) > 0 {
		items = append(items, permissionItem{key: "── Built-in Tools ──", source: "header"})
		for _, item := range builtinItems {
			items = append(items, item)
		}
	}

	if len(bashItems) > 0 {
		items = append(items, permissionItem{key: "── Bash Patterns ──", source: "header"})
		for _, item := range bashItems {
			items = append(items, item)
		}
	}

	if len(mcpItems) > 0 {
		var serverNames []string
		for name := range mcpItems {
			serverNames = append(serverNames, name)
		}
		sort.Strings(serverNames)

		for _, server := range serverNames {
			items = append(items, permissionItem{
				key:    fmt.Sprintf("── MCP: %s ──", server),
				source: "header",
			})
			serverItems := mcpItems[server]
			sort.Slice(serverItems, func(i, j int) bool { return serverItems[i].key < serverItems[j].key })
			for _, item := range serverItems {
				items = append(items, item)
			}
		}
	}

	m.permList.SetItems(items)
}

// countCustomChanges returns the number of permissions with "custom" source.
func countCustomChanges(sources map[string]string) int {
	count := 0
	for _, s := range sources {
		if s == "custom" {
			count++
		}
	}
	return count
}

// breadcrumb renders a navigation path at the top of add permission forms.
func breadcrumb(parts ...string) string {
	dim := lipgloss.NewStyle().Faint(true)
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	var rendered []string
	for i, p := range parts {
		if i == len(parts)-1 {
			rendered = append(rendered, cyan.Render(p))
		} else {
			rendered = append(rendered, dim.Render(p))
		}
	}
	return "\n  " + strings.Join(rendered, dim.Render(" > ")) + "\n"
}

// runAddPermission shows a guided flow for adding a new permission.
// Returns the permission keys to add, or empty slice if cancelled.
func runAddPermission(scope string) ([]string, error) {
	fmt.Print(breadcrumb("Add Permission"))

	var category string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Category").
			Options(
				huh.NewOption("Built-in tool", "builtin"),
				huh.NewOption("Bash pattern", "bash"),
				huh.NewOption("MCP tool", "mcp"),
			).
			Value(&category),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return nil, handleAbort(err)
	}

	switch category {
	case "builtin":
		key, err := runAddBuiltinTool()
		if err != nil {
			return nil, err
		}
		return []string{key}, nil
	case "bash":
		key, err := runAddBashPattern()
		if err != nil {
			return nil, err
		}
		return []string{key}, nil
	case "mcp":
		return runAddMCPTool(scope)
	}
	return nil, nil
}

// runAddBuiltinTool shows a selector for built-in tools.
func runAddBuiltinTool() (string, error) {
	fmt.Print(breadcrumb("Add Permission", "Built-in Tool"))

	var tool string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select tool").
			Options(
				huh.NewOption("Read", "Read"),
				huh.NewOption("Write", "Write"),
				huh.NewOption("Edit", "Edit"),
				huh.NewOption("Glob", "Glob"),
				huh.NewOption("Grep", "Grep"),
				huh.NewOption("WebFetch", "WebFetch"),
				huh.NewOption("WebSearch", "WebSearch"),
				huh.NewOption("Bash", "Bash"),
			).
			Value(&tool),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return "", handleAbort(err)
	}
	return tool, nil
}

// runAddBashPattern shows a selector with common bash patterns and a custom option.
func runAddBashPattern() (string, error) {
	fmt.Print(breadcrumb("Add Permission", "Bash Pattern"))

	var pattern string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select pattern or enter custom").
			Options(
				huh.NewOption("git:* (all git commands)", "git:*"),
				huh.NewOption("npm test (npm test only)", "npm test"),
				huh.NewOption("cargo test:* (cargo test commands)", "cargo test:*"),
				huh.NewOption("kubectl:* (all kubectl commands)", "kubectl:*"),
				huh.NewOption("make:* (all make targets)", "make:*"),
				huh.NewOption("go test:* (go test commands)", "go test:*"),
				huh.NewOption("python:* (python commands)", "python:*"),
				huh.NewOption("ls:* (list files)", "ls:*"),
				huh.NewOption("cat:* (read files)", "cat:*"),
				huh.NewOption("* (ALL bash commands)", "*"),
				huh.NewOption("Custom pattern...", "__custom__"),
			).
			Value(&pattern),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return "", handleAbort(err)
	}

	if pattern == "__custom__" {
		fmt.Print(breadcrumb("Add Permission", "Bash Pattern", "Custom"))

		var custom string
		err := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Enter pattern").
				Description("e.g., 'docker:*', 'npm run build'").
				Value(&custom),
		)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

		if err != nil {
			return "", handleAbort(err)
		}
		if custom == "" {
			return "", errCancelled
		}
		pattern = custom
	}

	return "Bash(" + pattern + ")", nil
}

// readOnlyClassificationProfile is used to classify tools when the user
// selects "Read-only tools" in the add MCP tool flow.
var readOnlyClassificationProfile = config.Profile{
	Permissions: config.ProfilePermissions{
		MCP: config.ProfileMCP{
			UseAnnotations: true,
			Heuristic: config.ProfileHeuristic{
				SafePrefixes:   []string{"list", "get", "read", "search", "describe", "show", "fetch", "count", "check", "view"},
				UnsafePrefixes: []string{"create", "delete", "update", "write", "execute", "run", "modify", "remove", "drop", "send", "batch_create", "batch_update", "batch_delete"},
			},
		},
	},
}

// runAddMCPTool shows a server selector followed by tool discovery and selection.
// Returns multiple permission keys (for bulk selection strategies like "read-only").
// Uses all configured servers (from central config), not just scope-enabled ones.
func runAddMCPTool(scope string) ([]string, error) {
	fmt.Print(breadcrumb("Add Permission", "MCP Tool"))

	servers, err := config.LoadServers()
	if err != nil {
		return nil, fmt.Errorf("loading servers: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println(display.StyleYellow.Render("  No MCP servers configured."))
		return nil, errCancelled
	}

	// Show ALL configured servers (not just scope-enabled) so newly added ones appear
	serverNames := config.ServerNames(servers)
	enabledServers := config.ReadMcpServers(scope)

	var serverOptions []huh.Option[string]
	for _, name := range serverNames {
		label := name
		if _, enabled := enabledServers[name]; !enabled {
			label += " (not enabled)"
		}
		serverOptions = append(serverOptions, huh.NewOption(label, name))
	}

	var serverName string
	err = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select server").
			Options(serverOptions...).
			Value(&serverName),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return nil, handleAbort(err)
	}

	fmt.Print(breadcrumb("Add Permission", "MCP Tool", serverName))

	// Discover tools
	serverDef, ok := servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	var tools []mcpclient.ToolInfo
	if cached, ok := toolCache.Load(serverName); ok {
		tools = cached.([]mcpclient.ToolInfo)
	} else {
		fmt.Printf("  Discovering tools for %s...\n", display.StyleCyan.Render(serverName))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		tools, err = mcpclient.ListTools(ctx, serverName, serverDef)
		if err != nil {
			fmt.Printf("  %s %s: %v\n",
				display.StyleYellow.Render("Warning:"), serverName, err)
			fmt.Println(display.StyleDim.Render("  Tool discovery failed. You can add a wildcard permission instead."))

			var useWildcard bool
			useWildcard, err = confirm(fmt.Sprintf("Add mcp__%s__* (all tools)?", serverName), true)
			if err != nil {
				return nil, handleAbort(err)
			}
			if useWildcard {
				return []string{"mcp__" + serverName + "__*"}, nil
			}
			return nil, errCancelled
		}
		toolCache.Store(serverName, tools)
	}

	if len(tools) == 0 {
		fmt.Println(display.StyleYellow.Render("  No tools found for this server."))
		return nil, errCancelled
	}

	// Classify tools to count read-only ones for the option label
	var readOnlyCount int
	for _, t := range tools {
		ct := config.ClassifyTool(t.Name, t.ReadOnlyHint, t.DestructiveHint, t.HasAnnotations, readOnlyClassificationProfile)
		if ct.Approved {
			readOnlyCount++
		}
	}

	// Selection strategy
	var strategy string
	strategyOptions := []huh.Option[string]{
		huh.NewOption(fmt.Sprintf("* All tools (%d)", len(tools)), "all"),
		huh.NewOption(fmt.Sprintf("Read-only tools (%d of %d, auto-classified)", readOnlyCount, len(tools)), "readonly"),
		huh.NewOption("Pick a single tool...", "single"),
	}

	err = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Selection mode").
			Options(strategyOptions...).
			Value(&strategy),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

	if err != nil {
		return nil, handleAbort(err)
	}

	prefix := "mcp__" + serverName + "__"

	switch strategy {
	case "all":
		return []string{prefix + "*"}, nil

	case "readonly":
		var keys []string
		for _, t := range tools {
			ct := config.ClassifyTool(t.Name, t.ReadOnlyHint, t.DestructiveHint, t.HasAnnotations, readOnlyClassificationProfile)
			if ct.Approved {
				keys = append(keys, prefix+t.Name)
			}
		}
		if len(keys) == 0 {
			fmt.Println(display.StyleYellow.Render("  No read-only tools found."))
			return nil, errCancelled
		}
		fmt.Printf("  %s %d read-only tools selected for %s\n",
			display.StyleGreen.Render("Selected:"), len(keys), serverName)
		return keys, nil

	case "single":
		fmt.Print(breadcrumb("Add Permission", "MCP Tool", serverName, "Select tools"))

		var toolOptions []huh.Option[string]
		for _, t := range tools {
			desc := cleanDescription(t.Description)
			label := t.Name
			if desc != "" {
				label += " - " + desc
			}
			toolOptions = append(toolOptions, huh.NewOption(label, t.Name))
		}

		var selected []string
		err = huh.NewForm(huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select tools (space to toggle, enter to confirm)").
				Options(toolOptions...).
				Value(&selected),
		)).WithTheme(formTheme()).WithKeyMap(formKeyMap()).Run()

		if err != nil {
			return nil, handleAbort(err)
		}
		if len(selected) == 0 {
			return nil, errCancelled
		}

		var keys []string
		for _, name := range selected {
			keys = append(keys, prefix+name)
		}
		return keys, nil
	}

	return nil, nil
}

// runProfileConfirmation asks the user if they want to reset custom changes.
func runProfileConfirmation(customCount int) (bool, error) {
	confirmed, err := confirm(
		fmt.Sprintf("Reset %d custom change(s) and apply profile?", customCount),
		true,
	)
	if err != nil {
		return false, handleAbort(err)
	}
	return confirmed, nil
}
