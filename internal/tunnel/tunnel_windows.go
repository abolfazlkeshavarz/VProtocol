//go:build windows

package tunnel

import (
	"fmt"
	"net/netip"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// configureInterface assigns the address on the Wintun adapter via the
// Windows IP Helper API (winipcfg), which the wireguard-go/wireguard-windows
// projects use for the same purpose. Requires Administrator privileges
// (Wintun adapter creation already requires this).
func configureInterface(d *Device) error {
	nativeTun, ok := d.nt.(*tun.NativeTun)
	if !ok {
		return fmt.Errorf("unexpected tun.Device implementation on windows")
	}
	luid := winipcfg.LUID(nativeTun.LUID())

	if err := luid.SetIPAddressesForFamily(windows.AF_INET, []netip.Prefix{d.addr}); err != nil {
		return fmt.Errorf("set interface address: %w", err)
	}
	return nil
}
