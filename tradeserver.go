package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"tradingServer/entity"
	"tradingServer/server"
	"tradingServer/serviceMarket"
	"tradingServer/servicePriceVariation"
	"tradingServer/serviceUser"
	"tradingServer/storage"
)

func initPriceMakers(ev chan entity.MarketAsset) {
	db := storage.GetDatabase()
	assets, err := db.GetAssets()
	if err != nil {
		log.Fatalf("initPriceMakers() failed to fetch assets: %v", err)
	}

	for _, ass := range assets {
		pm := servicePriceVariation.NewPriceMaker(ass.Name, ass.Price, ev)
		go pm.Run()
	}
}

func runServer() {
	s := server.NewServer()

	f, err := os.OpenFile("tradingServer.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("failed to open or create tradingServer.log: %v", err)
	} else {
		log.SetOutput(f)
	}

	initPriceMakers(s.GetEventInputChannel())
	s.Run()
}

func usage() {
	fmt.Println(`usage: ./tradingServer <command> <options...>
commands:
	adduser <login> <password> [<email>]
		Create new user account.
	initdb
		Initialise database. User accounts and assets will be deleted.
	setpw <login> <password>
		Reset user's password.
	setemail <login> <email>
		Update user's email address.

	Without any sub command given the server will start up and wait for incoming requests.`)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "adduser":
			if len(os.Args) < 4 {
				fmt.Printf("missing arguments: %v adduser <login> <password> [<email>]\n", os.Args[0])
				os.Exit(1)
			}

			login, password := os.Args[2], os.Args[3]
			email := ""
			if len(os.Args) > 4 {
				email = os.Args[4]
			}
			serviceUser.AddUser(login, password, email)
		case "addasset":
			if len(os.Args) < 3 {
				fmt.Printf("missing arguments: %v addasset <name> <price>\n", os.Args[0])
				os.Exit(1)
			}
			name, priceStr := os.Args[2], os.Args[3]
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				fmt.Printf("price is not a number: %v", priceStr)
				os.Exit(1)
			}
			err = serviceMarket.AddAsset(name, price)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		case "initdb":
			if len(os.Args) > 2 {
				fmt.Printf("invalid arguments: %v (initdb takes no parameters)\n", strings.Join(os.Args[2:], " "))
			}

			serviceUser.RemoveUsers()
			serviceMarket.ResetPrices()
		case "setpw":
			if len(os.Args) < 3 {
				fmt.Printf("missing arguments: %v setpw <login> <password>\n", os.Args[0])
				os.Exit(1)
			}
			login, password := os.Args[2], os.Args[3]
			serviceUser.ChangeUser(login, password, "")
		case "setemail":
			if len(os.Args) < 3 {
				fmt.Printf("missing arguments: %v setemail <login> <email>\n", os.Args[0])
				os.Exit(1)
			}
			login, email := os.Args[2], os.Args[3]
			serviceUser.ChangeUser(login, "", email)
		case "help":
			fallthrough
		case "-help":
			fallthrough
		case "--help":
			fallthrough
		case "-h":
			usage()
			return
		default:
			log.Fatalf("unknown subcommand '%v'\n", os.Args[1])
		}
	} else {
		runServer()
	}
}
