package leads

import (
	"fmt"
	"strings"
)

type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}
type Query struct {
	Filters []Filter `json:"filters"`
	Sort    []struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sort"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

var fieldMap = map[string]string{
	"title": "position_title", "country": "country_code", "score": "overall_score", "company": "company_name", "industry": "industry_name", "status": "status", "city": "city", "email": "email",
}

func BuildWhere(filters []Filter) (string, []any, error) {
	if len(filters) == 0 {
		return "1=1", nil, nil
	}
	var wheres []string
	var args []any
	for _, f := range filters {
		col, ok := fieldMap[strings.ToLower(f.Field)]
		if !ok {
			col = f.Field
		}
		// whitelist col
		if !isSafe(col) {
			return "", nil, fmt.Errorf("invalid field %s", f.Field)
		}
		op := strings.ToUpper(f.Operator)
		switch op {
		case "=", "!=", "<", ">", "<=", ">=":
			wheres = append(wheres, fmt.Sprintf("%s %s ?", col, op))
			args = append(args, f.Value)
		case "CONTAINS":
			wheres = append(wheres, fmt.Sprintf("%s LIKE ?", col))
			args = append(args, "%"+fmt.Sprint(f.Value)+"%")
		case "IN":
			wheres = append(wheres, fmt.Sprintf("%s = ?", col))
			args = append(args, f.Value)
		case "EXISTS":
			wheres = append(wheres, fmt.Sprintf("%s IS NOT NULL", col))
		default:
			return "", nil, fmt.Errorf("unsupported operator %s", op)
		}
	}
	return strings.Join(wheres, " AND "), args, nil
}
func isSafe(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_') {
			return false
		}
	}
	return true
}
