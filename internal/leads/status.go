package leads

var allowed = map[string][]string{
	"NEW":          {"DISCOVERED", "DISQUALIFIED", "INVALID"},
	"DISCOVERED":   {"ENRICHING", "QUALIFIED", "DISQUALIFIED", "DUPLICATE"},
	"ENRICHING":    {"QUALIFIED", "INVALID"},
	"QUALIFIED":    {"VERIFIED", "DISQUALIFIED"},
	"VERIFIED":     {"READY", "INVALID"},
	"READY":        {"CONTACTED", "ARCHIVED"},
	"CONTACTED":    {"ENGAGED", "DISQUALIFIED"},
	"ENGAGED":      {"CONVERTED", "DISQUALIFIED"},
	"CONVERTED":    {"ARCHIVED"},
	"DISQUALIFIED": {"ARCHIVED"},
	"INVALID":      {"ARCHIVED"},
	"DUPLICATE":    {"ARCHIVED"},
}

func init() {
	// normalize
	for k, v := range allowed {
		allowed[k] = v
	}
}

func CanTransition(from, to string) bool {
	if from == "" {
		from = "NEW"
	}
	for _, a := range allowed[from] {
		if a == to {
			return true
		}
	}
	// allow to ARCHIVED from any
	if to == "ARCHIVED" {
		return true
	}
	return false
}
