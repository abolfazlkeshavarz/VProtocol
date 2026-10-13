// Package sshtunnel implements IP-layer VPN tunneling over SSH, using the
// same OpenSSH "tun@openssh.com" channel extension that the stock `ssh -w`
// flag uses. The remote sshd must be configured with `PermitTunnel yes` (and
// the connecting account must be allowed to use it) for this to work; that
// is a standard OpenSSH server-side setting, not anything custom to this
// project.
package sshtunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// tunChannelType is OpenSSH's well-known channel type for point-to-point
// tun forwarding (see PROTOCOL, section 3.2 "tun@openssh.com" in the OpenSSH
// source distribution).
const tunChannelType = "tun@openssh.com"

const (
	tunModePointToPoint = 1 // SSH_TUNMODE_POINTOPOINT
	tunUnitAny          = 0x7fffffff
)

// AuthMethod selects how the client authenticates to the SSH server.
type AuthMethod int

const (
	AuthPassword AuthMethod = iota
	AuthPrivateKey
)

// Config describes one SSH VPN server connection.
type Config struct {
	Host string
	Port int
	User string

	Auth       AuthMethod
	Password   string // used when Auth == AuthPassword
	PrivateKey []byte // PEM-encoded private key, used when Auth == AuthPrivateKey
	Passphrase []byte // optional, decrypts PrivateKey if set

	// HostKeyFingerprint, if set, must match the server's host key
	// (SHA256 base64 form, as printed by `ssh-keygen -lf`). If empty,
	// TOFU (trust-on-first-use) pinning is the caller's responsibility —
	// Tunnel itself does not silently accept unknown host keys.
	HostKeyFingerprint string

	DialTimeout time.Duration
}

// PacketIO is anything the tunnel can pump IP packets to/from — satisfied by
// *tunnel.Device without this package needing to import it, avoiding a
// dependency cycle and keeping this package testable in isolation.
type PacketIO interface {
	ReadPacket(buf []byte) (int, error)
	WritePacket(pkt []byte) error
	MTU() int
}

// Tunnel is an active SSH-VPN session: an SSH connection plus the
// tun@openssh.com channel carrying IP packets.
type Tunnel struct {
	client  *ssh.Client
	channel ssh.Channel
	done    chan struct{}
	errCh   chan error
}

// Dial connects to the SSH server and opens the tun forwarding channel.
// It does not start pumping packets; call Run for that.
func Dial(cfg Config) (*Tunnel, error) {
	authMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, err
	}

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if cfg.HostKeyFingerprint != "" {
		hostKeyCallback = pinnedHostKeyCallback(cfg.HostKeyFingerprint)
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         orDefault(cfg.DialTimeout, 15*time.Second),
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprint(orDefaultInt(cfg.Port, 22)))
	client, err := ssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("sshtunnel: dial %s: %w", addr, err)
	}

	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], tunUnitAny)
	binary.BigEndian.PutUint32(payload[4:8], tunModePointToPoint)

	ch, reqs, err := client.Conn.OpenChannel(tunChannelType, payload)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("sshtunnel: open %s channel (server needs PermitTunnel yes): %w", tunChannelType, err)
	}
	go ssh.DiscardRequests(reqs)

	return &Tunnel{
		client:  client,
		channel: ch,
		done:    make(chan struct{}),
		errCh:   make(chan error, 2),
	}, nil
}

// Run pumps packets bidirectionally between dev (the local TUN device) and
// the SSH tun channel until either side closes or ctx-equivalent Close is
// called. It blocks until the tunnel ends and returns the terminating error,
// if any (io.EOF-like closes are reported as nil).
func (t *Tunnel) Run(dev PacketIO) error {
	go t.pumpDeviceToChannel(dev)
	go t.pumpChannelToDevice(dev)

	select {
	case err := <-t.errCh:
		return err
	case <-t.done:
		return nil
	}
}

func (t *Tunnel) pumpDeviceToChannel(dev PacketIO) {
	buf := make([]byte, dev.MTU()+64)
	for {
		n, err := dev.ReadPacket(buf)
		if err != nil {
			t.reportAndClose(fmt.Errorf("sshtunnel: read from tun: %w", err))
			return
		}
		if n == 0 {
			continue
		}
		if _, err := t.channel.Write(buf[:n]); err != nil {
			t.reportAndClose(fmt.Errorf("sshtunnel: write to channel: %w", err))
			return
		}
	}
}

func (t *Tunnel) pumpChannelToDevice(dev PacketIO) {
	buf := make([]byte, dev.MTU()+64)
	for {
		n, err := t.channel.Read(buf)
		if err != nil {
			if err == io.EOF {
				t.reportAndClose(nil)
				return
			}
			t.reportAndClose(fmt.Errorf("sshtunnel: read from channel: %w", err))
			return
		}
		if n == 0 {
			continue
		}
		if err := dev.WritePacket(buf[:n]); err != nil {
			t.reportAndClose(fmt.Errorf("sshtunnel: write to tun: %w", err))
			return
		}
	}
}

func (t *Tunnel) reportAndClose(err error) {
	select {
	case t.errCh <- err:
	default:
	}
	t.Close()
}

// Close tears down the SSH connection and channel.
func (t *Tunnel) Close() error {
	select {
	case <-t.done:
		return nil
	default:
		close(t.done)
	}
	_ = t.channel.Close()
	return t.client.Close()
}

func buildAuthMethods(cfg Config) ([]ssh.AuthMethod, error) {
	switch cfg.Auth {
	case AuthPassword:
		if cfg.Password == "" {
			return nil, fmt.Errorf("sshtunnel: password auth selected but no password provided")
		}
		return []ssh.AuthMethod{ssh.Password(cfg.Password)}, nil
	case AuthPrivateKey:
		var signer ssh.Signer
		var err error
		if len(cfg.Passphrase) > 0 {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(cfg.PrivateKey, cfg.Passphrase)
		} else {
			signer, err = ssh.ParsePrivateKey(cfg.PrivateKey)
		}
		if err != nil {
			return nil, fmt.Errorf("sshtunnel: parse private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("sshtunnel: unknown auth method %v", cfg.Auth)
	}
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d == 0 {
		return fallback
	}
	return d
}

func orDefaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
