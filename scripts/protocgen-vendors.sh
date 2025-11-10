#!/usr/bin/env sh

set -eo pipefail

cd proto/v2
buf lint
buf generate --template buf.gen.mexc.yaml
