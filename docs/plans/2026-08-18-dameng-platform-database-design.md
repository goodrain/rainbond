# Rainbond 达梦平台数据库支持设计文档

## 一、项目背景

### 1.1 项目架构

Rainbond 平台由 `rainbond-operator` 根据 `RainbondCluster` 生成并维护
`rbd-api`、`rbd-worker` 和 `rbd-app-ui`。前两者共享 `region` 数据库，
Console 使用独立的 `console` 数据库。

当前链路将数据库实现固定为 MySQL：

```text
RainbondCluster.regionDatabase/uiDatabase
  -> Operator: --mysql=... + MYSQL_* 环境变量
  -> rbd-api/rbd-worker: GORM v1 + MySQL driver
  -> rbd-app-ui: Django MySQL backend + mysql client readiness check
```

### 1.2 现有基础

- Rainbond 核心使用 `github.com/jinzhu/gorm` v1。
- 达梦为 GORM v1 提供官方方言包，也提供官方 Go 驱动；GORM v1 仅推荐连接
  `CASE_SENSITIVE=N` 的 DM 数据库。
- Console 使用 Django 5.2；达梦提供 dmPython 和 dmDjango 3.x 后端。
- 当前 Operator 的 `Database` CR 类型没有数据库类型字段，预检查和工作负载
  参数均硬编码 MySQL。

### 1.3 核心需求

平台管理员能够在 `RainbondCluster` 中显式将 `regionDatabase` 和
`uiDatabase` 指向达梦，Operator 下发 `DB_TYPE=dm`，核心和 Console 使用达梦
原生协议与驱动连接。已有 MySQL 与 SQLite 部署必须保持兼容。

不在本次范围：把达梦伪装为 MySQL 协议、修改业务组件数据库插件、将数据库
账号密码明文写入 Git、或将现有 MySQL 数据无验证地迁入达梦。

## 二、用户旅程

### 2.1 用户操作流程

1. 平台管理员准备两个达梦 schema（`region` 与 `console`），并使用
   `CASE_SENSITIVE=N` 的独立达梦实例/库。
2. 管理员把非敏感连接信息和凭据引用写入 `RainbondCluster`，分别配置
   `regionDatabase.type: dm` 和 `uiDatabase.type: dm`。
3. Operator 使用达梦原生协议执行连通性检查，随后向 `rbd-api`、`rbd-worker`
   传递 `--db-type=dm` 和 DM DSN，向 `rbd-app-ui` 传递 `DB_TYPE=dm` 及通用
   `DB_*` 连接配置。
4. Console 以 dmDjango 执行 Django migration；核心以 GORM 达梦方言创建、迁移
   region schema。管理员通过 `RainbondCluster.status.conditions`、工作负载就绪
   状态和平台健康检查确认结果。

### 2.2 页面原型

本期没有 UI 入口；配置入口为 `RainbondCluster`（YAML/Helm/ROI）和 Operator
状态。后续若在控制台暴露平台数据库配置，需单独设计凭据 Secret 引用与权限。

### 2.3 外部系统交互

- 达梦数据库：使用官方原生 Go/dmPython/dmDjango 驱动，不走 MySQL 协议。
- Kubernetes：Operator 将数据库类型与连接信息渲染进受管组件。
- 镜像构建：DM 驱动由持证环境在构建时提供；驱动源码不提交到开源仓库。

## 三、整体架构设计

### 3.1 系统架构图

```text
RainbondCluster Database.type=dm
        |
        v
rainbond-operator
  |- precheck: database/sql + DM driver
  |- rbd-api/rbd-worker: --db-type=dm, --db=<dm DSN>
  `- rbd-app-ui: DB_TYPE=dm, DB_HOST/PORT/USER/PASSWORD/NAME
        |                         |
        v                         v
rainbond core: GORM v1 DM dialect  Console: dmDjango + dmPython
        \                         /
         `---- DM native protocol ----> Dameng
```

### 3.2 核心流程

`Database.type` 缺省为 `mysql`，确保所有现有 CR 不变。`dm` 类型必须通过
`dm://user:password@host:port/schema` 形式的 DSN 生成器，禁止沿用
`user:password@tcp(host:port)/database`。

核心不再把连接字段命名为 `MysqlConnectionInfo`；改为中性的连接信息名称，
同时保留旧 flag `--mysql` 作为兼容别名。核心仅接受白名单中的数据库类型。

Console 读取 `DB_TYPE`；MySQL 保留旧的 `MYSQL_*` 回退变量，DM 使用通用
`DB_*`。启动脚本用 Django connection/introspection 检查连接和空库，不依赖
`mysql` CLI。

## 四、数据模型设计

### 4.1 新增数据库表

无新增平台业务表。现有 `region` 和 `console` schema 由 GORM/Django 的迁移机制
创建。

### 4.2 数据关系

扩展 Operator CR 的 `Database`：

```yaml
regionDatabase:
  type: dm
  host: <DM_HOST>
  port: 5236
  username: <REGION_DB_USER>
  password: <REFERENCE_OR_INJECTED_SECRET>
  name: region
uiDatabase:
  type: dm
  host: <DM_HOST>
  port: 5236
  username: <CONSOLE_DB_USER>
  password: <REFERENCE_OR_INJECTED_SECRET>
  name: console
```

第一阶段为兼容当前 CR 字段而保留 `password`，但渲染后不得记录 DSN/密码。后续
兼容版本应新增 `secretRef` 并废弃明文 `password`。

## 五、API 设计

### 5.1 接口列表

无新增 HTTP API。扩展 Kubernetes CRD 的 `spec.regionDatabase.type` 与
`spec.uiDatabase.type`，取值为 `mysql`（默认）或 `dm`。

### 5.2 请求/响应结构

Operator 生成的组件配置契约：

| 组件 | MySQL（兼容） | DM |
| --- | --- | --- |
| rbd-api / rbd-worker | `--db-type=mysql` + 旧 `--mysql` DSN | `--db-type=dm` + DM DSN |
| rbd-app-ui | `DB_TYPE=mysql`，兼容 `MYSQL_*` | `DB_TYPE=dm` + `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME` |

## 六、核心实现设计

### 6.1 关键逻辑

1. 引入由构建环境提供的官方 DM Go driver 与 GORM v1 dialect，注册 `dm` 方言。
   用 Go build tag 将 DM 支持与默认 MySQL 镜像隔离，DM 变体必须在构建时失败
   （而不是在运行时静默回退）若驱动不存在。
2. 为 GORM 初始化、建表、schema 修补、批量 upsert、索引检查分别实现 DM 分支；
   不执行 MySQL 的 `ENGINE=InnoDB`、`information_schema`、`GROUP_CONCAT` 或
   `MODIFY COLUMN` 语句。
3. Console 使用 dmDjango，启动前以 Django API 检查连通性，所有 MySQL 专属
   `GROUP_CONCAT` 原生查询改为 ORM 或经数据库类型选择的等价表达式。
4. Operator 根据类型构造 DSN、执行预检查、渲染核心 flags 和 Console 环境变量。
   凭据在错误和日志中脱敏。
5. 修改 CRD、deepcopy、Helm/ROI 传递链路，并新增配置示例与迁移操作手册。

### 6.2 复用现有代码

- `db.Manager`、DAO 接口与现有 GORM model 保持复用。
- `goodrain_web.settings` 保留 MySQL/SQLite 分支并新增 DM 分支。
- Operator 的 `Database`、`RegionDataSource`、database precheck、api/worker/app-ui
  handlers 作为唯一配置渲染点。

## 七、实施计划

### 跨层覆盖检查

- [x] Go (rainbond): 需要 — 驱动注册、GORM DM 方言、DSN、方言化 schema/SQL、测试和 DM 镜像构建目标。
- [x] Python (rainbond-console): 需要 — dmDjango 设置、通用启动检查、镜像驱动安装、原生 SQL 兼容和测试。
- [ ] React (rainbond-ui): 不涉及 — 本期配置入口是 CR/YAML，没有用户 UI 触点。
- [ ] Plugin frontend (enterprise-base): 不涉及。
- [ ] Plugin backend (plugin-template): 不涉及。
- [x] rainbond-operator: 需要 — CRD 类型、预检查、配置渲染、受管工作负载测试与镜像构建。

### Sprint 1: 契约与核心驱动

#### Task 1.1: 定义数据库类型和 DSN 契约
- 仓库：rainbond-operator / rainbond
- 文件：`api/v1alpha1/rainbondcluster_types.go`、`config/configs/db_config.go`、`db/config/config.go`
- 实现内容：类型白名单、MySQL 默认值、DM DSN、无密码日志。
- 验收标准：单元测试覆盖默认 MySQL、DM、未知类型和含特殊字符凭据的脱敏日志。

#### Task 1.2: 接入 GORM v1 达梦方言
- 仓库：rainbond
- 文件：`db/`、`go.mod`、`Dockerfile`、DM 构建说明
- 实现内容：注册官方 DM driver/dialect、实现 DM 的建表与 schema 修补分支。
- 验收标准：在 `CASE_SENSITIVE=N` DM integration database 中完成建表、迁移、事务和 DAO 冒烟测试。

### Sprint 2: Console 与 Operator

#### Task 2.1: Console 连接和迁移支持
- 仓库：rainbond-console
- 文件：`goodrain_web/settings.py`、`entrypoint.sh`、`Dockerfile`、受影响 repository/service
- 实现内容：DM 配置、dmDjango 安装、通用 readiness、DM 原生 SQL 等价实现。
- 验收标准：Django `check`、`migrate`、创建/查询/事务冒烟测试通过，密码不出现在日志。

#### Task 2.2: Operator 下发与预检查
- 仓库：rainbond-operator
- 文件：CR 类型/CRD、`controllers/cluster-mgr/precheck/database.go`、`controllers/handler/{api,worker,app-ui}.go`
- 实现内容：按数据库类型检测、传递 `DB_TYPE=dm`、核心 DM flags、Console DB 环境变量。
- 验收标准：controller 单元测试验证 MySQL 回归和 DM 工作负载渲染，不含密码的错误消息。

### Sprint 3: 集成验收与迁移文档

#### Task 3.1: 验证和发布材料
- 仓库：rainbond / rainbond-console / rainbond-operator
- 文件：测试、构建说明、运维文档
- 实现内容：使用独立 DM 测试库执行全链路安装/升级验证，给出现有 MySQL 平台迁移、回滚和备份步骤。
- 验收标准：Go build/vet/test、Console check/pytest、Operator test/build、DM 部署健康检查均通过。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
| --- | --- | --- |
| 核心驱动选择 | `rainbond/db/db.go` | 当前只允许 mysql/cockroachdb/sqlite。 |
| GORM 初始化与 MySQL 专属 SQL | `rainbond/db/mysql/mysql.go` | 需拆出 DM 方言分支。 |
| 核心 DB flags | `rainbond/config/configs/db_config.go` | 当前仅有 `--mysql`。 |
| Console 数据库配置 | `rainbond-console/goodrain_web/settings.py` | 当前仅 SQLite/MySQL。 |
| Console 启动数据库检查 | `rainbond-console/entrypoint.sh` | 当前依赖 MySQL CLI。 |
| Operator CR 与 DSN | `rainbond-operator/api/v1alpha1/rainbondcluster_types.go` | 当前数据库类型/DSN 固定 MySQL。 |
| Operator 预检查 | `rainbond-operator/controllers/cluster-mgr/precheck/database.go` | 当前固定 `sql.Open("mysql")`。 |
| Operator 工作负载渲染 | `rainbond-operator/controllers/handler/{api,worker,app-ui}.go` | 当前下发 MySQL flag/env。 |

## 前置条件与风险

1. 达梦官方 GORM v1 文档要求 `CASE_SENSITIVE=N`。当前测试实例如果为 `Y`，必须
   新建符合条件的测试库；不能把它作为验收基线。
2. 官方 Go/dmDjango/dmPython 驱动随 DM 安装包分发，未在 Rainbond 源码中提供。
   在获得可再分发授权前，仓库只能提交构建接入点和驱动获取说明，不能提交驱动源码。
3. 平台现有 MySQL 数据库不能直接切换。正式切换需先完成备份、SQL/数据迁移、离线
   验收与回滚演练。
