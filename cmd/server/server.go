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

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpcreflect/v2"
	"connectrpc.com/otelconnect"

	"connectrpc.com/grpchealth/v2"
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

	otelInterceptor, err := otelconnect.NewServerInterceptor(otelconnect.WithMeterProvider(provider))
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

	mux := http.NewServeMux()
	srv := connect.NewServer(loggingInterceptor, otelInterceptor)
	apiv1connect.RegisterProjectServiceHandler(srv, projectService)
	apiv1connect.RegisterProjectMemberServiceHandler(srv, projectMemberService)
	apiv1connect.RegisterTenantServiceHandler(srv, tenantService)
	apiv1connect.RegisterTenantMemberServiceHandler(srv, tenantMemberService)
	apiv1connect.RegisterVersionServiceHandler(srv, versionService)
	connecthttp.Mount(mux, srv, connecthttp.WithReadMaxBytes(0))

	allServiceNames := []string{
		apiv1connect.ProjectServiceName,
		apiv1connect.ProjectMemberServiceName,
		apiv1connect.TenantServiceName,
		apiv1connect.TenantMemberServiceName,
		apiv1connect.VersionServiceName,
	}

	checker := grpchealth.NewStaticChecker(allServiceNames...)
	srv2 := connect.NewServer()
	grpchealth.Register(srv2, checker)

	// enable remote service listing by enabling reflection
	grpcreflect.Register(srv2, grpcreflect.WithNamer(grpcreflect.NamerFunc(func() []string {
		return allServiceNames
	})))
	connecthttp.Mount(mux, srv2, connecthttp.WithReadMaxBytes(0))

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

func newLoggingInterceptor(log *slog.Logger) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			var (
				callLog = log.With("procedure", spec.Procedure)
				debug   = callLog.Enabled(ctx, slog.LevelDebug)
				start   = time.Now()
			)

			if debug {
				callLog.Debug("handling call")
			}

			err := next(ctx, spec, &loggingStream{ServerStream: stream, log: callLog, debug: debug})

			callLog = callLog.With("duration", time.Since(start).String())

			if err != nil {
				callLog.Error("error during call", "error", err)
			}

			callLog.Info("handled call")

			return err
		}
	}
}

type loggingStream struct {
	connect.ServerStream

	log   *slog.Logger
	debug bool
}

func (s *loggingStream) Receive(msg any) error {
	if err := s.ServerStream.Receive(msg); err != nil {
		return err
	}
	if s.debug {
		s.log.Debug("received request", "request", msg)
	}
	return nil
}

func (s *loggingStream) Send(msg any) error {
	if s.debug {
		s.log.Debug("sending response", "response", msg)
	}
	return s.ServerStream.Send(msg)
}
