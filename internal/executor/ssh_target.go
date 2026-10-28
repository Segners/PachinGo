package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/Segners/PachinGo/internal/config"
	"github.com/Segners/PachinGo/internal/patch"
	"github.com/Segners/PachinGo/internal/sshx"
)

// LogFunc receives live console lines for a target.
type LogFunc func(target, line string)

// SSHTarget implements Target over SSH.
type SSHTarget struct {
	cfg    *config.Config
	target config.Target
}

// NewSSHTarget wraps a config target.
func NewSSHTarget(cfg *config.Config, t config.Target) *SSHTarget {
	return &SSHTarget{cfg: cfg, target: t}
}

// Name returns the target name.
func (t *SSHTarget) Name() string {
	return t.target.Name
}

// ConfigTarget returns the underlying config.
func (t *SSHTarget) ConfigTarget() config.Target {
	return t.target
}

func (t *SSHTarget) withManager(ctx context.Context, log LogFunc, fn func(*sshx.Client, *patch.Manager) error) error {
	if log != nil {
		log(t.target.Name, "→ connexion SSH…")
	}
	client, err := sshx.Dial(ctx, t.cfg, t.target)
	if err != nil {
		return err
	}
	defer client.Close()
	if log != nil {
		client.SetLineLogger(func(line string) {
			log(t.target.Name, line)
		})
	}
	mgr, err := patch.New(ctx, client)
	if err != nil {
		return err
	}
	return fn(client, mgr)
}

// CheckUpdates lists pending updates.
func (t *SSHTarget) CheckUpdates(ctx context.Context) ([]Update, error) {
	var updates []Update
	err := t.withManager(ctx, nil, func(_ *sshx.Client, mgr *patch.Manager) error {
		var err error
		updates, err = mgr.CheckUpdates(ctx)
		return err
	})
	return updates, err
}

// ApplyUpdates applies package upgrades.
func (t *SSHTarget) ApplyUpdates(ctx context.Context) error {
	return t.withManager(ctx, nil, func(_ *sshx.Client, mgr *patch.Manager) error {
		return mgr.ApplyUpdates(ctx)
	})
}

// IsRebootRequired checks reboot flag.
func (t *SSHTarget) IsRebootRequired(ctx context.Context) (bool, error) {
	var needed bool
	err := t.withManager(ctx, nil, func(_ *sshx.Client, mgr *patch.Manager) error {
		var err error
		needed, err = mgr.IsRebootRequired(ctx)
		return err
	})
	return needed, err
}

// HostResult is the outcome for one target in a pool run.
type HostResult struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Host           string   `json:"host"`
	OS             string   `json:"os,omitempty"`
	Updates        []Update `json:"updates,omitempty"`
	RebootRequired bool     `json:"reboot_required"`
	Error          string   `json:"error,omitempty"`
}

func emit(log LogFunc, target, line string) {
	if log != nil {
		log(target, line)
	}
}

func poolSize(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

// runPool ouvre une session SSH par cible et exécute action en parallèle.
func runPool(ctx context.Context, cfg *config.Config, names []string, log LogFunc, action func(context.Context, *patch.Manager, *HostResult) error) ([]HostResult, error) {
	targets, err := cfg.SelectTargets(names)
	if err != nil {
		return nil, err
	}
	results := make([]HostResult, len(targets))
	sem := make(chan struct{}, poolSize(cfg.Concurrency))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(i int, t config.Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			hr := HostResult{
				Name: t.Name,
				Kind: string(t.Kind),
				Host: t.Host,
			}
			emit(log, t.Name, "→ connexion SSH…")
			client, err := sshx.Dial(ctx, cfg, t)
			if err != nil {
				hr.Error = err.Error()
				emit(log, t.Name, "✗ "+err.Error())
				results[i] = hr
				return
			}
			defer client.Close()
			if log != nil {
				client.SetLineLogger(func(line string) { log(t.Name, line) })
			}
			mgr, err := patch.New(ctx, client)
			if err != nil {
				hr.Error = err.Error()
				emit(log, t.Name, "✗ "+err.Error())
				results[i] = hr
				return
			}
			hr.OS = mgr.OSPrettyName(ctx)
			if err := action(ctx, mgr, &hr); err != nil && hr.Error == "" {
				hr.Error = err.Error()
			}
			results[i] = hr
		}(i, t)
	}
	wg.Wait()
	return results, nil
}

// CheckAll vérifie les mises à jour des cibles demandées.
// Une liste de noms vide couvre toutes les cibles activées.
func CheckAll(ctx context.Context, cfg *config.Config, names []string, log LogFunc) ([]HostResult, error) {
	return runPool(ctx, cfg, names, log, func(ctx context.Context, mgr *patch.Manager, hr *HostResult) error {
		emit(log, hr.Name, "→ check updates ("+mgr.PackageManager()+")…")
		updates, err := mgr.CheckUpdates(ctx)
		if err != nil {
			emit(log, hr.Name, "✗ "+err.Error())
			return err
		}
		hr.Updates = updates
		reboot, _ := mgr.IsRebootRequired(ctx)
		hr.RebootRequired = reboot
		emit(log, hr.Name, fmt.Sprintf("✓ %d update(s), reboot=%v", len(updates), reboot))
		return nil
	})
}

// ApplyAll applique les upgrades sur les cibles demandées.
// La confirmation est exigée par l'appelant. Liste vide = toutes les cibles activées.
func ApplyAll(ctx context.Context, cfg *config.Config, names []string, log LogFunc) ([]HostResult, error) {
	return runPool(ctx, cfg, names, log, func(ctx context.Context, mgr *patch.Manager, hr *HostResult) error {
		emit(log, hr.Name, "→ upgrade ("+mgr.PackageManager()+")…")
		if err := mgr.ApplyUpdates(ctx); err != nil {
			emit(log, hr.Name, "✗ "+err.Error())
			return err
		}
		reboot, err := mgr.IsRebootRequired(ctx)
		if err != nil {
			hr.Error = fmt.Sprintf("upgrade ok but reboot check failed: %v", err)
			emit(log, hr.Name, "⚠ "+hr.Error)
			return nil
		}
		hr.RebootRequired = reboot
		emit(log, hr.Name, fmt.Sprintf("✓ upgrade ok, reboot=%v", reboot))
		return nil
	})
}
