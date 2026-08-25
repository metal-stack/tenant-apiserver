package test

import (
	"log/slog"
	"testing"

	"github.com/metal-stack/tenant-apiserver/test/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func StartPostgresContainer(log *slog.Logger, t testing.TB) testcontainers.Container {
	var (
		ctx = t.Context()
	)

	pgContainer, err := testpostgres.Run(ctx,
		"postgres:18-alpine",
		testpostgres.WithDatabase(config.DBName),
		testpostgres.WithUsername(config.DBUser),
		testpostgres.WithPassword(config.DBPassword),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql": "rw"}),
		testpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	return pgContainer
}
