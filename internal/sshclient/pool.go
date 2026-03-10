package sshclient

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Pool manages persistent SSH connections, one per user@host:port.
type Pool struct {
	mu      sync.Mutex
	clients map[string]*ssh.Client
}

// Default is the shared global pool.
var Default = &Pool{clients: make(map[string]*ssh.Client)}

// Get returns a live SSH client for the given host, creating one if needed.
func (p *Pool) Get(user, host string, port int, identity, password string) (*ssh.Client, error) {
	if port == 0 {
		port = 22
	}
	key := fmt.Sprintf("%s@%s:%d", user, host, port)

	p.mu.Lock()
	c, ok := p.clients[key]
	p.mu.Unlock()

	if ok {
		// Probe to check if still alive.
		_, _, err := c.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return c, nil
		}
		p.mu.Lock()
		delete(p.clients, key)
		p.mu.Unlock()
	}

	c, err := dial(user, host, port, identity, password)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.clients[key] = c
	p.mu.Unlock()
	return c, nil
}

// RunCommand executes cmd on the remote server and returns its combined stdout.
func (p *Pool) RunCommand(user, host string, port int, identity, password, cmd string) ([]byte, error) {
	client, err := p.Get(user, host, port, identity, password)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	return sess.Output(cmd)
}

func dial(user, host string, port int, identity, password string) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	if identity != "" {
		signers, err := loadSigners(identity)
		if err != nil {
			return nil, err
		}
		if len(signers) > 0 {
			authMethods = append(authMethods, ssh.PublicKeys(signers...))
		}
	}

	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	if len(authMethods) == 0 {
		// fallback: try all keys in ~/.ssh/
		signers, _ := loadSigners("")
		if len(signers) > 0 {
			authMethods = append(authMethods, ssh.PublicKeys(signers...))
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth method available; set \"identity\" or \"password\" in config")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("tcp connect %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func loadSigners(identity string) ([]ssh.Signer, error) {
	if identity != "" {
		key, err := os.ReadFile(expandTilde(identity))
		if err != nil {
			return nil, fmt.Errorf("read identity %s: %w", identity, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse identity %s: %w", identity, err)
		}
		return []ssh.Signer{signer}, nil
	}

	// Scan all files in ~/.ssh/ and try to parse each as a private key.
	// Skips .pub files, config, known_hosts, and anything that doesn't parse.
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil, nil // no ~/.ssh — no keys, no error
	}

	skip := map[string]bool{"config": true, "known_hosts": true, "known_hosts.old": true}
	var signers []ssh.Signer
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if skip[name] || strings.HasSuffix(name, ".pub") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sshDir, name))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue // not a private key file
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func expandTilde(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
