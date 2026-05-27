package cli

import "testing"

func TestParseSetValues(t *testing.T) {
	values, err := parseSetValues([]string{"name=alice", "status=active"})
	if err != nil {
		t.Fatal(err)
	}
	if values["name"] != "alice" || values["status"] != "active" {
		t.Fatalf("unexpected values: %+v", values)
	}
	if _, err := parseSetValues([]string{"bad"}); err == nil {
		t.Fatal("expected invalid set error")
	}
	if _, err := parseSetValues(nil); err == nil {
		t.Fatal("expected empty set error")
	}
}
