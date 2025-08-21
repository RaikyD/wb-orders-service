package presentation

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/RaikyD/wb-orders-service/internal/application"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/RaikyD/wb-orders-service/internal/logger"
	"github.com/RaikyD/wb-orders-service/internal/presentation/helpers"
	"github.com/go-chi/chi/v5"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type OrdersHandler struct {
	svc application.OrdersService
}

func NewOrdersHandler(svc application.OrdersService) *OrdersHandler {
	return &OrdersHandler{svc: svc}
}

func (h *OrdersHandler) Register(r chi.Router) {
	r.Post("/orders", h.CreateOrder)
	r.Get("/orders/{uuid}", h.GetOrder)
	r.Post("/orders/generate", h.GenerateOrders)
}

// тут мы будем рассматривать 3 юзер кейса:
// - application/json:   тело сразу объект domain.Order
// - text/plain:         тело — строка JSON (парсим)
// - multipart/form-data: ожидаем файл в поле "file" (parsing .json)
func (h *OrdersHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	mediatype, params, _ := mime.ParseMediaType(ct)

	var ord domain.Order
	var readErr error

	switch mediatype {
	case "application/json":
		readErr = helpers.DecodeJSON(r.Body, &ord)

	case "text/plain":
		// тело — строка, внутри которой JSON
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			readErr = err
			break
		}
		readErr = json.Unmarshal(raw, &ord)

	case "multipart/form-data":
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				readErr = err
				break
			}
			if part.FormName() != "file" {
				continue
			}
			// Читаем файл как JSON
			bufr := bufio.NewReader(io.LimitReader(part, 2<<20))
			readErr = helpers.DecodeJSON(bufr, &ord)
			_ = part.Close()
			break
		}
	default:
		helpers.HttpError(w, http.StatusUnsupportedMediaType, "unsupported content-type")
		return
	}

	if readErr != nil {
		helpers.HttpError(w, http.StatusBadRequest, "invalid JSON: "+readErr.Error())
		return
	}

	// Наличие всех ключей
	if strings.TrimSpace(ord.OrderUID) == "" {
		helpers.HttpError(w, http.StatusBadRequest, "order_uid is required")
		return
	}

	if err := h.svc.AddOrder(r.Context(), &ord); err != nil {
		helpers.HttpError(w, http.StatusInternalServerError, "failed to add order")
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, map[string]any{
		"status":    "ok",
		"order_uid": ord.OrderUID,
	})

}

func (h *OrdersHandler) GetOrder(w http.ResponseWriter, r *http.Request) {

}

func (h *OrdersHandler) GenerateOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("count")
	n := 1
	if q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 1000 {
			n = v
		}
	}

	var created []string
	for i := 0; i < n; i++ {
		o := genDemoOrder()
		if err := h.svc.AddOrder(r.Context(), &o); err != nil {
			// идемпотентность: если дубль — просто пропускаем
			if !errors.Is(err, application.ErrOrderAlreadyExists) {
				logger.Warn("generate: add failed", "err", err)
			}
			continue
		}
		created = append(created, o.OrderUID)
	}

	helpers.WriteJSON(w, http.StatusCreated, map[string]any{
		"status":       "ok",
		"created_uids": created,
	})
}

func (h *OrdersHandler) GetOrderByUID(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	if strings.TrimSpace(uid) == "" {
		helpers.HttpError(w, http.StatusBadRequest, "uid is empty")
		return
	}

	ord, err := h.svc.GetbyUID(r.Context(), uid)
	if err != nil {
		helpers.HttpError(w, http.StatusInternalServerError, "failed to get order")
		return
	}
	if ord == nil {
		helpers.HttpError(w, http.StatusNotFound, "order not found")
		return
	}
	helpers.WriteJSON(w, http.StatusOK, ord)
}

func genDemoOrder() domain.Order {
	now := time.Now().UTC()
	return domain.Order{
		OrderUID:          "demo-" + strconv.FormatInt(now.UnixNano(), 10),
		TrackNumber:       "WB" + strconv.FormatInt(now.Unix()%1_000_000, 10),
		Entry:             "WBIL",
		Locale:            "ru",
		InternalSignature: "",
		CustomerID:        "customer-1",
		DeliveryService:   "meest",
		Shardkey:          "0",
		SMID:              0,
		DateCreated:       now,
		OofShard:          "0",
		Delivery: domain.DeliveryData{
			Name:    "Ivan Petrov",
			Phone:   "+7 999 111-22-33",
			Zip:     "101000",
			City:    "Moscow",
			Address: "Tverskaya, 1",
			Region:  "Moscow",
			Email:   "ivan@example.com",
		},
		Payment: domain.PaymentData{
			Transaction:  "tr-" + strconv.FormatInt(now.UnixNano(), 10),
			RequestID:    "",
			Currency:     "RUB",
			Provider:     "wbpay",
			Amount:       10000,
			PaymentDT:    now.Unix(),
			Bank:         "alpha",
			DeliveryCost: 200,
			GoodsTotal:   9800,
			CustomFee:    0,
		},
		Items: []domain.ItemData{
			{
				ChrtID:      1,
				TrackNumber: "WB123",
				Price:       9800,
				Rid:         "ab-1",
				Name:        "T-shirt",
				Sale:        0,
				Size:        "L",
				TotalPrice:  9800,
				NmID:        123,
				Brand:       "WB",
				Status:      202,
			},
		},
	}
}
