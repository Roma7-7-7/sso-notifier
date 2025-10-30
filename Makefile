
VERSION ?= dev
BUILD_TIME ?= $(shell date -u +%Y%m%d-%H%M%S)
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

build:
	go mod download
	CGO_ENABLED=0 go build -o ./bin/sso-notifier ./cmd/bot/main.go

ci-build:
	go mod download
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ./bin/sso-notifier ./cmd/bot/main.go
	echo "Version: $(VERSION)\nBuild Time: $(BUILD_TIME)" > ./bin/VERSION
