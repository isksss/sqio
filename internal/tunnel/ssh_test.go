package tunnel

import "testing"

// TestStartDisabled verifies the behavior covered by this test helper or case.
func TestStartDisabled(t *testing.T) {
	tunnel, err := Start(t.Context(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if tunnel != nil {
		t.Fatal("expected nil tunnel")
	}
}

// TestStartRequiresFields verifies the behavior covered by this test helper or case.
func TestStartRequiresFields(t *testing.T) {
	_, err := Start(t.Context(), Config{Enabled: true})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
