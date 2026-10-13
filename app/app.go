package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"vprotocol/internal/config"
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

	mu       sync.Mutex
	active   *sshtunnel.Tunnel
	dev      *tunnel.Device
	cancel   func()
	state    ConnectionState
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
func (a *App) Connect(profileID string) error {
	profile, ok := a.store.Find(profileID)
	if !ok {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	a.mu.Lock()
	if a.active != nil {
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

	t, err := sshtunnel.Dial(sshCfg)
	if err != nil {
		a.setState(ConnectionState{Status: "error", ProfileID: profileID, Message: err.Error()})
		return err
	}

	a.setState(ConnectionState{Status: "connecting", ProfileID: profileID, Message: "creating network interface..."})

	devCfg := tunnel.DefaultConfig()
	dev, err := tunnel.Open(devCfg)
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
	a.active = t
	a.dev = dev
	a.mu.Unlock()

	a.store.Touch(profileID)
	_ = a.store.Save()

	a.setState(ConnectionState{
		Status:    "connected",
		ProfileID: profileID,
		Message:   fmt.Sprintf("connected via %s", dev.Name()),
		Since:     time.Now().UnixMilli(),
	})

	go func() {
		err := t.Run(dev)
		a.mu.Lock()
		a.active = nil
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

// Disconnect tears down the active tunnel, if any.
func (a *App) Disconnect() error {
	a.mu.Lock()
	t := a.active
	a.mu.Unlock()

	if t == nil {
		a.setState(ConnectionState{Status: "disconnected"})
		return nil
	}
	return t.Close()
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
