# Rainbond 数据库可移植性设计文档

## 一、项目背景

### 1.1 项目架构

```text
rainbond-ui
    -> rainbond-console (Django, console schema)
    -> rainbond (Go region services, region schema)
    -> MySQL / 达梦等关系数据库
```

Console 与 region 使用同一数据库实例时仍是两个独立逻辑 schema，必须各自迁移、各自验收。数据库供应商的选择和连接信息由平台安装配置下发给 `rbd-api`、`rbd-worker`、`rbd-chaos` 和 `rbd-app-ui`。

### 1.2 现有基础

- Go 侧使用 GORM v1，支持 MySQL、CockroachDB、SQLite；33 个 DAO 批量写入点依赖 MySQL 风格的 `gorm-bulk-upsert`。
- Go 服务在每个副本启动时都会执行 `CreateTable`、`AutoMigrate`、历史 DDL 补丁和种子数据写入；这不是多副本安全的迁移模型。
- Go 模型中至少 13 处硬编码 `longtext`，并有 11 处 MySQL `ALTER ... MODIFY`、`information_schema`、`GROUP_CONCAT` 等数据库特定 SQL。
- Console 仅配置 MySQL/SQLite；入口脚本在容器启动时运行 `makemigrations`、修复历史表、再执行 `migrate`。`migrations/` 被 `.gitignore` 和镜像 `.dockerignore` 排除，因此初始 schema 不是受版本控制的交付物。
- Console 至少有 4 处 `GROUP_CONCAT`，以及原生查询、`LIMIT offset,count`、`CAST(... AS SIGNED)` 等 MySQL 假设。
- 达梦官方驱动不来自公共容器镜像：Go 驱动位于 DM 安装包的 `drivers/go`；Python `dmPython` 需要从安装包源码编译，并随运行镜像提供 DPI 动态库和库路径。驱动构建必须成为镜像的可重复输入，不能在用户部署时临时选择或下载。

### 1.3 核心需求

在不降低现有 MySQL 行为和性能的前提下，建立明确、可测试的数据库可移植层，首个目标是支持达梦。目标不是把所有 SQL 强行改成最低公共语法，也不是为每次达梦错误增加一处分支；目标是把数据库差异限定在少数已声明的能力实现中。

本期成功标准：

1. MySQL 和达梦均可从空 schema 由平台唯一迁移任务初始化。
2. 任意数量的 API、worker、chaos、console 副本只验证 schema，不再并发改表或写种子数据。
3. 批量写入、事务、长文本、索引和已审计的原生查询有明确的跨数据库语义。
4. MySQL 继续使用现有的优化路径；达梦走已验证的实现；未声明支持的数据库在启动前明确失败。

## 二、用户旅程（MUST — 禁止跳过）

### 2.1 用户操作流程

- 平台安装管理员在安装配置中选择受支持的数据库类型，并填写连接信息与 region/console 的逻辑 schema。
- Operator 根据配置创建或更新两个一次性迁移 Job：region Job 和 console Job。每个 Job 的状态和失败原因可在 Kubernetes Job/Event 中查看。
- 两个 Job 成功后，Operator 才发布或重启控制面服务；正常服务仅做连接和 schema 版本校验。
- 升级版本时，Operator 以版本化 Job 再次执行幂等迁移；同一 schema 同一版本只允许一个 Job 生效。
- 用户继续使用现有 UI 创建、构建、部署、配置组件，无新增业务页面或 API。

### 2.2 页面原型

- 不新增业务 UI 页面。
- 安装和运维层需展示现有 Kubernetes Job 状态：`region database migration`、`console database migration`。
- 安装配置页/`RainbondCluster` 配置字段是唯一新增的管理员触点；不修改团队用户工作流。

### 2.3 外部系统交互

- MySQL：默认且必须保持兼容。
- 达梦：通过官方 Go、Python 驱动及运行时库访问；连接凭据只以 Secret/环境变量注入，不进入镜像、代码、日志或文档。
- Kubernetes Operator：负责把数据库配置、安全引用、迁移 Job 和服务依赖关系落到集群对象。

## 三、整体架构设计

### 3.1 系统架构图

```text
RainbondCluster / installation database configuration
                    |
                    v
          Operator database reconciler
             |                     |
             v                     v
 rbd-region-db-migrate Job   rbd-console-db-migrate Job
             |                     |
             +---------+-----------+
                       v
        schema/version/seed state is ready
                       |
                       v
 rbd-api / rbd-worker / rbd-chaos / rbd-app-ui
        open connection + verify only
                       |
                       v
  Database capability boundary
  - physical type mapping
  - transaction ownership
  - semantic batch upsert
  - schema/index operations
  - isolated vendor SQL
```

### 3.2 核心流程

1. 配置校验：数据库类型必须在受支持列表内，连接、权限、字符集、schema 可访问性在 Job 中校验。
2. region 迁移：专用二进制打开数据库，按固定模型清单创建/升级表、执行受版本控制的历史数据补丁、写种子数据、建立索引并记录成功版本。
3. console 迁移：专用命令执行已提交的 Django migrations 和初始化数据；禁止在运行容器内生成 migration 文件。
4. 服务启动：所有普通副本只执行 `Open` 与 `Verify`。缺表或版本不足时明确失败并提示先查看迁移 Job。
5. 数据访问：业务 DAO 调用“批量 upsert”这一语义 API，而不是直接依赖 `ON DUPLICATE KEY UPDATE`；原生 SQL 通过能力模块或 ORM 表达，不能散落供应商判断。

## 四、数据模型设计

### 4.1 新增数据库表

不新增业务表。为迁移可观测性，region schema 使用一个平台内部的迁移版本记录（名称和字段在实施前确定），至少包含：迁移标识、应用版本、执行时间、成功状态和校验摘要。

Console 使用 Django 自带的 `django_migrations` 作为版本记录；初始 migration 必须纳入版本控制。

### 4.2 数据关系

```text
region migration record  -> region model tables + region seed rows
django_migrations        -> console/www/auth model tables + console seed rows
```

数据类型原则：

- 模型表达业务含义，例如“短文本”“大文本”“时间”“布尔”“JSON 文本”，不在业务模型中扩散供应商字面类型。
- 物理类型由 provider 映射：MySQL 保持现有适当类型；达梦的大文本映射为 CLOB 等等效类型。
- 既有数据迁移必须是幂等的，重复执行不会覆盖用户配置或重复插入种子数据。

## 五、API 设计

### 5.1 接口列表

不新增面向终端用户的 HTTP API。新增内部执行契约：

- Go region 迁移命令：显式 `migrate` 模式。
- Go 服务运行模式：显式 `verify` 模式。
- Console 迁移命令：受版本控制 migration 的非交互执行模式。
- Operator 与迁移 Job 的配置契约：数据库类型、Secret 引用、逻辑 schema、目标版本、Job 名称/状态。

### 5.2 请求/响应结构

服务对 Console 和 UI 的既有 REST 协议不变。数据库 provider 不透传至租户业务 API；失败信息只返回“迁移未完成/数据库不可用”的脱敏原因，不返回连接串或凭据。

## 六、核心实现设计

### 6.1 关键逻辑

#### 6.1.1 Provider 与能力边界

在 Go 中新增窄接口，而不是在 33 个 DAO 中插入 `if dbType == "dm"`：

- `Open`：打开驱动、标准化连接池、Ping、注册 GORM 方言。
- `Schema`：创建/升级/验证模型、表选项、索引和历史补丁。
- `Types`：逻辑大文本等类型的物理映射。
- `Upsert`：按每个 DAO 的已定义唯一标识执行写入；MySQL 可保留批量优化，其他 provider 使用正确的事务内更新/插入或 provider 原生合并语句。
- `SQLCapabilities`：少量确实无法用 ORM 表达的功能（索引元数据、分页、聚合、字符串拼接）集中实现。

不采用反射猜测唯一键的通用 upsert，也不再复制和临时修改第三方 bulk-upsert 模块；前者会造成数据语义不明确，后者已导致事务和不可寻址值等不可重复问题。

#### 6.1.2 事务规则

- 事务由 handler/service 边界创建和结束。
- DAO 和批量工具接收调用方传入的 `*gorm.DB`，若已在事务中，绝不再次 `Begin`。
- 每个批量写入在同一调用方事务中要么全成功，要么全回滚；唯一约束冲突只在 provider 实现规定的重试范围内处理。
- 增加事务回滚、嵌套调用和批量写入的回归测试，防止再次出现“can’t start transaction”或“using unaddressable value”。

#### 6.1.3 region schema 生命周期

- 从 `CreateManager` 中拆出 `RegisterModels`、`Migrate`、`Verify`。
- 当前 `patchTable` 中的 MySQL `ALTER ... MODIFY`、`GROUP_CONCAT`、`information_schema` 等逐条分类：可由 GORM 处理的改为 ORM；需方言的进入 Schema provider；不再在普通服务启动时执行。
- 创建 `rbd-region-db-migrate` Job 作为唯一执行者。Job 成功后才能滚动控制面服务。
- MySQL 已有集群升级必须兼容：首次启用新机制时识别已有表和种子，安全写入迁移记录，不重建、不丢数据。

#### 6.1.4 console schema 生命周期

- 提交稳定的初始 Django migrations；从 `.gitignore`、`.dockerignore` 和启动脚本中移除“运行时生成 migration”依赖。
- `rbd-console-db-migrate` Job 只运行 `migrate` 和受控初始化/修复步骤；Web 容器只连接并验证。
- Django DM backend 的 `ENGINE`、连接字段、DPI 库路径和构建产物通过单一 settings/provider 配置处理。
- Console 的原生 SQL 分为：可改 ORM、可移植参数化 SQL、供应商 capability SQL。所有字符串拼接 SQL 同时改为参数化，避免兼容性与安全风险叠加。
- 从数据库游标返回的列名统一为逻辑小写键，避免达梦未加引号标识符返回大写导致 Console 访问字典失败。

#### 6.1.5 驱动与镜像交付

- 从经批准的 DM 安装介质抽取 Go driver、dmPython 源码和 DPI 运行库，生成带校验摘要的内部 driver bundle。
- Go 和 Console Dockerfile 都从同一个受控 bundle 构建；不依赖 Docker Hub 中不存在的 `dameng` 镜像，不在最终用户部署时联网下载。
- 镜像同时包含 MySQL 驱动和 DM 驱动；选择 provider 只影响运行配置，不影响镜像是否具备驱动。
- 构建流程必须验证：Go 编译、Python `import dmPython`/Django backend import、运行库动态链接检查。

### 6.2 复用现有代码

- 复用 GORM v1、现有 DAO 接口和 `db/model` 模型清单，避免一次升级 ORM。
- 复用 Console 的 Django ORM 与既有 `repair_legacy_schema` 测试，将它从启动时的补救逻辑改为迁移 Job 的受控兼容步骤。
- 复用 `RbdComponent`/`RainbondCluster` 配置下发机制；正式自动化需要 Operator 代码共同实现 Job 编排。
- 不涉及 UI、插件前后端的业务代码改造；UI 仅用于 E2E 回归。

## 七、实施计划

### 跨层覆盖检查（MUST）

- [x] Go (rainbond): 需要 — provider 抽象、DM driver/dialect、Schema migrate/verify、upsert、事务和 SQL 能力测试。
- [x] Python (rainbond-console): 需要 — DM backend 配置、受控 migrations、入口脚本职责拆分、原生 SQL 收口和测试。
- [x] React (rainbond-ui): 不涉及 — 无用户交互或 API 契约变化；仅执行 E2E 回归。
- [x] Plugin frontend (enterprise-base): 不涉及 — 无插件 UI 变更。
- [x] Plugin backend (plugin-template): 不涉及 — 无插件 API 或数据库变更。
- [x] Operator: 需要 — 迁移 Job、Secret/环境变量、服务依赖和 Job 状态编排；当前工作区未检出 Operator 源码，实施前需引入对应仓库并按同一 feature branch 协同提交。

### Phase 0：基线与契约（本次先完成）

1. 固化本设计和任务规范；所有后续提交绑定 capability 测试清单。
2. 为 MySQL、SQLite、达梦定义明确支持级别和最小能力矩阵。
3. 将既有原生 SQL、类型、upsert、事务和迁移代码分类，禁止未审计的批量复制式改动。

### Phase 1：Go 数据库基础

1. 引入 provider 注册、连接配置和逻辑类型映射，保持 MySQL 默认行为不变。
2. 将 region 服务启动从“迁移”改为“验证”，提供可独立运行的迁移命令。
3. 将 33 个 upsert 调用收敛到语义 API，逐个定义其唯一键和事务归属。
4. 迁移已审计的 MySQL 特定 DDL、索引和原生 SQL；每类至少有 MySQL + DM 行为测试。

### Phase 2：Console 基础

1. 生成并提交可重复的初始 migrations，移除运行时 `makemigrations`。
2. 增加数据库 provider settings 与镜像内驱动验证。
3. 用 Django ORM 或 capability helper 改造高风险 SQL：聚合、分页、类型转换、游标字段名和字符串拼接。
4. 启动容器改为数据库 readiness/schema verification，不再拥有 DDL 权限。

### Phase 3：Operator 与镜像供应链

1. 在 Operator 中定义数据库配置校验、Secret 引用和 migration Job reconciler。
2. 加入 region/console Job 成功依赖与失败可观测性。
3. 把 DM driver bundle 纳入内部可追溯构建输入，Go/Console 镜像一次构建同时具备 MySQL 和 DM 驱动。

### Phase 4：双数据库验收

1. 空 schema 安装：MySQL 和达梦分别验证两个 migration Job。
2. 升级：旧 MySQL 安装升级后数据、索引、种子和 API 行为保持一致；达梦执行版本升级。
3. 并发：多副本启动、重复 Job、事务回滚、批量 upsert。
4. E2E：登录、创建团队/应用/组件、环境变量、端口/域名、构建检测、部署、插件列表和删除恢复。

### 7.1 取舍与非目标

- 不是一次性承诺支持所有数据库。首期正式支持 MySQL 和达梦；SQLite/CockroachDB 保留现有开发/实验定位，必须通过能力矩阵才可宣称正式支持。
- 不采用“把达梦切 MySQL 兼容模式即可”的结论。兼容模式可降低部分语法差异，但不能替代驱动、DDL、事务、ORM backend、迁移和语义测试。
- 不对所有历史 SQL 做盲目格式替换。每条 SQL 必须有分类、测试和语义保持证明。
- 不在本分支直接改生产数据库或强制重置现有测试实例。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|---|---|---|
| Go 连接和启动迁移 | `db/mysql/mysql.go:58-350` | 当前连接、建表、AutoMigrate、补丁和种子耦合处 |
| Go provider 白名单 | `db/db.go:151-180` | 支持类型注册和 manager 创建入口 |
| Go DB 参数 | `config/configs/db_config.go:7-16` | 当前 MySQL 命名的配置字段 |
| 失败的 env 批量写入 | `db/mysql/dao/tenants.go:1148-1181` | 必须先定义唯一键和事务的代表性 upsert |
| Go MySQL 特定索引补丁 | `db/mysql/mysql.go:412-446` | `GROUP_CONCAT`、`information_schema` 和 DDL 方言边界 |
| Console DB settings | `goodrain_web/settings.py:91-123` | 当前仅 MySQL/SQLite 配置 |
| Console 入口脚本 | `entrypoint.sh:8-88` | 当前运行时 makemigrations 问题 |
| Console schema 修复 | `console/management/commands/repair_legacy_schema.py:1-122` | 现有历史 schema 兼容逻辑 |
| Console 原生查询入口 | `console/repositories/base.py:16-19` | 游标结果字段名标准化位置 |
| Console 聚合 SQL | `console/services/service_services.py:266` | MySQL 特定聚合的代表性调用 |
| DM Go 官方文档 | `https://eco.dameng.com/document/dm/zh-cn/start/GO_DM_NEW.html` | 官方驱动来源与 DSN 形态 |
| DM Python 官方文档 | `https://eco.dameng.com/document/dm/zh-cn/start/python-development.html` | dmPython 编译与 DPI 运行时要求 |
