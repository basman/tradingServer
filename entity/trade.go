package entity

import (
	"github.com/shopspring/decimal"
	"time"
)

type MarketAsset struct {
	Name  string
	Price decimal.Decimal
	When  time.Time
}

type Transaction struct {
	Asset  string
	Amount decimal.Decimal
}

