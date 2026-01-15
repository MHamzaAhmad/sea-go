// Event streaming for SimpleEmailAPI SDK.
//
// This file provides real-time event streaming with typed callbacks.
// Events run in a background goroutine for non-blocking operation.
//
// Example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	client.OnReceive(ctx, emailapi.EventHandlers{
//	    OnDelivered: func(e *v1.EmailDeliveredEvent) {
//	        log.Println("Delivered to:", e.Recipients)
//	    },
//	    OnBounced: func(e *v1.EmailBouncedEvent) {
//	        log.Println("Bounced:", e.BounceType)
//	    },
//	    OnError: func(err error) {
//	        log.Println("Stream error:", err)
//	    },
//	})
package emailapi

import (
	"context"
	"log"
	"sync"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/emailapi/sdk-go/gen/v1"
)

// AckMode specifies how events are acknowledged.
type AckMode string

const (
	// AckModeAuto automatically acknowledges events after handler completes.
	AckModeAuto AckMode = "auto"

	// AckModeManual requires explicit acknowledgment.
	AckModeManual AckMode = "manual"
)

// Reconnection settings
const (
	initialDelay     = 1 * time.Second
	maxDelay         = 30 * time.Second
	backoffMultipler = 2

	// Ack batching settings
	ackBatchSize       = 10
	ackFlushIntervalMs = 1000 * time.Millisecond
)

// EventHandlers contains callbacks for different email events.
// Define only the callbacks you care about.
type EventHandlers struct {
	// OnSent is called when an email is accepted for delivery.
	OnSent func(*v1.EmailSentEvent)

	// OnDelivered is called when an email is successfully delivered.
	OnDelivered func(*v1.EmailDeliveredEvent)

	// OnBounced is called when an email bounces.
	OnBounced func(*v1.EmailBouncedEvent)

	// OnComplained is called when a recipient marks the email as spam.
	OnComplained func(*v1.EmailComplainedEvent)

	// OnRejected is called when SES rejects the email.
	OnRejected func(*v1.EmailRejectedEvent)

	// OnDelayed is called when email delivery is delayed.
	OnDelayed func(*v1.EmailDelayedEvent)

	// OnReplied is called when a reply is received.
	OnReplied func(*v1.EmailRepliedEvent)

	// OnFailed is called when email sending fails permanently.
	OnFailed func(*v1.EmailFailedEvent)

	// OnError is called when an error occurs in the stream.
	OnError func(error)

	// AckMode specifies acknowledgment mode. Default is AckModeAuto.
	AckMode AckMode

	// BatchSize is the number of events to buffer per batch. Default is 10.
	BatchSize int32
}

// eventStreamer manages a streaming connection with reconnection logic.
type eventStreamer struct {
	client   *Client
	handlers EventHandlers
	ctx      context.Context

	// Ack batching
	pendingAcks   []string
	pendingAcksMu sync.Mutex
	ackTimer      *time.Timer
}

// OnReceive starts streaming events with typed callbacks.
// Events run in a background goroutine for non-blocking operation.
// The stream automatically reconnects on disconnect with exponential backoff.
//
// Use the context to control the stream lifecycle:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	client.OnReceive(ctx, handlers)
//	// Later: stop streaming
//	cancel()
func (c *Client) OnReceive(ctx context.Context, handlers EventHandlers) {
	// Set defaults
	if handlers.AckMode == "" {
		handlers.AckMode = AckModeAuto
	}
	if handlers.BatchSize <= 0 {
		handlers.BatchSize = 10
	}

	streamer := &eventStreamer{
		client:      c,
		handlers:    handlers,
		ctx:         ctx,
		pendingAcks: make([]string, 0, ackBatchSize),
	}

	go streamer.run()
}

func (s *eventStreamer) run() {
	currentDelay := initialDelay

	for {
		select {
		case <-s.ctx.Done():
			s.flushAcks()
			return
		default:
		}

		err := s.stream()

		select {
		case <-s.ctx.Done():
			s.flushAcks()
			return
		default:
		}

		if err != nil && s.handlers.OnError != nil {
			s.handlers.OnError(err)
		}

		// Flush acks before sleeping
		s.flushAcks()

		// Exponential backoff
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(currentDelay):
		}

		currentDelay = time.Duration(float64(currentDelay) * backoffMultipler)
		if currentDelay > maxDelay {
			currentDelay = maxDelay
		}
	}
}

func (s *eventStreamer) stream() error {
	stream, err := s.client.Emails.StreamEvents(s.ctx, connect.NewRequest(&v1.StreamEventsRequest{
		EventTypes: []v1.EventType{}, // All events
		BatchSize:  s.handlers.BatchSize,
	}))
	if err != nil {
		return err
	}
	defer stream.Close()

	for stream.Receive() {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		event := stream.Msg()

		// Skip heartbeats
		if event.Type == v1.EventType_EVENT_TYPE_HEARTBEAT {
			continue
		}

		// Dispatch to appropriate handler
		s.dispatchEvent(event)

		// Auto-ack if enabled
		if s.handlers.AckMode == AckModeAuto && event.Id != "" {
			s.queueAck(event.Id)
		}
	}

	return stream.Err()
}

func (s *eventStreamer) dispatchEvent(event *v1.Event) {
	defer func() {
		if r := recover(); r != nil {
			if s.handlers.OnError != nil {
				s.handlers.OnError(&Error{
					Code:    ErrCodeInternal,
					Message: "panic in event handler",
				})
			}
			log.Printf("emailapi: panic in event handler: %v", r)
		}
	}()

	switch payload := event.Payload.(type) {
	case *v1.Event_EmailSent:
		if s.handlers.OnSent != nil {
			s.handlers.OnSent(payload.EmailSent)
		}
	case *v1.Event_EmailDelivered:
		if s.handlers.OnDelivered != nil {
			s.handlers.OnDelivered(payload.EmailDelivered)
		}
	case *v1.Event_EmailBounced:
		if s.handlers.OnBounced != nil {
			s.handlers.OnBounced(payload.EmailBounced)
		}
	case *v1.Event_EmailComplained:
		if s.handlers.OnComplained != nil {
			s.handlers.OnComplained(payload.EmailComplained)
		}
	case *v1.Event_EmailRejected:
		if s.handlers.OnRejected != nil {
			s.handlers.OnRejected(payload.EmailRejected)
		}
	case *v1.Event_EmailDelayed:
		if s.handlers.OnDelayed != nil {
			s.handlers.OnDelayed(payload.EmailDelayed)
		}
	case *v1.Event_EmailReplied:
		if s.handlers.OnReplied != nil {
			s.handlers.OnReplied(payload.EmailReplied)
		}
	case *v1.Event_EmailFailed:
		if s.handlers.OnFailed != nil {
			s.handlers.OnFailed(payload.EmailFailed)
		}
	}
}

func (s *eventStreamer) queueAck(eventID string) {
	s.pendingAcksMu.Lock()
	defer s.pendingAcksMu.Unlock()

	s.pendingAcks = append(s.pendingAcks, eventID)

	// Flush immediately if batch is full
	if len(s.pendingAcks) >= ackBatchSize {
		s.flushAcksLocked()
		return
	}

	// Schedule flush if not already scheduled
	if s.ackTimer == nil {
		s.ackTimer = time.AfterFunc(ackFlushIntervalMs, func() {
			s.pendingAcksMu.Lock()
			defer s.pendingAcksMu.Unlock()
			s.flushAcksLocked()
		})
	}
}

func (s *eventStreamer) flushAcks() {
	s.pendingAcksMu.Lock()
	defer s.pendingAcksMu.Unlock()
	s.flushAcksLocked()
}

func (s *eventStreamer) flushAcksLocked() {
	if len(s.pendingAcks) == 0 {
		return
	}

	// Stop timer if running
	if s.ackTimer != nil {
		s.ackTimer.Stop()
		s.ackTimer = nil
	}

	idsToAck := s.pendingAcks
	s.pendingAcks = make([]string, 0, ackBatchSize)

	// Fire and forget - server will replay unacked events on reconnect
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := s.client.Emails.AckEvents(ctx, connect.NewRequest(&v1.AckEventsRequest{
			EventIds: idsToAck,
		}))
		if err != nil {
			log.Printf("emailapi: failed to ack events: %v", err)
		}
	}()
}
