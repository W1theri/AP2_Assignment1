package order

import timestamppb "google.golang.org/protobuf/types/known/timestamppb"

type OrderRequest struct {
	OrderId string `json:"order_id,omitempty"`
}

type OrderStatusUpdate struct {
	OrderId   string                 `json:"order_id,omitempty"`
	Status    string                 `json:"status,omitempty"`
	UpdatedAt *timestamppb.Timestamp `json:"updated_at,omitempty"`
}
