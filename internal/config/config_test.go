package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Segners/PachinGo/internal/config"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	src := config.Default()
	src.SSH.DefaultUser = "deploy"
	enabled := true
	if err := src.UpsertTarget(config.Target{
		Name: "web-01", Kind: config.KindVM, Host: "10.0.0.1", Enabled: &enabled,
	}); err != nil {
		t.Fatal(err)
	}
	if err := src.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SSH.DefaultUser != "deploy" {
		t.Fatalf("user=%s", got.SSH.DefaultUser)
	}
	if len(got.Targets) != 1 || got.Targets[0].Name != "web-01" {
		t.Fatalf("targets=%v", got.Targets)
	}
}

func TestUpsertAndDelete(t *testing.T) {
	cfg := config.Default()
	if err := cfg.UpsertTarget(config.Target{Name: "a", Kind: config.KindLXC, Host: "1.1.1.1"}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UpsertTarget(config.Target{Name: "a", Kind: config.KindVM, Host: "2.2.2.2"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Targets[0].Host != "2.2.2.2" || cfg.Targets[0].Kind != config.KindVM {
		t.Fatalf("upsert replace failed: %+v", cfg.Targets[0])
	}
	if err := cfg.DeleteTarget("a"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 0 {
		t.Fatal("expected empty")
	}
}

func TestSelectTargets(t *testing.T) {
	off := false
	cfg := config.Default()
	cfg.Targets = []config.Target{
		{Name: "web", Kind: config.KindVM, Host: "10.0.0.1"},
		{Name: "db", Kind: config.KindLXC, Host: "10.0.0.2"},
		{Name: "old", Kind: config.KindVM, Host: "10.0.0.3", Enabled: &off},
	}
	all, err := cfg.SelectTargets(nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("all=%v err=%v", all, err)
	}
	got, err := cfg.SelectTargets([]string{"DB"})
	if err != nil || len(got) != 1 || got[0].Name != "db" {
		t.Fatalf("filter=%v err=%v", got, err)
	}
	if _, err := cfg.SelectTargets([]string{"missing"}); err == nil {
		t.Fatal("expected unknown target error")
	}
	if _, err := cfg.SelectTargets([]string{"old"}); err == nil {
		t.Fatal("expected disabled target error")
	}
}

func TestValidateDuplicate(t *testing.T) {
	cfg := config.Default()
	cfg.Targets = []config.Target{
		{Name: "x", Kind: config.KindVM, Host: "1"},
		{Name: "X", Kind: config.KindVM, Host: "2"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.example.yml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("example config not found")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SSH.ConnectTimeout.Std() != 10*time.Second {
		t.Fatalf("connect timeout=%v", cfg.SSH.ConnectTimeout.Std())
	}
	if len(cfg.Targets) < 1 {
		t.Fatal("expected sample targets")
	}
}
