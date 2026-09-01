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
make check              # CI lint check (golint on changed files) + test manifest validation
make test-manifest-check # Validate test-manifest.json against capability_id markers
make build              # Build binaries via localbuild.sh
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

## Rainbond Development Workflow Override

When working in this Rainbond repository, the development flow MUST follow this chain automatically. The user does NOT need to type any slash command — each step flows into the next naturally.

### Automatic Flow Chain

1. **User describes a feature or task** (natural language)
2. **Superpowers brainstorming activates** (auto, via session hook — do NOT skip)
   - After approval, the design document MUST use the **Rainbond 7-section template** (see below)
   - Save to `docs/plans/YYYY-MM-DD-<topic>-design.md` and git commit
3. **Superpowers worktrees** — create isolated workspace
4. **Run `/spec-gen`** — convert design document into YAML task specification with commit grouping, 2-5 min step granularity, complete code, and line-number precision (replaces `writing-plans`)
5. **Execution** — use Superpowers `subagent-driven-development` via `/spec-driven`:
   - For each commit group, dispatch a **fresh subagent** per task
   - Each subagent follows `test-driven-development` (Red-Green-Refactor) **for Go/Python**
   - **For React (rainbond-ui):** use `yarn build` as quality gate + `frontend-patterns` review (no TDD)
   - After each task: **two-stage review** (spec compliance → code quality)
   - When all tasks pass review → `git commit` with the spec's commit message
   - Proceed to next commit group
6. **If tasks are independent** → use Superpowers `dispatching-parallel-agents`
7. **After all commits** → run `/go-review` (Go) or `/python-review` (Python) or `frontend-patterns` (React)
8. **If cross-repo** → run `/check-api-compat`
9. **Finally** → use Superpowers `finishing-a-development-branch`

### Rainbond 7-Section Design Template

When brainstorming produces a design document for this Rainbond repository, it MUST follow this structure:

```markdown
# {项目名称} 设计文档

## 一、项目背景
### 1.1 项目架构
### 1.2 现有基础
### 1.3 核心需求

## 二、整体架构设计
### 2.1 系统架构图
### 2.2 核心流程

## 三、数据模型设计
### 3.1 新增数据库表
### 3.2 数据关系

## 四、API设计
### 4.1 接口列表
### 4.2 请求/响应结构

## 五、核心实现设计
### 5.1 关键逻辑
### 5.2 复用现有代码

## 六、实施计划
### Sprint 1: {阶段名称}
#### Task 1.1: {任务名称}
- 文件：{精确路径:行号}
- 实现内容：
- 验收标准：

## 七、关键参考代码
| 功能 | 文件 | 说明 |
|------|------|------|
```

### What comes from where

| Capability | Source | Why |
|-----------|--------|-----|
| Requirement discussion + hard gate | Superpowers `brainstorming` | Mandatory, cannot be skipped |
| 7-section design template | Project instructions (injected into brainstorming) | Rainbond-specific architecture |
| Isolated workspace | Superpowers `using-git-worktrees` | Branch isolation |
| YAML spec + commit grouping + 2-5 min steps | Rainbond `/spec-gen` (replaces `writing-plans`) | Richer structure than writing-plans |
| Fresh subagent per task | Superpowers `subagent-driven-development` | Solves context pollution |
| Two-stage adversarial review | Superpowers `subagent-driven-development` | Spec compliance + code quality |
| Parallel task execution | Superpowers `dispatching-parallel-agents` | Speed up independent tasks |
| TDD iron law | Superpowers `test-driven-development` | No code without failing test |
| Evidence before completion | Superpowers `verification-before-completion` | No unverified claims |
| Commit grouping + auto-commit | Rainbond `/spec-driven` | Logical commit units |
| Language-specific review | ECC `/go-review`, `/python-review` | Idiomatic checks |
| Cross-repo API check | Rainbond `/check-api-compat` | Multi-repo consistency |
| Branch completion | Superpowers `finishing-a-development-branch` | Merge/PR workflow |

### Key Rules

- **Do NOT use `writing-plans`** in this Rainbond repository — `/spec-gen` replaces it with YAML + commit grouping + step-level code
- **DO use `subagent-driven-development`** — it provides the execution engine (subagent isolation + adversarial review)
- **DO use `executing-plans`** as alternative — for parallel session batch execution
