# --- Makefile ---
APP_NAME   := pachingo
BUILD_DIR  := bin
MAIN_PATH  := ./cmd/pachingo
GO_FLAGS   := -trimpath -ldflags="-s -w"

.PHONY: all build test lint fmt vet run clean check-tools help

all: lint test build

## build: Compile le binaire optimisé pour la plateforme locale
build:
	@echo "--> Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GO_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## test: Exécute les tests unitaires avec race detector
test:
	@echo "--> Running unit tests with race detector..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

## lint: Vérifie le style via golangci-lint
lint:
	@echo "--> Running golangci-lint..."
	golangci-lint run ./...

## fmt: Vérifie le formatage gofmt (échec si fichiers non formatés)
fmt:
	@echo "--> Checking gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)

## vet: Analyse statique go vet
vet:
	@echo "--> Running go vet..."
	go vet ./...

## run: Exécute l'application en dry-run
run: build
	@echo "--> Running $(APP_NAME) dry-run..."
	./$(BUILD_DIR)/$(APP_NAME) check --config configs/config.example.yml

## clean: Supprime les binaires et fichiers temporaires
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
