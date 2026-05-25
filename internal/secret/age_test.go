package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
)

func TestDecryptAge(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	armored := armor.NewWriter(&encrypted)
	writer, err := age.Encrypt(armored, identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := armored.Close(); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := DecryptAge(encrypted.String(), identityPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("expected secret, got %q", got)
	}
}
