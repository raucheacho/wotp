.PHONY: all build-cli build-core build-dashboard clean

# On récupère le tag git (ex: v1.0.7), et si aucun tag n'existe on utilise "dev"
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")

# Commande par défaut (quand on tape juste "make")
all: build-dashboard build-core build-cli

build-cli:
	@echo "==>Construction du CLI (Version: $(VERSION))..."
	@cd cli && go build -ldflags "-s -w -X 'main.version=$(VERSION)'" -o ../bin/wotp ./cmd/wotp
	@echo "CLI généré dans le dossier bin/wotp"

build-core:
	@echo "==>Construction du Core (Version: $(VERSION))..."
	@cd core && go build -ldflags "-s -w -X 'github.com/wotp/core/internal/config.version=$(VERSION)'" -o ../bin/wotp-core ./cmd/wotp-core
	@echo "Core généré dans le dossier bin/wotp-core"

build-dashboard:
	@echo "==>Construction du Dashboard..."
	@cd core/dashboard && bun install && VITE_APP_VERSION=$(VERSION) bun run build
	@echo "Dashboard généré"

clean:
	@echo "==>Nettoyage..."
	@rm -rf bin/
	@echo "Dossier bin/ supprimé"
