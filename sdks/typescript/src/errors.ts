/**
 * wotp-client — Typed error hierarchy.
 *
 * Business errors are thrown as typed exceptions so callers can
 * catch specific failure modes without parsing HTTP status codes.
 */

/** Base class for all Wotp SDK errors. */
export class WotpError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'WotpError';
    Object.setPrototypeOf(this, new.target.prototype);
  }
}

/**
 * Thrown when the API returns a 429 — the phone number or IP
 * has exceeded the configured rate limit.
 */
export class RateLimitError extends WotpError {
  /** Seconds until the next request is allowed (if provided by API). */
  public readonly retryAfter?: number;

  constructor(message = 'Rate limit exceeded', retryAfter?: number) {
    super(message);
    this.name = 'RateLimitError';
    this.retryAfter = retryAfter;
  }
}

/**
 * Thrown when verification is attempted with an expired token.
 */
export class ExpiredTokenError extends WotpError {
  constructor(message = 'OTP token has expired') {
    super(message);
    this.name = 'ExpiredTokenError';
  }
}

/**
 * Thrown when verification fails because the code is incorrect.
 */
export class InvalidCodeError extends WotpError {
  /** Number of attempts remaining before the token is invalidated. */
  public readonly attemptsRemaining?: number;

  constructor(message = 'Invalid OTP code', attemptsRemaining?: number) {
    super(message);
    this.name = 'InvalidCodeError';
    this.attemptsRemaining = attemptsRemaining;
  }
}
