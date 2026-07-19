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
 * it has no notion of a single connected phone number, since one instance
 * can host many projects each with their own numbers. See `getChats()` or
 * the dashboard for per-project connection state.
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

export interface SendMediaRequest {
  phone: string;
  type: 'media';
  url?: string;
  base64?: string;
  caption?: string;
}

/**
 * Response from `POST /v1/messages/send`. There is no `success` field — a
 * failed send comes back as a non-2xx status and is thrown as a `WotpError`
 * instead.
 */
export interface MessageResponse {
  messageId?: string;
}

/** A WhatsApp contact visible to one of the project's connected numbers. */
export interface Chat {
  jid: string;
  name?: string;
}

/** State accepted by `setPresence()`. */
export type PresenceState = 'typing' | 'paused';
