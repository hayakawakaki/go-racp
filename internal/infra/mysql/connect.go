package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers the MySQL driver with database/sql
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	connMaxLifetime     = 5 * time.Minute
	connMaxIdleTime     = 1 * time.Minute
	connRetryInterval   = 3 * time.Second
	connMaxRetryAttempt = 5
)

func Connect(env *config.EnvConfig) (mainDB, logsDB *sql.DB) {
	fmt.Println("connecting to MySQL...")

	for i := 1; i <= connMaxRetryAttempt; i++ {
		main, logs, err := attemptConnect(env)
		if err == nil {
			fmt.Println("connected to MySQL successfully.")
			return main, logs
		}

		log.Printf("MySQL connection attempt %d/%d failed: %v", i, connRetryInterval, err)

		if i < connMaxRetryAttempt {
			time.Sleep(connRetryInterval)
		}
	}

	log.Fatalf("unable to connect to MySQL after %d attempts", connMaxRetryAttempt)
	return nil, nil
}

func attemptConnect(env *config.EnvConfig) (mainDB, logsDB *sql.DB, err error) {
	main, err := open(env.DBMainURL, env.DBMaxOpenConn, env.DBMaxIdleConn)
	if err != nil {
		return nil, nil, fmt.Errorf("main db: %w", err)
	}

	logs, err := open(env.DBLogURL, env.DBMaxOpenConn, env.DBMaxIdleConn)
	if err != nil {
		_ = main.Close()
		return nil, nil, fmt.Errorf("log db: %w", err)
	}

	return main, logs, nil
}

func open(url string, maxOpen, maxIdle int) (*sql.DB, error) {
	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db.Ping: %w", err)
	}
	return db, nil
}
