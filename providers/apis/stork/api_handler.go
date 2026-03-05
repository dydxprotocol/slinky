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

	return fmt.Sprintf("%s?assets=%s", h.api.Endpoints[0].URL, strings.Join(ids, ",")), nil
}

// scaleFactor is 10^18 as a big.Float, used to divide Stork's scaled price values.
var scaleFactor, _ = new(big.Float).SetString("1000000000000000000")

// ParseResponse parses the response from the Stork API. Stork returns prices as
// integer strings scaled by 10^18, so we divide to get the actual price.
func (h *APIHandler) ParseResponse(
	tickers []types.ProviderTicker,
	resp *http.Response,
) types.PriceResponse {
	var result LatestPricesResponse
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

	for assetID, assetPrice := range result.Data {
		ticker, ok := h.cache.FromOffChainTicker(assetID)
		if !ok {
			continue
		}

		rawPrice, ok := new(big.Float).SetString(assetPrice.Price)
		if !ok {
			wErr := fmt.Errorf("failed to parse price %s for %s", assetPrice.Price, assetID)
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(wErr, providertypes.ErrorFailedToParsePrice),
			}
			continue
		}

		price := new(big.Float).Quo(rawPrice, scaleFactor)
		resolved[ticker] = types.NewPriceResult(price, time.Now().UTC())
	}

	for _, ticker := range tickers {
		_, resolvedOk := resolved[ticker]
		_, unresolvedOk := unresolved[ticker]

		if !resolvedOk && !unresolvedOk {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorNoResponse),
			}
		}
	}

	return types.NewPriceResponse(resolved, unresolved)
}
