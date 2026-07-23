# PachinGo

Outil de **patch management** en Go pour automatiser les mises à jour de sécurité sur des **VM** et des **conteneurs LXC**.

## Prérequis

- [Go](https://go.dev/dl/) **1.26+**
- [Make](https://www.gnu.org/software/make/) (Git Bash / WSL sous Windows)
- [golangci-lint](https://golangci-lint.run/) (optionnel en local)

```bash
make check-tools 
```

## Démarrage rapide

```bash
git clone https://github.com/Segners/PachinGo.git
cd PachinGo

go run ./cmd/pachingo
# ou
make build
./bin/pachingo
```

## Commandes Makefile


| Cible        | Description                                 |
| ------------ | ------------------------------------------- |
| `make build` | Compile le binaire dans `bin/pachingo`      |
| `make test`  | Tests unitaires + race detector             |
| `make lint`  | Analyse golangci-lint                       |
| `make fmt`   | Vérifie le formatage `gofmt`                |
| `make vet`   | Lance `go vet`                              |
| `make all`   | lint + test + build                         |
| `make clean` | Supprime `bin/` et les fichiers de coverage |




## Architecture prévue

```
Encore en cours
```



## CI/CD


| Workflow                                 | Déclencheur          | Rôle                                                     |
| ---------------------------------------- | -------------------- | -------------------------------------------------------- |
| [CI Pipeline](.github/workflows/ci.yml)  | Push / PR sur `main` | fmt, vet, lint, tests (`-race`), build linux amd64/arm64 |
| [Release](.github/workflows/release.yml) | Tag `v*.*.*`         | Binaires via [GoReleaser](https://goreleaser.com/)       |


Exemple de release :

```bash
git tag v0.1.0
git push origin v0.1.0
```



## Licence

[GPL-3.0](LICENSE)