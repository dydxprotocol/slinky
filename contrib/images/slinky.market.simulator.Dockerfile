FROM golang:1.23 AS builder
LABEL org.opencontainers.image.source="https://github.com/dydxprotocol/slinky"

WORKDIR /src/slinky
COPY go.mod .
RUN go mod download
COPY . .
RUN make build
RUN mkdir -p /data
VOLUME /data

FROM ubuntu:rolling
COPY --from=builder /src/slinky/build/* /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/scripts"]
