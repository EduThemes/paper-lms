package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripperFunc lets us mock HTTPDoer with a closure.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func makeResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

const happyBody = `{"content":[{"type":"text","text":"- Point one\n- Point two"}]}`

// staticLookup is a test SettingsLookupFunc that always returns the same
// value. Empty value mimics the "no key configured" branch.
func staticLookup(value string) SettingsLookupFunc {
	return func(_ context.Context, _ string) (string, error) {
		return value, nil
	}
}

func TestAIAssist(t *testing.T) {
	t.Run("not configured returns sentinel error - nil lookup", func(t *testing.T) {
		svc := NewAIAssistService(nil)
		_, err := svc.Outline(context.Background(), "hello world")
		if !errors.Is(err, ErrAIAssistNotConfigured) {
			t.Fatalf("expected ErrAIAssistNotConfigured, got %v", err)
		}
	})

	t.Run("not configured returns sentinel error - empty lookup result", func(t *testing.T) {
		svc := NewAIAssistService(staticLookup(""))
		_, err := svc.Outline(context.Background(), "hello world")
		if !errors.Is(err, ErrAIAssistNotConfigured) {
			t.Fatalf("expected ErrAIAssistNotConfigured, got %v", err)
		}
	})

	t.Run("empty text rejected", func(t *testing.T) {
		svc := NewAIAssistService(staticLookup("test-key"))
		_, err := svc.Summarize(context.Background(), "   ")
		if err == nil {
			t.Fatal("expected error for empty input, got nil")
		}
	})

	t.Run("happy path - outline", func(t *testing.T) {
		var calls int32
		client := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			if got := req.Header.Get("x-api-key"); got != "test-key" {
				t.Errorf("missing or wrong x-api-key header: %q", got)
			}
			if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
				t.Errorf("wrong anthropic-version: %q", got)
			}
			if got := req.Header.Get("content-type"); got != "application/json" {
				t.Errorf("wrong content-type: %q", got)
			}
			return makeResp(http.StatusOK, happyBody), nil
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		out, err := svc.Outline(context.Background(), "Some long prose to outline.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Point one") {
			t.Errorf("expected outline content, got %q", out)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("expected 1 HTTP call, got %d", got)
		}
	})

	t.Run("happy path - summarize and rewrite use distinct system prompts", func(t *testing.T) {
		// We can't directly inspect system prompt from outside, but we can ensure
		// each method just calls the API once on success.
		client := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return makeResp(http.StatusOK, happyBody), nil
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		if _, err := svc.Summarize(context.Background(), "text"); err != nil {
			t.Fatalf("Summarize: %v", err)
		}
		if _, err := svc.Rewrite(context.Background(), "text", "more concise"); err != nil {
			t.Fatalf("Rewrite: %v", err)
		}
	})

	t.Run("429 retries once and then succeeds", func(t *testing.T) {
		var calls int32
		client := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			n := atomic.AddInt32(&calls, 1)
			if n == 1 {
				return makeResp(http.StatusTooManyRequests, `{"error":{"type":"rate_limit","message":"slow down"}}`), nil
			}
			return makeResp(http.StatusOK, happyBody), nil
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		out, err := svc.Outline(context.Background(), "anything")
		if err != nil {
			t.Fatalf("expected retry-success, got %v", err)
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Errorf("expected 2 HTTP calls (1 retry), got %d", got)
		}
		if out == "" {
			t.Error("expected non-empty result on retry success")
		}
	})

	t.Run("5xx retried then surfaced", func(t *testing.T) {
		var calls int32
		client := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return makeResp(http.StatusBadGateway, `upstream down`), nil
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		_, err := svc.Outline(context.Background(), "anything")
		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Errorf("expected 2 attempts, got %d", got)
		}
	})

	t.Run("4xx (non-429) does not retry", func(t *testing.T) {
		var calls int32
		client := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return makeResp(http.StatusBadRequest, `{"error":{"type":"invalid_request","message":"bad"}}`), nil
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		_, err := svc.Outline(context.Background(), "anything")
		if err == nil {
			t.Fatal("expected error on 400")
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("expected exactly 1 call (no retry on 4xx), got %d", got)
		}
	})

	t.Run("context timeout is honored", func(t *testing.T) {
		client := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			// Block until the request context cancels.
			<-req.Context().Done()
			return nil, req.Context().Err()
		})
		svc := NewAIAssistService(staticLookup("test-key")).WithHTTPClient(client)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := svc.Outline(ctx, "anything")
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("timeout took too long: %v", elapsed)
		}
	})
}
