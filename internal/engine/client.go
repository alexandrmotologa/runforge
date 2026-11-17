package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CompletionRequest encapsulates parameters for a model call.
type CompletionRequest struct {
	Model        string  `json:"model"`
	Prompt       string  `json:"prompt"`
	SystemPrompt string  `json:"system_prompt,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
}

// CompletionResponse returns generated content and token metrics.
type CompletionResponse struct {
	Content   string `json:"content"`
	TokensIn  int64  `json:"tokens_in"`
	TokensOut int64  `json:"tokens_out"`
	Model     string `json:"model"`
}

// ModelClient provides unified communication with language model providers.
type ModelClient interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// HTTPClient calls OpenAI-compatible or local inference endpoints.
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewHTTPClient(baseURL, apiKey string) *HTTPClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *HTTPClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]map[string]string, 0)
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": req.Prompt,
	})

	body := map[string]interface{}{
		"model":       req.Model,
		"messages":    messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse json response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return &CompletionResponse{
		Content:   content,
		TokensIn:  result.Usage.PromptTokens,
		TokensOut: result.Usage.CompletionTokens,
		Model:     req.Model,
	}, nil
}

// MockClient provides deterministic offline responses for local testing and zero-token evaluations.
type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Synthesize realistic token counts and deterministic response
	tokensIn := int64(len(req.Prompt) / 4)
	if tokensIn < 10 {
		tokensIn = 10
	}
	content := fmt.Sprintf("[runforge-offline-eval] Processed request using model %s successfully.", req.Model)
	tokensOut := int64(len(content) / 4)

	return &CompletionResponse{
		Content:   content,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Model:     req.Model,
	}, nil
}

// NewDefaultModelClient selects an active client based on environment variables.
func NewDefaultModelClient() ModelClient {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		baseURL := os.Getenv("OPENAI_BASE_URL")
		return NewHTTPClient(baseURL, key)
	}
	if ollamaHost := os.Getenv("OLLAMA_HOST"); ollamaHost != "" {
		return NewHTTPClient(fmt.Sprintf("%s/v1", ollamaHost), "")
	}
	return NewMockClient()
}
