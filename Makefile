# --- Makefile ---
APP_NAME   := pachingo
BUILD_DIR  := bin
MAIN_PATH  := ./cmd/pachingo
GO_FLAGS   := -trimpath -ldflags="-s -w"

.PHONY: all build test lint fmt vet run clean check-tools desktop-dev desktop-build help

all: lint test build

## build: Compile le binaire optimisé pour la plateforme locale
build:
	@echo "--> Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GO_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## test: Exécute les tests unitaires avec race detector
test:
	@echo "--> Running unit tests with race detector..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./cmd/... ./internal/...

## vet: Analyse statique go vet
vet:
	@echo "--> Running go vet..."
	go vet ./cmd/... ./internal/...

## lint: Vérifie le style via golangci-lint
lint:
	@echo "--> Running golangci-lint..."
	golangci-lint run ./cmd/... ./internal/...

## fmt: Vérifie le formatage gofmt (échec si fichiers non formatés)
fmt:
	@echo "--> Checking gofmt..."
	@test -z "$$(gofmt -l ./cmd ./internal ./desktop/*.go)" || (echo "gofmt needed on:" && gofmt -l ./cmd ./internal ./desktop/*.go && exit 1)

## run: Exécute l'application en dry-run
run: build
	@echo "--> Running $(APP_NAME) dry-run..."
	./$(BUILD_DIR)/$(APP_NAME) check --config configs/config.example.yml

## desktop-dev: Lance le desktop Wails en mode live
desktop-dev:
	cd desktop && wails dev

## desktop-build: Compile le desktop Wails
desktop-build:
	cd desktop && wails build

## clean: Supprime les binaires et fichiers de coverage
clean:
	@echo "--> Cleaning build directory..."
	@rm -rf $(BUILD_DIR) coverage.out coverage.txt

## check-tools: Installe les outils de développement requis
check-tools:
	@echo "--> Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "--> Tools ready!"

## help: Affiche les cibles disponibles
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
