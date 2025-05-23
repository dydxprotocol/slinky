package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtabci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/rand"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/pelletier/go-toml/v2"
	interchaintest "github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	compression "github.com/dydxprotocol/slinky/abci/strategies/codec"
	slinkyabci "github.com/dydxprotocol/slinky/abci/ve/types"
	oracleconfig "github.com/dydxprotocol/slinky/oracle/config"
	slinkytypes "github.com/dydxprotocol/slinky/pkg/types"
	"github.com/dydxprotocol/slinky/providers/static"
	mmtypes "github.com/dydxprotocol/slinky/x/marketmap/types"
	oracletypes "github.com/dydxprotocol/slinky/x/oracle/types"
)

const (
	oracleConfigPath = "oracle.json"
	appConfigPath    = "config/app.toml"
)

var (
	extCommitCodec = compression.NewCompressionExtendedCommitCodec(
		compression.NewDefaultExtendedCommitCodec(),
		compression.NewZStdCompressor(),
	)

	veCodec = compression.NewCompressionVoteExtensionCodec(
		compression.NewDefaultVoteExtensionCodec(),
		compression.NewZLibCompressor(),
	)
)

// ChainConstructor returns the chain that will be using slinky, as well as any additional chains
// that are needed for the test. The first chain returned will be the chain that is used in the
// slinky integration tests.
type ChainConstructor func(t *testing.T, spec *interchaintest.ChainSpec) []*cosmos.CosmosChain

// Interchain is an interface representing the set of chains that are used in the slinky e2e tests, as well
// as any additional relayer / ibc-path information
type Interchain interface {
	Relayer() ibc.Relayer
	Reporter() *testreporter.RelayerExecReporter
	IBCPath() string
}

// InterchainConstructor returns an interchain that will be used in the slinky integration tests.
// The chains used in the interchain constructor should be the chains constructed via the ChainConstructor
type InterchainConstructor func(ctx context.Context, t *testing.T, chains []*cosmos.CosmosChain) Interchain

// DefaultChainConstructor is the default construct of a chan that will be used in the slinky
// integration tests. There is only a single chain that is created.
func DefaultChainConstructor(t *testing.T, spec *interchaintest.ChainSpec) []*cosmos.CosmosChain {
	// require that NumFullNodes == NumValidators == 4
	require.Equal(t, 4, *spec.NumValidators)

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{spec})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	// require that the chain is a cosmos chain
	require.Len(t, chains, 1)
	chain := chains[0]

	cosmosChain, ok := chain.(*cosmos.CosmosChain)
	require.True(t, ok)

	return []*cosmos.CosmosChain{cosmosChain}
}

// DefaultInterchainConstructor is the default constructor of an interchain that will be used in the slinky.
func DefaultInterchainConstructor(ctx context.Context, t *testing.T, chains []*cosmos.CosmosChain) Interchain {
	require.Len(t, chains, 1)

	ic := interchaintest.NewInterchain()
	ic.AddChain(chains[0])

	// create docker network
	client, networkID := interchaintest.DockerSetup(t)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// build the interchain
	err := ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		SkipPathCreation: true,
		Client:           client,
		NetworkID:        networkID,
		TestName:         t.Name(),
	})
	require.NoError(t, err)

	return nil
}

// SetOracleConfigsOnApp writes the oracle configuration to the given node's application config.
func SetOracleConfigsOnApp(node *cosmos.ChainNode) {
	oracle := GetOracleSideCar(node)

	// read the app config from the node
	bz, err := node.ReadFile(context.Background(), appConfigPath)
	if err != nil {
		panic(err)
	}

	// Unmarshall the app config to update the oracle and metrics file paths.
	var appConfig map[string]interface{}
	err = toml.Unmarshal(bz, &appConfig)
	if err != nil {
		panic(err)
	}

	oracleAppConfig, ok := appConfig["oracle"].(map[string]interface{})
	if !ok {
		panic("oracle config not found")
	}

	// Update the file paths to the oracle and metrics configs.
	oracleAppConfig["enabled"] = true
	oracleAppConfig["oracle_address"] = fmt.Sprintf("%s:%s", oracle.HostName(), "8080")
	oracleAppConfig["client_timeout"] = "1s"
	oracleAppConfig["metrics_enabled"] = true
	oracleAppConfig["prometheus_server_address"] = fmt.Sprintf("localhost:%s", "8081")
	oracleAppConfig["price_ttl"] = "5s"
	oracleAppConfig["interval"] = "1s"

	appConfig["oracle"] = oracleAppConfig
	bz, err = toml.Marshal(appConfig)
	if err != nil {
		panic(err)
	}

	// Write back the app config.
	err = node.WriteFile(context.Background(), bz, appConfigPath)
	if err != nil {
		panic(err)
	}
}

// AddSidecarToNode adds the sidecar configured by the given config to the given node. These are configured
// so that the sidecar is started before the node is started.
func AddSidecarToNode(node *cosmos.ChainNode, conf ibc.SidecarConfig) {
	// create the sidecar process
	node.NewSidecarProcess(
		context.Background(),
		true,
		conf.ProcessName,
		node.DockerClient,
		node.NetworkID,
		conf.Image,
		conf.HomeDir,
		conf.Ports,
		conf.StartCmd,
		conf.Env,
	)
}

// SetOracleConfigsOnOracle writes the oracle and metrics configs to the given node's
// oracle sidecar.
func SetOracleConfigsOnOracle(
	oracle *cosmos.SidecarProcess,
	oracleCfg oracleconfig.OracleConfig,
) {
	// marshal the oracle config
	bz, err := json.Marshal(oracleCfg)
	if err != nil {
		panic(err)
	}

	// write the oracle config to the node
	err = oracle.WriteFile(context.Background(), bz, oracleConfigPath)
	if err != nil {
		panic(err)
	}
}

// RestartOracle restarts the oracle sidecar for a given node
func RestartOracle(node *cosmos.ChainNode) error {
	if len(node.Sidecars) != 1 {
		panic("expected node to have oracle sidecar")
	}

	oracle := node.Sidecars[0]

	if err := oracle.StopContainer(context.Background()); err != nil {
		return err
	}

	return oracle.StartContainer(context.Background())
}

// StopOracle stops the oracle sidecar for a given node
func StopOracle(node *cosmos.ChainNode) error {
	if len(node.Sidecars) != 1 {
		panic("expected node to have oracle sidecar")
	}

	oracle := node.Sidecars[0]

	return oracle.StopContainer(context.Background())
}

// StartOracle starts the oracle sidecar for a given node
func StartOracle(node *cosmos.ChainNode) error {
	if len(node.Sidecars) != 1 {
		panic("expected node to have oracle sidecar")
	}

	oracle := node.Sidecars[0]

	return oracle.StartContainer(context.Background())
}

// GetChainGRPC gets a GRPC client of the given chain
//
// NOTICE: this client must be closed after use
func GetChainGRPC(chain *cosmos.CosmosChain) (cc *grpc.ClientConn, close func(), err error) {
	// get grpc address
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err = grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	return cc, func() { cc.Close() }, nil
}

// QueryCurrencyPairs queries the chain for the given CurrencyPair, this method returns the grpc response from the module
func QueryCurrencyPairs(chain *cosmos.CosmosChain) (*oracletypes.GetAllCurrencyPairsResponse, error) {
	// get grpc address
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// create the oracle client
	client := oracletypes.NewQueryClient(cc)

	// query the currency pairs
	resp, err := client.GetAllCurrencyPairs(context.Background(), &oracletypes.GetAllCurrencyPairsRequest{})

	// check that there is a correspondence between mappings and the raw response
	mappingResp, err := QueryCurrencyPairMappings(chain)
	if err != nil {
		return nil, err
	}

	if len(resp.CurrencyPairs) != len(mappingResp.CurrencyPairMapping) {
		return nil, fmt.Errorf("list and map responses should be the same length: got %d list, %d map",
			len(resp.CurrencyPairs),
			len(mappingResp.CurrencyPairMapping),
		)
	}
	for _, v := range mappingResp.CurrencyPairMapping {
		found := false
		for _, cp := range resp.CurrencyPairs {
			if v.Equal(cp) {
				found = true
			}
		}

		if !found {
			return nil, fmt.Errorf("currency pair %v was found in mapping response but not in currency pair list", v)
		}
	}

	return resp, err
}

// QueryCurrencyPairMappings queries the chain for the given currency pair mappings
func QueryCurrencyPairMappings(chain *cosmos.CosmosChain) (*oracletypes.GetCurrencyPairMappingResponse, error) {
	// get grpc address
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// create the oracle client
	client := oracletypes.NewQueryClient(cc)

	// query the currency pairs map
	mapRes, err := client.GetCurrencyPairMapping(context.Background(), &oracletypes.GetCurrencyPairMappingRequest{})
	if err != nil {
		return nil, err
	}

	// query the currency pairs list
	listRes, err := client.GetCurrencyPairMappingList(context.Background(), &oracletypes.GetCurrencyPairMappingListRequest{})
	if err != nil {
		return nil, err
	}

	if len(listRes.Mappings) != len(mapRes.CurrencyPairMapping) {
		return nil, fmt.Errorf("map and list responses should be the same length: got %d list, %d map",
			len(listRes.Mappings),
			len(mapRes.CurrencyPairMapping),
		)
	}
	for _, m := range listRes.Mappings {
		cp, found := mapRes.CurrencyPairMapping[m.Id]
		if !found {
			return nil, fmt.Errorf("mapping for %d not found", m.Id)
		}

		if !m.CurrencyPair.Equal(cp) {
			return nil, fmt.Errorf("market %s is not equal to %s", m.CurrencyPair.String(), cp.String())
		}
	}

	return mapRes, nil
}

// QueryCurrencyPair queries the price for the given currency-pair given a desired height to query from
func QueryCurrencyPair(chain *cosmos.CosmosChain, cp slinkytypes.CurrencyPair, height uint64) (*oracletypes.QuotePrice, int64, error) {
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, 0, err
	}
	defer cc.Close()

	// create the oracle client
	client := oracletypes.NewQueryClient(cc)

	ctx := context.Background()

	md := metadata.New(map[string]string{
		grpctypes.GRPCBlockHeightHeader: strconv.FormatInt(int64(height), 10),
	})

	ctx = metadata.NewOutgoingContext(ctx, md)

	// query the currency pairs
	res, err := client.GetPrice(ctx, &oracletypes.GetPriceRequest{
		CurrencyPair: cp,
	})
	if err != nil {
		return nil, 0, err
	}

	return res.Price, int64(res.Nonce), nil
}

// QueryMarket queries a market from the market map.
func QueryMarket(chain *cosmos.CosmosChain, cp slinkytypes.CurrencyPair) (mmtypes.Market, error) {
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return mmtypes.Market{}, err
	}
	defer cc.Close()

	// create the mm client
	client := mmtypes.NewQueryClient(cc)

	ctx := context.Background()

	// query the currency pairs
	res, err := client.Market(ctx, &mmtypes.MarketRequest{
		CurrencyPair: cp,
	})
	if err != nil {
		return mmtypes.Market{}, err
	}

	return res.Market, nil
}

// QueryMarketMap queries the market map.  This query util provides an additional query to the list endpoint
// and ensures that the response data in both queries is equal.
func QueryMarketMap(chain *cosmos.CosmosChain) (*mmtypes.MarketMapResponse, error) {
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// create the mm client
	client := mmtypes.NewQueryClient(cc)

	ctx := context.Background()

	// query the currency pairs
	mapRes, err := client.MarketMap(ctx, &mmtypes.MarketMapRequest{})
	if err != nil {
		return nil, err
	}

	if mapRes == nil {
		return nil, fmt.Errorf("map response is nil")
	}

	// query markets to check that there is 1-1 correspondence to the map query
	listRes, err := QueryMarkets(chain)
	if err != nil {
		return nil, err
	}

	if len(listRes.Markets) != len(mapRes.MarketMap.Markets) {
		return nil, fmt.Errorf("map and list responses should be the same length: got %d list, %d map",
			len(listRes.Markets),
			len(mapRes.MarketMap.Markets),
		)
	}
	for _, market := range listRes.Markets {
		mapMarket, found := mapRes.MarketMap.Markets[market.Ticker.String()]
		if !found {
			return nil, fmt.Errorf("market %s not found", market.Ticker.String())
		}

		if !market.Equal(mapMarket) {
			return nil, fmt.Errorf("market %s is not equal to %s", market.Ticker.String(), mapMarket.String())
		}
	}

	return mapRes, nil
}

// QueryMarkets queries all markets .
func QueryMarkets(chain *cosmos.CosmosChain) (*mmtypes.MarketsResponse, error) {
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// create the mm client
	client := mmtypes.NewQueryClient(cc)

	ctx := context.Background()

	// query the currency pairs
	res, err := client.Markets(ctx, &mmtypes.MarketsRequest{})
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, fmt.Errorf("response is nil")
	}

	return res, nil
}

// SubmitProposal creates and submits a proposal to the chain
func SubmitProposal(chain *cosmos.CosmosChain, deposit sdk.Coin, submitter string, msgs ...sdk.Msg) (string, error) {
	// build the proposal
	randStr := rand.Str(10)
	protoMsgs := make([]cosmos.ProtoMessage, len(msgs))
	for i, msg := range msgs {
		protoMsgs[i] = msg
	}

	prop, err := chain.BuildProposal(protoMsgs, randStr, randStr, randStr, deposit.String(), submitter, false)
	if err != nil {
		return "", err
	}

	// submit the proposal
	tx, err := chain.SubmitProposal(context.Background(), submitter, prop)
	return tx.ProposalID, err
}

// PassProposal given a proposal id, vote for the proposal and wait for it to pass
func PassProposal(chain *cosmos.CosmosChain, propId string, timeout time.Duration) error {
	if err := WaitForProposalStatus(chain, propId, timeout, govtypesv1.StatusVotingPeriod); err != nil {
		return fmt.Errorf("proposal did not enter voting period: %v", err)
	}

	propIdUint, err := strconv.ParseUint(propId, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert proposal id to uint: %v", err)
	}
	// have all nodes vote on the proposal
	wg := sync.WaitGroup{}
	for _, node := range chain.Nodes() {
		wg.Add(1)
		go func(node *cosmos.ChainNode) {
			defer wg.Done()
			node.VoteOnProposal(context.Background(), validatorKey, propIdUint, yes)
		}(node)
	}
	wg.Wait()

	// wait for the proposal to pass
	if err := WaitForProposalStatus(chain, propId, timeout, govtypesv1.StatusPassed); err != nil {
		prop, queryErr := QueryProposal(chain, propId)
		if queryErr != nil {
			return queryErr
		}

		return fmt.Errorf("proposal did not pass: %v, status: %v", err, prop.Proposal.FailedReason)
	}
	return nil
}

// AddCurrencyPairs creates + submits the proposal to add the given currency-pairs to state, votes for the prop w/ all nodes,
// and waits for the proposal to pass.
func (s *SlinkyIntegrationSuite) AddCurrencyPairs(chain *cosmos.CosmosChain, user cosmos.User, price float64,
	tickers ...mmtypes.Ticker,
) error {
	creates := make([]mmtypes.Market, len(tickers))
	for i, ticker := range tickers {
		creates[i] = mmtypes.Market{
			Ticker: ticker,
			ProviderConfigs: []mmtypes.ProviderConfig{
				{
					Name:           static.Name,
					OffChainTicker: ticker.String(),
					Metadata_JSON:  fmt.Sprintf(`{"price": %f}`, price),
				},
			},
		}
	}

	msg := &mmtypes.MsgUpsertMarkets{
		Authority: s.user.FormattedAddress(),
		Markets:   creates,
	}

	tx := CreateTx(s.T(), s.chain, user, gasPrice, msg)

	// get an rpc endpoint for the chain
	client := chain.Nodes()[0].Client

	ctx := context.Background()

	// broadcast the tx
	txResp, err := client.BroadcastTxCommit(ctx, tx)
	if err != nil {
		return err
	}

	if txResp.TxResult.Code != abcitypes.CodeTypeOK {
		return fmt.Errorf(txResp.TxResult.Log)
	}

	time.Sleep(2 * time.Second)

	// check market map and lastUpdated
	mmResp, err := QueryMarketMap(chain)
	s.Require().NoError(err)

	// ensure that the market exists
	for _, create := range creates {
		got, found := mmResp.MarketMap.Markets[create.Ticker.String()]
		s.Require().True(found)
		s.Require().Equal(create, got)
	}

	s.Require().Equal(uint64(txResp.Height), mmResp.LastUpdated)

	return nil
}

func (s *SlinkyIntegrationSuite) RemoveMarket(
	chain *cosmos.CosmosChain,
	markets []slinkytypes.CurrencyPair,
) error {
	marketString := make([]string, len(markets))
	for i, market := range markets {
		marketString[i] = market.String()
	}

	msg := &mmtypes.MsgRemoveMarkets{
		Authority: s.user.FormattedAddress(),
		Markets:   marketString,
	}

	tx := CreateTx(s.T(), s.chain, s.user, gasPrice, msg)

	// get an rpc endpoint for the chain
	client := chain.Nodes()[0].Client
	// broadcast the tx
	resp, err := client.BroadcastTxCommit(context.Background(), tx)
	if err != nil {
		return err
	}

	if resp.TxResult.Code != abcitypes.CodeTypeOK {
		return fmt.Errorf(resp.TxResult.Log)
	}

	// check market map and lastUpdated
	mmResp, err := QueryMarketMap(chain)
	s.Require().NoError(err)

	// ensure that the market no longer exist
	for _, market := range markets {
		_, found := mmResp.MarketMap.Markets[market.String()]
		s.Require().False(found)
	}

	return nil
}

func (s *SlinkyIntegrationSuite) UpdateCurrencyPair(chain *cosmos.CosmosChain, markets []mmtypes.Market) error {
	msg := &mmtypes.MsgUpsertMarkets{
		Authority: s.user.FormattedAddress(),
		Markets:   markets,
	}

	tx := CreateTx(s.T(), s.chain, s.user, gasPrice, msg)

	// get an rpc endpoint for the chain
	client := chain.Nodes()[0].Client
	// broadcast the tx
	txResp, err := client.BroadcastTxCommit(context.Background(), tx)
	if err != nil {
		return err
	}

	if txResp.TxResult.Code != abcitypes.CodeTypeOK {
		return fmt.Errorf(txResp.TxResult.Log)
	}

	// check market map and lastUpdated
	mmResp, err := QueryMarketMap(chain)
	s.Require().NoError(err)

	// ensure that the market exists
	for _, create := range markets {
		got, found := mmResp.MarketMap.Markets[create.Ticker.String()]
		s.Require().True(found)
		s.Require().Equal(create, got)
	}

	s.Require().Equal(uint64(txResp.Height), mmResp.LastUpdated)

	return nil
}

// QueryProposal queries the chain for a given proposal
func QueryProposal(chain *cosmos.CosmosChain, propID string) (*govtypesv1.QueryProposalResponse, error) {
	// get grpc address
	grpcAddr := chain.GetHostGRPCAddress()

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	// create the oracle client
	client := govtypesv1.NewQueryClient(cc)

	propId, err := strconv.ParseUint(propID, 10, 64)
	if err != nil {
		return nil, err
	}
	// query the currency pairs
	return client.Proposal(context.Background(), &govtypesv1.QueryProposalRequest{
		ProposalId: propId,
	})
}

// WaitForProposalStatus waits for the deposit period for the proposal to end
func WaitForProposalStatus(chain *cosmos.CosmosChain, propID string, timeout time.Duration, status govtypesv1.ProposalStatus) error {
	return testutil.WaitForCondition(timeout, 1*time.Second, func() (bool, error) {
		prop, err := QueryProposal(chain, propID)
		if err != nil {
			return false, err
		}

		return prop.Proposal.Status == status, nil
	})
}

// WaitForHeight waits for the giuve height to be reached
func WaitForHeight(chain *cosmos.CosmosChain, height uint64, timeout time.Duration) error {
	return testutil.WaitForCondition(timeout, 100*time.Millisecond, func() (bool, error) {
		h, err := chain.Height(context.Background())
		if err != nil {
			return false, err
		}

		return uint64(h) >= height, nil
	})
}

// ExpectVoteExtensions waits for empty oracle update waits for a pre-determined number of blocks for an extended commit with the given oracle-vote extensions provided
// per validator. This method returns the height at which the condition was satisfied.
//
// Notice: the height returned is safe for querying, i.e the prices will have been written to state if a quorum reported
func ExpectVoteExtensions(chain *cosmos.CosmosChain, timeout time.Duration, ves []slinkyabci.OracleVoteExtension) (uint64, error) {
	client := chain.Nodes()[0].Client

	var blockHeight int64
	if err := testutil.WaitForCondition(timeout, 100*time.Millisecond, func() (bool, error) {
		var err error

		blockHeight, err = chain.Height(context.Background())
		if err != nil {
			return false, err
		}

		height := int64(blockHeight)
		// get the block
		block, err := client.Block(context.Background(), &height)
		if err != nil {
			return false, err
		}

		// get the oracle update
		if len(block.Block.Txs) == 0 {
			return false, fmt.Errorf("block is invalid: no oracle transaction")
		}

		// attempt to unmarshal extended commit info
		extendedCommitInfo, err := extCommitCodec.Decode(block.Block.Txs[0])
		if err != nil {
			return false, err
		}

		sort.Sort(validatorVotes(extendedCommitInfo.Votes))

		// iterate through all votes (votes in the extended-commit are deterministically ordered by voting power -> address)
		for i, vote := range extendedCommitInfo.Votes {
			// get the oracle vote extension
			gotVe, err := veCodec.Decode(vote.VoteExtension)
			if err != nil {
				return false, err
			}

			if len(ves[i].Prices) != len(gotVe.Prices) {
				return false, nil
			}

			// check that the vote extension is correct
			for ticker, price := range gotVe.Prices {
				if !bytes.Equal(price, ves[i].Prices[ticker]) {
					return false, nil
				}
			}
		}

		return true, nil
	}); err != nil {
		return 0, err
	}

	// we want to wait for the application state to reflect the proposed state from blockHeight
	return uint64(blockHeight), WaitForHeight(chain, uint64(blockHeight+1), timeout)
}

// wrapper around extendedVoteInfo for use in sorting (to make ordering deterministic in tests)
type validatorVotes []cmtabci.ExtendedVoteInfo

func (vv validatorVotes) Len() int { return len(vv) }

func (vv validatorVotes) Swap(i, j int) { vv[i], vv[j] = vv[j], vv[i] }

// order the votes by the number of reports first, then by the contents of the vote-extensions.
func (vv validatorVotes) Less(i, j int) bool {
	// break ties by vote-extension data
	var (
		iPrice, jPrice, iTotalPrice, jTotalPrice int
	)

	ve, err := veCodec.Decode(vv[i].VoteExtension)
	if err == nil {
		iPrice = len(ve.Prices)

		for _, priceBz := range ve.Prices {
			var price big.Int
			if err := price.GobDecode(priceBz); err != nil {
				panic(err)
			}

			iTotalPrice += int(price.Int64())
		}
	}

	ve, err = veCodec.Decode(vv[j].VoteExtension)
	if err == nil {
		jPrice = len(ve.Prices)

		for _, priceBz := range ve.Prices {
			var price big.Int
			if err := price.GobDecode(priceBz); err != nil {
				panic(err)
			}

			jTotalPrice += int(price.Int64())
		}
	}

	// check if the number of prices is different
	if iPrice != jPrice {
		return iPrice < jPrice
	}

	// break ties by the sum of the prices for each validator
	return iTotalPrice < jTotalPrice
}
