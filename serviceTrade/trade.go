package serviceTrade

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"log"
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
	payedPrice := amount.Mul(assetPrice)
	acc.Balance = acc.Balance.Sub(payedPrice)

	err = db.SaveAccount(*acc)
	if err != nil {
		return err
	}

	return db.LogTransaction(storage.TransactionLogEntry{
		Time:         time.Now().Format(time.RFC3339),
		Login:        acc.Login,
		Action:       "buy",
		PricePerUnit: assetPrice.InexactFloat64(),
		PricePayed:   payedPrice.InexactFloat64(),
		Amount:       asset.Amount.InexactFloat64(),
		Asset:        assetName,
		Balance:      acc.Balance.InexactFloat64(),
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
	payedPrice := amount.Mul(assetPrice)
	acc.Balance = acc.Balance.Add(payedPrice)

	err = db.SaveAccount(*acc)
	if err != nil {
		return err
	}

	return db.LogTransaction(storage.TransactionLogEntry{
		Time:         time.Now().Format(time.RFC3339),
		Login:        acc.Login,
		Action:       "sell",
		PricePerUnit: assetPrice.InexactFloat64(),
		PricePayed:   payedPrice.InexactFloat64(),
		Amount:       asset.Amount.InexactFloat64(),
		Asset:        assetName,
		Balance:      acc.Balance.InexactFloat64(),
	})
}

func ResetLog() {
	db := storage.GetDatabase()

	q1 := `DELETE FROM access_log`
	res, err := db.Exec(q1)
	if err != nil {
		log.Fatalf("failed clearing access log: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("clearing access log row count failed: %v", err)
	}
	log.Printf("Cleared %v rows from access log.\n", n)

	q2 := `DELETE FROM transaction_log`
	res, err = db.Exec(q2)
	if err != nil {
		log.Fatalf("failed clearing transaction log: %v", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		log.Fatalf("clearing transaction log row count failed: %v", err)
	}
	log.Printf("Cleared %v rows from transaction log.\n", n)
}
