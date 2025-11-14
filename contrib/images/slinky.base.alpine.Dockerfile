FROM golang:1.25.4-alpine3.22
LABEL org.opencontainers.image.source="https://github.com/dydxprotocol/slinky"

WORKDIR /src/slinky

RUN apk add --no-cache make git curl bash dasel jq ca-certificates
