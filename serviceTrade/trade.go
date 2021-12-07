package serviceTrade

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"time"
	"tradingServer/entity"
	"tradingServer/storage"
)

func BuyAsset(acc *entity.Account, assetName string, amount decimal.Decimal) error {
	db := storage.GetDatabase()

	assetPrice, err := db.GetAssetPrice(assetName)
	if err != nil {
		return err
	}

	if acc.Balance.LessThan(assetPrice.Mul(amount)) {
		return errors.New("account has not enough money for the requested amount")
	}

	asset := acc.GetOrCreateUserAsset(assetName)

	asset.Amount = asset.Amount.Add(amount)
	acc.Balance = acc.Balance.Sub(amount.Mul(assetPrice))

	err = db.SaveAccount(*acc)
	if err != nil {
		return err
	}

	return db.LogTransaction(storage.TransactionLogEntry{
		Time:   time.Now().Format(time.RFC3339),
		Login:  acc.Login,
		Action: "buy",
		Price:  assetPrice.InexactFloat64(),
		Amount: asset.Amount.InexactFloat64(),
		Asset:  assetName,
	})
}

func SellAsset(acc *entity.Account, assetName string, amount decimal.Decimal) error {
	db := storage.GetDatabase()

	asset := acc.GetOrCreateUserAsset(assetName)

	if asset.Amount.LessThan(amount) {
		return fmt.Errorf("user %v has not enough of %v for the requested amount", acc.Login, assetName)
	}

	assetPrice, err := db.GetAssetPrice(assetName)
	if err != nil {
		return err
	}

	asset.Amount = asset.Amount.Sub(amount)
	acc.Balance = acc.Balance.Add(amount.Mul(assetPrice))

	err = db.SaveAccount(*acc)
	if err != nil {
		return err
	}

	return db.LogTransaction(storage.TransactionLogEntry{
		Time:   time.Now().Format(time.RFC3339),
		Login:  acc.Login,
		Action: "sell",
		Price:  assetPrice.InexactFloat64(),
		Amount: asset.Amount.InexactFloat64(),
		Asset:  assetName,
	})
}
