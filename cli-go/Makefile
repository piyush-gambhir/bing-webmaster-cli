BINARY := bwt
MODULE := github.com/piyush-gambhir/bing-webmaster-cli/cli-go
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# The built-in OAuth client comes from the environment or an untracked .env.local;
# it is never committed. Builds without it still support API-key login.
-include .env.local
export BWT_OAUTH_CLIENT_ID BWT_OAUTH_CLIENT_SECRET

LDFLAGS := -s -w -X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.Commit=$(COMMIT) -X $(MODULE)/internal/build.Date=$(DATE) \
	-X $(MODULE)/internal/auth.ClientID=$(BWT_OAUTH_CLIENT_ID) -X $(MODULE)/internal/auth.ClientSecret=$(BWT_OAUTH_CLIENT_SECRET)
INSTALL_DIR ?= $(shell go env GOPATH)/bin

.PHONY: build install test vet fmt tidy docs
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .
install:
	mkdir -p "$(INSTALL_DIR)"
	go build -ldflags "$(LDFLAGS)" -o "$(INSTALL_DIR)/$(BINARY)$(shell go env GOEXE)" .
test:
	go test -race -count=1 ./...
vet:
	go vet ./...
fmt:
	gofmt -w .
tidy:
	go mod tidy
docs:
	go run ./tools/gendocs > ../docs/commands.md
	go run ./tools/gencoverage > ../docs/api-coverage.md
