FROM golang:1.23-bullseye AS builder

RUN curl -sSLf "$(curl -sSLf https://api.github.com/repos/tomwright/dasel/releases/latest | grep browser_download_url | grep linux_amd64 | grep -v .gz | cut -d\" -f 4)" -L -o dasel && chmod +x dasel && mv ./dasel /usr/local/bin/dasel

RUN apt-get update && apt-get install jq -y && apt-get install ca-certificates -y

WORKDIR /src/slinky

COPY go.mod .

RUN go mod download

COPY . .

RUN make build

FROM ubuntu:rolling
EXPOSE 8080 8002

COPY --from=builder /src/slinky/build/* /usr/local/bin/
RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /usr/local/bin/
ENTRYPOINT [ "slinky" ]
