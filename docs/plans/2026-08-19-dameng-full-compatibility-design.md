# Rainbond 达梦数据库全链路兼容性设计文档

## 一、项目背景

### 1.1 项目架构

Rainbond 的控制面由以下链路组成：

```text
rainbond-ui -> rainbond-console -> rainbond API / worker / chaos -> Kubernetes
                      |                    |
                      +----- database -----+
```

`rainbond` 的 Region 数据与 `rainbond-console` 的 Console 数据使用同一数据库实例中的独立逻辑库（达梦中为 Schema）。配置应由 `RainbondCluster` 经 Operator 下发到受管组件，而不是依赖手工修改 Deployment。

### 1.2 现有基础

- 默认数据库仍是 MySQL；已有用户和镜像必须保持现有行为。
- Core 已能按 `DB_TYPE=dm` 选择达梦驱动，Console 已能选择 `dmDjango` 后端。
- 当前局部修复仅处理了部分 GORM 字段类型映射，尚不能作为发布版本。
- 在真实达梦实例上已经复现：网络和认证成功，但部分自动建表失败后进程仍继续运行，导致服务检测结果无法写入或读取，前端持续轮询“检测中”。

### 1.3 核心需求

1. 达梦模式必须能够完整初始化 Region 与 Console 的既有表结构，并清晰暴露迁移失败。
2. 组件创建、服务检测、应用状态查询、环境变量/域名配置等既有平台流程必须在达梦上可用。
3. `DB_TYPE=mysql` 与未设置 `DB_TYPE` 的行为、SQL 和镜像运行方式保持不变。
4. Operator 必须能从 `RainbondCluster` 声明式识别数据库类型并下发配置，避免手改受管 Deployment 后被回写。

## 二、用户旅程

### 2.1 用户操作流程

1. 平台管理员在 `RainbondCluster` 中声明外置数据库的类型、地址、端口、用户名、密码引用及 Region/Console 逻辑库名。
2. Operator 校验数据库连通性和类型，生成 Core、Worker、Chaos、Console 的一致配置并滚动更新。
3. 管理员查看所有控制面 Pod 就绪；若迁移失败，Pod 日志和 CR Status 显示明确原因，而不是前端无限等待。
4. 普通用户沿用现有 UI 创建组件；镜像检测结束后能够进入下一步、创建和删除组件、查看应用状态。

### 2.2 页面原型

- 不新增 UI 页面、弹窗或表单。
- 继续使用既有的创建组件检测页、应用状态页、环境变量页和域名页作为验收入口。
- 前端轮询接口返回的状态由后端保证终态；本次不以修改前端掩盖数据库错误。

### 2.3 外部系统交互

- 外部数据库：MySQL 或达梦 DM8。
- Kubernetes：Operator 更新 `RbdComponent`，组件滚动升级。
- 不新增 webhook、回调或通知协议。

## 三、整体架构设计

### 3.1 数据库方言边界

新增统一的数据库能力边界，而非将达梦伪装成 MySQL：

```text
RainbondCluster.database.type
          |
       Operator
          +-> Core: DB_TYPE + driver DSN + database capability
          +-> Console: DB_TYPE + Django ENGINE + database capability

Core capability: GORM 类型、建表、初始化 SQL、DAO 原生 SQL
Console capability: Django backend、迁移、Repository/Service 原生 SQL
```

所有差异仅在类型为 `dm` 时进入达梦分支；`mysql` 和缺省值继续执行原路径。

### 3.2 核心流程

1. Core 连接数据库后根据类型选择方言能力。
2. 达梦模式下，GORM 显式 MySQL 类型映射为达梦等价类型，再执行建表。
3. 任一达梦建表或必要初始化失败时记录表名、SQL 错误并让组件不可就绪；不得继续提供看似正常的 API。
4. 原生查询仅对不兼容的字符串聚合、索引检查等语法生成方言分支；达梦已支持的 `LIMIT offset,count` 与 `CONCAT` 保持原样。
5. Console 复用同一原则：Django ORM 保持原样，原生 SQL 按后端生成等价且参数化的语句。

## 四、数据模型设计

### 4.1 新增数据库表

不新增业务表。必须完整创建既有 Region 与 Console 的全部表及索引。

### 4.2 数据关系

- Region Schema：`key_value`、服务检测记录、租户服务及其他 Core 模型表。
- Console Schema：Django migration 表和现有 Console 模型表。
- 两者不共享业务表；仅共享同一数据库实例和认证配置。

### 4.3 类型与 DDL 映射

| MySQL 定义/语法 | 达梦等价 | 处理位置 |
| --- | --- | --- |
| `tinytext` / `text` / `mediumtext` / `longtext` | `CLOB` | Core GORM 数据类型归一化 |
| `tinyblob` / `blob` / `mediumblob` / `longblob` | `BLOB` | Core GORM 数据类型归一化 |
| `decimal(p,s)` 且 `p > 38` | 截断至达梦支持的精度 | Core GORM 数据类型归一化 |
| `ENGINE=InnoDB` / MySQL 字符集表选项 | 无 | 达梦建表路径不下发该选项 |
| `LIMIT offset,count` | MySQL 与达梦均支持 | 保留现有语句，不为方言适配而改写 |
| `GROUP_CONCAT` | `LISTAGG` / `WM_CONCAT` | Console 查询帮助函数 |
| `CONCAT` | MySQL 与达梦均支持；`||` 也可用 | 保留现有语句，按实际查询验证 |

## 五、API 设计

### 5.1 接口列表

不新增面向终端用户的 HTTP API。新增/补全声明式配置能力：

| 配置接口 | 变更 |
| --- | --- |
| `RainbondCluster.spec.database` | 增加并校验 `type: mysql | dm`；保留现有字段语义 |
| Operator -> `RbdComponent` | 从 Cluster 生成 DB 环境变量和连接参数，配置变更触发受管组件更新 |
| `RainbondCluster.status.conditions` | 数据库预检、迁移或配置失败显示可操作错误原因 |

密码只通过现有 Secret/引用下发；日志、CR Status、测试输出和文档均不得包含凭据。

### 5.2 请求/响应结构

现有 Console/Region HTTP 响应格式不变。创建检测接口在数据库错误时返回明确的失败终态或可诊断错误，禁止以空结果永久返回 `Checking`。

## 六、核心实现设计

### 6.1 Core（rainbond）

1. 完整枚举 `db/model` 显式类型，集中完成 MySQL 类型到达梦类型的映射，并以测试覆盖每一种映射。
2. 将达梦迁移错误变为启动/就绪错误，错误信息包含模型表名但不包含连接凭据。
3. 将 `db/mysql/mysql.go` 中的 MySQL 专属建表选项、索引修复、语言版本初始化拆分为 MySQL 和达梦等价实现；不再以“跳过初始化”代替达梦支持。
4. 为 `db/mysql/dao` 中的分页和聚合原生 SQL 提供数据库能力分支；DAO API 对上层保持不变。
5. 移除或方言化 MySQL 专属的独立迁移脚本，保证同一安装路径可用于两种数据库。

### 6.2 Console（rainbond-console）

1. 保持 MySQL 的 Django `ENGINE`、选项和连接语义不变；仅 `DB_TYPE=dm` 使用达梦后端和 Schema 选项。
2. 建立数据库能力帮助函数，仅替换 Repository、Service、View 中经真实达梦验证不兼容的字符串聚合等 SQL。
3. 逐个审核所有 `cursor.execute`、`.raw()` 与底层连接调用：可改 ORM 的改 ORM；必须保留的使用参数绑定与方言分支。
4. 处理达梦驱动异常的字符串化和日志记录，避免异常对象再次导致 JSON 序列化错误。
5. 审核默认 Region 与升级脚本，消除绕过 Django 数据库配置而硬编码 MySQL 驱动的路径。

### 6.3 Operator（rainbond-operator）

1. CRD 类型、默认值和预检逻辑识别 MySQL/达梦。
2. 预检按数据库类型加载驱动及连接语法；不以 MySQL 驱动探测达梦端口。
3. 监听 `RainbondCluster` 的数据库配置变更并使关联 `RbdComponent` 重算，避免旧 `spec.args` 覆盖新连接。
4. MySQL CR 兼容旧字段和缺省行为；达梦仅在显式声明时启用。

### 6.4 前端与插件

- React（rainbond-ui）：不需要源码修改；现有创建检测页用于端到端验收。
- Plugin frontend：不涉及。
- Plugin backend：不涉及；KubeBlocks 适配器的 DNS 缺失是独立插件部署问题，不作为数据库兼容性的一部分。

## 七、实施计划

### 跨层覆盖检查

- [x] Go（rainbond）：需要 — GORM 类型、迁移失败语义、初始化 SQL、DAO SQL。
- [x] Python（rainbond-console）：需要 — 原生 SQL 方言、初始化脚本、异常边界。
- [ ] React（rainbond-ui）：不涉及 — 无新增交互，使用现有页面验收。
- [ ] Plugin frontend/backend：不涉及 — 无数据库接口变更。
- [x] Operator：需要 — Cluster 声明、预检、受管组件重调谐。

### Sprint 1：可观测的 Schema 初始化

#### Task 1.1：完成 Core GORM 类型覆盖

- 仓库：rainbond
- 文件：`scripts/prepare-dameng-go-driver.sh`、`scripts/dameng_gorm_v1_compat.go`、`hack/contrib/docker/dameng_dockerfile_test.go`
- 实现内容：覆盖所有显式文本、二进制、数值类型及边界精度。
- 验收标准：单元测试对 MySQL 输入不变、达梦输出可执行；完整 Region 模型建表无失败。

#### Task 1.2：迁移失败不可伪装为正常

- 仓库：rainbond
- 文件：`db/mysql/mysql.go` 及初始化调用方
- 实现内容：达梦迁移失败时返回带表名的错误并阻止就绪。
- 验收标准：缺表场景不会出现 API 正常但 `key_value` 不存在的状态。

### Sprint 2：Core 与 Console 原生 SQL

#### Task 2.1：Core 方言化 SQL 与初始化

- 仓库：rainbond
- 文件：`db/mysql/mysql.go`、`db/mysql/dao/tenants.go`、`db/mysql/dao/event.go`、`db/upload_sessions_migration.sql`
- 实现内容：实现达梦分页、索引、语言版本种子和 DDL 路径。
- 验收标准：MySQL SQL 快照不变；达梦执行无 MySQL 专属语法。

#### Task 2.2：Console 全量原生 SQL 清单与修复

- 仓库：rainbond-console
- 文件：`console/repositories/**/*.py`、`console/services/**/*.py`、`console/views/**/*.py`、`www/**/*.py`
- 实现内容：为每个原生 SQL 标记 ORM/通用 SQL/DM 分支，并替换不兼容语法。
- 验收标准：静态测试禁止未登记的 MySQL 专属 SQL；关键页面查询在两种数据库均通过。

### Sprint 3：声明式安装与端到端验收

#### Task 3.1：Operator 数据库类型支持

- 仓库：rainbond-operator
- 文件：CRD 类型、数据库预检、组件调谐控制器
- 实现内容：声明式识别数据库类型、按类型预检、配置变更传播。
- 验收标准：仅编辑 `RainbondCluster` 即更新受管组件；默认 MySQL 安装回归通过。

#### Task 3.2：双数据库验收矩阵

- 仓库：rainbond、rainbond-console、rainbond-operator
- 实现内容：单元/契约测试、真实达梦实例建表和平台创建组件烟测。
- 验收标准：两种数据库均完成启动、迁移、创建 nginx 组件、状态查询、删除组件；无无限 `Checking`。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
| --- | --- | --- |
| Core 数据库初始化 | `rainbond/db/mysql/mysql.go:269` | MySQL 表选项、建表与索引修复路径 |
| Core DAO 分页 | `rainbond/db/mysql/dao/tenants.go:365` | `LIMIT ?,?` 需方言化 |
| Core 服务检测状态 | `rainbond/api/handler/service_check.go` | 空检测结果会保持 Checking 的调用路径 |
| Core 长文本模型 | `rainbond/db/model/key_value.go:6` | `longtext` 导致关键状态表建表失败的代表 |
| Console 达梦设置 | `rainbond-console/goodrain_web/settings.py` | Django 后端与 Schema 选择 |
| Console 聚合查询 | `rainbond-console/console/services/service_services.py:266` | `GROUP_CONCAT` 差异 |
| Console 分页查询 | `rainbond-console/console/views/app_config/app_env.py:100` | 已验证达梦兼容，保留原语句 |
| Operator 数据库配置 | `rainbond-operator/pkg/apis/rainbond/v1alpha1/*` | CRD 字段和调谐入口 |
