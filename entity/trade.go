package entity

import "github.com/shopspring/decimal"

type MarketAsset struct {
	Name  string
	Price decimal.Decimal
}

type Transaction struct {
	Asset  string
	Amount decimal.Decimal
}

