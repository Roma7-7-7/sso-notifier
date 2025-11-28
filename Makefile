
VERSION ?= dev
BUILD_TIME ?= $(shell date -u +%Y%m%d-%H%M%S)
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

lint:
	golangci-lint run ./...

lint\:changed:
	@echo "Linting only changed packages..."
	@CHANGED_PKGS=$$(git diff --name-only --diff-filter=ACM HEAD | grep '\.go$$' | xargs -I {} dirname {} | sort -u | sed 's|^|./|' | paste -sd ' ' -); \
	if [ -n "$$CHANGED_PKGS" ]; then \
		echo "Changed packages: $$CHANGED_PKGS"; \
		golangci-lint run $$CHANGED_PKGS; \
	else \
		echo "No Go files changed"; \
	fi

build:
	go mod download
	CGO_ENABLED=0 go build -o ./bin/sso-notifier ./cmd/bot/main.go

ci-build:
	go mod download
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o ./bin/sso-notifier ./cmd/bot/main.go
	echo "Version: $(VERSION)\nBuild Time: $(BUILD_TIME)" > ./bin/VERSION

testserver:
	go mod download
	CGO_ENABLED=0 go build -o ./bin/testserver ./cmd/testserver/main.go

test:
	go test ./...

coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@echo "Filtering excluded packages..."
	@grep -v -E '(github\.com/Roma7-7-7/sso-notifier/cmd/|github\.com/Roma7-7-7/sso-notifier/internal/dal/migrations/|github\.com/Roma7-7-7/sso-notifier/internal/dal/testutil/|/mocks/)' coverage.out > coverage.filtered.out || true
	@go tool cover -func=coverage.filtered.out | grep total | awk '{print "Total Coverage: " $$3}'
