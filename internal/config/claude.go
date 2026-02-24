package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mcpJSONName = ".mcp.json"

// ConfigPath returns the Claude config file path for a scope.
func ConfigPath(scope string) string {
	if scope == "user" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude.json")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, mcpJSONName)
}

// readMcpServersFromPath reads the mcpServers dict from the given file path.
func readMcpServersFromPath(path string) ServerMap {
	data, err := os.ReadFile(path)
	if err != nil {
		return ServerMap{}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ServerMap{}
	}

	serversRaw, ok := raw["mcpServers"]
	if !ok {
		return ServerMap{}
	}

	var servers ServerMap
	if err := json.Unmarshal(serversRaw, &servers); err != nil {
		return ServerMap{}
	}
	return servers
}

// ReadMcpServers reads the mcpServers dict from the Claude config for the given scope.
func ReadMcpServers(scope string) ServerMap {
	return readMcpServersFromPath(ConfigPath(scope))
}

// ReadInheritedMcpServers walks parent directories from the current working directory
// upward to the filesystem root, reading .mcp.json files along the way. It returns
// a map of server names to the source .mcp.json file path for servers that are
// inherited (i.e., present in a parent directory but not in the local .mcp.json).
// Closest parent wins for conflicts (first-found semantics).
// Only relevant for project scope.
func ReadInheritedMcpServers() map[string]string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	// Read local .mcp.json to know which servers to skip
	localPath := filepath.Join(cwd, mcpJSONName)
	localServers := readMcpServersFromPath(localPath)

	inherited := make(map[string]string)
	dir := filepath.Dir(cwd)

	for {
		parentConfig := filepath.Join(dir, mcpJSONName)
		parentServers := readMcpServersFromPath(parentConfig)
		for name := range parentServers {
			// Skip if already in local config (local wins)
			if _, inLocal := localServers[name]; inLocal {
				continue
			}
			// Skip if already found in a closer parent (first-found wins)
			if _, found := inherited[name]; found {
				continue
			}
			inherited[name] = parentConfig
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	if len(inherited) == 0 {
		return nil
	}
	return inherited
}

// WriteMcpServers merges servers into the Claude config file, removing deselected servers.
// It preserves any existing keys in the file that are not mcpServers.
func WriteMcpServers(scope string, servers ServerMap, toRemove []string) (string, error) {
	path := ConfigPath(scope)

	// Read existing file to preserve other keys
	var data map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &data); err != nil {
			data = make(map[string]json.RawMessage)
		}
	} else {
		data = make(map[string]json.RawMessage)
	}

	// Read existing mcpServers
	existing := ServerMap{}
	if raw, ok := data["mcpServers"]; ok {
		_ = json.Unmarshal(raw, &existing)
	}

	// Merge new servers
	for name, entry := range servers {
		existing[name] = entry
	}

	// Remove deselected servers
	for _, name := range toRemove {
		delete(existing, name)
	}

	// Marshal mcpServers back
	serversJSON, err := json.Marshal(existing)
	if err != nil {
		return "", fmt.Errorf("marshalling servers: %w", err)
	}
	data["mcpServers"] = serversJSON

	// Write the complete file, preserving all keys
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling config: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(path, output, 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

// ReadDisabledMcpServers reads the disabledMcpServers list from ~/.claude.json
// for the current working directory: .projects[cwd].disabledMcpServers
func ReadDisabledMcpServers() []string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Level 1: top-level keys
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	projectsRaw, ok := top["projects"]
	if !ok {
		return nil
	}

	// Level 2: projects map
	var projects map[string]json.RawMessage
	if err := json.Unmarshal(projectsRaw, &projects); err != nil {
		return nil
	}
	cwd, _ := os.Getwd()
	projRaw, ok := projects[cwd]
	if !ok {
		return nil
	}

	// Level 3: project entry
	var projEntry map[string]json.RawMessage
	if err := json.Unmarshal(projRaw, &projEntry); err != nil {
		return nil
	}
	disabledRaw, ok := projEntry["disabledMcpServers"]
	if !ok {
		return nil
	}

	var disabled []string
	if err := json.Unmarshal(disabledRaw, &disabled); err != nil {
		return nil
	}
	return disabled
}

// WriteDisabledMcpServers writes the disabledMcpServers list into ~/.claude.json
// under .projects[cwd]. All other keys at every level are preserved.
// If disabled is empty, the disabledMcpServers key is removed.
func WriteDisabledMcpServers(disabled []string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude.json")

	// Level 1: read top-level
	var top map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &top); err != nil {
			top = make(map[string]json.RawMessage)
		}
	} else {
		top = make(map[string]json.RawMessage)
	}

	// Level 2: read projects
	var projects map[string]json.RawMessage
	if raw, ok := top["projects"]; ok {
		if err := json.Unmarshal(raw, &projects); err != nil {
			projects = make(map[string]json.RawMessage)
		}
	} else {
		projects = make(map[string]json.RawMessage)
	}

	// Level 3: read project entry
	cwd, _ := os.Getwd()
	var projEntry map[string]json.RawMessage
	if raw, ok := projects[cwd]; ok {
		if err := json.Unmarshal(raw, &projEntry); err != nil {
			projEntry = make(map[string]json.RawMessage)
		}
	} else {
		projEntry = make(map[string]json.RawMessage)
	}

	// Set or remove disabledMcpServers
	if len(disabled) == 0 {
		delete(projEntry, "disabledMcpServers")
	} else {
		disabledJSON, err := json.Marshal(disabled)
		if err != nil {
			return fmt.Errorf("marshalling disabledMcpServers: %w", err)
		}
		projEntry["disabledMcpServers"] = disabledJSON
	}

	// Write back level 3 -> 2 -> 1
	projJSON, err := json.Marshal(projEntry)
	if err != nil {
		return fmt.Errorf("marshalling project entry: %w", err)
	}
	projects[cwd] = projJSON

	projectsJSON, err := json.Marshal(projects)
	if err != nil {
		return fmt.Errorf("marshalling projects: %w", err)
	}
	top["projects"] = projectsJSON

	output, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling claude.json: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(path, output, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// DisabledMcpServersSet returns the disabled MCP servers as a set for O(1) lookup.
func DisabledMcpServersSet() map[string]bool {
	disabled := ReadDisabledMcpServers()
	if len(disabled) == 0 {
		return nil
	}
	set := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		set[name] = true
	}
	return set
}

const settingsFileName = "settings.local.json"

// SettingsPath returns the path to settings.local.json for the given scope.
// "user" -> ~/.claude/settings.local.json
// "project" -> .claude/settings.local.json (relative to cwd)
func SettingsPath(scope string) string {
	if scope == "user" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude", settingsFileName)
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".claude", settingsFileName)
}

// mcpPermPrefix returns the permission entry prefix for a server,
// e.g. "mcp__my-server__".
func mcpPermPrefix(serverName string) string {
	return "mcp__" + serverName + "__"
}

// ReadToolPermissions reads settings.local.json for the given scope and returns
// the list of tool names that have permissions.allow entries matching
// mcp__<serverName>__*. Returns ["*"] if a wildcard entry exists.
func ReadToolPermissions(scope, serverName string) []string {
	path := SettingsPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	permsRaw, ok := raw["permissions"]
	if !ok {
		return nil
	}

	var perms struct {
		Allow []string `json:"allow"`
	}
	if err := json.Unmarshal(permsRaw, &perms); err != nil {
		return nil
	}

	prefix := mcpPermPrefix(serverName)
	wildcard := prefix + "*"
	var tools []string
	for _, entry := range perms.Allow {
		if entry == wildcard {
			return []string{"*"}
		}
		if strings.HasPrefix(entry, prefix) {
			toolName := strings.TrimPrefix(entry, prefix)
			if toolName != "" {
				tools = append(tools, toolName)
			}
		}
	}
	return tools
}

// WriteToolPermissions updates the permissions.allow array in settings.local.json
// for the given scope. It removes all existing mcp__<serverName>__* entries, then
// adds new entries based on toolNames. If all tools are selected (len(toolNames)
// == len(allToolNames)), a single wildcard entry is written instead.
// Deselected tools are also removed from permissions.deny to avoid conflicts.
// Returns the written file path.
func WriteToolPermissions(scope, serverName string, toolNames []string, allToolNames []string) (string, error) {
	path := SettingsPath(scope)

	// Read existing file to preserve all other keys
	var data map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &data); err != nil {
			data = make(map[string]json.RawMessage)
		}
	} else {
		data = make(map[string]json.RawMessage)
	}

	// Parse existing permissions
	type permissionsBlock struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
	}
	var perms permissionsBlock
	if raw, ok := data["permissions"]; ok {
		_ = json.Unmarshal(raw, &perms)
	}

	prefix := mcpPermPrefix(serverName)

	// Remove all existing entries for this server from allow
	filtered := make([]string, 0, len(perms.Allow))
	for _, entry := range perms.Allow {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}

	// Add new entries
	if len(toolNames) > 0 {
		if len(toolNames) == len(allToolNames) {
			// All selected: use wildcard
			filtered = append(filtered, prefix+"*")
		} else {
			for _, t := range toolNames {
				filtered = append(filtered, prefix+t)
			}
		}
	}
	perms.Allow = filtered

	// Build a set of selected tools for deny cleanup
	selectedSet := make(map[string]bool, len(toolNames))
	for _, t := range toolNames {
		selectedSet[t] = true
	}

	// Remove deselected tools from deny to avoid conflicts
	filteredDeny := make([]string, 0, len(perms.Deny))
	for _, entry := range perms.Deny {
		if strings.HasPrefix(entry, prefix) {
			toolName := strings.TrimPrefix(entry, prefix)
			// Keep deny entries for tools that are NOT in the selected set
			// (i.e., only remove deny entries for tools we're allowing)
			if selectedSet[toolName] {
				continue
			}
		}
		filteredDeny = append(filteredDeny, entry)
	}
	perms.Deny = filteredDeny

	// Marshal permissions back
	permsJSON, err := json.Marshal(perms)
	if err != nil {
		return "", fmt.Errorf("marshalling permissions: %w", err)
	}
	data["permissions"] = permsJSON

	// Write the file, creating parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling settings: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(path, output, 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}
