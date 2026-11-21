.PHONY: help dev build test lint clean swagger docker check-docs

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development
dev: ## Start backend dev server
	go run ./app/server

dev-frontend: ## Start frontend dev server
	cd web && npm run dev

# 构建元数据：只注入 Commit 与 BuildDate。
# Version 是 internal/version 里的 **const**，`-X` 注入不进 const（会被静默忽略，
# 于是"以为注入了 2.1.0 而二进制里还是 2.0.0"）—— 版本号的变更走代码 + 门禁，不走 ldflags。
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VPKG       := github.com/listen808/citebase/internal/version
LDFLAGS    := -X $(VPKG).Commit=$(COMMIT) -X $(VPKG).BuildDate=$(BUILD_DATE)

# Build
build: ## Build backend binary
	go build -ldflags "$(LDFLAGS)" -o citebase ./app/server

build-frontend: ## Build frontend assets
	cd web && npm run build

build-all: build build-frontend ## Build everything

# Testing
test: ## Run all Go tests (with race detector)
	go test -race ./internal/... ./tools/repocheck/... -v -count=1

test-coverage: ## Run tests with coverage
	go test ./internal/... -v -count=1 -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-middleware: ## Run middleware tests only
	go test -race ./internal/middleware/... -v -count=1

test-response: ## Run response tests only
	go test -race ./internal/response/... -v -count=1

# Code quality
lint: ## Run Go lint (golangci-lint if available, else gofmt + go vet)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... ; \
	else \
		echo "golangci-lint not installed; falling back to gofmt + go vet"; \
		test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1); \
		go vet ./... ; \
	fi

lint-frontend: ## Run frontend lint
	cd web && npm run lint:check

check-docs: ## 检查 markdown 里的相对链接是否都能落到真实文件
	node tools/validate-doc-links.mjs

changelog: ## 从 git log 重新生成 CHANGELOG.md（改范围：node tools/gen-changelog.mjs HEAD~20..HEAD）
	node tools/gen-changelog.mjs

notice: ## 重新生成 NOTICE.md（第三方许可清单，按现场构建清单算，勿手编）
	node tools/gen-notice.mjs

typecheck: ## Run frontend type check
	cd web && npm run type-check

ci: lint test build build-frontend check-docs ## Full CI gate (lint + test + build)

# Swagger
swagger: ## Regenerate Swagger API docs
	swag init -g app/server/main.go -o docs/swagger

# Docker
docker-build: ## Build Docker image
	docker build -t citebase:latest .

docker-up: ## Start all services with Docker Compose
	docker-compose up -d

docker-down: ## Stop all Docker Compose services
	docker-compose down

docker-logs: ## View Docker Compose logs
	docker-compose logs -f app

# Database
db-init: ## Print safe bootstrap order (table.sql 含 91 个 DROP TABLE，只能用于空库)
	@echo " databases/table.sql 含 91 个 DROP TABLE：只对空库执行，跑在已有数据的库上等于整库清空。"
	@echo " 空库初始化顺序（仅两步，全部 91 张表都在 table.sql 里）："
	@echo "   mysql -u root -p < databases/table.sql"
	@echo "   mysql -u root -p < databases/data.sql"
	@echo " 已上线的库不要重跑 table.sql（会 DROP 全部表）；结构升级交给服务启动时的 GORM AutoMigrate。"

db-dump: ## 整库导出 SQL（纯 Go，不依赖 mysqldump；DSN 取自 configs/config.yaml）
	go run ./tools/aidump -out citebase-dump-$$(date +%Y%m%d-%H%M%S).sql

# Clean
clean: ## Clean build artifacts
	rm -f citebase citebase.exe
	rm -f coverage.out coverage.html
	rm -rf web/dist
