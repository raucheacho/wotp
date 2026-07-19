# wotp-client

Official TypeScript SDK for **Wotp** — WhatsApp OTP, self-hosted, one command.

[![npm](https://img.shields.io/npm/v/wotp-client)](https://www.npmjs.com/package/wotp-client)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Installation

```bash
npm install wotp-client
```

## Quick Start

```ts
import { createClient } from 'wotp-client'

const wotp = createClient('http://localhost:54321', 'wotp_anon_xxx')

// Send an OTP
const { token, expiresAt } = await wotp.sendOTP('+212600000000')

// Verify the code entered by the user
const { verified } = await wotp.verifyOTP(token, '483920')

// Send a text message
const { messageId } = await wotp.sendText('+212600000000', 'Hello world')

// Send a media message
await wotp.sendMedia('+212600000000', { url: 'https://example.com/image.png' })

// Show a typing indicator
await wotp.setPresence('+212600000000', 'typing')

// List chats
const chats = await wotp.getChats()
```

## API Reference

### `createClient(url, apiKey, options?)`

Creates a new Wotp client instance.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | `string` | Base URL of your Wotp instance |
| `apiKey` | `string` | Your anon or service API key |
| `options.maxRetries` | `number` | Max retries on transient errors (default: `3`) |
| `options.retryDelay` | `number` | Base delay in ms between retries (default: `500`) |
| `options.timeout` | `number` | Request timeout in ms (default: `10000`) |

### `wotp.sendOTP(phone)`

Send an OTP to the given phone number.

- **Parameters:** `phone` — E.164 formatted phone number (e.g. `+212600000000`)
- **Returns:** `Promise<{ token: string; expiresAt: string }>`
- **Throws:** `RateLimitError` if the rate limit is exceeded

### `wotp.verifyOTP(token, code)`

Verify an OTP code against a previously issued token.

- **Parameters:** `token` — from `sendOTP`, `code` — the user-entered code
- **Returns:** `Promise<{ verified: boolean; phone?: string; attemptsRemaining?: number }>`
- **Throws:** `ExpiredTokenError`, `InvalidCodeError`

### `wotp.health()`

Instance-wide liveness check (no notion of a single connected phone number — an instance can host many projects, each with their own numbers).

- **Returns:** `Promise<{ status: string; uptimeSeconds: number }>`

### `wotp.sendText(phone, text)`

Send a text message to the given phone number.

- **Returns:** `Promise<{ messageId?: string }>`

### `wotp.sendMedia(phone, media)`

Send a media message. `media` is `{ url?: string; base64?: string; caption?: string }` — provide either `url` or `base64`.

- **Returns:** `Promise<{ messageId?: string }>`

### `wotp.getChats()`

List the WhatsApp contacts visible to the project's connected numbers.

- **Returns:** `Promise<Chat[]>` — each `Chat` is `{ jid: string; name?: string }`

### `wotp.setPresence(phone, state)`

Set the typing indicator for a chat without sending a message. `state` is `'typing' | 'paused'`.

- **Returns:** `Promise<void>`

## Error Handling

The SDK throws typed errors for business failures — no need to parse HTTP status codes:

```ts
import { createClient, RateLimitError, ExpiredTokenError, InvalidCodeError } from 'wotp-client'

const wotp = createClient('http://localhost:54321', 'wotp_anon_xxx')

try {
  const { verified } = await wotp.verifyOTP(token, code)
} catch (error) {
  if (error instanceof RateLimitError) {
    console.log(`Rate limited. Retry after ${error.retryAfter}s`)
  } else if (error instanceof ExpiredTokenError) {
    console.log('Token expired — request a new OTP')
  } else if (error instanceof InvalidCodeError) {
    console.log(`Wrong code. ${error.attemptsRemaining} attempts left`)
  }
}
```

| Error Class | When |
|-------------|------|
| `RateLimitError` | Phone/IP exceeded rate limit (HTTP 429) |
| `ExpiredTokenError` | Token has expired (HTTP 410 or `expired_token`) |
| `InvalidCodeError` | Wrong OTP code (HTTP 400 + `invalid_code`) |
| `WotpError` | Base class for all SDK errors |

## Auto-Retry

Transient network errors (502, 503, 504, timeouts) are automatically retried with exponential backoff. Business errors (rate limit, expired token, invalid code) are **never** retried.

## Requirements

- Node.js ≥ 18 (uses native `fetch`)
- A running Wotp instance

## License

MIT — see [LICENSE](../../LICENSE) for details.
