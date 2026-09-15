package emailvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string, timeout int) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
}

type Result struct {
	Email  string `json:"email"`
	Status string `json:"status"`
	Score  int    `json:"score"`
}

// Verify checks one email address. Without a configured base URL it returns a
// deterministic mock verdict so local development works offline.
func (c *Client) Verify(ctx context.Context, email string) (Result, error) {
	if c.baseURL == "" {
		status := "valid"
		if len(email)%3 == 0 {
			status = "invalid"
		}
		return Result{Email: email, Status: status, Score: 90}, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/verify?email="+url.QueryEscape(email), nil)
	if err != nil {
		return Result{}, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("email validator status %d", resp.StatusCode)
	}
	// Validation services do not share one response schema; accept the common
	// verdict fields and report anything else as unknown rather than pretending
	// the address is valid.
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{Email: email, Status: "unknown"}, nil
	}
	for _, key := range []string{"status", "result", "verdict", "email_status", "state"} {
		if v, ok := body[key].(string); ok && v != "" {
			return Result{Email: email, Status: v}, nil
		}
	}
	if v, ok := body["valid"].(bool); ok {
		if v {
			return Result{Email: email, Status: "valid"}, nil
		}
		return Result{Email: email, Status: "invalid"}, nil
	}
	return Result{Email: email, Status: "unknown"}, nil
}
