package cbe

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

// Return SQL connection using definitions in config.go
func ReturnSQLConnection() (*sql.DB, error) {
	dsn := SQLConfig.User + ":" + SQLConfig.Password + "@tcp(" + SQLConfig.Host + ":" + string(SQLConfig.Port) + ")/" + SQLConfig.DBName
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}
