package tools

import (
	"context"
	"encoding/json"
	"research-leads/internal/emailvalidator"
)

type VerifyEmailTool struct{ ev *emailvalidator.Client }

func NewVerify(ev *emailvalidator.Client) *VerifyEmailTool { return &VerifyEmailTool{ev: ev} }
func (t *VerifyEmailTool) Name() string                    { return "verify_email" }
func (t *VerifyEmailTool) Description() string             { return "Verify email" }
func (t *VerifyEmailTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		Email string `json:"email"`
	}
	json.Unmarshal(input, &p)
	return t.ev.Verify(ctx, p.Email)
}
