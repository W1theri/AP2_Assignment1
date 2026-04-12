package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/YOURUSERNAME/ap2-generated/payment"

	// UseCase остался ТОЧНО таким же, как в Assignment 1.
	// Меняется только этот delivery-слой (было HTTP, стало gRPC).
	"payment-service/internal/domain"
	"payment-service/internal/usecase"
)

// PaymentGRPCServer реализует pb.PaymentServiceServer.
// Это единственный новый файл в payment-service — UseCase и Repository не изменились.
type PaymentGRPCServer struct {
	pb.UnimplementedPaymentServiceServer
	uc *usecase.PaymentUseCase
}

func NewPaymentGRPCServer(uc *usecase.PaymentUseCase) *PaymentGRPCServer {
	return &PaymentGRPCServer{uc: uc}
}

// ProcessPayment — gRPC метод. Вызывает тот же uc.Authorize, что и HTTP-хендлер.
// Таким образом бизнес-логика (лимит $1000, идемпотентность) полностью сохраняется.
func (s *PaymentGRPCServer) ProcessPayment(
	ctx context.Context,
	req *pb.PaymentRequest,
) (*pb.PaymentResponse, error) {

	result, err := s.uc.Authorize(ctx, usecase.AuthorizeRequest{
		OrderID: req.OrderId,
		Amount:  req.Amount, // int64 cents — совпадает с доменом
	})
	if err != nil {
		// Маппинг доменных ошибок в gRPC status codes
		if err == domain.ErrAmountMustBePositive {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "authorize: %v", err)
	}

	return &pb.PaymentResponse{
		TransactionId: result.TransactionID, // поле TransactionID из Assignment 1
		Status:        result.Status,        // "Authorized" | "Declined"
		ProcessedAt:   timestamppb.New(time.Now().UTC()),
	}, nil
}
