package model

import (
	"context"
	"time"

	"github.com/yamato0211/tsumaziro-faq-server/pkg/db"
)

type Model struct {
	ID           string    `bun:",pk"`
	AgentID      string    `bun:"agent_id,notnull,unique"`
	AgentAliasID string    `bun:"agent_alias_id,notnull,unique"`
	Prompt       string    `bun:"prompt"`
	CreatedAt    time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func MigrateModel(db *db.DB) error {
	if _, err := db.NewCreateTable().Model(&Model{}).IfNotExists().Exec(context.Background()); err != nil {
		return err
	}
	return nil
}
