package stork

import (
	"time"

	"github.com/dydxprotocol/slinky/oracle/config"
)

const (
	// Name is the name of the Stork provider.
	Name = "stork_api"

	// URL is the base URL of the Stork REST API for fetching latest prices.
	URL = "https://rest.jp.stork-oracle.network/v1/prices/latest"
)

// DefaultAPIConfig is the default configuration for the Stork API.
var DefaultAPIConfig = config.APIConfig{
	Name:             Name,
	Atomic:           true,
	Enabled:          true,
	Timeout:          3000 * time.Millisecond,
	Interval:         3000 * time.Millisecond,
	ReconnectTimeout: 2000 * time.Millisecond,
	MaxQueries:       1,
	Endpoints: []config.Endpoint{{
		URL: URL,
		Authentication: config.Authentication{
			APIKeyHeader: "Authorization",
			APIKey:       "STORK_API_KEY",
		},
	}},
}

// LatestPricesResponse is the top-level response from the Stork /v1/prices/latest endpoint.
//
// ex.
//
//	{
//	  "data": {
//	    "XAGUSD": {
//	      "timestamp": 1234567890000000000,
//	      "asset_id": "XAGUSD",
//	      "price": "30500000000000000000"
//	    }
//	  }
//	}
type LatestPricesResponse struct {
	Data map[string]AssetPrice `json:"data"`
}

// AssetPrice represents a single asset's price data from the Stork API.
type AssetPrice struct {
	Timestamp int64  `json:"timestamp"`
	AssetID   string `json:"asset_id"`
	Price     string `json:"price"`
}
