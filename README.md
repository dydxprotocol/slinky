# slinky

[![Project Status: Active – The project has reached a stable, usable state and is being actively developed.](https://www.repostatus.org/badges/latest/active.svg)](https://www.repostatus.org/#wip)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue?style=flat-square&logo=go)](https://godoc.org/github.com/dydxprotocol/slinky)
[![Go Report Card](https://goreportcard.com/badge/github.com/dydxprotocol/slinky?style=flat-square)](https://goreportcard.com/report/github.com/dydxprotocol/slinky)
[![Version](https://img.shields.io/github/tag/dydxprotocol/slinky.svg?style=flat-square)](https://github.com/dydxprotocol/slinky/releases/latest)
[![Lines Of Code](https://img.shields.io/tokei/lines/github/dydxprotocol/slinky?style=flat-square)](https://github.com/dydxprotocol/slinky)

## Overview

The slinky repository is composed of the following core packages:

* **abci** - This package contains the [vote extension](./abci/ve/README.md), [proposal](./abci/proposals/README.md), and [preblock handlers](./abci/preblock/oracle/README.md) that are used to broadcast oracle data to the network and to store it in the blockchain.
* **oracle** - This [package](./oracle/) contains the main oracle that aggregates external data sources before broadcasting it to the network. You can reference the provider documentation [here](./providers/base/README.md) to get a high level overview of how the oracle works.
* **providers** - This package contains a collection of [websocket](./providers/websockets/README.md) and [API](./providers/apis/README.md) based data providers that are used by the oracle to collect external data.
* **x/oracle** - This package contains a Cosmos SDK module that allows you to store oracle data on a blockchain.
* **x/marketmap** - This [package](./x/marketmap/README.md) contains  a Cosmos SDK module that allows for market configuration to be stored and updated on a blockchain.

## Installation

```bash
git clone https://github.com/dydxprotocol/slinky.git
cd slinky/cmd/slinky
go install .
```
## Validator Usage

To read how to run the oracle as a validator based on the chain, please reference the [validator documentation](https://docs.dydx.xyz/nodes/running-node/running-a-validator).

## Developer Usage

To run the oracle, run the following command.

```bash
$ make start-all-dev
```

This will:

1. Start a blockchain with a single validator node. It may take a few minutes to build and reach a point where vote extensions can be submitted.
2. Start the oracle side-car that will aggregate prices from external data providers and broadcast them to the network. To check the current aggregated prices on the side-car, you can run `curl localhost:8080/slinky/oracle/v1/prices`.
3. Host a prometheus instance that will scrape metrics from the oracle sidecar. Navigate to http://localhost:9091 to see all network traffic and metrics pertaining to the oracle sidecar. Navigate to http://localhost:8002 to see all application-side oracle metrics.
4. Host a profiler that will allow you to profile the oracle side-car. Navigate to http://localhost:6060 to see the profiler.
5. Host a grafana instance that will allow you to visualize the metrics scraped by prometheus. Navigate to http://localhost:3000 to see the grafana dashboard. The default username and password are `admin` and `admin`, respectively.

After a few minutes, run the following commands to see the prices written to the blockchain:

```bash
# access the blockchain container
$ docker exec -it compose-blockchain-1 bash

# query the price of bitcoin in USD on the node
$ (compose-blockchain-1) ./build/slinkyd q oracle price BTC USD
```

Result:

```bash
decimals: "8"
id: "0"
nonce: "44"
price:
  block_height: "46"
  block_timestamp: "2024-01-29T01:43:48.735542Z"
  price: "4221100000000"
```

To stop the oracle, run the following command:

```bash
$ make stop-all-dev
```

## Metrics

### Oracle Service Metrics

We have an extensive suite of metrics available to validators and chain operators.
 Please [join our discord](https://discord.gg/PeBGE9jrbu) if you want help setting them up!

* metrics relevant to the oracle service's health + operation are [here](./metrics.md)

### Oracle Application / Network Metrics

* metrics relevant to the network's (that is running the instance of slinky) performance are [here](./service/metrics/README.md)

---

## dYdX Fork

### Slinky vs. Connect Rename

* Skip renamed their repo (and code refs) from Slinky --> Connect for branding purposes.
* Unfortunately it is impossible to update V4 protocol to use the renamed version, as message names etc. were changed. This would cause downtime during the interval when validators have updated `v4-chain` to use the rename but have not yet updated sidecar (or vice-versa).
* As a result, the dYdX fork of `skip-mev/connect` has chosen to roll back to the pre-rename state, `slinky`, as the base for future development (along with backporting any essential post-rename changes to functionality).
