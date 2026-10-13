// Package xraycore embeds Xray-core (MPL-2.0 — compatible with this
// project's MIT license, since MPL's copyleft is per-file and explicitly
// permits combining with differently-licensed code in a larger work) as an
// in-process library to provide VLESS, VMess, Trojan and Shadowsocks
// outbound support.
//
// Each Instance runs a minimal Xray config: one SOCKS inbound bound to
// 127.0.0.1 on an ephemeral port, and the one outbound the caller supplies.
// VProtocol's own netstack forwarder then dials through that local SOCKS
// port (via internal/socks5dial) exactly as it would dial through an SSH
// direct-tcpip channel — the protocol-specific complexity (AEAD framing,
// REALITY, transports, ...) stays entirely inside Xray.
package xraycore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"

	_ "github.com/xtls/xray-core/main/distro/all" // registers all protocols/transports
)

// Instance is a running embedded Xray-core with one outbound.
type Instance struct {
	core      *core.Instance
	socksAddr string
}

// Start builds a minimal Xray config around outbound (as produced by
// proxylink.Spec.XrayOutbound) and starts it. outbound must not be nil.
func Start(outbound map[string]any) (*Instance, error) {
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("xraycore: find free port: %w", err)
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     port,
				"protocol": "socks",
				"settings": map[string]any{
					"udp":  true,
					"auth": "noauth",
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []any{"http", "tls"},
				},
			},
		},
		"outbounds": []any{outbound},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("xraycore: marshal config: %w", err)
	}

	pbConfig, err := serial.LoadJSONConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("xraycore: parse config: %w", err)
	}

	inst, err := core.New(pbConfig)
	if err != nil {
		return nil, fmt.Errorf("xraycore: create instance: %w", err)
	}

	if err := inst.Start(); err != nil {
		return nil, fmt.Errorf("xraycore: start instance: %w", err)
	}

	return &Instance{
		core:      inst,
		socksAddr: fmt.Sprintf("127.0.0.1:%d", port),
	}, nil
}

// SocksAddr returns the local SOCKS5 proxy address ("127.0.0.1:PORT") that
// the running instance's outbound is reachable through.
func (i *Instance) SocksAddr() string { return i.socksAddr }

// Close stops the embedded instance.
func (i *Instance) Close() error { return i.core.Close() }

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
