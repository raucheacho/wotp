import { create } from 'zustand';
import type { ConnectionStatus, DashboardStats, LogEntry, OtpMessage, Theme, WsEvent } from './types';

export interface AppState {
  connectionStatus: ConnectionStatus;
  connectedPhone: string | null;
  messages: OtpMessage[];
  logs: LogEntry[];
  stats: DashboardStats;
  theme: Theme;
  wsStatus: 'connecting' | 'connected' | 'disconnected';
  
  // Actions
  setConnectionStatus: (status: ConnectionStatus, phone?: string) => void;
  setWsStatus: (status: 'connecting' | 'connected' | 'disconnected') => void;
  setTheme: (theme: Theme) => void;
  setMessages: (messages: any[]) => void;
  addWsEvent: (event: WsEvent) => void;
  clearLogs: () => void;
}

function generateId(): string {
  return Math.random().toString(36).substring(2, 10) + Date.now().toString(36);
}

function maskPhone(phone: string): string {
  if (phone.length < 8) return phone;
  const cleaned = phone.replace(/\s/g, '');
  if (cleaned.length >= 12) {
    return `${cleaned.slice(0, 4)} ${cleaned.slice(4, 5)}XX XX ${cleaned.slice(-4, -2)} ${cleaned.slice(-2)}`;
  }
  return `${cleaned.slice(0, 3)}${'•'.repeat(cleaned.length - 5)}${cleaned.slice(-2)}`;
}

function isToday(dateStr: string): boolean {
  try {
    const date = new Date(dateStr);
    const today = new Date();
    return date.toDateString() === today.toDateString();
  } catch {
    return false;
  }
}

function formatEventLog(event: WsEvent): string {
  switch (event.type) {
    case 'message.sent':
      return `OTP sent to ${maskPhone(event.phone || '')} (${event.message_id || 'unknown'})`;
    case 'message.delivered':
      return `Message ${event.message_id} delivered`;
    case 'message.read':
      return `Message ${event.message_id} read`;
    case 'message.failed':
      return `Message failed for ${event.phone || 'unknown'}: ${event.error || 'unknown error'}`;
    case 'session.disconnected':
      return 'WhatsApp session disconnected';
    case 'session.reconnected':
      return 'WhatsApp session reconnected';
    default:
      return `Unknown event: ${event.type}`;
  }
}

export const useStore = create<AppState>((set) => ({
  connectionStatus: 'connecting',
  connectedPhone: null,
  messages: [],
  logs: [],
  stats: { messagesToday: 0, successRate: 100, avgResponseMs: 0 },
  theme: (localStorage.getItem('wotp-theme') as Theme) || 'dark',
  wsStatus: 'disconnected',

  setConnectionStatus: (status, phone) => set((state) => ({
    connectionStatus: status,
    connectedPhone: phone || state.connectedPhone,
  })),

  setWsStatus: (status) => set({ wsStatus: status }),

  setTheme: (theme) => {
    localStorage.setItem('wotp-theme', theme);
    document.documentElement.setAttribute('data-theme', theme);
    set({ theme });
  },

  setMessages: (apiMessages) => set((state) => {
    const messages = apiMessages.map((m: any) => ({
      id: m.message_id || m.id,
      phone: m.phone,
      code: '••••••',
      status: m.status,
      messageId: m.message_id,
      sentAt: m.created_at,
      deliveredAt: m.status === 'delivered' || m.status === 'read' ? m.created_at : undefined,
      readAt: m.status === 'read' ? m.created_at : undefined,
    }));
    const todayCount = messages.filter(m => isToday(m.sentAt)).length;
    const successCount = messages.filter(m => m.status !== 'failed' && isToday(m.sentAt)).length;
    return {
      messages,
      stats: {
        ...state.stats,
        messagesToday: todayCount,
        successRate: todayCount > 0 ? Math.round((successCount / todayCount) * 100) : 100,
      },
    };
  }),

  clearLogs: () => set({ logs: [] }),

  addWsEvent: (event) => set((state) => {
    const logEntry: LogEntry = {
      id: generateId(),
      timestamp: event.at || new Date().toISOString(),
      level: event.type === 'message.failed' || event.type === 'session.disconnected' ? 'error' :
             event.type === 'session.reconnected' ? 'info' : 'info',
      message: formatEventLog(event),
    };
    const newLogs = [...state.logs, logEntry].slice(-500);

    switch (event.type) {
      case 'message.sent': {
        const msg: OtpMessage = {
          id: event.message_id || generateId(),
          phone: event.phone || '',
          code: event.code || '••••••',
          status: 'sent',
          messageId: event.message_id,
          sentAt: event.at,
        };
        const newMessages = [msg, ...state.messages].slice(0, 200);
        const todayCount = newMessages.filter(m => isToday(m.sentAt)).length;
        const successCount = newMessages.filter(m => m.status !== 'failed' && isToday(m.sentAt)).length;
        return {
          messages: newMessages,
          logs: newLogs,
          stats: {
            ...state.stats,
            messagesToday: todayCount,
            successRate: todayCount > 0 ? Math.round((successCount / todayCount) * 100) : 100,
          },
        };
      }
      case 'message.delivered': {
        return {
          messages: state.messages.map(m =>
            m.messageId === event.message_id
              ? { ...m, status: 'delivered' as const, deliveredAt: event.at }
              : m
          ),
          logs: newLogs,
        };
      }
      case 'message.read': {
        return {
          messages: state.messages.map(m =>
            m.messageId === event.message_id
              ? { ...m, status: 'read' as const, readAt: event.at }
              : m
          ),
          logs: newLogs,
        };
      }
      case 'message.failed': {
        return {
          messages: state.messages.map(m =>
            (m.messageId === event.message_id) || (m.phone === event.phone && m.status === 'sent')
              ? { ...m, status: 'failed' as const, error: event.error }
              : m
          ),
          logs: newLogs,
        };
      }
      case 'session.disconnected': {
        return {
          connectionStatus: 'disconnected',
          connectedPhone: null,
          logs: newLogs,
        };
      }
      case 'session.reconnected': {
        return {
          connectionStatus: 'connected',
          logs: newLogs,
        };
      }
      default:
        return { logs: newLogs };
    }
  }),
}));
