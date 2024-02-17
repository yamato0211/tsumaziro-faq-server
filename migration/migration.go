package main

import (
	"context"

	"github.com/yamato0211/tsumaziro-faq-server/db/model"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/config"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/db"
)

func main() {
	d := db.NewMySQLConnector(config.NewDBConfig())
	if _, err := d.DB.NewDropTable().Model(&model.Account{}).Exec(context.TODO()); err != nil {
		panic(err)
	}
	if err := model.Migrate(d); err != nil {
		panic(err)
	}
}
