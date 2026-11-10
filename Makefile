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

# ==========================================
# Docker
# ==========================================

docker-build: ## Build Docker image
	docker-compose build

docker-up: ## Build and run with Docker Compose
	docker-compose up --build -d

docker-down: ## Stop Docker Compose services
	docker-compose down

# ==========================================
# Schedule API (SwiftBar menu bar status)
# ==========================================

SCHEDULE_API_PID = bin/scheduleapi.pid
SCHEDULE_API_LOG = bin/scheduleapi.log

schedule-api-build:
	CGO_ENABLED=0 go build -o ./bin/scheduleapi ./cmd/scheduleapi

# configure via env, e.g. SCHEDULE_API_ADDR=127.0.0.1:5555 make schedule-api-start
schedule-api-start: schedule-api-build
	@if [ -f $(SCHEDULE_API_PID) ] && kill -0 $$(cat $(SCHEDULE_API_PID)) 2>/dev/null; then \
		echo "already running (pid $$(cat $(SCHEDULE_API_PID))), use schedule-api-restart to apply changes"; \
	else \
		nohup ./bin/scheduleapi >> $(SCHEDULE_API_LOG) 2>&1 & echo $$! > $(SCHEDULE_API_PID); \
		echo "started (pid $$(cat $(SCHEDULE_API_PID))), logs: $(SCHEDULE_API_LOG)"; \
	fi

schedule-api-stop:
	@if [ -f $(SCHEDULE_API_PID) ] && kill -0 $$(cat $(SCHEDULE_API_PID)) 2>/dev/null; then \
		kill $$(cat $(SCHEDULE_API_PID)) && echo "stopped"; \
	else \
		echo "not running"; \
	fi
	@rm -f $(SCHEDULE_API_PID)

schedule-api-restart: schedule-api-stop schedule-api-start

schedule-api-status:
	@if [ -f $(SCHEDULE_API_PID) ] && kill -0 $$(cat $(SCHEDULE_API_PID)) 2>/dev/null; then \
		echo "running (pid $$(cat $(SCHEDULE_API_PID)))"; \
	else \
		echo "not running"; \
	fi

schedule-api-logs:
	tail -f $(SCHEDULE_API_LOG)
