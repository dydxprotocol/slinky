package pyth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/gagliardetto/solana-go"

	"github.com/dydxprotocol/slinky/oracle/config"
)

const (
	PythPubKeyEnv = "PYTH_PUB_KEY"
)

const (
	Name = "pyth_api"

	URL = "https://oracle-relay.dydx.trade/prices"

	// SolanaFormatMagic is the Pyth Lazer Solana envelope magic (LE u32).
	SolanaFormatMagic uint32 = 0x821a01b9

	// solanaEnvelopeMinLen is magic(4) + sig(64) + pubkey(32) + msgLen(2).
	solanaEnvelopeMinLen = 102

	// PayloadFormatMagic is the first 4 bytes (LE) of the inner signed payload.
	// Decimal 2479346549.
	PayloadFormatMagic uint32 = 0x93C7D375
)

// PriceFeedProperty discriminant values from the Pyth Lazer protocol.
const (
	propPrice               uint8 = 0
	propBestBidPrice        uint8 = 1
	propBestAskPrice        uint8 = 2
	propPublisherCount      uint8 = 3
	propExponent            uint8 = 4
	propConfidence          uint8 = 5
	propFundingRate         uint8 = 6
	propFundingTimestamp    uint8 = 7
	propFundingRateInterval uint8 = 8
	propMarketSession       uint8 = 9
	propEmaPrice            uint8 = 10
	propEmaConfidence       uint8 = 11
	propFeedUpdateTimestamp uint8 = 12
)

var DefaultAPIConfig = config.APIConfig{
	Name:             Name,
	Atomic:           false,
	Enabled:          true,
	Timeout:          3000 * time.Millisecond,
	Interval:         3000 * time.Millisecond,
	ReconnectTimeout: 2000 * time.Millisecond,
	MaxQueries:       1,
	Endpoints:        []config.Endpoint{{URL: URL}},
}

// BatchPriceResponse is the top-level response containing prices for multiple feeds.
type BatchPriceResponse struct {
	Data []PriceResponse `json:"data"`
}

// PriceResponse is a single feed entry within the batch response.
type PriceResponse struct {
	Market            string `json:"market"`
	Price             string `json:"price"`
	TimestampMs       int64  `json:"timestampMs"`
	PythSolanaPayload string `json:"pythSolanaPayload"`
}

// ParsedFeedPrice holds price data extracted from a signed Pyth Lazer payload.
type ParsedFeedPrice struct {
	FeedID        uint32
	PriceMantissa int64
	HasPrice      bool
	Exponent      int16
	HasExponent   bool
	TimestampUs   uint64
	HasTimestamp  bool
}

// ComputePrice returns mantissa * 10^exponent as a *big.Float.
func (p *ParsedFeedPrice) ComputePrice() (*big.Float, error) {
	if !p.HasPrice {
		return nil, fmt.Errorf("no price property in signed payload for feed %d", p.FeedID)
	}
	if !p.HasExponent {
		return nil, fmt.Errorf("no exponent property in signed payload for feed %d", p.FeedID)
	}
	if p.PriceMantissa == 0 {
		return nil, fmt.Errorf("price is zero/absent in signed payload for feed %d", p.FeedID)
	}

	mantissa := new(big.Float).SetInt64(p.PriceMantissa)
	exp := int(p.Exponent)
	factor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(exp))), nil))

	if exp >= 0 {
		return mantissa.Mul(mantissa, factor), nil
	}
	return mantissa.Quo(mantissa, factor), nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// VerifyAndExtractFeed decodes a base64 Pyth Lazer Solana payload, verifies
// the ed25519 signature against the trusted PYTH_PUB_KEY, parses the signed
// message, and returns the ParsedFeedPrice for the requested feedID.
//
// The caller should use ComputePrice() if both price and exponent are present.
// If the subscription doesn't include exponent, the caller may fall back to
// the JSON price while still benefiting from signature + feed ID verification.
//
// Solana envelope layout:
//
//	[0..4)            magic     – LE u32, must be 0x821a01b9
//	[4..68)           signature – 64-byte ed25519 signature
//	[68..100)         pubkey    – 32-byte ed25519 public key
//	[100..102)        msgLen    – LE u16
//	[102..102+msgLen) msg       – signed payload bytes
//
// Inner payload layout (LE):
//
//	[0..4)   PAYLOAD_FORMAT_MAGIC (LE u32 = 0x93C7D375)
//	[4..12)  timestamp_us (LE u64)
//	[12]     channel_id (u8)
//	[13]     num_feeds (u8)
//	Per feed:
//	  feed_id (LE u32), num_properties (u8),
//	  then tag(u8)+value pairs per property.
func VerifyAndExtractFeed(payloadBase64 string, feedID uint32) (*ParsedFeedPrice, error) {
	expectedKeyStr := os.Getenv(PythPubKeyEnv)
	if expectedKeyStr == "" {
		expectedKeyStr = "9gKEEcFzSd1PDYBKWAKZi4Sq4ZCUaVX5oTr8kEjdwsfR"
	}

	expectedKey, err := solana.PublicKeyFromBase58(expectedKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value: %w", PythPubKeyEnv, err)
	}

	data, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 payload: %w", err)
	}

	if len(data) < solanaEnvelopeMinLen {
		return nil, fmt.Errorf("payload too short: %d bytes, minimum %d", len(data), solanaEnvelopeMinLen)
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != SolanaFormatMagic {
		return nil, fmt.Errorf("invalid envelope magic: got 0x%08x, want 0x%08x", magic, SolanaFormatMagic)
	}

	sig := data[4:68]
	pubKey := data[68:100]
	msgSize := int(binary.LittleEndian.Uint16(data[100:102]))

	if len(data) < solanaEnvelopeMinLen+msgSize {
		return nil, fmt.Errorf("payload truncated: need %d bytes, have %d", solanaEnvelopeMinLen+msgSize, len(data))
	}

	msg := data[102 : 102+msgSize]

	if !bytes.Equal(pubKey, expectedKey[:]) {
		return nil, fmt.Errorf("public key mismatch: got %s, expected %s",
			solana.PublicKeyFromBytes(pubKey).String(), expectedKey.String())
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKey), msg, sig) {
		return nil, fmt.Errorf("ed25519 signature verification failed")
	}

	feeds, err := parsePythPayload(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed payload: %w", err)
	}

	for i := range feeds {
		if feeds[i].FeedID == feedID {
			return &feeds[i], nil
		}
	}

	return nil, fmt.Errorf("feed %d not found in signed payload (have %d feeds)", feedID, len(feeds))
}

// parsePythPayload parses the inner LE-encoded Pyth Lazer payload message,
// extracting feed IDs and their properties.
func parsePythPayload(msg []byte) ([]ParsedFeedPrice, error) {
	if len(msg) < 14 {
		return nil, fmt.Errorf("payload too short: %d bytes", len(msg))
	}

	payloadMagic := binary.LittleEndian.Uint32(msg[0:4])
	if payloadMagic != PayloadFormatMagic {
		return nil, fmt.Errorf("invalid payload magic: got 0x%08x, want 0x%08x", payloadMagic, PayloadFormatMagic)
	}

	// timestamp_us at [4..12), channel_id at [12], num_feeds at [13]
	numFeeds := msg[13]
	off := 14

	feeds := make([]ParsedFeedPrice, 0, numFeeds)
	for i := 0; i < int(numFeeds); i++ {
		if off+5 > len(msg) {
			return nil, fmt.Errorf("payload truncated reading feed %d header", i)
		}
		feedID := binary.LittleEndian.Uint32(msg[off : off+4])
		numProps := msg[off+4]
		off += 5

		feed := ParsedFeedPrice{FeedID: feedID}
		for j := 0; j < int(numProps); j++ {
			if off >= len(msg) {
				return nil, fmt.Errorf("payload truncated reading property %d of feed %d", j, i)
			}
			propTag := msg[off]
			off++

			var err error
			off, err = parseProperty(msg, off, propTag, &feed)
			if err != nil {
				return nil, fmt.Errorf("feed %d property %d (tag %d): %w", i, j, propTag, err)
			}
		}
		feeds = append(feeds, feed)
	}

	return feeds, nil
}

// parseProperty reads a single property value from msg at the given offset,
// populates feed fields as appropriate, and returns the new offset.
func parseProperty(msg []byte, off int, tag uint8, feed *ParsedFeedPrice) (int, error) {
	switch tag {
	case propPrice, propBestBidPrice, propBestAskPrice, propConfidence,
		propEmaPrice, propEmaConfidence:
		if off+8 > len(msg) {
			return 0, fmt.Errorf("need 8 bytes for i64 property, have %d", len(msg)-off)
		}
		val := int64(binary.LittleEndian.Uint64(msg[off : off+8])) //nolint:gosec // reinterpreting unsigned bits as signed per Pyth wire format
		if tag == propPrice {
			feed.PriceMantissa = val
			feed.HasPrice = true
		}
		return off + 8, nil

	case propPublisherCount:
		if off+2 > len(msg) {
			return 0, fmt.Errorf("need 2 bytes for u16 property, have %d", len(msg)-off)
		}
		return off + 2, nil

	case propExponent:
		if off+2 > len(msg) {
			return 0, fmt.Errorf("need 2 bytes for i16 property, have %d", len(msg)-off)
		}
		feed.Exponent = int16(binary.LittleEndian.Uint16(msg[off : off+2])) //nolint:gosec // reinterpreting unsigned bits as signed per Pyth wire format
		feed.HasExponent = true
		return off + 2, nil

	case propMarketSession:
		if off+2 > len(msg) {
			return 0, fmt.Errorf("need 2 bytes for i16 property, have %d", len(msg)-off)
		}
		return off + 2, nil

	case propFundingRate:
		// u8 present flag + optional i64
		if off >= len(msg) {
			return 0, fmt.Errorf("need 1 byte for presence flag, have %d", len(msg)-off)
		}
		if msg[off] != 0 {
			if off+9 > len(msg) {
				return 0, fmt.Errorf("need 9 bytes for optional i64, have %d", len(msg)-off)
			}
			return off + 9, nil
		}
		return off + 1, nil

	case propFundingTimestamp, propFundingRateInterval, propFeedUpdateTimestamp:
		// u8 present flag + optional u64
		if off >= len(msg) {
			return 0, fmt.Errorf("need 1 byte for presence flag, have %d", len(msg)-off)
		}
		if msg[off] != 0 {
			if off+9 > len(msg) {
				return 0, fmt.Errorf("need 9 bytes for optional u64, have %d", len(msg)-off)
			}
			if tag == propFeedUpdateTimestamp {
				feed.TimestampUs = binary.LittleEndian.Uint64(msg[off+1 : off+9])
				feed.HasTimestamp = true
			}
			return off + 9, nil
		}
		return off + 1, nil

	default:
		return 0, fmt.Errorf("unknown property tag %d", tag)
	}
}
