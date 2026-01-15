# simpleemailapi-go

[![Go Reference](https://pkg.go.dev/badge/github.com/simpleemailapi/sdk-go.svg)](https://pkg.go.dev/github.com/simpleemailapi/sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

Go SDK for SimpleEmailAPI. Type-safe email sending with real-time event streaming.

## Installation

```bash
go get github.com/emailapi/sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    emailapi "github.com/emailapi/sdk-go"
    v1 "github.com/emailapi/sdk-go/gen/v1"
)

func main() {
    client := emailapi.NewClient("em_...")

    resp, err := client.Send(context.Background(), &v1.SendEmailRequest{
        From:    "hello@yourdomain.com",
        To:      []string{"user@example.com"},
        Subject: "Hello!",
        Body:    "World",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Email ID:", resp.Msg.Id)
}
```

## Real-time Event Streaming

Stream email events with typed callbacks:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

client.OnReceive(ctx, emailapi.EventHandlers{
    OnDelivered: func(e *v1.EmailDeliveredEvent) {
        log.Println("Delivered to:", e.Recipients)
    },
    OnReplied: func(e *v1.EmailRepliedEvent) {
        log.Println("Reply from:", e.From)
    },
    OnBounced: func(e *v1.EmailBouncedEvent) {
        log.Println("Bounced:", e.BounceType, e.Recipients)
    },
    OnError: func(err error) {
        log.Println("Stream error:", err)
    },
})
```

## Error Handling

```go
resp, err := client.Send(ctx, req)
if err != nil {
    if e := emailapi.ParseError(err); e != nil {
        switch {
        case e.Is(emailapi.ErrCodeDomainNotVerified):
            log.Println("Please verify your domain first")
        case e.IsCategory(emailapi.CategoryValidation):
            log.Println("Validation error on field:", e.Field)
        case e.IsCategory(emailapi.CategoryRateLimit):
            log.Println("Rate limited, retry later")
        }
    }
}
```

## Available Event Handlers

| Handler | Description |
|---------|-------------|
| `OnSent` | Email accepted for delivery |
| `OnDelivered` | Email delivered to recipient's mailbox |
| `OnBounced` | Email bounced (hard or soft) |
| `OnComplained` | Recipient marked email as spam |
| `OnRejected` | Email rejected before sending |
| `OnDelayed` | Email delivery delayed |
| `OnReplied` | Reply received to a sent email |
| `OnFailed` | Email sending failed permanently |
| `OnError` | Stream error occurred |

## Service Clients

Access underlying service clients for advanced usage:

```go
// Email operations
resp, err := client.Emails.SendEmail(ctx, connect.NewRequest(&v1.SendEmailRequest{...}))

// Domain management
domains, err := client.Domains.ListDomains(ctx, connect.NewRequest(&v1.ListDomainsRequest{}))
```

## Requirements

- Go 1.21+

## Documentation

Visit [simpleemailapi.dev](https://simpleemailapi.dev) for full API documentation.

## License

MIT
