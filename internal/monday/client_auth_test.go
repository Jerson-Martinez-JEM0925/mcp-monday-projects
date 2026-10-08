package monday

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
)

func TestRequestTokensStayIsolatedConcurrently(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		token := request.Header.Get("Authorization")
		mu.Lock()
		seen[token]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"token": token}})
	}))
	t.Cleanup(server.Close)

	client := NewClient(config.Config{
		APIVersion: "2026-07", APIURL: server.URL, HTTPTimeout: time.Second,
		MaxResponseBytes: 1 << 20, AuthMode: "request", APIToken: "environment-token",
	})
	var wg sync.WaitGroup
	for _, token := range []string{"monday_user_a", "monday_user_b"} {
		token := token
		wg.Add(1)
		go func() {
			defer wg.Done()
			var output map[string]any
			if err := client.Do(WithRequestToken(context.Background(), token), "query", nil, &output); err != nil {
				t.Errorf("Do(%s): %v", token, err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if seen["monday_user_a"] != 1 || seen["monday_user_b"] != 1 || seen["environment-token"] != 0 {
		t.Fatalf("authorization headers = %+v", seen)
	}
}

func TestRequestAuthFailsClosedWithoutContextToken(t *testing.T) {
	client := NewClient(config.Config{
		APIToken: "environment-token", AuthMode: "request", HTTPTimeout: time.Second,
	})
	if err := client.Do(context.Background(), "query", nil, nil); err == nil || err.Error() != "monday request requires a bearer credential" {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestRequestErrorRedactsContextToken(t *testing.T) {
	const token = "monday_secret_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"upstream echoed monday_secret_token"}]}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(config.Config{
		APIVersion: "2026-07", APIURL: server.URL, HTTPTimeout: time.Second,
		MaxResponseBytes: 1 << 20, AuthMode: "request",
	})
	err := client.Do(WithRequestToken(context.Background(), token), "query", nil, nil)
	if err == nil || err.Error() == "" {
		t.Fatal("Do() error = nil, want sanitized GraphQL error")
	}
	if contains := err.Error(); contains == token || !strings.Contains(contains, "[REDACTED]") {
		t.Fatalf("error = %q, want token redacted", contains)
	}
}
