package stork

import "github.com/dydxprotocol/slinky/oracle/config"

const (
	// Name is the name of the Stork provider.
	Name = "stork_ws"

	// URL is the production Stork Websocket URL.
	URL = "wss://api.jp.stork-oracle.network/evm/subscribe"

	// DefaultMaxSubscriptionsPerConnection is the default maximum number of subscriptions per connection.
	DefaultMaxSubscriptionsPerConnection = 100

	// DefaultMaxSubscriptionsPerBatch is the default maximum number of subscriptions per batch.
	DefaultMaxSubscriptionsPerBatch = 50
)

// DefaultWebSocketConfig is the default configuration for the Stork Websocket.
var DefaultWebSocketConfig = config.WebSocketConfig{
	Enabled:                       true,
	Name:                          Name,
	MaxBufferSize:                 config.DefaultMaxBufferSize,
	ReconnectionTimeout:           config.DefaultReconnectionTimeout,
	PostConnectionTimeout:         config.DefaultPostConnectionTimeout,
	Endpoints:                     []config.Endpoint{{URL: URL}},
	ReadBufferSize:                config.DefaultReadBufferSize,
	WriteBufferSize:               config.DefaultWriteBufferSize,
	HandshakeTimeout:              config.DefaultHandshakeTimeout,
	EnableCompression:             config.DefaultEnableCompression,
	WriteTimeout:                  config.DefaultWriteTimeout,
	ReadTimeout:                   config.DefaultReadTimeout,
	PingInterval:                  config.DefaultPingInterval,
	WriteInterval:                 config.DefaultWriteInterval,
	MaxReadErrorCount:             config.DefaultMaxReadErrorCount,
	MaxSubscriptionsPerConnection: DefaultMaxSubscriptionsPerConnection,
	MaxSubscriptionsPerBatch:      DefaultMaxSubscriptionsPerBatch,
}
