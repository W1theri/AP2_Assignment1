package grpc

import (
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/YOURUSERNAME/ap2-generated/order"
	"order-service/internal/repository"
)

// OrderGRPCServer реализует pb.OrderServiceServer.
// Order Service здесь выступает gRPC СЕРВЕРОМ (streaming endpoint).
// Для ProcessPayment Order Service выступает gRPC КЛИЕНТОМ (см. client/payment_grpc_client.go).
type OrderGRPCServer struct {
	pb.UnimplementedOrderServiceServer
	repo *repository.PostgresOrderRepository
}

func NewOrderGRPCServer(repo *repository.PostgresOrderRepository) *OrderGRPCServer {
	return &OrderGRPCServer{repo: repo}
}

// SubscribeToOrderUpdates — Server-side Streaming RPC.
//
// Логика:
//  1. Клиент отправляет order_id
//  2. Сервер немедленно присылает текущий статус из БД
//  3. При любом изменении статуса в таблице orders — мгновенно шлёт новое значение
//  4. Стрим живёт пока клиент подключён (или до закрытия контекста)
//
// Реальное взаимодействие с БД через WatchOrderStatus (polling каждые 500мс).
func (s *OrderGRPCServer) SubscribeToOrderUpdates(
	req *pb.OrderRequest,
	stream pb.OrderService_SubscribeToOrderUpdatesServer,
) error {
	if req.OrderId == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	log.Printf("[stream] client subscribed to order_id=%s", req.OrderId)

	ctx := stream.Context()
	done := ctx.Done()

	statusCh, errCh := s.repo.WatchOrderStatus(req.OrderId, done)

	for {
		select {
		case <-done:
			// Клиент отключился или таймаут
			log.Printf("[stream] client disconnected from order_id=%s", req.OrderId)
			return nil

		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			log.Printf("[stream] watch error for order_id=%s: %v", req.OrderId, err)
			return status.Errorf(codes.Internal, "watch error: %v", err)

		case newStatus, ok := <-statusCh:
			if !ok {
				return nil
			}

			update := &pb.OrderStatusUpdate{
				OrderId:   req.OrderId,
				Status:    newStatus, // "Pending" | "Paid" | "Failed" | "Cancelled"
				UpdatedAt: timestamppb.New(time.Now().UTC()),
			}

			if err := stream.Send(update); err != nil {
				return status.Errorf(codes.Internal, "send failed: %v", err)
			}

			log.Printf("[stream] pushed status=%s for order_id=%s", newStatus, req.OrderId)
		}
	}
}
