package polymarket

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
	providertypes "github.com/dydxprotocol/slinky/providers/types"
)

const (
	// Name is the name of the Polymarket provider.
	Name = "polymarket_api"

	// URL is the default base URL of the Polymarket CLOB API. It uses the `markets` endpoint with a given market ID.
	URL = "https://clob.polymarket.com/midpoint?token_id=%s"

	// priceAdjustmentMin is the value the price gets set to in the event of price == 0.
	priceAdjustmentMin = 0.0001
)

var _ types.PriceAPIDataHandler = (*APIHandler)(nil)

// APIHandler implements the PriceAPIDataHandler interface for Polymarket, which can be used
// by a base provider. The handler fetches data from the `markets` endpoint.
type APIHandler struct {
	api config.APIConfig
}

// NewAPIHandler returns a new Polymarket PriceAPIDataHandler.
func NewAPIHandler(api config.APIConfig) (types.PriceAPIDataHandler, error) {
	if api.Name != Name {
		return nil, fmt.Errorf("expected api config name %s, got %s", Name, api.Name)
	}

	if !api.Enabled {
		return nil, fmt.Errorf("api config for %s is not enabled", Name)
	}

	if err := api.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid api config for %s: %w", Name, err)
	}

	if len(api.Endpoints) != 1 {
		return nil, fmt.Errorf("invalid polymarket endpoint config: expected 1 endpoint got %d", len(api.Endpoints))
	}

	return &APIHandler{
		api: api,
	}, nil
}

// CreateURL returns the URL that is used to fetch data from the Polymarket API for the
// given ticker. Since the markets endpoint's price data is automatically denominated in USD, only one ID is expected to be passed
// into this method.
func (h APIHandler) CreateURL(ids []types.ProviderTicker) (string, error) {
	if len(ids) != 1 {
		return "", fmt.Errorf("expected 1 ticker, got %d", len(ids))
	}
	_, tokenID, err := getMarketAndTokenFromTicker(ids[0])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(h.api.Endpoints[0].URL, tokenID), nil
}

type PriceResponse struct {
	Mid *float64 `json:"mid,string"`
}

// ParseResponse parses the HTTP response from the markets endpoint of the Polymarket API endpoint and returns
// the resulting data.
func (h APIHandler) ParseResponse(ids []types.ProviderTicker, response *http.Response) types.PriceResponse {
	if len(ids) != 1 {
		return priceResponseError(ids, fmt.Errorf("expected 1 ticker, got %d", len(ids)), providertypes.ErrorInvalidResponse)
	}
	var result PriceResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return priceResponseError(ids, fmt.Errorf("failed to decode price response"), providertypes.ErrorFailedToDecode)
	}

	if result.Mid == nil {
		return priceResponseError(ids, fmt.Errorf("unable to get price from response"), providertypes.ErrorFailedToDecode)
	}

	price := new(big.Float).SetFloat64(*result.Mid)

	// switch price to priceAdjustmentMin if its 0.00.
	if big.NewFloat(0.00).Cmp(price) == 0 {
		price = new(big.Float).SetFloat64(priceAdjustmentMin)
	}

	resolved := types.ResolvedPrices{
		ids[0]: types.NewPriceResult(price, time.Now().UTC()),
	}

	return types.NewPriceResponse(resolved, nil)
}

func priceResponseError(ids []types.ProviderTicker, err error, code providertypes.ErrorCode) providertypes.GetResponse[types.ProviderTicker, *big.Float] {
	return types.NewPriceResponseWithErr(
		ids,
		providertypes.NewErrorWithCode(err, code),
	)
}

func getMarketAndTokenFromTicker(t types.ProviderTicker) (marketID string, tokenID string, err error) {
	split := strings.Split(t.GetOffChainTicker(), "/")
	if len(split) != 2 {
		return "", "", fmt.Errorf("expected ticker format market_id/token_id, got: %s", t.GetOffChainTicker())
	}
	return split[0], split[1], nil
}
