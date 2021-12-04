package main

import (
	"log"
	"tradingServer/server"
	"tradingServer/servicePriceVariation"
	"tradingServer/storage"
)

func initPriceMakers() {
	db := storage.GetDatabase()
	assets, err := db.GetAssets()
	if err != nil {
		log.Fatalf("initPriceMakers() failed to fetch assets: %v", err)
	}

	for _, ass := range assets {
		pm := servicePriceVariation.NewPriceMaker(ass.Name, ass.Price)
		go pm.Run()
	}
}

func main() {
	initPriceMakers()

	s := server.NewServer()
	s.Run()
}
