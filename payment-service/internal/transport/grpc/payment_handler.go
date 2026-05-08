package grpc

import (
	"context"
	"time"

	pb "github.com/W1theri/ap2-generated/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"payment-service/internal/domain"
	"payment-service/internal/usecase"
)

// PaymentGRPCServer implements the PaymentService gRPC contract.
type PaymentGRPCServer struct {
	pb.UnimplementedPaymentServiceServer
	uc *usecase.PaymentUseCase
}

func NewPaymentGRPCServer(uc *usecase.PaymentUseCase) *PaymentGRPCServer {
	return &PaymentGRPCServer{uc: uc}
}

func (s *PaymentGRPCServer) ProcessPayment(
	ctx context.Context,
	req *pb.PaymentRequest,
) (*pb.PaymentResponse, error) {
	result, err := s.uc.Authorize(ctx, usecase.AuthorizeRequest{
		OrderID:       req.OrderId,
		Amount:        req.Amount,
		CustomerEmail: req.CustomerEmail,
	})
	if err != nil {
		if err == domain.ErrAmountMustBePositive {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "authorize: %v", err)
	}

	return &pb.PaymentResponse{
		TransactionId: result.TransactionID,
		Status:        result.Status,
		ProcessedAt:   timestamppb.New(time.Now().UTC()),
	}, nil
}
