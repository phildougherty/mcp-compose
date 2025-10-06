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
	defaultOllamaBaseURL = "http://localhost:11434"
	defaultOllamaModel   = "llama2"
)

type OllamaProvider struct {
	config     *OllamaConfig
	httpClient *http.Client
}

func NewOllamaProvider(config *OllamaConfig) (*OllamaProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = defaultOllamaBaseURL
	}

	if !strings.HasPrefix(config.BaseURL, "http://") && !strings.HasPrefix(config.BaseURL, "https://") {
		config.BaseURL = "http://" + config.BaseURL
	}

	if config.Model == "" {
		config.Model = defaultOllamaModel
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 120 * time.Second
	}

	return &OllamaProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}, nil
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	Error     string        `json:"error,omitempty"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		ModifiedAt string `json:"modified_at"`
		Size       int64  `json:"size"`
	} `json:"models"`
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
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
		Provider: "ollama",
		Message:  "failed after retries",
		Err:      lastErr,
	}
}

func (p *OllamaProvider) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
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

func (p *OllamaProvider) makeRequest(ctx context.Context, messages []Message, stream bool) (string, error) {
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := ollamaRequest{
		Model:    p.config.Model,
		Messages: ollamaMessages,
		Stream:   stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "ollama",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	url := p.config.BaseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", &ProviderError{
			Provider: "ollama",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: "ollama",
			Message:  "request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", &ProviderError{
			Provider: "ollama",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", &ProviderError{
			Provider: "ollama",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	if ollamaResp.Error != "" {
		return "", &ProviderError{
			Provider: "ollama",
			Message:  ollamaResp.Error,
		}
	}

	return ollamaResp.Message.Content, nil
}

func (p *OllamaProvider) streamRequest(ctx context.Context, messages []Message, ch chan<- string) error {
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := ollamaRequest{
		Model:    p.config.Model,
		Messages: ollamaMessages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	url := p.config.BaseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &ProviderError{
			Provider: "ollama",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		var chunk ollamaResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		if chunk.Error != "" {
			return &ProviderError{
				Provider: "ollama",
				Message:  chunk.Error,
			}
		}

		if chunk.Message.Content != "" {
			select {
			case ch <- chunk.Message.Content:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "stream read error",
			Err:      err,
		}
	}

	return nil
}

func (p *OllamaProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	response, err := p.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Content: []ContentBlock{
			TextBlock{Type: "text", Text: response},
		},
		TextContent: response,
	}, nil
}

func (p *OllamaProvider) StreamWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan *ChatResponse, error) {
	ch := make(chan *ChatResponse, 100)

	go func() {
		defer close(ch)

		streamCh, err := p.Stream(ctx, messages)
		if err != nil {
			ch <- &ChatResponse{
				TextContent: fmt.Sprintf("ERROR: %v", err),
			}
			return
		}

		var fullText strings.Builder
		for chunk := range streamCh {
			fullText.WriteString(chunk)
		}

		ch <- &ChatResponse{
			Content: []ContentBlock{
				TextBlock{Type: "text", Text: fullText.String()},
			},
			TextContent: fullText.String(),
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) Health(ctx context.Context) error {
	url := p.config.BaseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "failed to create health check request",
			Err:      err,
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "health check failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ProviderError{
			Provider: "ollama",
			Message:  fmt.Sprintf("health check failed with status %d", resp.StatusCode),
		}
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return &ProviderError{
			Provider: "ollama",
			Message:  "failed to decode health check response",
			Err:      err,
		}
	}

	modelFound := false
	for _, model := range tags.Models {
		if model.Name == p.config.Model {
			modelFound = true

			break
		}
	}

	if !modelFound {
		return &ProviderError{
			Provider: "ollama",
			Message:  fmt.Sprintf("model %s not found", p.config.Model),
		}
	}

	return nil
}

func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	url := p.config.BaseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &ProviderError{
			Provider: "ollama",
			Message:  "failed to create list models request",
			Err:      err,
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: "ollama",
			Message:  "list models request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, &ProviderError{
			Provider: "ollama",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, &ProviderError{
			Provider: "ollama",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	models := make([]string, len(tags.Models))
	for i, model := range tags.Models {
		models[i] = model.Name
	}

	return models, nil
}