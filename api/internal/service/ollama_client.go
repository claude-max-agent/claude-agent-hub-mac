package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/zono819/claude-agent-hub/api/internal/config"
)

// OllamaClient handles communication with Ollama API
type OllamaClient struct {
	baseURL      string
	model        string
	systemPrompt string
	options      map[string]interface{}
	historySize  int
	history      []OllamaMessage
	mu           sync.Mutex
	client       *http.Client
}

// NewOllamaClient creates a new Ollama API client with default settings
func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		baseURL:      baseURL,
		model:        model,
		systemPrompt: "必ず日本語で回答してください。",
		historySize:  0,
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

// NewOllamaClientWithSystem creates a new Ollama API client with a custom system prompt
func NewOllamaClientWithSystem(baseURL, model, systemPrompt string) *OllamaClient {
	return &OllamaClient{
		baseURL:      baseURL,
		model:        model,
		systemPrompt: systemPrompt,
		historySize:  0,
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

// NewOllamaClientFromConfig creates a new Ollama API client from configuration
func NewOllamaClientFromConfig(cfg *config.OllamaConfig) *OllamaClient {
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Port)
	model := cfg.Model
	if model == "" {
		model = "huihui_ai/qwen3.5-abliterated:9b"
	}

	c := &OllamaClient{
		baseURL:      baseURL,
		model:        model,
		systemPrompt: cfg.SystemPrompt,
		historySize:  cfg.ConversationHistorySize,
		client: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}

	if cfg.Options != nil {
		c.options = map[string]interface{}{
			"temperature":    cfg.Options.Temperature,
			"top_k":          cfg.Options.TopK,
			"top_p":          cfg.Options.TopP,
			"num_ctx":        cfg.Options.NumCtx,
			"num_predict":    cfg.Options.NumPredict,
			"repeat_penalty": cfg.Options.RepeatPenalty,
		}
	}

	return c
}

// OllamaChatRequest represents the request body for Ollama chat API
type OllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// OllamaMessage represents a chat message
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatResponse represents the response from Ollama chat API
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// Chat sends a message to Ollama and returns the response with confidence score
// Confidence is estimated based on response characteristics (0.0-1.0)
func (c *OllamaClient) Chat(message string) (response string, confidence float64, error error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build messages: system + history + user
	messages := []OllamaMessage{}
	if c.systemPrompt != "" {
		messages = append(messages, OllamaMessage{
			Role:    "system",
			Content: c.systemPrompt,
		})
	}

	// Append conversation history
	messages = append(messages, c.history...)

	// Append current user message
	userMsg := OllamaMessage{Role: "user", Content: message}
	messages = append(messages, userMsg)

	reqBody := OllamaChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options:  c.options,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0.0, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", 0.0, fmt.Errorf("failed to decode response: %w", err)
	}

	response = chatResp.Message.Content

	// Update conversation history
	if c.historySize > 0 {
		c.history = append(c.history, userMsg, OllamaMessage{
			Role:    "assistant",
			Content: response,
		})
		// Trim history to keep last N pairs (2 messages per pair)
		maxMessages := c.historySize * 2
		if len(c.history) > maxMessages {
			c.history = c.history[len(c.history)-maxMessages:]
		}
	}

	// Estimate confidence based on response length and characteristics
	confidence = c.estimateConfidence(response)

	return response, confidence, nil
}

// ClearHistory clears the conversation history
func (c *OllamaClient) ClearHistory() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = nil
}

// estimateConfidence estimates the confidence score of a response
func (c *OllamaClient) estimateConfidence(response string) float64 {
	if response == "" {
		return 0.0
	}

	confidence := 0.5

	length := len(response)
	if length > 100 {
		confidence += 0.1
	}
	if length > 300 {
		confidence += 0.1
	}

	if hasStructuredContent(response) {
		confidence += 0.15
	}

	if hasUncertaintyPhrases(response) {
		confidence -= 0.2
	}

	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// hasStructuredContent checks if the response contains structured elements
func hasStructuredContent(text string) bool {
	indicators := []string{"- ", "* ", "1.", "2.", "3.", "```", "##", "**"}
	for _, indicator := range indicators {
		if containsString(text, indicator) {
			return true
		}
	}
	return false
}

// hasUncertaintyPhrases checks if the response contains uncertainty phrases
func hasUncertaintyPhrases(text string) bool {
	phrases := []string{
		"わからない", "分からない", "不明", "perhaps", "maybe", "possibly",
		"I'm not sure", "I don't know", "かもしれない", "可能性がある",
	}
	for _, phrase := range phrases {
		if containsString(text, phrase) {
			return true
		}
	}
	return false
}

// containsString checks if a string contains a substring
func containsString(text, substr string) bool {
	return bytes.Contains([]byte(text), []byte(substr))
}

// Ping checks if Ollama server is reachable
func (c *OllamaClient) Ping() error {
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	return nil
}
