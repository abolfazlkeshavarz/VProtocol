// Package tunnel provides a cross-platform abstraction over a TUN virtual
// network interface. On Windows it uses the Wintun driver; on Linux it uses
// the native /dev/net/tun. Both are handled by the WireGuard project's
// well-tested tun implementation, so this package is a thin, VProtocol-shaped
// wrapper around it rather than a reimplementation.
package tunnel

import (
	"fmt"
	"net"
	"net/netip"

	"golang.zx2c4.com/wireguard/tun"
)

// Device wraps a platform TUN device plus the IP configuration applied to it.
type Device struct {
	nt      tun.Device
	name    string
	mtu     int
	addr    netip.Prefix
	closeCh chan struct{}
}

// Config describes how the TUN interface should be created and addressed.
type Config struct {
	// Name is the interface name on Linux (e.g. "vprotocol0"); ignored on
	// Windows, where Wintun assigns the name from the pool/GUID.
	Name string
	// MTU is the interface MTU. 1400 is a safe default that leaves room for
	// SSH + TCP/IP overhead without fragmentation on most paths.
	MTU int
	// Address is the local point-to-point / interface address, e.g.
	// 10.66.0.2/24.
	Address netip.Prefix
}

// DefaultConfig returns sane defaults for a client-side tunnel interface.
func DefaultConfig() Config {
	addr := netip.MustParsePrefix("10.66.0.2/24")
	return Config{
		Name:    "vprotocol0",
		MTU:     1400,
		Address: addr,
	}
}

// Open creates and brings up a TUN device according to cfg. The caller is
// responsible for calling Close when done.
func Open(cfg Config) (*Device, error) {
	if cfg.MTU == 0 {
		cfg.MTU = 1400
	}
	nt, err := tun.CreateTUN(cfg.Name, cfg.MTU)
	if err != nil {
		return nil, fmt.Errorf("tunnel: create TUN device: %w", err)
	}

	realName, err := nt.Name()
	if err != nil {
		realName = cfg.Name
	}

	d := &Device{
		nt:      nt,
		name:    realName,
		mtu:     cfg.MTU,
		addr:    cfg.Address,
		closeCh: make(chan struct{}),
	}

	if err := configureInterface(d); err != nil {
		_ = nt.Close()
		return nil, fmt.Errorf("tunnel: configure interface %s: %w", realName, err)
	}

	return d, nil
}

// Name returns the OS-assigned interface name.
func (d *Device) Name() string { return d.name }

// MTU returns the interface MTU in bytes.
func (d *Device) MTU() int { return d.mtu }

// Address returns the IP prefix assigned to the interface.
func (d *Device) Address() netip.Prefix { return d.addr }

// ReadPacket reads one raw IP packet from the TUN device into buf, returning
// the number of bytes read. buf should be at least MTU+something safe
// (e.g. 2048) to avoid truncation.
func (d *Device) ReadPacket(buf []byte) (int, error) {
	bufs := [][]byte{buf}
	sizes := make([]int, 1)
	n, err := d.nt.Read(bufs, sizes, 0)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return sizes[0], nil
}

// WritePacket writes one raw IP packet to the TUN device.
func (d *Device) WritePacket(pkt []byte) error {
	bufs := [][]byte{pkt}
	_, err := d.nt.Write(bufs, 0)
	return err
}

// Close tears down the TUN device.
func (d *Device) Close() error {
	close(d.closeCh)
	return d.nt.Close()
}

// LocalAddr is a convenience for callers that want a net.IP for the
// configured interface address.
func (d *Device) LocalAddr() net.IP {
	return net.IP(d.addr.Addr().AsSlice())
}
