# Build
FROM golang:1.19-alpine as build

WORKDIR /app/src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/bin/app

# Run
FROM alpine:3.17

WORKDIR /
VOLUME /data

COPY --from=build /app/bin/app /
CMD ["/app"]