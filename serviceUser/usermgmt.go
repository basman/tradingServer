package serviceUser

import (
	"log"
	"tradingServer/storage"
)

func AddUser(login, password, email string) {
	db := storage.GetDatabase()
	if err := db.AddUser(login, password, email); err != nil {
		log.Fatalf("could not create user account: %v", err)
	}

	log.Printf("user account '%v' has been created\n", login)
	db.Close()
}
