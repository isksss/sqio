package cli

import "testing"

func TestAccessLabels(t *testing.T) {
	if loginLabel(true) != "login" || loginLabel(false) != "no-login" {
		t.Fatal("unexpected login label")
	}
	if grantableLabel(true) != "grantable" || grantableLabel(false) != "-" {
		t.Fatal("unexpected grantable label")
	}
}
