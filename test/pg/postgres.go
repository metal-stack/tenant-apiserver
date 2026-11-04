package pg

import (
	"log/slog"
	"testing"

	"github.com/jmoiron/sqlx"
	apiv1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/pkg/health"
	"github.com/metal-stack/tenant-apiserver/test"
	"github.com/metal-stack/tenant-apiserver/test/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func StartPostgres(log *slog.Logger, t testing.TB) (testcontainers.Container, *sqlx.DB) {
	ctx := t.Context()
	pgContainer := test.StartPostgresContainer(log, t)

	ip, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	db, err := postgres.NewPostgresDB(log, ip, port.Port(), config.DBUser, config.DBPassword, config.DBName, "disable", AllEntities()...)
	require.NoError(t, err)

	require.NoError(t, postgres.MigrateDB(log, db, health.NewHealthServer()))

	return pgContainer, db
}

func AllEntities() []api.Entity {
	return []api.Entity{
		&apiv1.ProjectConfig{},
		&apiv1.Project{},
		&apiv1.ProjectMember{},
		&apiv1.TenantConfig{},
		&apiv1.Tenant{},
		&apiv1.TenantMember{},
	}
}
