BUILD_DIR := .build
SVC_NAME := vi-sql
REPOSITORY := github.com/kopecmaciej/vi-sql
VERSION ?= $(shell git describe --tags --always --dirty)
DB_URL ?= postgres://postgres:postgres@localhost:5432/tui_sample_db?sslmode=disable

.PHONY: build run test-wezterm test-wezterm-slow

all: tidy build run

build:
	go build -ldflags="-s -w -X $(REPOSITORY)/internal/build.Version=$(VERSION)" -o $(BUILD_DIR)/$(SVC_NAME) .

run:
	env $$(cat .env) $(BUILD_DIR)/$(SVC_NAME)

just-test:
	make build; env $(cat .env | grep POSTGRES | xargs) ./.build/vi-sql -d

tidy:
	go mod tidy

test:
	go test -race ./...

test-verbose:
	go test -race -v ./...

test-cover:
	go test -race -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration:
	go test -race -tags integration -timeout 120s ./...

test-tui:
	go test -race ./internal/tui/...

test-all:
	go test -race -tags integration -timeout 120s ./...

test-wezterm: build
	VI_SQL_TESTS_DSN=$(DB_URL) go test -tags=wezterm -count=1 -v -timeout 120s ./tests/wezterm/scenarios/

test-wezterm-slow: build
	VI_SQL_TESTS_DSN=$(DB_URL) VI_SQL_TESTS_SLOW=1 go test -tags=wezterm -count=1 -v -timeout 300s ./tests/wezterm/scenarios/

debug:
	if [ -f /proc/sys/kernel/yama/ptrace_scope ]; then \
		sudo sysctl kernel.yama.ptrace_scope=0; \
	fi
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(SVC_NAME) .
	$(BUILD_DIR)/$(SVC_NAME)

lint:
	golangci-lint run

release:
	@if [ ! -f "./release-notes/$(VERSION).md" ]; then \
		echo "Error: Release notes not found for $(VERSION)"; \
		echo "Expected file: ./release-notes/$(VERSION).md"; \
		exit 1; \
	fi
	GITHUB_TOKEN=$$(grep GITHUB_TOKEN .env | cut -d'=' -f2) goreleaser release --release-notes ./release-notes/$(VERSION).md --clean

snapshot:
	goreleaser release --snapshot --clean

bump-version:
	@git describe --tags --abbrev=0 | awk -F. '{OFS="."; $NF+=1; print $0}'
