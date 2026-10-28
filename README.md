# PachinGo

Outil de **patch management** et **inventaire techno** en Go pour des **VM** et **conteneurs LXC** via SSH.

Fonctionnalités :
- Connexion SSH (clé privée / agent / mot de passe chiffré localement)
- Check & upgrade apt/dnf (dry-run ou `--confirm`)
- Inventaire Angular, React, Flask, Django, Blazor/.NET
- Export Excel multi-feuilles
- Desktop Wails pour CRUD des cibles dans le YAML

## Prérequis

- [Go](https://go.dev/dl/) **1.26+** (CLI)
- [Make](https://www.gnu.org/software/make/) (Git Bash / WSL sous Windows)
- Pour le desktop : [Wails v2](https://wails.io/) + Node.js

```bash
make check-tools
```

## Configuration

Copiez l’exemple puis adaptez vos hôtes :

```bash
cp configs/config.example.yml configs/config.yml
```

```yaml
concurrency: 8
ssh:
  default_user: deploy
  default_port: 22
  connect_timeout: 10s
  command_timeout: 5m
  known_hosts: ~/.ssh/known_hosts
  default_private_key: ~/.ssh/id_ed25519
scan_roots:
  - /var/www
  - /opt
  - $HOME
targets:
  - name: web-01
    kind: vm          # vm | lxc
    host: 10.0.0.11
    user: deploy
    private_key: ~/.ssh/id_ed25519
    tags: [prod, web]
    enabled: true
```

Les hôtes doivent déjà être présents dans `known_hosts` (StrictHostKeyChecking).  
`configs/config.yml` est ignoré par git — ne committez jamais de secrets.

Depuis le **desktop**, l’onglet Cibles écrit directement dans ce YAML (ajout / édition / suppression).

## CLI

```bash
go run ./cmd/pachingo
# ou
make build
./bin/pachingo --config configs/config.yml check
./bin/pachingo --config configs/config.yml run --confirm
./bin/pachingo --config configs/config.yml inventory
./bin/pachingo --config configs/config.yml export -o exports/inventory.xlsx
```

| Commande | Rôle |
|----------|------|
| `check` | Liste les paquets upgradables (dry-run) |
| `run --confirm` | Applique `apt/dnf upgrade` |
| `inventory` | Scanne Angular / React / Flask / Django / Blazor /.NET |
| `export` | Excel (Hosts, Technologies, Updates) |
| `version` | Version du binaire |

## Desktop (Wails)

```bash
cd desktop
wails dev      # développement
wails build    # binaire desktop
```

Ou depuis la racine : `make desktop-dev` / `make desktop-build`.

Écrans : **Cibles** (CRUD YAML), **Patch**, **Inventaire**, **Export** (+ journal d’audit).

## Sécurité

- Auth SSH : clé / agent en priorité ; mot de passe uniquement via `password_enc` (AES-GCM, clé locale dans le dossier config utilisateur)
- Vérification `known_hosts`
- Upgrade CLI/UI uniquement avec confirmation explicite
- Audit JSONL local des actions sensibles
- Commandes distantes construites côté Go (pas d’entrée shell libre)

## Architecture

Le desktop Wails partage le **même** `go.mod` racine (les packages `internal/` ne sont pas importables depuis un autre module).

```
cmd/pachingo/          CLI cobra
desktop/               App Wails (React + Go bindings)
internal/config/       YAML load/save/CRUD
internal/sshx/         Client SSH
internal/patch/        apt / dnf
internal/executor/     Pool concurrent + SSHTarget
internal/inventory/    Scan techno
internal/report/       Export excelize
internal/audit/        Journal JSONL
internal/crypto/       Chiffrement mots de passe
configs/config.example.yml
```

## Makefile

| Cible | Description |
|-------|-------------|
| `make build` | Binaire `bin/pachingo` |
| `make test` | Tests + race detector |
| `make lint` | golangci-lint |
| `make all` | lint + test + build |
| `make desktop-dev` | `wails dev` dans `desktop/` |
| `make desktop-build` | `wails build` |

## CI/CD

| Workflow | Déclencheur | Rôle |
|----------|-------------|------|
| [CI Pipeline](.github/workflows/ci.yml) | Push / PR | fmt, vet, lint, tests, build |
| [Release](.github/workflows/release.yml) | Tag `v*.*.*` | GoReleaser |

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Licence

[GPL-3.0](LICENSE)
