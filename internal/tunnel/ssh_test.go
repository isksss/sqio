package tunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
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

func TestStartConfig(t *testing.T) {
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, clientConfig, err := startConfig(Config{
		Enabled: true, Host: "bastion", User: "deploy", Password: "secret",
		KnownHosts: knownHosts, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 22 || clientConfig.User != "deploy" || len(clientConfig.Auth) != 1 {
		t.Fatalf("unexpected start config: cfg=%+v client=%+v", cfg, clientConfig)
	}
	if _, _, err := startConfig(Config{Enabled: true, Host: "bastion", User: "deploy", RemoteHost: "db", RemotePort: 5432, KnownHosts: knownHosts}); err == nil {
		t.Fatal("expected missing auth error")
	}
}

func TestStartWithInjectedDial(t *testing.T) {
	oldSSHDial := sshDial
	oldNetListen := netListen
	t.Cleanup(func() {
		sshDial = oldSSHDial
		netListen = oldNetListen
	})
	client := &fakeSSHClient{}
	sshDial = func(network, addr string, _ *ssh.ClientConfig) (sshClient, error) {
		if network != "tcp" || addr != "bastion:2222" {
			t.Fatalf("unexpected ssh dial target: %s %s", network, addr)
		}
		return client, nil
	}
	netListen = func(network, addr string) (net.Listener, error) {
		if network != "tcp" || addr != "127.0.0.1:0" {
			t.Fatalf("unexpected listen target: %s %s", network, addr)
		}
		return &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 15432}, err: errors.New("closed")}, nil
	}
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	tunnel, err := Start(t.Context(), Config{
		Enabled: true, Host: "bastion", Port: 2222, User: "deploy", Password: "secret",
		KnownHosts: knownHosts, RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tunnel.LocalHost() != "127.0.0.1" || tunnel.LocalPort() == 0 {
		t.Fatalf("unexpected local endpoint: %s:%d", tunnel.LocalHost(), tunnel.LocalPort())
	}
	if err := tunnel.Close(); err != nil {
		t.Fatal(err)
	}
	if !client.closed {
		t.Fatal("expected ssh client close")
	}
}

func TestStartClosesClientOnListenError(t *testing.T) {
	oldSSHDial := sshDial
	oldNetListen := netListen
	t.Cleanup(func() {
		sshDial = oldSSHDial
		netListen = oldNetListen
	})
	client := &fakeSSHClient{}
	sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
		return client, nil
	}
	netListen = func(string, string) (net.Listener, error) {
		return nil, errors.New("listen failed")
	}
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Start(t.Context(), Config{
		Enabled: true, Host: "bastion", User: "deploy", Password: "secret",
		KnownHosts: knownHosts, RemoteHost: "db", RemotePort: 5432,
	})
	if err == nil {
		t.Fatal("expected listen error")
	}
	if !client.closed {
		t.Fatal("expected ssh client close on listen error")
	}
}

func TestDialSSHThroughJumpHost(t *testing.T) {
	oldSSHDial := sshDial
	oldNewClient := sshNewClientFromConn
	t.Cleanup(func() {
		sshDial = oldSSHDial
		sshNewClientFromConn = oldNewClient
	})
	parentConn, childConn := net.Pipe()
	defer childConn.Close()
	jumpClient := &fakeSSHClient{remote: parentConn}
	targetClient := &fakeSSHClient{}
	var dialTargets []string
	sshDial = func(_ string, addr string, config *ssh.ClientConfig) (sshClient, error) {
		dialTargets = append(dialTargets, addr)
		if config.User != "jump" {
			t.Fatalf("unexpected jump user: %s", config.User)
		}
		return jumpClient, nil
	}
	sshNewClientFromConn = func(conn net.Conn, addr string, config *ssh.ClientConfig) (sshClient, error) {
		if addr != "inner:22" || config.User != "deploy" {
			t.Fatalf("unexpected inner target: %s user=%s", addr, config.User)
		}
		_ = conn.Close()
		return targetClient, nil
	}
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := dialSSH(Config{
		Host: "inner", Port: 22, User: "deploy", Password: "secret", KnownHosts: knownHosts,
		JumpHost: "jumpbox", JumpPort: 2022, JumpUser: "jump", JumpPassword: "jump-secret",
		RemoteHost: "db", RemotePort: 5432,
	}, &ssh.ClientConfig{User: "deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if len(dialTargets) != 1 || dialTargets[0] != "jumpbox:2022" || jumpClient.dialAddr != "inner:22" {
		t.Fatalf("unexpected dial chain: targets=%v jumpDial=%s", dialTargets, jumpClient.dialAddr)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if !jumpClient.closed || !targetClient.closed {
		t.Fatal("expected chained clients to close")
	}
}

func TestDialSSHJumpHostErrors(t *testing.T) {
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("jump dial", func(t *testing.T) {
		oldSSHDial := sshDial
		t.Cleanup(func() { sshDial = oldSSHDial })
		sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
			return nil, errors.New("jump failed")
		}
		_, err := dialSSH(Config{
			Host: "inner", Port: 22, User: "deploy", Password: "secret", KnownHosts: knownHosts,
			JumpHost: "jumpbox", JumpUser: "jump", JumpPassword: "jump-secret", RemoteHost: "db", RemotePort: 5432,
		}, &ssh.ClientConfig{User: "deploy"})
		if err == nil {
			t.Fatal("expected jump dial error")
		}
	})
	t.Run("inner dial", func(t *testing.T) {
		oldSSHDial := sshDial
		t.Cleanup(func() { sshDial = oldSSHDial })
		jumpClient := &fakeSSHClient{err: errors.New("inner failed")}
		sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
			return jumpClient, nil
		}
		_, err := dialSSH(Config{
			Host: "inner", Port: 22, User: "deploy", Password: "secret", KnownHosts: knownHosts,
			JumpHost: "jumpbox", JumpUser: "jump", JumpPassword: "jump-secret", RemoteHost: "db", RemotePort: 5432,
		}, &ssh.ClientConfig{User: "deploy"})
		if err == nil || !jumpClient.closed {
			t.Fatalf("expected inner dial error and closed jump client, err=%v closed=%v", err, jumpClient.closed)
		}
	})
	t.Run("new client", func(t *testing.T) {
		oldSSHDial := sshDial
		oldNewClient := sshNewClientFromConn
		t.Cleanup(func() {
			sshDial = oldSSHDial
			sshNewClientFromConn = oldNewClient
		})
		parentConn, childConn := net.Pipe()
		defer childConn.Close()
		jumpClient := &fakeSSHClient{remote: parentConn}
		sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
			return jumpClient, nil
		}
		sshNewClientFromConn = func(conn net.Conn, _ string, _ *ssh.ClientConfig) (sshClient, error) {
			_ = conn.Close()
			return nil, errors.New("new client failed")
		}
		_, err := dialSSH(Config{
			Host: "inner", Port: 22, User: "deploy", Password: "secret", KnownHosts: knownHosts,
			JumpHost: "jumpbox", JumpUser: "jump", JumpPassword: "jump-secret", RemoteHost: "db", RemotePort: 5432,
		}, &ssh.ClientConfig{User: "deploy"})
		if err == nil || !jumpClient.closed {
			t.Fatalf("expected new client error and closed jump client, err=%v closed=%v", err, jumpClient.closed)
		}
	})
}

func TestJumpConfigDefaults(t *testing.T) {
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	jump, clientConfig, err := jumpConfig(Config{
		Host: "inner", Port: 2222, User: "deploy", Password: "secret", KnownHosts: knownHosts,
		JumpHost: "jumpbox", RemoteHost: "db", RemotePort: 5432,
	})
	if err != nil {
		t.Fatal(err)
	}
	if jump.Port != 22 || jump.User != "deploy" || jump.Password != "secret" || jump.RemoteHost != "inner" || jump.RemotePort != 2222 || clientConfig.User != "deploy" {
		t.Fatalf("unexpected jump config: jump=%+v client=%+v", jump, clientConfig)
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

func TestHostKeyCallbackAcceptsExistingKnownHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	callback, err := hostKeyCallback(Config{KnownHosts: path})
	if err != nil {
		t.Fatal(err)
	}
	if callback == nil {
		t.Fatal("expected callback")
	}
}

func TestDefaultKnownHostsPath(t *testing.T) {
	path := defaultKnownHostsPath()
	if path == "" || filepath.Base(path) != "known_hosts" {
		t.Fatalf("unexpected known_hosts path: %s", path)
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

func TestAuthMethodsMissingPrivateKey(t *testing.T) {
	if _, err := authMethods(Config{PrivateKey: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected missing private key error")
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
	if err := tunnel.Close(); err != nil {
		t.Fatal(err)
	}
	listener := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 15432}}
	tunnel = &Tunnel{listener: listener}
	if err := tunnel.Close(); err != nil {
		t.Fatal(err)
	}
	if !listener.closed {
		t.Fatal("expected listener close")
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

func TestForwardCopiesLocalToRemote(t *testing.T) {
	localA, localB := net.Pipe()
	remoteA, remoteB := net.Pipe()
	client := &fakeSSHClient{remote: remoteA}
	tunnel := &Tunnel{client: client}
	done := make(chan struct{})
	go func() {
		tunnel.forward(localA, Config{RemoteHost: "db", RemotePort: 5432})
		close(done)
	}()
	go func() {
		_, _ = localB.Write([]byte("abc"))
		_ = localB.Close()
	}()
	buf := make([]byte, 3)
	if _, err := io.ReadFull(remoteB, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "abc" {
		t.Fatalf("unexpected forwarded bytes: %q", string(buf))
	}
	_ = remoteB.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("forward did not finish")
	}
	if client.dialAddr != "db:5432" {
		t.Fatalf("unexpected remote dial target: %s", client.dialAddr)
	}
}

func TestDialRemoteReconnectsAfterFailure(t *testing.T) {
	oldSSHDial := sshDial
	t.Cleanup(func() { sshDial = oldSSHDial })
	firstClient := &fakeSSHClient{err: errors.New("remote dial failed")}
	remoteA, remoteB := net.Pipe()
	defer remoteB.Close()
	secondClient := &fakeSSHClient{remote: remoteA}
	dialCount := 0
	sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
		dialCount++
		return secondClient, nil
	}
	tunnel := &Tunnel{
		client:       firstClient,
		cfg:          Config{Host: "bastion", Port: 22, User: "deploy", Password: "secret", RemoteHost: "db", RemotePort: 5432},
		clientConfig: &ssh.ClientConfig{User: "deploy"},
	}
	remote, err := tunnel.dialRemote(Config{RemoteHost: "db", RemotePort: 5432, Reconnect: true, ReconnectAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	_ = remote.Close()
	if dialCount != 1 || !firstClient.closed || secondClient.dialAddr != "db:5432" {
		t.Fatalf("unexpected reconnect state: dialCount=%d firstClosed=%v secondDial=%s", dialCount, firstClient.closed, secondClient.dialAddr)
	}
}

func TestDialRemoteWithoutReconnectReturnsError(t *testing.T) {
	tunnel := &Tunnel{client: &fakeSSHClient{err: errors.New("remote dial failed")}}
	if _, err := tunnel.dialRemote(Config{RemoteHost: "db", RemotePort: 5432}); err == nil {
		t.Fatal("expected remote dial error")
	}
}

func TestDialRemoteClosedAndReconnectFailure(t *testing.T) {
	tunnel := &Tunnel{}
	if _, err := tunnel.currentClientDial(Config{RemoteHost: "db", RemotePort: 5432}); err == nil {
		t.Fatal("expected closed tunnel error")
	}
	oldSSHDial := sshDial
	t.Cleanup(func() { sshDial = oldSSHDial })
	sshDial = func(string, string, *ssh.ClientConfig) (sshClient, error) {
		return nil, errors.New("reconnect failed")
	}
	tunnel = &Tunnel{
		client:       &fakeSSHClient{err: errors.New("remote dial failed")},
		cfg:          Config{Host: "bastion", Port: 22, User: "deploy", Password: "secret", RemoteHost: "db", RemotePort: 5432},
		clientConfig: &ssh.ClientConfig{User: "deploy"},
	}
	if _, err := tunnel.dialRemote(Config{RemoteHost: "db", RemotePort: 5432, Reconnect: true, ReconnectAttempts: 2}); err == nil {
		t.Fatal("expected reconnect failure")
	}
}

func TestSendKeepAlive(t *testing.T) {
	client := &fakeSSHClient{}
	if err := sendKeepAlive(client); err != nil {
		t.Fatal(err)
	}
	if client.name != "keepalive@openssh.com" || !client.wantReply || client.payload != nil {
		t.Fatalf("unexpected keepalive request: %+v", client)
	}
	client.err = errors.New("keepalive failed")
	if err := sendKeepAlive(client); err == nil {
		t.Fatal("expected keepalive error")
	}
}

func TestKeepAliveClosesTunnelOnFailure(t *testing.T) {
	client := &fakeSSHClient{err: errors.New("keepalive failed")}
	listener := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 15432}}
	tunnel := &Tunnel{client: client, listener: listener}
	done := make(chan struct{})
	go func() {
		tunnel.keepAlive(context.Background(), time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("keepalive did not stop after failure")
	}
	if !client.closed || !listener.closed {
		t.Fatalf("expected tunnel close on keepalive failure: client=%v listener=%v", client.closed, listener.closed)
	}
}

func TestKeepAliveStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tunnel := &Tunnel{client: &fakeSSHClient{}}
	done := make(chan struct{})
	go func() {
		tunnel.keepAlive(ctx, time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("keepalive did not stop after context cancellation")
	}
}

type fakeSSHClient struct {
	name      string
	wantReply bool
	payload   []byte
	err       error
	closed    bool
	remote    net.Conn
	dialAddr  string
}

type fakeListener struct {
	addr   net.Addr
	err    error
	closed bool
}

func (l *fakeListener) Accept() (net.Conn, error) {
	if l.err != nil {
		return nil, l.err
	}
	return nil, errors.New("closed")
}

func (l *fakeListener) Close() error {
	l.closed = true
	return nil
}

func (l *fakeListener) Addr() net.Addr {
	return l.addr
}

func (c *fakeSSHClient) Dial(_, addr string) (net.Conn, error) {
	c.dialAddr = addr
	if c.err != nil {
		return nil, c.err
	}
	return c.remote, nil
}

func (c *fakeSSHClient) Close() error {
	c.closed = true
	return nil
}

func (c *fakeSSHClient) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	c.name = name
	c.wantReply = wantReply
	c.payload = payload
	return true, nil, c.err
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
