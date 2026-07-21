/**
 * wotp-client — HTTP client with auto-retry on transient errors.
 */

import type {
  WotpClientOptions,
  SendOTPResponse,
  VerifyOTPResponse,
  SendMediaOptions,
  SendMediaRequest,
  SendLocationOptions,
  SendLocationRequest,
  MessageResponse,
  Chat,
  HealthResponse,
  PresenceState,
  APIErrorResponse,
  Conversation,
  ConversationMessage,
  ConversationStateChangeOptions,
  MediaFile,
} from './types';
import {
  WotpError,
  RateLimitError,
  ExpiredTokenError,
  InvalidCodeError,
} from './errors';

const DEFAULT_MAX_RETRIES = 3;
const DEFAULT_RETRY_DELAY = 500;
const DEFAULT_TIMEOUT = 10_000;

// The API responds with snake_case JSON; Conversation/ConversationMessage
// are exposed camelCase like every other type in this SDK, so raw wire
// shapes are mapped explicitly here (same as sendOTP/verifyOTP/health do
// inline) rather than casting the response directly.
interface RawConversation {
  id: string;
  phone: string;
  state: 'bot' | 'human';
  created_at: string;
  updated_at: string;
}

interface RawConversationMessage {
  direction: 'inbound' | 'outbound';
  kind?: string;
  content: string;
  push_name?: string;
  media_mime_type?: string;
  message_id?: string;
  status?: string;
  at: string;
}

function toConversation(raw: RawConversation): Conversation {
  return {
    id: raw.id,
    phone: raw.phone,
    state: raw.state,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

function toConversationMessage(raw: RawConversationMessage): ConversationMessage {
  return {
    direction: raw.direction,
    kind: raw.kind,
    content: raw.content,
    pushName: raw.push_name,
    mediaMimeType: raw.media_mime_type,
    messageId: raw.message_id,
    status: raw.status,
    at: raw.at,
  };
}

/** Determines if an error is a transient network error worth retrying. */
function isTransientError(status: number): boolean {
  return status === 502 || status === 503 || status === 504 || status === 0;
}

/** Wait for a given number of milliseconds. */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Core Wotp client. Use `createClient()` to instantiate.
 */
export class WotpClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;
  private readonly maxRetries: number;
  private readonly retryDelay: number;
  private readonly timeout: number;

  constructor(options: WotpClientOptions) {
    this.baseUrl = options.url.replace(/\/+$/, '');
    this.apiKey = options.apiKey;
    this.maxRetries = options.maxRetries ?? DEFAULT_MAX_RETRIES;
    this.retryDelay = options.retryDelay ?? DEFAULT_RETRY_DELAY;
    this.timeout = options.timeout ?? DEFAULT_TIMEOUT;
  }

  // ─── Public API ──────────────────────────────────────────────

  /**
   * Send an OTP to the given phone number.
   *
   * @param phone — E.164 formatted phone number (e.g. `+212600000000`)
   * @returns Token and expiration timestamp.
   * @throws {RateLimitError} If the phone/IP has exceeded the rate limit.
   */
  async sendOTP(phone: string): Promise<SendOTPResponse> {
    const data = await this.request<{
      token: string;
      expires_at: string;
      warning?: string;
    }>('POST', '/v1/otp/send', { phone });
    return {
      token: data.token,
      expiresAt: data.expires_at,
      warning: data.warning,
    };
  }

  /**
   * Verify an OTP code against a previously issued token.
   *
   * @param token — The opaque token returned by `sendOTP`.
   * @param code  — The OTP code entered by the user.
   * @returns Verification result.
   * @throws {ExpiredTokenError} If the token has expired.
   * @throws {InvalidCodeError} If the code is incorrect.
   */
  async verifyOTP(token: string, code: string): Promise<VerifyOTPResponse> {
    const data = await this.request<{
      verified: boolean;
      phone?: string;
      attempts_remaining?: number;
    }>('POST', '/v1/otp/verify', { token, code });

    return {
      verified: data.verified,
      phone: data.phone,
      attemptsRemaining: data.attempts_remaining,
    };
  }

  /**
   * Check the liveness of the Wotp instance. This is instance-wide — see
   * `getChats()` or the dashboard for the connected number's own status.
   *
   * @returns Status and uptime.
   */
  async health(): Promise<HealthResponse> {
    const data = await this.request<{
      status: string;
      uptime_seconds: number;
    }>('GET', '/v1/health');

    return {
      status: data.status,
      uptimeSeconds: data.uptime_seconds,
    };
  }

  /**
   * Send a text message to the given phone number.
   */
  async sendText(phone: string, text: string): Promise<MessageResponse> {
    const data = await this.request<{ message_id?: string }>('POST', '/v1/messages/send', {
      phone,
      type: 'text',
      text,
    });
    return { messageId: data.message_id };
  }

  /**
   * Send a media message (image, video, audio, or document) to the given
   * phone number. `kind` defaults to `'image'` when omitted.
   */
  async sendMedia(phone: string, media: SendMediaOptions): Promise<MessageResponse> {
    const body: SendMediaRequest = {
      phone,
      type: media.kind ?? 'image',
      url: media.url,
      base64: media.base64,
      caption: media.caption,
      filename: media.filename,
    };
    const data = await this.request<{ message_id?: string }>('POST', '/v1/messages/send', body as unknown as Record<string, unknown>);
    return { messageId: data.message_id };
  }

  /**
   * Send a WhatsApp location message to the given phone number.
   */
  async sendLocation(
    phone: string,
    latitude: number,
    longitude: number,
    options?: SendLocationOptions,
  ): Promise<MessageResponse> {
    const body: SendLocationRequest = {
      phone,
      type: 'location',
      latitude,
      longitude,
      name: options?.name,
      address: options?.address,
    };
    const data = await this.request<{ message_id?: string }>('POST', '/v1/messages/send', body as unknown as Record<string, unknown>);
    return { messageId: data.message_id };
  }

  /**
   * List the WhatsApp contacts visible to the connected number.
   */
  async getChats(): Promise<Chat[]> {
    const data = await this.request<Chat[]>('GET', '/v1/chats');
    return data;
  }

  /**
   * Set the typing indicator for a chat without sending a message.
   */
  async setPresence(phone: string, state: PresenceState): Promise<void> {
    await this.request<{ ok: boolean }>('POST', '/v1/messages/presence', { phone, state });
  }

  // ─── Conversations & takeover ──────────────────────────────────

  /** List every tracked conversation (one per contact that has ever messaged in). */
  async listConversations(): Promise<Conversation[]> {
    const data = await this.request<RawConversation[]>('GET', '/v1/conversations');
    return data.map(toConversation);
  }

  /** Fetch a single conversation by id. */
  async getConversation(id: string): Promise<Conversation> {
    const data = await this.request<RawConversation>('GET', `/v1/conversations/${encodeURIComponent(id)}`);
    return toConversation(data);
  }

  /**
   * Get the full chronological thread for a conversation — inbound
   * replies, outbound sends, and OTP sends merged together.
   */
  async getConversationMessages(id: string): Promise<ConversationMessage[]> {
    const data = await this.request<RawConversationMessage[]>(
      'GET',
      `/v1/conversations/${encodeURIComponent(id)}/messages`,
    );
    return data.map(toConversationMessage);
  }

  /**
   * Mark a conversation as human-owned. wotp keeps forwarding
   * `message.received` either way — it's up to your own bot logic to read
   * `conversationState` from the webhook payload and stay quiet.
   */
  async takeoverConversation(id: string, options?: ConversationStateChangeOptions): Promise<void> {
    await this.setConversationState(id, 'takeover', options);
  }

  /** Hand a conversation back to the bot. */
  async resumeConversation(id: string, options?: ConversationStateChangeOptions): Promise<void> {
    await this.setConversationState(id, 'resume', options);
  }

  private async setConversationState(
    id: string,
    action: 'takeover' | 'resume',
    options?: ConversationStateChangeOptions,
  ): Promise<void> {
    await this.request<{ state: string }>(
      'POST',
      `/v1/conversations/${encodeURIComponent(id)}/${action}`,
      { actor: options?.actor, reason: options?.reason },
    );
  }

  // ─── Inbound media ──────────────────────────────────────────────

  /**
   * Download the raw bytes of an inbound media message wotp captured at
   * receive time (see `ConversationMessage.kind` / `MediaKind`). Throws a
   * `WotpError` with `statusCode` 404 if the message wasn't a media
   * message, or if the download itself failed when the message arrived.
   */
  async getMedia(messageId: string): Promise<MediaFile> {
    const url = `${this.baseUrl}/v1/media/${encodeURIComponent(messageId)}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: { apikey: this.apiKey },
    });

    if (response.ok) {
      return {
        data: await response.arrayBuffer(),
        contentType: response.headers.get('Content-Type') ?? '',
      };
    }

    const errorBody = (await response.json().catch(() => ({}))) as APIErrorResponse;
    throw new WotpError(
      errorBody.message ?? `Request failed with status ${response.status}`,
      response.status,
    );
  }

  // ─── Internal ────────────────────────────────────────────────

  /**
   * Make an HTTP request with auto-retry on transient errors.
   * Business errors (4xx) are never retried.
   */
  private async request<T>(
    method: 'GET' | 'POST',
    path: string,
    body?: Record<string, unknown>,
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    let lastError: Error | undefined;

    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeout);

        const response = await fetch(url, {
          method,
          headers: {
            'Content-Type': 'application/json',
            apikey: this.apiKey,
          },
          body: body ? JSON.stringify(body) : undefined,
          signal: controller.signal,
        });

        clearTimeout(timeoutId);

        // Successful response — parse and return
        if (response.ok) {
          return (await response.json()) as T;
        }

        // Business errors — throw typed exceptions, never retry
        const errorBody = (await response.json().catch(() => ({}))) as APIErrorResponse;

        if (response.status === 429) {
          const retryAfter = response.headers.get('Retry-After');
          throw new RateLimitError(
            errorBody.message ?? 'Rate limit exceeded',
            retryAfter ? parseInt(retryAfter, 10) : undefined,
          );
        }

        if (response.status === 400 || response.status === 410) {
          if (errorBody.error === 'token_expired' || response.status === 410) {
            throw new ExpiredTokenError(errorBody.message ?? 'OTP token has expired');
          }
          if (errorBody.error === 'invalid_code') {
            throw new InvalidCodeError(
              errorBody.message ?? 'Invalid OTP code',
              errorBody.attempts_remaining,
            );
          }
        }

        // Transient server errors — retry
        if (isTransientError(response.status)) {
          lastError = new WotpError(
            `Server error ${response.status}: ${response.statusText}`,
          );
          if (attempt < this.maxRetries) {
            await sleep(this.retryDelay * 2 ** attempt);
            continue;
          }
          throw lastError;
        }

        // Unknown error
        throw new WotpError(
          `Request failed with status ${response.status}: ${JSON.stringify(errorBody)}`,
          response.status,
        );
      } catch (error) {
        // If it's a typed business error, rethrow immediately
        if (
          error instanceof RateLimitError ||
          error instanceof ExpiredTokenError ||
          error instanceof InvalidCodeError
        ) {
          throw error;
        }

        // Network / timeout errors — retry
        if (error instanceof Error && error.name === 'AbortError') {
          lastError = new WotpError(`Request to ${path} timed out after ${this.timeout}ms`);
        } else if (error instanceof WotpError) {
          lastError = error;
        } else {
          lastError = new WotpError(
            `Network error: ${error instanceof Error ? error.message : String(error)}`,
          );
        }

        if (attempt < this.maxRetries) {
          await sleep(this.retryDelay * 2 ** attempt);
          continue;
        }
      }
    }

    throw lastError ?? new WotpError('Request failed after retries');
  }
}
