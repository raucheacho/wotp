/**
 * @wotp/client — HTTP client with auto-retry on transient errors.
 */

import type {
  WotpClientOptions,
  SendOTPResponse,
  VerifyOTPResponse,
  HealthResponse,
  APIErrorResponse,
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
    const data = await this.request<{ token: string; expires_at: string }>(
      'POST',
      '/otp/send',
      { phone },
    );
    return {
      token: data.token,
      expiresAt: data.expires_at,
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
    }>('POST', '/otp/verify', { token, code });

    return {
      verified: data.verified,
      phone: data.phone,
      attemptsRemaining: data.attempts_remaining,
    };
  }

  /**
   * Check the health of the Wotp instance.
   *
   * @returns Connection status, phone number, and uptime.
   */
  async health(): Promise<HealthResponse> {
    const data = await this.request<{
      status: string;
      phone: string;
      uptime_seconds: number;
    }>('GET', '/health');

    return {
      status: data.status,
      phone: data.phone,
      uptimeSeconds: data.uptime_seconds,
    };
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
