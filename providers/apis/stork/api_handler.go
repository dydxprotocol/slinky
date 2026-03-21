package stork

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	providertypes "github.com/dydxprotocol/slinky/providers/types"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
)

var _ types.PriceAPIDataHandler = (*APIHandler)(nil)

// APIHandler implements the PriceAPIDataHandler interface for Stork.
type APIHandler struct {
	logger *zap.Logger
	api    config.APIConfig
	cache  types.ProviderTickers
}

// NewAPIHandler returns a new Stork PriceAPIDataHandler.
func NewAPIHandler(
	logger *zap.Logger,
	api config.APIConfig,
) (types.PriceAPIDataHandler, error) {
	if api.Name != Name {
		return nil, fmt.Errorf("expected api config name %s, got %s", Name, api.Name)
	}

	if !api.Enabled {
		return nil, fmt.Errorf("api config for %s is not enabled", Name)
	}

	if err := api.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid api config for %s: %w", Name, err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &APIHandler{
		logger: logger.With(zap.String("provider", Name)),
		api:    api,
		cache:  types.NewProviderTickers(),
	}, nil
}

// CreateURL returns the URL used to fetch prices from the Stork API. The asset IDs
// are passed as a comma-separated query parameter.
func (h *APIHandler) CreateURL(
	tickers []types.ProviderTicker,
) (string, error) {
	if len(tickers) == 0 {
		return "", fmt.Errorf("no tickers provided")
	}

	ids := make([]string, len(tickers))
	for i, ticker := range tickers {
		ids[i] = ticker.GetOffChainTicker()
		h.cache.Add(ticker)
	}

	url := fmt.Sprintf("%s?asset=%s", h.api.Endpoints[0].URL, strings.Join(ids, ","))
	h.logger.Debug("created request URL",
		zap.String("url", url),
		zap.Int("num_tickers", len(tickers)),
		zap.Strings("tickers", ids),
	)
	return url, nil
}

// scaleFactor is 10^18 as a big.Float, used to divide Stork's scaled price values.
var scaleFactor, _ = new(big.Float).SetString("1000000000000000000")

// ParseResponse parses a batch Stork API response ({"data": [...]}), verifies
// each aggregator's ECDSA signature, and returns prices scaled down by 10^18.
func (h *APIHandler) ParseResponse(
	tickers []types.ProviderTicker,
	resp *http.Response,
) types.PriceResponse {
	h.logger.Debug("parsing response",
		zap.Int("num_tickers", len(tickers)),
		zap.Int("http_status", resp.StatusCode),
	)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Error("failed to read response body", zap.Error(err))
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to read stork response body: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	h.logger.Debug("raw response body", zap.String("body", string(body)))

	var batch BatchPriceResponse
	if err := json.Unmarshal(body, &batch); err != nil {
		h.logger.Error("failed to decode batch response",
			zap.Error(err),
			zap.String("body", string(body)),
		)
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to decode stork response: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	h.logger.Debug("decoded batch response",
		zap.Int("num_items", len(batch.Data)),
	)

	// Index response items by normalized market name (uppercase, no dashes)
	// so that "XAU-USD" matches off_chain_ticker "XAUUSD".
	byMarket := make(map[string]PriceResponse, len(batch.Data))
	for _, item := range batch.Data {
		key := normalizeMarket(item.Market)
		byMarket[key] = item
		h.logger.Debug("batch item",
			zap.String("market", item.Market),
			zap.String("normalized_key", key),
			zap.String("price", item.Price),
		)
	}

	var (
		resolved   = make(types.ResolvedPrices)
		unresolved = make(types.UnResolvedPrices)
	)

	for _, ticker := range tickers {
		offChain := ticker.GetOffChainTicker()

		item, ok := byMarket[normalizeMarket(offChain)]
		if !ok {
			h.logger.Warn("no response for ticker in batch",
				zap.String("ticker", offChain),
				zap.Int("batch_size", len(byMarket)),
			)
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("no stork response for ticker %s", offChain),
					providertypes.ErrorNoResponse,
				),
			}
			continue
		}

		if err := VerifyStorkSignature(item.StorkSignatureVerification.StorkSignedPrice); err != nil {
			h.logger.Error("signature verification failed",
				zap.String("market", item.Market),
				zap.String("ticker", offChain),
				zap.Error(err),
			)
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("stork signature verification failed for %s: %w", item.Market, err),
					providertypes.ErrorInvalidResponse,
				),
			}
			continue
		}
		h.logger.Debug("signature verification passed",
			zap.String("market", item.Market),
		)

		rawPrice, ok := new(big.Float).SetString(item.Price)
		if !ok {
			h.logger.Error("failed to parse price string",
				zap.String("market", item.Market),
				zap.String("raw_price", item.Price),
			)
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("failed to parse price %s for %s", item.Price, item.Market),
					providertypes.ErrorFailedToParsePrice,
				),
			}
			continue
		}

		price := new(big.Float).Quo(rawPrice, scaleFactor)
		resolved[ticker] = types.NewPriceResult(price, time.Now().UTC())

		h.logger.Info("resolved stork price",
			zap.String("market", item.Market),
			zap.String("ticker", offChain),
			zap.String("raw_price", item.Price),
			zap.String("scaled_price", price.Text('f', 18)),
		)
	}

	return types.NewPriceResponse(resolved, unresolved)
}

// normalizeMarket strips dashes/underscores and uppercases the market name
// so that "XAU-USD" and "XAUUSD" both become "XAUUSD".
func normalizeMarket(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return strings.ToUpper(s)
}
