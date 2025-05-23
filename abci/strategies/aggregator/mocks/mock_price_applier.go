// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	abcitypes "github.com/cometbft/cometbft/abci/types"

	big "math/big"

	mock "github.com/stretchr/testify/mock"

	pkgtypes "github.com/dydxprotocol/slinky/pkg/types"

	types "github.com/cosmos/cosmos-sdk/types"
)

// PriceApplier is an autogenerated mock type for the PriceApplier type
type PriceApplier struct {
	mock.Mock
}

// ApplyPricesFromVoteExtensions provides a mock function with given fields: ctx, req
func (_m *PriceApplier) ApplyPricesFromVoteExtensions(ctx types.Context, req *abcitypes.RequestFinalizeBlock) (map[pkgtypes.CurrencyPair]*big.Int, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for ApplyPricesFromVoteExtensions")
	}

	var r0 map[pkgtypes.CurrencyPair]*big.Int
	var r1 error
	if rf, ok := ret.Get(0).(func(types.Context, *abcitypes.RequestFinalizeBlock) (map[pkgtypes.CurrencyPair]*big.Int, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(types.Context, *abcitypes.RequestFinalizeBlock) map[pkgtypes.CurrencyPair]*big.Int); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[pkgtypes.CurrencyPair]*big.Int)
		}
	}

	if rf, ok := ret.Get(1).(func(types.Context, *abcitypes.RequestFinalizeBlock) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPricesForValidator provides a mock function with given fields: validator
func (_m *PriceApplier) GetPricesForValidator(validator types.ConsAddress) map[pkgtypes.CurrencyPair]*big.Int {
	ret := _m.Called(validator)

	if len(ret) == 0 {
		panic("no return value specified for GetPricesForValidator")
	}

	var r0 map[pkgtypes.CurrencyPair]*big.Int
	if rf, ok := ret.Get(0).(func(types.ConsAddress) map[pkgtypes.CurrencyPair]*big.Int); ok {
		r0 = rf(validator)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[pkgtypes.CurrencyPair]*big.Int)
		}
	}

	return r0
}

// NewPriceApplier creates a new instance of PriceApplier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPriceApplier(t interface {
	mock.TestingT
	Cleanup(func())
}) *PriceApplier {
	mock := &PriceApplier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
