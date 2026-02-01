package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Appointment struct {
	bun.BaseModel `bun:"table:appointments"`

	ID        uuid.UUID `bun:"id,pk,type:uuid"`
	UserID    string    `bun:"user_id,notnull"`
	Title     string    `bun:"title,notnull"`
	Notes     string    `bun:"notes"`
	StartTime time.Time `bun:"start_time,notnull"`
	EndTime   time.Time `bun:"end_time,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

func (a *Appointment) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	now := time.Now().UTC()
	switch query.(type) {
	case *bun.InsertQuery:
		if a.ID == uuid.Nil {
			id, err := uuid.NewV7()
			if err != nil {
				return err
			}
			a.ID = id
		}
		if a.CreatedAt.IsZero() {
			a.CreatedAt = now
		}
		if a.UpdatedAt.IsZero() {
			a.UpdatedAt = now
		}
	case *bun.UpdateQuery:
		a.UpdatedAt = now
	}
	return nil
}
