//go:build linux

// Package router applies OS routing/DNS so traffic actually flows through
// the TUN interface once it's up — full-tunnel (route everything) or
// split-tunnel (only the given CIDRs).
package router

import (
	"fmt"
	"os/exec"
)

// Apply routes the given CIDRs (or the whole internet, if cidrs is empty)
// through the named TUN interface.
func Apply(ifaceName string, cidrs []string) error {
	if len(cidrs) == 0 {
		cidrs = []string{"0.0.0.0/1", "128.0.0.0/1"} // full tunnel, avoids clobbering default route entry
	}
	for _, cidr := range cidrs {
		if out, err := exec.Command("ip", "route", "add", cidr, "dev", ifaceName).CombinedOutput(); err != nil {
			return fmt.Errorf("router: add route %s via %s: %w (%s)", cidr, ifaceName, err, string(out))
		}
	}
	return nil
}

// Clear removes previously applied routes for the given interface.
func Clear(ifaceName string, cidrs []string) error {
	if len(cidrs) == 0 {
		cidrs = []string{"0.0.0.0/1", "128.0.0.0/1"}
	}
	var firstErr error
	for _, cidr := range cidrs {
		if out, err := exec.Command("ip", "route", "del", cidr, "dev", ifaceName).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("router: del route %s via %s: %w (%s)", cidr, ifaceName, err, string(out))
		}
	}
	return firstErr
}
