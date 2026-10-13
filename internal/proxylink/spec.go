// Package proxylink parses the share-link formats used across the
// VLESS/VMess/Trojan/Shadowsocks/SOCKS5 ecosystem (vless://, vmess://,
// trojan://, ss://, socks5://) into one normalized Spec, and builds the
// corresponding Xray-core JSON outbound config from it.
package proxylink

import "fmt"

// Protocol identifies which proxy protocol a Spec describes.
type Protocol string

const (
	ProtoVLESS       Protocol = "vless"
	ProtoVMess       Protocol = "vmess"
	ProtoTrojan      Protocol = "trojan"
	ProtoShadowsocks Protocol = "shadowsocks"
	ProtoSOCKS5      Protocol = "socks5"
)

// Spec is the normalized form of a parsed share link, covering the fields
// common across the ecosystem's subscription/share-link generators
// (Xray's own export, v2rayN, NekoBox). Fields not relevant to a given
// Protocol/Network/Security combination are simply left zero.
type Spec struct {
	Protocol Protocol
	Name     string // display label, from the link's #fragment

	Server string
	Port   int

	// Auth
	UUID     string // vless, vmess
	Password string // trojan, shadowsocks, socks5
	Username string // socks5 (optional)
	Method   string // shadowsocks cipher, e.g. "aes-256-gcm", "chacha20-ietf-poly1305"
	AlterID  int    // vmess legacy (0 for modern "aid=0")
	Flow     string // vless, e.g. "xtls-rprx-vision"

	// Transport
	Network string // "tcp" (default) | "ws" | "grpc" | "httpupgrade"
	Path    string // ws/httpupgrade path
	Host    string // ws/httpupgrade Host header
	Service string // grpc serviceName

	// Security
	Security    string // "" | "tls" | "reality"
	SNI         string
	ALPN        []string
	Fingerprint string // utls fingerprint, e.g. "chrome"
	Insecure    bool

	// REALITY
	PublicKey string
	ShortID   string
	SpiderX   string
}

// Validate does a minimal sanity check so obviously-incomplete profiles
// fail fast with a clear message instead of a confusing Xray startup error.
func (s *Spec) Validate() error {
	if s.Server == "" {
		return fmt.Errorf("proxylink: missing server address")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("proxylink: invalid port %d", s.Port)
	}
	switch s.Protocol {
	case ProtoVLESS, ProtoVMess:
		if s.UUID == "" {
			return fmt.Errorf("proxylink: %s requires a UUID", s.Protocol)
		}
	case ProtoTrojan, ProtoShadowsocks:
		if s.Password == "" {
			return fmt.Errorf("proxylink: %s requires a password", s.Protocol)
		}
		if s.Protocol == ProtoShadowsocks && s.Method == "" {
			return fmt.Errorf("proxylink: shadowsocks requires a cipher method")
		}
	case ProtoSOCKS5:
		// server/port alone is enough; auth is optional
	default:
		return fmt.Errorf("proxylink: unknown protocol %q", s.Protocol)
	}
	return nil
}
