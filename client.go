// Package emailapi provides a Go SDK for SimpleEmailAPI.
//
// This SDK provides type-safe access to the SimpleEmailAPI service
// using Connect RPC for HTTP/2 communication.
//
// # Quick Start
//
//	client := emailapi.NewClient("em_...")
//
//	// Send an email
//	resp, err := client.Send(ctx, &emailapi.SendEmailRequest{
//	    From:    "hello@yourdomain.com",
//	    To:      []string{"user@example.com"},
//	    Subject: "Hello!",
//	    Body:    "World",
//	})
//
// # Error Handling
//
//	resp, err := client.Send(ctx, req)
//	if err != nil {
//	    if e := emailapi.ParseError(err); e != nil {
//	        if e.Is(emailapi.ErrCodeDomainNotVerified) {
//	            // Handle unverified domain
//	        }
//	    }
//	}
//
// # Event Streaming
//
//	cancel := client.OnReceive(ctx, emailapi.EventHandlers{
//	    OnDelivered: func(e *v1.EmailDeliveredEvent) {
//	        log.Println("Delivered to:", e.Recipients)
//	    },
//	    OnBounced: func(e *v1.EmailBouncedEvent) {
//	        log.Println("Bounced:", e.BounceType)
//	    },
//	})
//	defer cancel()
package emailapi

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	v1 "github.com/emailapi/sdk-go/gen/v1"
	"github.com/emailapi/sdk-go/gen/v1/v1connect"
)

const (
	// DefaultBaseURL is the default API endpoint.
	DefaultBaseURL = "https://api.simpleemailapi.dev"
)

// Client is the SimpleEmailAPI client.
type Client struct {
	// Emails provides access to email operations.
	Emails v1connect.EmailServiceClient

	// Domains provides access to domain management operations.
	Domains v1connect.DomainServiceClient

	// Internal state
	apiKey  string
	baseURL string
	http    connect.HTTPClient
}

// ClientOption configures the client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client connect.HTTPClient) ClientOption {
	return func(c *Client) {
		c.http = client
	}
}

// NewClient creates a new SimpleEmailAPI client.
//
// Example:
//
//	client := emailapi.NewClient("em_...")
//
//	resp, err := client.Send(ctx, &v1.SendEmailRequest{
//	    From:    "sender@example.com",
//	    To:      []string{"recipient@example.com"},
//	    Subject: "Hello",
//	    Body:    "World",
//	})
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		http:    http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Create auth interceptor
	authInterceptor := connect.UnaryInterceptorFunc(
		func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+c.apiKey)
				return next(ctx, req)
			}
		},
	)

	clientOpts := []connect.ClientOption{
		connect.WithInterceptors(authInterceptor),
	}

	c.Emails = v1connect.NewEmailServiceClient(c.http, c.baseURL, clientOpts...)
	c.Domains = v1connect.NewDomainServiceClient(c.http, c.baseURL, clientOpts...)

	return c
}

// Send sends an email. This is a convenience wrapper around Emails.SendEmail.
//
// Example:
//
//	resp, err := client.Send(ctx, &v1.SendEmailRequest{
//	    From:    "hello@example.com",
//	    To:      []string{"user@example.com"},
//	    Subject: "Hello!",
//	    Body:    "World",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Email ID:", resp.Msg.Id)
func (c *Client) Send(ctx context.Context, req *v1.SendEmailRequest) (*connect.Response[v1.SendEmailResponse], error) {
	return c.Emails.SendEmail(ctx, connect.NewRequest(req))
}
