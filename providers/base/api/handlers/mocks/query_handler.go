// Code generated by mockery v2.30.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	types "github.com/dydxprotocol/slinky/providers/types"
)

// QueryHandler is an autogenerated mock type for the QueryHandler type
type QueryHandler[K types.ResponseKey, V types.ResponseValue] struct {
	mock.Mock
}

// Query provides a mock function with given fields: ctx, ids, responseCh
func (_m *QueryHandler[K, V]) Query(ctx context.Context, ids []K, responseCh chan<- types.GetResponse[K, V]) {
	_m.Called(ctx, ids, responseCh)
}

// NewQueryHandler creates a new instance of QueryHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewQueryHandler[K types.ResponseKey, V types.ResponseValue](t interface {
	mock.TestingT
	Cleanup(func())
},
) *QueryHandler[K, V] {
	mock := &QueryHandler[K, V]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
