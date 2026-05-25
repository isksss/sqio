package secret

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"filippo.io/age/armor"
)

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
