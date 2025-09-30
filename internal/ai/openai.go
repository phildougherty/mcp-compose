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
	openaiAPIURL     = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIModel = "gpt-4"
	defaultOpenAIMaxTokens = 4096
	defaultOpenAITemperature = 1.0
)

type OpenAIProvider struct {
	config     *OpenAIConfig
	httpClient *http.Client
	apiURL     string
}

func NewOpenAIProvider(config *OpenAIConfig) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, &ProviderError{
			Provider: "openai",
			Message:  "API key is required",
		}
	}

	if config.Model == "" {
		config.Model = defaultOpenAIModel
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaultOpenAIMaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaultOpenAITemperature
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 60 * time.Second
	}

	return &OpenAIProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		apiURL: openaiAPIURL,
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

type openaiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
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
		Provider: "openai",
		Message:  "failed after retries",
		Err:      lastErr,
	}
}

func (p *OpenAIProvider) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
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

func (p *OpenAIProvider) makeRequest(ctx context.Context, messages []Message, stream bool) (string, error) {
	reqBody := openaiRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "openai",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return "", &ProviderError{
			Provider: "openai",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: "openai",
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
			Provider: "openai",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", &ProviderError{
			Provider: "openai",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return "", &ProviderError{
			Provider: "openai",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	if openaiResp.Error != nil {
		return "", &ProviderError{
			Provider: "openai",
			Message:  openaiResp.Error.Message,
		}
	}

	if len(openaiResp.Choices) == 0 {
		return "", &ProviderError{
			Provider: "openai",
			Message:  "empty response",
		}
	}

	return openaiResp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) streamRequest(ctx context.Context, messages []Message, ch chan<- string) error {
	reqBody := openaiRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &ProviderError{
			Provider: "openai",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return &ProviderError{
			Provider: "openai",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: "openai",
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
			Provider: "openai",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &ProviderError{
			Provider: "openai",
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

		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Error != nil {
			return &ProviderError{
				Provider: "openai",
				Message:  chunk.Error.Message,
			}
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			select {
			case ch <- chunk.Choices[0].Delta.Content:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &ProviderError{
			Provider: "openai",
			Message:  "stream read error",
			Err:      err,
		}
	}

	return nil
}

func (p *OpenAIProvider) Health(ctx context.Context) error {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := p.makeRequest(ctx, messages, false)

	return err
}