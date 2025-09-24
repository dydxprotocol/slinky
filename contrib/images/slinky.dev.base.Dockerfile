FROM golang:1.25.1-trixie

LABEL org.opencontainers.image.source="https://github.com/dydxprotocol/slinky"

RUN curl -sSLf "$(curl -sSLf https://api.github.com/repos/tomwright/dasel/releases/latest | grep browser_download_url | grep linux_amd64 | grep -v .gz | cut -d\" -f 4)" -L -o dasel && chmod +x dasel && mv ./dasel /usr/local/bin/dasel

# Install only what downstream builds require
RUN apt-get update && apt-get install -y --no-install-recommends \
        make \
        git \
        curl \
        bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/slinky
