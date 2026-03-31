package pyth

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	providertypes "github.com/dydxprotocol/slinky/providers/types"

	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/types"
)

var _ types.PriceAPIDataHandler = (*APIHandler)(nil)

// APIHandler implements the PriceAPIDataHandler interface for Pyth.
type APIHandler struct {
	api   config.APIConfig
	cache types.ProviderTickers
}

// NewAPIHandler returns a new Pyth PriceAPIDataHandler.
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

// CreateURL returns the URL used to fetch prices from the Pyth oracle service.
// Feed IDs are passed as a comma-separated "asset" query parameter, with
// "&provider=pyth" appended.
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

	return fmt.Sprintf(
		"%s?asset=%s&provider=pyth",
		h.api.Endpoints[0].URL,
		strings.Join(ids, ","),
	), nil
}

// ParseResponse parses a batch Pyth API response ({"data": [...]}), verifies
// each entry's Pyth Solana ed25519 signature, and returns the parsed prices.
//
// If the signed payload contains both price mantissa and exponent, the price is
// computed directly from signed data (mantissa * 10^exponent). If the payload
// only contains price (no exponent), the signature and feed ID are still
// verified, but the JSON price field is used as a fallback.
func (h *APIHandler) ParseResponse(
	tickers []types.ProviderTicker,
	resp *http.Response,
) types.PriceResponse {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to read pyth response body: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	var batch BatchPriceResponse
	if err := json.Unmarshal(body, &batch); err != nil {
		return types.NewPriceResponseWithErr(
			tickers,
			providertypes.NewErrorWithCode(
				fmt.Errorf("failed to decode pyth response: %w", err),
				providertypes.ErrorFailedToDecode,
			),
		)
	}

	byMarket := make(map[string]PriceResponse, len(batch.Data))
	for _, item := range batch.Data {
		byMarket[item.Market] = item
	}

	var (
		resolved   = make(types.ResolvedPrices)
		unresolved = make(types.UnResolvedPrices)
	)

	for _, ticker := range tickers {
		offChain := ticker.GetOffChainTicker()

		item, ok := byMarket[offChain]
		if !ok {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("no pyth response for feed %s", offChain),
					providertypes.ErrorNoResponse,
				),
			}
			continue
		}

		feedID, err := strconv.ParseUint(offChain, 10, 32)
		if err != nil {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("invalid feed ID %q: %w", offChain, err),
					providertypes.ErrorInvalidResponse,
				),
			}
			continue
		}

		feed, err := VerifyAndExtractFeed(item.PythSolanaPayload, uint32(feedID)) //nolint:gosec // bounded by ParseUint 32-bit
		if err != nil {
			unresolved[ticker] = providertypes.UnresolvedResult{
				ErrorWithCode: providertypes.NewErrorWithCode(
					fmt.Errorf("pyth payload verification failed for feed %s: %w", offChain, err),
					providertypes.ErrorInvalidResponse,
				),
			}
			continue
		}

		var price *big.Float
		if feed.HasPrice && feed.HasExponent {
			price, err = feed.ComputePrice()
			if err != nil {
				unresolved[ticker] = providertypes.UnresolvedResult{
					ErrorWithCode: providertypes.NewErrorWithCode(
						fmt.Errorf("failed to compute price from signed payload for feed %s: %w", offChain, err),
						providertypes.ErrorFailedToParsePrice,
					),
				}
				continue
			}
		} else {
			// Exponent not in the signed payload; fall back to JSON price.
			// Signature and feed ID have already been verified above.
			price, ok = new(big.Float).SetString(item.Price)
			if !ok {
				unresolved[ticker] = providertypes.UnresolvedResult{
					ErrorWithCode: providertypes.NewErrorWithCode(
						fmt.Errorf("failed to parse JSON price %q for feed %s", item.Price, offChain),
						providertypes.ErrorFailedToParsePrice,
					),
				}
				continue
			}
		}

		resolved[ticker] = types.NewPriceResult(price, time.Now().UTC())
	}

	return types.NewPriceResponse(resolved, unresolved)
}
