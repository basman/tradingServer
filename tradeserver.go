package main

import (
	"fmt"
	"log"
	"os"
	"tradingServer/entity"
	"tradingServer/server"
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

	f, err := os.OpenFile("tradingServer.log", os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0644)
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
