package stork_test

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/providers/apis/stork"
	"github.com/dydxprotocol/slinky/providers/base/testutils"
	providertypes "github.com/dydxprotocol/slinky/providers/types"
)

var (
	xagusd = types.DefaultProviderTicker{
		OffChainTicker: "XAGUSD",
	}
	spxusd = types.DefaultProviderTicker{
		OffChainTicker: "SPXUSD",
	}
)

// signedItemJSON returns a single PriceResponse item JSON with a valid ECDSA
// signature from key. The handler under test must trust the key's address.
func signedItemJSON(t *testing.T, key *ecdsa.PrivateKey, market, price string) string {
	t.Helper()
	addr := ethcrypto.PubkeyToAddress(key.PublicKey)

	msgHash := ethcrypto.Keccak256([]byte("test-stork-msg-" + market))
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digest := ethcrypto.Keccak256(append(prefix, msgHash...))
	sig, err := ethcrypto.Sign(digest, key)
	require.NoError(t, err)

	return fmt.Sprintf(`{
		"market": %q,
		"price": %q,
		"timestampMs": 1234567890000,
		"storkSignatureVerification": {
			"stork_signed_price": {
				"public_key": %q,
				"encoded_asset_id": "0xabcd",
				"price": %q,
				"timestamped_signature": {
					"signature": {
						"r": "0x%s",
						"s": "0x%s",
						"v": "0x%02x"
					},
					"timestamp": 1234567890000000000,
					"msg_hash": "0x%s"
				},
				"publisher_merkle_root": "0x1234",
				"calculation_alg": {"type":"median","version":"v1","checksum":"abc"}
			},
			"signed_prices": []
		}
	}`, market, price,
		addr.Hex(), price,
		hex.EncodeToString(sig[0:32]),
		hex.EncodeToString(sig[32:64]),
		sig[64]+27,
		hex.EncodeToString(msgHash))
}

// signedBatchJSON builds a full {"data": [...]} response with one or more
// signed items. It generates a key and sets STORK_PUB_KEY for the test.
func signedBatchJSON(t *testing.T, items ...struct {
	market string
	price  string
},
) string {
	t.Helper()
	key, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	addr := ethcrypto.PubkeyToAddress(key.PublicKey)
	t.Setenv(stork.StorkPubKeyEnv, addr.Hex())

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = signedItemJSON(t, key, item.market, item.price)
	}

	return `{"data":[` + strings.Join(parts, ",") + `]}`
}

// badSigBatchJSON returns a {"data":[...]} response where the public_key does
// not match the actual signer, so verification should fail.
func badSigBatchJSON() string {
	return `{"data":[{
		"market": "XAGUSD",
		"price": "30500000000000000000",
		"timestampMs": 1234567890000,
		"storkSignatureVerification": {
			"stork_signed_price": {
				"public_key": "0x0000000000000000000000000000000000000001",
				"encoded_asset_id": "0xabcd",
				"price": "30500000000000000000",
				"timestamped_signature": {
					"signature": {
						"r": "0x5b3ef6c1e990d8f8761633386eb1bbaf2c584b048daef58fbb8927936f51def5",
						"s": "0x2d91200de4f245d846a8bf54c3e51b78dc03f81814dba74765dcc602f5103c32",
						"v": "0x1c"
					},
					"timestamp": 1234567890000000000,
					"msg_hash": "0xf5a5d4cf42bf421f48d00a8eb4f0752cd1079061383972b99c57b64a59cce21d"
				},
				"publisher_merkle_root": "0x1234",
				"calculation_alg": {"type":"median","version":"v1","checksum":"abc"}
			},
			"signed_prices": []
		}
	}]}`
}

func TestCreateURL(t *testing.T) {
	testCases := []struct {
		name        string
		cps         []types.ProviderTicker
		url         string
		expectedErr bool
	}{
		{
			name:        "empty",
			cps:         []types.ProviderTicker{},
			url:         "",
			expectedErr: true,
		},
		{
			name: "valid single",
			cps: []types.ProviderTicker{
				xagusd,
			},
			url:         fmt.Sprintf("%s?asset=%s&provider=stork", stork.URL, "XAGUSD"),
			expectedErr: false,
		},
		{
			name: "valid multiple",
			cps: []types.ProviderTicker{
				xagusd,
				spxusd,
			},
			url:         fmt.Sprintf("%s?asset=%s&provider=stork", stork.URL, "XAGUSD,SPXUSD"),
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := stork.NewAPIHandler(stork.DefaultAPIConfig)
			require.NoError(t, err)

			url, err := h.CreateURL(tc.cps)
			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.url, url)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	type item struct {
		market string
		price  string
	}

	testCases := []struct {
		name     string
		cps      []types.ProviderTicker
		response func(t *testing.T) *http.Response
		expected types.PriceResponse
	}{
		{
			name: "valid single in batch",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", "30500000000000000000"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {Value: big.NewFloat(30.5)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "multiple tickers resolved",
			cps:  []types.ProviderTicker{xagusd, spxusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t,
						item{"XAGUSD", "30500000000000000000"},
						item{"SPXUSD", "5500000000000000000000"},
					),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {Value: big.NewFloat(30.5)},
					spxusd: {Value: big.NewFloat(5500)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "bad json response",
			cps:  []types.ProviderTicker{xagusd},
			response: func(_ *testing.T) *http.Response {
				return testutils.CreateResponseFromJSON(`not valid json`)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("decode error"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "signature verification fails",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001")
				return testutils.CreateResponseFromJSON(badSigBatchJSON())
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("sig mismatch"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "bad price value",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", "$30.50"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("parse error"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "empty price string",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", ""}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("parse error"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "zero price",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", "0"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {Value: big.NewFloat(0)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "very large price",
			cps:  []types.ProviderTicker{xagusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", "999999999000000000000000000000"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {Value: big.NewFloat(999999999000)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "ticker not in batch response",
			cps:  []types.ProviderTicker{xagusd, spxusd},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"XAGUSD", "30500000000000000000"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {Value: big.NewFloat(30.5)},
				},
				types.UnResolvedPrices{
					spxusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("no response"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "empty batch data",
			cps:  []types.ProviderTicker{xagusd},
			response: func(_ *testing.T) *http.Response {
				return testutils.CreateResponseFromJSON(`{"data":[]}`)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("no response"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// The response helpers set STORK_PUB_KEY, which the handler reads at construction.
			httpResp := tc.response(t)

			h, err := stork.NewAPIHandler(stork.DefaultAPIConfig)
			require.NoError(t, err)

			_, err = h.CreateURL(tc.cps)
			require.NoError(t, err)

			now := time.Now()
			resp := h.ParseResponse(tc.cps, httpResp)

			require.Len(t, resp.Resolved, len(tc.expected.Resolved))
			require.Len(t, resp.UnResolved, len(tc.expected.UnResolved))

			for cp, result := range tc.expected.Resolved {
				require.Contains(t, resp.Resolved, cp)
				r := resp.Resolved[cp]
				require.Equal(t, result.Value.SetPrec(18), r.Value.SetPrec(18))
				require.True(t, r.Timestamp.After(now))
			}

			for cp := range tc.expected.UnResolved {
				require.Contains(t, resp.UnResolved, cp)
				require.Error(t, resp.UnResolved[cp])
			}
		})
	}
}

// productionSignedPrice is a real XAU-USD aggregator price signed by
// 0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44.
func productionSignedPrice() stork.SignedPrice {
	return stork.SignedPrice{
		PublicKey:      "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
		EncodedAssetID: "0xe21c86d8b6a127bfef214d88fdb0c279e55d27dd8c443733e46c8d3de3c98cd6",
		Price:          "5176579999999999000000",
		TimestampedSignature: stork.TimestampedSignature{
			Signature: stork.EvmSignature{
				R: "0x5b3ef6c1e990d8f8761633386eb1bbaf2c584b048daef58fbb8927936f51def5",
				S: "0x2d91200de4f245d846a8bf54c3e51b78dc03f81814dba74765dcc602f5103c32",
				V: "0x1c",
			},
			Timestamp: 1773266051641470000,
			MsgHash:   "0xf5a5d4cf42bf421f48d00a8eb4f0752cd1079061383972b99c57b64a59cce21d",
		},
		PublisherMerkleRoot: "0x7e7d41d87fedc065729e40eb6d51e62580dcb5f614c8e50dee27ae3eff70fb8d",
		CalculationAlg: stork.CalculationAlg{
			Type:     "median",
			Version:  "v1",
			Checksum: "9be7e9f9ed459417d96112a7467bd0b27575a2c7847195c68f805b70ce1795ba",
		},
	}
}

// generatedSignedPrice signs msg with a fresh key and returns the signed price
// together with the signer's address.
func generatedSignedPrice(t *testing.T, msg string) (stork.SignedPrice, common.Address) {
	t.Helper()
	key, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	addr := ethcrypto.PubkeyToAddress(key.PublicKey)

	msgHash := ethcrypto.Keccak256([]byte(msg))
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digest := ethcrypto.Keccak256(append(prefix, msgHash...))
	sig, err := ethcrypto.Sign(digest, key)
	require.NoError(t, err)

	return stork.SignedPrice{
		PublicKey: addr.Hex(),
		TimestampedSignature: stork.TimestampedSignature{
			Signature: stork.EvmSignature{
				R: "0x" + hex.EncodeToString(sig[0:32]),
				S: "0x" + hex.EncodeToString(sig[32:64]),
				V: fmt.Sprintf("0x%02x", sig[64]+27),
			},
			MsgHash: "0x" + hex.EncodeToString(msgHash),
		},
	}, addr
}

func addrs(hexes ...string) []common.Address {
	out := make([]common.Address, len(hexes))
	for i, h := range hexes {
		out[i] = common.HexToAddress(h)
	}
	return out
}

func TestVerifyStorkSignature(t *testing.T) {
	t.Run("valid signature from generated key", func(t *testing.T) {
		sp, addr := generatedSignedPrice(t, "verify-test")
		require.NoError(t, stork.VerifyStorkSignature(sp, []common.Address{addr}))
	})

	t.Run("valid production signature for XAU-USD", func(t *testing.T) {
		signers := addrs("0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44")
		require.NoError(t, stork.VerifyStorkSignature(productionSignedPrice(), signers))
	})

	t.Run("signer matches any entry in the list", func(t *testing.T) {
		signers := addrs(
			"0x0000000000000000000000000000000000000001",
			"0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
			"0x0000000000000000000000000000000000000002",
		)
		require.NoError(t, stork.VerifyStorkSignature(productionSignedPrice(), signers))
	})

	t.Run("signer matches last entry in the list", func(t *testing.T) {
		signers := addrs(
			"0x0000000000000000000000000000000000000001",
			"0x0000000000000000000000000000000000000002",
			"0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
		)
		require.NoError(t, stork.VerifyStorkSignature(productionSignedPrice(), signers))
	})

	t.Run("production signature verifies against defaults", func(t *testing.T) {
		require.NoError(t, stork.VerifyStorkSignature(productionSignedPrice(), stork.DefaultSignerAddresses))
	})

	t.Run("production signature with wrong signer list", func(t *testing.T) {
		signers := addrs(
			"0x0000000000000000000000000000000000000001",
			"0x0000000000000000000000000000000000000002",
		)
		err := stork.VerifyStorkSignature(productionSignedPrice(), signers)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("wrong public key", func(t *testing.T) {
		sp, _ := generatedSignedPrice(t, "verify-test")
		sp.PublicKey = "0x0000000000000000000000000000000000000001"
		err := stork.VerifyStorkSignature(sp, addrs("0x0000000000000000000000000000000000000001"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("no signers configured", func(t *testing.T) {
		sp, _ := generatedSignedPrice(t, "verify-test")
		err := stork.VerifyStorkSignature(sp, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no trusted signer addresses")
	})

	t.Run("invalid msg_hash length", func(t *testing.T) {
		sp := stork.SignedPrice{
			PublicKey: "0x0000000000000000000000000000000000000001",
			TimestampedSignature: stork.TimestampedSignature{
				Signature: stork.EvmSignature{R: "0xaa", S: "0xbb", V: "0x1c"},
				MsgHash:   "0xdead",
			},
		}
		err := stork.VerifyStorkSignature(sp, addrs("0x0000000000000000000000000000000000000001"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid msg_hash length")
	})
}

func TestParseSignerAddresses(t *testing.T) {
	testCases := []struct {
		name        string
		raw         string
		expected    []common.Address
		expectedErr string
	}{
		{
			name:     "single address",
			raw:      "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
			expected: addrs("0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44"),
		},
		{
			name: "multiple addresses with whitespace",
			raw:  " 0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44 ,0x0bb53E0d5E89778DCD13C2720667D292368dD053 ",
			expected: addrs(
				"0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
				"0x0bb53E0d5E89778DCD13C2720667D292368dD053",
			),
		},
		{
			name:     "lowercase without prefix",
			raw:      "0a803f9b1cce32e2773e0d2e98b37e0775ca5d44",
			expected: addrs("0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44"),
		},
		{
			name:        "trailing comma",
			raw:         "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44,",
			expectedErr: "invalid signer address",
		},
		{
			name:        "leading comma",
			raw:         ",0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44",
			expectedErr: "invalid signer address",
		},
		{
			name:        "double comma",
			raw:         "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44,,0x0bb53E0d5E89778DCD13C2720667D292368dD053",
			expectedErr: "invalid signer address",
		},
		{
			name:        "malformed entry",
			raw:         "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44,not-an-address",
			expectedErr: "invalid signer address",
		},
		{
			name:        "empty",
			raw:         "",
			expectedErr: "invalid signer address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stork.ParseSignerAddresses(tc.raw)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestSignerAddressesFromEnv(t *testing.T) {
	t.Run("unset uses defaults", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "")
		got, err := stork.SignerAddressesFromEnv()
		require.NoError(t, err)
		require.Equal(t, stork.DefaultSignerAddresses, got)
	})

	t.Run("defaults are copied", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "")
		got, err := stork.SignerAddressesFromEnv()
		require.NoError(t, err)

		got[0] = common.HexToAddress("0x0000000000000000000000000000000000000001")
		require.NotEqual(t, got[0], stork.DefaultSignerAddresses[0])
	})

	t.Run("whitespace only uses defaults", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "   ")
		got, err := stork.SignerAddressesFromEnv()
		require.NoError(t, err)
		require.Equal(t, stork.DefaultSignerAddresses, got)
	})

	t.Run("set overrides defaults", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001,0x0000000000000000000000000000000000000002")
		got, err := stork.SignerAddressesFromEnv()
		require.NoError(t, err)
		require.Equal(t, addrs(
			"0x0000000000000000000000000000000000000001",
			"0x0000000000000000000000000000000000000002",
		), got)
	})

	t.Run("invalid value is an error", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001,bogus")
		_, err := stork.SignerAddressesFromEnv()
		require.Error(t, err)
		require.Contains(t, err.Error(), stork.StorkPubKeyEnv)
	})
}

func TestNewAPIHandlerSignerConfig(t *testing.T) {
	t.Run("invalid signer list fails construction", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "bogus")
		_, err := stork.NewAPIHandler(stork.DefaultAPIConfig)
		require.Error(t, err)
		require.Contains(t, err.Error(), stork.StorkPubKeyEnv)
	})

	t.Run("signers are read once at construction", func(t *testing.T) {
		key, err := ethcrypto.GenerateKey()
		require.NoError(t, err)
		addr := ethcrypto.PubkeyToAddress(key.PublicKey)

		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001")
		h, err := stork.NewAPIHandler(stork.DefaultAPIConfig)
		require.NoError(t, err)

		// Changing the env after construction must not affect the handler.
		t.Setenv(stork.StorkPubKeyEnv, addr.Hex())

		body := `{"data":[` + signedItemJSON(t, key, "XAGUSD", "30500000000000000000") + `]}`
		resp := h.ParseResponse([]types.ProviderTicker{xagusd}, testutils.CreateResponseFromJSON(body))
		require.Empty(t, resp.Resolved)
		require.Contains(t, resp.UnResolved, xagusd)
		require.Contains(t, resp.UnResolved[xagusd].Error(), "signature mismatch")
	})
}
