package sshx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Segners/PachinGo/internal/config"
	"github.com/Segners/PachinGo/internal/crypto"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client wraps an SSH connection for a single target.
type Client struct {
	cfg    *config.Config
	target config.Target
	client *ssh.Client
	onLine func(line string) // optional live console callback
}

// SetLineLogger registers a callback invoked for each stdout/stderr line.
func (c *Client) SetLineLogger(fn func(line string)) {
	c.onLine = fn
}

// Dial opens an SSH session to the target using key, agent, or encrypted password.
func Dial(ctx context.Context, cfg *config.Config, t config.Target) (*Client, error) {
	user := t.User
	if user == "" {
		user = cfg.SSH.DefaultUser
	}
	port := t.Port
	if port == 0 {
		port = cfg.SSH.DefaultPort
	}
	timeout := cfg.SSH.ConnectTimeout.Std()
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	auth, err := buildAuth(t, cfg)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := hostKeyCallback(cfg.SSH.KnownHosts)
	if err != nil {
		return nil, err
	}

	sshCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(t.Host, fmt.Sprintf("%d", port))
	type dialResult struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan dialResult, 1)
	go func() {
		c, err := ssh.Dial("tcp", addr, sshCfg)
		ch <- dialResult{c, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", addr, res.err)
		}
		return &Client{cfg: cfg, target: t, client: res.client}, nil
	}
}

func buildAuth(t config.Target, cfg *config.Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	keyPath := t.PrivateKey
	if keyPath == "" {
		keyPath = cfg.SSH.DefaultKey
	}
	if keyPath != "" {
		signer, err := loadPrivateKey(config.ExpandHome(keyPath))
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if a := agentAuth(); a != nil {
		methods = append(methods, a)
	}

	if t.PasswordEnc != "" {
		pw, err := crypto.Decrypt(t.PasswordEnc)
		if err != nil {
			return nil, fmt.Errorf("decrypt password: %w", err)
		}
		methods = append(methods, ssh.Password(pw))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("target %s: no SSH auth method (private key, agent, or password_enc)", t.Name)
	}
	return methods, nil
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: %w", path, err)
	}
	return signer, nil
}

func agentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

func hostKeyCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	path := config.ExpandHome(knownHostsPath)
	if path == "" {
		path = filepath.Join(mustHome(), ".ssh", "known_hosts")
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("known_hosts file not found at %s — add host keys before connecting", path)
		}
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("known_hosts: %w", err)
	}
	return cb, nil
}

func mustHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Run executes a remote command and returns combined stdout+stderr.
func (c *Client) Run(ctx context.Context, command string) (string, error) {
	return c.RunWithStdin(ctx, command, nil)
}

// RunWithStdin executes a remote command, optionally feeding stdin (e.g. sudo -S).
func (c *Client) RunWithStdin(ctx context.Context, command string, stdin []byte) (string, error) {
	timeout := c.cfg.SSH.CommandTimeout.Std()
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	// PTY only when feeding sudo -S (some hosts use Defaults requiretty).
	if stdin != nil {
		_ = session.RequestPty("xterm", 40, 80, ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		})
		session.Stdin = bytes.NewReader(stdin)
	}

	// Never share one bytes.Buffer across Stdout+Stderr without sync.
	var out safeBuffer
	var streamer *lineStreamer
	var writer io.Writer = &out
	if c.onLine != nil {
		streamer = newLineStreamer(c.onLine)
		writer = io.MultiWriter(&out, streamer)
	}
	session.Stdout = writer
	session.Stderr = writer

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		if streamer != nil {
			streamer.Flush()
		}
		return out.String(), ctx.Err()
	case err := <-errCh:
		if streamer != nil {
			streamer.Flush()
		}
		return out.String(), err
	}
}

// lineStreamer splits writes into lines and invokes onLine for each.
type lineStreamer struct {
	mu     sync.Mutex
	buf    strings.Builder
	onLine func(line string)
}

func newLineStreamer(onLine func(line string)) *lineStreamer {
	return &lineStreamer{onLine: onLine}
}

func (l *lineStreamer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			line := strings.TrimRight(l.buf.String(), "\r")
			l.buf.Reset()
			if l.onLine != nil {
				l.onLine(line)
			}
			continue
		}
		l.buf.WriteByte(b)
	}
	return len(p), nil
}

func (l *lineStreamer) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buf.Len() == 0 {
		return
	}
	line := strings.TrimRight(l.buf.String(), "\r")
	l.buf.Reset()
	if l.onLine != nil {
		l.onLine(line)
	}
}

// safeBuffer is safe for concurrent Write from SSH stdout/stderr copy goroutines.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// Shell runs script with /bin/sh -c and safe quoting.
func (c *Client) Shell(ctx context.Context, script string) (string, error) {
	return c.Run(ctx, "/bin/sh -c "+ShellQuote(script))
}

// ShellQuote wraps s in single quotes for POSIX sh.
func ShellQuote(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

// Target returns the associated config target.
func (c *Client) Target() config.Target {
	return c.target
}
