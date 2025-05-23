// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	metrics "github.com/dydxprotocol/slinky/service/metrics"

	time "time"

	types "github.com/dydxprotocol/slinky/pkg/types"
)

// Metrics is an autogenerated mock type for the Metrics type
type Metrics struct {
	mock.Mock
}

// AddABCIRequest provides a mock function with given fields: method, status
func (_m *Metrics) AddABCIRequest(method metrics.ABCIMethod, status metrics.Labeller) {
	_m.Called(method, status)
}

// AddOracleResponse provides a mock function with given fields: status
func (_m *Metrics) AddOracleResponse(status metrics.Labeller) {
	_m.Called(status)
}

// AddValidatorPriceForTicker provides a mock function with given fields: validator, ticker, price
func (_m *Metrics) AddValidatorPriceForTicker(validator string, ticker types.CurrencyPair, price float64) {
	_m.Called(validator, ticker, price)
}

// AddValidatorReportForTicker provides a mock function with given fields: validator, ticker, status
func (_m *Metrics) AddValidatorReportForTicker(validator string, ticker types.CurrencyPair, status metrics.ReportStatus) {
	_m.Called(validator, ticker, status)
}

// ObserveABCIMethodLatency provides a mock function with given fields: method, duration
func (_m *Metrics) ObserveABCIMethodLatency(method metrics.ABCIMethod, duration time.Duration) {
	_m.Called(method, duration)
}

// ObserveMessageSize provides a mock function with given fields: msg, size
func (_m *Metrics) ObserveMessageSize(msg metrics.MessageType, size int) {
	_m.Called(msg, size)
}

// ObserveOracleResponseLatency provides a mock function with given fields: duration
func (_m *Metrics) ObserveOracleResponseLatency(duration time.Duration) {
	_m.Called(duration)
}

// ObservePriceForTicker provides a mock function with given fields: ticker, price
func (_m *Metrics) ObservePriceForTicker(ticker types.CurrencyPair, price float64) {
	_m.Called(ticker, price)
}

// NewMetrics creates a new instance of Metrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *Metrics {
	mock := &Metrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
