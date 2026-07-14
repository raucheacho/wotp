// WebSocket event types from the backend
export interface WsEvent {
  type: 'message.sent' | 'message.delivered' | 'message.read' | 'message.failed' | 'session.disconnected' | 'session.reconnected' | 'generic.message.sent' | 'webhook.event' | 'otp.verified';
  phone?: string;
  to?: string;
  message_id?: string;
  code?: string;
  error?: string;
  at: string;
  content?: string;
  msgType?: 'text' | 'image' | 'document' | 'template';
  event_name?: string;
  url?: string;
  payload?: any;
  status?: string;
  status_code?: number;
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

export interface GenericMessage {
  id: string;
  to: string;
  type: 'text' | 'image' | 'document' | 'template';
  content: string;
  status: MessageStatus;
  sentAt: string;
}

export interface WebhookEvent {
  id: string;
  event: string;
  url: string;
  payload: any;
  status: 'success' | 'failed' | 'retrying';
  statusCode: number;
  timestamp: string;
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
  activeWebhooks: number;
  genericMessagesToday: number;
}

export type ConnectionStatus = 'connected' | 'disconnected' | 'connecting';

export type Theme = 'dark' | 'light';
