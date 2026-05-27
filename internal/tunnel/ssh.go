// Package tunnel manages SSH local port forwarding for database connections.
package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type sshClient interface {
	Dial(network, addr string) (net.Conn, error)
	Close() error
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}

var sshDial = func(network, addr string, config *ssh.ClientConfig) (sshClient, error) {
	return ssh.Dial(network, addr, config)
}

var sshNewClientFromConn = func(conn net.Conn, addr string, config *ssh.ClientConfig) (sshClient, error) {
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

var netListen = net.Listen

// Config describes the SSH endpoint, authentication material, and remote
// database endpoint used to create a local tunnel.
type Config struct {
	Enabled           bool
	Host              string
	Port              int
	User              string
	Password          string
	PrivateKey        string
	KnownHosts        string
	KeepAliveInterval time.Duration
	Reconnect         bool
	ReconnectAttempts int
	JumpHost          string
	JumpPort          int
	JumpUser          string
	JumpPassword      string
	JumpPrivateKey    string
	JumpKnownHosts    string
	RemoteHost        string
	RemotePort        int
}

// Tunnel represents an active local TCP listener backed by an SSH client.
type Tunnel struct {
	mu           sync.Mutex
	listener     net.Listener
	client       sshClient
	cfg          Config
	clientConfig *ssh.ClientConfig
	localHost    string
	localPort    int
}

// Start opens an SSH tunnel when cfg.Enabled is true. Disabled tunnels return
// nil without error so callers can handle optional tunneling uniformly.
func Start(ctx context.Context, cfg Config) (*Tunnel, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	cfg, clientConfig, err := startConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := dialSSH(cfg, clientConfig)
	if err != nil {
		return nil, err
	}
	listener, err := netListen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	t := &Tunnel{
		listener:     listener,
		client:       client,
		cfg:          cfg,
		clientConfig: clientConfig,
		localHost:    "127.0.0.1",
		localPort:    listener.Addr().(*net.TCPAddr).Port,
	}
	if cfg.KeepAliveInterval > 0 {
		go t.keepAlive(ctx, cfg.KeepAliveInterval)
	}
	go t.accept(ctx, cfg)
	return t, nil
}

func startConfig(cfg Config) (Config, *ssh.ClientConfig, error) {
	if cfg.Host == "" || cfg.User == "" || cfg.RemoteHost == "" || cfg.RemotePort == 0 {
		return cfg, nil, fmt.Errorf("ssh tunnel requires host, user, remote host, and remote port")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.ReconnectAttempts == 0 {
		cfg.ReconnectAttempts = 1
	}
	if cfg.JumpHost != "" && cfg.JumpPort == 0 {
		cfg.JumpPort = 22
	}
	auth, err := authMethods(cfg)
	if err != nil {
		return cfg, nil, err
	}
	hostKeyCallback, err := hostKeyCallback(cfg)
	if err != nil {
		return cfg, nil, err
	}
	return cfg, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}, nil
}

func dialSSH(cfg Config, clientConfig *ssh.ClientConfig) (sshClient, error) {
	target := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	if cfg.JumpHost == "" {
		return sshDial("tcp", target, clientConfig)
	}
	jumpCfg, jumpClientConfig, err := jumpConfig(cfg)
	if err != nil {
		return nil, err
	}
	jumpClient, err := sshDial("tcp", net.JoinHostPort(jumpCfg.Host, strconv.Itoa(jumpCfg.Port)), jumpClientConfig)
	if err != nil {
		return nil, err
	}
	conn, err := jumpClient.Dial("tcp", target)
	if err != nil {
		_ = jumpClient.Close()
		return nil, err
	}
	client, err := sshNewClientFromConn(conn, target, clientConfig)
	if err != nil {
		_ = jumpClient.Close()
		return nil, err
	}
	return &chainedSSHClient{sshClient: client, parent: jumpClient}, nil
}

func jumpConfig(cfg Config) (Config, *ssh.ClientConfig, error) {
	jump := Config{
		Host:       cfg.JumpHost,
		Port:       cfg.JumpPort,
		User:       firstNonEmpty(cfg.JumpUser, cfg.User),
		Password:   firstNonEmpty(cfg.JumpPassword, cfg.Password),
		PrivateKey: firstNonEmpty(cfg.JumpPrivateKey, cfg.PrivateKey),
		KnownHosts: firstNonEmpty(cfg.JumpKnownHosts, cfg.KnownHosts),
		RemoteHost: cfg.Host,
		RemotePort: cfg.Port,
	}
	return startConfig(jump)
}

// LocalHost returns the local address database clients should connect to.
func (t *Tunnel) LocalHost() string {
	return t.localHost
}

// LocalPort returns the ephemeral local port assigned to the tunnel.
func (t *Tunnel) LocalPort() int {
	return t.localPort
}

// Close stops accepting local connections and closes the SSH client.
func (t *Tunnel) Close() error {
	if t == nil {
		return nil
	}
	var err error
	if t.listener != nil {
		err = t.listener.Close()
	}
	if closeErr := t.closeClient(); err == nil {
		err = closeErr
	}
	return err
}

func (t *Tunnel) closeClient() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client == nil {
		return nil
	}
	err := t.client.Close()
	t.client = nil
	return err
}

// accept forwards each accepted local connection until the context is canceled
// or the listener is closed.
func (t *Tunnel) accept(ctx context.Context, cfg Config) {
	go func() {
		<-ctx.Done()
		_ = t.Close()
	}()
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		go t.forward(local, cfg)
	}
}

// forward connects one local client to the configured remote host through SSH.
func (t *Tunnel) forward(local net.Conn, cfg Config) {
	defer local.Close()
	remote, err := t.dialRemote(cfg)
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go copyAndSignal(done, remote, local)
	go copyAndSignal(done, local, remote)
	<-done
}

func (t *Tunnel) dialRemote(cfg Config) (net.Conn, error) {
	remote, err := t.currentClientDial(cfg)
	if err == nil || !cfg.Reconnect {
		return remote, err
	}
	attempts := cfg.ReconnectAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if err := t.reconnect(); err != nil {
			continue
		}
		remote, err = t.currentClientDial(cfg)
		if err == nil {
			return remote, nil
		}
	}
	return nil, err
}

func (t *Tunnel) currentClientDial(cfg Config) (net.Conn, error) {
	client := t.currentClient()
	if client == nil {
		return nil, fmt.Errorf("ssh tunnel is closed")
	}
	return client.Dial("tcp", net.JoinHostPort(cfg.RemoteHost, strconv.Itoa(cfg.RemotePort)))
}

func (t *Tunnel) currentClient() sshClient {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client
}

func (t *Tunnel) reconnect() error {
	t.mu.Lock()
	oldClient := t.client
	t.client = nil
	t.mu.Unlock()
	if oldClient != nil {
		_ = oldClient.Close()
	}
	client, err := dialSSH(t.cfg, t.clientConfig)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.client = client
	t.mu.Unlock()
	return nil
}

// keepAlive periodically sends a global SSH request and closes the tunnel when
// the server stops responding.
func (t *Tunnel) keepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			client := t.currentClient()
			if client == nil {
				return
			}
			if err := sendKeepAlive(client); err != nil {
				_ = t.Close()
				return
			}
		}
	}
}

func sendKeepAlive(client sshClient) error {
	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

type chainedSSHClient struct {
	sshClient
	parent sshClient
}

func (c *chainedSSHClient) Close() error {
	err := c.sshClient.Close()
	if closeErr := c.parent.Close(); err == nil {
		err = closeErr
	}
	return err
}

// copyAndSignal copies one half of a bidirectional stream and signals when that
// direction ends.
func copyAndSignal(done chan<- struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	done <- struct{}{}
}

// authMethods builds SSH authentication methods from password and private key
// settings.
func authMethods(cfg Config) ([]ssh.AuthMethod, error) {
	auth := []ssh.AuthMethod{}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if cfg.PrivateKey != "" {
		key, err := os.ReadFile(cfg.PrivateKey)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("ssh tunnel requires password or private key")
	}
	return auth, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// hostKeyCallback builds a known_hosts based callback for SSH host key
// verification. An explicit path can be supplied; otherwise the user's default
// known_hosts file is used.
func hostKeyCallback(cfg Config) (ssh.HostKeyCallback, error) {
	path := cfg.KnownHosts
	if path == "" {
		path = defaultKnownHostsPath()
	}
	if path == "" {
		return nil, fmt.Errorf("ssh tunnel requires known_hosts path")
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, err
	}
	return callback, nil
}

func defaultKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}
