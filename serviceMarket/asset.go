package serviceMarket

import (
	"github.com/shopspring/decimal"
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
