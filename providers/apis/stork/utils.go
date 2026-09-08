package stork

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/dydxprotocol/slinky/oracle/config"
)

const (
	// StorkPubKeyEnv is the environment variable holding the trusted Stork
	// aggregator signer addresses (Ethereum addresses) used for signature
	// verification, as a comma-separated list.
	StorkPubKeyEnv = "STORK_PUB_KEY"
)

// DefaultSignerAddresses are the trusted Stork aggregator signer addresses
// used when StorkPubKeyEnv is not set. Deployments should set the variable
// explicitly rather than rely on these.
var DefaultSignerAddresses = []common.Address{
	common.HexToAddress("0x0a803F9b1CCe32e2773e0d2e98b37E0775cA5d44"),
	common.HexToAddress("0x0bb53E0d5E89778DCD13C2720667D292368dD053"),
}

const (
	// Name is the name of the Stork provider.
	Name = "stork_api"

	// URL is the base URL of the Stork REST API for fetching latest prices.
	URL = "https://oracle-relay.dydx.trade/prices"
)

// DefaultAPIConfig is the default configuration for the Stork API.
var DefaultAPIConfig = config.APIConfig{
	Name:             Name,
	Atomic:           false,
	Enabled:          true,
	Timeout:          3000 * time.Millisecond,
	Interval:         3000 * time.Millisecond,
	ReconnectTimeout: 2000 * time.Millisecond,
	MaxQueries:       1,
	Endpoints: []config.Endpoint{{
		URL: URL,
		Authentication: config.Authentication{
			APIKeyHeader: "Authorization",
			APIKey:       "STORK_API_KEY",
		},
	}},
}

// BatchPriceResponse is the top-level response from the Stork API containing
// prices for multiple assets.
type BatchPriceResponse struct {
	Data []PriceResponse `json:"data"`
}

// PriceResponse is a single asset entry within the batch response.
type PriceResponse struct {
	Market                     string                `json:"market"`
	Price                      string                `json:"price"`
	TimestampMs                int64                 `json:"timestampMs"`
	StorkSignatureVerification SignatureVerification `json:"storkSignatureVerification"`
}

// SignatureVerification wraps the aggregator-signed price and the
// individual publisher-signed prices used to derive it.
type SignatureVerification struct {
	StorkSignedPrice SignedPrice            `json:"stork_signed_price"`
	SignedPrices     []PublisherSignedPrice `json:"signed_prices"`
}

// SignedPrice is the aggregator-level signed price produced by Stork.
type SignedPrice struct {
	PublicKey            string               `json:"public_key"`
	EncodedAssetID       string               `json:"encoded_asset_id"`
	Price                string               `json:"price"`
	TimestampedSignature TimestampedSignature `json:"timestamped_signature"`
	PublisherMerkleRoot  string               `json:"publisher_merkle_root"`
	CalculationAlg       CalculationAlg       `json:"calculation_alg"`
}

// TimestampedSignature pairs an EVM ECDSA signature with the timestamp
// and message hash that was signed.
type TimestampedSignature struct {
	Signature EvmSignature `json:"signature"`
	Timestamp int64        `json:"timestamp"`
	MsgHash   string       `json:"msg_hash"`
}

// EvmSignature holds the r, s, v components of a secp256k1 ECDSA signature.
type EvmSignature struct {
	R string `json:"r"`
	S string `json:"s"`
	V string `json:"v"`
}

// CalculationAlg describes how the aggregated price was computed.
type CalculationAlg struct {
	Type     string `json:"type"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

// PublisherSignedPrice is a single publisher's signed price contribution.
type PublisherSignedPrice struct {
	PublisherKey         string               `json:"publisher_key"`
	ExternalAssetID      string               `json:"external_asset_id"`
	SignatureType        string               `json:"signature_type"`
	Price                string               `json:"price"`
	TimestampedSignature TimestampedSignature `json:"timestamped_signature"`
}

// SignerAddressesFromEnv returns the trusted signer addresses configured in
// StorkPubKeyEnv, falling back to DefaultSignerAddresses when it is unset.
func SignerAddressesFromEnv() ([]common.Address, error) {
	raw := strings.TrimSpace(os.Getenv(StorkPubKeyEnv))
	if raw == "" {
		// Copy so that callers cannot mutate the package-level defaults through the handler.
		return append([]common.Address(nil), DefaultSignerAddresses...), nil
	}

	addrs, err := ParseSignerAddresses(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value: %w", StorkPubKeyEnv, err)
	}

	return addrs, nil
}

// ParseSignerAddresses parses a comma-separated list of hex-encoded Ethereum
// addresses. Every entry must be a valid address; blank entries are rejected
// so that a malformed list fails loudly instead of silently shrinking.
func ParseSignerAddresses(raw string) ([]common.Address, error) {
	parts := strings.Split(raw, ",")
	addrs := make([]common.Address, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !common.IsHexAddress(part) {
			return nil, fmt.Errorf("invalid signer address %q", part)
		}
		addrs = append(addrs, common.HexToAddress(part))
	}

	return addrs, nil
}

// VerifyStorkSignature recovers the signer address from the aggregator's
// ECDSA signature and checks that it is one of the trusted signer addresses.
func VerifyStorkSignature(sp SignedPrice, signers []common.Address) error {
	if len(signers) == 0 {
		return fmt.Errorf("no trusted signer addresses configured")
	}

	msgHash := common.FromHex(sp.TimestampedSignature.MsgHash)
	if len(msgHash) != 32 {
		return fmt.Errorf("invalid msg_hash length: got %d, want 32", len(msgHash))
	}

	sig := sp.TimestampedSignature.Signature
	r := common.FromHex(sig.R)
	s := common.FromHex(sig.S)
	v := common.FromHex(sig.V)

	if len(r) != 32 || len(s) != 32 || len(v) == 0 {
		return fmt.Errorf("invalid signature component lengths: r=%d s=%d v=%d", len(r), len(s), len(v))
	}

	// Ethereum signatures use v=27/28; crypto.Ecrecover expects v=0/1.
	vByte := v[len(v)-1]
	if vByte >= 27 {
		vByte -= 27
	}

	sigBytes := make([]byte, 65)
	copy(sigBytes[0:32], r)
	copy(sigBytes[32:64], s)
	sigBytes[64] = vByte

	// Stork signs using EIP-191 personal sign: the actual signed digest is
	// keccak256("\x19Ethereum Signed Message:\n32" || msgHash).
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digest := crypto.Keccak256(append(prefix, msgHash...))

	pubKeyBytes, err := crypto.Ecrecover(digest, sigBytes)
	if err != nil {
		return fmt.Errorf("ecrecover failed: %w", err)
	}

	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal recovered public key: %w", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	for _, signer := range signers {
		if recoveredAddr == signer {
			return nil
		}
	}

	trusted := make([]string, len(signers))
	for i, signer := range signers {
		trusted[i] = signer.Hex()
	}

	return fmt.Errorf("signature mismatch: recovered %s, trusted signers %s",
		recoveredAddr.Hex(), strings.Join(trusted, ","))
}
