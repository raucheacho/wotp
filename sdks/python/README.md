# wotp

Official Python SDK for **Wotp** — WhatsApp OTP, self-hosted, one command.

[![PyPI](https://img.shields.io/pypi/v/wotp)](https://pypi.org/project/wotp/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Installation

```bash
pip install wotp
```

## Quick Start

```python
from wotp import create_client

client = create_client("http://localhost:54321", "wotp_anon_xxx")

# Send an OTP
resp = client.send_otp("+212600000000")
print(f"Token: {resp.token}, expires at: {resp.expires_at}")

# Verify the code entered by the user
result = client.verify_otp(resp.token, "483920")

# Send a text message
text_res = client.send_text("+212600000000", "Hello world")

# Send a media message
media_res = client.send_media("+212600000000", url="https://example.com/image.png")

# Show a typing indicator
client.set_presence("+212600000000", "typing")

# List chats
chats = client.get_chats()

print(f"Verified: {result.verified}")
```

## API Reference

### `create_client(url, api_key, **options)`

Creates a new Wotp client instance.

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | `str` | Base URL of your Wotp instance |
| `api_key` | `str` | Your anon or service API key |
| `max_retries` | `int` | Max retries on transient errors (default: `3`) |
| `retry_delay` | `float` | Base delay in seconds between retries (default: `0.5`) |
| `timeout` | `float` | Request timeout in seconds (default: `10.0`) |

### `client.send_otp(phone)`

Send an OTP to the given phone number.

- **Parameters:** `phone` — E.164 formatted phone number
- **Returns:** `SendOTPResponse` with `.token` and `.expires_at`
- **Raises:** `RateLimitError`

### `client.verify_otp(token, code)`

Verify an OTP code against a previously issued token.

- **Parameters:** `token` — from `send_otp`, `code` — the user-entered code
- **Returns:** `VerifyOTPResponse` with `.verified`, `.phone`, `.attempts_remaining`
- **Raises:** `ExpiredTokenError`, `InvalidCodeError`

### `client.health()`

Instance-wide liveness check (no notion of a single connected phone number — an instance can host many projects, each with their own numbers).

- **Returns:** `HealthResponse` with `.status`, `.uptime_seconds`

### `client.send_text(phone, text)`

Send a text message to the given phone number.

- **Returns:** `MessageResponse` with `.message_id`

### `client.send_media(phone, url=None, base64=None, caption=None)`

Send a media message. Provide either `url` or `base64`.

- **Returns:** `MessageResponse` with `.message_id`

### `client.get_chats()`

List the WhatsApp contacts visible to the project's connected numbers.

- **Returns:** `list[Chat]` — each `Chat` has `.jid` and `.name`

### `client.set_presence(phone, state)`

Set the typing indicator for a chat without sending a message. `state` is `"typing"` or `"paused"`.

## Error Handling

The SDK raises typed exceptions for business failures:

```python
from wotp import create_client, RateLimitError, ExpiredTokenError, InvalidCodeError

client = create_client("http://localhost:54321", "wotp_anon_xxx")

try:
    result = client.verify_otp(token, code)
except RateLimitError as e:
    print(f"Rate limited. Retry after {e.retry_after}s")
except ExpiredTokenError:
    print("Token expired — request a new OTP")
except InvalidCodeError as e:
    print(f"Wrong code. {e.attempts_remaining} attempts left")
```

| Exception | When |
|-----------|------|
| `RateLimitError` | Phone/IP exceeded rate limit (HTTP 429) |
| `ExpiredTokenError` | Token has expired (HTTP 410 or `expired_token`) |
| `InvalidCodeError` | Wrong OTP code (HTTP 400 + `invalid_code`) |
| `WotpError` | Base class for all SDK errors |

## Context Manager

The client can be used as a context manager to ensure proper cleanup:

```python
with create_client("http://localhost:54321", "wotp_anon_xxx") as client:
    resp = client.send_otp("+212600000000")
```

## Auto-Retry

Transient errors (502, 503, 504, network timeouts) are automatically retried with exponential backoff. Business errors are **never** retried.

## Requirements

- Python ≥ 3.10
- A running Wotp instance

## License

MIT — see [LICENSE](../../LICENSE) for details.
