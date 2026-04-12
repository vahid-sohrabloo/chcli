// Package tunnel provides SSH tunnel support for forwarding local ports to
// remote ClickHouse instances through a bastion host.
package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHConfig holds the parameters for connecting to an SSH bastion host.
type SSHConfig struct {
	Host     string
	Port     uint16
	User     string
	KeyFile  string
	Password string
}

// Tunnel represents an active SSH tunnel that forwards a local port to a
// remote address through a bastion host.
type Tunnel struct {
	client     *ssh.Client
	listener   net.Listener
	agentConn  net.Conn // may be nil; closed on Close()
	localPort  int
	remoteAddr string // resolved bastion address (host:port)
}

// Open creates an SSH tunnel forwarding a local port to remoteHost:remotePort.
// Auth is attempted in order: SSH agent, key file, password.
// If cfg.Host matches an alias in ~/.ssh/config, HostName/User/Port/IdentityFile
// are resolved from the SSH config (explicit SSHConfig fields take precedence).
func Open(cfg SSHConfig, remoteHost string, remotePort uint16) (*Tunnel, error) {
	// Resolve ~/.ssh/config defaults for the given host alias.
	cfg = resolveSSHConfig(cfg)

	var authMethods []ssh.AuthMethod
	var agentConn net.Conn

	// 1. SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentConn = conn
			authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
		}
	}

	// 2. Key file
	keyPath := cfg.KeyFile
	if keyPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
				p := filepath.Join(home, ".ssh", name)
				if _, err := os.Stat(p); err == nil {
					keyPath = p
					break
				}
			}
		}
	}
	if keyPath != "" {
		if strings.HasPrefix(keyPath, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				keyPath = filepath.Join(home, keyPath[2:])
			}
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err == nil {
			signer, parseErr := ssh.ParsePrivateKey(keyBytes)
			if parseErr == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			} else {
				var passphraseMissing *ssh.PassphraseMissingError
				if errors.As(parseErr, &passphraseMissing) && cfg.Password != "" {
					// Key is encrypted — try with password as passphrase
					signer, parseErr = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(cfg.Password))
					if parseErr == nil {
						authMethods = append(authMethods, ssh.PublicKeys(signer))
					}
				}
			}
		}
	}

	// 3. Password auth
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		if agentConn != nil {
			agentConn.Close()
		}
		return nil, errors.New("no SSH auth methods available (no agent, no key file, no password)")
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // TODO: proper host key verification
	}

	// Connect to bastion
	addr := net.JoinHostPort(cfg.Host, strconv.FormatUint(uint64(cfg.Port), 10))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		if agentConn != nil {
			agentConn.Close()
		}
		return nil, fmt.Errorf("SSH connect to %s: %w", addr, err)
	}

	// Bind a local listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		if agentConn != nil {
			agentConn.Close()
		}
		return nil, fmt.Errorf("local listener: %w", err)
	}

	localPort := listener.Addr().(*net.TCPAddr).Port
	remoteAddr := net.JoinHostPort(remoteHost, strconv.FormatUint(uint64(remotePort), 10))

	// Forward accepted connections to the remote address through the SSH client
	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			remoteConn, err := client.Dial("tcp", remoteAddr)
			if err != nil {
				localConn.Close()
				continue
			}
			go forward(localConn, remoteConn)
		}
	}()

	return &Tunnel{client: client, listener: listener, agentConn: agentConn, localPort: localPort, remoteAddr: addr}, nil
}

// forward copies data bidirectionally between two connections.
// When either direction finishes, both connections are closed.
func forward(local, remote net.Conn) {
	done := make(chan struct{})
	go func() {
		io.Copy(local, remote) //nolint:errcheck
		close(done)
	}()
	io.Copy(remote, local) //nolint:errcheck
	<-done
	local.Close()
	remote.Close()
}

// LocalPort returns the local port that the tunnel is listening on.
func (t *Tunnel) LocalPort() int { return t.localPort }

// RemoteAddr returns the resolved bastion address (host:port).
func (t *Tunnel) RemoteAddr() string { return t.remoteAddr }

// Close shuts down the local listener, agent connection, and the SSH client.
func (t *Tunnel) Close() error {
	t.listener.Close()
	if t.agentConn != nil {
		t.agentConn.Close()
	}
	return t.client.Close()
}

// resolveSSHConfig fills in zero-value fields in cfg from ~/.ssh/config.
func resolveSSHConfig(cfg SSHConfig) SSHConfig {
	alias := cfg.Host

	// HostName: the real address to connect to.
	if hostname := ssh_config.Get(alias, "HostName"); hostname != "" {
		cfg.Host = hostname
	}

	// User
	if cfg.User == "" {
		if user := ssh_config.Get(alias, "User"); user != "" {
			cfg.User = user
		}
	}

	// Port
	if cfg.Port == 0 {
		if portStr := ssh_config.Get(alias, "Port"); portStr != "" {
			if p, err := strconv.ParseUint(portStr, 10, 16); err == nil {
				cfg.Port = uint16(p)
			}
		}
	}

	// IdentityFile
	if cfg.KeyFile == "" {
		if id := ssh_config.Get(alias, "IdentityFile"); id != "" && id != "~/.ssh/identity" {
			cfg.KeyFile = id
		}
	}

	return cfg
}
