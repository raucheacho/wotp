import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  NavLink,
} from "react-router-dom";
import { useEffect, useCallback, useRef, useState } from "react";
import { ChevronsUpDown, Check } from "lucide-react";
import { useStore } from "./store";
import { useWebSocket } from "./hooks/useWebSocket";
import type { WsEvent, WaNumber, CloudStatus } from "./types";

import logoDark from "./assets/logo_dark.svg";
import logoLight from "./assets/logo_light.svg";

// Screens
import OverviewScreen from "./screens/OverviewScreen";
import NumbersScreen from "./screens/NumbersScreen";
import ActivityScreen from "./screens/ActivityScreen";
import WebhooksScreen from "./screens/WebhooksScreen";
import LogsScreen from "./screens/LogsScreen";

const OverviewIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="20"
    height="20"
    stroke="currentColor"
    strokeWidth="2"
    fill="none"
  >
    <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
    <line x1="3" y1="9" x2="21" y2="9"></line>
    <line x1="9" y1="21" x2="9" y2="9"></line>
  </svg>
);

const NumbersIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="20"
    height="20"
    stroke="currentColor"
    strokeWidth="2"
    fill="none"
  >
    <rect x="5" y="2" width="14" height="20" rx="2" ry="2"></rect>
    <line x1="12" y1="18" x2="12.01" y2="18"></line>
  </svg>
);

const ActivityIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="20"
    height="20"
    stroke="currentColor"
    strokeWidth="2"
    fill="none"
  >
    <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path>
  </svg>
);

const WebhookIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="20"
    height="20"
    stroke="currentColor"
    strokeWidth="2"
    fill="none"
  >
    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
  </svg>
);

function ProjectSwitcher() {
  const projects = useStore((state) => state.projects);
  const selectedProjectId = useStore((state) => state.selectedProjectId);
  const selectProject = useStore((state) => state.selectProject);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  if (projects.length === 0) return null;

  const selected = projects.find((p) => p.id === selectedProjectId);

  return (
    <div className="px-6 pb-4">
      <label className="text-xs text-muted-foreground mb-1 block">Project</label>
      <div ref={rootRef} className="relative">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-haspopup="listbox"
          aria-expanded={open}
          className="w-full flex items-center justify-between gap-2 text-sm bg-muted border rounded-md px-2.5 py-1.5 text-foreground hover:bg-muted/70 transition-colors"
        >
          <span className="truncate">{selected?.name ?? "Select a project"}</span>
          <ChevronsUpDown className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
        </button>

        {open && (
          <div
            role="listbox"
            className="absolute left-0 right-0 top-[calc(100%+4px)] z-50 max-h-64 overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-lg py-1"
          >
            {projects.map((p) => {
              const isSelected = p.id === selectedProjectId;
              return (
                <button
                  key={p.id}
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  onClick={() => {
                    selectProject(p.id);
                    setOpen(false);
                  }}
                  className={`w-full flex items-center justify-between gap-2 px-3 py-1.5 text-sm text-left transition-colors ${
                    isSelected ? "text-primary" : "text-foreground hover:bg-muted"
                  }`}
                >
                  <span className="truncate">{p.name}</span>
                  {isSelected && <Check className="w-3.5 h-3.5 shrink-0" />}
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

const LogsIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="20"
    height="20"
    stroke="currentColor"
    strokeWidth="2"
    fill="none"
  >
    <polyline points="4 17 10 11 4 5"></polyline>
    <line x1="12" y1="19" x2="20" y2="19"></line>
  </svg>
);

function NavSection({ label, children }: { label?: string; children: React.ReactNode }) {
  return (
    <div className="mt-4 first:mt-0">
      {label && (
        <div className="px-3 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
          {label}
        </div>
      )}
      <div className="flex flex-col gap-1">{children}</div>
    </div>
  );
}

function NavItem({ to, icon, label }: { to: string; icon: React.ReactNode; label: string }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? "bg-primary/10 text-primary" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`
      }
    >
      {icon}
      <span>{label}</span>
    </NavLink>
  );
}

function Sidebar() {
  const theme = useStore((state) => state.theme);

  return (
    <aside className="flex flex-col h-full bg-background border-r w-62.5 shrink-0">
      <div className="p-6 flex items-end gap-3 border-b">
        <img
          src={theme === "dark" ? logoDark : logoLight}
          alt="Wotp Logo"
          className="h-8"
        />
        <span className="text-xs text-muted">
          version : {import.meta.env.VITE_APP_VERSION || "dev"}
        </span>
      </div>

      <ProjectSwitcher />

      {/* Grouped by what an operator is checking, not one tab per API
          resource — Numbers/Webhooks are the infra you administer, Activity
          is what went out, Logs is raw diagnostics. */}
      <nav className="flex flex-col px-4">
        <NavSection>
          <NavItem to="/overview" icon={<OverviewIcon />} label="Overview" />
        </NavSection>

        <NavSection label="Infra">
          <NavItem to="/numbers" icon={<NumbersIcon />} label="Numbers" />
          <NavItem to="/webhooks" icon={<WebhookIcon />} label="Webhooks" />
        </NavSection>

        <NavSection label="Activity">
          <NavItem to="/activity" icon={<ActivityIcon />} label="Activity" />
        </NavSection>

        <NavSection label="Diagnostics">
          <NavItem to="/logs" icon={<LogsIcon />} label="Logs" />
        </NavSection>
      </nav>

      <div className="mt-auto p-6">
        <ConnectionIndicator />
      </div>
    </aside>
  );
}

// Reflects whichever backend is actually carrying this project's OTP
// traffic — whatsmeow (a paired, connected number) or Cloud API (enabled
// and verified) — rather than a single stale "engine" light that only ever
// knew about whatsmeow. See the Numbers screen for the per-backend detail.
function ConnectionIndicator() {
  const connectionStatus = useStore((state) => state.connectionStatus);
  const cloudEnabled = useStore((state) => state.cloudEnabled);
  const ok = connectionStatus === "connected";

  return (
    <div className="flex items-center gap-2 p-3 bg-muted rounded-lg text-sm">
      <div
        className={`w-2 h-2 rounded-full shrink-0 ${ok ? "bg-[#25D366] shadow-[0_0_8px_rgba(37,211,102,0.5)]" : "bg-muted-foreground"}`}
      ></div>
      <span className="text-muted-foreground font-medium">
        {ok ? (cloudEnabled ? "Operational (Cloud API)" : "Operational") : "Not connected"}
      </span>
    </div>
  );
}

function Layout() {
  const theme = useStore((state) => state.theme);
  const projects = useStore((state) => state.projects);
  const selectedProjectId = useStore((state) => state.selectedProjectId);
  const setProjects = useStore((state) => state.setProjects);
  const setConnectionStatus = useStore((state) => state.setConnectionStatus);
  const setCloudEnabled = useStore((state) => state.setCloudEnabled);
  const setMessages = useStore((state) => state.setMessages);
  const addWsEvent = useStore((state) => state.addWsEvent);
  const setWsStatus = useStore((state) => state.setWsStatus);

  // Initialize theme on mount
  useEffect(() => {
    if (theme === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }, [theme]);

  // Projects list is fetched once; the store picks/keeps a selection.
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const res = await fetch("/dashboard/api/projects");
        if (res.ok && mounted) setProjects(await res.json());
      } catch {}
    })();
    return () => {
      mounted = false;
    };
  }, [setProjects]);

  const fetchHistory = useCallback(async () => {
    if (!selectedProjectId) return;
    const qs = `?project_id=${encodeURIComponent(selectedProjectId)}`;
    try {
      const res = await fetch(`/dashboard/api/messages${qs}`);
      if (res.ok) {
        setMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch(`/dashboard/api/generic-messages${qs}`);
      if (res.ok) {
        useStore.getState().setGenericMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch(`/dashboard/api/webhooks${qs}`);
      if (res.ok) {
        useStore.getState().setWebhookEvents(await res.json());
      }
    } catch {}
  }, [selectedProjectId, setMessages]);

  const wsUrl = selectedProjectId
    ? `/v1/ws/events?project_id=${encodeURIComponent(selectedProjectId)}`
    : "/v1/ws/events";

  const { status: wsStatus } = useWebSocket({
    url: wsUrl,
    onMessage: (data: unknown) => {
      const event = data as WsEvent;
      if (event && event.type) {
        addWsEvent(event);
        if (
          event.type.startsWith("message.") ||
          event.type.startsWith("otp.")
        ) {
          fetchHistory();
        }
      }
    },
    reconnect: true,
  });

  useEffect(() => {
    setWsStatus(wsStatus);
  }, [wsStatus, setWsStatus]);

  // A project is "connected" if EITHER backend can actually send: its
  // whatsmeow number is connected, or its Cloud API backend is enabled and
  // verified. Checking only whatsmeow.Pool.Numbers() (as before) would trap
  // a Cloud-only project on the QR pairing screen forever, since Cloud API
  // has no QR to scan.
  useEffect(() => {
    if (!selectedProjectId) return;

    const checkConnection = async () => {
      let waConnected = false;
      let cloudOk = false;
      let cloudEnabled = false;
      try {
        const res = await fetch(
          `/dashboard/api/numbers?project_id=${encodeURIComponent(selectedProjectId)}`,
        );
        if (res.ok) {
          const numbers: WaNumber[] = await res.json();
          waConnected = numbers.some((n) => n.connected);
        }
      } catch {
        // API not available yet
      }
      try {
        const res = await fetch(
          `/dashboard/api/cloud-status?project_id=${encodeURIComponent(selectedProjectId)}`,
        );
        if (res.ok) {
          const status: CloudStatus = await res.json();
          cloudEnabled = status.enabled;
          cloudOk = status.enabled && status.connected;
        }
      } catch {
        // API not available yet
      }
      setCloudEnabled(cloudEnabled);
      setConnectionStatus(waConnected || cloudOk ? "connected" : "disconnected");
    };

    checkConnection();
    fetchHistory();
    const interval = setInterval(checkConnection, 30000);
    return () => clearInterval(interval);
  }, [selectedProjectId, setConnectionStatus, setCloudEnabled, fetchHistory]);

  if (projects.length === 0) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center text-muted-foreground">
        Loading projects…
      </div>
    );
  }

  // Navigation is always available, regardless of connection state — a
  // fresh or disconnected project no longer forces a full-page whatsmeow QR
  // takeover (see the removed QRScreen). That screen assumed whatsmeow was
  // the only way to get started; now Cloud API is an equally valid choice,
  // and railroading every new project into scanning a QR made it impossible
  // to reach the Numbers screen's own "configure Cloud API instead" path.
  // Overview's status cards + the alert banner communicate what's missing;
  // Numbers is where you actually act on it (pair a number or enable Cloud).
  return (
    <div className="flex h-screen w-screen bg-background overflow-hidden text-foreground">
      <Sidebar />
      <main className="flex-1 overflow-y-auto bg-background">
        <Routes>
          <Route path="/" element={<Navigate to="/overview" replace />} />
          <Route path="/overview" element={<OverviewScreen />} />
          <Route path="/numbers" element={<NumbersScreen />} />
          <Route path="/activity" element={<ActivityScreen />} />
          <Route path="/webhooks" element={<WebhooksScreen />} />
          <Route path="/logs" element={<LogsScreen />} />
          <Route path="*" element={<Navigate to="/overview" replace />} />
        </Routes>
      </main>
    </div>
  );
}

export default function App() {
  return (
    <BrowserRouter basename="/dashboard">
      <Layout />
    </BrowserRouter>
  );
}
