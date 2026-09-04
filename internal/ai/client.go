package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func New(baseURL, apiKey, model string, timeout int) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
}

type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}
type ResponseFormat struct {
	Type string `json:"type"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *Client) Chat(ctx context.Context, prompt string, jsonMode bool) (string, error) {
	fmt.Printf("{\"time\":\"%s\",\"level\":\"DEBUG\",\"msg\":\"ai request\",\"model\":\"%s\",\"prompt_len\":%d,\"jsonMode\":%v}\n", time.Now().Format(time.RFC3339), c.model, len(prompt), jsonMode)
	reqBody := ChatRequest{Model: c.model, Messages: []Message{{Role: "user", Content: prompt}}, Temperature: 0.2}
	if jsonMode {
		reqBody.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	fmt.Printf("{\"time\":\"%s\",\"level\":\"DEBUG\",\"msg\":\"ai response\",\"duration_ms\":%d,\"err\":\"%v\"}\n", time.Now().Format(time.RFC3339), time.Since(start).Milliseconds(), err)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai status %d", resp.StatusCode)
	}
	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return out.Choices[0].Message.Content, nil
}
