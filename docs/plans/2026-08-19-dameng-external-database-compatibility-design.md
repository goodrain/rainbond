# Rainbond 外置达梦数据库兼容性设计

## 一、项目背景

### 1.1 项目架构

Rainbond 的控制面数据路径为 `rainbond-ui -> rainbond-console -> rbd-api / rbd-worker / rbd-chaos -> Kubernetes`。Console 与 Region 使用同一数据库实例中的不同逻辑 Schema；默认数据库仍为 MySQL。

### 1.2 现有基础

- 标准 Core 镜像已编入官方达梦 Go 驱动，标准 Console 镜像已编入 `dmPython`、`dmDjango` 与 DPI 运行库。
- `DB_TYPE=dm` 已能选择达梦连接驱动；MySQL 仍是默认路径。
- 真实测试已证明网络、认证和基础连接可用，但暴露三类应用兼容问题：并发 GORM DDL、DM 返回的大写列标签，以及遗留的 MySQL 原生 SQL。

### 1.3 核心需求

1. 以外置、已创建 `REGION` 和 `CONSOLE` Schema 的 DM8 为目标，使 Core、Console 可稳定启动并完成基础平台流程。
2. 不依赖“达梦兼容 MySQL 协议”；驱动、DDL 和 SQL 按达梦语义处理。
3. `DB_TYPE=mysql` 或未配置该变量时，镜像、连接方式、初始化和查询结果保持不变。
4. 本轮为临时直配测试，不修改 `rainbond-operator`、CRD 或 `rainbond-ui`。

## 二、用户旅程

### 2.1 用户操作流程

1. 管理员将 Core 的 API、Worker、Chaos 及 Console 指向同一达梦实例中各自的 Region/Console Schema，并设置 `DB_TYPE=dm`。
2. 管理员执行一次明确、可重跑的 Schema 初始化；控制面服务随后只验证 Schema，不在多个 Pod 启动时竞争执行 DDL。
3. 管理员确认四个控制面 Pod 就绪后，普通用户继续使用既有 UI 创建 nginx 组件、等待检测完成、查看状态和删除组件。
4. 若 Schema 或查询不兼容，服务以安全且可诊断的错误失败；前端不得以无限 `Checking` 掩盖后端错误。

### 2.2 页面原型

- 不新增页面、弹窗或配置项。
- 既有“创建组件检测”“应用状态”“插件”“删除组件”页面是验收入口。
- React 层不访问数据库，因此不修改 `rainbond-ui`。

### 2.3 外部系统交互

- 外部系统为 DM8 或 MySQL 数据库及 Kubernetes。
- 本轮不增加 webhook、回调、Operator 资源或部署控制器。
- 数据库凭据只由部署环境/Secret 提供；不得写入代码、测试、日志或本文档。

## 三、整体架构设计

### 3.1 数据库能力边界

```text
DB_TYPE=mysql  -> 保留现有连接、DDL 与 SQL 路径
DB_TYPE=dm     -> 驱动 + DM Schema 初始化/验证 + DM SQL 能力层
```

达梦不是 MySQL 协议服务。`COMPATIBLE_MODE` 只能降低部分 SQL 差异，不能代替驱动、参数绑定、DDL 类型映射或返回列标签规范化。

### 3.2 Schema 所有权

```text
显式一次性初始化命令  -->  REGION Schema
                               |
                               +--> API / Worker / Chaos：连接并验证，不竞争 DDL

Django 初始化命令      -->  CONSOLE Schema
                               |
                               +--> Console：连接并验证
```

Core 的初始化与服务启动必须可区分；达梦服务进程不允许在并发启动时自行 `CreateTable` 或 `AutoMigrate`。Console 不再在每次容器启动时生成迁移文件；达梦初始化使用受版本控制的命令路径。

## 四、数据模型设计

### 4.1 既有业务模型

不新增业务表。Region 与 Console 分别创建当前版本的既有表、索引和必要种子数据。两者不共享业务表。

### 4.2 兼容规则

| 类别 | MySQL 行为 | 达梦行为 |
| --- | --- | --- |
| 文本/BLOB/超高精度 numeric | MySQL 类型 | 映射为 CLOB、BLOB 与达梦支持的 DECIMAL |
| 表选项、索引元数据 | `ENGINE`、`information_schema` | 不发送 MySQL 表选项；使用达梦安全验证 |
| 列标签 | 通常小写 | 查询结果统一转为调用方既有的小写键 |
| 字符串聚合 | `GROUP_CONCAT` | 方言帮助函数返回达梦等价表达式 |
| SQL 值 | 历史上有格式化插值 | 全部保留值参数绑定，禁止把值作为双引号标识符 |

### 4.3 大小写

达梦 Schema 名按驱动规则规范化。业务代码继续使用既有小写字段键；底层查询结果在进入 `addict.Dict` 前规范化，防止缺失小写属性时被自动创建为不可哈希的 `Dict`。

## 五、API 设计

### 5.1 对外接口

不新增 Console、Region 或 UI HTTP API；返回结构保持不变。

### 5.2 运维命令

新增/完善仅供部署使用的显式 Schema 初始化与验证入口。它接收既有数据库配置，不打印连接串或密码；成功后退出，失败时返回表/步骤级安全错误。正常服务启动不执行达梦 DDL。

## 六、核心实现设计

### 6.1 Core（rainbond）

1. 拆开打开连接、初始化 Schema 与验证 Schema；MySQL 启动路径维持现状。
2. 达梦初始化在一个进程中顺序执行模型 DDL、索引与种子数据，重跑必须幂等；达梦服务模式只验证全部必要表。
3. 集中覆盖所有 GORM 显式 MySQL 类型映射，避免 `longtext`、不支持精度和 MySQL 表选项泄入达梦。
4. 审计所有 Core 原生 SQL；只对真实不兼容的聚合、分页、元数据查询增加 DM 分支，DAO 对上层接口不变。

### 6.2 Console（rainbond-console）

1. 保留 MySQL Django 设置；DM 使用已打包的 `dmDjango` 后端及独立 Schema 配置。
2. 两个 `BaseConnection` 统一归一化达梦返回列标签并保持 MySQL 行为不变。
3. 建立小型数据库能力模块，收敛 vendor 判断、聚合、分页和安全参数绑定。
4. 全量审计 `cursor.execute`、`.raw()` 和 `BaseConnection` 调用；优先 ORM，保留原生 SQL 时分别覆盖 MySQL/DM。
5. 达梦容器启动不执行运行时 `makemigrations`，改用显式初始化/迁移和就绪验证。

### 6.3 非涉及层

- Operator：不涉及；当前测试继续通过临时 RbdComponent/部署环境变量配置。
- rainbond-ui：不涉及；只做端到端验收。
- Plugin 前后端：不涉及；KubeBlocks 等插件 DNS/生命周期问题独立处理。

## 七、实施计划

### 跨层覆盖检查

- [x] Go（rainbond）：需要 — Schema 初始化/验证、类型映射、种子及原生 SQL。
- [x] Python（rainbond-console）：需要 — 初始化、结果键、原生 SQL 与启动脚本。
- [ ] React（rainbond-ui）：不涉及 — 仅作为验收入口。
- [ ] Operator：不涉及 — 本轮不改 CRD 或调谐逻辑。
- [ ] Plugin：不涉及。

### Sprint 1：失败可复现的测试与 Schema 边界

- 为 Core 写 Schema 模式、DM 结果和 MySQL 回归测试；实现显式初始化与服务验证边界。
- 为 Console 写大写列标签、聚合和参数绑定回归测试；实现最小能力帮助模块。

### Sprint 2：全量 SQL 审计

- 对 Core DAO 及 Console 的 `cursor.execute`、`.raw()`、BaseConnection 消费者逐条归类为 ORM、通用 SQL 或 DM 分支。
- 将值插值改为参数绑定；把 MySQL 专属聚合、转换、排序规则和元数据查询置于能力层。

### Sprint 3：双库验证与一次构建

- 在全新 DM Schema 与未改配置的 MySQL 基线运行迁移/启动/查询回归。
- 完成 API、Worker、Chaos 与 Console 的统一构建前，只提交源代码和可重复测试；验证通过后再触发一次镜像构建。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
| --- | --- | --- |
| Core 连接与建表 | `db/mysql/mysql.go` | 当前连接、模型注册、DDL 和种子入口 |
| Core 启动循环 | `db/db.go` | 需要区分初始化与服务验证 |
| 达梦 GORM 补丁 | `scripts/prepare-dameng-go-driver.sh` | 仅编译期修改官方驱动，不是业务 SQL 兼容层 |
| Console 数据库设置 | `goodrain_web/settings.py` | `DB_TYPE` 及 Schema 配置 |
| Console 结果包装 | `console/repositories/base.py`、`www/db/base.py` | 达梦列标签兼容边界 |
| Console 组件状态查询 | `console/repositories/app.py` | 已复现大写字段导致的状态页异常 |
| Console 启动流程 | `entrypoint.sh`、`scripts/database_state.py` | 初始化与就绪检查入口 |
