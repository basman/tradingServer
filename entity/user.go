package entity

import "github.com/shopspring/decimal"

type UserAsset struct {
	Name   string
	Amount decimal.Decimal
}

type Account struct {
	Login   string
	Balance decimal.Decimal
	Assets  []*UserAsset
}

func (acc *Account) GetOrCreateUserAsset(assetName string) *UserAsset {
	for _, ass := range acc.Assets {
		if ass.Name == assetName {
			return ass
		}
	}

	// fallback to empty asset
	ass := &UserAsset{
		Name:   assetName,
		Amount: decimal.Zero,
	}

	acc.Assets = append(acc.Assets, ass)

	return ass
}
