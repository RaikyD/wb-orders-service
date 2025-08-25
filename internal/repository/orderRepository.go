package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type OrderRepo interface {
	AddOrder(ctx context.Context, order *domain.Order) error
	GetOrderByUID(ctx context.Context, uid string) (*domain.Order, error)
	GetOrderById(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	ListRecentPayloads(ctx context.Context, limit int) ([]struct {
		ID      uuid.UUID
		Payload []byte
	},
		error)
}

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(p *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: p}
}

var ErrOrderAlreadyExists = errors.New("order already exists")

func (p *OrderRepository) AddOrder(ctx context.Context, o *domain.Order) error {
	payload, err := json.Marshal(o)
	if err != nil {
		logger.Warn("Error while marshalling json-data")
		return err
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		logger.Info("")
		return err
	}

	// If error occur while operations with db during tx we rollback everything
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
		//обрабатываем уникальное нарушение по order_uid
		var pgErr *pgconn.PgError
		// 23505 код для обозначение дупликата
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Заказ уже есть — достанем его id, чтобы вызвать позже кэширование
			if err2 := p.pool.QueryRow(ctx,
				`SELECT id FROM wb.orders WHERE order_uid = $1`, o.OrderUID,
			).Scan(&orderID); err2 == nil {
				o.OrderID = orderID
				return ErrOrderAlreadyExists
			}
			return ErrOrderAlreadyExists
		}

		logger.Warn("insert into wb.orders failed")
		return err
	}

	//Working with delivery-table
	_, err = tx.Exec(ctx,
		`INSERT INTO wb.delivery (order_id, name, phone, zip, city, address, region, email)
			 VALUES
			     ($1, $2, $3, $4, $5, $6, $7, $8)
			`, orderID,
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

	//working with items table
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
		return err
	}
	tx = nil
	o.OrderID = orderID
	return nil
}

func (p *OrderRepository) GetOrderById(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order := &domain.Order{}
	err := p.pool.QueryRow(ctx,
		`
		  SELECT order_uid, track_number, entry, locale, internal_signature,
				 customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard
		  FROM wb.orders WHERE id = $1
		`, id).Scan(
		&order.OrderUID,
		&order.TrackNumber,
		&order.Entry,
		&order.Locale,
		&order.InternalSignature,
		&order.CustomerID,
		&order.DeliveryService,
		&order.Shardkey,
		&order.SMID,
		&order.DateCreated,
		&order.OofShard,
	)
	if err != nil {
		logger.Warn("Error while geting data from wb.orders")
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	err = p.pool.QueryRow(ctx, `
		SELECT
		  name, phone, zip, city, address, region, email
		FROM wb.delivery
		WHERE order_id = $1
	`, id).Scan(
		&order.Delivery.Name,
		&order.Delivery.Phone,
		&order.Delivery.Zip,
		&order.Delivery.City,
		&order.Delivery.Address,
		&order.Delivery.Region,
		&order.Delivery.Email,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	err = p.pool.QueryRow(ctx, `
		SELECT
		  transaction, request_id, currency, provider,
		  amount_cents, payment_dt, bank,
		  delivery_cost_cents, goods_total_cents, custom_fee_cents
		FROM wb.payment
		WHERE order_id = $1
	`, id).Scan(
		&order.Payment.Transaction,
		&order.Payment.RequestID,
		&order.Payment.Currency,
		&order.Payment.Provider,
		&order.Payment.Amount,
		&order.Payment.PaymentDT,
		&order.Payment.Bank,
		&order.Payment.DeliveryCost,
		&order.Payment.GoodsTotal,
		&order.Payment.CustomFee,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	rows, err := p.pool.Query(ctx, `
		SELECT
		  chrt_id, track_number, price_cents, rid, name, sale, size, total_price_cents, nm_id, brand, status
		FROM wb.items
		WHERE order_id = $1
		ORDER BY chrt_id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.ItemData
	for rows.Next() {
		var it domain.ItemData
		err = rows.Scan(
			&it.ChrtID,
			&it.TrackNumber,
			&it.Price,
			&it.Rid,
			&it.Name,
			&it.Sale,
			&it.Size,
			&it.TotalPrice,
			&it.NmID,
			&it.Brand,
			&it.Status,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	order.Items = items
	order.OrderID = id

	return order, nil
}

func (p *OrderRepository) GetOrderByUID(ctx context.Context, uid string) (*domain.Order, error) {
	var id uuid.UUID
	err := p.pool.QueryRow(ctx, `SELECT id FROM wb.orders WHERE order_uid = $1`, uid).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p.GetOrderById(ctx, id)
}

func (p *OrderRepository) ListRecentPayloads(ctx context.Context, limit int) ([]struct {
	ID      uuid.UUID
	Payload []byte
},
	error,
) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, payload
			FROM wb.orders
			ORDER BY created_at DESC
			LIMIT $1
			`, limit)
	if err != nil {
		logger.Warn("Error at gettings cache from db")
		return nil, err
	}
	defer rows.Close()

	var out []struct {
		ID      uuid.UUID
		Payload []byte
	}
	for rows.Next() {
		var id uuid.UUID
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, err
		}
		out = append(out, struct {
			ID      uuid.UUID
			Payload []byte
		}{ID: id, Payload: payload})
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

type OrderBrief struct {
	ID          uuid.UUID `json:"id"`
	OrderUID    string    `json:"order_uid"`
	TrackNumber string    `json:"track_number"`
	CustomerID  string    `json:"customer_id"`
	DateCreated time.Time `json:"date_created"`
	AmountCents *int      `json:"amount_cents"`
}

func (p *OrderRepository) ListOrdersBrief(ctx context.Context, limit, offset int) ([]OrderBrief, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT o.id, o.order_uid, o.track_number, o.customer_id, o.date_created,
		       pay.amount_cents
		FROM wb.orders o
		LEFT JOIN wb.payment pay ON pay.order_id = o.id
		ORDER BY o.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderBrief
	for rows.Next() {
		var r OrderBrief
		if err := rows.Scan(&r.ID, &r.OrderUID, &r.TrackNumber, &r.CustomerID, &r.DateCreated, &r.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
