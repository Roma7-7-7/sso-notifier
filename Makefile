
build:
	go mod download
	CGO_ENABLED=0 go build -o ./bin/sso-notifier ./cmd/bot/main.go
