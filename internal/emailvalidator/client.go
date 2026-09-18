package emailvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"research-leads/internal/researchtools"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	local   *researchtools.Verifier
}

// New builds a client for an external validation service. When baseURL is
// empty the client verifies locally instead (syntax, MX, optional SMTP probe)
// via local - or a DNS-only verifier when local is nil - so every caller gets
// real checks without a paid provider.
func New(baseURL, apiKey string, timeout int, local ...*researchtools.Verifier) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
	if len(local) > 0 && local[0] != nil {
		c.local = local[0]
	} else {
		c.local = &researchtools.Verifier{}
	}
	return c
}

// localScores give the agent's score hint per local verdict status.
var localScores = map[string]int{"valid": 90, "risky": 50, "unknown": 30, "invalid": 0}

type Result struct {
	Email  string `json:"email"`
	Status string `json:"status"`
	Score  int    `json:"score"`
}

// Verify checks one email address. Without a configured base URL it runs the
// built-in local verifier (syntax, disposable/role heuristics, MX, and an
// optional SMTP probe) so no address is ever silently rubber-stamped.
func (c *Client) Verify(ctx context.Context, email string) (Result, error) {
	if c.baseURL == "" {
		v := c.local.Verify(ctx, email)
		return Result{Email: v.Email, Status: v.Status, Score: localScores[v.Status]}, nil
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
