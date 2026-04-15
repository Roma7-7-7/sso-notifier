# --- Build stage ---
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 go build -o /sso-notifier ./cmd/bot

# --- Runtime stage ---
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 1000 appuser
WORKDIR /app

COPY --from=builder /sso-notifier .

RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

ENTRYPOINT ["./sso-notifier"]
