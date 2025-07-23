FROM golang:1.23-bullseye AS builder

RUN curl -sSLf "$(curl -sSLf https://api.github.com/repos/tomwright/dasel/releases/latest | grep browser_download_url | grep linux_amd64 | grep -v .gz | cut -d\" -f 4)" -L -o dasel && chmod +x dasel && mv ./dasel /usr/local/bin/dasel

RUN apt-get update && apt-get install jq -y && apt-get install ca-certificates -y

WORKDIR /src/slinky

COPY go.mod .

RUN go mod download

COPY . .

RUN make build-test-app

## Prepare the final clear binary
## This will expose the tendermint and cosmos ports alongside 
## starting up the sim app and the slinky daemon
EXPOSE 26656 26657 1317 9090 7171 26655 8081 26660
ENTRYPOINT ["make", "build-and-start-app"]

