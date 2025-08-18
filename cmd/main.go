package main

import (
	"encoding/json"
	"fmt"
	"github.com/RaikyD/wb-orders-service/internal/config"
	"github.com/RaikyD/wb-orders-service/internal/domain"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"os"
)

func main() {
	_ = godotenv.Load()
	logger, _ := zap.NewDevelopment()
	//logger, _ := zap.NewProduction() //расширенная версия логов, для json выводит поле - значение
	defer logger.Sync()
	sugar := logger.Sugar()
	sugar.Info("Load config data")
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Warn("Error while loading config!", zap.Error(err))
	}

	sugar.Info("successful import!")
	fmt.Println(cfg.DB_STRING)

	t, err := os.Open("./model.json")
	if err != nil {
		sugar.Errorw("open failed", "path", "./model.json", "err", err)
		return
	}
	defer t.Close()

	dec := json.NewDecoder(t)
	dec.DisallowUnknownFields() // полезно для ловли опечаток

	var order domain.Order
	if err := dec.Decode(&order); err != nil {
		sugar.Errorw("decode failed", "err", err)
		return
	}

	sugar.Infow("order decoded", "order", order, "source", "model.json")

}
