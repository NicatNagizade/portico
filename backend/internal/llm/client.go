package llm

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

var (
	ErrNotConfigured = errors.New("openai is not configured (set OPENAI_API_KEY)")
	ErrEmptyResponse = errors.New("empty openai response")
)

// Client talks to an OpenAI-compatible chat completions API.
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewClient(apiKey, baseURL, model string) *Client {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Completer is satisfied by *Client (and fakes in tests).
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

type chatRequest struct {
	Model          string        `json:"model"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
	Messages       []chatMessage `json:"messages"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	if c == nil || c.APIKey == "" {
		return "", ErrNotConfigured
	}
	body, err := json.Marshal(chatRequest{
		Model:       c.Model,
		Temperature: 0,
		ResponseFormat: &respFormat{Type: "json_object"},
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("openai decode: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("openai http %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", ErrEmptyResponse
	}
	return parsed.Choices[0].Message.Content, nil
}
