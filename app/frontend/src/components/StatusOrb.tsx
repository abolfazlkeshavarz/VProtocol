import { motion } from "framer-motion";
import { Lock, Loader2, ShieldAlert, ShieldX } from "lucide-react";
import type { ReactElement } from "react";
import type { Status } from "../App";

const palette: Record<Status, { ring: string; bg: string; icon: ReactElement }> = {
  connected: {
    ring: "ring-brand-500/30",
    bg: "bg-brand-500",
    icon: <Lock className="text-surface" size={30} strokeWidth={2.4} />,
  },
  connecting: {
    ring: "ring-amber-400/20",
    bg: "bg-amber-400",
    icon: <Loader2 className="text-surface animate-spin" size={30} strokeWidth={2.4} />,
  },
  disconnected: {
    ring: "ring-white/5",
    bg: "bg-white/10",
    icon: <ShieldX className="text-white/50" size={30} strokeWidth={2.2} />,
  },
  error: {
    ring: "ring-red-500/20",
    bg: "bg-red-500/90",
    icon: <ShieldAlert className="text-surface" size={30} strokeWidth={2.4} />,
  },
};

export default function StatusOrb({ status }: { status: Status }) {
  const p = palette[status];

  return (
    <div className="relative h-32 w-32 flex items-center justify-center">
      {status === "connected" && (
        <>
          <span className="absolute inset-0 rounded-full bg-brand-500/20 animate-pulseRing" />
          <span className="absolute inset-0 rounded-full bg-brand-500/20 animate-pulseRing [animation-delay:1.1s]" />
        </>
      )}
      <motion.div
        layout
        className={`relative h-24 w-24 rounded-full ${p.bg} ring-8 ${p.ring} flex items-center justify-center shadow-lg`}
        animate={{ scale: status === "connecting" ? [1, 1.04, 1] : 1 }}
        transition={{ duration: 1.4, repeat: status === "connecting" ? Infinity : 0 }}
      >
        {p.icon}
      </motion.div>
    </div>
  );
}
