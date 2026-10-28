package audit

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

// Event is a single audit log entry.
type Event struct {
	Time    time.Time `json:"time"`
	User    string    `json:"user"`
	Action  string    `json:"action"`
	Target  string    `json:"target,omitempty"`
	Success bool      `json:"success"`
	Detail  string    `json:"detail,omitempty"`
}

// Logger appends JSONL audit events.
type Logger struct {
	mu   sync.Mutex
	path string
}

// DefaultPath returns ~/.local/share/pachingo/audit.jsonl (or UserConfigDir fallback).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pachingo", "audit.jsonl"), nil
}

// New creates a logger writing to path (created if missing).
func New(path string) (*Logger, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &Logger{path: path}, nil
}

// Path returns the log file path.
func (l *Logger) Path() string {
	return l.path
}

// Log appends an event.
func (l *Logger) Log(action, target string, success bool, detail string) error {
	if l == nil {
		return nil
	}
	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	ev := Event{
		Time:    time.Now().UTC(),
		User:    username,
		Action:  action,
		Target:  target,
		Success: success,
		Detail:  detail,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// Tail returns the last n events (best-effort, reads whole file for simplicity).
func (l *Logger) Tail(n int) ([]Event, error) {
	if l == nil || n <= 0 {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := splitNonEmpty(data)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]Event, 0, len(lines))
	for _, line := range lines {
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func splitNonEmpty(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
