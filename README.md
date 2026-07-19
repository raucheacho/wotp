<div align="center">

# Wotp (WhatsApp Open Tooling Platform)

**Your own WhatsApp API Gateway, self-hosted, one command.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://img.shields.io/github/actions/workflow/status/wotp/wotp/ci.yml?branch=main)](https://github.com/wotp/wotp/actions)

Your own WhatsApp infrastructure — OTPs, transactional alerts, and webhooks. Includes API, dashboard, and SDKs in Go, TypeScript, and Python. A single instance can host multiple isolated projects, each with its own WhatsApp numbers, API keys, and message history.

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
- [Multi-project walkthrough](#multi-project-walkthrough)
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
┌───────────────────────────────────────────────────────────────────┐
│                   wotp-core (single Go binary)                    │
│                                                                    │
│  ┌───────────────────────────────────────────────────────────┐    │
│  │  REST API (Chi) + WebSocket hub                           │    │
│  └───────────────────────────────────────────────────────────┘    │
│                              │                                    │
│           apikey → project ──┴── root key → /v1/projects*         │
│                              │                                    │
│   ┌──────────────────────────▼──────────────────────────────┐     │
│   │  project A          project B          project C  ...   │     │
│   │  ┌────────────┐    ┌────────────┐    ┌────────────┐     │     │
│   │  │ 1 number   │    │ 1 number   │    │ Cloud API  │     │     │
│   │  │ (whatsmeow)│    │ (whatsmeow)│    │ (Meta, OTP)│     │     │
│   │  │ OTP engine │    │ OTP engine │    │ OTP engine │     │     │
│   │  │ SQLite     │    │ SQLite     │    │ SQLite     │     │     │
│   │  └────────────┘    └────────────┘    └────────────┘     │     │
│   └───────────────────────────────────────────────────────────┘   │
│  ┌───────────────────────────────────────────────────────────┐    │
│  │  Dashboard (static, embed.FS) — project switcher           │    │
│  └───────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                            ▲
                            │ HTTP + WebSocket, port 54321
                            ▼
                   Browser / SDK client
```

One binary. One image. One port. One process — but **multi-tenant**: a single instance can host any number of isolated projects, each with its own WhatsApp number, API keys, and message history. A project is capped at exactly one number (see below for why) — running several numbers means running several projects, or several instances. No Redis, no Postgres, no message queue required — just SQLite: one file per project (`data/projects/<id>/{data.db,session.db}`), plus a shared `control.db` for the project/key registry.

**Why one number per project, not a pool:** an earlier design let a project round-robin/fail over across several whatsmeow numbers. It's not worth the complexity: for a two-way conversation, a customer's reply could land on a different number than the one they messaged, which is confusing at best. For one-shot sends like OTP, spreading traffic across several unofficial numbers dilutes ban risk without removing it, at the real operational cost of running several actual phone numbers. If OTP reliability at real scale matters, the answer is the Cloud API backend below, not more whatsmeow numbers.

A fresh install still works with zero project management: `wotp init && wotp start` auto-creates a `default` project, so the single-tenant quickstart above works unchanged. Multi-project is opt-in via `wotp project create`.

## Multi-project walkthrough

```bash
wotp init my-instance
cd my-instance
wotp start
# the "default" project is ready — scan its QR from the dashboard

# Create an isolated project for a client or a second app
wotp project create acme --name "Acme Corp"
# -> prints acme's anon_key / service_key, shown once

# Link a WhatsApp number to it
wotp project add-number acme
# -> open the dashboard, switch to "Acme Corp", scan the QR code shown there
# Each project is capped at one number — a second add-number call is
# rejected with a clear error instead of silently round-robining.

# List projects, or a project's numbers
wotp project list
wotp project keys acme

# Use acme's own anon key — completely separate data/history from "default"
curl -X POST http://localhost:54321/v1/otp/send \
  -H "apikey: <acme's anon key>" \
  -d '{"phone": "+212600000000"}'
```

Each project's OTPs, messages, webhooks, and WhatsApp numbers are fully isolated: a key from one project can never read or send through another's data.

## CLI reference

Every command besides `wotp project *` operates on the local `wotp/` directory (found by walking up from the current directory) and Docker directly.

| Command                                         | What it does                                                                                                      |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `wotp init [name]`                              | Scaffold a new instance: `config.toml`, seed templates, `.env` with anon/service/root keys                        |
| `wotp start`                                    | Render `docker-compose.yml`, pull the image if needed, start the container, print keys and the dashboard URL      |
| `wotp stop`                                     | Stop containers; all data is preserved                                                                            |
| `wotp restart`                                  | Stop, re-render config (in case it changed), start again                                                          |
| `wotp status`                                   | Check whether the container is running and the API is responding                                                  |
| `wotp logs`                                     | Stream container logs in real time                                                                                |
| `wotp keys`                                     | Show the anon/service/root keys stored in `.env`                                                                  |
| `wotp update`                                   | Pull the latest image and restart                                                                                 |
| `wotp reset`                                    | Delete **all** instance data (every project's WhatsApp sessions and history) and restart — forces a fresh QR scan |
| `wotp destroy`                                  | Remove containers and volumes; keeps `config.toml`, `.env`, and `seed/` so `wotp start` can recreate the instance |
| `wotp project create <slug> [--name]`           | Create a new project; prints its anon/service keys once                                                           |
| `wotp project list`                             | List projects on this instance                                                                                    |
| `wotp project rm <slug>`                        | Permanently delete a project and all its data                                                                     |
| `wotp project add-number <slug>`                | Start pairing a new WhatsApp number for a project                                                                 |
| `wotp project keys <slug>`                      | Show a project's numbers and their connection status                                                              |
| `wotp project keys <slug> --regenerate-anon`    | Issue a new anon key for that project, invalidating the previous one                                              |
| `wotp project keys <slug> --regenerate-service` | Same, for the service key                                                                                         |

`wotp project *` commands talk HTTP to the already-running instance using its root key (read from the local `.env`) — the instance itself is the source of truth for projects, not the CLI's local files.

## API Reference

Most endpoints require an `apikey` header with your **anon** or **service** key — each key belongs to exactly one project, and every request is scoped to that project automatically (no `project_id` to pass). Project _management_ (`/v1/projects*` below) requires the instance's **root** key instead.

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

List recent generic messages, or the WhatsApp contacts visible to the project's connected numbers. Both scoped to the calling key's project.

### Webhooks & inbound messages

Wotp supports **two-way communication**. Each project can configure its own webhook endpoint (via the dashboard or API) to instantly receive HTTP POST payloads when:

- `message.received`: a user replies to one of that project's WhatsApp numbers (useful for connecting AI support agents like n8n/Make).
- `message.sent`, `message.delivered`, `message.read`: status updates for your OTPs and outbound messages.
- `session.disconnected` / `session.reconnected`: a number loses or regains connection.

By default, wotp **ignores group chats and WhatsApp status (stories) updates** for every project — `message.received` only fires for direct 1:1 messages, keeping your webhook feed and dashboard focused on transactional conversations. Toggle `ignore_groups`/`ignore_status` per project from the dashboard.

### `GET /v1/health`

Instance-wide liveness check (no auth required). Per-number connection status is per-project — see `wotp project keys <slug>` or the dashboard.

```json
{
  "status": "ok",
  "uptime_seconds": 123456
}
```

### Multi-project management

Managing projects requires the instance's **root** key (printed by `wotp init`/`wotp start`, or via `wotp keys`) — everything below is also available as `wotp project ...` CLI commands.

#### `POST /v1/projects`

```bash
curl -X POST http://localhost:54321/v1/projects \
  -H "apikey: wotp_root_xxx" \
  -d '{"slug": "acme", "name": "Acme Corp"}'
```

Returns the new project plus its freshly generated `anon_key`/`service_key` — shown once, like at `wotp init` time.

#### `GET /v1/projects` · `DELETE /v1/projects/{id}`

List or permanently delete a project (its numbers, message history, and API keys).

#### `POST /v1/projects/{id}/numbers/pair`

Starts pairing a new WhatsApp number for that project. The QR code streams via the WebSocket hub (`number.qr` event, scoped to that project) and is also renderable at `GET /v1/projects/{id}/numbers/qr` — same PNG/JSON behavior the dashboard uses.

#### `GET /v1/projects/{id}/numbers`

```json
[
  {
    "jid": "212600000000@s.whatsapp.net",
    "phone": "212600000000",
    "connected": true
  }
]
```

#### `POST /v1/projects/{id}/keys/regenerate`

`{"tier": "anon" | "service"}` — issues a new key for that tier, immediately invalidating the previous one.

### Error responses

| HTTP Code     | Error                                       | Description                                                                |
| ------------- | ------------------------------------------- | -------------------------------------------------------------------------- |
| `400`         | `invalid_code`                              | OTP code is incorrect                                                      |
| `400` / `410` | `expired_token`                             | Token has expired                                                          |
| `401`         | `invalid api key` / `missing apikey header` | Key is missing, unknown, or malformed                                      |
| `403`         | `insufficient permissions`                  | Key's tier can't access this endpoint (e.g. an anon key on `/v1/projects`) |
| `429`         | `rate_limit_exceeded`                       | Rate limit exceeded (per phone number for OTPs, per minute for messages)   |

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

All three SDKs take the same `apikey` your project's anon or service key — nothing SDK-side changes when you add more projects, since each project simply has its own key.

## Configuration

`wotp/config.toml` holds **instance-wide** settings only — everything else (OTP parameters, WhatsApp inbound filters, messaging, webhooks, default locale) is **per-project**, since a single instance can host many projects with different needs:

```toml
[project]
name = "my-project"

[api]
port = 54321
enable_dashboard = true

[storage]
driver = "sqlite"          # "sqlite" | "postgres"
```

Per-project settings (`code_length`, `expiry_minutes`, `max_attempts`, `rate_limit_per_phone_per_hour`, `ignore_groups`, `ignore_status`, `simulate_typing`, webhook endpoint/secret, default locale, etc.) live in each project's row in the instance's control database and are managed via the dashboard or the API — not `config.toml`. A freshly created project gets sensible defaults (6-digit codes, 5-minute expiry, group/status events ignored) matching what a single-tenant instance shipped with before.

Message templates in `wotp/seed/templates.toml` (shared across every project on the instance for now — per-project templates are on the roadmap):

```toml
[en]
otp_message = "Your verification code: {{code}}. Valid for {{expiry}} minutes."

[fr]
otp_message = "Votre code de vérification : {{code}}. Valable {{expiry}} minutes."

[magic_link]
# You can use templates to send Magic Links instead of raw codes!
otp_message = "Click here to securely login: https://your-app.com/verify?code={{code}}"
```

### Meta Cloud API backend (optional, per project, alongside whatsmeow)

By default a project sends everything through whatsmeow (unofficial, instant to set up — scan a QR and go). For OTP traffic specifically, whatsmeow numbers can get banned by WhatsApp: automated, one-shot messages to strangers are exactly the pattern its abuse detection looks for. If that matters for your deployment, a project can instead send OTPs through the official Meta WhatsApp Cloud API, which has no such ban risk — Meta hosts the connection, there's no session/device to protect.

This is a **per-project** setting (`Settings.Cloud`), not instance-wide — two projects on the same instance never share a Cloud number/token. Configure it from the dashboard's Numbers screen ("Configure" on the Cloud API card), or set it directly via `PATCH /v1/projects/{id}/settings`:

```bash
curl -X PATCH http://localhost:54321/v1/projects/<project-id>/settings \
  -H "apikey: <root key>" \
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

Once enabled, `POST /v1/otp/send` for that project sends the code through `otp_template_name` instead of rendering a free-text `templates.toml` message — Meta requires OTPs to go through a pre-approved template, since an OTP is always the first message in a conversation and so always outside the 24-hour window free-form text needs. You can get a Meta phone number ID and access token without any business verification via Meta's free developer test tier (capped at 5 allow-listed recipients) to build and test against before registering a real number.

Everything other than OTP sending (messages, presence, chat listing) still goes through whatsmeow for that project — the Cloud backend today only covers the OTP path. Since a project is capped at one whatsmeow number anyway (see above), a project either sends OTPs via whatsmeow or via Cloud; it never needs both for the same traffic.

## Project structure

```
wotp/
├── core/                        # wotp-core: Go API server + embedded dashboard
│   ├── cmd/wotp-core/           # main() — wiring, graceful shutdown
│   ├── internal/
│   │   ├── api/                 # HTTP handlers, routing, auth middleware
│   │   ├── project/             # Registry: lazily loads/caches a per-project Runtime
│   │   ├── whatsapp/            # whatsmeow client (Pool, one number per project) + Meta Cloud API client (CloudClient)
│   │   ├── otp/                 # code generation, hashing, verification
│   │   ├── keys/                # API key generation/validation (anon/service/root tiers)
│   │   ├── store/               # SQLite: ControlStore (projects/keys) + ProjectStore (otps/messages/webhooks)
│   │   ├── templates/           # OTP message template rendering
│   │   ├── webhooks/            # outbound webhook dispatch
│   │   └── ws/                  # WebSocket hub (broadcasts scoped per project)
│   ├── dashboard/                # React + Vite dashboard, embedded into the Go binary via go:embed
│   └── Dockerfile
├── cli/                          # wotp: the CLI (Cobra), drives Docker + talks HTTP for `project` commands
│   ├── cmd/wotp/
│   └── internal/
│       ├── commands/             # init, start, stop, project, ...
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

- **Three API key tiers**: `anon` (send/verify only, rate-limited, scoped to one project), `service` (per-project admin), `root` (instance-wide — create/list/delete projects, pair numbers)
- **Rate limiting** per phone number (OTPs) and per minute (messages), enforced per project
- **OTP codes are never stored in plain text** — hashed, never logged, never returned
- **Dashboard is local-only and unauthenticated by default** (trusted by network position) — put it behind a reverse proxy with auth before exposing it publicly (see Roadmap)
- **`.env` is gitignored** and holds anon/service/root keys in plaintext locally — never commit it
- **Each project's data lives in its own SQLite file** — a bug or corruption in one project can't affect another's

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
| **v1.0** | Single-number instance, dashboard, 3 SDKs, CLI                                                             |
| **v2.0** | Shipped — multi-project instances (one WhatsApp number per project), optional Cloud API backend for OTP, admin dashboard |
| **v3.0** | Dashboard authentication when exposed publicly                                                             |

## License

[MIT](LICENSE) — Use it, fork it, ship it.

---

<div align="center">

Built by [raucheacho](https://github.com/raucheacho) · Star this repo if it saves you from a BSP invoice

</div>
