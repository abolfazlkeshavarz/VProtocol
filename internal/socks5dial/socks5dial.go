// Package socks5dial builds a Dialer that connects through a SOCKS5 proxy,
// using the standard library's well-tested implementation rather than a
// hand-rolled client. Used both for profiles whose protocol IS plain
// SOCKS5, and internally to reach Xray-core's local SOCKS inbound once a
// VLESS/VMess/Trojan/Shadowsocks tunnel is up.
package socks5dial

import (
	"fmt"
	"net"

	"golang.org/x/net/proxy"
)

// Dialer is the minimal interface the rest of VProtocol forwards
// connections through — satisfied by this package's output, by *ssh.Client,
// and by anything else with the same shape.
type Dialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// New returns a Dialer that connects to proxyAddr ("host:port") and asks it
// to CONNECT to whatever address is passed to Dial. username/password may
// be empty for an unauthenticated SOCKS5 proxy.
func New(proxyAddr, username, password string) (Dialer, error) {
	var auth *proxy.Auth
	if username != "" || password != "" {
		auth = &proxy.Auth{User: username, Password: password}
	}
	d, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("socks5dial: build dialer for %s: %w", proxyAddr, err)
	}
	return d, nil
}
