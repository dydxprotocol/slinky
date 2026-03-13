package stork

import (
	"encoding/json"
	"fmt"
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

// ParseResponse parses a single-asset Stork API response, verifies the
// aggregator's ECDSA signature, and returns the price scaled down by 10^18.
func (h *APIHandler) ParseResponse(
	tickers []types.ProviderTicker,
	resp *http.Response,
) types.PriceResponse {
	var result StorkPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to decode stork response: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}
	var (
		resolved   = make(types.ResolvedPrices)
		unresolved = make(types.UnResolvedPrices)
	)

	if len(tickers) == 0 {
		return types.NewPriceResponse(resolved, unresolved)
	}

	// The API is atomic (one ticker per request), so the response
	// corresponds to the first (and only) requested ticker.
	ticker := tickers[0]

	if !result.IsValid {
		unresolved[ticker] = providertypes.UnresolvedResult{
			ErrorWithCode: providertypes.NewErrorWithCode(
				fmt.Errorf("stork marked price invalid for %s", result.Market),
				providertypes.ErrorInvalidResponse,
			),
		}
		markRemainingUnresolved(tickers[1:], unresolved)
		return types.NewPriceResponse(resolved, unresolved)
	}
	if err := VerifyStorkSignature(result.StorkSignatureVerification.StorkSignedPrice); err != nil {
		unresolved[ticker] = providertypes.UnresolvedResult{
			ErrorWithCode: providertypes.NewErrorWithCode(
				fmt.Errorf("stork signature verification failed for %s: %w", result.Market, err),
				providertypes.ErrorInvalidResponse,
			),
		}
		markRemainingUnresolved(tickers[1:], unresolved)
		return types.NewPriceResponse(resolved, unresolved)
	}
	rawPrice, ok := new(big.Float).SetString(result.Price)
	if !ok {
		unresolved[ticker] = providertypes.UnresolvedResult{
			ErrorWithCode: providertypes.NewErrorWithCode(
				fmt.Errorf("failed to parse price %s for %s", result.Price, result.Market),
				providertypes.ErrorFailedToParsePrice,
			),
		}
		markRemainingUnresolved(tickers[1:], unresolved)
		return types.NewPriceResponse(resolved, unresolved)
	}
	price := new(big.Float).Quo(rawPrice, scaleFactor)
	resolved[ticker] = types.NewPriceResult(price, time.Now().UTC())
	markRemainingUnresolved(tickers[1:], unresolved)
	return types.NewPriceResponse(resolved, unresolved)
}

func markRemainingUnresolved(tickers []types.ProviderTicker, out types.UnResolvedPrices) {
	for _, t := range tickers {
		out[t] = providertypes.UnresolvedResult{
			ErrorWithCode: providertypes.NewErrorWithCode(
				fmt.Errorf("no response"),
				providertypes.ErrorNoResponse,
			),
		}
	}
}
