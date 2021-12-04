package main

import (
	"log"
	"os"
	"tradingServer/server"
	"tradingServer/servicePriceVariation"
	"tradingServer/serviceUser"
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

func runServer() {
	initPriceMakers()

	s := server.NewServer()
	s.Run()
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "adduser":
			if len(os.Args) < 4 {
				log.Fatalf("missing arguments: %v adduser <login> <password> [<email>]\n", os.Args[0])
			}

			login, password := os.Args[2], os.Args[3]
			email := ""
			if len(os.Args) > 4 {
				email = os.Args[4]
			}
			serviceUser.AddUser(login, password, email)
		default:
			log.Fatalf("unknown subcommand '%v'\n", os.Args[1])
		}
	} else {
		runServer()
	}
}
