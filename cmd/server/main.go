package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	apiv1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/memory"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/v"
	cli "github.com/urfave/cli/v3"
)

func main() {

	app := &cli.Command{
		Name:    "tenant-apiserver",
		Usage:   "connectrpc/grpc server for tenant related data",
		Version: v.V.String(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "http-server-endpoint",
				Value:   "0.0.0.0:8080",
				Usage:   "http server endpoint",
				Sources: cli.EnvVars("TENANT_APISERVER_GRPC_SERVER_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:    "metrics-endpoint",
				Value:   ":2112",
				Usage:   "metrics endpoint",
				Sources: cli.EnvVars("TENANT_APISERVER_METRICS_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "log-level can be one of error|warn|info|debug",
				Sources: cli.EnvVars("TENANT_APISERVER_LOG_LEVEL"),
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "postgres",
				Aliases: []string{"pg"},
				Usage:   "start with postgres backend",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "host",
						Value:   "localhost",
						Usage:   "postgres db hostname",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_HOST"),
					},
					&cli.StringFlag{
						Name:    "port",
						Value:   "5432",
						Usage:   "postgres db port",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_PORT"),
					},
					&cli.StringFlag{
						Name:    "user",
						Value:   "masterdata",
						Usage:   "postgres db user",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_USER"),
					},
					&cli.StringFlag{
						Name:    "password",
						Value:   "password",
						Usage:   "postgres db password",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_PASSWORD"),
					},
					&cli.StringFlag{
						Name:    "dbname",
						Value:   "masterdata",
						Usage:   "postgres db name",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_DBNAME"),
					},
					&cli.StringFlag{
						Name:    "sslmode",
						Value:   "disable",
						Usage:   "postgres sslmode, possible values: disable|require|verify-ca|verify-full",
						Sources: cli.EnvVars("TENANT_APISERVER_PG_SSLMODE"),
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c := getConfig(cmd)
					host := cmd.String("host")
					port := cmd.String("port")
					user := cmd.String("user")
					password := cmd.String("password")
					dbname := cmd.String("dbname")
					sslmode := cmd.String("sslmode")

					ves := []api.Entity{
						&apiv1.Project{},
						&apiv1.ProjectMember{},
						&apiv1.Tenant{},
						&apiv1.TenantMember{},
					}

					db, err := postgres.NewPostgresDB(c.Log, host, port, user, password, dbname, sslmode, ves...)
					if err != nil {
						return fmt.Errorf("failed to create postgres connection: %w", err)
					}
					ps := postgres.New(c.Log, db, &apiv1.Project{})
					pms := postgres.New(c.Log, db, &apiv1.ProjectMember{})
					ts := postgres.New(c.Log, db, &apiv1.Tenant{})
					tms := postgres.New(c.Log, db, &apiv1.TenantMember{})
					c.ProjectDataStore = ps
					c.ProjectMemberDataStore = pms
					c.TenantDataStore = ts
					c.TenantMemberDataStore = tms
					c.DB = db
					s := newServer(c)
					return s.Run()
				},
			},
			{
				Name:    "memory",
				Aliases: []string{"mem"},
				Usage:   "start with memory backend",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c := getConfig(cmd)

					ps := memory.NewMemory(c.Log, &apiv1.Project{})
					pms := memory.NewMemory(c.Log, &apiv1.ProjectMember{})
					ts := memory.NewMemory(c.Log, &apiv1.Tenant{})
					tms := memory.NewMemory(c.Log, &apiv1.TenantMember{})

					c.ProjectDataStore = ps
					c.ProjectMemberDataStore = pms
					c.TenantDataStore = ts
					c.TenantMemberDataStore = tms
					s := newServer(c)
					return s.Run()
				},
			},
		},
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		log.Fatalf("unable to start tenant-apiserver service: %v", err)
	}
}

func getConfig(cmd *cli.Command) config {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	switch cmd.String("log-level") {
	case "debug":
		opts.Level = slog.LevelDebug
	case "error":
		opts.Level = slog.LevelError
	}

	return config{
		HttpServerEndpoint: cmd.String("http-server-endpoint"),
		MetricsEndpoint:    cmd.String("metrics-endpoint"),
		Log:                slog.New(slog.NewJSONHandler(os.Stdout, opts)),
	}
}
