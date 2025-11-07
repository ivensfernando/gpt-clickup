# ============================================
# 🇮🇹 Makefile per il progetto GPT-ClickUp (Go)
# ============================================
# Autore: Ivens Fernando Verdi Signorini
# Linguaggio: Go
# Descrizione: automatizza i comandi comuni per sviluppo, build e debug
# ============================================

APP_NAME = gpt-clickup
MAIN_PATH = ./main.go
BINARY_PATH = ./bin/$(APP_NAME)
GO_FILES = $(shell find . -name '*.go' -not -path "./vendor/*")

# Permette di configurare mirrors alternativi se il proxy predefinito non è raggiungibile.
	GOPROXY_MIRRORS ?= https://proxy.golang.org,https://goproxy.cn,direct
	GOSUMDB_ENDPOINT ?= sum.golang.org


# ============================================
# 🇮🇹 Installa e aggiorna i moduli Go
# ============================================
deps:
	@echo "📦 Installazione / aggiornamento moduli Go..."
	@echo "   → GOPROXY=$(GOPROXY_MIRRORS)"
	@echo "   → GOSUMDB=$(GOSUMDB_ENDPOINT)"
	GOPROXY=$(GOPROXY_MIRRORS) GOSUMDB=$(GOSUMDB_ENDPOINT) go mod tidy
	GOPROXY=$(GOPROXY_MIRRORS) GOSUMDB=$(GOSUMDB_ENDPOINT) go mod download

    # 🇮🇹 Alternativa che forza l'uso di fonti dirette e disabilita la verifica SumDB.
    deps-direct:
	@echo "📦 Installazione moduli (fallback diretto, senza SumDB)..."
	GOPROXY=direct GOSUMDB=off go mod tidy
	GOPROXY=direct GOSUMDB=off go mod download

# ============================================
# 🇮🇹 Compila l'applicazione
# ============================================
build: deps
	@echo "🔧 Compilazione dell'applicazione..."
	mkdir -p bin
	go build -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "✅ Compilato: $(BINARY_PATH)"

# ============================================
# 🇮🇹 Esegue il programma normalmente
# ============================================
run:
	@echo "🚀 Avvio dell'applicazione..."
	go run $(MAIN_PATH)

# ============================================
# 🇮🇹 Esegue il programma in modalità debug
# (usa delve - debugger per Go)
# ============================================
debug:
	@echo "🐞 Avvio in modalità debug..."
	dlv debug $(MAIN_PATH) --headless --listen=:2345 --api-version=2

# ============================================
# 🇮🇹 Pulisce cache, moduli e binari
# ============================================
clean:
	@echo "🧹 Pulizia dei file temporanei e cache..."
	go clean -modcache
	rm -rf bin
	@echo "✅ Pulizia completata."

# ============================================
# 🇮🇹 Risolve automaticamente gli import mancanti
# e formatta il codice con goimports + gofmt
# (richiede 'go install golang.org/x/tools/cmd/goimports@latest')
# ============================================
fmt:
	@echo "🎨 Sistemazione degli import e formattazione..."
	goimports -w $(GO_FILES)
	gofmt -s -w $(GO_FILES)
	@echo "✅ Codice formattato."

# ============================================
# 🇮🇹 Alias veloce per tutto: fmt + build + run
# ============================================
dev: fmt build run

# ============================================
# 🇮🇹 Mostra tutti i comandi disponibili
# ============================================
help:
	@echo "Comandi disponibili:"
	@echo "  make deps     → Installa moduli Go"
	@echo "  make build    → Compila il progetto"
	@echo "  make run      → Esegue l'app"
	@echo "  make debug    → Avvia in modalità debug"
	@echo "  make clean    → Pulisce cache e binari"
	@echo "  make fmt      → Sistema import e formatta il codice"
	@echo "  make deps-direct → Scarica le dipendenze usando solo sorgenti dirette"
	@echo "  make dev      → fmt + build + run"

.PHONY: deps deps-direct build run debug clean fmt dev help
