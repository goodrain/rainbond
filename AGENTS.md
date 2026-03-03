# Rainbond — Go Core Services

## Overview

Rainbond is a cloud-native application management platform. This repository contains the Go backend services that interface directly with Kubernetes. It is called by `rainbond-console` (Django) via HTTP REST APIs.

- Language: Go 1.23
- Module: `github.com/goodrain/rainbond`
- Router: go-chi/chi
- ORM: jinzhu/gorm (v1)
- Logging: sirupsen/logrus
- Vendor: dependencies vendored in `vendor/`

## Binary Components

Entry points in `cmd/`:

| Binary | Description |
|--------|-------------|
| `cmd/api` | REST API server (the main service called by console) |
| `cmd/builder` | Source code build and image build service |
| `cmd/worker` | Application runtime management, Kubernetes operator |
| `cmd/mq` | Message queue service for async tasks |
| `cmd/grctl` | CLI tool for cluster management |
| `cmd/init-probe` | Init container health probe |

## Key Directories

```
api/
  api_routers/version2/  — chi route registration (v2Routers.go)
  controller/            — HTTP handlers (request parsing, response writing)
  handler/               — Business logic layer
  model/                 — API request/response structs
  middleware/             — Auth, tenant context, CORS
  proxy/                 — Reverse proxy utilities
db/
  model/                 — GORM model definitions (database schema)
  dao/                   — Data access objects (CRUD operations)
  mysql/                 — MySQL-specific DAO implementations
builder/                 — Build system (source code, Docker, slug)
worker/                  — Kubernetes controller/operator logic
pkg/
  apis/rainbond/v1alpha1/ — CRD type definitions
  component/             — Shared components (k8s client, storage)
util/
  http/                  — HTTP response helpers (ReturnSuccess, ReturnError, ReturnBcodeError)
```

## Architecture: Request Flow

```
HTTP Request → chi Router → Middleware → Controller → Handler → DAO → Database
                                                        ↓
                                                   Kubernetes API
```

- Controllers: parse HTTP request, validate input, call handler, write response
- Handlers: business logic, orchestrate DAO calls and K8s operations
- DAOs: database CRUD via GORM

## Adding a New API Endpoint

1. Define request/response structs in `api/model/`
2. Add GORM model in `db/model/` if new table needed
3. Add DAO interface in `db/dao/` and implementation in `db/mysql/`
4. Implement business logic in `api/handler/`
5. Add controller method in `api/controller/`
6. Register route in `api/api_routers/version2/v2Routers.go`

## Code Patterns

### Controller Pattern
```go
func (t *TenantStruct) CreateSomething(w http.ResponseWriter, r *http.Request) {
    var req model.CreateSomethingRequest
    if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
        return
    }
    // Extract context values
    tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)

    result, err := handler.GetSomethingHandler().Create(tenantID, &req)
    if err != nil {
        httputil.ReturnBcodeError(r, w, err)
        return
    }
    httputil.ReturnSuccess(r, w, result)
}
```

### Handler Singleton Pattern
```go
// In handler package, handlers are accessed via GetXxxHandler() functions
handler.GetAppHandler().ExportApp(&tr)
handler.GetApplicationHandler().AddConfigGroup(appID, &configReq)
```

### DAO Access Pattern
```go
db.GetManager().AppDao().AddModel(app)
db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
```

## Cross-Repository Relationship

- This repo is called by `rainbond-console` via `www/apiclient/regionapi.py` (RegionInvokeApi)
- API base path: `/v2/tenants/{tenant_name}/...`
- Console sends requests with region token in Authorization header

## Build & Verify

```bash
go build ./...          # Compile all packages
go vet ./...            # Static analysis
make check              # CI lint check (golint on changed files)
make build              # Build binaries via localbuild.sh
```

## Coding Conventions

- Use `logrus` for all logging (not `log` or `fmt.Println`)
- Use `httputil.ReturnSuccess/ReturnError/ReturnBcodeError` for HTTP responses
- Use `chi.URLParam(r, "param")` for path parameters
- Use `r.Context().Value(ctxutil.ContextKey("key"))` for middleware-injected values
- Run `goimports` before committing
- Error codes defined in `api/util/bcode/`
- Commit messages in English, Conventional Commits format
