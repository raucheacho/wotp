import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { useEffect } from 'react';
import { useStore } from './store';
import { useWebSocket } from './hooks/useWebSocket';
import type { WsEvent } from './types';
import LiveFeedScreen from './screens/LiveFeedScreen';
import LogsScreen from './screens/LogsScreen';
import QRScreen from './screens/QRScreen';
import SettingsScreen from './screens/SettingsScreen';

const ActivityIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
  </svg>
);

const TerminalIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <polyline points="4 17 10 11 4 5"></polyline>
    <line x1="12" y1="19" x2="20" y2="19"></line>
  </svg>
);

const SettingsIcon = () => (
  <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" strokeWidth="2" fill="none">
    <circle cx="12" cy="12" r="3"></circle>
    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
  </svg>
);

function Sidebar() {
  const connectionStatus = useStore(state => state.connectionStatus);
  
  return (
    <aside className="sidebar">
      <div className="brand" style={{ padding: '24px', display: 'flex', alignItems: 'center', gap: '12px' }}>
        <div style={{ background: 'var(--accent)', color: '#fff', width: '32px', height: '32px', borderRadius: '8px', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 'bold' }}>w</div>
        <div style={{ fontSize: '1.2rem', fontWeight: 700, letterSpacing: '-0.5px' }}>wotp</div>
      </div>
      
      <nav className="nav-menu" style={{ display: 'flex', flexDirection: 'column', gap: '8px', padding: '0 16px' }}>
        <NavLink to="/live" style={({isActive}) => ({ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 16px', borderRadius: '8px', color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)', background: isActive ? 'var(--bg-card-hover)' : 'transparent', textDecoration: 'none' })}>
          <ActivityIcon />
          <span>Live Feed</span>
        </NavLink>
        <NavLink to="/logs" style={({isActive}) => ({ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 16px', borderRadius: '8px', color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)', background: isActive ? 'var(--bg-card-hover)' : 'transparent', textDecoration: 'none' })}>
          <TerminalIcon />
          <span>Logs</span>
        </NavLink>
        <NavLink to="/settings" style={({isActive}) => ({ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 16px', borderRadius: '8px', color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)', background: isActive ? 'var(--bg-card-hover)' : 'transparent', textDecoration: 'none' })}>
          <SettingsIcon />
          <span>Settings</span>
        </NavLink>
      </nav>

      <div style={{ marginTop: 'auto', padding: '24px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px', background: 'var(--bg-tertiary)', borderRadius: '8px', fontSize: '0.85rem' }}>
          <div style={{ width: '8px', height: '8px', borderRadius: '50%', background: connectionStatus === 'connected' ? 'var(--accent)' : 'var(--text-tertiary)' }}></div>
          <span style={{ color: 'var(--text-secondary)' }}>{connectionStatus === 'connected' ? 'Connected' : 'Disconnected'}</span>
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
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  const { status: wsStatus } = useWebSocket({
    url: '/ws/events',
    onMessage: (data: unknown) => {
      const event = data as WsEvent;
      if (event && event.type) {
        addWsEvent(event);
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
    
    const fetchHistory = async () => {
      try {
        const res = await fetch('/dashboard/api/messages');
        if (res.ok) {
          const data = await res.json();
          setMessages(data);
        }
      } catch {
        // API not available yet
      }
    };
    
    checkHealth();
    fetchHistory();
    const interval = setInterval(checkHealth, 30000);
    return () => clearInterval(interval);
  }, [setConnectionStatus, setMessages]);

  if (connectionStatus !== 'connected' && connectionStatus !== 'connecting') {
    return <QRScreen />;
  }

  return (
    <div style={{ display: 'flex', height: '100vh', background: 'var(--bg-primary)' }}>
      <Sidebar />
      <main style={{ flex: 1, overflow: 'auto', padding: '32px' }}>
        <Routes>
          <Route path="/" element={<Navigate to="/live" replace />} />
          <Route path="/live" element={<LiveFeedScreen />} />
          <Route path="/logs" element={<LogsScreen />} />
          <Route path="/settings" element={<SettingsScreen />} />
          <Route path="*" element={<Navigate to="/live" replace />} />
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
