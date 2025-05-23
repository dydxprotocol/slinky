// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: slinky/oracle/v1/query.proto

package oraclev1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Query_GetAllCurrencyPairs_FullMethodName        = "/slinky.oracle.v1.Query/GetAllCurrencyPairs"
	Query_GetPrice_FullMethodName                   = "/slinky.oracle.v1.Query/GetPrice"
	Query_GetPrices_FullMethodName                  = "/slinky.oracle.v1.Query/GetPrices"
	Query_GetCurrencyPairMapping_FullMethodName     = "/slinky.oracle.v1.Query/GetCurrencyPairMapping"
	Query_GetCurrencyPairMappingList_FullMethodName = "/slinky.oracle.v1.Query/GetCurrencyPairMappingList"
)

// QueryClient is the client API for Query service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Query is the query service for the x/oracle module.
type QueryClient interface {
	// Get all the currency pairs the x/oracle module is tracking price-data for.
	GetAllCurrencyPairs(ctx context.Context, in *GetAllCurrencyPairsRequest, opts ...grpc.CallOption) (*GetAllCurrencyPairsResponse, error)
	// Given a CurrencyPair (or its identifier) return the latest QuotePrice for
	// that CurrencyPair.
	GetPrice(ctx context.Context, in *GetPriceRequest, opts ...grpc.CallOption) (*GetPriceResponse, error)
	GetPrices(ctx context.Context, in *GetPricesRequest, opts ...grpc.CallOption) (*GetPricesResponse, error)
	// Get the mapping of currency pair ID -> currency pair. This is useful for
	// indexers that have access to the ID of a currency pair, but no way to get
	// the underlying currency pair from it.
	GetCurrencyPairMapping(ctx context.Context, in *GetCurrencyPairMappingRequest, opts ...grpc.CallOption) (*GetCurrencyPairMappingResponse, error)
	// Get the mapping of currency pair ID <-> currency pair as a list. This is
	// useful for indexers that have access to the ID of a currency pair, but no
	// way to get the underlying currency pair from it.
	GetCurrencyPairMappingList(ctx context.Context, in *GetCurrencyPairMappingListRequest, opts ...grpc.CallOption) (*GetCurrencyPairMappingListResponse, error)
}

type queryClient struct {
	cc grpc.ClientConnInterface
}

func NewQueryClient(cc grpc.ClientConnInterface) QueryClient {
	return &queryClient{cc}
}

func (c *queryClient) GetAllCurrencyPairs(ctx context.Context, in *GetAllCurrencyPairsRequest, opts ...grpc.CallOption) (*GetAllCurrencyPairsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetAllCurrencyPairsResponse)
	err := c.cc.Invoke(ctx, Query_GetAllCurrencyPairs_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetPrice(ctx context.Context, in *GetPriceRequest, opts ...grpc.CallOption) (*GetPriceResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetPriceResponse)
	err := c.cc.Invoke(ctx, Query_GetPrice_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetPrices(ctx context.Context, in *GetPricesRequest, opts ...grpc.CallOption) (*GetPricesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetPricesResponse)
	err := c.cc.Invoke(ctx, Query_GetPrices_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetCurrencyPairMapping(ctx context.Context, in *GetCurrencyPairMappingRequest, opts ...grpc.CallOption) (*GetCurrencyPairMappingResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetCurrencyPairMappingResponse)
	err := c.cc.Invoke(ctx, Query_GetCurrencyPairMapping_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetCurrencyPairMappingList(ctx context.Context, in *GetCurrencyPairMappingListRequest, opts ...grpc.CallOption) (*GetCurrencyPairMappingListResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetCurrencyPairMappingListResponse)
	err := c.cc.Invoke(ctx, Query_GetCurrencyPairMappingList_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// QueryServer is the server API for Query service.
// All implementations must embed UnimplementedQueryServer
// for forward compatibility.
//
// Query is the query service for the x/oracle module.
type QueryServer interface {
	// Get all the currency pairs the x/oracle module is tracking price-data for.
	GetAllCurrencyPairs(context.Context, *GetAllCurrencyPairsRequest) (*GetAllCurrencyPairsResponse, error)
	// Given a CurrencyPair (or its identifier) return the latest QuotePrice for
	// that CurrencyPair.
	GetPrice(context.Context, *GetPriceRequest) (*GetPriceResponse, error)
	GetPrices(context.Context, *GetPricesRequest) (*GetPricesResponse, error)
	// Get the mapping of currency pair ID -> currency pair. This is useful for
	// indexers that have access to the ID of a currency pair, but no way to get
	// the underlying currency pair from it.
	GetCurrencyPairMapping(context.Context, *GetCurrencyPairMappingRequest) (*GetCurrencyPairMappingResponse, error)
	// Get the mapping of currency pair ID <-> currency pair as a list. This is
	// useful for indexers that have access to the ID of a currency pair, but no
	// way to get the underlying currency pair from it.
	GetCurrencyPairMappingList(context.Context, *GetCurrencyPairMappingListRequest) (*GetCurrencyPairMappingListResponse, error)
	mustEmbedUnimplementedQueryServer()
}

// UnimplementedQueryServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedQueryServer struct{}

func (UnimplementedQueryServer) GetAllCurrencyPairs(context.Context, *GetAllCurrencyPairsRequest) (*GetAllCurrencyPairsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAllCurrencyPairs not implemented")
}
func (UnimplementedQueryServer) GetPrice(context.Context, *GetPriceRequest) (*GetPriceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPrice not implemented")
}
func (UnimplementedQueryServer) GetPrices(context.Context, *GetPricesRequest) (*GetPricesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPrices not implemented")
}
func (UnimplementedQueryServer) GetCurrencyPairMapping(context.Context, *GetCurrencyPairMappingRequest) (*GetCurrencyPairMappingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCurrencyPairMapping not implemented")
}
func (UnimplementedQueryServer) GetCurrencyPairMappingList(context.Context, *GetCurrencyPairMappingListRequest) (*GetCurrencyPairMappingListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCurrencyPairMappingList not implemented")
}
func (UnimplementedQueryServer) mustEmbedUnimplementedQueryServer() {}
func (UnimplementedQueryServer) testEmbeddedByValue()               {}

// UnsafeQueryServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to QueryServer will
// result in compilation errors.
type UnsafeQueryServer interface {
	mustEmbedUnimplementedQueryServer()
}

func RegisterQueryServer(s grpc.ServiceRegistrar, srv QueryServer) {
	// If the following call pancis, it indicates UnimplementedQueryServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Query_ServiceDesc, srv)
}

func _Query_GetAllCurrencyPairs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAllCurrencyPairsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetAllCurrencyPairs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetAllCurrencyPairs_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetAllCurrencyPairs(ctx, req.(*GetAllCurrencyPairsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetPrice_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPriceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetPrice(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetPrice_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetPrice(ctx, req.(*GetPriceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetPrices_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPricesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetPrices(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetPrices_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetPrices(ctx, req.(*GetPricesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetCurrencyPairMapping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCurrencyPairMappingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetCurrencyPairMapping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetCurrencyPairMapping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetCurrencyPairMapping(ctx, req.(*GetCurrencyPairMappingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetCurrencyPairMappingList_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCurrencyPairMappingListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetCurrencyPairMappingList(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetCurrencyPairMappingList_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetCurrencyPairMappingList(ctx, req.(*GetCurrencyPairMappingListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Query_ServiceDesc is the grpc.ServiceDesc for Query service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Query_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "slinky.oracle.v1.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetAllCurrencyPairs",
			Handler:    _Query_GetAllCurrencyPairs_Handler,
		},
		{
			MethodName: "GetPrice",
			Handler:    _Query_GetPrice_Handler,
		},
		{
			MethodName: "GetPrices",
			Handler:    _Query_GetPrices_Handler,
		},
		{
			MethodName: "GetCurrencyPairMapping",
			Handler:    _Query_GetCurrencyPairMapping_Handler,
		},
		{
			MethodName: "GetCurrencyPairMappingList",
			Handler:    _Query_GetCurrencyPairMappingList_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "slinky/oracle/v1/query.proto",
}
