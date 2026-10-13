package proxylink

import "fmt"

// XrayOutbound builds the Xray-core JSON "outbound" object for this Spec.
// Returns nil for ProtoSOCKS5, since that protocol is handled directly
// (dialed as a plain SOCKS5 upstream) without needing Xray at all.
func (s *Spec) XrayOutbound() (map[string]any, error) {
	streamSettings := s.streamSettings()

	switch s.Protocol {
	case ProtoVLESS:
		user := map[string]any{
			"id":         s.UUID,
			"encryption": "none",
		}
		if s.Flow != "" {
			user["flow"] = s.Flow
		}
		return map[string]any{
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": s.Server,
						"port":    s.Port,
						"users":   []any{user},
					},
				},
			},
			"streamSettings": streamSettings,
		}, nil

	case ProtoVMess:
		return map[string]any{
			"protocol": "vmess",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": s.Server,
						"port":    s.Port,
						"users": []any{
							map[string]any{
								"id":       s.UUID,
								"alterId":  s.AlterID,
								"security": orDefault(s.Method, "auto"),
							},
						},
					},
				},
			},
			"streamSettings": streamSettings,
		}, nil

	case ProtoTrojan:
		return map[string]any{
			"protocol": "trojan",
			"settings": map[string]any{
				"servers": []any{
					map[string]any{
						"address":  s.Server,
						"port":     s.Port,
						"password": s.Password,
					},
				},
			},
			"streamSettings": streamSettings,
		}, nil

	case ProtoShadowsocks:
		return map[string]any{
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"servers": []any{
					map[string]any{
						"address":  s.Server,
						"port":     s.Port,
						"method":   s.Method,
						"password": s.Password,
					},
				},
			},
		}, nil

	case ProtoSOCKS5:
		return nil, nil

	default:
		return nil, fmt.Errorf("proxylink: no Xray outbound for protocol %q", s.Protocol)
	}
}

func (s *Spec) streamSettings() map[string]any {
	network := orDefault(s.Network, "tcp")
	ss := map[string]any{
		"network": network,
	}

	switch network {
	case "ws":
		ws := map[string]any{"path": orDefault(s.Path, "/")}
		if s.Host != "" {
			ws["headers"] = map[string]any{"Host": s.Host}
		}
		ss["wsSettings"] = ws
	case "grpc":
		ss["grpcSettings"] = map[string]any{
			"serviceName": s.Service,
		}
	case "httpupgrade":
		hu := map[string]any{"path": orDefault(s.Path, "/")}
		if s.Host != "" {
			hu["host"] = s.Host
		}
		ss["httpupgradeSettings"] = hu
	}

	switch s.Security {
	case "tls":
		tls := map[string]any{
			"serverName":    orDefault(s.SNI, s.Server),
			"allowInsecure": s.Insecure,
		}
		if len(s.ALPN) > 0 {
			tls["alpn"] = s.ALPN
		}
		if s.Fingerprint != "" {
			tls["fingerprint"] = s.Fingerprint
		}
		ss["security"] = "tls"
		ss["tlsSettings"] = tls
	case "reality":
		reality := map[string]any{
			"serverName":  orDefault(s.SNI, s.Server),
			"publicKey":   s.PublicKey,
			"shortId":     s.ShortID,
			"fingerprint": orDefault(s.Fingerprint, "chrome"),
		}
		if s.SpiderX != "" {
			reality["spiderX"] = s.SpiderX
		}
		ss["security"] = "reality"
		ss["realitySettings"] = reality
	}

	return ss
}
