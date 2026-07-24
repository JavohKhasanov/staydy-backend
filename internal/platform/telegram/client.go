// Package telegram is a thin REST client for the Telegram Bot API — only the handful of methods
// the retention bot needs (getMe, long-poll getUpdates, sendMessage, answerCallbackQuery).
package telegram

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

// ErrNotConfigured signals a missing bot token.
var ErrNotConfigured = errors.New("telegram: bot token not configured")

const defaultBaseURL = "https://api.telegram.org"

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// New builds a client. baseURL is overridable for tests ("" → the real endpoint). The HTTP
// timeout must exceed the long-poll timeout (getUpdates blocks up to `timeout` seconds).
func New(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{token: token, baseURL: baseURL, http: &http.Client{Timeout: 65 * time.Second}}
}

func (c *Client) Configured() bool { return strings.TrimSpace(c.token) != "" }

// --- API types (only the fields we use) ---

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type WebAppInfo struct {
	URL string `json:"url"`
}

type InlineKeyboardButton struct {
	Text         string      `json:"text"`
	CallbackData string      `json:"callback_data,omitempty"`
	WebApp       *WebAppInfo `json:"web_app,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type BotInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// call POSTs a JSON payload to a Bot API method and unmarshals result into out (out may be nil).
func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %s request failed: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("telegram: %s bad response (status %d): %w", method, resp.StatusCode, err)
	}
	if !env.OK {
		return fmt.Errorf("telegram: %s error: %s", method, env.Description)
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

func (c *Client) GetMe(ctx context.Context) (BotInfo, error) {
	var info BotInfo
	err := c.call(ctx, "getMe", nil, &info)
	return info, err
}

// GetUpdates long-polls: it blocks up to timeoutSec for new updates after `offset`.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)
	return updates, err
}

// SendMessage sends Markdown text, optionally with an inline keyboard.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	return c.call(ctx, "sendMessage", payload, nil)
}

// AnswerCallback acknowledges a button tap (stops the client's loading spinner).
func (c *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": callbackID,
		"text":              text,
	}, nil)
}

// DeleteWebhook removes any configured webhook so long polling can run (the two are mutually
// exclusive in the Bot API).
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}
