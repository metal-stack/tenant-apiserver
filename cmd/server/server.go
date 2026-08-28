package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // nolint:gosec
	"time"

	"github.com/jmoiron/sqlx"
	apiv1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"

	apiv1connect "github.com/metal-stack/tenant-api/go/tenant/api/v1/apiv1connect"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/service"

	"github.com/metal-stack/v"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"

	"connectrpc.com/grpchealth"
)

type config struct {
	HttpServerEndpoint     string
	MetricsEndpoint        string
	Log                    *slog.Logger
	ProjectDataStore       api.Storage[*apiv1.Project]
	ProjectMemberDataStore api.Storage[*apiv1.ProjectMember]
	TenantDataStore        api.Storage[*apiv1.Tenant]
	TenantMemberDataStore  api.Storage[*apiv1.TenantMember]
	DB                     *sqlx.DB
}
type server struct {
	c config

	projectDataStore       api.Storage[*apiv1.Project]
	projectMemberDataStore api.Storage[*apiv1.ProjectMember]
	tenantDataStore        api.Storage[*apiv1.Tenant]
	tenantMemberDataStore  api.Storage[*apiv1.TenantMember]
}

func newServer(c config) *server {
	return &server{
		c:                      c,
		projectDataStore:       c.ProjectDataStore,
		projectMemberDataStore: c.ProjectMemberDataStore,
		tenantDataStore:        c.TenantDataStore,
		tenantMemberDataStore:  c.TenantMemberDataStore,
	}
}
func (s *server) Run() error {
	s.c.Log.Info("starting tenant-apiserver", "version", v.V.String())

	// The exporter embeds a default OpenTelemetry Reader and
	// implements prometheus.Collector, allowing it to be used as
	// both a Reader and Collector.
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	// Start the prometheus HTTP server and pass the exporter Collector to it
	go func() {
		s.c.Log.Info("serving metrics", "at", fmt.Sprintf("%s/metrics", s.c.MetricsEndpoint))
		metricsServer := http.NewServeMux()
		metricsServer.Handle("/metrics", promhttp.Handler())
		ms := &http.Server{
			Addr:              s.c.MetricsEndpoint,
			Handler:           metricsServer,
			ReadHeaderTimeout: time.Minute,
		}
		err := ms.ListenAndServe()
		if err != nil {
			s.c.Log.Error("unable to start metric endpoint", "error", err)
			return
		}
	}()
	go func() {
		s.c.Log.Info("starting pprof endpoint of :2113")
		// inspect via
		// go tool pprof -http :8080 localhost:2113/debug/pprof/heap
		// go tool pprof -http :8080 localhost:2113/debug/pprof/goroutine
		server := http.Server{
			Addr:              ":2113",
			ReadHeaderTimeout: 1 * time.Minute,
		}
		err := server.ListenAndServe()
		if err != nil {
			s.c.Log.Error("failed to start pprof endpoint", "error", err)
			return
		}
	}()

	otelInterceptor, err := otelconnect.NewInterceptor(otelconnect.WithMeterProvider(provider))
	if err != nil {
		return err
	}

	loggingInterceptor := newLoggingInterceptor(s.c.Log)

	projectService := service.NewProjectService(s.c.Log, s.c.ProjectDataStore, s.c.ProjectMemberDataStore, s.c.TenantDataStore)
	projectMemberService := service.NewProjectMemberService(s.c.Log, s.c.ProjectDataStore, s.c.ProjectMemberDataStore, s.c.TenantDataStore)
	// FIXME db should not be required here
	tenantService := service.NewTenantService(s.c.DB, s.c.Log, s.c.TenantDataStore, s.c.TenantMemberDataStore)
	tenantMemberService := service.NewTenantMemberService(s.c.Log, s.c.TenantDataStore, s.c.TenantMemberDataStore)
	versionService := service.NewVersionService()

	// healthv1.RegisterHealthServer(grpcServer, healthServer)
	interceptors := connect.WithInterceptors(loggingInterceptor, otelInterceptor)

	mux := http.NewServeMux()
	mux.Handle(apiv1connect.NewProjectServiceHandler(projectService, interceptors))
	mux.Handle(apiv1connect.NewProjectMemberServiceHandler(projectMemberService, interceptors))
	mux.Handle(apiv1connect.NewTenantServiceHandler(tenantService, interceptors))
	mux.Handle(apiv1connect.NewTenantMemberServiceHandler(tenantMemberService, interceptors))
	mux.Handle(apiv1connect.NewVersionServiceHandler(versionService, interceptors))

	allServiceNames := []string{
		apiv1connect.ProjectServiceName,
		apiv1connect.ProjectMemberServiceName,
		apiv1connect.TenantServiceName,
		apiv1connect.TenantMemberServiceName,
		apiv1connect.VersionServiceName,
	}

	checker := grpchealth.NewStaticChecker(allServiceNames...)
	mux.Handle(grpchealth.NewHandler(checker))

	// enable remote service listing by enabling reflection
	reflector := grpcreflect.NewStaticReflector(allServiceNames...)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	server := http.Server{
		Addr:              s.c.HttpServerEndpoint,
		ReadHeaderTimeout: 1 * time.Minute,
		Handler:           mux,

		Protocols: new(http.Protocols)}
	server.Protocols.SetHTTP1(true)
	server.Protocols.SetHTTP2(true)
	// For gRPC clients, it's convenient to support HTTP/2 without TLS
	server.Protocols.SetUnencryptedHTTP2(true)

	s.c.Log.Info("started tenant api-server", "at", server.Addr)
	err = server.ListenAndServe()
	return err
}

func newLoggingInterceptor(log *slog.Logger) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			var (
				log   = log.With("procedure", req.Spec().Procedure)
				debug = log.Enabled(ctx, slog.LevelDebug)
				start = time.Now()
			)

			if debug {
				log = log.With("request", req.Any())
			}

			if req.Spec().Procedure == apiv1connect.VersionServiceGetProcedure {
				return next(ctx, req)
			}
			log.Info("handling unary call")

			response, err := next(ctx, req)

			if debug && response != nil {
				log = log.With("response", response.Any())
			}

			if err != nil {
				log.Error("error during unary call", "error", err)
			} else if debug {
				log.Debug("handled call successfully", "duration", time.Since(start).String())
			}

			return response, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
