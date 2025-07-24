package stork

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/providers/base/websocket/handlers"
)

var _ types.PriceWebSocketDataHandler = (*WebSocketHandler)(nil)

// WebSocketHandler implements the WebSocketDataHandler interface for Stork.
type WebSocketHandler struct {
	logger *zap.Logger

	// ws is the config for the Stork websocket.
	ws config.WebSocketConfig
	
	// cache maintains the latest set of tickers seen by the handler.
	cache types.ProviderTickers
	
	// assetIDToTicker maps Stork asset IDs to provider tickers.
	assetIDToTicker map[string]types.ProviderTicker
	
	// tickerToAssetID maps provider tickers to Stork asset IDs.
	tickerToAssetID map[types.ProviderTicker]string
}

// NewWebSocketDataHandler returns a new Stork PriceWebSocketDataHandler.
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
		logger:          logger,
		ws:              ws,
		cache:           types.NewProviderTickers(),
		assetIDToTicker: make(map[string]types.ProviderTicker),
		tickerToAssetID: make(map[types.ProviderTicker]string),
	}, nil
}

// HandleMessage handles messages received from Stork websocket.
func (h *WebSocketHandler) HandleMessage(
	message []byte,
) (types.PriceResponse, []handlers.WebsocketEncodedMessage, error) {
	var (
		resp types.PriceResponse
		msg  BaseMessage
	)

	if err := json.Unmarshal(message, &msg); err != nil {
		return resp, nil, fmt.Errorf("failed to unmarshal base message: %w", err)
	}

	switch MessageType(msg.Type) {
	case OraclePricesMessageType:
		h.logger.Debug("received oracle prices message")

		var pricesMsg OraclePricesMessage
		if err := json.Unmarshal(message, &pricesMsg); err != nil {
			return resp, nil, fmt.Errorf("failed to unmarshal oracle prices message: %w", err)
		}

		resp, err := h.parseOraclePricesMessage(pricesMsg)
		return resp, nil, err

	default:
		h.logger.Debug("received unknown message type", zap.String("type", msg.Type))
		return resp, nil, nil
	}
}

// CreateMessages creates subscription messages for the given tickers.
func (h *WebSocketHandler) CreateMessages(
	tickers []types.ProviderTicker,
) ([]handlers.WebsocketEncodedMessage, error) {
	assets := make([]string, 0, len(tickers))

	for _, ticker := range tickers {
		// Map the ticker to Stork asset ID format
		assetID := h.getAssetIDFromTicker(ticker)
		assets = append(assets, assetID)
		
		// Update the mappings
		h.cache.Add(ticker)
		h.assetIDToTicker[assetID] = ticker
		h.tickerToAssetID[ticker] = assetID
	}

	return h.NewSubscribeMessage(assets)
}

// HeartBeatMessages is not used for Stork.
func (h *WebSocketHandler) HeartBeatMessages() ([]handlers.WebsocketEncodedMessage, error) {
	return nil, nil
}

// Copy creates a copy of the WebSocketHandler.
func (h *WebSocketHandler) Copy() types.PriceWebSocketDataHandler {
	return &WebSocketHandler{
		logger:          h.logger,
		ws:              h.ws,
		cache:           types.NewProviderTickers(),
		assetIDToTicker: make(map[string]types.ProviderTicker),
		tickerToAssetID: make(map[types.ProviderTicker]string),
	}
}
