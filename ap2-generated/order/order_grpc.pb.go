package order

import (
	context "context"
	io "io"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const (
	OrderService_SubscribeToOrderUpdates_FullMethodName = "/order.OrderService/SubscribeToOrderUpdates"
)

type OrderServiceClient interface {
	SubscribeToOrderUpdates(ctx context.Context, in *OrderRequest, opts ...grpc.CallOption) (OrderService_SubscribeToOrderUpdatesClient, error)
}

type orderServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewOrderServiceClient(cc grpc.ClientConnInterface) OrderServiceClient {
	return &orderServiceClient{cc}
}

func (c *orderServiceClient) SubscribeToOrderUpdates(ctx context.Context, in *OrderRequest, opts ...grpc.CallOption) (OrderService_SubscribeToOrderUpdatesClient, error) {
	stream, err := c.cc.NewStream(ctx, &OrderService_ServiceDesc.Streams[0], OrderService_SubscribeToOrderUpdates_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &orderServiceSubscribeToOrderUpdatesClient{ClientStream: stream}
	if err := x.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type OrderService_SubscribeToOrderUpdatesClient interface {
	Recv() (*OrderStatusUpdate, error)
	grpc.ClientStream
}

type orderServiceSubscribeToOrderUpdatesClient struct {
	grpc.ClientStream
}

func (x *orderServiceSubscribeToOrderUpdatesClient) Recv() (*OrderStatusUpdate, error) {
	m := new(OrderStatusUpdate)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type OrderServiceServer interface {
	SubscribeToOrderUpdates(*OrderRequest, OrderService_SubscribeToOrderUpdatesServer) error
	mustEmbedUnimplementedOrderServiceServer()
}

type UnimplementedOrderServiceServer struct{}

func (UnimplementedOrderServiceServer) SubscribeToOrderUpdates(*OrderRequest, OrderService_SubscribeToOrderUpdatesServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeToOrderUpdates not implemented")
}

func (UnimplementedOrderServiceServer) mustEmbedUnimplementedOrderServiceServer() {}

func RegisterOrderServiceServer(s grpc.ServiceRegistrar, srv OrderServiceServer) {
	s.RegisterService(&OrderService_ServiceDesc, srv)
}

func _OrderService_SubscribeToOrderUpdates_Handler(srv any, stream grpc.ServerStream) error {
	m := new(OrderRequest)
	if err := stream.RecvMsg(m); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return srv.(OrderServiceServer).SubscribeToOrderUpdates(m, &orderServiceSubscribeToOrderUpdatesServer{ServerStream: stream})
}

type OrderService_SubscribeToOrderUpdatesServer interface {
	Send(*OrderStatusUpdate) error
	grpc.ServerStream
}

type orderServiceSubscribeToOrderUpdatesServer struct {
	grpc.ServerStream
}

func (x *orderServiceSubscribeToOrderUpdatesServer) Send(m *OrderStatusUpdate) error {
	return x.ServerStream.SendMsg(m)
}

var OrderService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "order.OrderService",
	HandlerType: (*OrderServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SubscribeToOrderUpdates",
			Handler:       _OrderService_SubscribeToOrderUpdates_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "order/order.proto",
}
