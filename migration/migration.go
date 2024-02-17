package main

import (
	"github.com/yamato0211/tsumaziro-faq-server/db/model"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/config"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/db"
)

func main() {
	d := db.NewMySQLConnector(config.NewDBConfig())
	if err := model.Migrate(d); err != nil {
		panic(err)
	}
}
