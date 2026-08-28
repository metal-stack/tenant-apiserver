package service

import (
	"context"
	"time"

	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
)

type (
	ProjectDataStore       api.Storage[*v1.Project]
	ProjectMemberDataStore api.Storage[*v1.ProjectMember]
	TenantDataStore        api.Storage[*v1.Tenant]
	TenantMemberDataStore  api.Storage[*v1.TenantMember]
)

type StorageStatusWrapper[E api.Entity] struct {
	storage api.Storage[E]
}

func NewStorageStatusWrapper[E api.Entity](s api.Storage[E]) api.Storage[E] {
	return StorageStatusWrapper[E]{
		storage: s,
	}
}

func (s StorageStatusWrapper[E]) Create(ctx context.Context, ve E) error {
	return s.storage.Create(ctx, ve)
}

func (s StorageStatusWrapper[E]) Update(ctx context.Context, ve E) error {
	return s.storage.Update(ctx, ve)
}

func (s StorageStatusWrapper[E]) Delete(ctx context.Context, id string) error {
	return s.storage.Delete(ctx, id)
}

func (s StorageStatusWrapper[E]) DeleteAll(ctx context.Context, ids ...string) error {
	return s.storage.DeleteAll(ctx, ids...)
}

func (s StorageStatusWrapper[E]) Get(ctx context.Context, id string) (E, error) {
	return s.storage.Get(ctx, id)
}

func (s StorageStatusWrapper[E]) GetHistory(ctx context.Context, id string, at time.Time, ve E) error {
	return s.storage.GetHistory(ctx, id, at, ve)
}

func (s StorageStatusWrapper[E]) GetHistoryCreated(ctx context.Context, id string, ve E) error {
	return s.storage.GetHistoryCreated(ctx, id, ve)
}

func (s StorageStatusWrapper[E]) Find(ctx context.Context, paging *v1.Paging, filters ...any) ([]E, *uint64, error) {
	return s.storage.Find(ctx, paging, filters...)
}
