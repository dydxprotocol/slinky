# Stork Provider

## Overview

The Stork provider is used to fetch ticker prices from the [Stork Network WebSocket API](https://api.jp.stork-oracle.network/evm/subscribe). Stork provides signed oracle price data with cryptographic proofs for various cryptocurrency assets.

## Authentication

The Stork WebSocket API requires Basic authentication using an API key. The API key should be provided in the configuration or as an environment variable `STORK_API_KEY`.

## Message Types

The provider handles the following message types:

### Subscribe Message
Used to subscribe to price feeds for specific assets:
```json
{
  "type": "subscribe",
  "trace_id": "optional_string",
  "data": ["BTCUSD", "ETHUSD", "BTCUSDMARK"]
}
```

### Oracle Prices Message
Contains the signed price data from Stork:
```json
{
  "type": "oracle_prices",
  "trace_id": "optional_string",
  "data": {
    "BTCUSD": {
      "stork_signed_price": {
        "public_key": "...",
        "encoded_asset_id": "...",
        "price": "67734000000000000000000",
        "timestampNs": "1716915868145000000",
        "evm_signature": "...",
        "starknet_signature": "..."
      }
    }
  }
}
```

## Configuration

Example configuration:
```json
{
  "name": "stork_ws",
  "enabled": true,
  "endpoints": [
    {
      "url": "wss://api.jp.stork-oracle.network/evm/subscribe",
      "authentication": {
        "apiKey": "${STORK_API_KEY}"
      }
    }
  ],
  "maxBufferSize": 1000,
  "reconnectionTimeout": "30s",
  "maxSubscriptionsPerConnection": 100,
  "maxSubscriptionsPerBatch": 50
}
```

## Asset ID Mapping

The provider automatically converts Slinky currency pair formats (e.g., "BTC/USD") to Stork asset ID formats (e.g., "BTCUSD") by removing separators and converting to uppercase.

## Rate Limits

Please refer to Stork's documentation for current rate limits and connection requirements.

## Error Handling

The provider implements comprehensive error handling for:
- Connection failures and reconnection
- Message parsing errors
- Price validation failures
- Authentication errors
- Rate limiting

## Supported Features

- Real-time price updates
- Automatic reconnection
- Message batching for subscriptions
- Cryptographic signature verification (signatures are included in responses)
- Timestamp handling (nanosecond precision)
