// Error handling for SimpleEmailAPI SDK.
//
// This file provides structured error handling with typed error codes
// for programmatic error handling.
//
// Example:
//
//	resp, err := client.Send(ctx, req)
//	if err != nil {
//	    if e := emailapi.ParseError(err); e != nil {
//	        if e.Is(emailapi.ErrCodeDomainNotVerified) {
//	            fmt.Println("Please verify your domain first")
//	        }
//	        if e.IsCategory(CategoryValidation) {
//	            fmt.Println("Validation error on field:", e.Field)
//	        }
//	    }
//	}
package emailapi

import (
	"errors"

	"connectrpc.com/connect"
)

// ErrorCode represents specific API error codes.
// Ranges: 1xx=auth, 2xx=authz, 3xx=validation, 4xx=resources,
// 5xx=domain, 6xx=rate/usage, 7xx=attachments, 8xx=tokens, 9xx=internal
type ErrorCode int32

// Error codes matching the proto definition.
const (
	ErrCodeUnspecified ErrorCode = 0

	// Authentication (1xx)
	ErrCodeUnauthenticated   ErrorCode = 100
	ErrCodeInvalidAPIKey     ErrorCode = 101
	ErrCodeExpiredAPIKey     ErrorCode = 102
	ErrCodeClerkTokenInvalid ErrorCode = 104

	// Authorization (2xx)
	ErrCodePermissionDenied  ErrorCode = 200
	ErrCodeAdminRequired     ErrorCode = 201
	ErrCodeAccountSuspended  ErrorCode = 202
	ErrCodeInsufficientScope ErrorCode = 203

	// Validation (3xx)
	ErrCodeInvalidArgument      ErrorCode = 300
	ErrCodeMissingRequiredField ErrorCode = 301
	ErrCodeInvalidEmailSyntax   ErrorCode = 302
	ErrCodeNoMXRecords          ErrorCode = 303
	ErrCodeEmailTypoDetected    ErrorCode = 304
	ErrCodeUnsafeURL            ErrorCode = 305
	ErrCodeEmailSuppressed      ErrorCode = 306
	ErrCodeSandboxRestriction   ErrorCode = 307

	// Resource Not Found (4xx)
	ErrCodeNotFound            ErrorCode = 400
	ErrCodeDomainNotFound      ErrorCode = 401
	ErrCodeAPIKeyNotFound      ErrorCode = 402
	ErrCodeUserNotFound        ErrorCode = 403
	ErrCodeAlreadyExists       ErrorCode = 410
	ErrCodeDomainAlreadyExists ErrorCode = 411

	// Domain Verification (5xx)
	ErrCodeDomainNotOwned       ErrorCode = 500
	ErrCodeDomainNotVerified    ErrorCode = 501
	ErrCodeDomainDNSMismatch    ErrorCode = 502
	ErrCodeDomainVerifyCooldown ErrorCode = 503

	// Rate Limiting & Usage (6xx)
	ErrCodeRateLimited             ErrorCode = 600
	ErrCodeDailyLimitExceeded      ErrorCode = 601
	ErrCodeMonthlyCreditsExhausted ErrorCode = 602
	ErrCodeMaxConcurrentStreams    ErrorCode = 603

	// Attachment Errors (7xx)
	ErrCodeAttachmentTooLarge     ErrorCode = 700
	ErrCodeTotalSizeExceeded      ErrorCode = 701
	ErrCodeAttachmentThreatFound  ErrorCode = 702
	ErrCodeUnsupportedContentType ErrorCode = 703

	// Token Errors (8xx)
	ErrCodeInvalidToken         ErrorCode = 800
	ErrCodeExpiredToken         ErrorCode = 801
	ErrCodeWebhookSecretInvalid ErrorCode = 810

	// Internal Errors (9xx)
	ErrCodeInternal              ErrorCode = 900
	ErrCodeUpstreamProviderError ErrorCode = 901
	ErrCodeServiceUnavailable    ErrorCode = 902
)

// Error category constants for IsCategory checks.
const (
	CategoryAuth       = "auth"
	CategoryAuthz      = "authz"
	CategoryValidation = "validation"
	CategoryNotFound   = "notfound"
	CategoryDomain     = "domain"
	CategoryRateLimit  = "ratelimit"
	CategoryInternal   = "internal"
)

// Error is a structured error from the SimpleEmailAPI.
type Error struct {
	// Code is the specific error code from the API.
	Code ErrorCode

	// Message is the human-readable error message.
	Message string

	// Field is the field that caused the error (for validation errors).
	Field string

	// Metadata contains additional context (e.g., limits, upgrade URLs).
	Metadata map[string]string

	// ConnectCode is the underlying Connect/gRPC status code.
	ConnectCode connect.Code
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Is checks if this error matches a specific error code.
func (e *Error) Is(code ErrorCode) bool {
	return e.Code == code
}

// IsCategory checks if this error is in a specific category.
func (e *Error) IsCategory(category string) bool {
	c := int32(e.Code)
	switch category {
	case CategoryAuth:
		return c >= 100 && c < 200
	case CategoryAuthz:
		return c >= 200 && c < 300
	case CategoryValidation:
		return c >= 300 && c < 400
	case CategoryNotFound:
		return c >= 400 && c < 410
	case CategoryDomain:
		return c >= 500 && c < 600
	case CategoryRateLimit:
		return c >= 600 && c < 700
	case CategoryInternal:
		return c >= 900
	default:
		return false
	}
}

// ParseError extracts a structured Error from any error.
// Returns nil if the error is not a Connect error.
func ParseError(err error) *Error {
	if err == nil {
		return nil
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		// Not a Connect error, wrap as unknown
		return &Error{
			Code:        ErrCodeUnspecified,
			Message:     err.Error(),
			ConnectCode: connect.CodeUnknown,
		}
	}

	// Create base error from Connect error
	e := &Error{
		Code:        ErrCodeUnspecified,
		Message:     connectErr.Message(),
		ConnectCode: connectErr.Code(),
		Metadata:    make(map[string]string),
	}

	// Try to extract ErrorDetail from error details
	for _, detail := range connectErr.Details() {
		// Check if this is our ErrorDetail type
		// The type URL format is: type.googleapis.com/v1.ErrorDetail
		if detail.Type() == "v1.ErrorDetail" {
			// Attempt to extract the proto message
			msg, extractErr := detail.Value()
			if extractErr != nil {
				continue
			}

			// The msg is a proto.Message, we need to handle it
			// Since we can't import the generated types here without circular deps,
			// we parse the detail manually from JSON representation
			if protoMsg, ok := msg.(interface {
				GetCode() int32
				GetMessage() string
				GetField() string
				GetMetadata() map[string]string
			}); ok {
				e.Code = ErrorCode(protoMsg.GetCode())
				if m := protoMsg.GetMessage(); m != "" {
					e.Message = m
				}
				e.Field = protoMsg.GetField()
				if meta := protoMsg.GetMetadata(); meta != nil {
					e.Metadata = meta
				}
			}
			break
		}
	}

	return e
}

// IsError checks if the given error is a SimpleEmailAPI Error.
func IsError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}
