FROM ghcr.io/dydxprotocol/slinky-base AS builder
LABEL org.opencontainers.image.source="https://github.com/dydxprotocol/slinky"

WORKDIR /src/slinky
ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/go/pkg/mod

RUN --mount=type=cache,target=${GOMODCACHE} \
    --mount=type=cache,target=${GOCACHE} \
    go env

COPY go.mod go.sum ./
RUN --mount=type=cache,target=${GOMODCACHE} \
    --mount=type=cache,target=${GOCACHE} \
    go mod download

COPY . .
RUN --mount=type=cache,target=${GOMODCACHE} \
    --mount=type=cache,target=${GOCACHE} \
    make build

FROM ubuntu:rolling
RUN apt-get update \
    && apt-get install -y --no-install-recommends jq ca-certificates make git curl bash dasel \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /src/slinky/build/* /usr/local/bin/

WORKDIR /usr/local/bin/
EXPOSE 8080 8002
ENTRYPOINT [ "slinky" ]
