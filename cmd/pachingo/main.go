package main

import (
	"fmt"
	"log/slog"
	"os"

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
		Short: "PachinGo — patch management automatisé pour VM et LXC",
		Long:  `PachinGo est un outil concurrent en Go pour vérifier et appliquer les patchs sur des infrastructures hybrides.`,
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
		Run: func(cmd *cobra.Command, args []string) {
			slog.Info("Démarrage de l'analyse des cibles en mode check...",
				"config", cfgFile,
			)
			// TODO: Charger la config, instancier les targets et lancer les vérifications simultanées
		},
	}
}

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Applique les mises à jour sur l'ensemble du parc configuré",
		Run: func(cmd *cobra.Command, args []string) {
			slog.Info("Démarrage du processus de mise à jour général...",
				"config", cfgFile,
			)
			// TODO: Implémentation du moteur de patch concurrent avec worker pool
		},
	}
}
