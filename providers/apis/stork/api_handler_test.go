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
// signature. The caller must have already set STORK_PUB_KEY via t.Setenv.
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
}) string {
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
			url:         fmt.Sprintf("%s?asset=%s", stork.URL, "XAGUSD"),
			expectedErr: false,
		},
		{
			name: "valid multiple",
			cps: []types.ProviderTicker{
				xagusd,
				spxusd,
			},
			url:         fmt.Sprintf("%s?asset=%s", stork.URL, "XAGUSD,SPXUSD"),
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := stork.NewAPIHandler(nil, stork.DefaultAPIConfig)
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
			h, err := stork.NewAPIHandler(nil, stork.DefaultAPIConfig)
			require.NoError(t, err)

			_, err = h.CreateURL(tc.cps)
			require.NoError(t, err)

			now := time.Now()
			resp := h.ParseResponse(tc.cps, tc.response(t))

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

func TestVerifyStorkSignature(t *testing.T) {
	t.Run("valid signature from generated key", func(t *testing.T) {
		key, err := ethcrypto.GenerateKey()
		require.NoError(t, err)
		addr := ethcrypto.PubkeyToAddress(key.PublicKey)
		t.Setenv(stork.StorkPubKeyEnv, addr.Hex())

		msgHash := ethcrypto.Keccak256([]byte("verify-test"))
		prefix := []byte("\x19Ethereum Signed Message:\n32")
		digest := ethcrypto.Keccak256(append(prefix, msgHash...))
		sig, err := ethcrypto.Sign(digest, key)
		require.NoError(t, err)

		sp := stork.SignedPrice{
			PublicKey: addr.Hex(),
			TimestampedSignature: stork.TimestampedSignature{
				Signature: stork.EvmSignature{
					R: "0x" + hex.EncodeToString(sig[0:32]),
					S: "0x" + hex.EncodeToString(sig[32:64]),
					V: fmt.Sprintf("0x%02x", sig[64]+27),
				},
				MsgHash: "0x" + hex.EncodeToString(msgHash),
			},
		}
		require.NoError(t, stork.VerifyStorkSignature(sp))
	})

	t.Run("valid production signature for XAU-USD", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44")

		sp := stork.SignedPrice{
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
		require.NoError(t, stork.VerifyStorkSignature(sp))
	})

	t.Run("production signature with wrong public key", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001")

		sp := stork.SignedPrice{
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
		err := stork.VerifyStorkSignature(sp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("wrong public key", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001")

		key, err := ethcrypto.GenerateKey()
		require.NoError(t, err)
		msgHash := ethcrypto.Keccak256([]byte("verify-test"))
		prefix := []byte("\x19Ethereum Signed Message:\n32")
		digest := ethcrypto.Keccak256(append(prefix, msgHash...))
		sig, err := ethcrypto.Sign(digest, key)
		require.NoError(t, err)

		sp := stork.SignedPrice{
			PublicKey: "0x0000000000000000000000000000000000000001",
			TimestampedSignature: stork.TimestampedSignature{
				Signature: stork.EvmSignature{
					R: "0x" + hex.EncodeToString(sig[0:32]),
					S: "0x" + hex.EncodeToString(sig[32:64]),
					V: fmt.Sprintf("0x%02x", sig[64]+27),
				},
				MsgHash: "0x" + hex.EncodeToString(msgHash),
			},
		}
		err = stork.VerifyStorkSignature(sp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("env var not set", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "")

		sp := stork.SignedPrice{
			PublicKey: "0x0000000000000000000000000000000000000001",
			TimestampedSignature: stork.TimestampedSignature{
				Signature: stork.EvmSignature{R: "0xaa", S: "0xbb", V: "0x1c"},
				MsgHash:   "0xdead",
			},
		}
		err := stork.VerifyStorkSignature(sp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "STORK_PUB_KEY")
	})

	t.Run("invalid msg_hash length", func(t *testing.T) {
		t.Setenv(stork.StorkPubKeyEnv, "0x0000000000000000000000000000000000000001")

		sp := stork.SignedPrice{
			PublicKey: "0x0000000000000000000000000000000000000001",
			TimestampedSignature: stork.TimestampedSignature{
				Signature: stork.EvmSignature{R: "0xaa", S: "0xbb", V: "0x1c"},
				MsgHash:   "0xdead",
			},
		}
		err := stork.VerifyStorkSignature(sp)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid msg_hash length")
	})
}
