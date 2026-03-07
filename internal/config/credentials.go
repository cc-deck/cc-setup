package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// OAuthCredential represents a single OAuth credential entry stored by Claude Code.
type OAuthCredential struct {
	ServerName   string `json:"serverName"`
	ServerURL    string `json:"serverUrl"`
	ClientID     string `json:"clientId"`
	AccessToken  string `json:"accessToken"`
	ExpiresAt    int64  `json:"expiresAt"`    // milliseconds since epoch
	RefreshToken string `json:"refreshToken"`
	Scope        string `json:"scope"`
}

// IsExpired returns true if the token has expired or will expire within the
// given grace period.
func (c *OAuthCredential) IsExpired(grace time.Duration) bool {
	if c.ExpiresAt == 0 {
		return false // no expiry set, assume valid
	}
	expiryMS := c.ExpiresAt
	nowMS := time.Now().UnixMilli()
	graceMS := grace.Milliseconds()
	return nowMS+graceMS >= expiryMS
}

// credentialsMu serializes reads and writes to the credentials file.
var credentialsMu sync.Mutex

// credentialsPath returns the path to Claude's OAuth credentials file.
func credentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

// credentialsFile is the on-disk format of .credentials.json.
type credentialsFile struct {
	McpOAuth map[string]*OAuthCredential `json:"mcpOAuth"`
}

// LoadOAuthCredentials reads all mcpOAuth entries. It first tries the legacy
// file-based storage (~/.claude/.credentials.json), then falls back to the
// macOS Keychain where Claude Code currently stores OAuth tokens.
func LoadOAuthCredentials() (map[string]*OAuthCredential, error) {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	return loadOAuthCredentialsLocked()
}

// loadOAuthCredentialsLocked reads credentials without acquiring the mutex.
// The caller must hold credentialsMu.
func loadOAuthCredentialsLocked() (map[string]*OAuthCredential, error) {
	// Try file-based storage first (legacy).
	data, err := os.ReadFile(credentialsPath())
	if err == nil {
		var f credentialsFile
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		if len(f.McpOAuth) > 0 {
			return f.McpOAuth, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Fall back to macOS Keychain.
	return loadOAuthFromKeychain()
}

// keychainService is the service name Claude Code uses in macOS Keychain.
const keychainService = "Claude Code-credentials"

// loadOAuthFromKeychain reads OAuth credentials from macOS Keychain using the
// security command. Returns nil, nil if not on macOS or entry not found.
func loadOAuthFromKeychain() (map[string]*OAuthCredential, error) {
	if runtime.GOOS != "darwin" {
		return nil, nil
	}

	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-w").Output()
	if err != nil {
		// Entry not found or keychain locked, not an error for us.
		return nil, nil
	}

	var f credentialsFile
	if err := json.Unmarshal(out, &f); err != nil {
		return nil, nil // malformed data, treat as absent
	}
	return f.McpOAuth, nil
}

// FindCredentialKey returns the map key (e.g. "google-private|19a442662846b0de")
// for a credential matching serverName and optionally serverURL.
// Returns empty string if no match is found.
func FindCredentialKey(creds map[string]*OAuthCredential, serverName, serverURL string) string {
	if len(creds) == 0 || serverName == "" {
		return ""
	}

	// Iterate entries and match by the serverName field stored inside each entry.
	// If multiple entries share the same serverName, use serverURL as a tiebreaker.
	var candidate string
	for key, cred := range creds {
		if cred.ServerName != serverName {
			continue
		}
		// Exact match on both fields is preferred.
		if serverURL != "" && cred.ServerURL == serverURL {
			return key
		}
		// Otherwise remember the first match by serverName.
		if candidate == "" {
			candidate = key
		}
	}
	return candidate
}

// SaveOAuthCredential updates a single entry, preserving all other data.
// It writes to the file-based store if it exists, otherwise to macOS Keychain.
func SaveOAuthCredential(entryKey string, cred *OAuthCredential) error {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()

	path := credentialsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return saveOAuthToKeychain(entryKey, cred)
		}
		return err
	}

	// Preserve unknown top-level keys by using raw JSON.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Parse mcpOAuth sub-object.
	var mcpOAuth map[string]*OAuthCredential
	if existing, ok := raw["mcpOAuth"]; ok {
		if err := json.Unmarshal(existing, &mcpOAuth); err != nil {
			return err
		}
	}
	if mcpOAuth == nil {
		mcpOAuth = make(map[string]*OAuthCredential)
	}

	mcpOAuth[entryKey] = cred

	updated, err := json.Marshal(mcpOAuth)
	if err != nil {
		return err
	}
	raw["mcpOAuth"] = updated

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	return os.WriteFile(path, out, 0o600)
}

// saveOAuthToKeychain updates a single credential in the macOS Keychain entry.
func saveOAuthToKeychain(entryKey string, cred *OAuthCredential) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// Read existing keychain data.
	var f credentialsFile
	out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-w").Output()
	if err == nil {
		_ = json.Unmarshal(out, &f)
	}
	if f.McpOAuth == nil {
		f.McpOAuth = make(map[string]*OAuthCredential)
	}
	f.McpOAuth[entryKey] = cred

	data, err := json.Marshal(f)
	if err != nil {
		return err
	}

	// Delete old entry then add updated one (security doesn't support in-place update).
	_ = exec.Command("security", "delete-generic-password", "-s", keychainService).Run()
	return exec.Command("security", "add-generic-password",
		"-s", keychainService,
		"-a", os.Getenv("USER"),
		"-w", string(data),
		"-U",
	).Run()
}
