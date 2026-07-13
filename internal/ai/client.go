// Package ai is the hand-rolled Anthropic Messages API client and the
// batch content-generation pipeline built on top of it (word problems,
// logic puzzles, saga stories). It never grades or writes game state --
// see ARCHITECTURE.md "AI content generation".
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	messagesURL      = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"

	// DefaultModel is used when AI_MODEL is unset.
	DefaultModel     = "claude-sonnet-5"
	DefaultMaxTokens = 8192

	RoleUser = "user"
)

// Client is a minimal Anthropic Messages API client -- no SDK, matching the
// food/finance/journal pattern. Generation here is always a single
// non-agentic text-in/text-out call, so unlike the food client this one
// carries no tool-use or image scaffolding.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient returns a disabled client when apiKey is empty; callers check
// Enabled() before calling CreateMessage.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		model:      model,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

func (c *Client) Model() string {
	return c.model
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Response struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
	Error      *apiError      `json:"error,omitempty"`
}

// Text concatenates every text content block in the response.
func (r *Response) Text() string {
	var sb strings.Builder
	for _, b := range r.Content {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// Messenger is the interface generation code depends on, so tests can
// script responses without any HTTP transport.
type Messenger interface {
	CreateMessage(ctx context.Context, req Request) (*Response, error)
}

// CreateMessage sends one Messages API call, retrying on transient
// overloaded/server-error responses.
func (c *Client) CreateMessage(ctx context.Context, req Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = DefaultMaxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	const maxRetries = 4
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create anthropic request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicVersion)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("anthropic request: %w", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read anthropic response: %w", err)
		}

		if (resp.StatusCode == 529 || resp.StatusCode == http.StatusServiceUnavailable) && attempt < maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, string(respBody))
		}

		var out Response
		if err := json.Unmarshal(respBody, &out); err != nil {
			return nil, fmt.Errorf("unmarshal anthropic response: %w", err)
		}
		if out.Error != nil {
			return nil, fmt.Errorf("anthropic error: %s", out.Error.Message)
		}
		return &out, nil
	}

	return nil, fmt.Errorf("anthropic API overloaded after %d retries", maxRetries)
}
