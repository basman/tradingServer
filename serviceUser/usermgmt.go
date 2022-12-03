package serviceUser

import (
	"log"
	"tradingServer/entity"
	"tradingServer/storage"
)

func AddUser(login, password, email string) {
	db := storage.GetDatabase()
	if err := db.AddAccount(login, password, email); err != nil {
		log.Fatalf("could not create user account: %v", err)
	}

	log.Printf("user account '%v' has been created\n", login)
}

func ChangeUser(login, password, email string) {
	db := storage.GetDatabase()
	if err := db.UpdateAccount(login, password, email); err != nil {
		log.Fatalf("could not update user account: %v", err)
	}
	log.Printf("user account '%v' has been updated\n", login)
}

// RemoveUsers deletes all user accounts except "roman"
func RemoveUsers() {
	const exception = "roman"
	db := storage.GetDatabase()
	var accs []*entity.PublicAccount
	var err error
	if accs, err = db.GetAccounts(); err != nil {
		log.Fatalf("failed to list accounts: %v", err)
	}

	for _, a := range accs {
		if a.Login == exception {
			log.Printf("skipping account %v\n", a.Login)
			continue
		}

		err = db.RemoveAccount(a)
		if err != nil {
			log.Fatalf("failed to remove account '%v': %v", a.Login, err)
		}
		log.Printf("deleted account '%v'\n", a.Login)
	}
}
