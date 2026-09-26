package monday

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// APIError describes a sanitized HTTP/API failure from monday.com.
type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
	Temporary  bool
}

func (e *APIError) Error() string {
	if e.StatusCode == 0 {
		return e.Message
	}
	return fmt.Sprintf("monday API returned HTTP %d: %s", e.StatusCode, e.Message)
}

// RequestError is a GraphQL-level error returned with HTTP 200. Code carries
// monday's extensions.code (for example ComplexityException or
// USER_UNAUTHORIZED) so callers can branch without parsing messages.
type RequestError struct {
	Code       string
	Message    string
	Path       string
	RetryAfter time.Duration
}

func (e *RequestError) Error() string {
	var parts []string
	parts = append(parts, "monday graphql request failed")
	if e.Code != "" {
		parts = append(parts, "["+e.Code+"]")
	}
	message := e.Message
	if e.Path != "" {
		message = e.Path + ": " + message
	}
	return strings.Join(parts, " ") + ": " + message
}

// Temporary reports whether retrying later can succeed.
func (e *RequestError) Temporary() bool {
	switch strings.ToUpper(e.Code) {
	case "COMPLEXITYEXCEPTION", "COMPLEXITY_BUDGET_EXHAUSTED", "RATE_LIMIT_EXCEEDED", "MAXCONCURRENCYEXCEEDED", "IP_RATE_LIMIT_EXCEEDED":
		return true
	}
	return false
}

// NotFoundError represents a missing Monday resource without exposing payloads.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return e.Resource + " not found: " + e.ID
}

// IsNotFound reports whether err wraps a NotFoundError.
func IsNotFound(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}
