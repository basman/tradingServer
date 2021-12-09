package entity

import (
	"crypto/sha1"
	"encoding/base64"
	"github.com/shopspring/decimal"
	"log"
	"strings"
)

type UserAsset struct {
	Name   string
	Amount decimal.Decimal
}

type PublicAccount struct {
	Login    string
	Balance  decimal.Decimal
	Assets   []*UserAsset
}

type Account struct {
	PublicAccount
	Email    string
	password string
}

func (acc *Account) SetPassword(pw string) {
	acc.password = pw
}

func (acc *Account) GetPassword() string {
	return acc.password
}

func (acc *Account) SetAndHashPassword(pw string) {
	acc.password = HashEncodePassword(pw)
}

func (acc *Account) VerifyPassword(pw string) bool {
	hashedPw := HashEncodePassword(pw)
	return acc.password == hashedPw
}

func HashEncodePassword(pw string) string {
	h := sha1.Sum([]byte(pw))

	var mimeEncodedHash = &strings.Builder{}
	enc := base64.NewEncoder(base64.StdEncoding, mimeEncodedHash)
	_, err := enc.Write(h[:])
	if err != nil {
		log.Printf("password hashing failed: %v", err)
		return ""
	}

	return mimeEncodedHash.String()
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
