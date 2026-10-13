// Shared metadata for the protocols VProtocol supports, so the server list,
// add-server form, and badges all agree on labels/colors without repeating
// a switch statement in five places.

export type ProtocolId =
  | "ssh"
  | "vless"
  | "vmess"
  | "trojan"
  | "shadowsocks"
  | "socks5";

export interface ProtocolMeta {
  id: ProtocolId;
  label: string;
  short: string; // for compact badges
  colorClass: string; // text color for the badge
  bgClass: string; // background for the badge
}

export const PROTOCOLS: ProtocolMeta[] = [
  { id: "ssh", label: "SSH", short: "SSH", colorClass: "text-brand-300", bgClass: "bg-brand-500/15" },
  { id: "vless", label: "VLESS", short: "VLESS", colorClass: "text-violet-300", bgClass: "bg-violet-500/15" },
  { id: "vmess", label: "VMess", short: "VMess", colorClass: "text-fuchsia-300", bgClass: "bg-fuchsia-500/15" },
  { id: "trojan", label: "Trojan", short: "TROJ", colorClass: "text-red-300", bgClass: "bg-red-500/15" },
  { id: "shadowsocks", label: "Shadowsocks", short: "SS", colorClass: "text-amber-300", bgClass: "bg-amber-500/15" },
  { id: "socks5", label: "SOCKS5", short: "SOCKS", colorClass: "text-cyan-300", bgClass: "bg-cyan-500/15" },
];

export function protocolMeta(id: string): ProtocolMeta {
  return PROTOCOLS.find((p) => p.id === id) ?? PROTOCOLS[0];
}

export function defaultPortFor(id: ProtocolId): number {
  switch (id) {
    case "ssh":
      return 22;
    case "socks5":
      return 1080;
    default:
      return 443;
  }
}
