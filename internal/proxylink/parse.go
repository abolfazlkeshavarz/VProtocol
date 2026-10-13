package proxylink

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Parse detects the scheme of link and parses it into a Spec. Supported
// schemes: vless://, vmess://, trojan://, ss://, socks5:// (socks:// also
// accepted as an alias).
func Parse(link string) (*Spec, error) {
	link = strings.TrimSpace(link)
	scheme, _, ok := strings.Cut(link, "://")
	if !ok {
		return nil, fmt.Errorf("proxylink: not a recognized share link")
	}

	var spec *Spec
	var err error
	switch strings.ToLower(scheme) {
	case "vless":
		spec, err = parseVLESS(link)
	case "vmess":
		spec, err = parseVMess(link)
	case "trojan":
		spec, err = parseTrojan(link)
	case "ss":
		spec, err = parseShadowsocks(link)
	case "socks5", "socks":
		spec, err = parseSOCKS5(link)
	default:
		return nil, fmt.Errorf("proxylink: unsupported scheme %q", scheme)
	}
	if err != nil {
		return nil, err
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return spec, nil
}

func decodeName(fragment string) string {
	name, err := url.QueryUnescape(fragment)
	if err != nil {
		return fragment
	}
	return name
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseVLESS handles vless://uuid@host:port?params#name
func parseVLESS(link string) (*Spec, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("proxylink: parse vless link: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("proxylink: vless: invalid port: %w", err)
	}
	q := u.Query()
	return &Spec{
		Protocol:    ProtoVLESS,
		Name:        decodeName(u.Fragment),
		Server:      u.Hostname(),
		Port:        port,
		UUID:        u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     orDefault(q.Get("type"), "tcp"),
		Path:        q.Get("path"),
		Host:        q.Get("host"),
		Service:     q.Get("serviceName"),
		Security:    q.Get("security"),
		SNI:         q.Get("sni"),
		ALPN:        splitCSV(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || q.Get("allowInsecure") == "true",
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		SpiderX:     q.Get("spx"),
	}, nil
}

// parseTrojan handles trojan://password@host:port?params#name — the same
// query parameter set as vless for transport/security.
func parseTrojan(link string) (*Spec, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("proxylink: parse trojan link: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("proxylink: trojan: invalid port: %w", err)
	}
	q := u.Query()
	security := q.Get("security")
	if security == "" {
		security = "tls" // trojan implies TLS unless explicitly overridden
	}
	return &Spec{
		Protocol:    ProtoTrojan,
		Name:        decodeName(u.Fragment),
		Server:      u.Hostname(),
		Port:        port,
		Password:    u.User.Username(),
		Network:     orDefault(q.Get("type"), "tcp"),
		Path:        q.Get("path"),
		Host:        q.Get("host"),
		Service:     q.Get("serviceName"),
		Security:    security,
		SNI:         q.Get("sni"),
		ALPN:        splitCSV(q.Get("alpn")),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || q.Get("allowInsecure") == "true",
	}, nil
}

// vmessJSON mirrors the de-facto vmess share-link JSON schema (as produced
// by v2rayN/NekoBox/Xray's own client export).
type vmessJSON struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"` // sometimes a string, sometimes a number
	ID   string `json:"id"`
	AID  any    `json:"aid"`
	SCY  string `json:"scy"`
	Net  string `json:"net"`
	Type string `json:"type"` // header type, usually "none"
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	ALPN string `json:"alpn"`
	FP   string `json:"fp"`
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.Itoa(int(t))
	default:
		return ""
	}
}

// parseVMess handles vmess://BASE64(JSON).
func parseVMess(link string) (*Spec, error) {
	raw := strings.TrimPrefix(link, "vmess://")
	raw = strings.SplitN(raw, "#", 2)[0]
	data, err := base64DecodeLoose(raw)
	if err != nil {
		return nil, fmt.Errorf("proxylink: vmess: base64 decode: %w", err)
	}
	var vj vmessJSON
	if err := json.Unmarshal(data, &vj); err != nil {
		return nil, fmt.Errorf("proxylink: vmess: invalid JSON payload: %w", err)
	}
	port, err := strconv.Atoi(anyToString(vj.Port))
	if err != nil {
		return nil, fmt.Errorf("proxylink: vmess: invalid port: %w", err)
	}
	aid, _ := strconv.Atoi(anyToString(vj.AID))

	security := ""
	if vj.TLS == "tls" || vj.TLS == "reality" {
		security = vj.TLS
	}

	return &Spec{
		Protocol:    ProtoVMess,
		Name:        vj.PS,
		Server:      vj.Add,
		Port:        port,
		UUID:        vj.ID,
		AlterID:     aid,
		Method:      orDefault(vj.SCY, "auto"),
		Network:     orDefault(vj.Net, "tcp"),
		Path:        vj.Path,
		Host:        vj.Host,
		Service:     vj.Path, // grpc serviceName is carried in "path" for vmess links
		Security:    security,
		SNI:         vj.SNI,
		ALPN:        splitCSV(vj.ALPN),
		Fingerprint: vj.FP,
	}, nil
}

// parseShadowsocks handles both ss:// forms:
//
//	ss://BASE64(method:password)@host:port#name
//	ss://BASE64(method:password@host:port)#name   (legacy, fully encoded)
func parseShadowsocks(link string) (*Spec, error) {
	rest := strings.TrimPrefix(link, "ss://")
	rest, fragment, _ := strings.Cut(rest, "#")

	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userinfo, hostport := rest[:at], rest[at+1:]
		hostport = strings.SplitN(hostport, "?", 2)[0]

		decoded, err := base64DecodeLoose(userinfo)
		methodPass := userinfo
		if err == nil {
			methodPass = string(decoded)
		}
		method, password, ok := strings.Cut(methodPass, ":")
		if !ok {
			return nil, fmt.Errorf("proxylink: shadowsocks: malformed method:password")
		}
		host, portStr, err := splitHostPort(hostport)
		if err != nil {
			return nil, fmt.Errorf("proxylink: shadowsocks: %w", err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("proxylink: shadowsocks: invalid port: %w", err)
		}
		return &Spec{
			Protocol: ProtoShadowsocks,
			Name:     decodeName(fragment),
			Server:   host,
			Port:     port,
			Method:   method,
			Password: password,
		}, nil
	}

	// Legacy fully-encoded form.
	decoded, err := base64DecodeLoose(rest)
	if err != nil {
		return nil, fmt.Errorf("proxylink: shadowsocks: base64 decode: %w", err)
	}
	methodPass, hostport, ok := strings.Cut(string(decoded), "@")
	if !ok {
		return nil, fmt.Errorf("proxylink: shadowsocks: malformed legacy link")
	}
	method, password, ok := strings.Cut(methodPass, ":")
	if !ok {
		return nil, fmt.Errorf("proxylink: shadowsocks: malformed method:password")
	}
	host, portStr, err := splitHostPort(hostport)
	if err != nil {
		return nil, fmt.Errorf("proxylink: shadowsocks: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("proxylink: shadowsocks: invalid port: %w", err)
	}
	return &Spec{
		Protocol: ProtoShadowsocks,
		Name:     decodeName(fragment),
		Server:   host,
		Port:     port,
		Method:   method,
		Password: password,
	}, nil
}

// parseSOCKS5 handles socks5://[user:pass@]host:port#name
func parseSOCKS5(link string) (*Spec, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("proxylink: parse socks5 link: %w", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("proxylink: socks5: invalid port: %w", err)
	}
	password, _ := u.User.Password()
	return &Spec{
		Protocol: ProtoSOCKS5,
		Name:     decodeName(u.Fragment),
		Server:   u.Hostname(),
		Port:     port,
		Username: u.User.Username(),
		Password: password,
	}, nil
}

func splitHostPort(hostport string) (host, port string, err error) {
	i := strings.LastIndex(hostport, ":")
	if i < 0 {
		return "", "", fmt.Errorf("missing port in %q", hostport)
	}
	return hostport[:i], hostport[i+1:], nil
}

// base64DecodeLoose tries standard, URL-safe, and unpadded variants, since
// different generators pad/encode differently.
func base64DecodeLoose(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	} {
		if data, err := enc.DecodeString(s); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no base64 variant matched")
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
