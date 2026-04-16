package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mcpJSONName = ".mcp.json"

// canonicalCwd returns the current working directory with symlinks and
// filesystem case resolved. On macOS (case-insensitive APFS), $PWD may
// contain non-canonical casing (e.g. "work" instead of "Work"), causing
// path-keyed lookups in ~/.claude.json to miss entries written by Claude
// Code under the real casing.
//
// filepath.EvalSymlinks resolves symlinks but not case, so we walk each
// path component and match it against actual directory entries to recover
// the on-disk casing.
func canonicalCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		resolved = cwd
	}
	return resolveCase(resolved)
}

// resolveCase walks an absolute path component by component and replaces
// each segment with its actual on-disk name, fixing case mismatches on
// case-insensitive filesystems. Returns the original path on any error.
func resolveCase(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}

	parts := strings.Split(path, string(filepath.Separator))
	// parts[0] is "" for an absolute path starting with /
	resolved := string(filepath.Separator)
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return path // fall back to original
		}
		found := false
		for _, e := range entries {
			if strings.EqualFold(e.Name(), part) {
				resolved = filepath.Join(resolved, e.Name())
				found = true
				break
			}
		}
		if !found {
			return path // component doesn't exist, return original
		}
	}
	return resolved
}

// ConfigPath returns the Claude config file path for a scope.
func ConfigPath(scope string) string {
	if scope == "user" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude.json")
	}
	return filepath.Join(canonicalCwd(), mcpJSONName)
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
	cwd := canonicalCwd()
	if cwd == "" {
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

// ReadDisabledMcpServers reads the disabledMcpjsonServers list from the
// project's .claude/settings.local.json. Claude Code uses this key to
// disable servers defined in .mcp.json files (including inherited ones
// from parent directories).
func ReadDisabledMcpServers() []string {
	path := SettingsPath("project")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	disabledRaw, ok := raw["disabledMcpjsonServers"]
	if !ok {
		return nil
	}

	var disabled []string
	if err := json.Unmarshal(disabledRaw, &disabled); err != nil {
		return nil
	}
	return disabled
}

// WriteDisabledMcpServers writes the disabledMcpjsonServers list into the
// project's .claude/settings.local.json. All other keys are preserved.
// If disabled is empty, the disabledMcpjsonServers key is removed.
func WriteDisabledMcpServers(disabled []string) error {
	path := SettingsPath("project")

	// Read existing file to preserve all other keys
	var data map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &data); err != nil {
			data = make(map[string]json.RawMessage)
		}
	} else {
		data = make(map[string]json.RawMessage)
	}

	// Set or remove disabledMcpjsonServers
	if len(disabled) == 0 {
		delete(data, "disabledMcpjsonServers")
	} else {
		disabledJSON, err := json.Marshal(disabled)
		if err != nil {
			return fmt.Errorf("marshalling disabledMcpjsonServers: %w", err)
		}
		data["disabledMcpjsonServers"] = disabledJSON
	}

	// Write the file, creating parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling settings: %w", err)
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
	return filepath.Join(canonicalCwd(), ".claude", settingsFileName)
}

// ReadAllPermissions reads the full permissions.allow array from settings.local.json
// for the given scope. Unlike ReadToolPermissions, this returns all entries
// (built-in tools, bash patterns, and MCP tools) without filtering.
func ReadAllPermissions(scope string) []string {
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
	return perms.Allow
}

// ReadDenyPermissions reads the full permissions.deny array from settings.local.json.
func ReadDenyPermissions(scope string) []string {
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
		Deny []string `json:"deny"`
	}
	if err := json.Unmarshal(permsRaw, &perms); err != nil {
		return nil
	}
	return perms.Deny
}

// WriteAllPermissions replaces the full permissions.allow and permissions.deny arrays
// in settings.local.json for the given scope, preserving all other keys.
// Returns the written file path.
func WriteAllPermissions(scope string, permissions []string, denied ...[]string) (string, error) {
	path := SettingsPath(scope)

	var data map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &data); err != nil {
			data = make(map[string]json.RawMessage)
		}
	} else {
		data = make(map[string]json.RawMessage)
	}

	// Parse existing permissions map to preserve other keys (deny, ask, etc.)
	permsMap := make(map[string]json.RawMessage)
	if raw, ok := data["permissions"]; ok {
		_ = json.Unmarshal(raw, &permsMap)
	}

	if permissions == nil {
		permissions = []string{}
	}
	allowJSON, err := json.Marshal(permissions)
	if err != nil {
		return "", fmt.Errorf("marshalling allow: %w", err)
	}
	permsMap["allow"] = allowJSON

	// Write deny array if provided
	if len(denied) > 0 {
		denyList := denied[0]
		if denyList == nil {
			denyList = []string{}
		}
		denyJSON, err := json.Marshal(denyList)
		if err != nil {
			return "", fmt.Errorf("marshalling deny: %w", err)
		}
		permsMap["deny"] = denyJSON
	}

	permsJSON, err := json.Marshal(permsMap)
	if err != nil {
		return "", fmt.Errorf("marshalling permissions: %w", err)
	}
	data["permissions"] = permsJSON

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

// ReadPermissionMode reads the permissions.defaultMode from settings.json
// for the given scope.
func ReadPermissionMode(scope string) string {
	path := PluginSettingsPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}

	permsRaw, ok := raw["permissions"]
	if !ok {
		return ""
	}

	var perms struct {
		DefaultMode string `json:"defaultMode"`
	}
	if err := json.Unmarshal(permsRaw, &perms); err != nil {
		return ""
	}
	return perms.DefaultMode
}

// WritePermissionMode writes the permissions.defaultMode to settings.json
// for the given scope, preserving all other keys (enabledPlugins, etc.).
// Returns the written file path.
func WritePermissionMode(scope, mode string) (string, error) {
	path := PluginSettingsPath(scope)

	var data map[string]json.RawMessage
	if content, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(content, &data); err != nil {
			data = make(map[string]json.RawMessage)
		}
	} else {
		data = make(map[string]json.RawMessage)
	}

	// Parse existing permissions map to preserve other keys
	permsMap := make(map[string]json.RawMessage)
	if raw, ok := data["permissions"]; ok {
		_ = json.Unmarshal(raw, &permsMap)
	}

	if mode == "" {
		delete(permsMap, "defaultMode")
	} else {
		modeJSON, err := json.Marshal(mode)
		if err != nil {
			return "", fmt.Errorf("marshalling defaultMode: %w", err)
		}
		permsMap["defaultMode"] = modeJSON
	}

	if len(permsMap) > 0 {
		permsJSON, err := json.Marshal(permsMap)
		if err != nil {
			return "", fmt.Errorf("marshalling permissions: %w", err)
		}
		data["permissions"] = permsJSON
	} else {
		delete(data, "permissions")
	}

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

	// Parse existing permissions as a generic map to preserve unknown keys
	// (e.g. "ask", "defaultMode") that are not part of allow/deny.
	permsMap := make(map[string]json.RawMessage)
	if raw, ok := data["permissions"]; ok {
		_ = json.Unmarshal(raw, &permsMap)
	}

	// Extract allow and deny arrays from the map.
	var allow []string
	if raw, ok := permsMap["allow"]; ok {
		_ = json.Unmarshal(raw, &allow)
	}
	var deny []string
	if raw, ok := permsMap["deny"]; ok {
		_ = json.Unmarshal(raw, &deny)
	}

	prefix := mcpPermPrefix(serverName)

	// Remove all existing entries for this server from allow
	filtered := make([]string, 0, len(allow))
	for _, entry := range allow {
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

	// Build a set of selected tools for deny cleanup
	selectedSet := make(map[string]bool, len(toolNames))
	for _, t := range toolNames {
		selectedSet[t] = true
	}

	// Remove deselected tools from deny to avoid conflicts
	filteredDeny := make([]string, 0, len(deny))
	for _, entry := range deny {
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

	// Write allow and deny back into the permissions map, preserving other keys.
	allowJSON, err := json.Marshal(filtered)
	if err != nil {
		return "", fmt.Errorf("marshalling allow: %w", err)
	}
	permsMap["allow"] = allowJSON

	denyJSON, err := json.Marshal(filteredDeny)
	if err != nil {
		return "", fmt.Errorf("marshalling deny: %w", err)
	}
	permsMap["deny"] = denyJSON

	// Marshal the full permissions map back (preserves "ask", "defaultMode", etc.)
	permsJSON, err := json.Marshal(permsMap)
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
