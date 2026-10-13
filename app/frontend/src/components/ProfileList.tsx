import { Trash2, Wifi } from "lucide-react";
import { config } from "../../wailsjs/go/models";

interface Props {
  profiles: config.Profile[];
  selectedId: string | null;
  activeId: string | null;
  disabled: boolean;
  onSelect: (id: string) => void;
  onRemove: (id: string) => void;
}

export default function ProfileList({
  profiles,
  selectedId,
  activeId,
  disabled,
  onSelect,
  onRemove,
}: Props) {
  return (
    <div className="flex-1 overflow-y-auto px-2.5 py-1 flex flex-col gap-1">
      {profiles.map((p) => {
        const isSelected = p.id === selectedId;
        const isActive = p.id === activeId;
        return (
          <div
            key={p.id}
            onClick={() => !disabled && onSelect(p.id)}
            className={[
              "group relative flex items-center gap-2.5 rounded-lg px-3 py-2.5 cursor-pointer transition-colors",
              isSelected ? "bg-brand-500/10 border border-brand-500/30" : "border border-transparent hover:bg-white/[0.04]",
              disabled && !isSelected ? "opacity-40 cursor-not-allowed" : "",
            ].join(" ")}
          >
            <div
              className={[
                "h-7 w-7 rounded-md flex items-center justify-center shrink-0",
                isActive ? "bg-brand-500/20 text-brand-400" : "bg-white/5 text-white/40",
              ].join(" ")}
            >
              <Wifi size={14} />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-white/85 truncate">
                {p.name || p.host}
              </p>
              <p className="text-[11px] text-white/35 truncate">
                {p.user}@{p.host}:{p.port}
              </p>
            </div>
            {isActive && (
              <span className="h-1.5 w-1.5 rounded-full bg-brand-400 shrink-0" />
            )}
            <button
              onClick={(e) => {
                e.stopPropagation();
                onRemove(p.id);
              }}
              className="opacity-0 group-hover:opacity-100 text-white/30 hover:text-red-400 transition-opacity shrink-0"
            >
              <Trash2 size={13} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
