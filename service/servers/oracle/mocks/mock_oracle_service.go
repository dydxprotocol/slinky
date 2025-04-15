// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	types "github.com/dydxprotocol/slinky/service/servers/oracle/types"
)

// OracleService is an autogenerated mock type for the OracleService type
type OracleService struct {
	mock.Mock
}

// MarketMap provides a mock function with given fields: _a0, _a1
func (_m *OracleService) MarketMap(_a0 context.Context, _a1 *types.QueryMarketMapRequest) (*types.QueryMarketMapResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for MarketMap")
	}

	var r0 *types.QueryMarketMapResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryMarketMapRequest) (*types.QueryMarketMapResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryMarketMapRequest) *types.QueryMarketMapResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.QueryMarketMapResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.QueryMarketMapRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Prices provides a mock function with given fields: _a0, _a1
func (_m *OracleService) Prices(_a0 context.Context, _a1 *types.QueryPricesRequest) (*types.QueryPricesResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Prices")
	}

	var r0 *types.QueryPricesResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryPricesRequest) (*types.QueryPricesResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryPricesRequest) *types.QueryPricesResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.QueryPricesResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.QueryPricesRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Start provides a mock function with given fields: _a0
func (_m *OracleService) Start(_a0 context.Context) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Stop provides a mock function with given fields: _a0
func (_m *OracleService) Stop(_a0 context.Context) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Version provides a mock function with given fields: _a0, _a1
func (_m *OracleService) Version(_a0 context.Context, _a1 *types.QueryVersionRequest) (*types.QueryVersionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Version")
	}

	var r0 *types.QueryVersionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryVersionRequest) (*types.QueryVersionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryVersionRequest) *types.QueryVersionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.QueryVersionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.QueryVersionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewOracleService creates a new instance of OracleService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewOracleService(t interface {
	mock.TestingT
	Cleanup(func())
}) *OracleService {
	mock := &OracleService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
