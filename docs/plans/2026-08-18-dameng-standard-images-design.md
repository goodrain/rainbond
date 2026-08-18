# Rainbond 达梦标准镜像构建设计文档

## 一、项目背景

### 1.1 项目架构

Rainbond 的数据库运行时由三个镜像组成：`rbd-api`、`rbd-worker` 和
`rainbond`（Console）。核心服务以 Go/GORM v1 访问 `region` 库，Console 以
Django 访问 `console` 库。已有实现已在运行时识别 `DB_TYPE=dm`，但官方达梦
Go、dmPython 和 dmDjango 驱动均未进入普通发布镜像。

GitHub Actions 的实际构建入口如下：

```text
rainbond/.github/workflows/{dev-build,release-v6}.yml
  -> hack/contrib/docker/{api,worker,chaos}/Dockerfile
  -> checkout rainbond-console -> Dockerfile
```

### 1.2 现有基础

- Go 核心已经具备 `dm` build tag、官方 Go driver/GORM v1 dialect 的接入点。
- Console 已经具备 `DB_TYPE=dm`、dmDjango 配置和通用数据库就绪检查。
- 当前核心 Dockerfile 仅在 `ENABLE_DM=true` 时编译 DM 变体；当前 Console 的
  DM 安装步骤只在 `Dockerfile.dm`，而 Action 使用普通 `Dockerfile`。
- 达梦官方驱动随达梦安装包提供，不能假定存在可公开、稳定且可在 CI 中直接
  `go get` 或 `pip install` 的依赖地址。

### 1.3 核心需求

标准发布镜像必须同时具备 MySQL 与 DM 驱动。Action 构建标准镜像时自动注入受控
的达梦驱动输入；平台管理员部署时仅需配置数据库连接与 `DB_TYPE=dm`，不选择
特殊镜像、不设置 `ENABLE_DM`。

本期不修改 Operator、RainbondCluster CRD 或用户界面，不将官方驱动源码、二进制
或凭据提交到任一源码仓库。

## 二、用户旅程

### 2.1 用户操作流程

1. 发布维护者一次性在受限镜像仓库发布按架构划分的 `dameng-driver-bundle`，并在
   GitHub Actions Repository Variables 中配置其不可变镜像引用。
2. 发布维护者触发既有 `dev-build` 或 `release-v6`。工作流先校验该变量，再把驱动
   bundle 作为 BuildKit 的命名上下文传给所有标准 Dockerfile。
3. 平台管理员使用标准 `rbd-api`、`rbd-worker`、`rbd-chaos` 和 Console 镜像，按
   已有临时组件覆盖方式设置 `DB_TYPE=dm` 与达梦连接信息。
4. 管理员以 Deployment 就绪、核心健康接口、Console migration 和基础建表/查询
   验证结果确认平台使用达梦；使用 `DB_TYPE=mysql` 时保持现有行为。

### 2.2 页面原型

本期无 UI 页面、表单或弹窗。数据库类型仍由 Kubernetes 组件环境变量和现有
`RainbondCluster` 的数据库连接配置间接提供；Operator 自动渲染属于单独阶段。

### 2.3 外部系统交互

- GitHub Actions 从受限 OCI registry 拉取只包含官方驱动构建材料的命名上下文。
- Docker BuildKit 将 bundle 提供给 Go/Console 的普通 Dockerfile；最终镜像仅保留
  运行必需文件。
- 运行时 Go/Console 通过官方原生协议连接达梦，不经 MySQL 协议转换。

## 三、整体架构设计

### 3.1 系统架构图

```text
官方达梦安装介质（受限、按架构）
              |
              v
    dameng-driver-bundle OCI 镜像
              |
              v
 GitHub Actions: build-contexts dameng=docker-image://<immutable reference>
              |                                      |
              v                                      v
 rbd-api / worker / chaos Dockerfile            Console Dockerfile
 (Go driver + GORM dialect)                 (dmPython + dmDjango + DPI)
              |                                      |
              `---------- 标准 Rainbond 镜像 ---------'
                                  |
                                  v
                   DB_TYPE=mysql（默认）/ dm（配置）
```

### 3.2 核心流程

工作流在 build 前检查与当前 `matrix.arch` 对应的驱动镜像变量。`dameng` 命名上下文
根目录包含：

```text
go/dm-go-driver.zip
go/gorm_v1_dialect.zip          # 或 go/dmgorm1.zip
python/bin/libdmdpi.so
python/include/                 # dmPython 编译所需头文件
python/drivers/python/dmPython/
python/drivers/python/dmDjango/dmDjango3.0/
```

核心 Dockerfile 把 Go bundle 复制到构建容器、准备本地 Go module、始终以 `-tags dm`
编译，并始终跳过 UPX（DM 驱动不兼容 UPX 压缩）。Console 的普通 Dockerfile 从同一
命名上下文安装 dmPython/dmDjango，并在最终镜像保留 `libdmdpi.so` 与运行时动态库
配置。MySQL 驱动及其原有配置保持不变。

## 四、数据模型设计

### 4.1 新增数据库表

无。此变更只影响 CI 构建输入和镜像依赖，不改变 `region` 或 `console` 数据表。

### 4.2 数据关系

无新增业务实体。驱动 bundle 是 OCI 构建材料，不进入 Kubernetes Secret、ConfigMap
或 RainbondCluster 状态。

## 五、API 设计

### 5.1 接口列表

无 HTTP API、CRD 或 Operator 接口变更。

### 5.2 请求/响应结构

运行时契约不变：核心使用 `DB_TYPE=dm`（默认仍为 `mysql`）；Console 使用
`DB_TYPE=dm` 及现有通用 `DB_*` 变量。该构建改动不会在日志、镜像标签或 API
响应中暴露驱动来源或任何凭据。

## 六、核心实现设计

### 6.1 关键逻辑

1. 将 `ENABLE_DM` 条件分支删除，三个核心 Dockerfile 都通过命名 `dameng` 上下文
   取得官方 Go bundle 并以 `dm` build tag 构建；禁止 UPX 压缩这些二进制。
2. 将 `Dockerfile.dm` 的驱动安装层合并到 Console 普通 Dockerfile；补齐 dmPython
   所需 include 目录、`libaio`、DPI 运行库和 `ldconfig`。专用 Dockerfile 删除，
   避免两套构建漂移。
3. 在两个仓库的 `dev-build.yml` 与 `release-v6.yml` 对标准核心和 Console job 使用
   `build-contexts`；工作流针对每个架构读取对应的仓库变量，变量为空时在构建前明确
   失败，避免产出缺少 DM 驱动的“成功”镜像。
4. 新增结构化回归测试，断言普通 Dockerfile 和正式 Action 都消费命名上下文，且
   没有 `ENABLE_DM` 或专用 Dockerfile 分流。测试不读取官方驱动文件。

### 6.2 复用现有代码

- 复用 `scripts/prepare-dameng-go-driver.sh` 对官方 Go archive 的校验和 module
  准备逻辑。
- 复用 Console 已有的 dmPython/dmDjango 构建命令和数据库运行时逻辑。
- 复用现有 GitHub Actions registry 登录与 build-push-action，不新增部署期参数。

## 七、实施计划

### 跨层覆盖检查

- [x] Go (rainbond): 需要 — 三个标准核心 Dockerfile、两个正式 Action、构建回归测试。
- [x] Python (rainbond-console): 需要 — 标准 Dockerfile、两个正式 Action、构建回归测试。
- [ ] React (rainbond-ui): 不涉及 — 无用户界面或前端调用变化。
- [ ] Plugin frontend (enterprise-base): 不涉及。
- [ ] Plugin backend (plugin-template): 不涉及。
- [ ] rainbond-operator: 不涉及 — 本期仅修复标准镜像内容，组件配置机制不变。

### Sprint 1: 标准核心镜像

#### Task 1.1: 使 Go 标准镜像始终内置 DM driver
- 仓库：rainbond
- 文件：`hack/contrib/docker/{api,worker,chaos}/Dockerfile`、
  `hack/contrib/docker/dameng_dockerfile_test.go`
- 实现内容：命名上下文复制、始终 DM build tag、移除 `ENABLE_DM` 和 UPX 分支。
- 验收标准：结构测试证明三个普通 Dockerfile 均需要 bundle，`go build`/`go vet`
  通过，MySQL 默认运行时配置未变。

#### Task 1.2: 使核心正式 Action 注入驱动上下文
- 仓库：rainbond
- 文件：`.github/workflows/dev-build.yml`、`.github/workflows/release-v6.yml`、工作流测试。
- 实现内容：每个核心构建 job 校验架构变量并传递 `build-contexts`。
- 验收标准：静态测试证明两个发布入口都覆盖 api/worker/chaos，变量缺失时不可产出
  缺驱动镜像。

### Sprint 2: 标准 Console 镜像

#### Task 2.1: 将 DM driver 合并进普通 Console Dockerfile
- 仓库：rainbond-console
- 文件：`Dockerfile`、`Dockerfile.dm`、`console/tests/dameng_driver_bundle_test.py`
- 实现内容：合并 driver layer、保留运行库、删除专用 Dockerfile。
- 验收标准：测试断言普通 Dockerfile 安装 dmPython/dmDjango 且最终镜像包含 DPI；
  没有独立的部署镜像路线。

#### Task 2.2: 使 Console 正式 Action 注入驱动上下文
- 仓库：rainbond
- 文件：`.github/workflows/dev-build.yml`、`.github/workflows/release-v6.yml`、工作流测试。
- 实现内容：给普通 Dockerfile 的 build job 注入并校验按架构的驱动 bundle。
- 验收标准：两个正式入口均通过 `build-contexts` 提供 `dameng`，无 bundle 时立即失败。

### Sprint 3: 构建验证与交付

#### Task 3.1: 验证标准镜像
- 仓库：rainbond、rainbond-console
- 文件：`docs/dameng-standard-image-build.md`、测试清单。
- 实现内容：记录 bundle 制作契约、Action 变量名、标准镜像验收命令与恢复方式。
- 验收标准：Go `go build ./...`、`go vet ./...`、相关测试和清单验证通过；Console
  相关 pytest/check 通过；使用受限 bundle 的实际构建完成后，标准镜像在 DM 和 MySQL
  两种配置下完成启动冒烟验证。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
| --- | --- | --- |
| 核心标准 Action | `rainbond/.github/workflows/dev-build.yml` | api/worker/chaos 的 build-push-action 调用。 |
| 核心发布 Action | `rainbond/.github/workflows/release-v6.yml` | 多架构正式发布入口。 |
| 核心镜像 | `rainbond/hack/contrib/docker/{api,worker,chaos}/Dockerfile` | 当前错误地把 DM 置于可选分支。 |
| Console 标准镜像 | `rainbond-console/Dockerfile` | 由 rainbond Action checkout 后实际使用的 Dockerfile。 |
| Console 专用镜像 | `rainbond-console/Dockerfile.dm` | 要合并并删除的重复实现。 |
| Go bundle 准备 | `rainbond/scripts/prepare-dameng-go-driver.sh` | 官方 Go archive 的输入契约与校验。 |

## 前置条件与风险

1. 必须在受限 registry 中维护可信、可审计、按架构发布的 driver bundle；GitHub
   Repository Variables 只保存镜像引用，镜像仓库凭据仍复用已有 Actions secrets。
2. 当前取得的是 x86_64 的官方介质。多架构 release 必须提供同版本 arm64 bundle，
   否则该架构构建会在校验步骤失败；不能把 x86_64 的 DPI 运行库塞进 arm64 镜像。
3. 达梦驱动的再分发许可须由发布方确认。源码仓库和公开镜像不提交或公开驱动材料。
4. 标准核心 DM 二进制不能使用 UPX，镜像/二进制体积会增加；这是避免运行时驱动加载
   失败的必要代价。
