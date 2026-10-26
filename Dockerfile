# syntax=docker/dockerfile:1

FROM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build

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

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a AS runtime

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
