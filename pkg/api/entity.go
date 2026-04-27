package api

import (
	"context"
	"time"

	v1 "github.com/metal-stack/tenant-api/go/api/v1"
)

// Entity defines a database entity which is stored in jsonb format and with version information
type Entity interface {
	JSONField() string
	TableName() string
	Schema() string
	GetMeta() *v1.Meta
	Kind() string
	APIVersion() string
}

// Storage is a interface to store objects.
type Storage[E Entity] interface {
	// generic
	Create(ctx context.Context, ve E) error
	Update(ctx context.Context, ve E) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, ids ...string) error
	Get(ctx context.Context, id string) (E, error)
	GetHistory(ctx context.Context, id string, at time.Time, ve E) error
	GetHistoryCreated(ctx context.Context, id string, ve E) error
	Find(ctx context.Context, paging *v1.Paging, filters ...any) ([]E, *uint64, error)
}
