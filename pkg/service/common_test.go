package service

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jmoiron/sqlx"
	apiv1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/tenant-api/go/api/v1/apiv1connect"
	"github.com/metal-stack/tenant-api/go/client"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func startTenantApiserverWithPostgres(t testing.TB, log *slog.Logger) (client.Client, func()) {
	ctx := t.Context()

	postgres, err := testpostgres.Run(ctx,
		"postgres:18-alpine",
		testpostgres.WithPassword("password"),
		testpostgres.BasicWaitStrategies(),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql": "rw"}),
		testcontainers.WithName("postgres"),
	)
	require.NoError(t, err)

	connectionString, err := postgres.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sqlx.Open("postgres", connectionString)
	require.NoError(t, err)

	closer := func() {
		_ = postgres.Terminate(ctx)
	}

	return startTenantApiserverWithDB(t, log, closer, db)
}

func startTenantApiserverWithDB(t testing.TB, log *slog.Logger, dbcloser func(), db *sqlx.DB) (client.Client, func()) {

	log = log.WithGroup("tenant-apiserver")
	ps := postgres.New(log, db, &apiv1.Project{})
	pms := postgres.New(log, db, &apiv1.ProjectMember{})
	ts := postgres.New(log, db, &apiv1.Tenant{})
	tms := postgres.New(log, db, &apiv1.TenantMember{})

	err := postgres.InitTables(log, db,
		&apiv1.Project{},
		&apiv1.ProjectMember{},
		&apiv1.Tenant{},
		&apiv1.TenantMember{},
	)
	require.NoError(t, err)

	projectService := NewProjectService(log, ps, pms, ts)
	projectMemberService := NewProjectMemberService(log, ps, pms, ts)
	tenantService := NewTenantService(db, log, ts, tms)
	tenantMemberService := NewTenantMemberService(log, ts, tms)
	versionService := NewVersionService()

	mux := http.NewServeMux()
	mux.Handle(apiv1connect.NewProjectServiceHandler(projectService))
	mux.Handle(apiv1connect.NewProjectMemberServiceHandler(projectMemberService))
	mux.Handle(apiv1connect.NewTenantServiceHandler(tenantService))
	mux.Handle(apiv1connect.NewTenantMemberServiceHandler(tenantMemberService))
	mux.Handle(apiv1connect.NewVersionServiceHandler(versionService))

	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()
	closer := func() {
		dbcloser()
		server.Close()
	}

	log.Debug("connecting to", "url", server.URL)

	client, err := client.New(&client.DialConfig{
		Transport: server.Client().Transport,
		BaseURL:   server.URL,
		Log:       log,
	})
	require.NoError(t, err)

	return client, closer
}

func IgnoreUnexported() cmp.Option {
	// the exporter opt allows all unexported fields: https://github.com/google/go-cmp/pull/176
	return cmp.Exporter(func(reflect.Type) bool { return true })
}
