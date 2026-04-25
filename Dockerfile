# syntax=docker/dockerfile:1.7
FROM golang:1.25-bookworm AS builder
WORKDIR /src

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        libwebp-dev \
        pkg-config \
        ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /out/webppipe .

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        git \
        ca-certificates \
        libwebp7 \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/webppipe /usr/local/bin/webppipe
WORKDIR /github/workspace
ENTRYPOINT ["/usr/local/bin/webppipe"]
