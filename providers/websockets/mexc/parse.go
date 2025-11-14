package mexc

import (
	"fmt"
	"time"

	providertypes "github.com/dydxprotocol/slinky/providers/types"

	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/pkg/math"
	"github.com/dydxprotocol/slinky/providers/websockets/mexc/pb"
)

func (h *WebSocketHandler) parseTickerResponseMessage(
	px *pb.PublicMiniTickerV3Api,
) (types.PriceResponse, error) {
	var (
		resolved   = make(types.ResolvedPrices)
		unResolved = make(types.UnResolvedPrices)
	)

	ticker, ok := h.cache.FromOffChainTicker(px.Symbol)
	if !ok {
		return types.NewPriceResponse(resolved, unResolved),
			fmt.Errorf("unknown ticker %s", px.Symbol)
	}

	price, err := math.Float64StringToBigFloat(px.Price)
	if err != nil {
		unResolved[ticker] = providertypes.UnresolvedResult{
			ErrorWithCode: providertypes.NewErrorWithCode(err, providertypes.ErrorFailedToParsePrice),
		}
		return types.NewPriceResponse(resolved, unResolved), err
	}

	resolved[ticker] = types.NewPriceResult(price, time.Now().UTC())
	return types.NewPriceResponse(resolved, unResolved), nil
}
