# build
FROM golang:1.22.3-alpine3.20 AS build

RUN apk add --no-cache make

COPY . /app
WORKDIR /app
RUN make build

# run
FROM alpine:3.20

ENV TOKEN=""
VOLUME /app/data

RUN apk add --no-cache tzdata

COPY --from=build /app/bin/sso-notifier /app/sso-notifier

WORKDIR /app

ENTRYPOINT ["/app/sso-notifier"]
