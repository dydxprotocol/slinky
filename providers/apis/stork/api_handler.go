package stork

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	providertypes "github.com/dydxprotocol/slinky/providers/types"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
)

var _ types.PriceAPIDataHandler = (*APIHandler)(nil)

// APIHandler implements the PriceAPIDataHandler interface for Stork.
type APIHandler struct {
	api   config.APIConfig
	cache types.ProviderTickers
}

// NewAPIHandler returns a new Stork PriceAPIDataHandler.
func NewAPIHandler(
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

	return &APIHandler{
		api:   api,
		cache: types.NewProviderTickers(),
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

	return fmt.Sprintf("%s?asset=%s", h.api.Endpoints[0].URL, strings.Join(ids, ",")), nil
}

// scaleFactor is 10^18 as a big.Float, used to divide Stork's scaled price values.
var scaleFactor, _ = new(big.Float).SetString("1000000000000000000")

// ParseResponse parses a batch Stork API response ({"data": [...]}), verifies
// each aggregator's ECDSA signature, and returns prices scaled down by 10^18.
func (h *APIHandler) ParseResponse(
	tickers []types.ProviderTicker,
	resp *http.Response,
) types.PriceResponse {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to read stork response body: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	var batch BatchPriceResponse
	if err := json.Unmarshal(body, &batch); err != nil {
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to decode stork response: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	// Index response items by normalized market name (uppercase, no dashes)
	// so that "XAU-USD" matches off_chain_ticker "XAUUSD".
	byMarket := make(map[string]PriceResponse, len(batch.Data))
	for _, item := range batch.Data {
		byMarket[normalizeMarket(item.Market)] = item
	}

	var (
		resolved   = make(types.ResolvedPrices)
		unresolved = make(types.UnResolvedPrices)
	)

	for _, ticker := range tickers {
		offChain := ticker.GetOffChainTicker()

		item, ok := byMarket[normalizeMarket(offChain)]
		if !ok {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("no stork response for ticker %s", offChain),
					providertypes.ErrorNoResponse,
				),
			}
			continue
		}

		if err := VerifyStorkSignature(item.StorkSignatureVerification.StorkSignedPrice); err != nil {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("stork signature verification failed for %s: %w", item.Market, err),
					providertypes.ErrorInvalidResponse,
				),
			}
			continue
		}

		rawPrice, ok := new(big.Float).SetString(item.Price)
		if !ok {
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
