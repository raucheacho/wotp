/**
 * wotp-client — Official TypeScript SDK for Wotp
 *
 * WhatsApp OTP, self-hosted, one command.
 *
 * @example
 * ```ts
 * import { createClient } from 'wotp-client'
 *
 * const wotp = createClient('http://localhost:54321', 'wotp_anon_xxx')
 *
 * const { token, expiresAt } = await wotp.sendOTP('+212600000000')
 * const { verified } = await wotp.verifyOTP(token, '483920')
 * ```
 *
 * @packageDocumentation
 */

export { WotpClient } from './client';
export { WotpError, RateLimitError, ExpiredTokenError, InvalidCodeError } from './errors';
export type {
  SendOTPResponse,
  VerifyOTPResponse,
  HealthResponse,
  MessageResponse,
  Chat,
  PresenceState,
  WotpClientOptions,
} from './types';

import { WotpClient } from './client';
import type { WotpClientOptions } from './types';

/**
 * Create a new Wotp client instance.
 *
 * @param url    — Base URL of your Wotp instance (e.g. `http://localhost:54321`)
 * @param apiKey — Your anon or service API key
 * @param options — Optional additional configuration
 * @returns A configured `WotpClient` instance
 *
 * @example
 * ```ts
 * const wotp = createClient('http://localhost:54321', 'wotp_anon_xxx')
 * ```
 */
export function createClient(
  url: string,
  apiKey: string,
  options?: Partial<Omit<WotpClientOptions, 'url' | 'apiKey'>>,
): WotpClient {
  return new WotpClient({ url, apiKey, ...options });
}
