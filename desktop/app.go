package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Segners/PachinGo/internal/audit"
	"github.com/Segners/PachinGo/internal/config"
	"github.com/Segners/PachinGo/internal/crypto"
	"github.com/Segners/PachinGo/internal/executor"
	"github.com/Segners/PachinGo/internal/inventory"
	"github.com/Segners/PachinGo/internal/report"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails backend binding PachinGo core.
type App struct {
	ctx       context.Context
	cfgPath   string
	cfg       *config.Config
	auditLog  *audit.Logger
	lastCheck []executor.HostResult
	lastInv   []inventory.HostInventory
}

// NewApp creates the application struct.
func NewApp() *App {
	return &App{
		cfgPath: defaultConfigPath(),
	}
}

func defaultConfigPath() string {
	if p := os.Getenv("PACHINGO_CONFIG"); p != "" {
		return p
	}
	// Prefer repo configs/ next to cwd, else user config dir.
	candidates := []string{
		filepath.Join("configs", "config.yml"),
		filepath.Join("..", "configs", "config.yml"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join("configs", "config.yml")
	}
	return filepath.Join(dir, "pachingo", "config.yml")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	al, err := audit.New("")
	if err == nil {
		a.auditLog = al
	}
	_ = a.reloadConfig()
}

func (a *App) reloadConfig() error {
	cfg, err := config.Load(a.cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = config.Default()
			cfg.SetPath(a.cfgPath)
			if mkErr := os.MkdirAll(filepath.Dir(a.cfgPath), 0o755); mkErr == nil {
				_ = cfg.Save(a.cfgPath)
			}
			a.cfg = cfg
			return nil
		}
		return err
	}
	a.cfg = cfg
	return nil
}

func (a *App) ensureCfg() error {
	if a.cfg == nil {
		return a.reloadConfig()
	}
	return nil
}

func (a *App) logAudit(action, target string, success bool, detail string) {
	if a.auditLog == nil {
		return
	}
	_ = a.auditLog.Log(action, target, success, detail)
}

// GetConfigPath returns the active config file path.
func (a *App) GetConfigPath() string {
	return a.cfgPath
}

// SetConfigPath switches the config file and reloads.
func (a *App) SetConfigPath(path string) error {
	a.cfgPath = path
	return a.reloadConfig()
}

// ListTargets returns all configured targets.
func (a *App) ListTargets() ([]config.Target, error) {
	if err := a.ensureCfg(); err != nil {
		return nil, err
	}
	return a.cfg.Targets, nil
}

// UpsertTarget creates or updates a target and saves YAML.
func (a *App) UpsertTarget(t config.Target, passwordPlain string) error {
	if err := a.ensureCfg(); err != nil {
		return err
	}
	if passwordPlain != "" {
		enc, err := crypto.Encrypt(passwordPlain)
		if err != nil {
			return err
		}
		t.PasswordEnc = enc
	}
	if err := a.cfg.UpsertTarget(t); err != nil {
		return err
	}
	if err := a.cfg.Save(a.cfgPath); err != nil {
		a.logAudit("config_write", t.Name, false, err.Error())
		return err
	}
	a.logAudit("config_write", t.Name, true, "upsert")
	return nil
}

// DeleteTarget removes a target and saves YAML.
func (a *App) DeleteTarget(name string) error {
	if err := a.ensureCfg(); err != nil {
		return err
	}
	if err := a.cfg.DeleteTarget(name); err != nil {
		return err
	}
	if err := a.cfg.Save(a.cfgPath); err != nil {
		a.logAudit("config_write", name, false, err.Error())
		return err
	}
	a.logAudit("config_write", name, true, "delete")
	return nil
}

// CheckUpdates vérifie les mises à jour des cibles choisies.
// names vide = toutes les cibles activées.
func (a *App) CheckUpdates(names []string) ([]executor.HostResult, error) {
	if err := a.ensureCfg(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.SSH.CommandTimeout.Std()+2*time.Minute)
	defer cancel()
	results, err := executor.CheckAll(ctx, a.cfg, names, a.emitPatchLog)
	if err != nil {
		return nil, err
	}
	a.lastCheck = results
	for _, r := range results {
		a.logAudit("check", r.Name, r.Error == "", r.Error)
	}
	return results, nil
}

// ApplyUpdates applique les upgrades des cibles choisies, après confirmation.
func (a *App) ApplyUpdates(confirm bool, names []string) ([]executor.HostResult, error) {
	if !confirm {
		return nil, fmt.Errorf("confirmation required")
	}
	if err := a.ensureCfg(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.SSH.CommandTimeout.Std()+10*time.Minute)
	defer cancel()
	results, err := executor.ApplyAll(ctx, a.cfg, names, a.emitPatchLog)
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		a.logAudit("upgrade", r.Name, r.Error == "", r.Error)
	}
	return results, nil
}

func (a *App) emitPatchLog(target, line string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "patch:log", map[string]string{
		"target": target,
		"line":   line,
	})
}

// ScanInventory scanne les technos des cibles choisies.
// names vide = toutes les cibles activées.
func (a *App) ScanInventory(names []string) ([]inventory.HostInventory, error) {
	if err := a.ensureCfg(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.SSH.CommandTimeout.Std()+5*time.Minute)
	defer cancel()
	results, err := inventory.ScanAll(ctx, a.cfg, names)
	if err != nil {
		return nil, err
	}
	a.lastInv = results
	for _, r := range results {
		a.logAudit("inventory", r.Name, r.Error == "", r.Error)
	}
	return results, nil
}

// ExportExcel writes an Excel report and returns the path.
func (a *App) ExportExcel(withCheck bool) (string, error) {
	if err := a.ensureCfg(); err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(a.cfgPath), "..", "exports")
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	// Prefer cwd/exports
	if cwd, err := os.Getwd(); err == nil {
		dir = filepath.Join(cwd, "exports")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("pachingo-%s.xlsx", time.Now().Format("20060102-150405")))

	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.SSH.CommandTimeout.Std()+10*time.Minute)
	defer cancel()

	inv := a.lastInv
	if len(inv) == 0 {
		scanned, err := inventory.ScanAll(ctx, a.cfg, nil)
		if err != nil {
			return "", err
		}
		inv = scanned
		a.lastInv = inv
	}
	checks := a.lastCheck
	if withCheck && len(checks) == 0 {
		checked, err := executor.CheckAll(ctx, a.cfg, nil, a.emitPatchLog)
		if err != nil {
			return "", err
		}
		checks = checked
		a.lastCheck = checks
	}
	if err := report.Export(path, inv, checks); err != nil {
		a.logAudit("export", "", false, err.Error())
		return "", err
	}
	a.logAudit("export", "", true, path)
	return path, nil
}

// GetAuditTail returns the last n audit events.
func (a *App) GetAuditTail(n int) ([]audit.Event, error) {
	if a.auditLog == nil {
		return nil, nil
	}
	if n <= 0 {
		n = 50
	}
	return a.auditLog.Tail(n)
}

// OpenPath opens a file or folder in the OS file manager.
func (a *App) OpenPath(path string) error {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(path))
	return nil
}

// SelectConfigFile opens a file dialog to pick a YAML config.
func (a *App) SelectConfigFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choisir config.yml",
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML", Pattern: "*.yml;*.yaml"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	a.cfgPath = path
	if err := a.reloadConfig(); err != nil {
		return "", err
	}
	return path, nil
}
