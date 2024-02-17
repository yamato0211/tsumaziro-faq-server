package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/config"
)

const driverName = "mysql"

type DB struct {
	*bun.DB
}

func NewMySQLConnector(cfg *config.DBConfig) *DB {
	dsn := mysqlConnDSN(cfg)
	sqldb, err := sql.Open(driverName, dsn)
	if err != nil {
		fmt.Println(err.Error())
		log.Fatal(err)
	}
	db := bun.NewDB(sqldb, mysqldialect.New())
	return &DB{db}
}

func mysqlConnDSN(cfg *config.DBConfig) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)
	return dsn
}
