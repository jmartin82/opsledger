package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"ops-ledger/backend/config"
)

func Connect(cfg config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)

	var db *sql.DB
	var err error

	for i := 0; i < 3; i++ {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("DB open attempt %d failed: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if err = db.Ping(); err != nil {
			log.Printf("DB ping attempt %d failed: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		log.Println("Connected to database")
		return db, nil
	}

	return nil, fmt.Errorf("failed to connect to database after 3 attempts: %w", err)
}
