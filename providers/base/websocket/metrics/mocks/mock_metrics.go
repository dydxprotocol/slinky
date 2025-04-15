// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	metrics "github.com/dydxprotocol/slinky/providers/base/websocket/metrics"

	time "time"
)

// WebSocketMetrics is an autogenerated mock type for the WebSocketMetrics type
type WebSocketMetrics struct {
	mock.Mock
}

// AddWebSocketConnectionStatus provides a mock function with given fields: provider, status
func (_m *WebSocketMetrics) AddWebSocketConnectionStatus(provider string, status metrics.ConnectionStatus) {
	_m.Called(provider, status)
}

// AddWebSocketDataHandlerStatus provides a mock function with given fields: provider, status
func (_m *WebSocketMetrics) AddWebSocketDataHandlerStatus(provider string, status metrics.HandlerStatus) {
	_m.Called(provider, status)
}

// ObserveWebSocketLatency provides a mock function with given fields: provider, duration
func (_m *WebSocketMetrics) ObserveWebSocketLatency(provider string, duration time.Duration) {
	_m.Called(provider, duration)
}

// NewWebSocketMetrics creates a new instance of WebSocketMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewWebSocketMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *WebSocketMetrics {
	mock := &WebSocketMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
