# Rainbond 数据库可移植性整理设计

## 一、项目背景

### 1.1 项目架构

`rainbond` 使用 Go + GORM v1 访问 region 数据，`rainbond-console` 使用 Django ORM 访问 console 数据。两者当前主要按 MySQL 编写，达梦是第一个需要兼容的非 MySQL 数据库。

### 1.2 现有基础

- Go 模型中存在 `longtext` 等 MySQL 字段类型。
- Go DAO 有 33 处依赖 MySQL `ON DUPLICATE KEY UPDATE` 的批量 upsert。
- Go 和 Console 都存在 `GROUP_CONCAT`、`LIMIT offset,count`、`CAST AS SIGNED` 等数据库特有 SQL。
- 直接把每次达梦报错改成 `if db == dm` 会把数据库差异扩散到业务代码，也无法帮助后续适配其他数据库。

### 1.3 核心需求

本阶段整理运行时代码和数据库接入边界的可移植性，不设计迁移执行器，不修改 Operator，也不改变部署流程：

1. 模型优先使用 MySQL 与达梦都支持、且容量满足业务需求的公共字段类型。
2. 查询优先使用 ORM，其次使用参数化的 ANSI SQL。
3. 只有确实无法统一的 upsert、聚合、分页等能力进入一个窄方言边界。
4. 业务 DAO 不出现散落的 `if dbType == "dm"`。
5. MySQL 的接口行为、事务归属和已有优化路径保持不变。
6. 驱动、DSN、Schema 元数据和 DDL 差异集中到数据库后端适配器；新增数据库时不修改业务 DAO。

## 二、用户旅程

### 2.1 用户操作流程

- 管理员仍通过现有配置选择数据库和连接信息。
- 平台用户仍按原流程创建应用、组件、环境变量、端口、域名并执行构建部署。
- 数据库差异由后端内部消化，不增加用户配置项和操作步骤。

### 2.2 页面原型

不涉及新页面、弹窗或表单。

### 2.3 外部系统交互

- MySQL 是默认数据库，必须通过原有测试和构建。
- 达梦通过其原生驱动工作，但业务层不直接依赖达梦 SQL。
- 本阶段不涉及 webhook、回调、通知或 Operator 编排。

## 三、整体架构设计

### 3.1 系统架构图

```text
handler/service
      |
      v
DAO / Django ORM  --------> ORM 或参数化 ANSI SQL
      |
      v
database portability boundary
  - connection/backend registry
  - schema metadata and DDL policy
  - bulk upsert
  - aggregate expression
  - pagination fallback
  - unavoidable metadata operations
      |
      +------ MySQL implementation
      +------ Dameng implementation
      +------ additional database implementation
```

### 3.2 核心流程

1. 静态盘点并禁止业务模型继续新增厂商字段类型。
2. 逐类替换字段和 SQL，而不是按运行时错误逐接口修补。
3. 可用 ORM 表达的查询直接交给 ORM 生成分页、绑定参数和标识符。
4. 无法统一的语义通过集中接口实现，DAO 只声明业务语义和冲突键。
5. 每类改动同时验证 MySQL 旧行为和可移植实现。

## 四、数据模型设计

### 4.1 新增数据库表

不新增数据库表，不处理 schema 迁移和初始化流程。

### 4.2 字段规则

- 短字符串必须声明合理的 `size`，避免 ORM 默认生成无界大文本。
- 文本字段统一使用 ORM 可映射的 `TEXT`；当前业务字段不声明厂商专属大文本类型。
- 业务模型禁止出现 `LONGTEXT`、`MEDIUMTEXT`、`CLOB` 等单一数据库类型。
- 如果后续出现不存在公共物理类型的字段，只允许在集中方言层映射，不在模型中增加厂商判断。

## 五、API 设计

### 5.1 接口列表

不新增或修改 HTTP API。内部新增窄能力接口：

- `BulkUpsert(db, rows, batchSize, conflictColumns...)`：调用方明确冲突键。
- `DatabaseBackend`：集中声明连接、默认 Schema 行为、表元数据查询和数据库专属补丁。
- 聚合和分页优先由 ORM 表达；只有无法表达时才增加方言方法。

### 5.2 请求/响应结构

Console、UI 与 region API 的请求和响应保持不变。

## 六、核心实现设计

### 6.1 关键逻辑

#### 6.1.1 字段类型

先替换显式 MySQL 类型，并增加静态回归测试。字段容量必须根据业务语义选择，不能为了语法统一把所有大文本降为 64 KiB。

#### 6.1.2 查询

- `LIMIT offset,count` 改为 ORM 的 `Limit/Offset` 或 ANSI `LIMIT count OFFSET offset`。
- `GROUP_CONCAT` 若只是为了组装列表，改为普通行查询后在应用层聚合。
- `CAST AS SIGNED` 改为一致的数据模型比较或应用层转换。
- 字符串格式化 SQL 改为参数绑定。
- 用户可见的模糊搜索显式使用 ORM 的不区分大小写查询，不依赖 MySQL 默认排序规则。
- 返回列名在 repository 边界统一为逻辑小写，避免驱动大小写差异泄漏。

#### 6.1.3 Upsert 与事务

- DAO 显式传入冲突列，不通过反射猜测业务唯一键。
- MySQL 仍可在集中实现中使用现有批量快速路径。
- 其他数据库使用对应的原生 merge/upsert，或在调用方已有事务内执行可移植的 update-then-insert。
- upsert 工具不得自行开启嵌套事务；事务始终由现有 service/handler 调用方拥有。
- 传入值统一转换为可寻址指针，防止再次出现 `using unaddressable value`。

#### 6.1.4 数据库后端边界

- 通用 Manager 只负责注册模型、调用后端能力和暴露 DAO，不直接判断 `dm`。
- 每种数据库在一个后端实现中处理驱动可用性、连接 DSN、默认 Schema 策略、表存在性检查和专属 DDL。
- MySQL 专属历史 DDL 只由 MySQL 后端调用，不能成为其他数据库的默认分支。
- 达梦专属目录、驱动构建和系统目录查询保留在达梦后端；这些是必要的驱动差异，不进入 DAO/Service/View。
- 新数据库的目标接入成本是“驱动 + 一个后端适配器 + 兼容性验证”，不承诺仅复制驱动文件即可支持所有 Schema 行为。

### 6.2 复用现有代码

- 保留 GORM v1、现有 DAO 接口和 MySQL bulk-upsert 依赖。
- 保留 Django ORM 和现有服务接口。
- 不升级 ORM，不重写数据库架构，不引入通用数据库框架。

## 七、实施计划

### 跨层覆盖检查

- [x] Go (rainbond): 需要 — 字段类型、查询、统一 upsert 边界和测试。
- [x] Python (rainbond-console): 需要 — ORM/参数化查询、聚合、分页、结果字段标准化和测试。
- [x] React (rainbond-ui): 不涉及 — API 契约和页面不变。
- [x] Plugin frontend (enterprise-base): 不涉及。
- [x] Plugin backend (plugin-template): 不涉及。
- [x] Operator: 不涉及 — 本阶段明确不处理迁移和部署编排。

### Sprint 1：Go 模型与查询

1. 建立模型厂商类型静态检查。
2. 将显式 `longtext` 改为满足容量要求的公共类型。
3. 将 DAO 中可直接替换的分页和原生 SQL 改为 ORM/ANSI SQL。

### Sprint 2：Go upsert

1. 建立统一 bulk-upsert 接口和 MySQL 特征测试。
2. 为每个调用点声明冲突列。
3. 覆盖外层事务、重复写入、零值、指针和值切片。

### Sprint 3：Console 查询

1. 将应用层可完成的 `GROUP_CONCAT` 改为 ORM 查询后聚合。
2. 将分页、类型转换和字符串拼接 SQL 改为 ORM/参数化写法。
3. 统一 cursor 列名，并将静态检查覆盖 `console`、`www`、`openapi` 三个运行时入口。
4. 执行 Console 回归测试，确认返回结构和 MySQL 搜索语义保持不变。

### Sprint 4：验证

1. Go：相关测试、`go test ./...`、`go build ./...`、`go vet ./...`、manifest 校验。
2. Console：相关测试、项目检查和 manifest 校验。
3. MySQL E2E 作为合并门禁；达梦 E2E 用于发现尚未进入集中边界的差异。

### Sprint 5：数据库后端边界收敛

1. 用注册表和后端接口替换通用 Manager 中散落的数据库类型判断。
2. 将 MySQL 历史 DDL、达梦 Schema 元数据查询和默认迁移策略归入各自后端。
3. 删除未被生产代码调用的达梦分页分支和旧 bulk-upsert 达梦补丁。
4. 增加静态测试，禁止 DAO、Handler、Service 和 View 新增达梦类型判断。
5. 保持 Console 的数据库判断仅存在于 Django `DATABASES` 驱动配置边界。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|---|---|---|
| Go 模型 | `db/model/*.go` | 清理厂商字段类型 |
| Go 查询 | `db/mysql/dao/*.go` | ORM/ANSI SQL 整理 |
| Go upsert | `db/mysql/dao/*.go` | 33 个调用点收口 |
| Go 后端适配器 | `db/mysql/backend*.go` | 连接、Schema 元数据和专属 DDL 边界 |
| Go 通用 Manager | `db/mysql/mysql.go` | 注册模型并调用后端能力，不判断具体数据库 |
| Console repository | `console/repositories/*.py` | 查询与结果标准化 |
| Console service/view | `console/services/*.py`, `console/views/*.py` | 原生 SQL 收口 |
| Console OpenAPI | `openapi/**/*.py` | OpenAPI 查询分页收口 |
