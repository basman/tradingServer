package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shopspring/decimal"
	"log"
	"os"
	"strings"
	"sync"
	"tradingServer/entity"
)

const dbFile = "database.sqlite3"

type Database struct {
	*sql.DB
}

var db *Database
var dbMu sync.Mutex

func GetDatabase() *Database {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		return db
	}

	freshlyCreated := false
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		freshlyCreated = true
		f, err := os.Create(dbFile)
		if err != nil {
			log.Fatalf("could not create file %v\n", dbFile)
		}
		f.Close()
	}

	sqlite3Db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("could not open database: %v\n", err)
	}

	db = &Database{sqlite3Db}

	// enable foreign keys
	q := "PRAGMA foreign_keys = ON;"
	_, err = sqlite3Db.Exec(q)
	if err != nil {
		log.Fatalln("could not activate foreign key checks")
	}

	if freshlyCreated {
		db.initDatabase()
	}

	return db
}

func (db *Database) initDatabase() {
	query1 := "CREATE TABLE users (login varchar(64) PRIMARY KEY, password varchar(255) NOT NULL, email varchar(255), balance real NOT NULL)"
	_, err := db.Exec(query1)
	if err != nil {
		log.Fatalf("could not create users table: %v", err)
	}

	query2 := "INSERT INTO users (login,password,balance) VALUES ('test',?,?)"
	_, err = db.Exec(query2, HashEncodePassword("test"), 100)
	if err != nil {
		log.Fatalf("could not insert into users table: %v", err)
	}

	query3 := "CREATE TABLE assets (name varchar(64) PRIMARY KEY, price real NOT NULL)"
	_, err = db.Exec(query3)
	if err != nil {
		log.Fatalf("could not create assets table: %v", err)
	}

	query31 := `INSERT INTO assets (name, price) VALUES 
			('white_wool',45),
			('black_wool',42)`
	_, err = db.Exec(query31)
	if err != nil {
		log.Fatalf("could not insert into assets table: %v", err)
	}

	query4 := `CREATE TABLE accounts (
    login varchar(64), 
    asset varchar(255), 
    amount real NOT NULL, 
    PRIMARY KEY (login, asset), 
    FOREIGN KEY (login) REFERENCES users (login),
    FOREIGN KEY (asset) REFERENCES assets (name)
                      )`
	_, err = db.Exec(query4)
	if err != nil {
		log.Fatalf("could not create accounts table: %v", err)
	}

	/*
	query5 := `CREATE TABLE price_history
(
	time INT,
	asset VARCHAR(255),
	price REAL,
	PRIMARY KEY(time, asset),
	FOREIGN KEY (asset) REFERENCES assets (name)
)`
	_, err = db.Exec(query5)
	if err != nil {
		log.Fatalf("could not create price_history table: %v", err)
	}
	 */
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

func (db *Database) GetAssets() ([]entity.MarketAsset, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	var assets []entity.MarketAsset

	q := `SELECT name,price FROM assets ORDER BY name`
	res, err := db.Query(q)
	if err != nil {
		log.Printf("query assets failed: %v", err)
		return nil, err
	}
	defer res.Close()

	for res.Next() {
		var n string
		var p float64
		if err := res.Scan(&n, &p); err != nil {
			log.Printf("scan assets failed: %v", err)
			return nil, err
		}
		assets = append(assets, entity.MarketAsset{Name: n, Price: decimal.NewFromFloat(p)})
	}

	return assets, nil
}

func (db *Database) GetAccounts() ([]*entity.Account, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	var accList []*entity.Account

	q := `SELECT login, balance FROM users ORDER BY balance DESC`
	res, err := db.Query(q)
	if err != nil {
		log.Fatalf("all users query failed: %v", err)
	}
	defer res.Close()

	for res.Next() {
		acc := entity.Account{}

		if err = res.Scan(&acc.Login, &acc.Balance); err != nil {
			log.Fatalf("scan user's row failed: %v", err)
		}

		if err = db.getAccountsAssets(&acc); err != nil {
			return nil, err
		}

		accList = append(accList, &acc)
	}

	return accList, nil
}

func (db *Database) getAccountsAssets(acc *entity.Account) error {
	acc.Assets = []*entity.UserAsset{}

	q := `SELECT asset,amount FROM accounts WHERE login = ? ORDER BY asset`
	res, err := db.Query(q, acc.Login)
	if err != nil {
		return fmt.Errorf("query account's assets failed: %v", err)
	}
	defer res.Close()

	for res.Next() {
		ass := &entity.UserAsset{}

		if err = res.Scan(&ass.Name, &ass.Amount); err != nil {
			return fmt.Errorf("scan account's asset failed: %v", err)
		}

		acc.Assets = append(acc.Assets, ass)
	}

	return nil
}

func (db *Database) GetAccount(login string) (*entity.Account, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	q1 := `SELECT balance FROM users WHERE login = ?`
	res1, err := db.Query(q1, login)
	if err != nil {
		log.Fatalf("users query failed: %v", err)
	}
	defer res1.Close()
	if !res1.Next() {
		return nil, fmt.Errorf("table users has no such login: %v", login)
	}

	acc := entity.Account{
		Login: login,
		Assets: []*entity.UserAsset{},
	}

	if err = res1.Scan(&acc.Balance); err != nil {
		log.Fatalf("scan user's balance failed: %v", err)
	}

	if err = db.getAccountsAssets(&acc); err != nil {
		return nil, err
	}

	return &acc, nil
}

func (db *Database) SaveAccount(acc entity.Account) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	q1 := `UPDATE users SET balance = ? WHERE login = ?`
	_, err := db.Exec(q1, acc.Balance, acc.Login)
	if err != nil {
		return fmt.Errorf("update balance for user %v failed: %v", acc.Login, err)
	}

	for _, ass := range acc.Assets {
		q2 := `UPDATE accounts SET amount = ? WHERE login = ? and asset = ?`
		stat, err := db.Exec(q2, ass.Amount, acc.Login, ass.Name)
		if err != nil {
			return fmt.Errorf("update asset's amount for user %v failed: %v", acc.Login, err)
		}

		rows, _ := stat.RowsAffected()
		if rows < 1 {
			q3 := `INSERT INTO accounts (login, asset, amount) VALUES (?, ?, ?)`
			_, err = db.Exec(q3, acc.Login, ass.Name, ass.Amount)
			if err != nil {
				return fmt.Errorf("insert asset %v for user %v failed: %v", ass.Name, acc.Login, err)
			}
		}
	}

	return nil
}

func (db *Database) GetAssetPrice(assetName string) (decimal.Decimal, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	q := `SELECT price FROM assets WHERE name = ?`
	res, err := db.Query(q, assetName)
	if err != nil {
		return decimal.Zero, err
	}
	defer res.Close()

	if !res.Next() {
		return decimal.Zero, fmt.Errorf("no such asset: %v", assetName)
	}

	var priceFloat float64
	if err = res.Scan(&priceFloat); err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromFloat(priceFloat), nil
}

func (db *Database) SetAssetPrice(assetName string, price decimal.Decimal) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	q1 := `UPDATE assets SET price = ? WHERE name = ?`
	priceFloat, _ := price.Float64()
	_, err := db.Exec(q1, priceFloat, assetName)
	if err != nil {
		return err
	}

	//q2 := `INSERT INTO price_history (time, asset, price) VALUES (?,?,?)`
	//_, err = db.Exec(q2, time.Now(), assetName, priceFloat)
	//if err != nil {
	//	return err
	//}

	return nil
}

func (db *Database) AddUser(login string, password string, email string) error {
	if password != "" {
		password = HashEncodePassword(password)
	}

	var emailOrNull *string
	if email != "" {
		emailOrNull = &email
	}

	q := "INSERT INTO users (login,password,balance,email) VALUES (?,?,?,?)"
	_, err := db.Exec(q, login, password, 100, emailOrNull)
	return err
}
