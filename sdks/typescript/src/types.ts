/**
 * @wotp/client — TypeScript SDK for Wotp
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

/** Response from `GET /health`. */
export interface HealthResponse {
  /** WhatsApp connection status. */
  status: string;
  /** Connected phone number. */
  phone: string;
  /** Uptime in seconds. */
  uptimeSeconds: number;
}

// ─── Client Options ──────────────────────────────────────────────

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
