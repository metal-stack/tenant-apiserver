package pg

import (
	"log/slog"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/test"
	"github.com/metal-stack/tenant-apiserver/test/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func StartPostgres(log *slog.Logger, t testing.TB, ves ...api.Entity) (testcontainers.Container, *sqlx.DB) {
	ctx := t.Context()
	pgContainer := test.StartPostgresContainer(log, t)

	ip, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	db, err := postgres.NewPostgresDB(log, ip, port.Port(), config.DBUser, config.DBPassword, config.DBName, "disable", ves...)
	require.NoError(t, err)

	return pgContainer, db
}
