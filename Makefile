
build:
	go mod download
	CGO_ENABLED=0 go build -o ./bin/sso-notifier ./main.go

docker-build:
	docker build -t sso-notifier .

docker-compose:
	docker-compose down
	docker-compose up -d