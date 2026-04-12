package client

// Этот файл ЗАМЕНЯЕТ payment_client.go из Assignment 1.
// Интерфейс usecase.PaymentClient остался прежним — поменялась только реализация:
// вместо HTTP теперь используется gRPC.
//
// usecase/ports.go (НЕ ИЗМЕНИЛСЯ):
//   type PaymentClient interface {
//       Authorize(ctx context.Context, orderID string, amount int64) (*PaymentResult, error)
//   }

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/YOURUSERNAME/ap2-generated/payment"
	"order-service/internal/usecase"
)

// GRPCPaymentClient реализует usecase.PaymentClient через gRPC.
// Является прямой заменой HTTPPaymentClient из Assignment 1.
type GRPCPaymentClient struct {
	client pb.PaymentServiceClient
}

// NewGRPCPaymentClient создаёт gRPC клиент по адресу из env (не хардкод).
func NewGRPCPaymentClient(addr string) (*GRPCPaymentClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &GRPCPaymentClient{
		client: pb.NewPaymentServiceClient(conn),
	}, nil
}

// Authorize реализует usecase.PaymentClient.
// Сигнатура идентична старому HTTPPaymentClient.Authorize — use case не меняется.
func (c *GRPCPaymentClient) Authorize(
	ctx context.Context,
	orderID string,
	amount int64, // cents, как в domain.Order.Amount
) (*usecase.PaymentResult, error) {

	resp, err := c.client.ProcessPayment(ctx, &pb.PaymentRequest{
		OrderId: orderID,
		Amount:  amount,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc ProcessPayment: %w", err)
	}

	return &usecase.PaymentResult{
		TransactionID: resp.TransactionId, // маппинг в те же поля, что были
		Status:        resp.Status,        // "Authorized" | "Declined"
	}, nil
}
