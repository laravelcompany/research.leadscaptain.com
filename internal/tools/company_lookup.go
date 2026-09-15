package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"research-leads/internal/companyreg"
)

// CompanyLookupTool verifies a company against free European registries,
// routed by country: GB Companies House, FR annuaire des entreprises,
// RO ANAF (fiscal code), and VIES VAT validation for every EU state.
// Every route fails safe with a clear, actionable error.
type CompanyLookupTool struct{ reg *companyreg.Client }

func NewCompanyLookup(reg *companyreg.Client) *CompanyLookupTool { return &CompanyLookupTool{reg: reg} }
func (t *CompanyLookupTool) Name() string                        { return "company_lookup" }
func (t *CompanyLookupTool) Description() string {
	return "Verify a company in free European registries: {\"name\":\"Acme Ltd\",\"country\":\"GB\"} (GB name search, free key), {\"name\":\"...\",\"country\":\"FR\"} (free, no key), {\"fiscal_code\":\"...\",\"country\":\"RO\"} (ANAF, no key), or {\"vat_number\":\"DE123456789\"} (VIES, any EU state, no key)."
}
func (t *CompanyLookupTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var q companyreg.Query
	if err := json.Unmarshal(input, &q); err != nil {
		return nil, err
	}
	if q.Name == "" && q.VATNumber == "" && q.FiscalCode == "" {
		return nil, fmt.Errorf("company_lookup needs name+country, fiscal_code+country=RO, or vat_number")
	}
	return t.reg.Lookup(ctx, q)
}
