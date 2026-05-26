package query

import "strings"

type Danger struct {
	Type    string
	Message string
}

func Dangerous(sql string) (Danger, bool) {
	for _, statement := range Statements(sql) {
		normalized := strings.ToLower(strings.Join(strings.Fields(AnalysisText(statement)), " "))
		switch {
		case strings.HasPrefix(normalized, "truncate "):
			return Danger{Type: "truncate", Message: "TRUNCATE is dangerous"}, true
		case strings.HasPrefix(normalized, "drop database "):
			return Danger{Type: "drop_database", Message: "DROP DATABASE is dangerous"}, true
		case strings.HasPrefix(normalized, "delete from ") && !strings.Contains(normalized, " where "):
			return Danger{Type: "delete_without_where", Message: "DELETE without WHERE is dangerous"}, true
		case strings.HasPrefix(normalized, "update ") && !strings.Contains(normalized, " where "):
			return Danger{Type: "update_without_where", Message: "UPDATE without WHERE is dangerous"}, true
		}
	}
	return Danger{}, false
}

func Mutating(sql string) bool {
	for _, statement := range Statements(sql) {
		normalized := strings.ToLower(strings.TrimSpace(AnalysisText(statement)))
		fields := strings.Fields(normalized)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "select", "with", "show", "describe", "explain", "pragma":
			continue
		default:
			return true
		}
	}
	return false
}
