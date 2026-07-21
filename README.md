<div align="center">

# Wotp (WhatsApp Open Tooling Platform)

**Your own WhatsApp API Gateway, self-hosted, one command.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://img.shields.io/github/actions/workflow/status/wotp/wotp/ci.yml?branch=main)](https://github.com/wotp/wotp/actions)

Your own WhatsApp infrastructure — OTPs, transactional alerts, and webhooks. Includes API, dashboard, and SDKs in Go, TypeScript, and Python. Each instance runs one WhatsApp number end to end; need a second number (e.g. OTP + a delivery bot), run a second instance.

</div>

---

> [!CAUTION]
> **Important disclaimer — read before using**
>
> Wotp relies on [whatsmeow](https://github.com/tulir/whatsmeow), an **unofficial** library that reverse-engineers the WhatsApp Web protocol. This is **not** the official Meta Cloud API. This means:
>
> - **Risk of phone number ban** — Meta may ban your number without warning if usage patterns are deemed abnormal. A ban permanently cuts the WhatsApp channel.
> - **No SLA, no official support** — If WhatsApp changes its protocol, Wotp may break until the community (whatsmeow or Wotp) ships a fix.
> - **Recommended for**: MVPs, side-projects, products in validation phase with moderate user volume.
> - **Not recommended for**: production at scale with zero downtime tolerance — in that case, use the [official Meta Cloud API](https://developers.facebook.com/docs/whatsapp/cloud-api/) once volume/budget justifies it.
>
> This transparency is intentional. It builds more trust than silence and prevents bad surprises for early adopters.

---

## Contents

- [Quickstart](#quickstart)
- [Architecture](#architecture)
- [CLI reference](#cli-reference)
- [API reference](#api-reference)
- [SDK examples](#sdk-examples)
- [Configuration](#configuration)
- [Project structure](#project-structure)
- [Security](#security)
- [Development](#development)
- [Roadmap](#roadmap)
- [License](#license)

## Quickstart

```bash
# macOS / Linux, via Homebrew
brew install raucheacho/tap/wotp

# or download a prebuilt binary for your platform from the Releases page:
# https://github.com/raucheacho/wotp/releases
```

```bash
wotp init my-project
cd my-project
wotp start
# Open the dashboard link printed in your terminal and scan the QR code with WhatsApp → done.
```

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                 wotp-core (single Go binary)                  │
│                                                                │
│  ┌───────────────────────────────────────────────────────┐    │
│  │  REST API (Chi) + WebSocket hub                        │    │
│  └───────────────────────────────────────────────────────┘    │
│                              │                                │
│                apikey → anon | service tier                   │
│                              │                                │
│   ┌──────────────────────────▼──────────────────────────┐     │
│   │  1 WhatsApp number (whatsmeow) or Cloud API (Meta)   │     │
│   │  OTP engine · conversations · SQLite                 │     │
│   └───────────────────────────────────────────────────────┘   │
│  ┌───────────────────────────────────────────────────────┐    │
│  │  Dashboard (static, embed.FS)                          │    │
│  └───────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                            ▲
                            │ HTTP + WebSocket, port 54321
                            ▼
                   Browser / SDK client
```

One binary. One image. One port. One process. **Mono-tenant**: one instance runs exactly one WhatsApp number (whatsmeow) and/or one Cloud API backend. No Redis, no Postgres, no message queue required — just SQLite, three files under `data/`: `control.db` (API keys, paired number, settings), `data.db` (OTPs, messages, conversations, webhook logs), `session.db` (whatsmeow's own device state).

**Why one number per instance, not a pool:** an earlier design let one instance round-robin/fail over across several whatsmeow numbers. It's not worth the complexity: for a two-way conversation, a customer's reply could land on a different number than the one they messaged, which is confusing at best. For one-shot sends like OTP, spreading traffic across several unofficial numbers dilutes ban risk without removing it, at the real operational cost of running several actual phone numbers. If OTP reliability at real scale matters, the answer is the Cloud API backend below, not more whatsmeow numbers. Need a genuinely separate number (an OTP number and a delivery-bot number, say) — run a second `wotp init && wotp start` in a second directory. Each instance is fully self-contained: its own keys, its own data, its own port.

## CLI reference

Every command operates on the local `wotp/` directory (found by walking up from the current directory) and Docker directly.

| Command             | What it does                                                                                                       |
| -------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `wotp init [name]`  | Scaffold a new instance: `config.toml`, seed templates, `.env` with anon/service keys                              |
| `wotp start`        | Render `docker-compose.yml`, pull the image if needed, start the container, print keys and the dashboard URL       |
| `wotp stop`         | Stop containers; all data is preserved                                                                             |
| `wotp restart`      | Stop, re-render config (in case it changed), start again                                                           |
| `wotp status`       | Check whether the container is running and the API is responding                                                  |
| `wotp logs`         | Stream container logs in real time                                                                                 |
| `wotp keys`         | Show the anon/service keys stored in `.env`                                                                        |
| `wotp update`       | Pull the latest image and restart                                                                                  |
| `wotp reset`        | Delete **all** instance data (WhatsApp session and history) and restart — forces a fresh QR scan                   |
| `wotp destroy`      | Remove containers and volumes; keeps `config.toml`, `.env`, and `seed/` so `wotp start` can recreate the instance  |

## API Reference

Most endpoints require an `apikey` header with your **anon** or **service** key. Instance-admin endpoints (number pairing, key rotation, settings) require **service**.

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

Send a generic transactional message to a phone number. `type` is `text`, `image`, `video`, `audio`, `document`, or `location` (`media` is accepted too, as a legacy alias for `image`). Media sends take `url` (a public link) and, for `document`, `filename`; `caption` works for every kind except `audio` — WhatsApp's protocol carries no caption field for voice notes. Location sends take `latitude`/`longitude` (required) and `name`/`address` (optional).

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

### `POST /v1/messages/presence`

Set the typing indicator for a chat without sending a message — useful for AI agents that want to show "typing…" while composing a reply.

```bash
curl -X POST http://localhost:54321/v1/messages/presence \
  -H "Content-Type: application/json" \
  -H "apikey: wotp_anon_xxx" \
  -d '{"phone": "+212600000000", "state": "typing"}'
```

`state` is `"typing"` or `"paused"`.

### `GET /v1/messages` · `GET /v1/chats`

List recent generic messages, or the WhatsApp contacts visible to the connected number.

### Webhooks & inbound messages

Wotp supports **two-way communication**. Configure a webhook endpoint (via the dashboard or `PATCH /v1/settings`) to instantly receive HTTP POST payloads when:

- `message.received`: a user replies to your WhatsApp number (useful for connecting AI support agents like n8n/Make).
- `message.sent`, `message.delivered`, `message.read`: status updates for your OTPs and outbound messages.
- `session.disconnected` / `session.reconnected`: the number loses or regains connection.

By default, wotp **ignores group chats and WhatsApp status (stories) updates** — `message.received` only fires for direct 1:1 messages, keeping your webhook feed and dashboard focused on transactional conversations. Toggle `ignore_groups`/`ignore_status` from the dashboard.

### Conversations & human takeover

Every inbound message from a contact is tracked in a **conversation** (one per phone number, created automatically on first contact) that's either `bot` (default) or `human`. Take a conversation over from your own app's admin UI or an LLM tool call — **not from the wotp dashboard**, which stays infra-only by design; conversations, agents, and takeover UX belong in your application, wotp just gives it the mechanism. Every send (OTP included) is linked to its conversation too, so `GET /v1/conversations/<id>/messages` shows the full thread — inbound replies, outbound sends, and OTPs — in one chronological list.

```bash
# List conversations
curl -H "apikey: wotp_anon_xxx" http://localhost:54321/v1/conversations

# Full history for one conversation (inbound + outbound, chronological)
curl -H "apikey: wotp_anon_xxx" http://localhost:54321/v1/conversations/<id>/messages

# Take over — marks the conversation human; wotp keeps forwarding
# message.received either way, now carrying conversation_state so your
# bot knows to stay quiet
curl -X POST http://localhost:54321/v1/conversations/<id>/takeover \
  -H "apikey: wotp_anon_xxx" \
  -d '{"actor": "agent-42", "reason": "customer asked for a refund"}'

# Hand it back to the bot
curl -X POST http://localhost:54321/v1/conversations/<id>/resume \
  -H "apikey: wotp_anon_xxx" \
  -d '{"actor": "agent-42", "reason": "resolved"}'
```

`actor`/`reason` are optional but recorded on every takeover/resume — a takeover is never a silent state flip. wotp doesn't decide whether your bot should act on a human-owned conversation — it always forwards `message.received` and always records/broadcasts it over the WebSocket hub, `human` or not. It's up to your app's own logic to read `conversation_state` from the webhook payload and skip auto-replying when it's `human`. This also means your app keeps full visibility (logging, notifications, anything else) even while a human is handling the thread — wotp never goes silent. Sending a reply while `human` uses the same `POST /v1/messages/send` you already use for outbound messages — there's no separate "send as human" endpoint.

### Inbound media (`GET /v1/media/{message_id}`)

When a contact sends an image, video, voice note, or document, wotp downloads it right away (both on whatsmeow and Cloud API) and makes the raw bytes retrievable — so a bot built on wotp can pull the file and run it through Whisper, OCR, or whatever else your app needs, without wotp itself trying to be smart about the content:

```bash
curl -H "apikey: wotp_anon_xxx" http://localhost:54321/v1/media/<message_id> -o file
```

`GET /v1/conversations/<id>/messages` and the `message.received` webhook payload both carry `media_kind` (`image`/`video`/`audio`/`document`) and the mimetype for any message that had an attachment — a caption, if the message had one, shows up as the message's `content`/`text` same as a plain text message would. Note two limitations by design: there's no retention or cleanup policy for downloaded media yet (files live under the instance's data directory until you delete them, same all-or-nothing model as `wotp reset`), and `GET /v1/media/{id}` 404s if the download itself failed at receive time even though the message row exists — check for a non-empty `media_kind` before assuming the file is there.

### `GET /v1/health`

Instance-wide liveness check (no auth required).

```json
{
  "status": "ok",
  "uptime_seconds": 123456
}
```

### Instance administration

These require the **service** key.

#### `POST /v1/numbers/pair`

Starts pairing a WhatsApp number. The QR code streams via the WebSocket hub (`number.qr` event) and is also renderable at `GET /v1/numbers/qr` — same PNG/JSON behavior the dashboard uses. Rejected with `409` if a number is already paired — each instance is capped at one.

#### `GET /v1/numbers`

```json
[
  {
    "jid": "212600000000@s.whatsapp.net",
    "phone": "212600000000",
    "connected": true
  }
]
```

#### `GET /v1/cloud-status`

Whether the Cloud API backend is enabled and its credentials verified — see [Meta Cloud API backend](#meta-cloud-api-backend-optional-alongside-whatsmeow) below.

#### `POST /v1/keys/regenerate`

`{"tier": "anon" | "service"}` — issues a new key for that tier, immediately invalidating the previous one.

#### `GET /v1/settings` · `PATCH /v1/settings`

Read or partially update the instance's OTP/messaging/WhatsApp/webhook/Cloud settings — see [Configuration](#configuration). A `PATCH` takes effect immediately (no restart needed), without disconnecting an already-paired number.

### Error responses

| HTTP Code     | Error                                       | Description                                                            |
| ------------- | -------------------------------------------- | ----------------------------------------------------------------------- |
| `400`         | `invalid_code`                              | OTP code is incorrect                                                  |
| `400` / `410` | `expired_token`                             | Token has expired                                                      |
| `401`         | `invalid api key` / `missing apikey header` | Key is missing, unknown, or malformed                                  |
| `403`         | `insufficient permissions`                  | Key's tier can't access this endpoint (e.g. an anon key on `/v1/numbers`) |
| `429`         | `rate_limit_exceeded`                       | Rate limit exceeded (per phone number for OTPs, per minute for messages) |

## SDK Examples

### TypeScript

```bash
npm install wotp-client
```

```ts
import { createClient, RateLimitError, InvalidCodeError } from "wotp-client";

const wotp = createClient("http://localhost:54321", "wotp_anon_xxx");

// Send OTP
const { token, expiresAt } = await wotp.sendOTP("+212600000000");

// Verify
try {
  const { verified } = await wotp.verifyOTP(token, "483920");
  console.log(verified ? "Verified" : "Failed");
} catch (err) {
  if (err instanceof InvalidCodeError) {
    console.log(`Wrong code. ${err.attemptsRemaining} attempts left`);
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
    print("Verified" if result.verified else "Failed")
except InvalidCodeError as e:
    print(f"Wrong code. {e.attempts_remaining} attempts left")
```

All three SDKs take the same `apikey` — the instance's anon or service key.

## Configuration

`wotp/config.toml` holds **instance-wide** deployment settings only — display name, API port, storage driver:

```toml
[project]
name = "my-project"

[api]
port = 54321
enable_dashboard = true

[storage]
driver = "sqlite"          # "sqlite" | "postgres"
```

Everything else — OTP parameters (`code_length`, `expiry_minutes`, `max_attempts`, `rate_limit_per_phone_per_hour`), messaging (`max_messages_per_minute`, `simulate_typing`), WhatsApp inbound filters (`ignore_groups`, `ignore_status`), webhook endpoint/secret, default locale, and the Cloud API backend — lives in one settings row in the instance's control database and is managed via the dashboard or `PATCH /v1/settings`, not `config.toml`. A fresh instance gets sensible defaults (6-digit codes, 5-minute expiry, group/status events ignored).

Message templates live in `wotp/seed/templates.toml`:

```toml
[en]
otp_message = "Your verification code: {{code}}. Valid for {{expiry}} minutes."

[fr]
otp_message = "Votre code de vérification : {{code}}. Valable {{expiry}} minutes."

[magic_link]
# You can use templates to send Magic Links instead of raw codes!
otp_message = "Click here to securely login: https://your-app.com/verify?code={{code}}"
```

### Meta Cloud API backend (optional, alongside whatsmeow)

By default an instance sends everything through whatsmeow (unofficial, instant to set up — scan a QR and go). For OTP traffic specifically, whatsmeow numbers can get banned by WhatsApp: automated, one-shot messages to strangers are exactly the pattern its abuse detection looks for. If that matters for your deployment, an instance can instead send OTPs through the official Meta WhatsApp Cloud API, which has no such ban risk — Meta hosts the connection, there's no session/device to protect.

Configure it from the dashboard's Numbers screen ("Configure" on the Cloud API card), or set it directly via `PATCH /v1/settings`:

```bash
curl -X PATCH http://localhost:54321/v1/settings \
  -H "apikey: <service key>" \
  -d '{
    "cloud": {
      "enabled": true,
      "phone_number_id": "<your Meta phone_number_id>",
      "access_token": "<your Meta access token>",
      "otp_template_name": "otp_verification",
      "otp_template_language": "en_US"
    }
  }'
```

Once enabled, `POST /v1/otp/send` sends the code through `otp_template_name` instead of rendering a free-text `templates.toml` message — Meta requires OTPs to go through a pre-approved template, since an OTP is always the first message in a conversation and so always outside the 24-hour window free-form text needs. You can get a Meta phone number ID and access token without any business verification via Meta's free developer test tier (capped at 5 allow-listed recipients) to build and test against before registering a real number.

`POST /v1/messages/send` (text and media — `type` can be `image`, `video`, `audio`, or `document`) also prefers Cloud once it's enabled, same as OTP — a Cloud-only instance isn't limited to OTP. `POST /v1/messages/presence` and `GET /v1/chats` prefer Cloud too. Since an instance is capped at one whatsmeow number anyway (see above), sends go via whichever backend is configured; it never needs both for the same traffic. This is deliberate: build and test against whatsmeow (no business verification needed, just scan a QR), then flip `cloud.enabled` for production — your integration code doesn't change.

#### Receiving replies on a Cloud number (optional)

By default a Cloud-backed instance is send-only: OTPs and messages go out, but a customer's reply never arrives — Meta delivers inbound traffic to a webhook URL you register, not a socket wotp can just listen on. To receive replies (and populate conversations — see above — for a Cloud number), set four more fields alongside the ones above:

```json
{
  "cloud": {
    "waba_id": "<your WhatsApp Business Account ID>",
    "pin": "<the 6-digit two-step-verification PIN set in Meta WhatsApp Manager for this number>",
    "app_secret": "<your Meta App Secret>",
    "verify_token": "<any string you choose>"
  }
}
```

Once saved, wotp calls Meta's `/register` and `/subscribed_apps` for you (logged, non-fatal if they fail — an OTP-only setup with no `pin` skips this step entirely). Then, in the Meta app dashboard's webhook subscription settings, paste the webhook URL shown on the dashboard's Numbers screen (`https://<your-domain>/webhooks/meta`) and the same `verify_token` — Meta will hit it with a verification handshake, then start POSTing inbound messages and delivery/read receipts, signed with your App Secret so wotp can verify they actually came from Meta. This needs a publicly reachable HTTPS URL for your instance (a domain + reverse proxy, or a tunnel like ngrok while testing) — `localhost` only works for your own dashboard, not for Meta to reach.

#### Presence and chat listing on Cloud

Both work on Cloud too, once inbound is set up (see above) — with two structural differences from whatsmeow that the API absorbs so your integration code doesn't need to care:

- **Presence** (`/v1/messages/presence`): Meta's Cloud API has no bare "start/stop typing for this number" call — a typing indicator is a side effect of marking a *specific inbound message* as read. wotp looks up the customer's most recent inbound message for you; if they've never messaged this number, presence errors clearly rather than silently no-op'ing (there's nothing to show a typing indicator in response to yet).
- **Chat listing** (`/v1/chats`): Cloud has no contact-list endpoint at all — Meta simply doesn't expose one. wotp returns the numbers recorded in this instance's own conversation history instead (see [Conversations & human takeover](#conversations--human-takeover)), which only has entries once inbound is set up.

## Project structure

```
wotp/
├── core/                        # wotp-core: Go API server + embedded dashboard
│   ├── cmd/wotp-core/           # main() — wiring, graceful shutdown
│   ├── internal/
│   │   ├── api/                 # HTTP handlers, routing, auth middleware
│   │   ├── project/             # Load(): builds the instance's single Runtime
│   │   ├── whatsapp/            # whatsmeow client (Pool, one number) + Meta Cloud API client (CloudClient)
│   │   ├── otp/                 # code generation, hashing, verification
│   │   ├── keys/                # API key generation/validation (anon/service tiers)
│   │   ├── store/                # SQLite: ControlStore (keys/number/settings) + ProjectStore (otps/messages/webhooks/conversations)
│   │   ├── templates/           # OTP message template rendering
│   │   ├── webhooks/            # outbound webhook dispatch
│   │   └── ws/                  # WebSocket hub
│   ├── dashboard/                # React + Vite dashboard, embedded into the Go binary via go:embed
│   └── Dockerfile
├── cli/                          # wotp: the CLI (Cobra), drives Docker
│   ├── cmd/wotp/
│   └── internal/
│       ├── commands/             # init, start, stop, ...
│       ├── config/               # local config.toml read/write, project directory discovery
│       ├── docker/               # docker-compose template rendering + exec wrappers
│       ├── keys/                 # local .env read/write
│       └── ui/                   # terminal output styling (lipgloss)
├── sdks/
│   ├── typescript/                # wotp-client (npm)
│   ├── go/                        # wotp-go (Go module)
│   └── python/                    # wotp (PyPI)
├── .github/workflows/             # CI/CD: build, GoReleaser, SDK publishing
├── .goreleaser.yml                 # CLI binary releases (GitHub, Homebrew, Scoop)
├── Makefile                        # local multi-binary build into bin/
├── LICENSE
└── README.md
```

## Security

- **Two API key tiers**: `anon` (send/verify only, rate-limited) and `service` (instance admin — number pairing, key rotation, settings)
- **Rate limiting** per phone number (OTPs) and per minute (messages)
- **OTP codes are never stored in plain text** — hashed, never logged, never returned
- **Dashboard is local-only and unauthenticated by default** (trusted by network position) — put it behind a reverse proxy with auth before exposing it publicly (see Roadmap)
- **`.env` is gitignored** and holds anon/service keys in plaintext locally — never commit it
- **Your number stays yours** — history lives in a SQLite file under `data/` that you own; back it up or export it like any local file, no vendor lock-in

## Development

Requires Go 1.23+, Node.js or Bun (dashboard), and Docker (for running the CLI's own integration path).

```bash
# Build every binary into bin/
make

# Or individually
make build-core        # bin/wotp-core
make build-cli          # bin/wotp
make build-dashboard     # builds core/dashboard/dist, embedded into wotp-core via go:embed

# Run wotp-core directly, without Docker or the CLI
cd core
go run ./cmd/wotp-core -config=config.toml -templates=templates.toml -data=./data

# Tests
cd core && go test ./...
cd cli  && go test ./...
cd core/dashboard && npx tsc --noEmit && npm run build
```

## Roadmap

| Version  | What's coming                                                                               |
| -------- | ------------------------------------------------------------------------------------------- |
| **v1.0** | Single-number instance, dashboard, 3 SDKs, CLI, optional Cloud API backend for OTP — shipped |
| **v2.0** | Dashboard authentication when exposed publicly                                              |

## License

[MIT](LICENSE) — Use it, fork it, ship it.

---

<div align="center">

Built by [raucheacho](https://github.com/raucheacho) · Star this repo if it saves you from a BSP invoice

</div>
