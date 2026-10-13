// Package netstackvpn makes a real OS TUN device behave like a full VPN
// without needing any special SSH server configuration (no PermitTunnel,
// no root on the server side) — the same trick sshuttle/tun2socks use.
//
// It runs a userspace TCP/IP stack (gVisor's netstack) directly on top of
// the raw packets coming out of the OS TUN device. The stack's network
// interface is put in promiscuous + spoofing mode so it accepts packets for
// *any* destination, not just addresses it owns. Every new TCP connection
// the stack sees is then individually re-dialed out over the existing SSH
// connection as a plain "direct-tcpip" channel — the exact same mechanism
// `ssh -L`/`-D` port forwarding relies on, which is enabled on essentially
// every SSH server by default.
//
// UDP has no equivalent SSH primitive, so only UDP/53 (DNS) is special-cased
// by resolving over a TCP connection through the tunnel; other UDP traffic
// is dropped. This mirrors sshuttle's own, well-known limitation.
package netstackvpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const nicID = tcpip.NICID(1)

// Device is the minimal surface this package needs from a real OS TUN
// device (satisfied by *tunnel.Device, without importing that package and
// risking an import cycle).
type Device interface {
	ReadPacket(buf []byte) (int, error)
	WritePacket(pkt []byte) error
	MTU() int
}

// Dialer is anything that can open an outbound connection on behalf of the
// forwarder — an *ssh.Client (its Dial method opens a direct-tcpip
// channel), a SOCKS5 dialer (internal/socks5dial), or anything else with
// this shape.
type Dialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// Forwarder bridges a real TUN device to an upstream connection: every
// connection attempt seen on the TUN is re-established through dialer.
type Forwarder struct {
	dev    Device
	dialer Dialer

	ep    *channel.Endpoint
	stack *stack.Stack

	ctx    context.Context
	cancel context.CancelFunc

	// Logf, if set, receives diagnostic lines (connection opened/closed,
	// errors). Defaults to a no-op.
	Logf func(format string, args ...any)
}

// New creates the userspace stack and wires it to dev, ready to run. It
// does not start pumping packets yet; call Run for that.
func New(dev Device, dialer Dialer) (*Forwarder, error) {
	ctx, cancel := context.WithCancel(context.Background())

	f := &Forwarder{
		dev:    dev,
		dialer: dialer,
		ctx:    ctx,
		cancel: cancel,
		Logf:   func(string, ...any) {},
	}

	f.ep = channel.New(1024, uint32(dev.MTU()), "")

	f.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		HandleLocal:        false,
	})

	if err := f.stack.CreateNIC(nicID, f.ep); err != nil {
		cancel()
		return nil, fmt.Errorf("netstackvpn: create NIC: %s", err)
	}
	// Accept and originate packets for any address — we are a router, not
	// an endpoint with a fixed set of owned IPs.
	if err := f.stack.SetPromiscuousMode(nicID, true); err != nil {
		cancel()
		return nil, fmt.Errorf("netstackvpn: set promiscuous mode: %s", err)
	}
	if err := f.stack.SetSpoofing(nicID, true); err != nil {
		cancel()
		return nil, fmt.Errorf("netstackvpn: set spoofing: %s", err)
	}
	f.stack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicID},
		{Destination: header.IPv6EmptySubnet, NIC: nicID},
	})

	tcpFwd := tcp.NewForwarder(f.stack, 0, 1<<14, f.handleTCP)
	f.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	udpFwd := udp.NewForwarder(f.stack, f.handleUDP)
	f.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)

	return f, nil
}

// Run pumps packets between the real TUN device and the userspace stack
// until Close is called. It blocks; call it in its own goroutine.
func (f *Forwarder) Run() {
	go f.deviceToStack()
	f.stackToDevice() // blocks until ctx is cancelled
}

// Close tears down the stack and stops pumping packets.
func (f *Forwarder) Close() {
	f.cancel()
	f.stack.RemoveNIC(nicID)
	f.ep.Close()
}

func (f *Forwarder) deviceToStack() {
	buf := make([]byte, f.dev.MTU()+64)
	for {
		if f.ctx.Err() != nil {
			return
		}
		n, err := f.dev.ReadPacket(buf)
		if err != nil {
			f.Logf("netstackvpn: read from tun: %v", err)
			return
		}
		if n == 0 {
			continue
		}
		packet := buf[:n]
		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(append([]byte(nil), packet...)),
		})
		switch packet[0] >> 4 {
		case 4:
			f.ep.InjectInbound(header.IPv4ProtocolNumber, pkb)
		case 6:
			f.ep.InjectInbound(header.IPv6ProtocolNumber, pkb)
		}
	}
}

func (f *Forwarder) stackToDevice() {
	for {
		pkt := f.ep.ReadContext(f.ctx)
		if pkt == nil {
			return // closed or ctx cancelled
		}
		view := pkt.ToView()
		pkt.DecRef()

		data := view.AsSlice()
		if err := f.dev.WritePacket(data); err != nil {
			f.Logf("netstackvpn: write to tun: %v", err)
		}
		view.Release()
	}
}

func (f *Forwarder) handleTCP(r *tcp.ForwarderRequest) {
	id := r.ID()
	dst := net.JoinHostPort(id.LocalAddress.String(), strconv.Itoa(int(id.LocalPort)))

	var wq waiter.Queue
	ep, err := r.CreateEndpoint(&wq)
	if err != nil {
		r.Complete(true) // RST — refuse the connection
		return
	}
	r.Complete(false)

	local := gonet.NewTCPConn(&wq, ep)

	remote, dialErr := f.dialer.Dial("tcp", dst)
	if dialErr != nil {
		f.Logf("netstackvpn: dial %s over SSH: %v", dst, dialErr)
		local.Close()
		return
	}

	f.Logf("netstackvpn: TCP %s -> %s", id.RemoteAddress.String(), dst)
	pipe(local, remote)
}

// handleUDP only serves DNS (port 53): it resolves by opening a short-lived
// TCP connection to the destination over SSH and speaking DNS-over-TCP,
// since SSH has no UDP forwarding primitive. All other UDP is dropped.
func (f *Forwarder) handleUDP(r *udp.ForwarderRequest) bool {
	id := r.ID()
	if id.LocalPort != 53 {
		return false
	}

	var wq waiter.Queue
	ep, err := r.CreateEndpoint(&wq)
	if err != nil {
		return false
	}
	local := gonet.NewUDPConn(&wq, ep)
	dst := net.JoinHostPort(id.LocalAddress.String(), "53")

	go f.serveDNSOverTCP(local, dst)
	return true
}

func (f *Forwarder) serveDNSOverTCP(local *gonet.UDPConn, dst string) {
	defer local.Close()
	buf := make([]byte, 4096)
	for {
		n, err := local.Read(buf)
		if err != nil {
			return
		}
		query := append([]byte(nil), buf[:n]...)

		resp, err := f.resolveOverSSH(dst, query)
		if err != nil {
			f.Logf("netstackvpn: dns query to %s failed: %v", dst, err)
			continue
		}
		if _, err := local.Write(resp); err != nil {
			return
		}
	}
}

func (f *Forwarder) resolveOverSSH(dst string, query []byte) ([]byte, error) {
	conn, err := f.dialer.Dial("tcp", dst)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	lenPrefix := []byte{byte(len(query) >> 8), byte(len(query))}
	if _, err := conn.Write(append(lenPrefix, query...)); err != nil {
		return nil, err
	}

	var sizeBuf [2]byte
	if _, err := io.ReadFull(conn, sizeBuf[:]); err != nil {
		return nil, err
	}
	size := int(sizeBuf[0])<<8 | int(sizeBuf[1])
	resp := make([]byte, size)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(b, a)
		done <- struct{}{}
	}()
	<-done
	a.Close()
	b.Close()
	<-done
}
