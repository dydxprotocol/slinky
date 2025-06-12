package stork

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	providertypes "github.com/dydxprotocol/slinky/providers/types"
	"go.uber.org/zap"

	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/pkg/math"
)

// parseOraclePricesMessage parses an oracle prices message from Stork.
func (h *WebSocketHandler) parseOraclePricesMessage(msg OraclePricesMessage) (types.PriceResponse, error) {
	var (
		resolved   = make(types.ResolvedPrices)
		unResolved = make(types.UnResolvedPrices)
	)

	for assetID, priceData := range msg.Data {
		// Get the ticker from the asset ID
		ticker, ok := h.assetIDToTicker[assetID]
		if !ok {
			h.logger.Debug("received price for unknown asset", zap.String("asset_id", assetID))
			continue
		}

		// Parse the price
		price, err := h.parsePrice(priceData.StorkSignedPrice.Price)
		if err != nil {
			unResolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(err, providertypes.ErrorFailedToParsePrice),
			}
			continue
		}

		// Parse the timestamp (convert from nanoseconds)
		timestamp, err := h.parseTimestamp(priceData.StorkSignedPrice.TimestampNS)
		if err != nil {
			unResolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(err, providertypes.ErrorInvalidResponse),
			}
			continue
		}

		resolved[ticker] = types.NewPriceResult(price, timestamp)
	}

	return types.NewPriceResponse(resolved, unResolved), nil
}

// parsePrice converts the Stork price string to a big.Float.
func (h *WebSocketHandler) parsePrice(priceStr string) (*big.Float, error) {
	// Stork prices are typically large integers that need to be scaled
	// The exact scaling factor should be determined from Stork documentation
	price, err := math.Float64StringToBigFloat(priceStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price string: %w", err)
	}

	// Apply scaling if needed (this is a placeholder - actual scaling depends on Stork's format)
	// For example, if prices are in 18 decimals:
	// divisor := new(big.Float).SetInt(big.NewInt(1).Exp(big.NewInt(10), big.NewInt(18), nil))
	// price.Quo(price, divisor)

	return price, nil
}

// parseTimestamp converts nanosecond timestamp string to time.Time.
func (h *WebSocketHandler) parseTimestamp(timestampNS string) (time.Time, error) {
	ns, err := strconv.ParseInt(timestampNS, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return time.Unix(0, ns).UTC(), nil
}

// getAssetIDFromTicker converts a provider ticker to Stork asset ID format.
func (h *WebSocketHandler) getAssetIDFromTicker(ticker types.ProviderTicker) string {
	// Convert the off-chain ticker format (e.g., "BTC/USD") to Stork format (e.g., "BTCUSD")
	offChainTicker := ticker.GetOffChainTicker()

	// Remove any separators and convert to uppercase
	assetID := strings.ReplaceAll(offChainTicker, "/", "")
	assetID = strings.ReplaceAll(assetID, "-", "")
	assetID = strings.ToUpper(assetID)

	// Check if there's a custom mapping in the config
	// This would need to be added to the config structure if needed

	return assetID
}
