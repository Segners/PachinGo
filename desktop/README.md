# PachinGo Desktop (Wails)

Interface graphique React + Go pour gérer les cibles YAML, patcher et exporter l’inventaire.

## Dev

Depuis la racine du dépôt (même `go.mod`) :

```bash
make desktop-dev
# ou
cd desktop && wails dev
```

## Build

```bash
make desktop-build
# ou
cd desktop && wails build
```

Le frontend doit être buildé (`frontend/dist`) avant `go build ./desktop` — `wails build` s’en charge.
