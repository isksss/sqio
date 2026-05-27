// Package tunnel manages SSH local port forwarding for database connections.
package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config describes the SSH endpoint, authentication material, and remote
// database endpoint used to create a local tunnel.
type Config struct {
	Enabled    bool
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string
	RemoteHost string
	RemotePort int
}

// Tunnel represents an active local TCP listener backed by an SSH client.
type Tunnel struct {
	listener  net.Listener
	client    *ssh.Client
	localHost string
	localPort int
}

// Start opens an SSH tunnel when cfg.Enabled is true. Disabled tunnels return
// nil without error so callers can handle optional tunneling uniformly.
func Start(ctx context.Context, cfg Config) (*Tunnel, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Host == "" || cfg.User == "" || cfg.RemoteHost == "" || cfg.RemotePort == 0 {
		return nil, fmt.Errorf("ssh tunnel requires host, user, remote host, and remote port")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	clientConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), clientConfig)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	t := &Tunnel{
		listener:  listener,
		client:    client,
		localHost: "127.0.0.1",
		localPort: listener.Addr().(*net.TCPAddr).Port,
	}
	go t.accept(ctx, cfg)
	return t, nil
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
	err := t.listener.Close()
	if closeErr := t.client.Close(); err == nil {
		err = closeErr
	}
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
	remote, err := t.client.Dial("tcp", net.JoinHostPort(cfg.RemoteHost, strconv.Itoa(cfg.RemotePort)))
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go copyAndSignal(done, remote, local)
	go copyAndSignal(done, local, remote)
	<-done
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
