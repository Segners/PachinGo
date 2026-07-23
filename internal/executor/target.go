package executor

import "context"

type Update struct {
	Package      string `json:"package"`
	CurrentVer   string `json:"current_version"`
	AvailableVer string `json:"available_version"`
	Severity     string `json:"severity"`
}

// Target définit le contrat que tout type de cible (LXC, VM ) doit respecter.
type Target interface {
	// Name retourne l'identifiant unique ou hostname de la cible.
	Name() string

	// CheckUpdates interroge le système et liste les paquets en attente de mise à jour.
	CheckUpdates(ctx context.Context) ([]Update, error)

	// ApplyUpdates exécute la mise à jour des paquets.
	ApplyUpdates(ctx context.Context) error

	// IsRebootRequired vérifie si la machine nécessite un redémarrage post-patching.
	IsRebootRequired(ctx context.Context) (bool, error)
}
