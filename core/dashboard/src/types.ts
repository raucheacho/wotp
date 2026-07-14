// WebSocket event types from the backend
export interface WsEvent {
  type: 'message.sent' | 'message.delivered' | 'message.read' | 'message.failed' | 'session.disconnected' | 'session.reconnected';
  phone?: string;
  message_id?: string;
  code?: string;
  error?: string;
  at: string;
}

export type MessageStatus = 'pending' | 'sent' | 'delivered' | 'read' | 'failed' | 'verified' | 'expired';

export interface OtpMessage {
  id: string;
  phone: string;
  code: string;
  status: MessageStatus;
  messageId?: string;
  error?: string;
  sentAt: string;
  deliveredAt?: string;
  readAt?: string;
}

export interface LogEntry {
  id: string;
  timestamp: string;
  level: 'info' | 'warn' | 'error';
  message: string;
}

export interface HealthResponse {
  status: 'connected' | 'disconnected' | 'connecting';
  phone?: string;
  uptime_seconds?: number;
}

export interface DashboardStats {
  messagesToday: number;
  successRate: number;
  avgResponseMs: number;
}

export type ConnectionStatus = 'connected' | 'disconnected' | 'connecting';

export type Theme = 'dark' | 'light';
