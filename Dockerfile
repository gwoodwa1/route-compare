# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY routecompare.go ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/routecompare \
    ./cmd/routecompare

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS runtime

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S routecompare \
    && adduser -S -G routecompare -h /workspace routecompare \
    && mkdir -p /workspace/input /workspace/reports \
    && chown -R routecompare:routecompare /workspace

COPY --from=build /out/routecompare /usr/local/bin/routecompare

USER routecompare:routecompare
WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/routecompare"]
