# tenant-apiserver

This microservice provides the source of truth for tenant-related data.

* Tenants
* Tenant members
* Project
* Project members
* Quotas
* Version

## Design

The API definition and the client are included in a dedicated repository that can be found on: https://github.com/metal-stack/tenant-api.

The services are exposed as grpc-services using connectrpc.

The data is stored in a generic way using a postgres database
with tables consisting of id and json-document fields.

Changes to the data are reflected in a history table-twin per entity. When data
is created, updated or deleted, the change is also written to the history table.

## Metrics

```bash
http://localhost:2112/metrics
```

### pprof

```bash
go tool pprof -http :8080 localhost:2113/debug/pprof/heap
go tool pprof -http :8080 localhost:2113/debug/pprof/goroutine
```
