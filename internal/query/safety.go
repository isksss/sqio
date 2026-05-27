package query

// Danger describes a SQL pattern that is unsafe enough to block in readonly or
// guarded execution modes.
type Danger struct {
	Type    string
	Message string
}

// Dangerous returns the first destructive SQL statement detected in sql.
func Dangerous(sql string) (Danger, bool) {
	for _, statement := range Statements(sql) {
		tokens := Tokens(statement)
		if len(tokens) == 0 {
			continue
		}
		switch {
		case tokens[0] == "truncate":
			return Danger{Type: "truncate", Message: "TRUNCATE is dangerous"}, true
		case HasTokenSequence(tokens, "drop", "database"):
			return Danger{Type: "drop_database", Message: "DROP DATABASE is dangerous"}, true
		case HasTokenSequence(tokens, "delete", "from") && !HasToken(tokens, "where"):
			return Danger{Type: "delete_without_where", Message: "DELETE without WHERE is dangerous"}, true
		case tokens[0] == "update" && !HasToken(tokens, "where"):
			return Danger{Type: "update_without_where", Message: "UPDATE without WHERE is dangerous"}, true
		}
	}
	return Danger{}, false
}

// Mutating reports whether any statement appears to change database state.
// Read-like statement prefixes are allowed; everything else is treated
// conservatively as a mutation.
func Mutating(sql string) bool {
	for _, statement := range Statements(sql) {
		tokens := Tokens(statement)
		if len(tokens) == 0 {
			continue
		}
		switch tokens[0] {
		case "select", "with", "show", "describe", "explain", "pragma":
			continue
		default:
			return true
		}
	}
	return false
}
