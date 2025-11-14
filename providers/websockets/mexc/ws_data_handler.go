package mexc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cosmos/gogoproto/proto"
	"go.uber.org/zap"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/providers/base/websocket/handlers"
	"github.com/dydxprotocol/slinky/providers/websockets/mexc/pb"
)

var _ types.PriceWebSocketDataHandler = (*WebSocketHandler)(nil)

// WebSocketDataHandler implements the WebSocketDataHandler interface. This is used to
// handle messages received from the MEXC websocket API.
type WebSocketHandler struct {
	logger *zap.Logger

	// ws is the config for the MEXC websocket.
	ws config.WebSocketConfig
	// cache maintains the latest set of tickers seen by the handler.
	cache types.ProviderTickers
}

// NewWebSocketDataHandler returns a new MEXC PriceWebSocketDataHandler.
func NewWebSocketDataHandler(
	logger *zap.Logger,
	ws config.WebSocketConfig,
) (types.PriceWebSocketDataHandler, error) {
	if ws.Name != Name {
		return nil, fmt.Errorf("expected websocket config name %s, got %s", Name, ws.Name)
	}

	if !ws.Enabled {
		return nil, fmt.Errorf("websocket config for %s is not enabled", Name)
	}

	if err := ws.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid websocket config for %s: %w", Name, err)
	}

	return &WebSocketHandler{
		logger: logger,
		ws:     ws,
		cache:  types.NewProviderTickers(),
	}, nil
}

// HandleMessage is used to handle a message received from the data provider. This is called
// when a message is received from the data provider. There are three types of messages that
// can be received from the data provider:
//
// 1. A message that confirms that the client has successfully subscribed to a channel.
// 2. A message that confirms that the client has successfully pinged the server.
// 3. A message that contains the latest price for a ticker.
func (h *WebSocketHandler) HandleMessage(
	message []byte,
) (types.PriceResponse, []handlers.WebsocketEncodedMessage, error) {
	var (
		resp types.PriceResponse
		msg  BaseMessage
	)

	// attempt to unmarshal the message as protobuf miniticker message
	// most message updates will be protobuf messages
	px := &pb.PublicMiniTickerV3Api{}
	if err := proto.Unmarshal(message, px); err == nil {
		resp, err := h.parseTickerResponseMessage(px)
		return resp, nil, err
	}

	// if the message is JSON, it is a subscription or ping response
	if err := json.Unmarshal(message, &msg); err != nil {
		return resp, nil, fmt.Errorf("failed to unmarshal JSON message %w", err)
	}

	if msg.Message == "PONG" {
		h.logger.Debug("received pong message")
		return resp, nil, nil
	}

	if strings.HasPrefix(msg.Message, MiniTickerChannel) {
		h.logger.Debug("subscribed to ticker channel", zap.String("instruments", msg.Message))
		return resp, nil, nil
	}

	return resp, nil, fmt.Errorf("invalid message type: %s", msg.Message)
}

// CreateMessages is used to create a message to send to the data provider. This is used to
// subscribe to the given ticker. This is called when the connection to the data provider is
// first established.
func (h *WebSocketHandler) CreateMessages(
	tickers []types.ProviderTicker,
) ([]handlers.WebsocketEncodedMessage, error) {
	if len(tickers) > MaxSubscriptionsPerConnection {
		return nil, fmt.Errorf("cannot subscribe to more than %d tickers per connection", MaxSubscriptionsPerConnection)
	}

	instruments := make([]string, 0)
	for _, ticker := range tickers {
		mexcTicker := fmt.Sprintf("%s%s%s", MiniTickerChannel, strings.ToUpper(ticker.GetOffChainTicker()), "@UTC+8")
		instruments = append(instruments, mexcTicker)
		h.cache.Add(ticker)
	}

	return h.NewSubscribeRequestMessage(instruments)
}

// HeartBeatMessages is used by the MEXC handler to send heart beat messages to the data provider.
// This is used to keep the connection alive when no messages are being sent from the data provider.
func (h *WebSocketHandler) HeartBeatMessages() ([]handlers.WebsocketEncodedMessage, error) {
	return NewPingRequestMessage()
}

// Copy is used to create a copy of the data handler.
func (h *WebSocketHandler) Copy() types.PriceWebSocketDataHandler {
	return &WebSocketHandler{
		logger: h.logger,
		ws:     h.ws,
		cache:  types.NewProviderTickers(),
	}
}
