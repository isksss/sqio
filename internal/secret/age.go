// Package secret resolves encrypted configuration values.
package secret

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

var runSecretCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// Resolve expands supported secret references. Plain values are returned as-is.
// Supported references are env:NAME, file:/path/to/secret, op:REF,
// aws-sm:SECRET_ID, and gcloud-secret:SECRET_ID.
func Resolve(value string) (string, error) {
	return ResolveContext(context.Background(), value)
}

// ResolveContext expands supported secret references using ctx for external
// secret-manager CLI calls.
func ResolveContext(ctx context.Context, value string) (string, error) {
	switch {
	case strings.HasPrefix(value, "env:"):
		name := strings.TrimPrefix(value, "env:")
		if name == "" {
			return "", fmt.Errorf("secret env name is required")
		}
		return os.Getenv(name), nil
	case strings.HasPrefix(value, "file:"):
		path := strings.TrimPrefix(value, "file:")
		if path == "" {
			return "", fmt.Errorf("secret file path is required")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	case strings.HasPrefix(value, "op:"):
		ref := strings.TrimPrefix(value, "op:")
		if ref == "" {
			return "", fmt.Errorf("1password secret reference is required")
		}
		return resolveCommand(ctx, "op", "op", "read", ref)
	case strings.HasPrefix(value, "aws-sm:"):
		secretID := strings.TrimPrefix(value, "aws-sm:")
		if secretID == "" {
			return "", fmt.Errorf("aws secrets manager secret id is required")
		}
		return resolveCommand(ctx, "aws secrets manager", "aws", "secretsmanager", "get-secret-value", "--secret-id", secretID, "--query", "SecretString", "--output", "text")
	case strings.HasPrefix(value, "gcloud-secret:"):
		secretID := strings.TrimPrefix(value, "gcloud-secret:")
		if secretID == "" {
			return "", fmt.Errorf("gcloud secret id is required")
		}
		return resolveCommand(ctx, "gcloud secret manager", "gcloud", "secrets", "versions", "access", "latest", "--secret", secretID)
	default:
		return value, nil
	}
}

func resolveCommand(ctx context.Context, provider, name string, args ...string) (string, error) {
	output, err := runSecretCommand(ctx, name, args...)
	if err != nil {
		return "", fmt.Errorf("%s secret resolve failed: %w", provider, err)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

// DecryptAge decrypts an age-encrypted value using identities from identityPath.
// Both armored and raw age payloads are supported.
func DecryptAge(ciphertext, identityPath string) (string, error) {
	if identityPath == "" {
		return "", fmt.Errorf("age identity is required")
	}
	identityData, err := os.ReadFile(identityPath)
	if err != nil {
		return "", err
	}
	identities, err := age.ParseIdentities(bytes.NewReader(identityData))
	if err != nil {
		return "", err
	}
	reader := bytes.NewReader([]byte(ciphertext))
	var input io.Reader = reader
	if bytes.HasPrefix([]byte(ciphertext), []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		armored := armor.NewReader(reader)
		input = armored
	}
	decrypted, err := age.Decrypt(input, identities...)
	if err != nil {
		return "", err
	}
	plain, err := io.ReadAll(decrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
