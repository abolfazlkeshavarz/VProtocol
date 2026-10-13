// Package config manages VProtocol server profiles: saved SSH VPN
// connection details, persisted as JSON in the user's config directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vprotocol/internal/proxylink"
)

// Profile is one saved VPN server connection — either an SSH server, or a
// VLESS/VMess/Trojan/Shadowsocks/SOCKS5 proxy server.
type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"` // server address, for every protocol
	Port int    `json:"port"`
	User string `json:"user"` // SSH username only

	// Protocol selects what kind of server this profile connects to.
	// "" is treated as "ssh" for backward compatibility with profiles saved
	// before this field existed.
	Protocol string `json:"protocol,omitempty"` // "ssh" | "vless" | "vmess" | "trojan" | "shadowsocks" | "socks5"

	AuthMethod string `json:"authMethod"` // "password" | "privateKey" (ssh only)

	// Mode selects how an SSH tunnel is driven:
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

	// --- VLESS / VMess / Trojan / Shadowsocks / SOCKS5 fields ---
	UUID          string   `json:"uuid,omitempty"`          // vless, vmess
	ProxyPassword string   `json:"proxyPassword,omitempty"` // trojan, shadowsocks, socks5
	ProxyUsername string   `json:"proxyUsername,omitempty"` // socks5 (optional)
	Method        string   `json:"method,omitempty"`        // shadowsocks cipher
	AlterID       int      `json:"alterId,omitempty"`       // vmess legacy
	Flow          string   `json:"flow,omitempty"`          // vless, e.g. xtls-rprx-vision
	Network       string   `json:"network,omitempty"`       // tcp | ws | grpc | httpupgrade
	WSPath        string   `json:"wsPath,omitempty"`
	WSHost        string   `json:"wsHost,omitempty"`
	GRPCService   string   `json:"grpcService,omitempty"`
	Security      string   `json:"security,omitempty"` // "" | tls | reality
	SNI           string   `json:"sni,omitempty"`
	ALPN          []string `json:"alpn,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	Insecure      bool     `json:"insecure,omitempty"`
	PublicKey     string   `json:"publicKey,omitempty"` // reality
	ShortID       string   `json:"shortId,omitempty"`   // reality
	SpiderX       string   `json:"spiderX,omitempty"`   // reality

	// SplitTunnel, if non-empty, lists CIDRs to route through the tunnel.
	// Empty means full-tunnel (route everything).
	SplitTunnel []string `json:"splitTunnel,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed,omitempty"`
}

// ProtocolOrDefault returns Protocol, defaulting to "ssh" for older saved
// profiles that predate this field.
func (p Profile) ProtocolOrDefault() string {
	if p.Protocol == "" {
		return "ssh"
	}
	return p.Protocol
}

// ToSpec converts a non-SSH Profile into a proxylink.Spec, ready to build
// an Xray outbound from (or to use directly, for "socks5").
func (p Profile) ToSpec() *proxylink.Spec {
	return &proxylink.Spec{
		Protocol:    proxylink.Protocol(p.ProtocolOrDefault()),
		Name:        p.Name,
		Server:      p.Host,
		Port:        p.Port,
		UUID:        p.UUID,
		Password:    p.ProxyPassword,
		Username:    p.ProxyUsername,
		Method:      p.Method,
		AlterID:     p.AlterID,
		Flow:        p.Flow,
		Network:     p.Network,
		Path:        p.WSPath,
		Host:        p.WSHost,
		Service:     p.GRPCService,
		Security:    p.Security,
		SNI:         p.SNI,
		ALPN:        p.ALPN,
		Fingerprint: p.Fingerprint,
		Insecure:    p.Insecure,
		PublicKey:   p.PublicKey,
		ShortID:     p.ShortID,
		SpiderX:     p.SpiderX,
	}
}

// FromSpec fills in a Profile's protocol fields from a parsed Spec (used
// when a user pastes a share link). Host/Port/Name are also set from spec.
func (p *Profile) FromSpec(spec *proxylink.Spec) {
	p.Protocol = string(spec.Protocol)
	if spec.Name != "" {
		p.Name = spec.Name
	}
	p.Host = spec.Server
	p.Port = spec.Port
	p.UUID = spec.UUID
	p.ProxyPassword = spec.Password
	p.ProxyUsername = spec.Username
	p.Method = spec.Method
	p.AlterID = spec.AlterID
	p.Flow = spec.Flow
	p.Network = spec.Network
	p.WSPath = spec.Path
	p.WSHost = spec.Host
	p.GRPCService = spec.Service
	p.Security = spec.Security
	p.SNI = spec.SNI
	p.ALPN = spec.ALPN
	p.Fingerprint = spec.Fingerprint
	p.Insecure = spec.Insecure
	p.PublicKey = spec.PublicKey
	p.ShortID = spec.ShortID
	p.SpiderX = spec.SpiderX
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
