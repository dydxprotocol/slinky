package stork_test

import (
	"fmt"
	"math/big"
	"net/http"
	"testing"
	"time"

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
			url:         fmt.Sprintf("%s?assets=%s", stork.URL, "XAGUSD"),
			expectedErr: false,
		},
		{
			name: "valid multiple",
			cps: []types.ProviderTicker{
				xagusd,
				spxusd,
			},
			url:         fmt.Sprintf("%s?assets=%s", stork.URL, "XAGUSD,SPXUSD"),
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
	testCases := []struct {
		name     string
		cps      []types.ProviderTicker
		response *http.Response
		expected types.PriceResponse
	}{
		{
			name: "valid single",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "30500000000000000000"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: big.NewFloat(30.5),
					},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "valid multiple",
			cps: []types.ProviderTicker{
				xagusd,
				spxusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "30500000000000000000"
						},
						"SPXUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "SPXUSD",
							"price": "5200750000000000000000"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: big.NewFloat(30.5),
					},
					spxusd: {
						Value: big.NewFloat(5200.75),
					},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "bad json response",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`not valid json`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "bad price value",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "$30.50"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("invalid syntax"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "no response for ticker",
			cps: []types.ProviderTicker{
				xagusd,
				spxusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{"data": {}}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorAPIGeneral),
					},
					spxusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "empty price string",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": ""
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("failed to parse price"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "missing data field",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "null data field",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{"data": null}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{},
				types.UnResolvedPrices{
					xagusd: providertypes.UnresolvedResult{
						ErrorWithCode: providertypes.NewErrorWithCode(fmt.Errorf("no response"), providertypes.ErrorAPIGeneral),
					},
				},
			),
		},
		{
			name: "price with hex value",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "0xdeadbeef"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: big.NewFloat(3.735928559e-09),
					},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "very small price value (sub-penny)",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "1"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: new(big.Float).Quo(big.NewFloat(1), new(big.Float).SetFloat64(1e18)),
					},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "very large price value",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "999999999000000000000000000000"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: big.NewFloat(999999999000),
					},
				},
				types.UnResolvedPrices{},
			),
		},
		{
			name: "zero price value",
			cps: []types.ProviderTicker{
				xagusd,
			},
			response: testutils.CreateResponseFromJSON(
				`{
					"data": {
						"XAGUSD": {
							"timestamp": 1234567890000000000,
							"asset_id": "XAGUSD",
							"price": "0"
						}
					}
				}`,
			),
			expected: types.NewPriceResponse(
				types.ResolvedPrices{
					xagusd: {
						Value: big.NewFloat(0),
					},
				},
				types.UnResolvedPrices{},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := stork.NewAPIHandler(stork.DefaultAPIConfig)
			require.NoError(t, err)

			// Update the cache since it is assumed that CreateURL is executed before ParseResponse.
			_, err = h.CreateURL(tc.cps)
			require.NoError(t, err)

			now := time.Now()
			resp := h.ParseResponse(tc.cps, tc.response)

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
