package main

import (
	"context"
	"log/slog"
	"os"

	apiv1 "github.com/metal-stack/tenant-api/go/api/v1"
	tenant "github.com/metal-stack/tenant-api/go/client"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})

	logger := slog.New(jsonHandler)
	logger.Info("Starting Client")

	c, err := tenant.New(&tenant.DialConfig{
		BaseURL: "http://localhost:9090",
		Log:     logger,
	})
	if err != nil {
		logger.Error("unable to create client", "error", err)
		os.Exit(1)
	}

	v, err := c.Apiv1().Version().Get(context.Background(), &apiv1.VersionServiceGetRequest{})
	if err != nil {
		logger.Error("unable to get version", "error", err)
		os.Exit(1)
	}
	logger.Info("version", "version", v)

	if err := createTenant(context.Background(), logger, c); err != nil {
		logger.Error("unable to create tenant", "error", err)
	}

	logger.Info("Success")
}

func createTenant(ctx context.Context, log *slog.Logger, c tenant.Client) error {
	t, err := c.Apiv1().Tenant().Create(ctx, &apiv1.TenantServiceCreateRequest{
		Tenant: &apiv1.Tenant{
			Meta: &apiv1.Meta{Id: "0000"},
			Name: "metal-stack",
		},
	})
	if err != nil {
		return err
	}
	log.Info("tenant created", "tenant", t)
	return nil
}
