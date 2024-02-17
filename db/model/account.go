package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	FirebaseID    string          `bun:"firebase_id,unique"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (a *Account) String() string {
	return fmt.Sprintf("Account<%s %s %s %s %s>", a.ID, a.Name, a.Email, a.ProjectID, a.FirebaseID)
}

func MigrateAccount(db *db.DB) error {
	if _, err := db.NewCreateTable().Model(&Account{}).IfNotExists().Exec(context.Background()); err != nil {
		return err
	}
	return nil
}
