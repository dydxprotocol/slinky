# slinky-dev-base
FROM golang:1.23-bullseye

LABEL maintainer="dYdX Protocol" \
      description="Base development image for building Slinky components" \
      org.opencontainers.image.source="https://github.com/dydxprotocol/slinky"

# Install only what downstream builds require
RUN apt-get update && apt-get install -y --no-install-recommends \
        make \
        git \
        curl \
        bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/slinky
