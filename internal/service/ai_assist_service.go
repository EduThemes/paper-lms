// Package service: AI Assist proxy to the Anthropic Messages API.
//
// AIAssistService wraps the Anthropic Claude API for three RCE V2 toolbar
// actions: Outline, Summarize, Rewrite. The frontend never sees the API key
// — the backend resolves it per-request via the Settings Engine, so a
// super-admin who rotates `ai.anthropic.api_key` through the
// /superadmin/settings UI takes effect on the very next call (no restart).
//
// Model: claude-haiku-4-5-20251001 (cheap + fast — see CLAUDE.md model table).
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
	aiAssistModel     = "claude-haiku-4-5-20251001"
	aiAssistMaxTokens = 1024
	aiAssistTimeout   = 15 * time.Second

	// aiAnthropicAPIKeyCatalogKey is the Settings Engine catalog key
	// resolved on every request. The catalog declares the env fallback
	// (ANTHROPIC_API_KEY) so deployments that never touch the
	// super-admin UI keep working unchanged.
	aiAnthropicAPIKeyCatalogKey = "ai.anthropic.api_key"
)

// HTTPDoer is the minimal interface AIAssistService needs from an HTTP client.
// It exists so tests can swap in a fake without spinning up a real server.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ErrAIAssistNotConfigured is returned when the resolved Anthropic API key
// is empty. Handlers translate this into HTTP 503 with the Canvas error
// format.
var ErrAIAssistNotConfigured = errors.New("AI Assist not configured")

// AIAssistService proxies a small set of authoring helpers to Anthropic.
//
// Wave 5: the API key is no longer held in a struct field at boot. Each
// public action resolves the catalog key `ai.anthropic.api_key` via the
// injected lookup function so cred rotation through the super-admin UI
// takes effect immediately. Empty resolved value -> ErrAIAssistNotConfigured.
type AIAssistService struct {
	lookup     SettingsLookupFunc
	httpClient HTTPDoer
}

// NewAIAssistService constructs a service. Pass a nil lookup to make the
// service report ErrAIAssistNotConfigured on every call (lets the server
// boot without the Settings Engine wired, and lets tests that don't
// exercise the network path skip the closure plumbing).
//
// The lookup signature mirrors the SMTP refactor in
// NotificationDeliveryService — see SettingsLookupFunc in
// notification_delivery_service.go for the cycle-breaking rationale.
func NewAIAssistService(lookup SettingsLookupFunc) *AIAssistService {
	return &AIAssistService{
		lookup: lookup,
		httpClient: &http.Client{
			Timeout: aiAssistTimeout + 5*time.Second, // context still bounds it tighter
		},
	}
}

// WithHTTPClient swaps the underlying HTTP client (used in tests).
func (s *AIAssistService) WithHTTPClient(c HTTPDoer) *AIAssistService {
	s.httpClient = c
	return s
}

// resolveAPIKey reads `ai.anthropic.api_key` via the injected lookup. A
// nil lookup or empty resolved value both yield ErrAIAssistNotConfigured;
// transient lookup failures bubble up unchanged.
func (s *AIAssistService) resolveAPIKey(ctx context.Context) (string, error) {
	if s.lookup == nil {
		return "", ErrAIAssistNotConfigured
	}
	v, err := s.lookup(ctx, aiAnthropicAPIKeyCatalogKey)
	if err != nil {
		return "", fmt.Errorf("settings %s: %w", aiAnthropicAPIKeyCatalogKey, err)
	}
	if strings.TrimSpace(v) == "" {
		return "", ErrAIAssistNotConfigured
	}
	return v, nil
}

// Configured reports whether the service has any chance of serving a
// request. Retained for the handler-level fast-path 503 so we don't burn
// a settings lookup on every COPPA-blocked request. Uses a background
// context because the call is a boolean probe, not a request-bound
// resolution — request-time errors still flow through ErrAIAssistNotConfigured
// in complete().
//
// Returns false only when no lookup is wired at all. The per-request
// resolveAPIKey is the source of truth for "is the key actually set"; we
// deliberately don't burn a settings round-trip here.
func (s *AIAssistService) Configured() bool {
	return s.lookup != nil
}

// --- Public actions -------------------------------------------------------

const outlineSystem = "You are an assistant that turns the user's text into a clear, hierarchical outline. " +
	"Output a Markdown bulleted outline only. Use nested bullets for sub-points. " +
	"Do not add commentary, headers, or anything outside the outline itself."

const summarizeSystem = "You are an assistant that writes concise summaries. " +
	"Summarize the user's text in 2-4 sentences, preserving the key facts and tone. " +
	"Output the summary text only, with no preamble."

func rewriteSystem(style string) string {
	style = strings.TrimSpace(style)
	if style == "" {
		style = "clearer"
	}
	return "You are an assistant that rewrites the user's text to be " + style + ". " +
		"Preserve the original meaning, voice, and any technical terms. " +
		"Output only the rewritten text — no commentary, no quotation marks, no preamble."
}

// Outline turns text into a Markdown bulleted outline.
func (s *AIAssistService) Outline(ctx context.Context, text string) (string, error) {
	return s.complete(ctx, outlineSystem, text)
}

// Summarize returns a short summary (2-4 sentences) of text.
func (s *AIAssistService) Summarize(ctx context.Context, text string) (string, error) {
	return s.complete(ctx, summarizeSystem, text)
}

// Rewrite returns text rewritten in the requested style (default "clearer").
func (s *AIAssistService) Rewrite(ctx context.Context, text, style string) (string, error) {
	return s.complete(ctx, rewriteSystem(style), text)
}

// --- HTTP plumbing --------------------------------------------------------

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// complete drives the Anthropic Messages API with one transparent retry on
// 429 / 5xx. Hard timeout is enforced via a derived context. Resolves the
// API key once per logical call (not per retry) — a rotation between
// attempts is rare enough that the slight staleness is acceptable.
func (s *AIAssistService) complete(ctx context.Context, system, userText string) (string, error) {
	if strings.TrimSpace(userText) == "" {
		return "", errors.New("text is required")
	}

	apiKey, err := s.resolveAPIKey(ctx)
	if err != nil {
		return "", err
	}

	body := anthropicRequest{
		Model:     aiAssistModel,
		MaxTokens: aiAssistMaxTokens,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: userText},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal anthropic request: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, aiAssistTimeout)
	defer cancel()

	// One retry on 429 / 5xx. Skip retry if the context is already done.
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		text, retry, err := s.doOnce(callCtx, apiKey, payload)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retry {
			return "", err
		}
		if callCtx.Err() != nil {
			return "", callCtx.Err()
		}
		// brief backoff between attempts
		select {
		case <-callCtx.Done():
			return "", callCtx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return "", lastErr
}

// doOnce performs a single HTTP round-trip. The bool return indicates whether
// the caller should retry (true on 429/5xx/transient transport errors).
func (s *AIAssistService) doOnce(ctx context.Context, apiKey string, payload []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Transport errors (incl. context deadline) — retry once.
		return "", true, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", true, fmt.Errorf("read anthropic response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", true, fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}
	if resp.StatusCode >= 400 {
		return "", false, fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", false, fmt.Errorf("decode anthropic response: %w", err)
	}
	if parsed.Error != nil {
		return "", false, fmt.Errorf("anthropic API error: %s", parsed.Error.Message)
	}

	var b strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", false, errors.New("anthropic API returned empty content")
	}
	return out, false, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
