package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Kind labels a target as VM or LXC.
type Kind string

const (
	KindVM  Kind = "vm"
	KindLXC Kind = "lxc"
)

// Duration wraps time.Duration for YAML strings like "10s".
type Duration time.Duration

// UnmarshalYAML parses duration strings.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		var n int64
		if err2 := value.Decode(&n); err2 != nil {
			return err
		}
		*d = Duration(time.Duration(n))
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML writes duration as a string.
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// Std converts to time.Duration.
func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

// Config is the root YAML configuration.
type Config struct {
	Concurrency int       `yaml:"concurrency" json:"concurrency"`
	SSH         SSHConfig `yaml:"ssh" json:"ssh"`
	ScanRoots   []string  `yaml:"scan_roots" json:"scan_roots"`
	Targets     []Target  `yaml:"targets" json:"targets"`
	path        string    `yaml:"-" json:"-"`
}

// SSHConfig holds shared SSH defaults.
type SSHConfig struct {
	DefaultUser    string   `yaml:"default_user" json:"default_user"`
	DefaultPort    int      `yaml:"default_port" json:"default_port"`
	ConnectTimeout Duration `yaml:"connect_timeout" json:"connect_timeout"`
	CommandTimeout Duration `yaml:"command_timeout" json:"command_timeout"`
	KnownHosts     string   `yaml:"known_hosts" json:"known_hosts"`
	DefaultKey     string   `yaml:"default_private_key" json:"default_private_key"`
}

// Target is a single SSH-reachable host.
type Target struct {
	Name           string   `yaml:"name" json:"name"`
	Kind           Kind     `yaml:"kind" json:"kind"`
	Host           string   `yaml:"host" json:"host"`
	User           string   `yaml:"user,omitempty" json:"user,omitempty"`
	Port           int      `yaml:"port,omitempty" json:"port,omitempty"`
	PrivateKey     string   `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	PasswordEnc    string   `yaml:"password_enc,omitempty" json:"password_enc,omitempty"`
	PackageManager string   `yaml:"package_manager,omitempty" json:"package_manager,omitempty"` // apt|dnf|yum|apk optional override
	Tags           []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Enabled        *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// IsEnabled returns whether the target is active (default true).
func (t Target) IsEnabled() bool {
	if t.Enabled == nil {
		return true
	}
	return *t.Enabled
}

// Path returns the file path this config was loaded from (if any).
func (c *Config) Path() string {
	return c.path
}

// SetPath sets the config file path used by Save.
func (c *Config) SetPath(path string) {
	c.path = path
}

// Default returns a sensible empty configuration.
func Default() *Config {
	return &Config{
		Concurrency: 8,
		SSH: SSHConfig{
			DefaultUser:    "root",
			DefaultPort:    22,
			ConnectTimeout: Duration(10 * time.Second),
			CommandTimeout: Duration(5 * time.Minute),
			KnownHosts:     "~/.ssh/known_hosts",
		},
		ScanRoots: []string{"/var/www", "/opt", "/srv", "/app", "$HOME"},
		Targets:   []Target{},
	}
}

// Load reads and validates a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.path = path
	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 8
	}
	if cfg.SSH.DefaultPort == 0 {
		cfg.SSH.DefaultPort = 22
	}
	if cfg.SSH.ConnectTimeout == 0 {
		cfg.SSH.ConnectTimeout = Duration(10 * time.Second)
	}
	if cfg.SSH.CommandTimeout == 0 {
		cfg.SSH.CommandTimeout = Duration(5 * time.Minute)
	}
	if cfg.SSH.KnownHosts == "" {
		cfg.SSH.KnownHosts = "~/.ssh/known_hosts"
	}
	if len(cfg.ScanRoots) == 0 {
		cfg.ScanRoots = []string{"/var/www", "/opt", "/srv", "/app", "$HOME"}
	}
	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		if t.User == "" {
			t.User = cfg.SSH.DefaultUser
		}
		if t.Port == 0 {
			t.Port = cfg.SSH.DefaultPort
		}
		if t.PrivateKey == "" {
			t.PrivateKey = cfg.SSH.DefaultKey
		}
		if t.Kind == "" {
			t.Kind = KindVM
		}
	}
}

// Validate checks required fields and uniqueness.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("nil config")
	}
	seen := map[string]struct{}{}
	for i, t := range c.Targets {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("targets[%d]: name is required", i)
		}
		key := strings.ToLower(t.Name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate target name: %s", t.Name)
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(t.Host) == "" {
			return fmt.Errorf("target %s: host is required", t.Name)
		}
		switch t.Kind {
		case KindVM, KindLXC, "":
		default:
			return fmt.Errorf("target %s: kind must be vm or lxc", t.Name)
		}
	}
	return nil
}

// EnabledTargets renvoie les cibles activées (enabled absent = activée).
func (c *Config) EnabledTargets() []Target {
	out := make([]Target, 0, len(c.Targets))
	for _, t := range c.Targets {
		if t.IsEnabled() {
			out = append(out, t)
		}
	}
	return out
}

// SelectTargets filtre les cibles activées par nom.
// Une liste vide renvoie toutes les cibles activées.
// Un nom inconnu ou désactivé renvoie une erreur.
func (c *Config) SelectTargets(names []string) ([]Target, error) {
	enabled := c.EnabledTargets()
	if len(names) == 0 {
		return enabled, nil
	}
	byName := make(map[string]Target, len(c.Targets))
	for _, t := range c.Targets {
		byName[strings.ToLower(t.Name)] = t
	}
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		key := strings.ToLower(strings.TrimSpace(n))
		if key == "" {
			continue
		}
		t, ok := byName[key]
		if !ok {
			return nil, fmt.Errorf("cible inconnue: %s", strings.TrimSpace(n))
		}
		if !t.IsEnabled() {
			return nil, fmt.Errorf("cible désactivée: %s", t.Name)
		}
		want[key] = struct{}{}
	}
	if len(want) == 0 {
		return enabled, nil
	}
	out := make([]Target, 0, len(want))
	for _, t := range enabled {
		if _, ok := want[strings.ToLower(t.Name)]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// Save writes the config atomically to its path (or path argument if set).
func (c *Config) Save(path string) error {
	if path == "" {
		path = c.path
	}
	if path == "" {
		return errors.New("config path is empty")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	c.path = path
	return nil
}

// UpsertTarget inserts or replaces a target by name.
func (c *Config) UpsertTarget(t Target) error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(t.Host) == "" {
		return errors.New("host is required")
	}
	if t.Kind == "" {
		t.Kind = KindVM
	}
	if t.Kind != KindVM && t.Kind != KindLXC {
		return errors.New("kind must be vm or lxc")
	}
	for i := range c.Targets {
		if strings.EqualFold(c.Targets[i].Name, t.Name) {
			if t.PasswordEnc == "" {
				t.PasswordEnc = c.Targets[i].PasswordEnc
			}
			if t.PackageManager == "" {
				t.PackageManager = c.Targets[i].PackageManager
			}
			c.Targets[i] = t
			return c.Validate()
		}
	}
	c.Targets = append(c.Targets, t)
	return c.Validate()
}

// DeleteTarget removes a target by name.
func (c *Config) DeleteTarget(name string) error {
	for i := range c.Targets {
		if strings.EqualFold(c.Targets[i].Name, name) {
			c.Targets = append(c.Targets[:i], c.Targets[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("target not found: %s", name)
}

// ExpandHome expands a leading ~/.
func ExpandHome(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
