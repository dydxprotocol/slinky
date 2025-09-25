FROM ghcr.io/dydxprotocol/slinky-dev-base AS builder
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
EXPOSE 8080 8002

COPY --from=builder /src/slinky/build/* /usr/local/bin/
RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /usr/local/bin/
ENTRYPOINT [ "slinky" ]
