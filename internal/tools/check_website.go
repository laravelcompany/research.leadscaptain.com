package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"research-leads/internal/webcheck"
)

// CheckWebsiteTool fetches a lead's company site and reports liveness, title,
// description and technology fingerprints. Free: no API key required.
type CheckWebsiteTool struct{}

func NewCheckWebsite() *CheckWebsiteTool { return &CheckWebsiteTool{} }
func (t *CheckWebsiteTool) Name() string { return "check_website" }
func (t *CheckWebsiteTool) Description() string {
	return "Fetch a company website: {\"domain\":\"acme.com\"}. Returns reachable, status_code, title, description, technologies. Free, no key."
}
func (t *CheckWebsiteTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	if p.Domain == "" {
		return nil, fmt.Errorf("check_website needs domain")
	}
	return webcheck.Check(ctx, p.Domain), nil
}
