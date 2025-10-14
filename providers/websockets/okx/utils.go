package okx

import (
	"time"

	"github.com/dydxprotocol/slinky/oracle/config"
)

const (
	Name = "okx_ws"
	URL  = "wss://ws.okx.com:8443/ws/v5/public"

	// WriteInterval is the interval at which the OKX Websocket will write to the connection.
	// By default, there can be 3 messages written to the connection every second. Or 480
	// messages every hour.
	//
	// ref: https://www.okx.com/docs-v5/en/#overview-websocket-overview
	WriteInterval = 3000 * time.Millisecond

	// MaxSubscriptionsPerConnection is the maximum number of subscriptions that can be
	// assigned to a single connection for the OKX provider.
	//
	// ref: https://www.okx.com/docs-v5/en/#overview-websocket-overview
	MaxSubscriptionsPerConnection = 50

	// MaxSubscriptionsPerBatch is the maximum number of subscriptions that can be
	// assigned to a single batch for the OKX provider. We set the limit to 5 to be safe.
	MaxSubscriptionsPerBatch = 25

	// ReadTimeout is the timeout for reading from the OKX Websocket connection.
	ReadTimeout = 15 * time.Second
)

// DefaultWebSocketConfig is the default configuration for the OKX Websocket.
var DefaultWebSocketConfig = config.WebSocketConfig{
	Name:                          Name,
	Enabled:                       true,
	MaxBufferSize:                 config.DefaultMaxBufferSize,
	ReconnectionTimeout:           config.DefaultReconnectionTimeout,
	PostConnectionTimeout:         config.DefaultPostConnectionTimeout,
	Endpoints:                     []config.Endpoint{{URL: URL}},
	ReadBufferSize:                config.DefaultReadBufferSize,
	WriteBufferSize:               config.DefaultWriteBufferSize,
	HandshakeTimeout:              config.DefaultHandshakeTimeout,
	EnableCompression:             config.DefaultEnableCompression,
	ReadTimeout:                   ReadTimeout,
	WriteTimeout:                  config.DefaultWriteTimeout,
	PingInterval:                  config.DefaultPingInterval,
	WriteInterval:                 WriteInterval,
	MaxReadErrorCount:             config.DefaultMaxReadErrorCount,
	MaxSubscriptionsPerConnection: MaxSubscriptionsPerConnection,
	MaxSubscriptionsPerBatch:      MaxSubscriptionsPerBatch,
}
