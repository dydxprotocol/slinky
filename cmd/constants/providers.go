package constants

import (
	"github.com/dydxprotocol/slinky/oracle/config"
	"github.com/dydxprotocol/slinky/oracle/constants"
	"github.com/dydxprotocol/slinky/oracle/types"
	binanceapi "github.com/dydxprotocol/slinky/providers/apis/binance"
	bitstampapi "github.com/dydxprotocol/slinky/providers/apis/bitstamp"
	coinbaseapi "github.com/dydxprotocol/slinky/providers/apis/coinbase"
	"github.com/dydxprotocol/slinky/providers/apis/coingecko"
	"github.com/dydxprotocol/slinky/providers/apis/coinmarketcap"
	"github.com/dydxprotocol/slinky/providers/apis/defi/osmosis"
	"github.com/dydxprotocol/slinky/providers/apis/defi/raydium"
	"github.com/dydxprotocol/slinky/providers/apis/defi/uniswapv3"
	"github.com/dydxprotocol/slinky/providers/apis/dydx"
	krakenapi "github.com/dydxprotocol/slinky/providers/apis/kraken"
	"github.com/dydxprotocol/slinky/providers/apis/marketmap"
	"github.com/dydxprotocol/slinky/providers/apis/polymarket"
	"github.com/dydxprotocol/slinky/providers/volatile"
	binancews "github.com/dydxprotocol/slinky/providers/websockets/binance"
	"github.com/dydxprotocol/slinky/providers/websockets/bitfinex"
	"github.com/dydxprotocol/slinky/providers/websockets/bitstamp"
	"github.com/dydxprotocol/slinky/providers/websockets/bybit"
	"github.com/dydxprotocol/slinky/providers/websockets/coinbase"
	"github.com/dydxprotocol/slinky/providers/websockets/cryptodotcom"
	"github.com/dydxprotocol/slinky/providers/websockets/gate"
	"github.com/dydxprotocol/slinky/providers/websockets/huobi"
	"github.com/dydxprotocol/slinky/providers/websockets/kraken"
	"github.com/dydxprotocol/slinky/providers/websockets/kucoin"
	"github.com/dydxprotocol/slinky/providers/websockets/mexc"
	"github.com/dydxprotocol/slinky/providers/websockets/okx"
	mmtypes "github.com/dydxprotocol/slinky/service/clients/marketmap/types"
)

var (
	Providers = []config.ProviderConfig{
		// DEFI providers
		{
			Name: raydium.Name,
			API:  raydium.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: uniswapv3.ProviderNames[constants.ETHEREUM],
			API:  uniswapv3.DefaultETHAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: uniswapv3.ProviderNames[constants.BASE],
			API:  uniswapv3.DefaultBaseAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: osmosis.Name,
			API:  osmosis.DefaultAPIConfig,
			Type: types.ConfigType,
		},

		// Exchange API providers
		{
			Name: binanceapi.Name,
			API:  binanceapi.DefaultNonUSAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: bitstampapi.Name,
			API:  bitstampapi.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: coinbaseapi.Name,
			API:  coinbaseapi.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: coingecko.Name,
			API:  coingecko.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: coinmarketcap.Name,
			API:  coinmarketcap.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: krakenapi.Name,
			API:  krakenapi.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		{
			Name: volatile.Name,
			API:  volatile.DefaultAPIConfig,
			Type: types.ConfigType,
		},
		// Exchange WebSocket providers
		{
			Name:      binancews.Name,
			WebSocket: binancews.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      bitfinex.Name,
			WebSocket: bitfinex.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      bitstamp.Name,
			WebSocket: bitstamp.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      bybit.Name,
			WebSocket: bybit.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      coinbase.Name,
			WebSocket: coinbase.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      cryptodotcom.Name,
			WebSocket: cryptodotcom.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      gate.Name,
			WebSocket: gate.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      huobi.Name,
			WebSocket: huobi.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      kraken.Name,
			WebSocket: kraken.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      kucoin.Name,
			WebSocket: kucoin.DefaultWebSocketConfig,
			API:       kucoin.DefaultAPIConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      mexc.Name,
			WebSocket: mexc.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},
		{
			Name:      okx.Name,
			WebSocket: okx.DefaultWebSocketConfig,
			Type:      types.ConfigType,
		},

		// Polymarket provider
		{
			Name: polymarket.Name,
			API:  polymarket.DefaultAPIConfig,
			Type: types.ConfigType,
		},

		// MarketMap provider
		{
			Name: marketmap.Name,
			API:  marketmap.DefaultAPIConfig,
			Type: mmtypes.ConfigType,
		},
	}

	AlternativeMarketMapProviders = []config.ProviderConfig{
		{
			Name: dydx.Name,
			API:  dydx.DefaultAPIConfig,
			Type: mmtypes.ConfigType,
		},
		{
			Name: dydx.SwitchOverAPIHandlerName,
			API:  dydx.DefaultSwitchOverAPIConfig,
			Type: mmtypes.ConfigType,
		},
		{
			Name: dydx.ResearchAPIHandlerName,
			API:  dydx.DefaultResearchAPIConfig,
			Type: mmtypes.ConfigType,
		},
		{
			Name: dydx.ResearchCMCAPIHandlerName,
			API:  dydx.DefaultResearchCMCAPIConfig,
			Type: mmtypes.ConfigType,
		},
	}

	MarketMapProviderNames = map[string]struct{}{
		dydx.Name:                      {},
		dydx.SwitchOverAPIHandlerName:  {},
		dydx.ResearchAPIHandlerName:    {},
		dydx.ResearchCMCAPIHandlerName: {},
		marketmap.Name:                 {},
	}
)
