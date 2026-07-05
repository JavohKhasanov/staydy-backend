// Package gemini is a thin REST client for Google's Generative Language API (Gemini).
// It deliberately avoids the heavyweight official SDK: one generateContent call is all we need.
package gemini

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

// ErrNotConfigured signals a missing API key — callers map it to a "feature unavailable" response
// rather than a hard error.
var ErrNotConfigured = errors.New("gemini: API key not configured")

// ErrBlocked means the model refused to answer (safety filter / no candidate).
var ErrBlocked = errors.New("gemini: response blocked or empty")

// ErrTruncated means the model hit the output-token budget before emitting any text — often the
// budget was consumed by the 2.5-family "thinking" tokens. It's a config issue, not a safety block.
var ErrTruncated = errors.New("gemini: response truncated (max output tokens)")

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Client calls the Gemini generateContent endpoint.
type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// New builds a client. baseURL is overridable for tests (pass "" for the real endpoint).
func New(apiKey, model, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Configured reports whether an API key is present.
func (c *Client) Configured() bool { return strings.TrimSpace(c.apiKey) != "" }

type genReq struct {
	SystemInstruction *content   `json:"systemInstruction,omitempty"`
	Contents          []content  `json:"contents"`
	GenerationConfig  *genConfig `json:"generationConfig,omitempty"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type genConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type genResp struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Generate sends a system instruction + user prompt and returns the model's text.
func (c *Client) Generate(ctx context.Context, system, prompt string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}

	body := genReq{
		Contents: []content{{Parts: []part{{Text: prompt}}}},
		// 2048 leaves headroom for the 3-section answer AND the 2.5-family "thinking" tokens,
		// which are charged against this budget (a too-low cap yields empty/truncated output).
		GenerationConfig: &genConfig{Temperature: 0.7, MaxOutputTokens: 2048},
	}
	if strings.TrimSpace(system) != "" {
		body.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	// API key is passed as a header (x-goog-api-key) so it never lands in URLs/logs.
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}

	var parsed genResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("gemini: bad response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := "unknown error"
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return "", fmt.Errorf("gemini: api error (status %d): %s", resp.StatusCode, msg)
	}
	if parsed.PromptFeedback.BlockReason != "" {
		return "", fmt.Errorf("%w: %s", ErrBlocked, parsed.PromptFeedback.BlockReason)
	}
	if len(parsed.Candidates) == 0 {
		return "", fmt.Errorf("%w: no candidates", ErrBlocked)
	}

	cand := parsed.Candidates[0]
	var sb strings.Builder
	for _, p := range cand.Content.Parts {
		sb.WriteString(p.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		// Separate a token-budget cutoff (fixable via config) from a real safety/content block,
		// so logs show the actual cause instead of a blanket "blocked or empty".
		if cand.FinishReason == "MAX_TOKENS" {
			return "", ErrTruncated
		}
		reason := cand.FinishReason
		if reason == "" {
			reason = "empty"
		}
		return "", fmt.Errorf("%w: finishReason=%s", ErrBlocked, reason)
	}
	return text, nil
}
