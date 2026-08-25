package cbe

import (
	"database/sql"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

/*
// Return SQL connection using definitions in config.go

	func ReturnSQLConnection() (*sql.DB, error) {
		dsn := SQLConfig.User + ":" + SQLConfig.Password + "@tcp(" + SQLConfig.Host + ":" + string(SQLConfig.Port) + ")/" + SQLConfig.DBName
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
*/
var globalDB *sql.DB
var globalDBErr error
var once sync.Once

func ReturnSQLConnection() (*sql.DB, error) {
	once.Do(func() {
		dsn := SQLConfig.User + ":" + SQLConfig.Password +
			"@tcp(" + SQLConfig.Host + ":" + (SQLConfig.Port) + ")/" + SQLConfig.DBName

		globalDB, globalDBErr = sql.Open("mysql", dsn)
		if globalDBErr != nil {
			return
		}

		globalDB.SetMaxOpenConns(20)
		globalDB.SetMaxIdleConns(10)
		globalDB.SetConnMaxLifetime(time.Hour)
	})

	return globalDB, globalDBErr
}
