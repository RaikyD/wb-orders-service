package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/RaikyD/wb-orders-service/internal/repository"
	"github.com/google/uuid"
	"sync"
)

type OrdersService struct {
	repo  repository.OrderRepo
	mu    sync.RWMutex
	byUID map[string]*domain.Order
}

func NewOrdersService(r repository.OrderRepo) *OrdersService {
	return &OrdersService{
		repo:  r,
		byUID: make(map[string]*domain.Order),
	}
}

var ErrOrderAlreadyExists = errors.New("order already exists")

func (s *OrdersService) AddOrder(ctx context.Context, order *domain.Order) error {
	err := s.repo.AddOrder(ctx, order)
	if err != nil {
		if errors.Is(err, repository.ErrOrderAlreadyExists) {
			var o *domain.Order
			var e error
			if order.OrderID != uuid.Nil {
				o, e = s.repo.GetOrderById(ctx, order.OrderID)
			}
			if e == nil && o != nil {
				s.mu.Lock()
				s.byUID[o.OrderUID] = o
				s.mu.Unlock()
			}
			return nil
		}
		logger.Warn("Error while adding order")
		return err
	}

	s.mu.Lock()
	s.byUID[order.OrderUID] = order
	s.mu.Unlock()
	return nil
}

func (s *OrdersService) GetbyUID(ctx context.Context, id string) (*domain.Order, error) {
	s.mu.RLock()
	if o, ok := s.byUID[id]; ok {
		s.mu.RUnlock()
		return o, nil
	}
	s.mu.RUnlock()

	o, err := s.repo.GetOrderByUID(ctx, id)
	if err != nil {
		logger.Warn("Order service getbyUID trouble")
		return nil, err
	}
	if o == nil {
		return nil, nil
	}

	s.mu.Lock()
	s.byUID[o.OrderUID] = o
	s.mu.Unlock()
	return o, nil
}

// limit в нашем случае мб можно ставить 1000 и не париться
func (s *OrdersService) RestoreCache(ctx context.Context, limit int) error {
	rows, err := s.repo.ListRecentPayloads(ctx, limit)
	if err != nil {
		return err
	}

	tmp := make(map[string]*domain.Order, len(rows))
	for _, r := range rows {
		var o domain.Order
		if len(r.Payload) > 0 {
			if err := json.Unmarshal(r.Payload, &o); err != nil {
				logger.Warn("failed to unmarshal payload; skip")
				continue
			}
		} else {
			oo, err := s.repo.GetOrderById(ctx, r.ID)
			if err != nil || oo == nil {
				logger.Warn("failed to load order by id; skip")
				continue
			}
			o = *oo
		}

		o.OrderID = r.ID
		tmp[o.OrderUID] = &o
	}

	s.mu.Lock()
	s.byUID = tmp
	s.mu.Unlock()
	return nil
}
