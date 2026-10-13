// Package lanshare exposes the currently active tunnel as a local-network
// proxy: one TCP listener that speaks both SOCKS5 and HTTP CONNECT
// (auto-detected per connection), forwarding everything through a
// netstackvpn.Dialer. This lets other devices on the same LAN (phones,
// other computers) point their own proxy settings at this machine and ride
// its VPN connection, without installing anything themselves.
package lanshare

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Dialer is the minimal interface needed to forward a connection —
// satisfied by netstackvpn.Dialer / socks5dial's output / *ssh.Client.
type Dialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// Server is a running LAN share listener.
type Server struct {
	ln     net.Listener
	dialer Dialer
}

// Start listens on 0.0.0.0:port and begins serving. Returns the server and
// every "ip:port" the UI can show the user to type into another device on
// the LAN (a machine often has several LAN-facing addresses — Wi-Fi,
// Ethernet, a VM bridge — so all of them are returned rather than guessing
// which one the user's phone can actually reach).
func Start(port int, dialer Dialer) (*Server, []string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, nil, fmt.Errorf("lanshare: listen on port %d: %w", port, err)
	}
	s := &Server{ln: ln, dialer: dialer}
	go s.acceptLoop()

	actualPort := ln.Addr().(*net.TCPAddr).Port
	addrs := LANAddrs(actualPort)
	return s, addrs, nil
}

// LANAddrs returns "ip:port" for every non-loopback IPv4 address this
// machine has on a local network.
func LANAddrs(port int) []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d", ip.String(), port))
		}
	}
	return out
}

// Close stops the server.
func (s *Server) Close() error {
	return s.ln.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	first, err := br.Peek(1)
	if err != nil {
		return
	}

	if first[0] == 0x05 {
		s.serveSOCKS5(conn, br)
	} else {
		s.serveHTTP(conn, br)
	}
}

// --- SOCKS5 ---

func (s *Server) serveSOCKS5(conn net.Conn, br *bufio.Reader) {
	// Greeting: VER NMETHODS METHODS...
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return
	}
	nMethods := int(hdr[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	// No authentication required — LAN-local convenience proxy. The OS
	// network (Wi-Fi password, router) is the trust boundary here, same as
	// most home-LAN proxy setups.
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// Request: VER CMD RSV ATYP DST.ADDR DST.PORT
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return
	}
	if req[1] != 0x01 { // only CONNECT is supported
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	target, err := readSOCKS5Addr(br, req[3])
	if err != nil {
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	remote, err := s.dialer.Dial("tcp", target)
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()

	// Success reply. The bound address we report back is nominal (0.0.0.0:0)
	// since clients generally only care that the byte 0x00 (succeeded) is
	// there, not the exact value.
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	pipe(conn, remote)
}

func readSOCKS5Addr(br *bufio.Reader, atyp byte) (string, error) {
	var host string
	switch atyp {
	case 0x01: // IPv4
		b := make([]byte, 4)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", err
		}
		host = net.IP(b).String()
	case 0x03: // domain name
		l := make([]byte, 1)
		if _, err := io.ReadFull(br, l); err != nil {
			return "", err
		}
		b := make([]byte, l[0])
		if _, err := io.ReadFull(br, b); err != nil {
			return "", err
		}
		host = string(b)
	case 0x04: // IPv6
		b := make([]byte, 16)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", err
		}
		host = net.IP(b).String()
	default:
		return "", fmt.Errorf("lanshare: unsupported SOCKS5 address type %d", atyp)
	}

	portB := make([]byte, 2)
	if _, err := io.ReadFull(br, portB); err != nil {
		return "", err
	}
	port := int(portB[0])<<8 | int(portB[1])
	return net.JoinHostPort(host, fmt.Sprint(port)), nil
}

// --- HTTP CONNECT (and plain absolute-URI forward proxying) ---

func (s *Server) serveHTTP(conn net.Conn, br *bufio.Reader) {
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		remote, err := s.dialer.Dial("tcp", req.Host)
		if err != nil {
			conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer remote.Close()
		conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		pipe(conn, remote)
		return
	}

	// Plain (non-TLS) forward proxying: the request line carries an
	// absolute URI, e.g. "GET http://example.com/ HTTP/1.1".
	target := req.Host
	if !strings.Contains(target, ":") {
		target += ":80"
	}
	remote, err := s.dialer.Dial("tcp", target)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()

	req.RequestURI = ""
	if err := req.Write(remote); err != nil {
		return
	}
	io.Copy(conn, remote)
}

func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { io.Copy(a, b); done <- struct{}{} }()
	go func() { io.Copy(b, a); done <- struct{}{} }()
	<-done
	a.Close()
	b.Close()
	<-done
}
