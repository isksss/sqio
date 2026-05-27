package secret

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// TestDecryptAge verifies the behavior covered by this test helper or case.
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

func TestDecryptAgeErrors(t *testing.T) {
	if _, err := DecryptAge("payload", ""); err == nil {
		t.Fatal("expected missing identity error")
	}
	if _, err := DecryptAge("payload", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing identity file error")
	}
	identityPath := filepath.Join(t.TempDir(), "bad-key.txt")
	if err := os.WriteFile(identityPath, []byte("not an age identity"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptAge("payload", identityPath); err == nil {
		t.Fatal("expected identity parse error")
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	goodIdentityPath := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(goodIdentityPath, []byte(identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptAge("not encrypted", goodIdentityPath); err == nil {
		t.Fatal("expected decrypt error")
	}
}

func TestResolveSecretReferences(t *testing.T) {
	t.Setenv("SQIO_SECRET_TEST", "from-env")
	got, err := Resolve("env:SQIO_SECRET_TEST")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("unexpected env secret: %q", got)
	}
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Resolve("file:" + path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-file" {
		t.Fatalf("unexpected file secret: %q", got)
	}
	got, err = Resolve("plain")
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain" {
		t.Fatalf("unexpected plain secret: %q", got)
	}
	if _, err := Resolve("file:"); err == nil {
		t.Fatal("expected empty file path error")
	}
	if _, err := Resolve("env:"); err == nil {
		t.Fatal("expected empty env name error")
	}
}

func TestResolveExternalSecretReferences(t *testing.T) {
	oldRunSecretCommand := runSecretCommand
	t.Cleanup(func() {
		runSecretCommand = oldRunSecretCommand
	})
	type call struct {
		name string
		args []string
	}
	var calls []call
	runSecretCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		return []byte("resolved\n"), nil
	}
	tests := []struct {
		value string
		call  call
	}{
		{
			value: "op:op://vault/item/password",
			call:  call{name: "op", args: []string{"read", "op://vault/item/password"}},
		},
		{
			value: "aws-sm:prod/db/password",
			call:  call{name: "aws", args: []string{"secretsmanager", "get-secret-value", "--secret-id", "prod/db/password", "--query", "SecretString", "--output", "text"}},
		},
		{
			value: "gcloud-secret:prod-db-password",
			call:  call{name: "gcloud", args: []string{"secrets", "versions", "access", "latest", "--secret", "prod-db-password"}},
		},
	}
	for _, tt := range tests {
		got, err := Resolve(tt.value)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", tt.value, err)
		}
		if got != "resolved" {
			t.Fatalf("Resolve(%q) = %q, want resolved", tt.value, got)
		}
	}
	if !reflect.DeepEqual(calls, []call{tests[0].call, tests[1].call, tests[2].call}) {
		t.Fatalf("unexpected command calls: %#v", calls)
	}
}

func TestResolveExternalSecretReferenceErrors(t *testing.T) {
	oldRunSecretCommand := runSecretCommand
	t.Cleanup(func() {
		runSecretCommand = oldRunSecretCommand
	})
	tests := []string{"op:", "aws-sm:", "gcloud-secret:"}
	for _, value := range tests {
		if _, err := Resolve(value); err == nil {
			t.Fatalf("Resolve(%q): expected error", value)
		}
	}
	runSecretCommand = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("command failed")
	}
	if _, err := Resolve("op:op://vault/item/password"); err == nil {
		t.Fatal("expected command error")
	}
}
