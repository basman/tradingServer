package serviceMarket

import (
	"github.com/shopspring/decimal"
	"log"
	"tradingServer/entity"
	"tradingServer/storage"
)

func AddAsset(name string, price float64) error {
	db := storage.GetDatabase()
	defer db.Close()

	ma := entity.MarketAsset{
		Name:  name,
		Price: decimal.NewFromFloat(price),
	}
	return db.CreateMarketAsset(ma)
}

func ResetPrices() {
	initialPrices := make(map[string]float64)
	initialPrices["white_wool"] = 35.0
	initialPrices["black_wool"] = 32.0
	initialPrices["toothpaste"] = 8.5
	initialPrices["old_tires"] = 19.2
	initialPrices["olive_oil"] = 127.0

	db := storage.GetDatabase()

	for name, price := range initialPrices {
		err := db.UpdateMarketAsset(name, price)
		if err != nil {
			log.Fatalf("reset price for %v failed: %v", name, err)
		}
		log.Printf("reset price of %v = %v\n", name, price)
	}
}
