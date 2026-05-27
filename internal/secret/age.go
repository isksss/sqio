// Package secret resolves encrypted configuration values.
package secret

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"filippo.io/age/armor"
)

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
