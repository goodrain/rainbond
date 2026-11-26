# Rainbond — Go Core Services

## Overview

Rainbond is a cloud-native application management platform. This repository contains the Go backend services that interface directly with Kubernetes. It is called by `rainbond-console` (Django) via HTTP REST APIs.

- Language: Go 1.25 (see `go.mod`)
- Module: `github.com/goodrain/rainbond`
- Router: go-chi/chi
- ORM: jinzhu/gorm (v1)
- Logging: sirupsen/logrus
- Dependencies: Go modules, no `vendor/` directory (CI runs `go test -mod=mod`)

## Binary Components

Entry points in `cmd/`:

| Binary | Description |
|--------|-------------|
| `cmd/api` | REST API server (the main service called by console) |
| `cmd/builder` | Source code build and image build service (image `rbd-chaos`) |
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
go test -mod=mod ./... -count=1   # What CI runs (needs libzmq3-dev / pkg-config)
make check              # CI lint check (golint on changed files) + test manifest validation
make test-manifest-check # Validate test-manifest.json against capability_id markers
```

## Test Manifest (CI-enforced)

`test-manifest.json` is a registry of behavior-guarding tests. Any test annotated
with a `capability_id` comment is a "managed" test and MUST have a matching entry
in the manifest, otherwise CI fails.

- Marker: `// capability_id: rainbond.<area>.<behavior>` (Go) or
  `# capability_id: ...` (Python), placed directly above the test function.
- Enforced by `scripts/validate_test_manifest.py`, invoked from `make check` and
  `make test-manifest-check`. Runs in CI as the **Check test manifest** step in
  `.github/workflows/pr-ci-build.yml` and `release-v6.yml`.
- `test-manifest.md` is the human-readable table — **auto-generated, never edit by hand**.

### Registering a managed test

Do NOT hand-edit `test-manifest.json`. Use the manager — it inserts in sorted order
and regenerates `test-manifest.md`:

```bash
python3 scripts/manage_test_manifest.py add rainbond.<area>.<behavior> \
  --title "Short English summary" \
  --interface-type workflow \
  --interface "builder/sources.buildKitTomlContent" \
  --code-path builder/sources/image.go \
  --test builder/sources/buildkit_toml_test.go::TestBuildKitTomlContent \
  --test-type regression
```

- `interface_type`: one of `service_method | view_endpoint | handler_method |
  dao_method | package_function | workflow | other` (internal funcs/methods use `workflow`).
- `test_type`: one of `unit | regression | characterization | integration`.
- Repeat `--code-path` / `--test` for multiple values; `--status` defaults to `active`.
- Other subcommands: `list`, `show <id>`, `render` (rebuild the `.md`), `prune <id>`.

## Coding Conventions

- Use `logrus` for all logging (not `log` or `fmt.Println`)
- Use `httputil.ReturnSuccess/ReturnError/ReturnBcodeError` for HTTP responses
- Use `chi.URLParam(r, "param")` for path parameters
- Use `r.Context().Value(ctxutil.ContextKey("key"))` for middleware-injected values
- Run `goimports` before committing
- Error codes defined in `api/util/bcode/`
- Commit messages in English, Conventional Commits format

## Agent Workflow (bug fixes and PRs)

These rules apply to every AI agent working in this repo (Multica agents, the
`claude` / `codex` / `mimo` comment bots, local sessions). CI enforces some of
them; a PR that breaks the others will be sent back.

### Scope

- One issue → one branch → one PR. Make the minimal correct change; do not
  refactor, reformat, rename or upgrade dependencies unless the issue asks for it.
- Find the repo that actually owns the bug before editing
  (UI → `rainbond-ui`, console API / permissions / DB → `rainbond-console`,
  Kubernetes / build / runtime → `rainbond`). A fix that spans repos gets one PR
  per repo, linked to each other.
- Reproduce first. If you cannot reproduce or cannot find the cause, report what
  you checked instead of guessing a fix.

### Branches

- Branch off the latest `main`; name it `fix/<short-slug>` or `feat/<short-slug>`
  (lowercase, no personal names). The comment bots keep the branch name their
  workflow prompt gives them.
- Push the branch to this repository itself, not a fork: the E2E gate only tests
  same-repo branches, and it builds components by branch name.
- Never push to `main`, never merge a PR, never force-push a branch you did not
  create. Humans merge.

### Commits (CI-enforced)

- First line: `type(scope): summary`, type one of
  `feat fix docs style refactor test ci perf revert chore`; English.
- **Every line ≤ 72 characters**, body included (Check Commit Message fails otherwise).
- Sign off every commit: `git commit -s` (the DCO check is required).
- No AI attribution lines (`Co-authored-by` an AI, "Generated with …").

### Tests

- Add a regression test that fails before the fix and passes after it, when the
  code path is testable. Call the real code; never copy the logic under test into
  the test.
- A test marked with a `capability_id` comment must be registered in
  `test-manifest.json` with `scripts/manage_test_manifest.py add` (see above);
  never hand-edit `test-manifest.md`.
- Run the verification commands below and paste the actual results into the PR.

### Pull request

- Title in the same Conventional Commits format. Body:
  **Problem** (link the issue / Multica task), **Root cause**, **Fix**,
  **Verification** (commands and results), **Risk**.
- After `pr-ci-build` passes, request the functional gate by adding the
  `run-e2e` label. It deploys the branch to the shared staging environment and
  reports `e2e/product-gate` on the PR. Runs are serialized; do not re-trigger
  while one is queued. If you cannot add labels, say so in the PR.
- Report back where the task came from: PR link, CI result, E2E result, and
  anything you could not verify.

### Stop and ask a human first when the fix

- changes the database schema beyond adding a nullable or defaulted
  column/field, or touches migrations;
- changes an API contract (console ↔ region `/v2/...`, `/openapi/v1/...`,
  response fields) or removes/renames a field;
- adds or renames configuration (env vars, Helm values, CRD fields);
- touches install, upgrade or release paths;
- needs a new dependency, or tests fail for reasons unrelated to your change.

### Never

- Edit `.github/workflows/`, release tooling, or generated files by hand.
- Print, commit or ask for credentials (tokens, passwords, AK/SK, kubeconfig).
- Run anything against production clusters, registries or download sites.

### Verify before opening a PR (rainbond)

```bash
go build ./...
go vet ./<changed packages>/...
go test -mod=mod ./<changed packages>/... -count=1
TARGET_BRANCH=main ./localcheck.sh   # golint on changed .go files
make test-manifest-check             # if you added or changed capability_id tests
```

Build one component the way its Dockerfile (`hack/contrib/docker/<comp>/`) does:

```bash
CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" \
  go build -tags "sqlite_omit_load_extension netgo" -o rainbond-api ./cmd/api
```

The Makefile `build` / `image` / `binary` targets call scripts that no longer
exist; do not use them.

### Repo-specific risks (rainbond)

- **Schema**: there are no migration files. `db/mysql/mysql.go` `CheckTable()`
  creates missing tables, runs GORM `AutoMigrate`, then `patchTable()`.
  AutoMigrate only adds tables and columns; renames, drops and type changes need
  a `patchTable()` step and human review. Region runs on MySQL and sqlite, so
  test model changes with sqlite (`t.TempDir()`, see
  `db/mysql/mysql_long_version_test.go`).
- **Mocks**: after changing an interface in `db/db.go`, `db/dao/dao.go`,
  `event/manager.go` or `worker/appm/store/store.go`, regenerate with
  `./mockgen.sh`; do not edit `*_mock.go` by hand.
- **CRDs**: `pkg/apis/` changes need `make manifests` / `make generate`.
- Do not edit `third_party/` or `hack/release/`.
- The E2E gate deploys `rbd-api` by default; mention it in the PR if the fix is
  in `rbd-worker`, `rbd-chaos` or `rbd-mq`, which need extra components deployed.
