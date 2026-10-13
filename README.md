# VProtocol

A cross-platform SSH VPN client for Windows and Linux (amd64 + arm64), with a modern desktop UI.

<img src="branding/logo.png" width="96" alt="VProtocol logo" />

## How it works

VProtocol tunnels your traffic over a plain SSH connection — no VPN protocol, no server-side setup beyond having SSH access.

- **Compatible mode (default):** works against *any* standard SSH server. A real network interface (TUN) on your machine feeds packets into a small userspace TCP/IP stack, which re-opens every connection over SSH as a plain `direct-tcpip` channel — the same mechanism `ssh -L`/`-D` port forwarding already uses, enabled by default on virtually every SSH server.
- **Fast mode (opt-in):** if your server has `PermitTunnel yes` set in `sshd_config`, VProtocol can instead forward the whole interface directly over SSH's `tun@openssh.com` channel, which is faster but needs that server-side config (and usually root there).

## Download

Prebuilt installers for Windows (`.exe` installer) and Linux (`.deb` / `.rpm`), for both amd64 and arm64, are published on the [Releases](../../releases) page.

- **Windows:** use `VProtocol-<arch>-installer.exe` — it bundles everything needed. If you instead want the portable (no-install) version, download `vprotocol-windows-<arch>-portable.zip` and run `vprotocol.exe` from inside the extracted folder — it needs `wintun.dll` to stay next to it, so don't move just the exe out on its own.
- **Linux:** install the `.deb` or `.rpm` matching your distro and arch. A raw `vprotocol-linux-<arch>` binary is also provided for other distros.

## Building from source

Requirements: Go 1.26+, Node 22+, and the [Wails](https://wails.io) CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```bash
cd app
wails build          # produces app/build/bin/vprotocol(.exe)
```

### Windows installer

Requires [NSIS](https://nsis.sourceforge.io/). From the repo root:

```bash
scripts/build-windows.sh   # builds and bundles wintun.dll next to the exe
```

Or, for a full NSIS installer:

```bash
cd app
wails build -nsis
```

### Linux packages

```bash
cd app
wails build
cd build/linux
VERSION=1.0.0 ARCH=amd64 nfpm pkg --config nfpm.yaml --packager deb --target ../bin/vprotocol.deb
```

## Project layout

```
internal/
  tunnel/       cross-platform TUN device (Wintun on Windows, /dev/net/tun on Linux)
  sshtunnel/    SSH connection handling, tun@openssh.com channel support
  netstackvpn/  userspace TCP/IP stack forwarding every connection over SSH
  router/       OS routing table management (full/split tunnel)
  config/       saved server profiles
app/            Wails desktop app (Go backend + React/TypeScript/Tailwind UI)
```

## License

MIT — see [LICENSE](LICENSE).
