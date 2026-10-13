// Package config manages VProtocol server profiles: saved SSH VPN
// connection details, persisted as JSON in the user's config directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Profile is one saved SSH VPN server connection.
type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`

	AuthMethod string `json:"authMethod"` // "password" | "privateKey"

	// Mode selects how the tunnel is driven:
	//   "netstack"     - (default) per-connection forwarding over plain SSH
	//                     direct-tcpip channels via a userspace TCP/IP stack.
	//                     Works with any default-configured SSH server.
	//   "permitTunnel" - whole-interface tun@openssh.com forwarding. Faster,
	//                     but requires `PermitTunnel yes` in the server's
	//                     sshd_config (and usually root there).
	Mode string `json:"mode,omitempty"`
	// Password is stored only if the user opts in; prefer PrivateKeyPath.
	Password       string `json:"password,omitempty"`
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`

	HostKeyFingerprint string `json:"hostKeyFingerprint,omitempty"`

	// SplitTunnel, if non-empty, lists CIDRs to route through the tunnel.
	// Empty means full-tunnel (route everything).
	SplitTunnel []string `json:"splitTunnel,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed,omitempty"`
}

// Store holds the full set of saved profiles and knows how to persist them.
type Store struct {
	path     string
	Profiles []Profile `json:"profiles"`
}

// DefaultPath returns the standard per-OS location for the profile store.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "VProtocol", "profiles.json"), nil
}

// Load reads the profile store from path, creating an empty one if it
// doesn't exist yet.
func Load(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return s, nil
}

// Save writes the store back to disk, creating parent directories as
// needed, with 0600 permissions since profiles may contain secrets.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("config: create config dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal store: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", s.path, err)
	}
	return nil
}

// Add appends a profile, assigning it an ID and CreatedAt if missing.
func (s *Store) Add(p Profile) Profile {
	if p.ID == "" {
		p.ID = fmt.Sprintf("p_%d", time.Now().UnixNano())
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	s.Profiles = append(s.Profiles, p)
	return p
}

// Remove deletes the profile with the given ID, if present.
func (s *Store) Remove(id string) bool {
	for i, p := range s.Profiles {
		if p.ID == id {
			s.Profiles = append(s.Profiles[:i], s.Profiles[i+1:]...)
			return true
		}
	}
	return false
}

// Find returns the profile with the given ID, if present.
func (s *Store) Find(id string) (Profile, bool) {
	for _, p := range s.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Touch updates LastUsed to now for the given profile ID.
func (s *Store) Touch(id string) {
	for i, p := range s.Profiles {
		if p.ID == id {
			s.Profiles[i].LastUsed = time.Now()
			return
		}
	}
}
