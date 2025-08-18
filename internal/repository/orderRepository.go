package repository

import (
	"context"
	"encoding/json"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
)

type OrderRepo interface {
	AddOrder(ctx context.Context, order *domain.Order) error
	GetOrderById(ctx context.Context, id uuid.UUID) (*domain.Order, error)
}

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(p pgxpool.Pool) (*OrderRepository, error) {
	return &OrderRepository{pool: &p}, nil
}

func (p *OrderRepository) AddOrder(ctx context.Context, o *domain.Order) error {
	payload, err := json.Marshal(&o)
	if err != nil {
		logger.Warn("Error while marshalling json-data")
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Println("Error while starting transaction")
	}

	// Comment then explanation
	defer func() {
		if tx != nil {
			tx.Rollback(ctx)
		}
	}()

	//Start with ordersTable
	var orderID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO wb.orders 
    			(order_uid, track_number, entry, locale, internal_signature, customer_id,
			 	delivery_service, shardkey, sm_id, date_created, oof_shard, payload)
			 VALUES
			     ($1, $2, $3, $4, $5, $6,
			 		$7, $8, $9, $10, $11, $12)
			RETURNING id
			`, o.OrderUID,
		o.TrackNumber,
		o.Entry,
		o.Locale,
		o.InternalSignature,
		o.CustomerID,
		o.DeliveryService,
		o.Shardkey,
		o.SMID,
		o.DateCreated, // timestamptz в схеме
		o.OofShard,
		payload,
	).Scan(&orderID)

	if err != nil {
		logger.Warn("Error occured while working with ordersTable")
		return err
	}

	//Working with delivery-table
	_, err = tx.Exec(ctx,
		`INSERT INTO wb.delivery (order_id, name, phone, zip, city, address, region, email)
			 VALUES
			     ($1, $2, $3, $4, $5, $6, $7, $8)
			`, o.OrderID,
		o.Delivery.Name,
		o.Delivery.Phone,
		o.Delivery.Zip,
		o.Delivery.City,
		o.Delivery.Address,
		o.Delivery.Region,
		o.Delivery.Email,
	)

	if err != nil {
		logger.Warn("Error while working with delivery-table")
		return err
	}

	//Working with payment-table
	pay := o.Payment
	_, err = tx.Exec(ctx, `
		INSERT INTO wb.payment
			(order_id, transaction, request_id, currency, provider,
			 amount_cents, payment_dt, bank, delivery_cost_cents, goods_total_cents, custom_fee_cents)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8, $9, $10, $11)
	`,
		orderID,
		pay.Transaction,
		pay.RequestID,
		pay.Currency,
		pay.Provider,
		pay.Amount,
		pay.PaymentDT,
		pay.Bank,
		pay.DeliveryCost,
		pay.GoodsTotal,
		pay.CustomFee,
	)
	if err != nil {
		return err
	}

	// 4) items — много к одному; используем Batch для эффективности
	if len(o.Items) > 0 {
		batch := &pgx.Batch{}
		for _, it := range o.Items {
			batch.Queue(`
				INSERT INTO wb.items
					(order_id, chrt_id, track_number, price_cents, rid, name, sale, size, total_price_cents, nm_id, brand, status)
				VALUES
					($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`,
				orderID,
				it.ChrtID,
				it.TrackNumber,
				it.Price,
				it.Rid,
				it.Name,
				it.Sale,
				it.Size,
				it.TotalPrice,
				it.NmID,
				it.Brand,
				it.Status,
			)
		}
		br := tx.SendBatch(ctx, batch)
		if err = br.Close(); err != nil {
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		logger.Warn("Error while commiting tx")
	}
	tx = nil
	o.OrderID = orderID
	return nil
}
