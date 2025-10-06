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
	"sync"
	"time"
)

const (
	openrouterAPIURL     = "https://openrouter.ai/api/v1/chat/completions"
	defaultOpenRouterModel = "anthropic/claude-3-5-sonnet"
	defaultOpenRouterMaxTokens = 4096
	defaultOpenRouterTemperature = 1.0
)

type OpenRouterProvider struct {
	config     *OpenRouterConfig
	httpClient *http.Client
	apiURL     string
	costMu     sync.RWMutex
	totalCost  float64
}

func NewOpenRouterProvider(config *OpenRouterConfig) (*OpenRouterProvider, error) {
	if config.APIKey == "" {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "API key is required",
		}
	}

	if config.Model == "" {
		config.Model = defaultOpenRouterModel
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaultOpenRouterMaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaultOpenRouterTemperature
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 60 * time.Second
	}
	if config.SiteURL == "" {
		config.SiteURL = "https://github.com/phildougherty/mcp-compose"
	}
	if config.AppName == "" {
		config.AppName = "mcp-compose"
	}

	return &OpenRouterProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		apiURL: openrouterAPIURL,
	}, nil
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

type openrouterRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools,omitempty"`
}

type openrouterContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type openrouterMessage struct {
	Role    string                     `json:"role"`
	Content interface{}                `json:"content"`
}

type openrouterToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openrouterResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string                `json:"role"`
			Content   string                `json:"content"`
			ToolCalls []openrouterToolCall  `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}

type openrouterStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
}

func (p *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (string, error) {
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
		Provider: "openrouter",
		Message:  "failed after retries",
		Err:      lastErr,
	}
}

func (p *OpenRouterProvider) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
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

func (p *OpenRouterProvider) makeRequest(ctx context.Context, messages []Message, stream bool) (string, error) {
	reqBody := openrouterRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "openrouter",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return "", &ProviderError{
			Provider: "openrouter",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("HTTP-Referer", p.config.SiteURL)
	req.Header.Set("X-Title", p.config.AppName)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: "openrouter",
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
			Provider: "openrouter",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", &ProviderError{
			Provider: "openrouter",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var openrouterResp openrouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openrouterResp); err != nil {
		return "", &ProviderError{
			Provider: "openrouter",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	if openrouterResp.Error != nil {
		return "", &ProviderError{
			Provider: "openrouter",
			Message:  openrouterResp.Error.Message,
		}
	}

	if len(openrouterResp.Choices) == 0 {
		return "", &ProviderError{
			Provider: "openrouter",
			Message:  "empty response",
		}
	}

	return openrouterResp.Choices[0].Message.Content, nil
}

func (p *OpenRouterProvider) streamRequest(ctx context.Context, messages []Message, ch chan<- string) error {
	reqBody := openrouterRequest{
		Model:       p.config.Model,
		Messages:    messages,
		MaxTokens:   p.config.MaxTokens,
		Temperature: p.config.Temperature,
		Stream:      true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return &ProviderError{
			Provider: "openrouter",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return &ProviderError{
			Provider: "openrouter",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("HTTP-Referer", p.config.SiteURL)
	req.Header.Set("X-Title", p.config.AppName)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: "openrouter",
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
			Provider: "openrouter",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &ProviderError{
			Provider: "openrouter",
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

		var chunk openrouterStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Error != nil {
			return &ProviderError{
				Provider: "openrouter",
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
			Provider: "openrouter",
			Message:  "stream read error",
			Err:      err,
		}
	}

	return nil
}

func (p *OpenRouterProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err := p.makeRequestWithTools(ctx, messages, tools, false)
		if err == nil {
			return response, nil
		}

		lastErr = err

		if !isRetryableError(err) {
			break
		}
	}

	return nil, &ProviderError{
		Provider: "openrouter",
		Message:  "failed after retries",
		Err:      lastErr,
	}
}

func (p *OpenRouterProvider) makeRequestWithTools(ctx context.Context, messages []Message, tools []Tool, stream bool) (*ChatResponse, error) {
	openrouterTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		openrouterTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		}
	}

	reqBody := map[string]interface{}{
		"model":       p.config.Model,
		"messages":    messages,
		"max_tokens":  p.config.MaxTokens,
		"temperature": p.config.Temperature,
		"stream":      stream,
		"tools":       openrouterTools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to marshal request",
			Err:      err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to create request",
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("HTTP-Referer", p.config.SiteURL)
	req.Header.Set("X-Title", p.config.AppName)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
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

		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "rate limit exceeded",
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to read response body",
			Err:      err,
		}
	}

	var openrouterResp openrouterResponse
	if err := json.Unmarshal(bodyBytes, &openrouterResp); err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	if openrouterResp.Error != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  openrouterResp.Error.Message,
		}
	}

	if len(openrouterResp.Choices) == 0 {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "empty response",
		}
	}

	chatResp := &ChatResponse{
		Content:    make([]ContentBlock, 0),
		StopReason: openrouterResp.Choices[0].FinishReason,
		ToolCalls:  make([]ToolUseBlock, 0),
	}

	messageContent := openrouterResp.Choices[0].Message.Content
	if messageContent != "" {
		chatResp.Content = append(chatResp.Content, TextBlock{
			Type: "text",
			Text: messageContent,
		})
		chatResp.TextContent = messageContent
	}

	if len(openrouterResp.Choices[0].Message.ToolCalls) > 0 {
		for _, toolCall := range openrouterResp.Choices[0].Message.ToolCalls {
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
				input = map[string]interface{}{
					"_raw_arguments": toolCall.Function.Arguments,
				}
			}

			toolUse := ToolUseBlock{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: input,
			}

			chatResp.Content = append(chatResp.Content, toolUse)
			chatResp.ToolCalls = append(chatResp.ToolCalls, toolUse)
		}
	}

	return chatResp, nil
}

func isRetryableError(err error) bool {
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

func (p *OpenRouterProvider) StreamWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan *ChatResponse, error) {
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

func (p *OpenRouterProvider) Health(ctx context.Context) error {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := p.makeRequest(ctx, messages, false)

	return err
}

func (p *OpenRouterProvider) GetTotalCost() float64 {
	p.costMu.RLock()
	defer p.costMu.RUnlock()

	return p.totalCost
}

func (p *OpenRouterProvider) ResetCost() {
	p.costMu.Lock()
	defer p.costMu.Unlock()
	p.totalCost = 0
}

func (p *OpenRouterProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to create list models request",
			Err:      err,
		}
	}

	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "list models request failed",
			Err:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(body)),
		}
	}

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ProviderError{
			Provider: "openrouter",
			Message:  "failed to decode response",
			Err:      err,
		}
	}

	models := make([]string, 0, len(result.Data))
	for _, model := range result.Data {
		models = append(models, model.ID)
	}

	return models, nil
}