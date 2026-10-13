import { AnimatePresence, motion } from "framer-motion";
import { Check, Copy, Share2 } from "lucide-react";
import { useState } from "react";
import { main } from "../../wailsjs/go/models";

interface Props {
  state: main.LANShareState;
  onStart: (port: number) => Promise<void>;
  onStop: () => Promise<void>;
}

export default function LANSharePanel({ state, onStart, onStop }: Props) {
  const [port, setPort] = useState("18080");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);

  async function toggle() {
    setBusy(true);
    try {
      if (state.active) {
        await onStop();
      } else {
        await onStart(parseInt(port, 10) || 18080);
      }
    } finally {
      setBusy(false);
    }
  }

  function copy(addr: string) {
    navigator.clipboard?.writeText(addr).then(() => {
      setCopied(addr);
      setTimeout(() => setCopied(null), 1500);
    });
  }

  return (
    <div className="w-full max-w-sm rounded-xl border border-white/10 bg-white/[0.03] p-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Share2 size={14} className={state.active ? "text-brand-400" : "text-white/40"} />
          <span className="text-sm font-medium text-white/80">Share on LAN</span>
        </div>
        <button
          onClick={toggle}
          disabled={busy}
          className={[
            "relative h-5 w-9 rounded-full transition-colors disabled:opacity-50",
            state.active ? "bg-brand-500" : "bg-white/15",
          ].join(" ")}
        >
          <span
            className={[
              // left-0.5 is the explicit base (off) position — without it
              // the browser falls back to this absolutely-positioned
              // span's ambiguous "static position", which rendered the
              // knob on the right even when off.
              "absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white transition-transform",
              state.active ? "translate-x-4" : "translate-x-0",
            ].join(" ")}
          />
        </button>
      </div>

      {!state.active && (
        <div className="mt-3 flex items-center gap-2">
          <span className="text-[11px] text-white/40">Port</span>
          <input
            value={port}
            onChange={(e) => setPort(e.target.value)}
            className="w-20 bg-white/[0.04] border border-white/10 rounded-md px-2 py-1 text-xs text-white/80 outline-none focus:border-brand-500/50"
          />
        </div>
      )}

      <AnimatePresence>
        {state.active && state.addrs && state.addrs.length > 0 && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="mt-3 flex flex-col gap-1.5 overflow-hidden"
          >
            <p className="text-[11px] text-white/35">
              Set this as the SOCKS5 or HTTP proxy on other devices:
            </p>
            {state.addrs.map((addr) => (
              <button
                key={addr}
                onClick={() => copy(addr)}
                className="flex items-center justify-between rounded-md bg-white/[0.04] hover:bg-white/[0.07] px-2.5 py-1.5 text-xs font-mono text-white/80 transition-colors"
              >
                {addr}
                {copied === addr ? (
                  <Check size={12} className="text-brand-400" />
                ) : (
                  <Copy size={12} className="text-white/30" />
                )}
              </button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>

      {state.error && <p className="mt-2 text-[11px] text-red-400">{state.error}</p>}
    </div>
  );
}
