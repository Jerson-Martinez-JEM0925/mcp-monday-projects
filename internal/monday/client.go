package monday

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
)

// Client is the transport boundary for monday.com's GraphQL API.
type Client struct {
	httpClient       *http.Client
	apiURL           string
	apiToken         string
	apiVersion       string
	maxResponseBytes int64
	maxRetries       int
}

// GraphQLRequest is the wire request sent to monday.com.
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQLError is a sanitized error returned by the GraphQL API.
type GraphQLError struct {
	Message string         `json:"message"`
	Path    []any          `json:"path,omitempty"`
	Extra   map[string]any `json:"extensions,omitempty"`
}

// GraphQLResponse contains raw data and API-level errors.
type GraphQLResponse struct {
	Data      json.RawMessage `json:"data"`
	Errors    []GraphQLError  `json:"errors,omitempty"`
	AccountID int64           `json:"account_id,omitempty"`
	// Legacy top-level error envelope still used by some monday failures.
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func newRequestError(gqlErr GraphQLError) *RequestError {
	reqErr := &RequestError{Message: gqlErr.Message}
	if code, ok := gqlErr.Extra["code"].(string); ok {
		reqErr.Code = code
	}
	if seconds, ok := gqlErr.Extra["retry_in_seconds"].(float64); ok && seconds > 0 {
		reqErr.RetryAfter = min(time.Duration(seconds)*time.Second, 30*time.Second)
	}
	if len(gqlErr.Path) > 0 {
		parts := make([]string, 0, len(gqlErr.Path))
		for _, part := range gqlErr.Path {
			parts = append(parts, fmt.Sprint(part))
		}
		reqErr.Path = strings.Join(parts, ".")
	}
	return reqErr
}

// NewClient creates a reusable Monday GraphQL client.
func NewClient(cfg config.Config) *Client {
	return &Client{
		httpClient:       &http.Client{Timeout: cfg.HTTPTimeout},
		apiURL:           cfg.APIURL,
		apiToken:         cfg.APIToken,
		apiVersion:       cfg.APIVersion,
		maxResponseBytes: cfg.MaxResponseBytes,
		maxRetries:       cfg.MaxRetries,
	}
}

// Do executes one GraphQL request and decodes the response into output.
func (c *Client) Do(ctx context.Context, query string, variables map[string]any, output any) error {
	if query == "" {
		return fmt.Errorf("graphql query cannot be empty")
	}
	payload, err := json.Marshal(GraphQLRequest{Query: query, Variables: compactVariables(variables)})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create monday request: %w", err)
		}
		req.Header.Set("Authorization", c.apiToken)
		req.Header.Set("API-Version", c.apiVersion)
		req.Header.Set("Content-Type", "application/json")

		response, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = fmt.Errorf("monday request failed: %w", err)
			if attempt < c.maxRetries {
				if err := waitForRetry(ctx, attempt, 0); err != nil {
					return err
				}
				continue
			}
			return lastErr
		}

		limited := io.LimitReader(response.Body, c.maxResponseBytes+1)
		body, readErr := io.ReadAll(limited)
		response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read monday response: %w", readErr)
		}
		if int64(len(body)) > c.maxResponseBytes {
			return fmt.Errorf("monday response exceeded configured limit")
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			apiErr := &APIError{
				StatusCode: response.StatusCode,
				Message:    http.StatusText(response.StatusCode),
				RetryAfter: retryAfter(response.Header.Get("Retry-After")),
				Temporary:  response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
			}
			if apiErr.Temporary && attempt < c.maxRetries {
				if err := waitForRetry(ctx, attempt, apiErr.RetryAfter); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}

		var envelope GraphQLResponse
		if err := json.Unmarshal(body, &envelope); err != nil {
			return fmt.Errorf("decode monday response: %w", err)
		}
		if len(envelope.Errors) > 0 {
			reqErr := newRequestError(envelope.Errors[0])
			if reqErr.Temporary() && attempt < c.maxRetries {
				lastErr = reqErr
				if err := waitForRetry(ctx, attempt, reqErr.RetryAfter); err != nil {
					return err
				}
				continue
			}
			return reqErr
		}
		if envelope.ErrorCode != "" {
			return &RequestError{Code: envelope.ErrorCode, Message: envelope.ErrorMessage}
		}
		if output == nil {
			return nil
		}
		if err := json.Unmarshal(envelope.Data, output); err != nil {
			return fmt.Errorf("decode monday data: %w", err)
		}
		return nil
	}
	return lastErr
}

// compactVariables drops nil variables. An omitted nullable variable means
// "argument not provided", while an explicit null is rejected by some monday
// resolvers (for example users(page: null) returns an internal error).
func compactVariables(variables map[string]any) map[string]any {
	if len(variables) == 0 {
		return nil
	}
	compact := make(map[string]any, len(variables))
	for key, value := range variables {
		if value == nil {
			continue
		}
		compact[key] = value
	}
	return compact
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0
	}
	return min(time.Duration(seconds)*time.Second, 30*time.Second)
}

func waitForRetry(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := retryAfter
	if delay == 0 {
		delay = min(250*time.Millisecond*time.Duration(1<<attempt), 5*time.Second)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Timeout returns the transport timeout for diagnostics and tests.
func (c *Client) Timeout() time.Duration {
	return c.httpClient.Timeout
}
