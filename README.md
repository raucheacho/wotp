<div align="center">

# 🔐 Wotp (WhatsApp Open Tooling Platform)

**Your own WhatsApp API Gateway, self-hosted, one command.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://img.shields.io/github/actions/workflow/status/wotp/wotp/ci.yml?branch=main)](https://github.com/wotp/wotp/actions)

Your own WhatsApp infrastructure — OTPs, Transactional Alerts, and Webhooks. Includes API, beautiful Dashboard, and SDKs in Go, TypeScript, and Python.

</div>

---

> [!CAUTION]
> **⚠️ Important Disclaimer — Read Before Using**
>
> Wotp relies on [whatsmeow](https://github.com/tulir/whatsmeow), an **unofficial** library that reverse-engineers the WhatsApp Web protocol. This is **not** the official Meta Cloud API. This means:
>
> - **Risk of phone number ban** — Meta may ban your number without warning if usage patterns are deemed abnormal. A ban permanently cuts the WhatsApp channel.
> - **No SLA, no official support** — If WhatsApp changes its protocol, Wotp may break until the community (whatsmeow or Wotp) ships a fix.
> - **Recommended for**: MVPs, side-projects, products in validation phase with moderate user volume.
> - **Not recommended for**: Production at scale with zero downtime tolerance — in that case, use the [official Meta Cloud API](https://developers.facebook.com/docs/whatsapp/cloud-api/) once volume/budget justifies it.
>
> This transparency is intentional. It builds more trust than silence and prevents bad surprises for early adopters.

---

## Quickstart

```bash
npm install -g wotp-cli
wotp init my-project
cd my-project
wotp start
# Scan the QR code with WhatsApp → Done.
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                wotp-core (single Go binary)                 │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐ │
│  │  whatsmeow   │  │  OTP engine  │  │  REST API (Chi)   │ │
│  │  WA client   │  │  gen / verify│  │  + WebSocket hub  │ │
│  └──────────────┘  └──────────────┘  └───────────────────┘ │
│  ┌──────────────┐  ┌──────────────────────────────────────┐ │
│  │  SQLite      │  │  Dashboard (static, embed.FS)        │ │
│  │  storage     │  │  served on /dashboard                │ │
│  └──────────────┘  └──────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                            ▲
                            │ HTTP + WebSocket, port 54321
                            ▼
                   Browser / SDK client
```

One binary. One image. One port. One process.

## API Reference

All endpoints require an `apikey` header with your anon or service key.

### `POST /v1/otp/send`

Send an OTP to a phone number. The code is generated internally — the client only receives an opaque token.

```bash
curl -X POST http://localhost:54321/v1/otp/send \
  -H "Content-Type: application/json" \
  -H "apikey: wotp_anon_xxx" \
  -d '{"phone": "+212600000000"}'
```

```json
{
  "token": "otp_tok_9f8e7d6c",
  "expires_at": "2026-07-13T10:35:00Z"
}
```

### `POST /v1/otp/verify`

Verify an OTP code against a token.

```bash
curl -X POST http://localhost:54321/v1/otp/verify \
  -H "Content-Type: application/json" \
  -H "apikey: wotp_anon_xxx" \
  -d '{"token": "otp_tok_9f8e7d6c", "code": "483920"}'
```

**Success:**
```json
{ "verified": true, "phone": "+212600000000" }
```

**Failure:**
```json
{ "verified": false, "error": "invalid_code", "attempts_remaining": 3 }
```

### `POST /v1/messages/send`

Send a generic transactional message (text or media) to a phone number.

```bash
curl -X POST http://localhost:54321/v1/messages/send \
  -H "Content-Type: application/json" \
  -H "apikey: wotp_anon_xxx" \
  -d '{"phone": "+212600000000", "type": "text", "text": "Your order #1234 has shipped!"}'
```

```json
{
  "message_id": "3EB00436D840039D465DC3"
}
```

### Webhooks & Inbound Messages

Wotp supports **two-way communication**. You can configure a Webhook endpoint in your `config.toml` to instantly receive HTTP POST payloads when:
- `message.received`: A user replies to your WhatsApp number (Perfect for connecting AI Support Agents like n8n/Make).
- `message.sent`, `message.delivered`, `message.read`: Status updates for your OTPs and outbound messages.
- `session.disconnected`: If your phone loses connection.

### `GET /v1/health`

Check instance health and WhatsApp connection status.

```json
{
  "status": "connected",
  "phone": "+212600000000",
  "uptime_seconds": 123456
}
```

### Error Responses

| HTTP Code | Error | Description |
|-----------|-------|-------------|
| `400` | `invalid_code` | OTP code is incorrect |
| `400` / `410` | `expired_token` | Token has expired |
| `429` | — | Rate limit exceeded |

## SDK Examples

### TypeScript

```bash
npm install @wotp/client
```

```ts
import { createClient, RateLimitError, InvalidCodeError } from '@wotp/client'

const wotp = createClient('http://localhost:54321', 'wotp_anon_xxx')

// Send OTP
const { token, expiresAt } = await wotp.sendOTP('+212600000000')

// Verify
try {
  const { verified } = await wotp.verifyOTP(token, '483920')
  console.log(verified ? '✅ Verified!' : '❌ Failed')
} catch (err) {
  if (err instanceof InvalidCodeError) {
    console.log(`Wrong code. ${err.attemptsRemaining} attempts left`)
  }
}
```

### Go

```bash
go get github.com/wotp/wotp-go
```

```go
import wotp "github.com/wotp/wotp-go"

client := wotp.NewClient("http://localhost:54321", wotp.WithApiKey("wotp_anon_xxx"))

resp, err := client.SendOTP(ctx, "+212600000000")
if err != nil {
    log.Fatal(err)
}

result, err := client.VerifyOTP(ctx, resp.Token, "483920")
if wotp.IsInvalidCodeError(err) {
    fmt.Println("Wrong code")
}
```

### Python

```bash
pip install wotp
```

```python
from wotp import create_client, InvalidCodeError

client = create_client("http://localhost:54321", "wotp_anon_xxx")

resp = client.send_otp("+212600000000")

try:
    result = client.verify_otp(resp.token, "483920")
    print("✅ Verified!" if result.verified else "❌ Failed")
except InvalidCodeError as e:
    print(f"Wrong code. {e.attempts_remaining} attempts left")
```

## Configuration

All configuration lives in `wotp/config.toml`:

```toml
[project]
name = "my-project"

[api]
port = 54321
enable_dashboard = true

[otp]
code_length = 6
expiry_minutes = 5
max_attempts = 5
rate_limit_per_phone_per_hour = 3

[whatsapp]
device_name = "Wotp - my-project"
reconnect_backoff_seconds = [5, 15, 60, 300]

[storage]
driver = "sqlite"          # "sqlite" | "postgres"

[templates]
default_locale = "en"
```

Message templates in `wotp/seed/templates.toml`:

```toml
[en]
otp_message = "Your verification code: {{code}}. Valid for {{expiry}} minutes."

[fr]
otp_message = "Votre code de vérification : {{code}}. Valable {{expiry}} minutes."

[magic_link]
# You can use templates to send Magic Links instead of raw codes!
otp_message = "Click here to securely login: https://your-app.com/verify?code={{code}}"
```

## Project Structure

```
wotp/
├── core/                     # Go API server + dashboard
│   └── Dockerfile
├── sdks/
│   ├── typescript/           # @wotp/client (npm)
│   ├── go/                   # wotp-go (Go module)
│   └── python/               # wotp (PyPI)
├── .github/workflows/        # CI/CD
├── LICENSE
└── README.md
```

## Security

- **Two-tier API keys**: `anon` (send/verify only, rate-limited) and `service` (admin ops)
- **Rate limiting** per IP and per phone number, enforced at the API level
- **OTP codes are never stored in plain text** — hashed, never logged, never returned
- **Dashboard local-only by default** — explicit config required to expose publicly
- **`.env` is gitignored** with warnings never to commit it

## Roadmap

| Version | What's coming |
|---------|---------------|
| **v1.0** | Single-number instance, dashboard, 3 SDKs, CLI |
| **v1.1** | Dashboard authentication when exposed publicly |
| **v1.2** | Full Postgres support alongside SQLite |
| **v1.3** | Multi-language templates editable from dashboard |
| **v2.0** | Multi-number per instance (failover/round-robin) |
| **v2.x** | Optional SMS fallback |

## License

[MIT](LICENSE) — Use it, fork it, ship it.

---

<div align="center">

Built by [raucheacho](https://github.com/raucheacho) · Star ⭐ if this saves you from a BSP invoice

</div>
