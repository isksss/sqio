package tunnel

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

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

// TestHostKeyCallbackRequiresReadableKnownHosts verifies SSH host key
// verification fails closed when the configured known_hosts file is unavailable.
func TestHostKeyCallbackRequiresReadableKnownHosts(t *testing.T) {
	_, err := hostKeyCallback(Config{KnownHosts: filepath.Join(t.TempDir(), "missing_known_hosts")})
	if err == nil {
		t.Fatal("expected known_hosts error")
	}
}

// TestAuthMethods verifies password auth and missing auth validation.
func TestAuthMethods(t *testing.T) {
	methods, err := authMethods(Config{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected one auth method, got %d", len(methods))
	}
	if _, err := authMethods(Config{}); err == nil {
		t.Fatal("expected missing auth error")
	}
}

// TestAuthMethodsInvalidPrivateKey verifies unreadable key material is rejected.
func TestAuthMethodsInvalidPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id")
	if err := os.WriteFile(path, []byte("not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := authMethods(Config{PrivateKey: path}); err == nil {
		t.Fatal("expected private key parse error")
	}
}

// TestTunnelAccessorsAndClose verifies local endpoint helpers and idempotent nil
// close behavior.
func TestTunnelAccessorsAndClose(t *testing.T) {
	tunnel := &Tunnel{localHost: "127.0.0.1", localPort: 15432}
	if tunnel.LocalHost() != "127.0.0.1" || tunnel.LocalPort() != 15432 {
		t.Fatalf("unexpected local endpoint: %s:%d", tunnel.LocalHost(), tunnel.LocalPort())
	}
	if err := (*Tunnel)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}

// TestCopyAndSignal verifies one half of the forwarding copy loop signals
// completion.
func TestCopyAndSignal(t *testing.T) {
	done := make(chan struct{}, 1)
	reader, writer := io.Pipe()
	var out testWriter
	go copyAndSignal(done, &out, reader)
	if _, err := writer.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if out.String() != "abc" {
		t.Fatalf("unexpected copied output: %q", out.String())
	}
}

type testWriter struct {
	b []byte
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func (w *testWriter) String() string {
	return string(w.b)
}
