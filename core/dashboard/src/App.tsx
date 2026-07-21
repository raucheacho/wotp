import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  NavLink,
} from "react-router-dom";
import { useEffect, useCallback } from "react";
import { useStore } from "./store";
import { useWebSocket } from "./hooks/useWebSocket";
import type { WsEvent, WaNumber, CloudStatus } from "./types";

import logoDark from "./assets/logo_dark.svg";
import logoLight from "./assets/logo_light.svg";

// Screens
import OverviewScreen from "./screens/OverviewScreen";
import NumbersScreen from "./screens/NumbersScreen";
import ScanScreen from "./screens/ScanScreen";
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

      {/* Grouped by what an operator is checking, not one tab per API
          resource — Numbers/Webhooks are the infra you administer, Activity
          is what went out, Logs is raw diagnostics. */}
      <nav className="flex flex-col px-4 pt-4">
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

// Reflects whichever backend is actually carrying this instance's OTP
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

  const fetchHistory = useCallback(async () => {
    try {
      const res = await fetch("/dashboard/api/messages");
      if (res.ok) {
        setMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch("/dashboard/api/generic-messages");
      if (res.ok) {
        useStore.getState().setGenericMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch("/dashboard/api/webhooks");
      if (res.ok) {
        useStore.getState().setWebhookEvents(await res.json());
      }
    } catch {}
  }, [setMessages]);

  const { status: wsStatus } = useWebSocket({
    url: "/v1/ws/events",
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

  // The instance is "connected" if EITHER backend can actually send: the
  // whatsmeow number is connected, or the Cloud API backend is enabled and
  // verified. Checking only whatsmeow.Pool.Numbers() (as before) would trap
  // a Cloud-only instance on the QR pairing screen forever, since Cloud API
  // has no QR to scan.
  useEffect(() => {
    const checkConnection = async () => {
      let waConnected = false;
      let cloudOk = false;
      let cloudEnabled = false;
      try {
        const res = await fetch("/dashboard/api/numbers");
        if (res.ok) {
          const numbers: WaNumber[] = await res.json();
          waConnected = numbers.some((n) => n.connected);
        }
      } catch {
        // API not available yet
      }
      try {
        const res = await fetch("/dashboard/api/cloud-status");
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
  }, [setConnectionStatus, setCloudEnabled, fetchHistory]);

  // Navigation is always available, regardless of connection state — a
  // fresh or disconnected instance no longer forces a full-page whatsmeow QR
  // takeover (see the removed QRScreen). That screen assumed whatsmeow was
  // the only way to get started; now Cloud API is an equally valid choice,
  // and railroading first boot into scanning a QR made it impossible to
  // reach the Numbers screen's own "configure Cloud API instead" path.
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
          <Route path="/numbers/scan" element={<ScanScreen />} />
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
