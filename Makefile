BUILD_DIR := .build
SVC_NAME := vi-sql
REPOSITORY := github.com/kopecmaciej/vi-sql
VERSION ?= $(shell git describe --tags --always --dirty)

.PHONY: build run

all: build run

build:
	go build -ldflags="-s -w -X $(REPOSITORY)/cmd.version=$(VERSION)" -o $(BUILD_DIR)/$(SVC_NAME) .

run:
	env $$(cat .env) $(BUILD_DIR)/$(SVC_NAME)

test:
	go test ./...

test-verbose:
	go test -v ./...

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
