package ai

import (
	"bufio"
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
	claudeAPIURL     = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion = "2023-06-01"
	defaultClaudeModel = "claude-3-5-sonnet-20241022"
	defaultClaudeMaxTokens = 4096
	defaultClaudeTemperature = 1.0
	maxRetries = 3
	initialBackoff = 500 * time.Millisecond
)

type ClaudeProvider struct {
	config     *ClaudeConfig
	httpClient *http.Client
	apiURL     string
}

func NewClaudeProvider(config *ClaudeConfig) (*ClaudeProvider, error) {
	if config.APIKey == "" {
		return nil, &ProviderError{
			Provider: "claude",
			Message:  "API key is required",
		}
	}

	if config.Model == "" {
		config.Model = defaultClaudeModel
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaultClaudeMaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaultClaudeTemperature
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 60 * time.Second
	}

	return &ClaudeProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		apiURL: claudeAPIURL,
	}, nil
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

type claudeRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
	Stream      bool             `json:"stream"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type claudeStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	Message *claudeResponse `json:"message,omitempty"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *ClaudeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err := p.makeRequest(ctx, messages, false)
		if err == nil {
			return response, nil
		}

		lastErr = err

		if !isRetryable(err) {
			break
		}
	}

	return "", &ProviderError{
		Provider: "claude",
		Message:  "failed after retries",
		Err:      lastErr,
	}
}

func (p *ClaudeProvider) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
	ch := make(chan string, 100)

	go func() {
		defer close(ch)

		var lastErr error

		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
			}

			err := p.streamRequest(ctx, messages, ch)
			if err == nil {
				return
			}

			lastErr = err

			if !isRetryable(err) {
				break
			}
		}

		if lastErr != nil {
			select {
			case ch <- fmt.Sprintf("ERROR: %v", lastErr):
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

func (p *ClaudeProvider) makeRequest(ctx context.Context, messages []Message, stream bool) (string, error) {
	reqBody := claudeRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "claude",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return "", &ProviderError{
			Provider: "claude",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: "claude",
			Message:  "request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if duration, err := time.ParseDuration(retryAfter + "s"); err == nil {
				time.Sleep(duration)
			}
		}

		return "", &ProviderError{
			Provider: "claude",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", &ProviderError{
			Provider: "claude",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var claudeResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", &ProviderError{
			Provider: "claude",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	if claudeResp.Error != nil {
		return "", &ProviderError{
			Provider: "claude",
			Message:  claudeResp.Error.Message,
		}
	}

	if len(claudeResp.Content) == 0 {
		return "", &ProviderError{
			Provider: "claude",
			Message:  "empty response",
		}
	}

	return claudeResp.Content[0].Text, nil
}

func (p *ClaudeProvider) streamRequest(ctx context.Context, messages []Message, ch chan<- string) error {
	reqBody := claudeRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &ProviderError{
			Provider: "claude",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return &ProviderError{
			Provider: "claude",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: "claude",
			Message:  "request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			if duration, err := time.ParseDuration(retryAfter + "s"); err == nil {
				time.Sleep(duration)
			}
		}

		return &ProviderError{
			Provider: "claude",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &ProviderError{
			Provider: "claude",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event claudeStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Error != nil {
			return &ProviderError{
				Provider: "claude",
				Message:  event.Error.Message,
			}
		}

		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Text != "" {
			select {
			case ch <- event.Delta.Text:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &ProviderError{
			Provider: "claude",
			Message:  "stream read error",
			Err:      err,
		}
	}

	return nil
}

func (p *ClaudeProvider) Health(ctx context.Context) error {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := p.makeRequest(ctx, messages, false)

	return err
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	provErr, ok := err.(*ProviderError)
	if !ok {
		return false
	}

	return strings.Contains(provErr.Message, "rate limit") ||
		strings.Contains(provErr.Message, "timeout") ||
		strings.Contains(provErr.Message, "connection")
}