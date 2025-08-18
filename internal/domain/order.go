package domain

import (
	"github.com/google/uuid"
	"time"
)

type Order struct {
	OrderUID          string `json:"order_uid"`
	OrderID           uuid.UUID
	TrackNumber       string       `json:"track_number"`
	Entry             string       `json:"entry"`
	Delivery          DeliveryData `json:"delivery"`
	Payment           PaymentData  `json:"payment"`
	Items             []ItemData   `json:"items"`
	Locale            string       `json:"locale"`
	InternalSignature string       `json:"internal_signature"`
	CustomerID        string       `json:"customer_id"`
	DeliveryService   string       `json:"delivery_service"`
	Shardkey          string       `json:"shardkey"`
	SMID              int          `json:"sm_id"`
	DateCreated       time.Time    `json:"date_created"`
	OofShard          string       `json:"oof_shard"`
}
