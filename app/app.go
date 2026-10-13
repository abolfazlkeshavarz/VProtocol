package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"vprotocol/internal/config"
	"vprotocol/internal/lanshare"
	"vprotocol/internal/netstackvpn"
	"vprotocol/internal/proxylink"
	"vprotocol/internal/router"
	"vprotocol/internal/socks5dial"
	"vprotocol/internal/sshtunnel"
	"vprotocol/internal/tunnel"
	"vprotocol/internal/xraycore"
)

// ConnectionState is broadcast to the frontend as the "connection:state"
// event so the UI can reflect what's actually happening.
type ConnectionState struct {
	Status    string `json:"status"` // "disconnected" | "connecting" | "connected" | "error"
	ProfileID string `json:"profileId,omitempty"`
	Message   string `json:"message,omitempty"`
	Since     int64  `json:"since,omitempty"` // unix millis, when Status == "connected"
}

// LANShareState is broadcast as the "lanshare:state" event.
type LANShareState struct {
	Active bool     `json:"active"`
	Addrs  []string `json:"addrs,omitempty"` // "192.168.x.x:PORT" per local network interface
	Error  string   `json:"error,omitempty"`
}

// App is the Wails-bound backend. Its exported methods are callable
// directly from the React frontend.
type App struct {
	ctx   context.Context
	store *config.Store

	mu           sync.Mutex
	sshClient    io.Closer              // closed on disconnect when protocol == ssh
	xray         *xraycore.Instance     // set when protocol is vless/vmess/trojan/shadowsocks
	fullTun      *sshtunnel.Tunnel      // set only in ssh "permitTunnel" mode
	fwd          *netstackvpn.Forwarder // set for every mode except permitTunnel
	dev          *tunnel.Device
	state        ConnectionState
	activeDialer netstackvpn.Dialer // mirrors fwd's dialer, so LAN sharing can reuse it
	lan          *lanshare.Server
	lanState     LANShareState
}

func NewApp() *App {
	return &App{state: ConnectionState{Status: "disconnected"}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	path, err := config.DefaultPath()
	if err != nil {
		a.store = &config.Store{}
		return
	}
	store, err := config.Load(path)
	if err != nil {
		store = &config.Store{}
	}
	a.store = store
}

// ----- Profiles -----

// ListProfiles returns all saved server profiles.
func (a *App) ListProfiles() []config.Profile {
	return a.store.Profiles
}

// ProfileInput is what the frontend submits when adding a server.
type ProfileInput struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "ssh" | "vless" | "vmess" | "trojan" | "shadowsocks" | "socks5"

	// SSH
	User           string `json:"user"`
	AuthMethod     string `json:"authMethod"` // "password" | "privateKey"
	Password       string `json:"password"`
	PrivateKeyPath string `json:"privateKeyPath"`
	Mode           string `json:"mode"` // "netstack" (default) | "permitTunnel"

	// VLESS / VMess / Trojan / Shadowsocks / SOCKS5
	UUID          string   `json:"uuid"`
	ProxyPassword string   `json:"proxyPassword"`
	ProxyUsername string   `json:"proxyUsername"`
	Method        string   `json:"method"`
	AlterID       int      `json:"alterId"`
	Flow          string   `json:"flow"`
	Network       string   `json:"network"`
	WSPath        string   `json:"wsPath"`
	WSHost        string   `json:"wsHost"`
	GRPCService   string   `json:"grpcService"`
	Security      string   `json:"security"`
	SNI           string   `json:"sni"`
	ALPN          []string `json:"alpn"`
	Fingerprint   string   `json:"fingerprint"`
	Insecure      bool     `json:"insecure"`
	PublicKey     string   `json:"publicKey"`
	ShortID       string   `json:"shortId"`
	SpiderX       string   `json:"spiderX"`

	SplitTunnel []string `json:"splitTunnel"`
}

// ParseLink parses a vless://, vmess://, trojan://, ss:// or socks5://
// share link and returns it as a ProfileInput ready for the Add Server
// form to show (and for the user to adjust before saving).
func (a *App) ParseLink(link string) (ProfileInput, error) {
	spec, err := proxylink.Parse(link)
	if err != nil {
		return ProfileInput{}, err
	}
	var p config.Profile
	p.FromSpec(spec)
	return ProfileInput{
		Name:          p.Name,
		Host:          p.Host,
		Port:          p.Port,
		Protocol:      p.Protocol,
		UUID:          p.UUID,
		ProxyPassword: p.ProxyPassword,
		ProxyUsername: p.ProxyUsername,
		Method:        p.Method,
		AlterID:       p.AlterID,
		Flow:          p.Flow,
		Network:       p.Network,
		WSPath:        p.WSPath,
		WSHost:        p.WSHost,
		GRPCService:   p.GRPCService,
		Security:      p.Security,
		SNI:           p.SNI,
		ALPN:          p.ALPN,
		Fingerprint:   p.Fingerprint,
		Insecure:      p.Insecure,
		PublicKey:     p.PublicKey,
		ShortID:       p.ShortID,
		SpiderX:       p.SpiderX,
	}, nil
}

// AddProfile saves a new server profile and returns it (with its assigned ID).
func (a *App) AddProfile(in ProfileInput) (config.Profile, error) {
	if in.Host == "" {
		return config.Profile{}, fmt.Errorf("host is required")
	}
	protocol := orDefaultStr(in.Protocol, "ssh")
	if protocol == "ssh" && in.User == "" {
		return config.Profile{}, fmt.Errorf("username is required for SSH")
	}

	p := config.Profile{
		Name:           orDefaultStr(in.Name, in.Host),
		Host:           in.Host,
		Port:           orDefaultPort(in.Port, protocol),
		Protocol:       protocol,
		User:           in.User,
		AuthMethod:     orDefaultStr(in.AuthMethod, "password"),
		Password:       in.Password,
		PrivateKeyPath: in.PrivateKeyPath,
		Mode:           orDefaultStr(in.Mode, "netstack"),
		SplitTunnel:    in.SplitTunnel,

		UUID:          in.UUID,
		ProxyPassword: in.ProxyPassword,
		ProxyUsername: in.ProxyUsername,
		Method:        in.Method,
		AlterID:       in.AlterID,
		Flow:          in.Flow,
		Network:       in.Network,
		WSPath:        in.WSPath,
		WSHost:        in.WSHost,
		GRPCService:   in.GRPCService,
		Security:      in.Security,
		SNI:           in.SNI,
		ALPN:          in.ALPN,
		Fingerprint:   in.Fingerprint,
		Insecure:      in.Insecure,
		PublicKey:     in.PublicKey,
		ShortID:       in.ShortID,
		SpiderX:       in.SpiderX,
	}
	saved := a.store.Add(p)
	if err := a.store.Save(); err != nil {
		return config.Profile{}, err
	}
	return saved, nil
}

// RemoveProfile deletes a saved profile by ID.
func (a *App) RemoveProfile(id string) error {
	a.store.Remove(id)
	return a.store.Save()
}

// ----- Connection lifecycle -----

// GetState returns the current connection state (for the UI to sync on load).
func (a *App) GetState() ConnectionState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// Connect establishes the VPN tunnel for the given profile ID, dispatching
// to the right backend for its protocol. This requires administrator/root
// privileges to create the TUN interface and modify routes — the OS will
// prompt accordingly if not already elevated.
func (a *App) Connect(profileID string) error {
	profile, ok := a.store.Find(profileID)
	if !ok {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	a.mu.Lock()
	busy := a.fwd != nil || a.fullTun != nil
	a.mu.Unlock()
	if busy {
		return fmt.Errorf("already connected; disconnect first")
	}

	switch profile.ProtocolOrDefault() {
	case "ssh":
		return a.connectSSH(profileID, profile)
	case "socks5":
		return a.connectSOCKS5(profileID, profile)
	case "vless", "vmess", "trojan", "shadowsocks":
		return a.connectViaXray(profileID, profile)
	default:
		err := fmt.Errorf("unknown protocol %q", profile.Protocol)
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
}

// connectSSH dials the server via SSH, then either per-connection forwards
// through the netstack (works with any server) or, if the profile opted
// into "permitTunnel" mode, uses the faster whole-interface
// tun@openssh.com channel (requires PermitTunnel yes server-side).
func (a *App) connectSSH(profileID string, profile config.Profile) error {
	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "dialing SSH server..."})

	sshCfg := sshtunnel.Config{
		Host:               profile.Host,
		Port:               profile.Port,
		User:               profile.User,
		HostKeyFingerprint: profile.HostKeyFingerprint,
		DialTimeout:        15 * time.Second,
	}
	switch profile.AuthMethod {
	case "privateKey":
		sshCfg.Auth = sshtunnel.AuthPrivateKey
		key, err := os.ReadFile(profile.PrivateKeyPath)
		if err != nil {
			a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: "read private key: " + err.Error()})
			return err
		}
		sshCfg.PrivateKey = key
	default:
		sshCfg.Auth = sshtunnel.AuthPassword
		sshCfg.Password = profile.Password
	}

	if orDefaultStr(profile.Mode, "netstack") == "permitTunnel" {
		return a.connectPermitTunnel(profileID, profile, sshCfg)
	}

	client, err := sshtunnel.DialClient(sshCfg)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
	return a.bringUpTunnel(profileID, profile, client, client, "SSH (compatible mode)")
}

// connectSOCKS5 uses the profile's server directly as a SOCKS5 upstream —
// no Xray instance needed, since SOCKS5 requires no protocol translation.
func (a *App) connectSOCKS5(profileID string, profile config.Profile) error {
	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "connecting to SOCKS5 server..."})

	addr := fmt.Sprintf("%s:%d", profile.Host, profile.Port)
	dialer, err := socks5dial.New(addr, profile.ProxyUsername, profile.ProxyPassword)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
	return a.bringUpTunnel(profileID, profile, dialer, nil, "SOCKS5")
}

// connectViaXray builds an Xray-core outbound for the profile's protocol
// (VLESS/VMess/Trojan/Shadowsocks), runs it in-process behind a local
// SOCKS5 inbound, and forwards through that.
func (a *App) connectViaXray(profileID string, profile config.Profile) error {
	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: fmt.Sprintf("starting %s...", profile.Protocol)})

	spec := profile.ToSpec()
	if err := spec.Validate(); err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
	outbound, err := spec.XrayOutbound()
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	inst, err := xraycore.Start(outbound)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	dialer, err := socks5dial.New(inst.SocksAddr(), "", "")
	if err != nil {
		inst.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	a.mu.Lock()
	a.xray = inst
	a.mu.Unlock()

	return a.bringUpTunnel(profileID, profile, dialer, nil, profile.Protocol)
}

// bringUpTunnel is the shared plumbing for every protocol that forwards
// per-connection through a Dialer (everything except ssh "permitTunnel"
// mode, which pumps a whole dedicated interface instead): create the TUN
// device, apply routes, start the netstack forwarder, and wire up its
// teardown. closer (optional) is closed alongside the tunnel on disconnect
// — the SSH client for "ssh" profiles, nil for the others (the Xray
// instance and SOCKS dialer have their own lifecycle, handled by caller /
// have nothing to close).
func (a *App) bringUpTunnel(profileID string, profile config.Profile, dialer netstackvpn.Dialer, closer io.Closer, label string) error {
	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "creating network interface..."})

	dev, err := tunnel.Open(tunnel.DefaultConfig())
	if err != nil {
		closeAll(closer)
		a.closeXrayIfAny()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	if err := router.Apply(dev.Name(), profile.SplitTunnel); err != nil {
		dev.Close()
		closeAll(closer)
		a.closeXrayIfAny()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	fwd, err := netstackvpn.New(dev, dialer)
	if err != nil {
		_ = router.Clear(dev.Name(), profile.SplitTunnel)
		dev.Close()
		closeAll(closer)
		a.closeXrayIfAny()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
	fwd.Logf = func(format string, args ...any) {
		runtime.LogDebugf(a.ctx, format, args...)
	}

	a.mu.Lock()
	a.sshClient = closer
	a.fwd = fwd
	a.dev = dev
	a.activeDialer = dialer
	a.mu.Unlock()

	a.store.Touch(profileID)
	_ = a.store.Save()

	a.setState(ConnectionState{
		Status:    "connected",
		ProfileID: profileID,
		Message:   fmt.Sprintf("connected via %s on %s", label, dev.Name()),
		Since:     time.Now().UnixMilli(),
	})

	go func() {
		fwd.Run() // blocks until Close()
		a.mu.Lock()
		a.sshClient = nil
		a.fwd = nil
		a.dev = nil
		a.activeDialer = nil
		a.mu.Unlock()
		_ = router.Clear(dev.Name(), profile.SplitTunnel)
		dev.Close()
		closeAll(closer)
		a.closeXrayIfAny()
		a.stopLANShareInternal()
		a.setState(ConnectionState{Status: "disconnected"})
	}()

	return nil
}

func (a *App) closeXrayIfAny() {
	a.mu.Lock()
	inst := a.xray
	a.xray = nil
	a.mu.Unlock()
	if inst != nil {
		inst.Close()
	}
}

func closeAll(c io.Closer) {
	if c != nil {
		c.Close()
	}
}

// connectPermitTunnel is the opt-in fast path for SSH servers configured
// with `PermitTunnel yes`.
func (a *App) connectPermitTunnel(profileID string, profile config.Profile, sshCfg sshtunnel.Config) error {
	t, err := sshtunnel.Dial(sshCfg)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "creating network interface..."})

	dev, err := tunnel.Open(tunnel.DefaultConfig())
	if err != nil {
		t.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	if err := router.Apply(dev.Name(), profile.SplitTunnel); err != nil {
		dev.Close()
		t.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	a.mu.Lock()
	a.fullTun = t
	a.dev = dev
	a.mu.Unlock()

	a.store.Touch(profileID)
	_ = a.store.Save()

	a.setState(ConnectionState{
		Status:    "connected",
		ProfileID: profileID,
		Message:   fmt.Sprintf("connected via %s (PermitTunnel mode)", dev.Name()),
		Since:     time.Now().UnixMilli(),
	})

	go func() {
		err := t.Run(dev)
		a.mu.Lock()
		a.fullTun = nil
		a.dev = nil
		a.mu.Unlock()
		_ = router.Clear(dev.Name(), profile.SplitTunnel)
		dev.Close()
		a.stopLANShareInternal()

		if err != nil {
			a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		} else {
			a.setState(ConnectionState{Status: "disconnected"})
		}
	}()

	return nil
}

// Disconnect tears down the active tunnel, if any, in whichever mode it's
// running.
func (a *App) Disconnect() error {
	a.mu.Lock()
	fwd := a.fwd
	full := a.fullTun
	a.mu.Unlock()

	if fwd != nil {
		fwd.Close()
		return nil
	}
	if full != nil {
		return full.Close()
	}
	a.setState(ConnectionState{Status: "disconnected"})
	return nil
}

func (a *App) setState(s ConnectionState) {
	a.mu.Lock()
	a.state = s
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "connection:state", s)
	}
}

// ----- LAN proxy sharing -----
// Shares the currently-active tunnel as an HTTP CONNECT + SOCKS5 proxy on
// the local network, so other devices (phones, other computers) can use
// this machine's VPN connection without installing anything themselves.

// GetLANShareState returns the current LAN-sharing state.
func (a *App) GetLANShareState() LANShareState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lanState
}

// StartLANShare starts the local HTTP+SOCKS5 proxy on the given port,
// bound to 0.0.0.0 so other devices on the LAN can reach it. Requires an
// active tunnel (permitTunnel mode doesn't expose a per-connection dialer,
// so sharing isn't available in that mode).
func (a *App) StartLANShare(port int) error {
	a.mu.Lock()
	dialer := a.activeDialer
	existing := a.lan
	a.mu.Unlock()

	if existing != nil {
		return fmt.Errorf("already sharing")
	}
	if dialer == nil {
		err := fmt.Errorf("connect to a server first (LAN sharing isn't available in PermitTunnel mode)")
		a.setLANState(LANShareState{Active: false, Error: err.Error()})
		return err
	}
	if port == 0 {
		// Not 1080 (the SOCKS5 convention): on Windows that port commonly
		// falls inside the OS's dynamic/Hyper-V port exclusion range,
		// making bind() fail with an access-denied error that looks like a
		// permissions bug but is really just a bad default port choice.
		port = 18080
	}

	srv, addrs, err := lanshare.Start(port, dialer)
	if err != nil {
		if isWindowsPortExcluded(err) {
			err = fmt.Errorf("port %d is reserved by Windows (often for Hyper-V/WSL) — try a different port, e.g. 18080 or 28080: %w", port, err)
		}
		a.setLANState(LANShareState{Active: false, Error: err.Error()})
		return err
	}

	a.mu.Lock()
	a.lan = srv
	a.mu.Unlock()

	a.setLANState(LANShareState{Active: true, Addrs: addrs})
	return nil
}

// StopLANShare stops sharing, if active.
func (a *App) StopLANShare() error {
	a.stopLANShareInternal()
	return nil
}

func (a *App) stopLANShareInternal() {
	a.mu.Lock()
	srv := a.lan
	a.lan = nil
	a.mu.Unlock()
	if srv != nil {
		srv.Close()
		a.setLANState(LANShareState{Active: false})
	}
}

func (a *App) setLANState(s LANShareState) {
	a.mu.Lock()
	a.lanState = s
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "lanshare:state", s)
	}
}

// isWindowsPortExcluded heuristically detects the "access forbidden" bind
// error Windows raises for ports inside its dynamic/Hyper-V/WSL port
// exclusion ranges (visible via `netsh interface ipv4 show
// excludedportrange protocol=tcp`). There's no portable error value for
// this, so this matches on the OS's own message text.
func isWindowsPortExcluded(err error) bool {
	return strings.Contains(err.Error(), "forbidden by its access permissions")
}

func orDefaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func orDefaultPort(p int, protocol string) int {
	if p != 0 {
		return p
	}
	switch protocol {
	case "ssh":
		return 22
	case "socks5":
		return 1080
	default:
		return 443
	}
}
