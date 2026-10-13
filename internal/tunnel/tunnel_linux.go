//go:build linux

package tunnel

import (
	"fmt"
	"os/exec"
)

// configureInterface assigns the address and brings the link up using the
// `ip` command, which is present on essentially every modern Linux system.
// Requires CAP_NET_ADMIN (i.e. root, or the binary granted that capability).
func configureInterface(d *Device) error {
	addrStr := d.addr.String()

	if out, err := exec.Command("ip", "addr", "add", addrStr, "dev", d.name).CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr add: %w (%s)", err, string(out))
	}
	if out, err := exec.Command("ip", "link", "set", "dev", d.name, "mtu", fmt.Sprint(d.mtu), "up").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set up: %w (%s)", err, string(out))
	}
	return nil
}
