package pyth_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
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

// buildInnerPayload constructs a Pyth Lazer inner payload (LE) containing a
// single feed with Price and Exponent properties.
func buildInnerPayload(feedID uint32, priceMantissa int64, exponent int16) []byte {
	buf := make([]byte, 31)
	off := 0

	binary.LittleEndian.PutUint32(buf[off:], pyth.PayloadFormatMagic)
	off += 4
	binary.LittleEndian.PutUint64(buf[off:], uint64(time.Now().UnixMicro())) //nolint:gosec // test only, always positive
	off += 8
	buf[off] = 3 // channel_id: FIXED_RATE_200
	off++
	buf[off] = 1 // num_feeds
	off++

	binary.LittleEndian.PutUint32(buf[off:], feedID)
	off += 4
	buf[off] = 2 // num_properties (price + exponent)
	off++

	buf[off] = 0 // propPrice tag
	off++
	binary.LittleEndian.PutUint64(buf[off:], uint64(priceMantissa)) //nolint:gosec // reinterpret signed as unsigned bits
	off += 8

	buf[off] = 4 // propExponent tag
	off++
	binary.LittleEndian.PutUint16(buf[off:], uint16(exponent)) //nolint:gosec // reinterpret signed as unsigned bits

	return buf
}

// buildInnerPayloadPriceOnly constructs a Pyth Lazer inner payload with only
// Price and FeedUpdateTimestamp (no Exponent), matching real production payloads.
func buildInnerPayloadPriceOnly(feedID uint32, priceMantissa int64) []byte {
	// magic(4) + timestamp(8) + channel(1) + numFeeds(1)
	// + feedID(4) + numProps(1)
	// + propPrice tag(1) + i64(8)
	// + propFeedUpdateTimestamp tag(1) + present(1) + u64(8)
	buf := make([]byte, 38)
	off := 0

	binary.LittleEndian.PutUint32(buf[off:], pyth.PayloadFormatMagic)
	off += 4
	ts := uint64(time.Now().UnixMicro()) //nolint:gosec // test only, always positive
	binary.LittleEndian.PutUint64(buf[off:], ts)
	off += 8
	buf[off] = 3 // channel_id
	off++
	buf[off] = 1 // num_feeds
	off++

	binary.LittleEndian.PutUint32(buf[off:], feedID)
	off += 4
	buf[off] = 2 // num_properties (price + feedUpdateTimestamp)
	off++

	buf[off] = 0 // propPrice tag
	off++
	binary.LittleEndian.PutUint64(buf[off:], uint64(priceMantissa)) //nolint:gosec // reinterpret signed as unsigned bits
	off += 8

	buf[off] = 12 // propFeedUpdateTimestamp tag
	off++
	buf[off] = 1 // present
	off++
	binary.LittleEndian.PutUint64(buf[off:], ts)

	return buf
}

// buildSolanaPayload constructs a Pyth Lazer Solana envelope:
//
//	magic(4) || signature(64) || pubkey(32) || msgLen(2) || msg
func buildSolanaPayload(t *testing.T, privKey ed25519.PrivateKey, msg []byte) string {
	t.Helper()
	sig := ed25519.Sign(privKey, msg)

	require.LessOrEqual(t, len(msg), math.MaxUint16)
	buf := make([]byte, 4+64+32+2+len(msg))
	binary.LittleEndian.PutUint32(buf[0:4], pyth.SolanaFormatMagic)
	copy(buf[4:68], sig)
	copy(buf[68:100], privKey.Public().(ed25519.PublicKey))
	binary.LittleEndian.PutUint16(buf[100:102], uint16(len(msg))) //nolint:gosec // bounded above
	copy(buf[102:], msg)

	return base64.StdEncoding.EncodeToString(buf)
}

// signedItemJSON returns a PriceResponse JSON entry with a valid signed Pyth
// Solana payload containing the given feed ID, price mantissa, and exponent.
func signedItemJSON(
	t *testing.T,
	privKey ed25519.PrivateKey,
	feedID uint32,
	priceMantissa int64,
	exponent int16,
) string {
	t.Helper()
	innerPayload := buildInnerPayload(feedID, priceMantissa, exponent)
	solPayload := buildSolanaPayload(t, privKey, innerPayload)
	return fmt.Sprintf(
		`{"market":"%d","price":"0","timestampMs":1774625883288,"pythSolanaPayload":%q}`,
		feedID, solPayload,
	)
}

// signedItemJSONPriceOnly returns a PriceResponse JSON entry with a signed
// payload that has no Exponent property (only Price + FeedUpdateTimestamp),
// plus a JSON price field for fallback.
func signedItemJSONPriceOnly(
	t *testing.T,
	privKey ed25519.PrivateKey,
	feedID uint32,
	priceMantissa int64,
	jsonPrice string,
) string {
	t.Helper()
	innerPayload := buildInnerPayloadPriceOnly(feedID, priceMantissa)
	solPayload := buildSolanaPayload(t, privKey, innerPayload)
	return fmt.Sprintf(
		`{"market":"%d","price":%q,"timestampMs":1774625883288,"pythSolanaPayload":%q}`,
		feedID, jsonPrice, solPayload,
	)
}

// signedBatchJSON builds a full {"data":[...]} response with price+exponent payloads.
func signedBatchJSON(t *testing.T, items ...struct {
	feedID        uint32
	priceMantissa int64
	exponent      int16
},
) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = signedItemJSON(t, priv, item.feedID, item.priceMantissa, item.exponent)
	}
	return `{"data":[` + strings.Join(parts, ",") + `]}`, pub
}

// badSigBatchJSON returns a response where the envelope pubkey differs from PYTH_PUB_KEY.
func badSigBatchJSON(t *testing.T) string {
	t.Helper()
	pub1, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	_, priv2, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub1).String())

	innerPayload := buildInnerPayload(2694, 9726473, -8)
	solPayload := buildSolanaPayload(t, priv2, innerPayload)
	return fmt.Sprintf(
		`{"data":[{"market":"2694","price":"0","timestampMs":1774625883288,"pythSolanaPayload":%q}]}`,
		solPayload,
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
		feedID        uint32
		priceMantissa int64
		exponent      int16
	}

	testCases := []struct {
		name     string
		cps      []types.ProviderTicker
		response func(t *testing.T) *http.Response
		expected types.PriceResponse
	}{
		{
			name: "valid single feed - mantissa 9726473 exp -8 = 0.09726473",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				body, _ := signedBatchJSON(t, item{2694, 9726473, -8})
				return testutils.CreateResponseFromJSON(body)
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
				body, _ := signedBatchJSON(t,
					item{2694, 9726473, -8},
					item{3032, 345678000000, -8},
				)
				return testutils.CreateResponseFromJSON(body)
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
			name: "no exponent in payload - falls back to JSON price",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				pub, priv, err := ed25519.GenerateKey(nil)
				require.NoError(t, err)
				t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

				jsonBody := `{"data":[` +
					signedItemJSONPriceOnly(t, priv, 2694, 10438000, "0.10438000") +
					`]}`
				return testutils.CreateResponseFromJSON(jsonBody)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0.10438)},
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
			name: "feed not in batch response",
			cps:  []types.ProviderTicker{feed2694, feed3032},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				body, _ := signedBatchJSON(t, item{2694, 9726473, -8})
				return testutils.CreateResponseFromJSON(body)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(0.09726473)},
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
			name: "zero price mantissa with exponent produces unresolved",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				body, _ := signedBatchJSON(t, item{2694, 0, -8})
				return testutils.CreateResponseFromJSON(body)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					feed2694: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(
							fmt.Errorf("zero/absent"), providertypes.ErrorAPIGeneral,
						),
					},
				},
			),
		},
		{
			name: "positive exponent - mantissa 5 exp 2 = 500",
			cps:  []types.ProviderTicker{feed2694},
			response: func(t *testing.T) *http.Response {
				t.Helper()
				body, _ := signedBatchJSON(t, item{2694, 5, 2})
				return testutils.CreateResponseFromJSON(body)
			},
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					feed2694: {Value: big.NewFloat(500)},
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

func TestVerifyAndExtractFeed(t *testing.T) {
	t.Run("valid single feed with exponent", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		inner := buildInnerPayload(2694, 9726473, -8)
		payload := buildSolanaPayload(t, priv, inner)

		feed, err := pyth.VerifyAndExtractFeed(payload, 2694)
		require.NoError(t, err)
		require.True(t, feed.HasPrice)
		require.True(t, feed.HasExponent)
		require.Equal(t, int64(9726473), feed.PriceMantissa)
		require.Equal(t, int16(-8), feed.Exponent)

		price, err := feed.ComputePrice()
		require.NoError(t, err)
		expected := big.NewFloat(0.09726473)
		require.Equal(t, expected.SetPrec(18), price.SetPrec(18))
	})

	t.Run("payload without exponent (price + feedUpdateTimestamp only)", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		inner := buildInnerPayloadPriceOnly(2694, 10438000)
		payload := buildSolanaPayload(t, priv, inner)

		feed, err := pyth.VerifyAndExtractFeed(payload, 2694)
		require.NoError(t, err)
		require.True(t, feed.HasPrice)
		require.False(t, feed.HasExponent)
		require.True(t, feed.HasTimestamp)
		require.Equal(t, int64(10438000), feed.PriceMantissa)
		require.Equal(t, uint32(2694), feed.FeedID)
	})

	t.Run("real production payload", func(t *testing.T) {
		// Actual payload from production Pyth Lazer for feed 2694 (WTI/USD).
		// Pubkey (base58): AbGSbqM2M5FeyPNqwiMkGBCqPb63HhR7RK9dBjxJ4mF1
		prodPayload := "uQEagpolDOQCDv+Fd6/CBw7HBUGPxzSY+41jnaBoGCA+eX+fqgdMmWAVMOBTmO23hWBiV0H7xIPReVnaAE9lVt4B0AeA78H0gMVhWvP7Zz1CKH6ZPan7w1BrbkHfoylQggwubCYAddPHk4AM8BtUTgYAAwGGCgAAAgBwRZ8AAAAAAAwBgAzwG1ROBgA="

		// The pubkey embedded in the envelope (hex):
		// 80efc1f480c5615af3fb673d42287e993da9fbc3506b6e41dfa32950820c2e6c
		pubKeyBytes, err := base64.StdEncoding.DecodeString(prodPayload)
		require.NoError(t, err)
		embeddedPubKey := pubKeyBytes[68:100]
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(embeddedPubKey).String())

		feed, err := pyth.VerifyAndExtractFeed(prodPayload, 2694)
		require.NoError(t, err)
		require.Equal(t, uint32(2694), feed.FeedID)
		require.True(t, feed.HasPrice)
		require.Equal(t, int64(10438000), feed.PriceMantissa)
		require.False(t, feed.HasExponent)
		require.True(t, feed.HasTimestamp)
		require.Equal(t, uint64(1774973013200000), feed.TimestampUs)
	})

	t.Run("feed not found in payload", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		inner := buildInnerPayload(2694, 9726473, -8)
		payload := buildSolanaPayload(t, priv, inner)

		_, err = pyth.VerifyAndExtractFeed(payload, 9999)
		require.Error(t, err)
		require.Contains(t, err.Error(), "feed 9999 not found")
	})

	t.Run("wrong public key", func(t *testing.T) {
		pub1, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		_, priv2, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub1).String())

		inner := buildInnerPayload(2694, 9726473, -8)
		payload := buildSolanaPayload(t, priv2, inner)

		_, err = pyth.VerifyAndExtractFeed(payload, 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "public key mismatch")
	})

	t.Run("tampered payload", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		original := buildInnerPayload(2694, 9726473, -8)
		sig := ed25519.Sign(priv, original)

		tampered := buildInnerPayload(2694, 99999999, -8)
		require.LessOrEqual(t, len(tampered), math.MaxUint16)
		buf := make([]byte, 4+64+32+2+len(tampered))
		binary.LittleEndian.PutUint32(buf[0:4], pyth.SolanaFormatMagic)
		copy(buf[4:68], sig)
		copy(buf[68:100], pub)
		binary.LittleEndian.PutUint16(buf[100:102], uint16(len(tampered))) //nolint:gosec // bounded above
		copy(buf[102:], tampered)

		payload := base64.StdEncoding.EncodeToString(buf)
		_, err = pyth.VerifyAndExtractFeed(payload, 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ed25519 signature verification failed")
	})

	t.Run("env var not set", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "")
		_, err := pyth.VerifyAndExtractFeed("dGVzdA==", 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "PYTH_PUB_KEY")
	})

	t.Run("invalid base64", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "11111111111111111111111111111111")
		_, err := pyth.VerifyAndExtractFeed("!!!not-base64!!!", 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "base64")
	})

	t.Run("payload too short", func(t *testing.T) {
		t.Setenv(pyth.PythPubKeyEnv, "11111111111111111111111111111111")
		short := base64.StdEncoding.EncodeToString([]byte("tooshort"))
		_, err := pyth.VerifyAndExtractFeed(short, 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too short")
	})

	t.Run("wrong envelope magic", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		buf := make([]byte, 102)
		binary.LittleEndian.PutUint32(buf[0:4], 0xDEADBEEF)
		payload := base64.StdEncoding.EncodeToString(buf)
		_, err = pyth.VerifyAndExtractFeed(payload, 2694)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid envelope magic")
	})

	t.Run("negative price mantissa", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		inner := buildInnerPayload(2694, -5000000, -5)
		payload := buildSolanaPayload(t, priv, inner)

		feed, err := pyth.VerifyAndExtractFeed(payload, 2694)
		require.NoError(t, err)
		price, err := feed.ComputePrice()
		require.NoError(t, err)
		expected := big.NewFloat(-50.0)
		require.Equal(t, expected.SetPrec(18), price.SetPrec(18))
	})

	t.Run("zero price mantissa", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		t.Setenv(pyth.PythPubKeyEnv, solana.PublicKeyFromBytes(pub).String())

		inner := buildInnerPayload(2694, 0, -8)
		payload := buildSolanaPayload(t, priv, inner)

		feed, err := pyth.VerifyAndExtractFeed(payload, 2694)
		require.NoError(t, err)
		_, err = feed.ComputePrice()
		require.Error(t, err)
		require.Contains(t, err.Error(), "zero/absent")
	})
}
