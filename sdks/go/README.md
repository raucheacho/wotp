# wotp-go

Official Go SDK for **Wotp** — WhatsApp OTP, self-hosted, one command.

[![Go Reference](https://pkg.go.dev/badge/github.com/wotp/wotp-go.svg)](https://pkg.go.dev/github.com/wotp/wotp-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Installation

```bash
go get github.com/wotp/wotp-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    wotp "github.com/wotp/wotp-go"
)

func main() {
    ctx := context.Background()
    client := wotp.NewClient("http://localhost:54321", wotp.WithApiKey("wotp_anon_xxx"))

    // Send an OTP
    resp, err := client.SendOTP(ctx, "+212600000000")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Token: %s (expires at %s)\n", resp.Token, resp.ExpiresAt)

    // Verify the code entered by the user
    result, err := client.VerifyOTP(ctx, resp.Token, "483920")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Verified: %v\n", result.Verified)

    // Send a text message
    textRes, err := client.SendText(ctx, "+212600000000", "Hello world")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Message ID: %s\n", textRes.MessageID)

    // Send a media message
    _, err = client.SendMedia(ctx, "+212600000000", wotp.SendMediaRequest{
        URL: "https://example.com/image.png",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Show a typing indicator
    if err := client.SetPresence(ctx, "+212600000000", wotp.PresenceTyping); err != nil {
        log.Fatal(err)
    }

    // List chats
    chats, err := client.GetChats(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Chats: %d\n", len(chats))
}
```

## API Reference

### `NewClient(baseURL, opts...)`

Creates a new Wotp client with functional options.

```go
client := wotp.NewClient("http://localhost:54321",
    wotp.WithApiKey("wotp_anon_xxx"),
    wotp.WithMaxRetries(5),
    wotp.WithRetryDelay(1 * time.Second),
    wotp.WithTimeout(15 * time.Second),
    wotp.WithHTTPClient(customHTTPClient),
)
```

| Option | Description | Default |
|--------|-------------|---------|
| `WithApiKey(key)` | API key for authentication | `""` |
| `WithMaxRetries(n)` | Max retries on transient errors | `3` |
| `WithRetryDelay(d)` | Base delay between retries | `500ms` |
| `WithTimeout(d)` | HTTP request timeout | `10s` |
| `WithHTTPClient(c)` | Custom `*http.Client` | default client |

### `client.SendOTP(ctx, phone)`

Send an OTP to the given phone number (E.164 format).

```go
resp, err := client.SendOTP(ctx, "+212600000000")
// resp.Token, resp.ExpiresAt
```

### `client.VerifyOTP(ctx, token, code)`

Verify an OTP code against a previously issued token.

```go
result, err := client.VerifyOTP(ctx, resp.Token, "483920")
// result.Verified, result.Phone, result.AttemptsRemaining
```

### `client.Health(ctx)`

Instance-wide liveness check (no notion of a single connected phone number — an instance can host many projects, each with their own numbers).

```go
health, err := client.Health(ctx)
// health.Status, health.UptimeSeconds
```

### `client.SendText(ctx, phone, text)` · `client.SendMedia(ctx, phone, media)`

Send a text or media message. Both return `*MessageResponse` with `.MessageID` — a failed send comes back as an `error`, not a `Success` field.

### `client.GetChats(ctx)`

Lists the WhatsApp contacts visible to the project's connected numbers as `[]Chat`, each with `.JID` and `.Name`.

### `client.SetPresence(ctx, phone, state)`

Sets the typing indicator for a chat without sending a message. `state` is `wotp.PresenceTyping` or `wotp.PresencePaused`.

## Error Handling

The SDK returns typed errors for business failures:

```go
resp, err := client.SendOTP(ctx, "+212600000000")
if err != nil {
    if wotp.IsRateLimitError(err) {
        rlErr := err.(*wotp.RateLimitError)
        fmt.Printf("Rate limited, retry after %ds\n", rlErr.RetryAfter)
    } else if wotp.IsExpiredTokenError(err) {
        fmt.Println("Token expired — request a new OTP")
    } else if wotp.IsInvalidCodeError(err) {
        codeErr := err.(*wotp.InvalidCodeError)
        fmt.Printf("Wrong code, %d attempts left\n", codeErr.AttemptsRemaining)
    }
}
```

| Error Type | When |
|------------|------|
| `*RateLimitError` | Phone/IP exceeded rate limit (HTTP 429) |
| `*ExpiredTokenError` | Token has expired (HTTP 410 or `expired_token`) |
| `*InvalidCodeError` | Wrong OTP code (HTTP 400 + `invalid_code`) |
| `*WotpError` | Base type for all SDK errors |

## Auto-Retry

Transient errors (502, 503, 504, network errors) are retried with exponential backoff. Business errors are **never** retried.

## Testing

```bash
go test ./...
```

## Requirements

- Go 1.22+
- A running Wotp instance

## License

MIT — see [LICENSE](../../LICENSE) for details.
