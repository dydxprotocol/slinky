FROM ghcr.io/dydxprotocol/slinky-dev-base AS builder

WORKDIR /src/slinky

COPY go.mod .

RUN go mod download

COPY . .

RUN make build

FROM gcr.io/distroless/base-debian11:debug
EXPOSE 8080 8002

COPY --from=builder /src/slinky/build/* /usr/local/bin/

WORKDIR /usr/local/bin/
ENTRYPOINT [ "slinky" ]
