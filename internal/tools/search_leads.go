package tools

import (
	"context"
	"encoding/json"
	"research-leads/internal/leadscaptain"
)

type SearchLeadsTool struct { lc *leadscaptain.Client }
func NewSearch(lc *leadscaptain.Client) *SearchLeadsTool { return &SearchLeadsTool{lc: lc} }
func (t *SearchLeadsTool) Name() string { return "search_leads" }
func (t *SearchLeadsTool) Description() string { return "Search LeadsCaptain" }
func (t *SearchLeadsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p leadscaptain.SearchParams
	if err:=json.Unmarshal(input,&p); err!=nil {return nil,err}
	if p.PerPage==0 {p.PerPage=20}
	return t.lc.Search(ctx,p)
}
