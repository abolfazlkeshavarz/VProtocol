import { AnimatePresence, motion } from "framer-motion";
import { Key, X } from "lucide-react";
import { useState } from "react";
import { main } from "../../wailsjs/go/models";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (input: main.ProfileInput) => Promise<void>;
}

export default function AddProfileModal({ open, onClose, onSubmit }: Props) {
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [user, setUser] = useState("");
  const [authMethod, setAuthMethod] = useState<"password" | "privateKey">("password");
  const [password, setPassword] = useState("");
  const [privateKeyPath, setPrivateKeyPath] = useState("");
  const [mode, setMode] = useState<"netstack" | "permitTunnel">("netstack");
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function reset() {
    setName("");
    setHost("");
    setPort("22");
    setUser("");
    setAuthMethod("password");
    setPassword("");
    setPrivateKeyPath("");
    setMode("netstack");
    setErr(null);
  }

  async function handleSubmit() {
    setErr(null);
    if (!host.trim() || !user.trim()) {
      setErr("Host and username are required.");
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit({
        name,
        host,
        port: parseInt(port, 10) || 22,
        user,
        authMethod,
        password,
        privateKeyPath,
        splitTunnel: [],
        mode,
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
          className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.16 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[420px] rounded-2xl bg-surface-card border border-surface-border shadow-2xl p-6"
          >
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-base font-semibold text-white/90">Add SSH Server</h2>
              <button onClick={onClose} className="text-white/40 hover:text-white">
                <X size={16} />
              </button>
            </div>

            <div className="flex flex-col gap-3">
              <Field label="Label (optional)" value={name} onChange={setName} placeholder="My VPS" />
              <div className="flex gap-3">
                <Field label="Host" value={host} onChange={setHost} placeholder="203.0.113.10" className="flex-[2]" />
                <Field label="Port" value={port} onChange={setPort} placeholder="22" className="flex-1" />
              </div>
              <Field label="Username" value={user} onChange={setUser} placeholder="root" />

              <div className="flex gap-1.5 bg-white/5 rounded-lg p-1 mt-1">
                <TabButton active={authMethod === "password"} onClick={() => setAuthMethod("password")}>
                  Password
                </TabButton>
                <TabButton active={authMethod === "privateKey"} onClick={() => setAuthMethod("privateKey")}>
                  <Key size={12} className="inline mr-1 -mt-0.5" />
                  Private Key
                </TabButton>
              </div>

              {authMethod === "password" ? (
                <Field label="Password" value={password} onChange={setPassword} type="password" placeholder="••••••••" />
              ) : (
                <Field
                  label="Private key path"
                  value={privateKeyPath}
                  onChange={setPrivateKeyPath}
                  placeholder="C:\Users\you\.ssh\id_ed25519"
                />
              )}

              <label className="flex flex-col gap-1.5 mt-1">
                <span className="text-[11px] font-medium text-white/40 uppercase tracking-wide">
                  Tunnel mode
                </span>
                <div className="flex gap-1.5 bg-white/5 rounded-lg p-1">
                  <TabButton active={mode === "netstack"} onClick={() => setMode("netstack")}>
                    Compatible
                  </TabButton>
                  <TabButton active={mode === "permitTunnel"} onClick={() => setMode("permitTunnel")}>
                    Fast (PermitTunnel)
                  </TabButton>
                </div>
                <p className="text-[11px] text-white/35 leading-snug">
                  {mode === "netstack"
                    ? "Works with any standard SSH server — no server config needed."
                    : "Faster, but requires \"PermitTunnel yes\" in the server's sshd_config."}
                </p>
              </label>

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
