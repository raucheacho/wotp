import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { useEffect, useCallback } from 'react';
import { useStore } from './store';
import { useWebSocket } from './hooks/useWebSocket';
import type { WsEvent } from './types';

// Screens
import OverviewScreen from './screens/OverviewScreen';
import OtpEngineScreen from './screens/OtpEngineScreen';
import MessagingApiScreen from './screens/MessagingApiScreen';
import WebhooksScreen from './screens/WebhooksScreen';
import LogsScreen from './screens/LogsScreen';
import QRScreen from './screens/QRScreen';


const OverviewIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
    <line x1="3" y1="9" x2="21" y2="9"></line>
    <line x1="9" y1="21" x2="9" y2="9"></line>
  </svg>
);

const OtpIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
  </svg>
);

const MessageIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path>
  </svg>
);

const WebhookIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
  </svg>
);

function Sidebar() {
  const connectionStatus = useStore(state => state.connectionStatus);
  
  return (
    <aside className="flex flex-col h-full bg-secondary border-r w-[250px] shrink-0">
      <div className="p-6 flex items-center gap-3 border-b mb-4">
        <div className="bg-primary text-primary-foreground w-8 h-8 rounded-lg flex items-center justify-center font-bold">w</div>
        <div className="text-xl font-bold tracking-tight">wotp</div>
      </div>
      
      <nav className="flex flex-col gap-1 px-4">
        <NavLink to="/overview" className={({isActive}) => `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}>
          <OverviewIcon />
          <span>Overview</span>
        </NavLink>
        <NavLink to="/otp" className={({isActive}) => `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}>
          <OtpIcon />
          <span>OTP Engine</span>
        </NavLink>
        <NavLink to="/messages" className={({isActive}) => `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}>
          <MessageIcon />
          <span>Messaging API</span>
        </NavLink>
        <NavLink to="/webhooks" className={({isActive}) => `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}>
          <WebhookIcon />
          <span>Webhooks</span>
        </NavLink>
        <div className="my-4 border-t"></div>
        <NavLink to="/logs" className={({isActive}) => `flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors ${isActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}>
          <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
          <span>Logs</span>
        </NavLink>
      </nav>

      <div className="mt-auto p-6">
        <div className="flex items-center gap-2 p-3 bg-muted rounded-lg text-sm">
          <div className={`w-2 h-2 rounded-full ${connectionStatus === 'connected' ? 'bg-[#25D366] shadow-[0_0_8px_rgba(37,211,102,0.5)]' : 'bg-muted-foreground'}`}></div>
          <span className="text-muted-foreground font-medium">{connectionStatus === 'connected' ? 'Engine Online' : 'Engine Offline'}</span>
        </div>
      </div>
    </aside>
  );
}

function Layout() {
  const connectionStatus = useStore(state => state.connectionStatus);
  const theme = useStore(state => state.theme);
  const setConnectionStatus = useStore(state => state.setConnectionStatus);
  const setMessages = useStore(state => state.setMessages);
  const addWsEvent = useStore(state => state.addWsEvent);
  const setWsStatus = useStore(state => state.setWsStatus);

  // Initialize theme on mount
  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }, [theme]);

  const fetchHistory = useCallback(async () => {
    try {
      const res = await fetch('/dashboard/api/messages');
      if (res.ok) {
        setMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch('/dashboard/api/generic-messages');
      if (res.ok) {
        useStore.getState().setGenericMessages(await res.json());
      }
    } catch {}
    try {
      const res = await fetch('/dashboard/api/webhooks');
      if (res.ok) {
        useStore.getState().setWebhookEvents(await res.json());
      }
    } catch {}
  }, [setMessages]);

  const { status: wsStatus } = useWebSocket({
    url: '/ws/events',
    onMessage: (data: unknown) => {
      const event = data as WsEvent;
      if (event && event.type) {
        addWsEvent(event);
        if (event.type.startsWith('message.') || event.type.startsWith('otp.')) {
          fetchHistory();
        }
      }
    },
    reconnect: true,
  });

  useEffect(() => {
    setWsStatus(wsStatus);
  }, [wsStatus, setWsStatus]);

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const res = await fetch('/health');
        if (res.ok) {
          const data = await res.json();
          setConnectionStatus(data.status || 'disconnected', data.phone);
        }
      } catch {
        // API not available yet
      }
    };
    
    checkHealth();
    fetchHistory();
    const interval = setInterval(checkHealth, 30000);
    return () => clearInterval(interval);
  }, [setConnectionStatus, fetchHistory]);

  if (connectionStatus !== 'connected' && connectionStatus !== 'connecting') {
    return <QRScreen />;
  }

  return (
    <div className="flex h-screen w-screen bg-background overflow-hidden text-foreground">
      <Sidebar />
      <main className="flex-1 overflow-y-auto bg-background">
        <Routes>
          <Route path="/" element={<Navigate to="/overview" replace />} />
          <Route path="/overview" element={<OverviewScreen />} />
          <Route path="/otp" element={<OtpEngineScreen />} />
          <Route path="/messages" element={<MessagingApiScreen />} />
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
