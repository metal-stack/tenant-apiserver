# Findings

## Critical / Bugs

### F1: Pagination off-by-one bug (`postgres.go:488-491`)

```go
offset = *paging.Page * limit
nextpage := *paging.Page + 1
```

When a client requests `Paging{Page: 1, Count: 60}`, the offset becomes `1 * 60 = 60`, which skips the first page of results entirely. If `Page: 1` means "first page" (standard 1-based indexing), this should be `(Page - 1) * limit`. While the code is internally consistent (first call with nil Page returns next page `1`), this is confusing and non-standard: consumers passing `Page: 1` expecting the first page will receive the second page.

### F2: `DeleteAll` in memory datastore ignores passed IDs (`memory.go:69-71`)

```go
func (m *memoryDatastore[E]) DeleteAll(ctx context.Context, ids ...string) error {
    m.entities = make(map[string]E)
    return nil
}
```

Instead of deleting only the specified entity IDs, the entire map is replaced. This is inconsistent with `postgres.go:296-347` which correctly filters by IDs. In tests or scenarios relying on partial deletion, this silently removes all entities.

### F3: Unimplemented methods panic (`memory.go:106-112`)

```go
func (m *memoryDatastore[E]) GetHistory(ctx context.Context, id string, at time.Time, ve E) error {
    panic("unimplemented")
}
func (m *memoryDatastore[E]) GetHistoryCreated(ctx context.Context, id string, ve E) error {
    panic("unimplemented")
}
```

Panic rather than returning an error will crash the entire server process if these methods are called. Should return `fmt.Errorf("not implemented")` instead.

### F4: `memory.go:48` — Find silently ignores all filters (`memory.go:82-84`)

```go
for _, e := range m.entities {
    // FIXME implement filtering
    result = append(result, e)
}
```

All query filters are silently dropped. The in-memory datastore is useful for testing, but this makes tests give misleading results since filtered queries return everything.

---

## Security

### S1: Hardcoded default DB credentials (`cmd/server/main.go:69-80`)

```go
Value:  "password",         // default password
Value:  "masterdata",       // default user
Value:  "masterdata",       // default db name
```

A default password of `"password"` with user `"masterdata"` is a severe security risk. Default credentials should never be hardcoded. Using sensible defaults here encourages insecure configuration in development environments.

### S2: SSL disabled by default (`cmd/server/main.go:82`)

```go
Value: "disable",
```

Connections are unencrypted by default. This should default to `require` or higher.

### S3: pprof endpoint unauthenticated (`cmd/server/server.go:8,91-99`)

The pprof endpoint on port 2113 is publicly accessible with no authentication. It allows memory/CPU profiling and can leak sensitive data. This should only be available on a restricted interface or behind authentication.

### S4: User-controlled annotation key interpolated into SQL (`pkg/service/tenant.go:136`, `pkg/service/project.go:126`)

```go
f := fmt.Sprintf("tenant -> 'meta' -> 'annotations' ->> '%s'", key)
mapFilter[f] = value
```

The `key` from the user request is interpolated directly into a SQL expression string via `fmt.Sprintf`. While postgres JSONB operators may prevent traditional SQL injection (malicious input would cause a syntax error), annotation keys are not sanitized or validated. An attacker could craft keys that break the query or potentially cause errors. The key should be validated or parameterized.

---

## Race Conditions / Concurrency

### R1: `DeleteAll` lock granularity mismatch (`memory.go:69-71`)

`DeleteAll` acquires `m.lock` at line 69 but releases it before resetting `m.entities` if an error occurs. More critically, line 70 does `m.entities = make(map[string]E)` which replaces the map reference, but any concurrent read between the lock release and this assignment could see a partially updated state. Meanwhile `Create` (line 41-42), `Delete` (line 56-57), `Update` (line 62-63) all properly hold the lock for their entire read-modify-write.

### R2: Global `Now` variable unsafe for parallel tests (`postgres.go:25`, `postgres_test.go`)

```go
var Now = time.Now
```

Package-level `Now` is overridden in tests (see `postgres_test.go:773-776`). Tests using `t.Parallel()` will clobber each other's time overrides. This is a test design issue that can lead to flaky tests.

---

## Architectural Inconsistencies

### A1: `NewTenantService` is the only service taking raw `*sqlx.DB` (`cmd/server/server.go:109-114`)

```go
tenantService := service.NewTenantService(c.DB, ...)  // raw DB
projectService := service.NewProjectService(log, ...)  // Storage interface only
```

`NewTenantService` is the sole exception that accepts a raw database handle, with a FIXME comment acknowledging this. Other services use only the `Storage` interface. The special methods (`FindParticipatingProjects`, `FindParticipatingTenants`, `ListTenantMembers`) use raw SQL with complex JOINs. The raw DB dependency should be injected via the Storage interface with extended methods.

### A2: Constructor argument order inconsistency (`cmd/server/server.go:109-114`)

```go
projectService := NewProjectService(log, pds, pmds, tds)     // log first
tenantService := NewTenantService(db, log, tds, tmds)          // db first (different!)
tenantMemberService := NewTenantMemberService(log, tds, tmds)  // log first
```

`NewTenantService` puts `db` as the first parameter while all other constructors put `log` first. This makes the constructor call visually stand out and is error-prone when reading code.

### A3: `var zero E` then returning `*e` after `Scan` (`postgres.go:231-249`)

```go
var zero E
// ... query ...
e := new(E)
err = rows.Scan(e)
if err != nil {
    return zero, Convert(err)  // returns zero value
}
return *e, nil
```

On scan failure, returns the zero value rather than an error. On success, returns `*e` (dereferenced pointer). The zero value return could silently hide errors if `rows.Err()` is checked separately (e.g., for `sql.ErrNoRows`). The explicit handling here is correct (`Convert(err)`), but the pattern of using `var zero E` as an error fallback is fragile — a zero value should never be treated as valid data.

---

## Dead Code / Unused

### D1: Health check server is never wired up (`pkg/health/health.go`)

The `health.Server` is initialized but never registered with the gRPC server. The registration line in `server.go:117` is commented out:

```go
// healthv1.RegisterHealthServer(grpcServer, healthServer)
```

The health endpoint is never reachable, yet the code exists and takes up maintenance burden.

### D2: Service package-level variables shadow real usage (`pkg/service/tenant.go:25-30`)

```go
var (
    projectMembers = api.Entity(&v1.ProjectMember{})
    tenantMembers  = api.Entity(&v1.TenantMember{})
    projects       = api.Entity(&v1.Project{})
    tenants        = api.Entity(&v1.Tenant{})
)
```

These are package-level variables used across multiple functions in the file (`queryDirectProjectParticipations`, `queryIndirectProjectParticipations`, etc.). They create a dependency on file-level ordering and would be cleaner as local variables in the functions that need them.

---

## Error Handling

### E1: `IsNotFound(nil)` would panic (`pkg/errorutil/errors.go:25-31`)

```go
func IsNotFound(err error) bool {
    connectErr := Convert(err)
    return connectErr.Code() == connect.CodeNotFound
}
```

If called with `nil`, `Convert(nil)` returns `nil`, and `connectErr.Code()` panics with nil pointer dereference. This is safe at call sites due to short-circuit evaluation (`err != nil && errorutil.IsNotFound(err)`), but it's a latent bug — callers that forget the `err != nil` guard will crash. The function should guard against nil input.

### E2: `IsNotFound` called with non-not-found errors returns unexpected error (`pkg/service/tenantmember.go:32-38`)

```go
if err != nil && errorutil.IsNotFound(err) {
    return nil, status.Error(codes.NotFound, ...)
}
if err != nil {
    return nil, err
}
```

If `err` is a not-found error, the first branch is taken (correct). If `err` is any other error (e.g., connection error), both conditions skip and fall through to the second check which returns `nil, err`. This is correct for other errors, but if future code adds more error types, the `IsNotFound` pattern could be confusing. The redundant nil check makes the intent unclear.

## Missing Context / Timeout Handling

### M1: No graceful shutdown for metrics and pprof servers (`cmd/server/server.go:73-84, 91-99`)

```go
go func() {
    log.Info("starting metrics server on", "address", s.c.MetricsEndpoint)
    if err := ms.ListenAndServe(); err != nil {
        log.Error("error starting metrics server", "err", err)
    }
}()
```

Neither the metrics server nor the pprof server implement graceful shutdown. They use raw `ListenAndServe` with no `Server.Shutdown()` call. When the process receives SIGTERM/SIGINT (e.g., Kubernetes pod termination), these servers are killed abruptly, dropping in-flight requests. The main HTTP server also lacks graceful shutdown (`server.go:146`).

### M2: `context.Background()` used in migration and bootstrap (`pkg/datastore/postgres/migrate.go:41`, `pkg/datastore/bootstrap.go:95`)

```go
ctx := context.Background()
```

Migration operations and individual YAML file processing use `context.Background()` with no timeout. If the database is slow or unresponsive, these operations can hang indefinitely. Migrations run at startup, so a timeout would be appropriate.

### M3: Server lack of ReadTimeout/WriteTimeout/IdleTimeout (`cmd/server/server.go:75, 91, 142-145`)

```go
ms := &http.Server{Addr: s.c.MetricsEndpoint, Handler: metricsServer, ReadHeaderTimeout: time.Minute}
// no ReadTimeout, WriteTimeout, IdleTimeout

pprofServer := &http.Server{Addr: s.c.PprofEndpoint, Handler: pprof.DefaultServeMux}
// no timeouts at all

server := http.Server{Addr: s.c.HttpServerEndpoint, ReadHeaderTimeout: 1 * time.Minute}
// no WriteTimeout, IdleTimeout
```

Only `ReadHeaderTimeout` is set. Missing `ReadTimeout` and `WriteTimeout` makes these servers vulnerable to slowloris-style denial-of-service attacks. The pprof server has no timeouts at all.

---

## Potential Issues

### P1: JSON field serialization in postgres `Create` (`postgres.go:113-119`)

```go
q := ds.sb.Insert(ds.tableName).SetMap(map[string]any{
    "id":       id,
    ds.jsonField: ve,
}).Suffix("RETURNING " + ds.jsonField)
```

The protobuf struct `ve` is passed directly as a map value to `squirrel.Insert().SetMap()`. Squirrel's default behavior may not serialize a protobuf to JSON automatically — it depends on the driver. If squirrel/sqlx tries to insert `ve` as a raw value rather than as JSON, the INSERT could silently fail or corrupt data. This should be verified or the struct should be explicitly marshaled to JSON before insertion.

### P2: `Now` variable shadowing in `postgres.go:182`

```go
pbNow, now := pbNow()
```

The local variable `pbNow` shadows the package-level function `pbNow()` at line 462. While Go scoping rules resolve this correctly (the right side calls the function, left side assigns to the variable), this is highly confusing and makes the code fragile to future refactoring. Renaming the variable to `nowPb` or `tsNow` would prevent confusion.

### P3: `BeginTx(ctx, nil)` uses no transaction options (`postgres.go:207-209`)

```go
tx, err := ds.db.BeginTx(ctx, nil)
```

Passing `nil` for `*sql.TxOptions` means no isolation level is specified. For operations with optimistic locking (using `version_number`), `Serializable` or `RepeatableRead` isolation would be more appropriate to prevent lost-update anomalies.

### P4: Config files processed twice in bootstrap (`pkg/datastore/bootstrap.go:47-60`)

```go
for _, f := range files { ... tbs.processConfig(f) ... }
for _, f := files { ... pbs.processConfig(f) ... }
```

All YAML files are parsed twice — once for tenants and once for projects. The `processConfig` filters by kind (line 122: `if kind != e.Kind() { return nil }`), so it's functionally correct but wastes I/O and parsing CPU. Files should be partitioned once and sent to the appropriate processor.

### P5: `connect` imported as blank in `pkg/errorutil/errors.go:3`

```go
import (
    "errors"
    "github.com/google/go-cmp/cmp"
    "connectrpc.com/connect"
)
```

The `cmp` import is only needed for `ConnectErrorComparer()` which is test utility code (lines 78-94). Having it in the production package bloats the binary. The comparer function and `cmp` import should be moved to a separate `_test` package or test helper file.

---

## Low Priority / Style

### L1: Commented-out test code (`pkg/service/tenant_test.go:30-158`)

A significant block of commented-out test code exists in the file. This is dead code that adds noise and should be removed or moved to a separate branch/history.

### L3: Return `err` as second value when known nil (`pkg/service/*.go`)

Throughout all service methods, the pattern `return response, err` is used even after `err` is known to be nil. This is stylistically inconsistent with `return response, nil` and could confuse linters or code reviewers.
