package stork

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	slinkymath "github.com/dydxprotocol/slinky/pkg/math"
	"github.com/dydxprotocol/slinky/providers/base/websocket/handlers"
)

type (
	// MessageType represents the type of message sent/received from the websocket.
	MessageType string
)

const (
	// SubscribeMessageType is the message type for subscription requests.
	SubscribeMessageType MessageType = "subscribe"

	// OraclePricesMessageType is the message type for oracle price updates.
	OraclePricesMessageType MessageType = "oracle_prices"
)

// SubscribeMessage represents a subscription request to Stork.
type SubscribeMessage struct {
	Type    string   `json:"type"`
	TraceID string   `json:"trace_id,omitempty"`
	Data    []string `json:"data"`
}

// NewSubscribeMessage creates subscription messages for the given assets.
func (h *WebSocketHandler) NewSubscribeMessage(assets []string) ([]handlers.WebsocketEncodedMessage, error) {
	numAssets := len(assets)
	if numAssets == 0 {
		return nil, fmt.Errorf("no assets to subscribe to")
	}

	numBatches := int(math.Ceil(float64(numAssets) / float64(h.ws.MaxSubscriptionsPerBatch)))
	msgs := make([]handlers.WebsocketEncodedMessage, numBatches)
	
	for i := 0; i < numBatches; i++ {
		start := i * h.ws.MaxSubscriptionsPerBatch
		end := slinkymath.Min((i+1)*h.ws.MaxSubscriptionsPerBatch, numAssets)
		
		msg := SubscribeMessage{
			Type: string(SubscribeMessageType),
			Data: assets[start:end],
		}
		
		bz, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal subscribe message: %w", err)
		}
		msgs[i] = bz
	}
	
	return msgs, nil
}

// BaseMessage is used to determine the message type.
type BaseMessage struct {
	Type string `json:"type"`
}

// OraclePricesMessage represents the oracle prices response from Stork.
type OraclePricesMessage struct {
	Type      string                       `json:"type"`
	TraceID   string                       `json:"trace_id,omitempty"`
	Data      map[string]OraclePriceData   `json:"data"`
}

// OraclePriceData contains the price data for a single asset.
type OraclePriceData struct {
	StorkSignedPrice StorkSignedPrice `json:"stork_signed_price"`
}

// StorkSignedPrice contains the aggregated signed price data.
type StorkSignedPrice struct {
	PublicKey         string `json:"public_key"`
	EncodedAssetID    string `json:"encoded_asset_id"`
	Price             string `json:"price"`
	TimestampNS       string `json:"timestampNs"`
	EVMSignature      string `json:"evm_signature"`
	StarknetSignature string `json:"starknet_signature"`
}

// GetAssetID extracts the asset ID from the encoded format.
func (s *StorkSignedPrice) GetAssetID() string {
	// The encoded asset ID format needs to be decoded to get the actual asset ID
	// This is a placeholder - actual implementation depends on Stork's encoding
	return strings.ToUpper(s.EncodedAssetID)
}
