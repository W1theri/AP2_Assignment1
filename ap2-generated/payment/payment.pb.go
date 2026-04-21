package payment

import timestamppb "google.golang.org/protobuf/types/known/timestamppb"

type PaymentRequest struct {
	OrderId string `json:"order_id,omitempty"`
	Amount  int64  `json:"amount,omitempty"`
}

type PaymentResponse struct {
	TransactionId string                 `json:"transaction_id,omitempty"`
	Status        string                 `json:"status,omitempty"`
	ProcessedAt   *timestamppb.Timestamp `json:"processed_at,omitempty"`
}
