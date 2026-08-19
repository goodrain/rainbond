# Rainbond 达梦完整平台适配设计

## 一、项目背景

### 1.1 项目架构

Rainbond 的控制面路径为：

```text
rainbond-ui -> rainbond-console -> rbd-api / rbd-worker / rbd-chaos -> Kubernetes
                     |                  |
                     +---- Console ------+---- Region
```

Console 和 Region 使用同一数据库实例中的不同逻辑 Schema。默认实现是 MySQL；达梦支持必须由 `RainbondCluster` 声明并由 Operator 下发，不能依赖手工修改 `RbdComponent` 或 Deployment。

### 1.2 现有基础

- 标准 Core 镜像已包含官方 DM Go 驱动，Console 镜像已包含 `dmPython`、`dmDjango` 和 DPI 运行库。
- 在测试环境中，Console 已可通过 `dmDjango` 连接达梦，Django migration 已完成。
- Core 可连接达梦，但 API、Worker、Chaos 会同时执行 GORM 自动建表；达梦遇到并发 `CREATE TABLE` 后返回“对象已存在”，服务反复初始化而无法就绪。
- Console 和 Core 仍保留 MySQL 专属初始化及原生 SQL。禁止通过跳过初始化或让 Pod 假就绪来掩盖这些差异。

### 1.3 核心需求

1. 新建的达梦 Region 与 Console Schema 能完整初始化，API、Worker、Chaos、Console 都能就绪。
2. 用户可以完成创建 nginx 组件、检测结束、查看状态、删除组件等平台基础流程。
3. `DB_TYPE=mysql` 和未设置数据库类型时的现有安装、配置和 SQL 行为不变。
4. 数据库类型、连接配置和迁移状态都由 `RainbondCluster`/Operator 管理；凭据不出现在日志、文档或状态信息中。

## 二、用户旅程

### 2.1 用户操作流程

1. 管理员在 `RainbondCluster` 为 Region 和 Console 分别声明数据库类型、地址、端口、用户、密码和 Schema 名称。
2. Operator 先校验数据库类型与网络可达性，创建唯一的 Region Schema 迁移 Job。
3. Region 迁移 Job 成功后，API、Worker、Chaos 只校验 Schema，不再执行建表；Console 使用同一声明完成 Django migration。
4. 管理员查看 `RainbondCluster`、`RbdComponent` 和迁移 Job 的状态；失败时得到表名或 SQL 类别，不得到凭据。
5. 普通用户从现有 UI 创建组件；检测接口必须得到成功或失败终态，不得无限停留在“检测中”。

### 2.2 页面原型

- 不新增前端页面。
- 现有“创建组件检测”“应用状态”“插件”“删除组件”页面是端到端验收入口。
- React（rainbond-ui）不需要功能改动；不得用前端超时掩盖后端数据库失败。

### 2.3 外部系统交互

- 外部数据库：MySQL 或 DM8。
- Kubernetes：Operator 创建/监控 Schema Job，并更新受管组件。
- 无新增 webhook、回调或通知协议。

## 三、整体架构设计

### 3.1 声明式数据库配置

```yaml
spec:
  regionDatabase:
    type: dm             # 缺省为 mysql
    host: <database-host>
    port: 5236
    username: <user>
    password: <secret-value>
    name: REGION
  uiDatabase:
    type: dm
    host: <database-host>
    port: 5236
    username: <user>
    password: <secret-value>
    name: CONSOLE
```

Operator 对 MySQL 保持已有 `--mysql` 参数和 `MYSQL_*` 环境变量。对 DM，Core 仍接收兼容的连接参数，但同时获得 `DB_TYPE=dm`；Console 获得 `DB_TYPE=dm` 与通用 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`。所有组件的配置从 Cluster 重算，手工 RbdComponent 覆盖不作为产品配置路径。

### 3.2 单一 Region Schema 迁移者

```text
RainbondCluster (type=dm)
       |
       v
Operator --> rbd-region-db-migrate Job --> REGION Schema
                                              |
                                              v
                             rbd-api / rbd-worker / rbd-chaos (verify only)
```

- Job 使用带 DM 驱动的 Core 镜像，执行一次连接、建表、索引和种子数据初始化。
- Job 的配置/镜像摘要改变时创建新的受控 Job；失败状态保留在 Job 和 RbdComponent 条件中。
- DM 服务进程只执行 Schema 验证，不调用 `CreateTable` 或 `AutoMigrate`，从根源上消除并发 DDL。
- MySQL 不启用该模式，继续现有启动路径，防止改变存量安装行为。

### 3.3 数据库能力边界

```text
Core: 连接/Schema/种子/DAO SQL
Console: Django backend/migration/Repository SQL
Operator: 类型校验/配置渲染/迁移 Job 生命周期
```

达梦分支只在显式 `type=dm` 时运行。MySQL SQL、参数名和默认值必须保持原样。官方驱动解决连接和 ORM 方言注册，不自动解决应用层 MySQL SQL。

## 四、数据模型设计

### 4.1 新增控制面配置

- `api/v1alpha1.Database.Type`：`mysql` 或 `dm`，缺省 `mysql`。
- 不新增业务表；Region Job 必须完整创建既有 Region 模型表、索引及语言版本种子数据。
- Console 继续由 Django migration 创建既有表；`CONSOLE` 是 DM Schema，不是假设为 MySQL database。

### 4.2 数据关系与大小写

- Region 和 Console 不共享业务表。
- DM Schema 名按连接驱动要求规范化为大写；表名维持 Rainbond 现有小写模型名，所有原生 SQL 必须与实际引用方式一致。
- 验收用全新测试 Schema/实例，禁止用已被失败迁移污染的表结构判断兼容性。

### 4.3 DDL 与 SQL 兼容策略

| 类别 | MySQL | DM 处理 |
| --- | --- | --- |
| GORM 文本/BLOB/高精度 decimal | MySQL 类型 | 在 DM 方言中映射为 CLOB、BLOB、DM 支持的 decimal |
| 初始化表选项 | `ENGINE`、charset | DM 不发送 MySQL 表选项 |
| 初始化索引与语言版本 | `information_schema`、bulk upsert | 写 DM 等价查询/逐条幂等写入，不跳过 |
| Console 聚合 | `GROUP_CONCAT` | 使用经真实 DM 验证的等价聚合并保持返回 JSON 语义 |
| 字符串条件 | 双引号字符串可在 MySQL 运行 | 全部参数绑定或使用单引号；DM 不接受把字符串当标识符 |
| 分页、转换、DDL | 当前原生 SQL | 逐条以真实 DM 集成测试决定保留或方言化，不凭假设放行 |

## 五、API 设计

### 5.1 对外接口

不新增 UI 或 Console HTTP API。既有 HTTP 请求/响应保持不变。

### 5.2 Operator 配置与状态接口

| 接口 | 行为 |
| --- | --- |
| `RainbondCluster.spec.regionDatabase.type` | 缺省 mysql，显式 dm 启用 DM 配置路径 |
| `RainbondCluster.spec.uiDatabase.type` | 同上，独立指定 Console Schema |
| Region Schema Job | Job 成功才允许 DM Region 组件最终就绪；失败事件不暴露密码 |
| `RbdComponent.status.conditions` | 迁移失败显示 `DatabaseMigrationFailed` 类别和安全错误摘要 |

## 六、核心实现设计

### 6.1 Operator（rainbond-operator）

1. 扩展 `Database` 类型并生成 CRD、deepcopy、测试。
2. `RegionDataSource` 保留旧参数兼容，但按 `Type` 生成组件 `DB_TYPE`；App UI 同时获得通用 DB 环境变量。
3. MySQL 预检保留驱动 Ping；DM 预检至少验证类型、端口可达性，最终认证/DDL 成功由 Region Job 作为权威结果。
4. 为 DM Region 创建受控迁移 Job，Worker/Chaos/API 在 Job 未成功时只重试 Schema 验证，不进行 DDL。

### 6.2 Core（rainbond）

1. 将“打开连接”“迁移 Schema”“验证 Schema”拆开；新迁移命令以失败退出，服务进程在 DM verify 模式不写 DDL。
2. 迁移命令完整执行模型建表、索引修复和语言版本种子；删除当前 `skipsMySQLBootstrap` 的“跳过”策略，改为 DM 等价实现。
3. 所有 Region 模型在真实空 DM Schema 上一次性建表；迁移可重跑且不产生对象已存在错误。
4. 审核 `db/mysql/dao` 的原生 SQL，优先参数化与 GORM；无法通用的查询增加 DM 分支和双库测试。

### 6.3 Console（rainbond-console）

1. 保持已验证的 `dmDjango`、DPI 运行库与 DM Schema 配置。
2. 建立小型数据库能力帮助层；将 `GROUP_CONCAT`、MySQL COLLATE、双引号插值 SQL 逐条改为参数化/DM 分支。
3. 所有 `cursor.execute`、`.raw()` 和 `BaseConnection` 调用建立清单并由测试保护，尤其是插件、应用市场、团队权限和应用状态路径。
4. 维持 MySQL Engine、选项和结果 JSON 格式不变。

### 6.4 前端与插件

- React（rainbond-ui）：不涉及数据库配置代码；使用现有页面做验收。
- Plugin 前后端：不涉及数据库适配。KubeBlocks DNS 缺失独立处理，不混入本次验收。

## 七、实施计划

### 跨层覆盖检查

- [x] Go（rainbond）：需要 — Schema 命令、DM verify 模式、DDL/种子/DAO 兼容。
- [x] Python（rainbond-console）：需要 — 原生 SQL 方言和完整 migration 启动验证。
- [ ] React（rainbond-ui）：不涉及 — 仅作为端到端验收入口。
- [x] Operator：需要 — CRD 类型、配置下发、预检、Region Schema Job。
- [ ] Plugin：不涉及 — 不改插件 API 或 UI。

### Sprint 1：配置和迁移所有权

#### Task 1.1：声明式数据库类型

- 仓库：`rainbond-operator`
- 文件：`api/v1alpha1/rainbondcluster_types.go:90-97`、`controllers/handler/common.go:88-111`、`controllers/handler/api.go:109-170`、`controllers/handler/worker.go:89-164`、`controllers/handler/chaos.go`、`controllers/handler/app-ui.go:130-244`
- 实现：新增类型、默认 MySQL、DM 环境变量及配置渲染。
- 验收：同一 Cluster 配置能渲染 API/Worker/Chaos/AppUI；MySQL 快照不变。

#### Task 1.2：Region Schema Job

- 仓库：`rainbond-operator`、`rainbond`
- 文件：`controllers/rbdcomponent_controller.go:120-220`、`controllers/handler/api.go`、`cmd/db-migrate/`、`db/db.go:162-181`、`db/mysql/mysql.go:62-155`
- 实现：只在 DM 下创建并跟踪 Job；Core 迁移和服务 verify 模式拆分。
- 验收：空 Schema 同时启动 API/Worker/Chaos 时仅 Job 执行 DDL，三个服务最终就绪。

### Sprint 2：Region 完整 Schema 与数据路径

#### Task 2.1：DM 初始化完整性

- 仓库：`rainbond`
- 文件：`db/mysql/mysql.go:267-478`、`scripts/prepare-dameng-go-driver.sh`、`db/model/**/*.go`
- 实现：完整 DM 建表、索引和语言版本种子，删除跳过初始化的逻辑。
- 验收：全新 DM Schema 有全部模型表、必要索引、种子数据；重跑 Job 幂等。

#### Task 2.2：Core DAO 双方言

- 仓库：`rainbond`
- 文件：`db/mysql/dao/tenants.go:300-405`、`db/mysql/dao/event.go:230-260`、`db/upload_sessions_migration.sql`
- 实现：审计全部原生 SQL，给不兼容语法增加 DM 分支与参数化。
- 验收：创建、检测、查询状态、分页、删除组件在 MySQL 和 DM 都通过。

### Sprint 3：Console 原生 SQL

#### Task 3.1：Console 数据库能力层与 SQL 清单

- 仓库：`rainbond-console`
- 文件：`goodrain_web/settings.py:91-139`、`console/repositories/base.py`、`www/db/base.py`
- 实现：统一识别 MySQL/DM，保存受管原生 SQL 清单。
- 验收：启动、Django migration、默认 Region 初始化在两种数据库都通过。

#### Task 3.2：关键页面 SQL 适配

- 仓库：`rainbond-console`
- 文件：`console/services/service_services.py:259-273`、`console/repositories/app.py:182-235`、`console/repositories/perm_repo.py:286-310`、`console/services/app_config/plugin_service.py:20-55`、`console/services/market_app_service.py:1439-1470`
- 实现：替换 MySQL 聚合、COLLATE 和双引号拼接；保证返回字段不变。
- 验收：插件、应用市场、团队权限、应用状态路径在 DM 上无无效列名错误。

### Sprint 4：双库验收与交付

#### Task 4.1：真实数据库集成矩阵

- 仓库：`rainbond`、`rainbond-console`、`rainbond-operator`
- 实现：在独立空 DM 测试实例/Schema 和 MySQL 基线执行自动化集成验收。
- 验收：首次安装、滚动重启、Schema 重跑、创建 nginx、检测完成、状态查询、删除均通过。

#### Task 4.2：最终镜像和部署

- 仓库：三个后端仓库
- 实现：在所有验证通过后一次构建 Operator、Core、Console 镜像，更新 `RainbondCluster` 声明式配置。
- 验收：不手工编辑 RbdComponent/Deployment；所有受管 Pod Ready。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
| --- | --- | --- |
| Core 数据库启动 | `rainbond/db/db.go:162` | 当前无限重试入口，需拆分迁移/验证 |
| Core 自动建表 | `rainbond/db/mysql/mysql.go:267` | 当前并发 DDL 根因 |
| Core MySQL 初始化 | `rainbond/db/mysql/mysql.go:307` | 不能继续以 skip 代替 DM 支持 |
| Console DM 设置 | `rainbond-console/goodrain_web/settings.py:124` | 已验证驱动和 Schema 配置 |
| Console MySQL SQL | `rainbond-console/console/services/service_services.py:266` | 聚合与双引号问题代表 |
| Operator Database CRD | `rainbond-operator/api/v1alpha1/rainbondcluster_types.go:90` | 需加 type |
| Operator MySQL 固定预检 | `rainbond-operator/controllers/cluster-mgr/precheck/database.go:40` | 需按类型分支 |
| Operator 参数渲染 | `rainbond-operator/controllers/handler/api.go:109` | Region API 参数和环境变量入口 |
