package pyth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/gagliardetto/solana-go"

	"github.com/dydxprotocol/slinky/oracle/config"
)

const (
	// PythPubKeyEnv is the environment variable holding the trusted Pyth
	// signer public key (base58-encoded Solana ed25519 public key).
	PythPubKeyEnv = "PYTH_PUB_KEY"
)

const (
	Name = "pyth_api"

	URL = "http://localhost:8444/prices"

	// SolanaFormatMagic is the Pyth Lazer Solana format magic (LE u32).
	SolanaFormatMagic uint32 = 0x821a01b9

	// solanaPayloadMinLen is magic(4) + sig(64) + pubkey(32) + msgLen(2).
	solanaPayloadMinLen = 102
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

// VerifyPythSolanaSignature decodes the base64 Pyth Lazer Solana-format
// payload, verifies the ed25519 signature, and checks that the embedded
// public key matches the trusted key from PYTH_PUB_KEY.
//
// Payload layout (Pyth Lazer "solana" format):
//
//	[0..4)    magic     – LE u32, must be 0x821a01b9
//	[4..68)   signature – 64-byte ed25519 signature
//	[68..100) pubkey    – 32-byte ed25519 public key
//	[100..102) msgLen   – LE u16, length of signed message
//	[102..102+msgLen)   – signed message bytes
func VerifyPythSolanaSignature(payloadBase64 string) error {
	expectedKeyStr := os.Getenv(PythPubKeyEnv)
	if expectedKeyStr == "" {
		return fmt.Errorf("%s environment variable is not set", PythPubKeyEnv)
	}

	expectedKey, err := solana.PublicKeyFromBase58(expectedKeyStr)
	if err != nil {
		return fmt.Errorf("invalid %s value: %w", PythPubKeyEnv, err)
	}

	data, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 payload: %w", err)
	}

	if len(data) < solanaPayloadMinLen {
		return fmt.Errorf("payload too short: %d bytes, minimum %d", len(data), solanaPayloadMinLen)
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != SolanaFormatMagic {
		return fmt.Errorf("invalid magic: got 0x%08x, want 0x%08x", magic, SolanaFormatMagic)
	}

	sig := data[4:68]
	pubKey := data[68:100]
	msgSize := int(binary.LittleEndian.Uint16(data[100:102]))

	if len(data) < solanaPayloadMinLen+msgSize {
		return fmt.Errorf("payload truncated: need %d bytes, have %d", solanaPayloadMinLen+msgSize, len(data))
	}

	msg := data[102 : 102+msgSize]

	if !bytes.Equal(pubKey, expectedKey[:]) {
		return fmt.Errorf("public key mismatch: got %s, expected %s",
			solana.PublicKeyFromBytes(pubKey).String(), expectedKey.String())
	}

	if !ed25519.Verify(ed25519.PublicKey(pubKey), msg, sig) {
		return fmt.Errorf("ed25519 signature verification failed")
	}

	return nil
}
