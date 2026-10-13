import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Plus, Server, ShieldCheck, Trash2, X } from "lucide-react";
import {
  AddProfile,
  Connect,
  Disconnect,
  GetState,
  ListProfiles,
  RemoveProfile,
} from "../wailsjs/go/main/App";
import { config, main } from "../wailsjs/go/models";
import { EventsOn } from "../wailsjs/runtime/runtime";
import AddProfileModal from "./components/AddProfileModal";
import ProfileList from "./components/ProfileList";
import StatusOrb from "./components/StatusOrb";

export type Status = "disconnected" | "connecting" | "connected" | "error";

function App() {
  const [profiles, setProfiles] = useState<config.Profile[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [state, setState] = useState<main.ConnectionState>({
    status: "disconnected",
  } as main.ConnectionState);
  const [showAdd, setShowAdd] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    refreshProfiles();
    GetState().then(setState).catch(() => {});

    const off = EventsOn("connection:state", (s: main.ConnectionState) => {
      setState(s);
      if (s.status === "error") setError(s.message ?? "Unknown error");
      else setError(null);
    });
    return () => off();
  }, []);

  function refreshProfiles() {
    ListProfiles().then((list) => {
      setProfiles(list ?? []);
      if (!selectedId && list && list.length > 0) setSelectedId(list[0].id);
    });
  }

  async function handleConnect() {
    if (!selectedId) return;
    setError(null);
    try {
      await Connect(selectedId);
    } catch (e: any) {
      setError(String(e));
    }
  }

  async function handleDisconnect() {
    try {
      await Disconnect();
    } catch (e: any) {
      setError(String(e));
    }
  }

  async function handleAddProfile(input: main.ProfileInput) {
    const saved = await AddProfile(input);
    setShowAdd(false);
    refreshProfiles();
    setSelectedId(saved.id);
  }

  async function handleRemoveProfile(id: string) {
    await RemoveProfile(id);
    if (selectedId === id) setSelectedId(null);
    refreshProfiles();
  }

  const status = (state.status ?? "disconnected") as Status;
  const selected = profiles.find((p) => p.id === selectedId) ?? null;
  const isBusy = status === "connecting";
  const isConnected = status === "connected";

  return (
    <div className="h-full w-full bg-surface flex overflow-hidden">
      {/* Sidebar */}
      <aside className="w-72 shrink-0 border-r border-surface-border bg-surface-raised flex flex-col">
        <div className="flex items-center gap-2 px-5 py-5">
          <div className="h-8 w-8 rounded-lg bg-brand-500/20 flex items-center justify-center">
            <ShieldCheck className="h-4.5 w-4.5 text-brand-400" size={18} />
          </div>
          <span className="font-semibold text-[15px] tracking-tight text-white/90">
            VProtocol
          </span>
        </div>

        <div className="flex items-center justify-between px-5 pb-2">
          <span className="text-xs font-medium uppercase tracking-wider text-white/40">
            Servers
          </span>
          <button
            onClick={() => setShowAdd(true)}
            className="h-6 w-6 rounded-md flex items-center justify-center text-white/50 hover:text-white hover:bg-white/5 transition-colors"
            title="Add server"
          >
            <Plus size={15} />
          </button>
        </div>

        <ProfileList
          profiles={profiles}
          selectedId={selectedId}
          activeId={isConnected || isBusy ? state.profileId ?? null : null}
          onSelect={setSelectedId}
          onRemove={handleRemoveProfile}
          disabled={isBusy || isConnected}
        />

        {profiles.length === 0 && (
          <div className="flex-1 flex flex-col items-center justify-center px-6 text-center gap-3">
            <Server className="text-white/20" size={28} />
            <p className="text-sm text-white/40">
              No servers yet. Add one to get started.
            </p>
            <button
              onClick={() => setShowAdd(true)}
              className="text-sm text-brand-400 hover:text-brand-300 font-medium"
            >
              + Add your first server
            </button>
          </div>
        )}
      </aside>

      {/* Main panel */}
      <main className="flex-1 flex flex-col items-center justify-center relative px-8">
        <StatusOrb status={status} />

        <div className="mt-8 text-center">
          <h1 className="text-xl font-semibold text-white/90">
            {status === "connected" && "Connected"}
            {status === "connecting" && "Connecting…"}
            {status === "disconnected" && "Not Connected"}
            {status === "error" && "Connection Failed"}
          </h1>
          <p className="mt-1.5 text-sm text-white/40 max-w-sm">
            {selected
              ? selected.name || `${selected.user}@${selected.host}`
              : "Select a server to connect"}
          </p>
          {state.message && status !== "error" && (
            <p className="mt-1 text-xs text-white/30">{state.message}</p>
          )}
        </div>

        <AnimatePresence>
          {error && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="mt-4 max-w-sm rounded-lg bg-red-500/10 border border-red-500/20 px-4 py-2.5 text-sm text-red-300 flex items-start gap-2"
            >
              <span className="flex-1">{error}</span>
              <button onClick={() => setError(null)} className="text-red-300/60 hover:text-red-300">
                <X size={14} />
              </button>
            </motion.div>
          )}
        </AnimatePresence>

        <button
          disabled={!selected || isBusy}
          onClick={isConnected ? handleDisconnect : handleConnect}
          className={[
            "mt-10 px-10 py-3 rounded-full font-semibold text-sm tracking-wide transition-all duration-200",
            "disabled:opacity-30 disabled:cursor-not-allowed",
            isConnected
              ? "bg-white/5 text-white/80 border border-white/10 hover:bg-white/10"
              : "bg-brand-500 text-surface shadow-glow hover:bg-brand-400 active:scale-[0.98]",
          ].join(" ")}
        >
          {isBusy ? "Connecting…" : isConnected ? "Disconnect" : "Connect"}
        </button>

        <p className="absolute bottom-6 text-[11px] text-white/25">
          SSH tunnel · split or full-tunnel routing · all architectures
        </p>
      </main>

      <AddProfileModal
        open={showAdd}
        onClose={() => setShowAdd(false)}
        onSubmit={handleAddProfile}
      />
    </div>
  );
}

export default App;
