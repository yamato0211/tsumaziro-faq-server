package model

import (
	"context"
	"encoding/json"

	"github.com/uptrace/bun"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/db"
)

type Account struct {
	bun.BaseModel `bun:"table:users,alias:u"`
	ID            string          `bun:",pk"`
	Name          string          `bun:"name,notnull"`
	Email         string          `bun:"email,unique"`
	ProjectID     string          `bun:"project_id,unique,notnull"`
	Faqs          json.RawMessage `bun:"faqs,type:json"`
}

func Migrate(db *db.DB) error {
	if _, err := db.NewCreateTable().Model(&Account{}).Exec(context.Background()); err != nil {
		return err
	}
	return nil
}
