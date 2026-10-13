//go:build windows

package router

import (
	"fmt"
	"os/exec"
)

// Apply routes the given CIDRs (or the whole internet, if cidrs is empty)
// through the named TUN interface using `netsh`, which accepts CIDR
// notation directly and is present on every Windows install.
func Apply(ifaceName string, cidrs []string) error {
	if len(cidrs) == 0 {
		cidrs = []string{"0.0.0.0/1", "128.0.0.0/1"}
	}
	for _, cidr := range cidrs {
		args := []string{"interface", "ipv4", "add", "route", cidr, ifaceName}
		if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
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
		args := []string{"interface", "ipv4", "delete", "route", cidr, ifaceName}
		if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("router: del route %s via %s: %w (%s)", cidr, ifaceName, err, string(out))
		}
	}
	return firstErr
}
