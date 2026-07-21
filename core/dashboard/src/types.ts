// WebSocket event types from the backend
export interface WsEvent {
  type: 'message.sent' | 'message.delivered' | 'message.read' | 'message.failed' | 'session.disconnected' | 'session.reconnected' | 'generic.message.sent' | 'webhook.event' | 'otp.verified' | 'number.qr';
  /** JID of the number that handled this event. */
  from?: string;
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
  /** Only present on live WS updates (ws.Event.URL) — REST history rows
   * (store.WebhookLog) don't record which endpoint was called. */
  url?: string;
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
  status: 'ok';
  uptime_seconds?: number;
}

// The instance's whatsmeow number — see whatsapp.Pool.Numbers(). At most one.
export interface WaNumber {
  jid: string;
  phone: string;
  connected: boolean;
}

// The instance's Meta Cloud API backend status — see api.CloudStatus. Never
// includes the access token, app secret, or pin (write-only, see the
// settings form) — waba_id/verify_token aren't secret, so they round-trip.
export interface CloudStatus {
  enabled: boolean;
  connected: boolean;
  phone_number_id?: string;
  display_phone?: string;
  otp_template_name?: string;
  otp_template_language?: string;
  waba_id?: string;
  verify_token?: string;
  webhook_url?: string;
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
