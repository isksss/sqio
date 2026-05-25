package tunnel

import "testing"

func TestStartDisabled(t *testing.T) {
	tunnel, err := Start(t.Context(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if tunnel != nil {
		t.Fatal("expected nil tunnel")
	}
}

func TestStartRequiresFields(t *testing.T) {
	_, err := Start(t.Context(), Config{Enabled: true})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
