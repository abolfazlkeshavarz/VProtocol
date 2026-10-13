package sshtunnel

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// pinnedHostKeyCallback returns an ssh.HostKeyCallback that accepts only a
// server key whose SHA256 fingerprint (base64, no padding — matching
// `ssh-keygen -lf` output format "SHA256:xxxx") equals want.
func pinnedHostKeyCallback(want string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		sum := sha256.Sum256(key.Marshal())
		got := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
		if got != want {
			return fmt.Errorf("sshtunnel: host key mismatch for %s: got %s, want %s", hostname, got, want)
		}
		return nil
	}
}

// Fingerprint computes the pinnable SHA256 fingerprint string for a given
// SSH public key, for use when first saving a server profile (TOFU).
func Fingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}
