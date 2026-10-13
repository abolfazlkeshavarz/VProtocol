package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"

	"vprotocol/internal/config"
	"vprotocol/internal/netstackvpn"
	"vprotocol/internal/router"
	"vprotocol/internal/sshtunnel"
	"vprotocol/internal/tunnel"
)

// ConnectionState is broadcast to the frontend as the "connection:state"
// event so the UI can reflect what's actually happening.
type ConnectionState struct {
	Status    string `json:"status"` // "disconnected" | "connecting" | "connected" | "error"
	ProfileID string `json:"profileId,omitempty"`
	Message   string `json:"message,omitempty"`
	Since     int64  `json:"since,omitempty"` // unix millis, when Status == "connected"
}

// App is the Wails-bound backend. Its exported methods are callable
// directly from the React frontend.
type App struct {
	ctx   context.Context
	store *config.Store

	mu        sync.Mutex
	sshClient *ssh.Client            // set when connected, regardless of mode
	fullTun   *sshtunnel.Tunnel      // set only in "permitTunnel" mode
	fwd       *netstackvpn.Forwarder // set only in "netstack" mode
	dev       *tunnel.Device
	state     ConnectionState
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
	Name           string   `json:"name"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	User           string   `json:"user"`
	AuthMethod     string   `json:"authMethod"` // "password" | "privateKey"
	Password       string   `json:"password"`
	PrivateKeyPath string   `json:"privateKeyPath"`
	SplitTunnel    []string `json:"splitTunnel"`
	Mode           string   `json:"mode"` // "netstack" (default) | "permitTunnel"
}

// AddProfile saves a new server profile and returns it (with its assigned ID).
func (a *App) AddProfile(in ProfileInput) (config.Profile, error) {
	if in.Host == "" || in.User == "" {
		return config.Profile{}, fmt.Errorf("host and user are required")
	}
	p := config.Profile{
		Name:           orDefaultStr(in.Name, in.Host),
		Host:           in.Host,
		Port:           orDefaultPort(in.Port),
		User:           in.User,
		AuthMethod:     orDefaultStr(in.AuthMethod, "password"),
		Password:       in.Password,
		PrivateKeyPath: in.PrivateKeyPath,
		SplitTunnel:    in.SplitTunnel,
		Mode:           orDefaultStr(in.Mode, "netstack"),
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

// Connect establishes the SSH VPN tunnel for the given profile ID.
// This requires administrator/root privileges to create the TUN interface
// and modify routes — the OS will prompt accordingly if not already
// elevated.
//
// By default ("netstack" mode) this works against any ordinary SSH server:
// it dials a plain SSH connection and forwards each individual TCP
// connection over it as a standard direct-tcpip channel (the same mechanism
// `ssh -L`/`-D` use), which needs no special sshd configuration. If the
// profile explicitly opts into "permitTunnel" mode, it instead uses the
// faster whole-interface tun@openssh.com channel, which requires the
// server to have `PermitTunnel yes` set.
func (a *App) Connect(profileID string) error {
	profile, ok := a.store.Find(profileID)
	if !ok {
		return fmt.Errorf("profile not found: %s", profileID)
	}
	mode := orDefaultStr(profile.Mode, "netstack")

	a.mu.Lock()
	if a.sshClient != nil {
		a.mu.Unlock()
		return fmt.Errorf("already connected; disconnect first")
	}
	a.mu.Unlock()

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

	if mode == "permitTunnel" {
		return a.connectPermitTunnel(profileID, profile, sshCfg)
	}
	return a.connectNetstack(profileID, profile, sshCfg)
}

// connectNetstack is the default, universally-compatible path: plain SSH
// connection, per-connection direct-tcpip forwarding via a userspace
// TCP/IP stack sitting on top of the real TUN device.
func (a *App) connectNetstack(profileID string, profile config.Profile, sshCfg sshtunnel.Config) error {
	client, err := sshtunnel.DialClient(sshCfg)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "creating network interface..."})

	dev, err := tunnel.Open(tunnel.DefaultConfig())
	if err != nil {
		client.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	if err := router.Apply(dev.Name(), profile.SplitTunnel); err != nil {
		dev.Close()
		client.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	fwd, err := netstackvpn.New(dev, client)
	if err != nil {
		_ = router.Clear(dev.Name(), profile.SplitTunnel)
		dev.Close()
		client.Close()
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}
	fwd.Logf = func(format string, args ...any) {
		runtime.LogDebugf(a.ctx, format, args...)
	}

	a.mu.Lock()
	a.sshClient = client
	a.fwd = fwd
	a.dev = dev
	a.mu.Unlock()

	a.store.Touch(profileID)
	_ = a.store.Save()

	a.setState(ConnectionState{
		Status:    "connected",
		ProfileID: profileID,
		Message:   fmt.Sprintf("connected via %s (compatible mode)", dev.Name()),
		Since:     time.Now().UnixMilli(),
	})

	go func() {
		fwd.Run() // blocks until Close()
		a.mu.Lock()
		a.sshClient = nil
		a.fwd = nil
		a.dev = nil
		a.mu.Unlock()
		_ = router.Clear(dev.Name(), profile.SplitTunnel)
		dev.Close()
		client.Close()
		a.setState(ConnectionState{Status: "disconnected"})
	}()

	return nil
}

// connectPermitTunnel is the opt-in fast path for servers configured with
// `PermitTunnel yes`.
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

func orDefaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func orDefaultPort(p int) int {
	if p == 0 {
		return 22
	}
	return p
}
