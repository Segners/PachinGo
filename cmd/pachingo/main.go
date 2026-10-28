package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Segners/PachinGo/internal/audit"
	"github.com/Segners/PachinGo/internal/config"
	"github.com/Segners/PachinGo/internal/executor"
	"github.com/Segners/PachinGo/internal/inventory"
	"github.com/Segners/PachinGo/internal/report"
	"github.com/spf13/cobra"
)

// Injected by GoReleaser / ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var cfgFile string

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	rootCmd := &cobra.Command{
		Use:   "pachingo",
		Short: "PachinGo — patch management et inventaire techno pour VM et LXC",
		Long:  `PachinGo vérifie/applique les mises à jour via SSH et inventorie Angular, React, Flask, Django.`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"configs/config.yml",
		"Chemin vers le fichier de configuration",
	)

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newCheckCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newInventoryCmd())
	rootCmd.AddCommand(newExportCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, *audit.Logger, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, nil, err
	}
	al, err := audit.New("")
	if err != nil {
		return cfg, nil, err
	}
	return cfg, al, nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Affiche la version de PachinGo",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pachingo %s (commit=%s date=%s)\n", version, commit, date)
		},
	}
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Vérifie les mises à jour disponibles sans les appliquer (Dry-Run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, al, err := loadConfig()
			if err != nil {
				return err
			}
			slog.Info("check started", "config", cfgFile, "targets", len(cfg.EnabledTargets()))
			ctx, cancel := context.WithTimeout(context.Background(), cfg.SSH.CommandTimeout.Std()+2*time.Minute)
			defer cancel()

			results, err := executor.CheckAll(ctx, cfg, nil, func(target, line string) {
				slog.Info("check", "target", target, "line", line)
			})
			if err != nil {
				return err
			}
			ok := 0
			for _, r := range results {
				success := r.Error == ""
				if success {
					ok++
				}
				_ = al.Log("check", r.Name, success, fmt.Sprintf("updates=%d err=%s", len(r.Updates), r.Error))
				if !success {
					slog.Error("check failed", "target", r.Name, "error", r.Error)
					continue
				}
				slog.Info("check ok", "target", r.Name, "updates", len(r.Updates), "reboot", r.RebootRequired)
				for _, u := range r.Updates {
					fmt.Printf("%s\t%s\t%s -> %s\n", r.Name, u.Package, u.CurrentVer, u.AvailableVer)
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(results)
			slog.Info("check done", "ok", ok, "total", len(results))
			return nil
		},
	}
}

func newRunCmd() *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Applique les mises à jour sur le parc configuré",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("refus: ajoutez --confirm pour appliquer les upgrades")
			}
			cfg, al, err := loadConfig()
			if err != nil {
				return err
			}
			slog.Info("upgrade started", "config", cfgFile, "targets", len(cfg.EnabledTargets()))
			ctx, cancel := context.WithTimeout(context.Background(), cfg.SSH.CommandTimeout.Std()+10*time.Minute)
			defer cancel()

			results, err := executor.ApplyAll(ctx, cfg, nil, func(target, line string) {
				slog.Info("upgrade", "target", target, "line", line)
			})
			if err != nil {
				return err
			}
			for _, r := range results {
				success := r.Error == ""
				_ = al.Log("upgrade", r.Name, success, r.Error)
				if success {
					slog.Info("upgrade ok", "target", r.Name, "reboot", r.RebootRequired)
				} else {
					slog.Error("upgrade failed", "target", r.Name, "error", r.Error)
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirme l'application des upgrades")
	return cmd
}

func newInventoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inventory",
		Short: "Scanne Angular, React, Flask et Django sur les cibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, al, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), cfg.SSH.CommandTimeout.Std()+5*time.Minute)
			defer cancel()
			results, err := inventory.ScanAll(ctx, cfg, nil)
			if err != nil {
				return err
			}
			for _, r := range results {
				success := r.Error == ""
				_ = al.Log("inventory", r.Name, success, fmt.Sprintf("techs=%d err=%s", len(r.Techs), r.Error))
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
}

func newExportCmd() *cobra.Command {
	var outPath string
	var withCheck bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Exporte l'inventaire (et optionnellement les updates) en Excel",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, al, err := loadConfig()
			if err != nil {
				return err
			}
			if outPath == "" {
				outPath = filepath.Join("exports", fmt.Sprintf("pachingo-%s.xlsx", time.Now().Format("20060102-150405")))
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.SSH.CommandTimeout.Std()+10*time.Minute)
			defer cancel()

			inv, err := inventory.ScanAll(ctx, cfg, nil)
			if err != nil {
				return err
			}
			var checks []executor.HostResult
			if withCheck {
				checks, err = executor.CheckAll(ctx, cfg, nil, nil)
				if err != nil {
					return err
				}
			}
			if err := report.Export(outPath, inv, checks); err != nil {
				_ = al.Log("export", "", false, err.Error())
				return err
			}
			_ = al.Log("export", "", true, outPath)
			fmt.Println(outPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "Chemin du fichier .xlsx")
	cmd.Flags().BoolVar(&withCheck, "with-check", true, "Inclure la feuille Updates (check apt/dnf)")
	return cmd
}
