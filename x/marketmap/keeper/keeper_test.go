package keeper_test

import (
	"testing"

	"github.com/skip-mev/chaintestutil/sample"

	oraclekeeper "github.com/dydxprotocol/slinky/x/oracle/keeper"
	oracletypes "github.com/dydxprotocol/slinky/x/oracle/types"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/stretchr/testify/suite"

	slinkytypes "github.com/dydxprotocol/slinky/pkg/types"
	"github.com/dydxprotocol/slinky/x/marketmap/keeper"
	"github.com/dydxprotocol/slinky/x/marketmap/types"
)

var r = sample.Rand()

type KeeperTestSuite struct {
	suite.Suite

	ctx sdk.Context

	// Keeper variables
	authority         sdk.AccAddress
	marketAuthorities []string
	admin             string

	keeper       *keeper.Keeper
	oracleKeeper oraclekeeper.Keeper
}

func (s *KeeperTestSuite) initKeeper() *keeper.Keeper {
	return s.initKeeperWithHooks(types.MultiMarketMapHooks{
		s.oracleKeeper.Hooks(),
	})
}

func (s *KeeperTestSuite) initKeeperWithHooks(hooks types.MarketMapHooks) *keeper.Keeper {
	mmKey := storetypes.NewKVStoreKey(types.StoreKey)
	oracleKey := storetypes.NewKVStoreKey(oracletypes.StoreKey)
	mmSS := runtime.NewKVStoreService(mmKey)
	oracleSS := runtime.NewKVStoreService(oracleKey)
	encCfg := moduletestutil.MakeTestEncodingConfig()

	keys := map[string]*storetypes.KVStoreKey{
		types.StoreKey:       mmKey,
		oracletypes.StoreKey: oracleKey,
	}

	transientKeys := map[string]*storetypes.TransientStoreKey{
		types.StoreKey:       storetypes.NewTransientStoreKey("transient_mm"),
		oracletypes.StoreKey: storetypes.NewTransientStoreKey("transient_oracle"),
	}

	s.authority = sdk.AccAddress("authority")
	s.ctx = testutil.DefaultContextWithKeys(keys, transientKeys, nil).WithBlockHeight(10)

	k := keeper.NewKeeper(mmSS, encCfg.Codec, s.authority)
	s.Require().NoError(k.SetLastUpdated(s.ctx, uint64(s.ctx.BlockHeight()))) //nolint:gosec

	s.admin = sample.Address(r)
	s.marketAuthorities = []string{sample.Address(r), sample.Address(r), sample.Address(r)}

	params := types.Params{
		MarketAuthorities: s.marketAuthorities,
		Admin:             s.admin,
	}
	s.Require().NoError(k.SetParams(s.ctx, params))

	s.oracleKeeper = oraclekeeper.NewKeeper(oracleSS, encCfg.Codec, k, s.authority)
	k.SetHooks(hooks)

	s.Require().NotPanics(func() {
		s.oracleKeeper.InitGenesis(s.ctx, *oracletypes.DefaultGenesisState())
	})

	return k
}

func (s *KeeperTestSuite) SetupTest() {
	s.keeper = s.initKeeper()
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

var (
	btcusdt = types.Market{
		Ticker: types.Ticker{
			CurrencyPair: slinkytypes.CurrencyPair{
				Base:  "BITCOIN",
				Quote: "USDT",
			},
			Decimals:         8,
			MinProviderCount: 1,
		},
		ProviderConfigs: []types.ProviderConfig{
			{
				Name:           "kucoin",
				OffChainTicker: "btc-usdt",
			},
		},
	}

	usdtusd = types.Market{
		Ticker: types.Ticker{
			CurrencyPair: slinkytypes.CurrencyPair{
				Base:  "USDT",
				Quote: "USD",
			},
			Decimals:         8,
			MinProviderCount: 1,
		},
		ProviderConfigs: []types.ProviderConfig{
			{
				Name:           "kucoin",
				OffChainTicker: "usdt-usd",
			},
		},
	}

	usdcusd = types.Market{
		Ticker: types.Ticker{
			CurrencyPair: slinkytypes.CurrencyPair{
				Base:  "USDC",
				Quote: "USD",
			},
			Decimals:         8,
			MinProviderCount: 1,
		},
		ProviderConfigs: []types.ProviderConfig{
			{
				Name:           "kucoin",
				OffChainTicker: "usdc-usd",
			},
		},
	}

	ethusdt = types.Market{
		Ticker: types.Ticker{
			CurrencyPair: slinkytypes.CurrencyPair{
				Base:  "ETHEREUM",
				Quote: "USDT",
			},
			Decimals:         8,
			MinProviderCount: 1,
		},
		ProviderConfigs: []types.ProviderConfig{
			{
				Name:           "kucoin",
				OffChainTicker: "eth-usdt",
			},
		},
	}

	markets = []types.Market{
		btcusdt,
		usdcusd,
		usdtusd,
		ethusdt,
	}

	marketsKeySorted = []types.Market{
		btcusdt,
		ethusdt,
		usdcusd,
		usdtusd,
	}

	marketsMap = map[string]types.Market{
		btcusdt.Ticker.String(): btcusdt,
		usdcusd.Ticker.String(): usdcusd,
		usdtusd.Ticker.String(): usdtusd,
		ethusdt.Ticker.String(): ethusdt,
	}
)

func (s *KeeperTestSuite) TestGets() {
	s.Run("get empty market map", func() {
		got, err := s.keeper.GetAllMarkets(s.ctx)
		s.Require().NoError(err)
		s.Require().Equal(map[string]types.Market{}, got)
	})

	s.Run("setup initial markets", func() {
		for _, market := range markets {
			s.Require().NoError(s.keeper.CreateMarket(s.ctx, market))
		}

		s.Run("unable to set markets again", func() {
			for _, market := range markets {
				s.Require().ErrorIs(s.keeper.CreateMarket(s.ctx, market), types.NewMarketAlreadyExistsError(types.TickerString(market.Ticker.String())))
			}
		})

		s.Require().NoError(s.keeper.ValidateState(s.ctx, markets))
	})

	s.Run("get all tickers map", func() {
		got, err := s.keeper.GetAllMarkets(s.ctx)
		s.Require().NoError(err)

		s.Require().Equal(len(markets), len(got))
		s.Require().Equal(marketsMap, got)
	})

	s.Run("get all tickers list", func() {
		got, err := s.keeper.GetAllMarketsList(s.ctx)
		s.Require().NoError(err)

		s.Require().Equal(len(marketsKeySorted), len(got))
		s.Require().Equal(marketsKeySorted, got)
	})

	s.Run("get all tickers list - deterministic", func() {
		for range 100 {
			got, err := s.keeper.GetAllMarketsList(s.ctx)
			s.Require().NoError(err)

			s.Require().Equal(len(marketsKeySorted), len(got))
			s.Require().Equal(marketsKeySorted, got)
		}
	})
}

func (s *KeeperTestSuite) TestSetParams() {
	params := types.DefaultParams()

	s.Run("can set and get params", func() {
		err := s.keeper.SetParams(s.ctx, params)
		s.Require().NoError(err)

		params2, err := s.keeper.GetParams(s.ctx)
		s.Require().NoError(err)
		s.Require().Equal(params, params2)
	})
}

func (s *KeeperTestSuite) TestInvalidCreate() {
	// invalid market with a normalize pair not in state
	invalidMarket := types.Market{
		Ticker: types.Ticker{
			CurrencyPair: slinkytypes.CurrencyPair{
				Base:  "BITCOIN",
				Quote: "USDT",
			},
			Decimals:         8,
			MinProviderCount: 1,
		},
		ProviderConfigs: []types.ProviderConfig{
			{
				Name:            "kucoin",
				OffChainTicker:  "btc-usdt",
				NormalizeByPair: &slinkytypes.CurrencyPair{Base: "invalid", Quote: "pair"},
			},
		},
	}

	s.Require().NoError(s.keeper.CreateMarket(s.ctx, invalidMarket))
	s.Require().Error(s.keeper.ValidateState(s.ctx, []types.Market{invalidMarket}))
}

func (s *KeeperTestSuite) TestInvalidUpdate() {
	// create a valid market
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, btcusdt))

	// invalid market with a normalize pair not in state
	invalidMarket := btcusdt
	invalidMarket.ProviderConfigs = append(invalidMarket.ProviderConfigs, types.ProviderConfig{
		Name:            "huobi",
		OffChainTicker:  "btc-usdt",
		NormalizeByPair: &slinkytypes.CurrencyPair{Base: "invalid", Quote: "pair"},
	})

	s.Require().NoError(s.keeper.UpdateMarket(s.ctx, invalidMarket))
	s.Require().Error(s.keeper.ValidateState(s.ctx, []types.Market{invalidMarket}))
}

func (s *KeeperTestSuite) TestValidUpdate() {
	// create a valid markets
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, btcusdt))
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, ethusdt))

	// valid market with a normalize pair that is in state
	validMarket := btcusdt
	validMarket.ProviderConfigs = append(validMarket.ProviderConfigs, types.ProviderConfig{
		Name:            "huobi",
		OffChainTicker:  "btc-usdt",
		NormalizeByPair: &ethusdt.Ticker.CurrencyPair,
	})

	s.Require().NoError(s.keeper.UpdateMarket(s.ctx, validMarket))
	s.Require().NoError(s.keeper.ValidateState(s.ctx, []types.Market{validMarket}))
}

func (s *KeeperTestSuite) TestInvalidUpdateDisabledNormalizeBy() {
	marketBTCUSDT := btcusdt
	marketETHUSDT := ethusdt

	// create a valid markets
	marketBTCUSDT.Ticker.Enabled = true
	marketETHUSDT.Ticker.Enabled = false

	s.Require().NoError(s.keeper.CreateMarket(s.ctx, marketBTCUSDT))
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, marketETHUSDT))

	// invalid market with a normalize pair that is in state but disabled
	invalidMarket := marketBTCUSDT
	invalidMarket.ProviderConfigs = append(invalidMarket.ProviderConfigs, types.ProviderConfig{
		Name:            "huobi",
		OffChainTicker:  "btc-usdt",
		NormalizeByPair: &marketETHUSDT.Ticker.CurrencyPair,
	})

	s.Require().NoError(s.keeper.UpdateMarket(s.ctx, invalidMarket))
	s.Require().Error(s.keeper.ValidateState(s.ctx, []types.Market{invalidMarket}))
}

func (s *KeeperTestSuite) TestInvalidCreateDisabledNormalizeBy() {
	marketBTCUSDT := btcusdt
	marketETHUSDT := ethusdt

	// create a valid markets
	marketBTCUSDT.Ticker.Enabled = true
	marketETHUSDT.Ticker.Enabled = false

	s.Require().NoError(s.keeper.CreateMarket(s.ctx, marketETHUSDT))

	// invalid market with a normalize pair that is in state but disabled
	invalidMarket := marketBTCUSDT
	invalidMarket.ProviderConfigs = append(invalidMarket.ProviderConfigs, types.ProviderConfig{
		Name:            "huobi",
		OffChainTicker:  "btc-usdt",
		NormalizeByPair: &marketETHUSDT.Ticker.CurrencyPair,
	})

	s.Require().NoError(s.keeper.CreateMarket(s.ctx, invalidMarket))
	s.Require().Error(s.keeper.ValidateState(s.ctx, []types.Market{invalidMarket}))
}

func (s *KeeperTestSuite) TestDeleteMarket() {
	// create a valid markets
	btcCopy := btcusdt
	btcCopy.Ticker.Enabled = true
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, btcCopy))

	// invalid delete will return nil - idempotent
	deleted, err := s.keeper.DeleteMarket(s.ctx, "foobar")
	s.Require().NoError(err)
	s.Require().False(deleted)

	// cannot delete enabled markets
	deleted, err = s.keeper.DeleteMarket(s.ctx, btcCopy.Ticker.String())
	s.Require().Error(err)
	s.Require().False(deleted)

	// disable market
	btcCopy.Ticker.Enabled = false
	s.Require().NoError(s.keeper.UpdateMarket(s.ctx, btcCopy))

	// delete disabled markets
	deleted, err = s.keeper.DeleteMarket(s.ctx, btcCopy.Ticker.String())
	s.Require().NoError(err)
	s.Require().True(deleted)

	_, err = s.keeper.GetMarket(s.ctx, btcCopy.Ticker.String())
	s.Require().Error(err)
}

func (s *KeeperTestSuite) TestEnableDisableMarket() {
	// create a valid markets
	s.Require().NoError(s.keeper.CreateMarket(s.ctx, btcusdt))

	// invalid enable/disable fails
	s.Require().Error(s.keeper.EnableMarket(s.ctx, "foobar"))
	s.Require().Error(s.keeper.DisableMarket(s.ctx, "foobar"))

	// valid enable works
	s.Require().NoError(s.keeper.EnableMarket(s.ctx, btcusdt.Ticker.String()))
	market, err := s.keeper.GetMarket(s.ctx, btcusdt.Ticker.String())
	s.Require().NoError(err)
	s.Require().True(market.Ticker.Enabled)

	// valid disable works
	s.Require().NoError(s.keeper.DisableMarket(s.ctx, btcusdt.Ticker.String()))
	market, err = s.keeper.GetMarket(s.ctx, btcusdt.Ticker.String())
	s.Require().NoError(err)
	s.Require().False(market.Ticker.Enabled)
}
