package test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	dbName     = "postgres"
	dbUser     = "postgres"
	dbPassword = "password"
)

func StartPostgresContainer(log *slog.Logger, t testing.TB) testcontainers.Container {
	var (
		ctx = t.Context()
	)

	pgContainer, err := testpostgres.Run(ctx,
		"postgres:18-alpine",
		testpostgres.WithDatabase(dbName),
		testpostgres.WithUsername(dbUser),
		testpostgres.WithPassword(dbPassword),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql": "rw"}),
		testpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	return pgContainer
}
