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
    make build-sim-app

## Prepare the final clear binary
## This will expose the tendermint and cosmos ports alongside
## starting up the sim app and the slinky daemon
EXPOSE 26656 26657 1317 9090 7171 26655 8081 26660

RUN apt-get update && apt-get install jq ca-certificates -y

ENTRYPOINT ["make", "build-and-start-app"]

