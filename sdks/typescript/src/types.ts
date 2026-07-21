/**
 * wotp-client — TypeScript SDK for Wotp
 * Type definitions for all request/response shapes.
 */

// ─── Request Types ───────────────────────────────────────────────

/** Payload for sending an OTP. */
export interface SendOTPRequest {
  phone: string;
}

/** Payload for verifying an OTP. */
export interface VerifyOTPRequest {
  token: string;
  code: string;
}

// ─── Response Types ──────────────────────────────────────────────

/** Successful response from `POST /otp/send`. */
export interface SendOTPResponse {
  /** Opaque token to use for verification. */
  token: string;
  /** ISO 8601 timestamp when this OTP expires. */
  expiresAt: string;
  /**
   * Set to `"message_send_failed"` when the OTP was created but the
   * WhatsApp send itself failed (e.g. no number is connected yet). The
   * token is still valid — only delivery failed.
   */
  warning?: string;
}

/** Successful response from `POST /otp/verify`. */
export interface VerifyOTPResponse {
  /** Whether the OTP code was correct. */
  verified: boolean;
  /** The phone number that was verified (only present when verified is true). */
  phone?: string;
  /** Number of attempts remaining (only present when verified is false). */
  attemptsRemaining?: number;
}

/**
 * Response from `GET /v1/health`. This is an instance-wide liveness check —
 * see `getChats()` or the dashboard for the connected number's own status
 * (an instance is mono-tenant: exactly one WhatsApp number).
 */
export interface HealthResponse {
  /** `"ok"` when the instance is up. */
  status: string;
  /** Uptime in seconds. */
  uptimeSeconds: number;
}

// ─── Client Options (Modified) ──────────────────────────────────────────────

/** Configuration options for the Wotp client. */
export interface WotpClientOptions {
  /** Base URL of the Wotp instance (e.g. `http://localhost:54321`). */
  url: string;
  /** API key for authentication. */
  apiKey: string;
  /** Maximum number of retries for transient network errors. @default 3 */
  maxRetries?: number;
  /** Base delay in ms between retries (exponential backoff). @default 500 */
  retryDelay?: number;
  /** Request timeout in ms. @default 10000 */
  timeout?: number;
}

// ─── API Error Response ──────────────────────────────────────────

/** Raw error body returned by the Wotp API. */
export interface APIErrorResponse {
  verified?: boolean;
  error?: string;
  attempts_remaining?: number;
  message?: string;
}

// ─── Messages & Chats Types ────────────────────────────────────────

export interface SendTextRequest {
  phone: string;
  type: 'text';
  text: string;
}

/** Kind of attachment for `sendMedia()` — wotp supports the same four kinds
 * on both its whatsmeow and Cloud API backends. */
export type MediaKind = 'image' | 'video' | 'audio' | 'document';

/** Options for `sendMedia()`. Exactly one of `url`/`base64` should be set. */
export interface SendMediaOptions {
  /** Defaults to `'image'` (the API's legacy `"media"` alias). */
  kind?: MediaKind;
  url?: string;
  base64?: string;
  caption?: string;
  /** Shown as the file name in the recipient's chat. Only meaningful when `kind` is `'document'`. */
  filename?: string;
}

export interface SendMediaRequest {
  phone: string;
  type: MediaKind;
  url?: string;
  base64?: string;
  caption?: string;
  filename?: string;
}

/** Options for `sendLocation()` — both fields are optional. */
export interface SendLocationOptions {
  name?: string;
  address?: string;
}

export interface SendLocationRequest {
  phone: string;
  type: 'location';
  latitude: number;
  longitude: number;
  name?: string;
  address?: string;
}

/**
 * Response from `POST /v1/messages/send`. There is no `success` field — a
 * failed send comes back as a non-2xx status and is thrown as a `WotpError`
 * instead.
 */
export interface MessageResponse {
  messageId?: string;
}

/** A WhatsApp contact visible to the connected number. */
export interface Chat {
  jid: string;
  name?: string;
}

/** State accepted by `setPresence()`. */
export type PresenceState = 'typing' | 'paused';

// ─── Conversations & takeover ─────────────────────────────────────

/** State of a `Conversation` — `'bot'` by default, `'human'` after a takeover. */
export type ConversationState = 'bot' | 'human';

/**
 * A contact's WhatsApp conversation thread — one per phone number, created
 * automatically on first inbound contact.
 */
export interface Conversation {
  id: string;
  phone: string;
  state: ConversationState;
  createdAt: string;
  updatedAt: string;
}

/**
 * One entry in `getConversationMessages()`'s merged, chronological thread —
 * inbound replies, outbound sends, and OTP sends all show up here. `kind`
 * is `'otp'`/`'text'`/`'media'` for outbound entries, or an inbound media
 * message's kind (`'image'`/`'video'`/`'audio'`/`'document'`); absent for a
 * plain inbound text/location message.
 */
export interface ConversationMessage {
  direction: 'inbound' | 'outbound';
  kind?: string;
  content: string;
  pushName?: string;
  /** Set alongside `kind` for an inbound media message — see `getMedia()` to fetch the actual bytes. */
  mediaMimeType?: string;
  messageId?: string;
  status?: string;
  at: string;
}

/**
 * Optional payload for `takeoverConversation()`/`resumeConversation()` —
 * both fields are freeform, but recording them is what makes a takeover
 * auditable instead of a silent state flip.
 */
export interface ConversationStateChangeOptions {
  actor?: string;
  reason?: string;
}

// ─── Inbound media ─────────────────────────────────────────────────

/**
 * Raw bytes of a downloaded inbound media message — an image, video, voice
 * note, or document a contact sent in, ready to feed to OCR, Whisper, or
 * wherever else your bot needs it.
 */
export interface MediaFile {
  data: ArrayBuffer;
  contentType: string;
}
