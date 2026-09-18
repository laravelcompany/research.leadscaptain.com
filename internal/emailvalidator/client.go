package emailvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"research-leads/internal/researchtools"
)

const DefaultBaseURL = "https://validation.laravelmail.com"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	local   *researchtools.Verifier
}

// New builds a client that uses LaravelMail's validation API first. If the
// service is unavailable or returns an unusable response, Verify falls back to
// the built-in verifier so lead ingestion can still finish with a real verdict.
func New(baseURL, apiKey string, timeout int, local ...*researchtools.Verifier) *Client {
	c := &Client{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), apiKey: apiKey, http: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
	if len(local) > 0 && local[0] != nil {
		c.local = local[0]
	} else {
		c.local = &researchtools.Verifier{}
	}
	return c
}

var localScores = map[string]int{"valid": 90, "risky": 50, "unknown": 30, "invalid": 0}

type Result struct {
	Email  string `json:"email"`
	Status string `json:"status"`
	Score  int    `json:"score"`
}

type apiResponse struct {
	Email   string `json:"email"`
	Verdict struct {
		Status string  `json:"status"`
		Score  float64 `json:"score"`
	} `json:"verdict"`
}

func (c *Client) endpoint() string {
	if strings.HasSuffix(c.baseURL, "/api/v1/verify-email") {
		return c.baseURL
	}
	return c.baseURL + "/api/v1/verify-email"
}

func (c *Client) verifyLocal(ctx context.Context, email string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	v := c.local.Verify(ctx, email)
	return Result{Email: v.Email, Status: v.Status, Score: localScores[v.Status]}, nil
}

// Verify checks LaravelMail first and falls back locally on network failures,
// non-2xx responses, malformed JSON, or an unsupported verdict. It does not
// retry 429s, respecting the service's Retry-After rate-limit contract.
func (c *Client) Verify(ctx context.Context, email string) (Result, error) {
	if c.baseURL == "" {
		return c.verifyLocal(ctx, email)
	}
	payload, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		return c.verifyLocal(ctx, email)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return c.verifyLocal(ctx, email)
	}
	req.Header.Set("Content-Type", "application/json")
	// The current LaravelMail service is keyless. Keep support for a key so a
	// deployment can add auth without another client change.
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return c.verifyLocal(ctx, email)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.verifyLocal(ctx, email)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return c.verifyLocal(ctx, email)
	}
	status := map[string]string{
		"safe": "valid", "risky": "risky", "unknown": "unknown", "invalid": "invalid",
	}[strings.ToLower(body.Verdict.Status)]
	if status == "" || math.IsNaN(body.Verdict.Score) || math.IsInf(body.Verdict.Score, 0) || body.Verdict.Score < 0 || body.Verdict.Score > 1 {
		return c.verifyLocal(ctx, email)
	}
	verifiedEmail := body.Email
	if verifiedEmail == "" {
		verifiedEmail = email
	}
	return Result{Email: verifiedEmail, Status: status, Score: int(math.Round(body.Verdict.Score * 100))}, nil
}
