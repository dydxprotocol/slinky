package pyth_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	"github.com/dydxprotocol/slinky/oracle/types"
	"github.com/dydxprotocol/slinky/providers/apis/pyth"
	"github.com/dydxprotocol/slinky/providers/base/testutils"
	providertypes "github.com/dydxprotocol/slinky/providers/types"
)

var (
	feed2694 = types.DefaultProviderTicker{
		OffChainTicker: "2694",
	}
	feed3032 = types.DefaultProviderTicker{
		OffChainTicker: "3032",
	}
)

// buildSolanaPayload constructs a Pyth Lazer Solana-format payload:
//
//	magic(4) || signature(64) || pubkey(32) || msgLen(2) || msg(msgLen)
func buildSolanaPayload(t *testing.T, privKey ed25519.PrivateKey, msg []byte) string {
	t.Helper()
	sig := ed25519.Sign(privKey, msg)

	buf := make([]byte, 4+64+32+2+len(msg))
	binary.LittleEndian.PutUint32(buf[0:4], pyth.SolanaFormatMagic)
	copy(buf[4:68], sig)
	copy(buf[68:100], privKey.Public().(ed25519.PublicKey))
	binary.LittleEndian.PutUint16(buf[100:102], uint16(len(msg)))
	copy(buf[102:], msg)

	return base64.StdEncoding.EncodeToString(buf)
}

// signedItemJSON returns a single PriceResponse JSON entry with a valid
// Pyth Solana payload. The caller must set PYTH_PUB_KEY via t.Setenv.
func signedItemJSON(t *testing.T, privKey ed25519.PrivateKey, market, price string) string {
	t.Helper()
	msg := []byte("pyth-test-msg-" + market)
	payload := buildSolanaPayload(t, privKey, msg)
	return fmt.Sprintf(
		`{"market":%q,"price":%q,"timestampMs":1774625883288,"pythSolanaPayload":%q}`,
		market, price, payload,
	)
}

// signedBatchJSON builds a full {"data":[...]} response, generates a key,
// and sets PYTH_PUB_KEY for the test.
func signedBatchJSON(t *testing.T, items ...struct {
	market string
	price  string
},
) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = signedItemJSON(t, priv, item.market, item.price)
	}
	return `{"data":[` + strings.Join(parts, ",") + `]}`
}

// badSigBatchJSON returns a response where the signature is for a different
// key than the one set in PYTH_PUB_KEY.
func badSigBatchJSON(t *testing.T) string {
	t.Helper()
	pub1, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	_, priv2, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub1).String())

	msg := []byte("pyth-test-msg-2694")
	payload := buildSolanaPayload(t, priv2, msg)
	return fmt.Sprintf(
		`{"data":[{"market":"2694","price":"0.097","timestampMs":1774625883288,"pythSolanaPayload":%q}]}`,
		payload,
	)
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
			cps:  []types.ProviderTicker{feed2694},
			url:  fmt.Sprintf("%s?asset=%s&provider=pyth", pyth.URL, "2694"),
		},
		{
			name: "valid multiple",
			cps:  []types.ProviderTicker{feed2694, feed3032},
			url:  fmt.Sprintf("%s?asset=%s&provider=pyth", pyth.URL, "2694,3032"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := pyth.NewAPIHandler(pyth.DefaultAPIConfig)
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
			name: "valid single",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"2694", "0.097264730000000000"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0.09726473)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "multiple feeds resolved",
			cps:  []types.ProviderTicker{feed2694, feed3032},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t,
						item{"2694", "0.097264730000000000"},
						item{"3032", "3456.780000000000000000"},
					),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0.09726473)},
					feed3032: {Value: big.NewFloat(3456.78)},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "bad json response",
			cps:  []types.ProviderTicker{feed2694},
			response: func(_ *testing.T) *http.Response {
				return testutils.CreateResponseFromJSON(`not valid json`)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					feed2694: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("decode error"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "signature verification fails - wrong pubkey",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(badSigBatchJSON(t))
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					feed2694: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("sig mismatch"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "bad price value",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"2694", "$bad"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					feed2694: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("parse error"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "feed not in batch response",
			cps:  []types.ProviderTicker{feed2694, feed3032},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"2694", "0.097"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0.097)},
				},
				types.UnResolvedPrices{
					feed3032: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("no response"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "empty batch data",
			cps:  []types.ProviderTicker{feed2694},
			response: func(_ *testing.T) *http.Response {
				return testutils.CreateResponseFromJSON(`{"data":[]}`)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					feed2694: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("no response"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "zero price",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				return testutils.CreateResponseFromJSON(
					signedBatchJSON(t, item{"2694", "0"}),
				)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0)},
				},
				types.UnResolvedPrices{},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := pyth.NewAPIHandler(pyth.DefaultAPIConfig)
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

func TestVerifyPythSolanaSignature(t *testing.T) {
	t.Run("valid signature from generated key", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		msg := []byte("hello-pyth")
		payload := buildSolanaPayload(t, priv, msg)
		require.NoError(t, pyth.VerifyPythSolanaSignature(payload))
	})

	t.Run("valid production sample payload", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		msg := []byte("production-like-price-data-for-feed-2694")
		payload := buildSolanaPayload(t, priv, msg)
		require.NoError(t, pyth.VerifyPythSolanaSignature(payload))
	})

	t.Run("wrong public key", func(t *testing.T) {
		pub1, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		_, priv2, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub1).String())

		msg := []byte("test-msg")
		payload := buildSolanaPayload(t, priv2, msg)
		err = pyth.VerifyPythSolanaSignature(payload)
		require.Error(t, err)
		require.Contains(t, err.Error(), "public key mismatch")
	})

	t.Run("tampered message", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		msg := []byte("original-msg")
		sig := ed25519.Sign(priv, msg)

		tamperedMsg := []byte("tampered-msg")
		buf := make([]byte, 4+64+32+2+len(tamperedMsg))
		binary.LittleEndian.PutUint32(buf[0:4], pyth.SolanaFormatMagic)
		copy(buf[4:68], sig)
		copy(buf[68:100], pub)
		binary.LittleEndian.PutUint16(buf[100:102], uint16(len(tamperedMsg)))
		copy(buf[102:], tamperedMsg)

		payload := base64.StdEncoding.EncodeToString(buf)
		err = pyth.VerifyPythSolanaSignature(payload)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ed25519 signature verification failed")
	})

	t.Run("env var not set", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "")
		err := pyth.VerifyPythSolanaSignature("dGVzdA==")
		require.Error(t, err)
		require.Contains(t, err.Error(), "PYTH_PUB_KEY")
	})

	t.Run("invalid base64", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "11111111111111111111111111111111")
		err := pyth.VerifyPythSolanaSignature("!!!not-base64!!!")
		require.Error(t, err)
		require.Contains(t, err.Error(), "base64")
	})

	t.Run("payload too short", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "11111111111111111111111111111111")
		short := base64.StdEncoding.EncodeToString([]byte("tooshort"))
		err := pyth.VerifyPythSolanaSignature(short)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too short")
	})

	t.Run("wrong magic", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		buf := make([]byte, 102)
		binary.LittleEndian.PutUint32(buf[0:4], 0xDEADBEEF)
		payload := base64.StdEncoding.EncodeToString(buf)
		err = pyth.VerifyPythSolanaSignature(payload)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid magic")
	})
}
