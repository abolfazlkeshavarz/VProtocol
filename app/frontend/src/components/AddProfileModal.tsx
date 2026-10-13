import { AnimatePresence, motion } from "framer-motion";
import { Check, ClipboardPaste, Key, X } from "lucide-react";
import { useState } from "react";
import { ParseLink } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { defaultPortFor, PROTOCOLS, ProtocolId } from "../protocols";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (input: main.ProfileInput) => Promise<void>;
}

const emptyState = {
  name: "",
  host: "",
  port: "",
  protocol: "ssh" as ProtocolId,
  // ssh
  user: "",
  authMethod: "password" as "password" | "privateKey",
  password: "",
  privateKeyPath: "",
  mode: "netstack" as "netstack" | "permitTunnel",
  // proxy protocols
  uuid: "",
  proxyPassword: "",
  proxyUsername: "",
  method: "",
  alterId: "0",
  flow: "none",
  network: "tcp" as string,
  wsPath: "/",
  wsHost: "",
  grpcService: "",
  security: "" as string,
  sni: "",
  alpn: "",
  fingerprint: "",
  insecure: false,
  publicKey: "",
  shortId: "",
  spiderX: "",
};

type FormState = typeof emptyState;

export default function AddProfileModal({ open, onClose, onSubmit }: Props) {
  const [f, setF] = useState<FormState>(emptyState);
  const [link, setLink] = useState("");
  const [linkStatus, setLinkStatus] = useState<"idle" | "ok" | "error">("idle");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setF((prev) => ({ ...prev, [key]: value }));
  }

  function reset() {
    setF(emptyState);
    setLink("");
    setLinkStatus("idle");
    setErr(null);
  }

  async function handleParseLink(text: string) {
    setLink(text);
    if (!text.trim() || !text.includes("://")) {
      setLinkStatus("idle");
      return;
    }
    try {
      const parsed = await ParseLink(text.trim());
      setF((prev) => ({
        ...prev,
        name: parsed.name || prev.name,
        host: parsed.host,
        port: String(parsed.port),
        protocol: parsed.protocol as ProtocolId,
        uuid: parsed.uuid || "",
        proxyPassword: parsed.proxyPassword || "",
        proxyUsername: parsed.proxyUsername || "",
        method: parsed.method || "",
        alterId: String(parsed.alterId ?? 0),
        flow: parsed.flow || "none",
        network: parsed.network || "tcp",
        wsPath: parsed.wsPath || "/",
        wsHost: parsed.wsHost || "",
        grpcService: parsed.grpcService || "",
        security: parsed.security || "",
        sni: parsed.sni || "",
        alpn: (parsed.alpn || []).join(","),
        fingerprint: parsed.fingerprint || "",
        insecure: !!parsed.insecure,
        publicKey: parsed.publicKey || "",
        shortId: parsed.shortId || "",
        spiderX: parsed.spiderX || "",
      }));
      setLinkStatus("ok");
    } catch {
      setLinkStatus("error");
    }
  }

  function selectProtocol(p: ProtocolId) {
    set("protocol", p);
    if (!f.port) set("port", String(defaultPortFor(p)));
  }

  async function handleSubmit() {
    setErr(null);
    if (!f.host.trim()) {
      setErr("Host is required.");
      return;
    }
    if (f.protocol === "ssh" && !f.user.trim()) {
      setErr("Username is required for SSH.");
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit({
        name: f.name,
        host: f.host,
        port: parseInt(f.port, 10) || defaultPortFor(f.protocol),
        protocol: f.protocol,
        user: f.user,
        authMethod: f.authMethod,
        password: f.password,
        privateKeyPath: f.privateKeyPath,
        mode: f.mode,
        uuid: f.uuid,
        proxyPassword: f.proxyPassword,
        proxyUsername: f.proxyUsername,
        method: f.method,
        alterId: parseInt(f.alterId, 10) || 0,
        flow: f.flow === "none" ? "" : f.flow,
        network: f.network,
        wsPath: f.wsPath,
        wsHost: f.wsHost,
        grpcService: f.grpcService,
        security: f.security,
        sni: f.sni,
        alpn: f.alpn ? f.alpn.split(",").map((s) => s.trim()).filter(Boolean) : [],
        fingerprint: f.fingerprint,
        insecure: f.insecure,
        publicKey: f.publicKey,
        shortId: f.shortId,
        spiderX: f.spiderX,
        splitTunnel: [],
      } as main.ProfileInput);
      reset();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-6"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.16 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[460px] max-h-[88vh] overflow-y-auto rounded-2xl bg-surface-card border border-surface-border shadow-2xl p-6"
          >
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-base font-semibold text-white/90">Add Server</h2>
              <button onClick={onClose} className="text-white/40 hover:text-white">
                <X size={16} />
              </button>
            </div>

            {/* Paste link quick-add */}
            <label className="flex flex-col gap-1.5 mb-5">
              <span className="text-[11px] font-medium text-white/40 uppercase tracking-wide">
                Paste a share link
              </span>
              <div className="relative">
                <ClipboardPaste
                  size={14}
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-white/25"
                />
                <input
                  value={link}
                  onChange={(e) => handleParseLink(e.target.value)}
                  placeholder="vless://, vmess://, trojan://, ss://, socks5://..."
                  className="w-full bg-white/[0.04] border border-white/10 rounded-lg pl-8 pr-8 py-2 text-sm text-white/90 placeholder:text-white/25 outline-none focus:border-brand-500/50 focus:bg-white/[0.06] transition-colors"
                />
                {linkStatus === "ok" && (
                  <Check size={14} className="absolute right-3 top-1/2 -translate-y-1/2 text-brand-400" />
                )}
                {linkStatus === "error" && (
                  <X size={14} className="absolute right-3 top-1/2 -translate-y-1/2 text-red-400" />
                )}
              </div>
            </label>

            {/* Protocol selector */}
            <div className="grid grid-cols-3 gap-1.5 bg-white/5 rounded-lg p-1 mb-4">
              {PROTOCOLS.map((p) => (
                <button
                  key={p.id}
                  onClick={() => selectProtocol(p.id)}
                  className={[
                    "text-xs font-medium py-1.5 rounded-md transition-colors",
                    f.protocol === p.id ? "bg-brand-500 text-surface" : "text-white/50 hover:text-white/80",
                  ].join(" ")}
                >
                  {p.label}
                </button>
              ))}
            </div>

            <div className="flex flex-col gap-3">
              <Field label="Label (optional)" value={f.name} onChange={(v) => set("name", v)} placeholder="My Server" />
              <div className="flex gap-3">
                <Field label="Host" value={f.host} onChange={(v) => set("host", v)} placeholder="203.0.113.10" className="flex-[2]" />
                <Field
                  label="Port"
                  value={f.port}
                  onChange={(v) => set("port", v)}
                  placeholder={String(defaultPortFor(f.protocol))}
                  className="flex-1"
                />
              </div>

              {f.protocol === "ssh" && (
                <SSHFields f={f} set={set} />
              )}
              {f.protocol === "vless" && <VLESSFields f={f} set={set} />}
              {f.protocol === "vmess" && <VMessFields f={f} set={set} />}
              {f.protocol === "trojan" && <TrojanFields f={f} set={set} />}
              {f.protocol === "shadowsocks" && <ShadowsocksFields f={f} set={set} />}
              {f.protocol === "socks5" && <SOCKS5Fields f={f} set={set} />}

              {err && <p className="text-xs text-red-400 mt-1">{err}</p>}

              <button
                onClick={handleSubmit}
                disabled={submitting}
                className="mt-3 w-full py-2.5 rounded-lg bg-brand-500 text-surface font-semibold text-sm hover:bg-brand-400 disabled:opacity-50 transition-colors"
              >
                {submitting ? "Saving…" : "Save Server"}
              </button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

// ----- Per-protocol field groups -----

function SSHFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <Field label="Username" value={f.user} onChange={(v) => set("user", v)} placeholder="root" />
      <TabRow>
        <TabButton active={f.authMethod === "password"} onClick={() => set("authMethod", "password")}>
          Password
        </TabButton>
        <TabButton active={f.authMethod === "privateKey"} onClick={() => set("authMethod", "privateKey")}>
          <Key size={12} className="inline mr-1 -mt-0.5" />
          Private Key
        </TabButton>
      </TabRow>
      {f.authMethod === "password" ? (
        <Field label="Password" value={f.password} onChange={(v) => set("password", v)} type="password" placeholder="••••••••" />
      ) : (
        <Field
          label="Private key path"
          value={f.privateKeyPath}
          onChange={(v) => set("privateKeyPath", v)}
          placeholder="C:\Users\you\.ssh\id_ed25519"
        />
      )}

      <label className="flex flex-col gap-1.5 mt-1">
        <span className="text-[11px] font-medium text-white/40 uppercase tracking-wide">Tunnel mode</span>
        <TabRow>
          <TabButton active={f.mode === "netstack"} onClick={() => set("mode", "netstack")}>
            Compatible
          </TabButton>
          <TabButton active={f.mode === "permitTunnel"} onClick={() => set("mode", "permitTunnel")}>
            Fast (PermitTunnel)
          </TabButton>
        </TabRow>
        <p className="text-[11px] text-white/35 leading-snug">
          {f.mode === "netstack"
            ? "Works with any standard SSH server — no server config needed."
            : 'Faster, but requires "PermitTunnel yes" in the server\'s sshd_config.'}
        </p>
      </label>
    </>
  );
}

function VLESSFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <Field label="UUID" value={f.uuid} onChange={(v) => set("uuid", v)} placeholder="e.g. 3f7a2b1c-..." />
      <div className="flex gap-3">
        <SelectField label="Flow" value={f.flow} onChange={(v) => set("flow", v)} options={["none", "xtls-rprx-vision"]} className="flex-1" />
        <SelectField
          label="Network"
          value={f.network}
          onChange={(v) => set("network", v)}
          options={["tcp", "ws", "grpc", "httpupgrade"]}
          className="flex-1"
        />
      </div>
      <TransportFields f={f} set={set} />
      <SecurityFields f={f} set={set} includeReality />
    </>
  );
}

function VMessFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <Field label="UUID" value={f.uuid} onChange={(v) => set("uuid", v)} placeholder="e.g. 3f7a2b1c-..." />
      <div className="flex gap-3">
        <SelectField
          label="Cipher"
          value={f.method || "auto"}
          onChange={(v) => set("method", v)}
          options={["auto", "aes-128-gcm", "chacha20-poly1305", "none"]}
          className="flex-[2]"
        />
        <Field label="Alter ID" value={f.alterId} onChange={(v) => set("alterId", v)} placeholder="0" className="flex-1" />
      </div>
      <SelectField
        label="Network"
        value={f.network}
        onChange={(v) => set("network", v)}
        options={["tcp", "ws", "grpc", "httpupgrade"]}
      />
      <TransportFields f={f} set={set} />
      <SecurityFields f={f} set={set} includeReality={false} />
    </>
  );
}

function TrojanFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <Field label="Password" value={f.proxyPassword} onChange={(v) => set("proxyPassword", v)} type="password" placeholder="••••••••" />
      <SelectField
        label="Network"
        value={f.network}
        onChange={(v) => set("network", v)}
        options={["tcp", "ws", "grpc", "httpupgrade"]}
      />
      <TransportFields f={f} set={set} />
      <SecurityFields f={f} set={set} includeReality defaultSecurity="tls" />
    </>
  );
}

const SS_METHODS = [
  "aes-256-gcm",
  "aes-128-gcm",
  "chacha20-ietf-poly1305",
  "xchacha20-ietf-poly1305",
  "2022-blake3-aes-256-gcm",
  "2022-blake3-chacha20-poly1305",
];

function ShadowsocksFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <SelectField label="Cipher" value={f.method || SS_METHODS[0]} onChange={(v) => set("method", v)} options={SS_METHODS} />
      <Field label="Password" value={f.proxyPassword} onChange={(v) => set("proxyPassword", v)} type="password" placeholder="••••••••" />
    </>
  );
}

function SOCKS5Fields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  return (
    <>
      <Field label="Username (optional)" value={f.proxyUsername} onChange={(v) => set("proxyUsername", v)} placeholder="" />
      <Field
        label="Password (optional)"
        value={f.proxyPassword}
        onChange={(v) => set("proxyPassword", v)}
        type="password"
        placeholder=""
      />
    </>
  );
}

function TransportFields({ f, set }: { f: FormState; set: <K extends keyof FormState>(k: K, v: FormState[K]) => void }) {
  if (f.network === "ws" || f.network === "httpupgrade") {
    return (
      <div className="flex gap-3">
        <Field label="Path" value={f.wsPath} onChange={(v) => set("wsPath", v)} placeholder="/" className="flex-1" />
        <Field label="Host header" value={f.wsHost} onChange={(v) => set("wsHost", v)} placeholder="" className="flex-1" />
      </div>
    );
  }
  if (f.network === "grpc") {
    return <Field label="gRPC service name" value={f.grpcService} onChange={(v) => set("grpcService", v)} placeholder="" />;
  }
  return null;
}

function SecurityFields({
  f,
  set,
  includeReality,
  defaultSecurity,
}: {
  f: FormState;
  set: <K extends keyof FormState>(k: K, v: FormState[K]) => void;
  includeReality: boolean;
  defaultSecurity?: string;
}) {
  const options = includeReality ? ["none", "tls", "reality"] : ["none", "tls"];
  const current = f.security || defaultSecurity || "none";

  return (
    <>
      <SelectField
        label="Security"
        value={current}
        onChange={(v) => set("security", v === "none" ? "" : v)}
        options={options}
      />
      {(current === "tls" || current === "reality") && (
        <>
          <div className="flex gap-3">
            <Field label="SNI" value={f.sni} onChange={(v) => set("sni", v)} placeholder="example.com" className="flex-1" />
            <Field label="Fingerprint" value={f.fingerprint} onChange={(v) => set("fingerprint", v)} placeholder="chrome" className="flex-1" />
          </div>
          {current === "tls" && (
            <label className="flex items-center gap-2 text-xs text-white/60 -mt-1">
              <input type="checkbox" checked={f.insecure} onChange={(e) => set("insecure", e.target.checked)} className="accent-brand-500" />
              Allow insecure (skip certificate verification)
            </label>
          )}
          {current === "reality" && (
            <>
              <Field label="Public key" value={f.publicKey} onChange={(v) => set("publicKey", v)} placeholder="" />
              <div className="flex gap-3">
                <Field label="Short ID" value={f.shortId} onChange={(v) => set("shortId", v)} placeholder="" className="flex-1" />
                <Field label="Spider X (optional)" value={f.spiderX} onChange={(v) => set("spiderX", v)} placeholder="" className="flex-1" />
              </div>
            </>
          )}
        </>
      )}
    </>
  );
}

// ----- Generic field primitives -----

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
  className = "",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
  className?: string;
}) {
  return (
    <label className={`flex flex-col gap-1.5 ${className}`}>
      <span className="text-[11px] font-medium text-white/40 uppercase tracking-wide">{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="bg-white/[0.04] border border-white/10 rounded-lg px-3 py-2 text-sm text-white/90 placeholder:text-white/25 outline-none focus:border-brand-500/50 focus:bg-white/[0.06] transition-colors"
      />
    </label>
  );
}

function SelectField({
  label,
  value,
  onChange,
  options,
  className = "",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: string[];
  className?: string;
}) {
  return (
    <label className={`flex flex-col gap-1.5 ${className}`}>
      <span className="text-[11px] font-medium text-white/40 uppercase tracking-wide">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="bg-white/[0.04] border border-white/10 rounded-lg px-3 py-2 text-sm text-white/90 outline-none focus:border-brand-500/50 focus:bg-white/[0.06] transition-colors"
      >
        {options.map((o) => (
          <option key={o} value={o} className="bg-surface-card">
            {o}
          </option>
        ))}
      </select>
    </label>
  );
}

function TabRow({ children }: { children: React.ReactNode }) {
  return <div className="flex gap-1.5 bg-white/5 rounded-lg p-1">{children}</div>;
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={[
        "flex-1 text-xs font-medium py-1.5 rounded-md transition-colors",
        active ? "bg-brand-500 text-surface" : "text-white/50 hover:text-white/80",
      ].join(" ")}
    >
      {children}
    </button>
  );
}
