package serviceTrade

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"tradingServer/storage"
)

type logEntry struct {
	Time       string
	Login      string // login and IP address
	ActionPath string // path or action
	Duration   float64
	Status     int    // optional: HTTP status
	AssetInfo  string // optional: name, unit_amount, unit_price, payed_price, new_balance
}

// ShowLastLog prints the last <count> transaction log messages and the last <count> access log messages to the console
func ShowLastLog(count int) {
	db := storage.GetDatabase()

	q1 := `SELECT time,login,action,unit_price,payed_price,amount,asset,balance FROM transaction_log ORDER BY time DESC LIMIT ?`
	res1, err := db.Query(q1, count)
	if err != nil {
		log.Fatalf("transaction log query failed: %v", err)
	}

	var logs []logEntry
	for res1.Next() {
		logs = append(logs, packLogTransaction(res1))
	}

	res1.Close()

	q2 := `SELECT time,duration,login,path,status,address FROM access_log ORDER BY time DESC LIMIT ?`
	res2, err := db.Query(q2, count)
	if err != nil {
		log.Fatalf("access log query failed: %v", err)
	}

	for res2.Next() {
		logs = append(logs, packLogAccess(res2))
	}

	res2.Close()

	sort.Slice(logs, func(i, j int) bool {
		// sort by time descending
		return logs[i].Time > logs[j].Time
	})

	for _, l := range logs {
		fmt.Println(l.String())
	}
}

func packLogAccess(row *sql.Rows) logEntry {
	var l logEntry
	var ip string

	err := row.Scan(&l.Time, &l.Duration, &l.Login, &l.ActionPath, &l.Status, &ip)
	if err != nil {
		log.Fatalf("scan access log: %v", err)
	}

	if l.Login != "" {
		l.Login += "/"
	}
	if ip != "" {
		l.Login += ip
	}

	l.Time = convertTimestampAccessLog(l.Time)

	return l
}

// convertTimestampAccessLog inserts 'T' between date and time expected in: 2022-12-09 12:13:59.480980086+01:00 out: 2022-12-09T12:13:59.480980086+01:00
func convertTimestampAccessLog(timeStr string) string {
	return strings.Replace(timeStr, " ", "T", 1)
}

func packLogTransaction(row *sql.Rows) logEntry {
	var l logEntry
	var unit_price float64
	var payed_price float64
	var unit_amount float64
	var asset_name string
	var new_balance float64

	err := row.Scan(&l.Time, &l.Login, &l.ActionPath, &unit_price, &payed_price, &unit_amount, &asset_name, &new_balance)
	if err != nil {
		log.Fatalf("scan transaction log: %v", err)
	}

	sign := "+"
	if l.ActionPath == "buy" {
		sign = "-"
	}

	l.AssetInfo = fmt.Sprintf("%v %v*cr%.3f %v%.3f -> %.3f", asset_name, unit_amount, unit_price, sign, payed_price, new_balance)

	return l
}

func (l logEntry) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%v|%v|%v|", l.Time, l.Login, l.ActionPath))

	if l.Status != 0 {
		sb.WriteString(fmt.Sprintf("%.3f sec|ACCESS|%v", l.Duration, l.Status))
	} else {
		sb.WriteString(fmt.Sprintf("?|TR.ACT|%v", l.AssetInfo))
	}

	return sb.String()
}

func ResetLog() {
	db := storage.GetDatabase()

	q1 := `DELETE FROM access_log`
	res, err := db.Exec(q1)
	if err != nil {
		log.Fatalf("failed clearing access log: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("clearing access log row count failed: %v", err)
	}
	log.Printf("Cleared %v rows from access log.\n", n)

	q2 := `DELETE FROM transaction_log`
	res, err = db.Exec(q2)
	if err != nil {
		log.Fatalf("failed clearing transaction log: %v", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		log.Fatalf("clearing transaction log row count failed: %v", err)
	}
	log.Printf("Cleared %v rows from transaction log.\n", n)
}
