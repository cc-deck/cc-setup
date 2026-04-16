package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile represents a permission profile loaded from a YAML file.
type Profile struct {
	SchemaVersion int                `yaml:"schema_version" json:"schema_version"`
	Name          string             `yaml:"name" json:"name"`
	Description   string             `yaml:"description" json:"description"`
	Builtin       bool               `yaml:"builtin" json:"builtin"`
	Mode          string             `yaml:"mode" json:"mode"`
	Permissions   ProfilePermissions `yaml:"permissions" json:"permissions"`
	FilePath      string             `yaml:"-" json:"-"` // populated at load time
}

// ProfilePermissions defines the permission entries within a profile.
type ProfilePermissions struct {
	Allow []string   `yaml:"allow" json:"allow"`
	Deny  []string   `yaml:"deny" json:"deny"`
	Bash  []string   `yaml:"bash" json:"bash"`
	MCP   ProfileMCP `yaml:"mcp" json:"mcp"`
}

// ProfileMCP defines MCP tool classification rules within a profile.
type ProfileMCP struct {
	UseAnnotations bool             `yaml:"use_annotations" json:"use_annotations"`
	Heuristic      ProfileHeuristic `yaml:"heuristic" json:"heuristic"`
	Include        []string         `yaml:"include" json:"include"`
	Exclude        []string         `yaml:"exclude" json:"exclude"`
}

// ProfileHeuristic defines name-based heuristic classification rules.
type ProfileHeuristic struct {
	SafePrefixes   []string `yaml:"safe_prefixes" json:"safe_prefixes"`
	UnsafePrefixes []string `yaml:"unsafe_prefixes" json:"unsafe_prefixes"`
}

// ClassifiedTool is the result of MCP tool classification during profile application.
type ClassifiedTool struct {
	Name     string // Tool name from MCP server
	Approved bool   // Whether the profile considers this tool safe
	Hint     string // Classification source indicator: "[read-only]", "[destructive]", "[heuristic]", ""
}

// Built-in profile YAML content. Underscore-prefixed files are owned by cc-setup
// and overwritten on upgrades.

const readOnlyYOLOProfile = `schema_version: 1
name: Read-only YOLO
description: Non-destructive operations only, auto-approve file edits
builtin: true
mode: acceptEdits
permissions:
  allow:
    - Agent
    - Glob
    - Grep
    - Read
    - Skill
    - ToolSearch
    - WebFetch
    - WebSearch
  bash:
    - "git:*"
    - "git log:*"
    - "git diff:*"
    - "git status"
    - "git branch:*"
    - "ls:*"
    - "cat:*"
    - "head:*"
    - "tail:*"
    - "wc:*"
    - "find:*"
    - "which:*"
    - "echo:*"
    - "date"
    - "pwd"
    - "env"
    - "uname:*"
  mcp:
    use_annotations: true
    heuristic:
      safe_prefixes:
        - list
        - get
        - read
        - search
        - describe
        - show
        - fetch
        - count
        - check
        - view
      unsafe_prefixes:
        - create
        - delete
        - update
        - write
        - execute
        - run
        - modify
        - remove
        - drop
        - send
`

const fullYOLOProfile = `schema_version: 1
name: Full YOLO
description: All permissions enabled, bypass all safety checks
builtin: true
mode: bypassPermissions
permissions:
  allow:
    - Agent
    - Bash
    - Edit
    - Glob
    - Grep
    - Read
    - Skill
    - ToolSearch
    - WebFetch
    - WebSearch
    - Write
  bash:
    - "*"
  mcp:
    use_annotations: false
    include:
      - "*"
`

// ProfilesDir returns the path to the profiles directory.
func ProfilesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cc-setup", "profiles")
}

// EnsureBuiltinProfiles writes/overwrites the built-in profile files to the
// profiles directory. Only underscore-prefixed files are managed; user-created
// profiles are never touched.
func EnsureBuiltinProfiles() error {
	dir := ProfilesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating profiles directory %s: %w", dir, err)
	}

	builtins := map[string]string{
		"_readonly-yolo.yaml": readOnlyYOLOProfile,
		"_full-yolo.yaml":     fullYOLOProfile,
	}

	for name, content := range builtins {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing profile %s: %w", path, err)
		}
	}
	return nil
}

// LoadProfiles scans the profiles directory, parses all .yaml files, and returns
// a sorted slice of profiles. Malformed YAML files are skipped with an error printed.
func LoadProfiles() ([]Profile, error) {
	dir := ProfilesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading profiles directory: %w", err)
	}

	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		p, err := loadProfileFromPath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed profile %s: %v\n", name, err)
			continue
		}
		if warning := ValidateProfileSchema(p); warning != "" {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		// Built-in profiles first, then alphabetical
		if profiles[i].Builtin != profiles[j].Builtin {
			return profiles[i].Builtin
		}
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// LoadProfile loads a single profile by name (filename without .yaml extension).
func LoadProfile(name string) (Profile, error) {
	dir := ProfilesDir()
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return loadProfileFromPath(path)
		}
	}
	// Try with underscore prefix
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, "_"+name+ext)
		if _, err := os.Stat(path); err == nil {
			return loadProfileFromPath(path)
		}
	}
	return Profile{}, fmt.Errorf("profile %q not found", name)
}

// ClassifyTool determines whether a single tool should be approved based on the
// profile's classification rules. The priority is:
// explicit include/exclude > annotations > heuristic > default deny
func ClassifyTool(toolName string, readOnlyHint, destructiveHint, hasAnnotations bool, profile Profile) ClassifiedTool {
	mcp := profile.Permissions.MCP

	// Explicit include/exclude first
	for _, pattern := range mcp.Include {
		if pattern == "*" || matchToolPattern(pattern, toolName) {
			return ClassifiedTool{Name: toolName, Approved: true, Hint: ""}
		}
	}
	for _, pattern := range mcp.Exclude {
		if pattern == "*" || matchToolPattern(pattern, toolName) {
			return ClassifiedTool{Name: toolName, Approved: false, Hint: ""}
		}
	}

	// Annotations (if profile enables them and tool provides them)
	if mcp.UseAnnotations && hasAnnotations {
		if readOnlyHint {
			return ClassifiedTool{Name: toolName, Approved: true, Hint: "[read-only]"}
		}
		if destructiveHint {
			return ClassifiedTool{Name: toolName, Approved: false, Hint: "[destructive]"}
		}
	}

	// Heuristic fallback based on name prefix.
	// Unsafe prefixes are checked first to prevent a malicious tool named
	// e.g. "list_then_delete_all" from matching the safe "list" prefix.
	lower := strings.ToLower(toolName)
	for _, prefix := range mcp.Heuristic.UnsafePrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return ClassifiedTool{Name: toolName, Approved: false, Hint: "[heuristic]"}
		}
	}
	for _, prefix := range mcp.Heuristic.SafePrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return ClassifiedTool{Name: toolName, Approved: true, Hint: "[heuristic]"}
		}
	}

	// Default: not approved
	return ClassifiedTool{Name: toolName, Approved: false, Hint: ""}
}

// matchToolPattern matches a tool name against a pattern.
// Supports simple prefix matching with * wildcard.
func matchToolPattern(pattern, toolName string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(toolName, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == toolName
}

// currentSchemaVersion is the version of the profile schema supported by this build.
const currentSchemaVersion = 1

// ValidateProfileSchema checks if a user profile's schema version is compatible.
// Returns a warning message if migration is needed, empty string if ok.
func ValidateProfileSchema(p Profile) string {
	if p.SchemaVersion == 0 || p.SchemaVersion == currentSchemaVersion {
		return ""
	}
	if p.SchemaVersion > currentSchemaVersion {
		return fmt.Sprintf("profile %q uses schema version %d (this build supports %d), some fields may be ignored",
			p.Name, p.SchemaVersion, currentSchemaVersion)
	}
	return ""
}

func loadProfileFromPath(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parsing YAML: %w", err)
	}
	p.FilePath = path
	return p, nil
}
