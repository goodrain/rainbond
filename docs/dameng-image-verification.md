# 达梦镜像替换与验证

本文只覆盖镜像级的首轮验证：以带达梦官方驱动的测试镜像手工替换平台组件，并连接
独立的达梦测试库。它不改变 `RainbondCluster`、Operator、Helm 或 ROI 的数据库
渲染逻辑；这些控制器仍可能在重调谐时覆盖直接修改的工作负载。

不要在命令行历史、YAML、镜像标签或日志中写入数据库密码、DSN 或驱动介质。连接
凭据应始终由现有 Secret 注入，下面所有尖括号内容均为占位符。

## 1. 前置条件

1. 准备两个全新的达梦测试 schema：`region` 供 `rbd-api`/`rbd-worker` 使用，
   `console` 供 Console 使用。不要用已有 MySQL 平台库直接切换或迁移。
2. 在目标架构的受控构建机上准备与达梦服务端兼容的官方驱动包。驱动文件位于
   被 Git 忽略的 `third_party/dameng/`，不能提交到仓库或发布到公共制品。
3. 记录当前组件镜像、环境变量和启动参数，作为回滚基线。先定位控制平面实际管理的
   `RbdComponent` 与工作负载；不同安装版本的资源名可能不同：

   ```bash
   kubectl -n <RBD_NAMESPACE> get rbdcomponent
   kubectl -n <RBD_NAMESPACE> get deployment \
     -o custom-columns=NAME:.metadata.name,IMAGES:.spec.template.spec.containers[*].image
   kubectl -n <RBD_NAMESPACE> get rbdcomponent <COMPONENT_NAME> -o yaml
   ```

   应确认管理 `rbd-api`、`rbd-worker`，以及 Console 工作负载（通常为
   `rbd-app-ui`）的资源和其镜像/环境变量来源。仅改 Pod 或 Deployment 可用于
   短时测试，但会被 Operator 恢复；验证配置应保存在该集群实际支持的组件覆盖入口中。

## 2. 构建达梦测试镜像

核心 API 与 Worker 使用相同的私有 Go 驱动准备目录。仅设置
`ENABLE_DM=true` 才会编入达梦驱动和 GORM v1 方言：

```bash
docker build --build-arg ENABLE_DM=true \
  -f hack/contrib/docker/api/Dockerfile \
  -t <PRIVATE_REGISTRY>/rbd-api:dm-test .

docker build --build-arg ENABLE_DM=true \
  -f hack/contrib/docker/worker/Dockerfile \
  -t <PRIVATE_REGISTRY>/rbd-worker:dm-test .
```

默认构建不传 `ENABLE_DM=true`，保持现有 MySQL/SQLite 行为及原有压缩流程。达梦
变体会跳过 UPX，避免原生驱动在压缩后的可执行文件中无法启动。

Console 使用独立 Dockerfile 和 BuildKit 命名上下文；不要向正常 Dockerfile 的
上下文加入驱动文件：

```bash
docker buildx build \
  --file Dockerfile.dm \
  --build-context dameng=third_party/dameng \
  --tag <PRIVATE_REGISTRY>/rainbond-console:dm-test \
  --push .
```

先在私有镜像仓库确认三个测试标签可拉取，再记录对应摘要。默认核心镜像构建不包含
达梦驱动；Console 的正常 `Dockerfile` 不应引用达梦驱动。只替换本次验证的测试标签。

## 3. 配置受管组件

在第 1 节识别出的**权威组件配置**中替换下列镜像。若当前版本没有可持久化的
`RbdComponent` 覆盖字段，只能把此步骤作为临时工作负载验证，并记录 Operator
重调谐会撤销该修改。

| 组件 | 测试镜像 | 达梦设置 |
| --- | --- | --- |
| `rbd-api` | `<PRIVATE_REGISTRY>/rbd-api:dm-test` | `DB_TYPE=dm`；将受控配置中的既有 `--mysql` 兼容参数/Secret 引用改为达梦 `region` 测试库，并确保 API 启动参数为 `--db-type=dm`。 |
| `rbd-worker` | `<PRIVATE_REGISTRY>/rbd-worker:dm-test` | `DB_TYPE=dm`；将既有 `--mysql` 兼容参数/Secret 引用改为与 API 相同的达梦 `region` 测试库，并确保启动参数为 `--db-type=dm`。 |
| Console 工作负载 | `<PRIVATE_REGISTRY>/rainbond-console:dm-test` | `DB_TYPE=dm`，并使用下方通用数据库环境变量。 |

Console 的连接信息使用通用变量；密码必须引用已有 Secret，不能以 `value` 明文配置：

```yaml
env:
  - name: DB_TYPE
    value: dm
  - name: DB_HOST
    value: <DM_HOST>
  - name: DB_PORT
    value: "<DM_PORT>"
  - name: DB_NAME
    value: console
  - name: DB_USER
    valueFrom:
      secretKeyRef:
        name: <DM_CONSOLE_SECRET>
        key: username
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: <DM_CONSOLE_SECRET>
        key: password
```

核心当前继续使用 `--mysql` 兼容参数通道，但该参数的值或 Secret 引用必须切换为指向
达梦 `region` 测试库；`DB_TYPE=dm` 只选择驱动，不会改写该连接目标，也不会因为设置了
`DB_HOST`、`DB_PORT` 等变量自行拼接连接信息。不要把连接字符串复制到日志、命令行或
文档中。确保 `region` 与 `console` 分别指向对应的独立测试 schema。

按一次只替换一个组件的顺序执行：先 `rbd-api`，再 `rbd-worker`，最后 Console。
每一步都等待新 Pod 就绪并保留前一个版本的镜像摘要。首次连接空库时，核心以 GORM
初始化 `region` schema，Console 通过 Django migration 和默认 Region 初始化
`console` schema。

## 4. 验收检查

达梦测试库的 `CASE_SENSITIVE=Y` 可作为本期第一轮验证目标；它不是自动拒绝条件。
验收中应记录表/字段创建、平台启动与基本读写是否成功。它仍不是全部业务 DAO 的
兼容性认证。

每次替换后执行以下不含凭据的检查：

```bash
kubectl -n <RBD_NAMESPACE> get pods -w
kubectl -n <RBD_NAMESPACE> get deployment
kubectl -n <RBD_NAMESPACE> get rbdcomponent
kubectl -n <RBD_NAMESPACE> logs deployment/<COMPONENT_DEPLOYMENT> --tail=200
```

检查项目：

- 新 Pod 使用预期镜像摘要、持续 Ready，且没有“未包含达梦支持”、驱动加载失败或数据库
  连接重试循环。
- `rbd-api` 和 `rbd-worker` 都能连接同一个新 `region` schema；Console 能连接新
  `console` schema 并完成迁移与默认 Region 初始化。
- 平台登录、集群信息读取及一次非破坏性基础操作可完成；数据库侧以受控客户端执行
  `SELECT 1` 并确认两个 schema 中存在预期表。
- 日志、事件和对象定义中均不出现密码或完整连接字符串。

如 `CASE_SENSITIVE=Y` 下出现标识符、建表或迁移错误，应停止扩大替换范围，保留错误
的脱敏证据并将该结果记录为兼容性问题；不要通过修改生产库的大小写选项来绕过。

## 5. 回滚

1. 将每个组件镜像恢复为第 1 节记录的原镜像摘要，并移除 `DB_TYPE=dm` 覆盖。
2. 恢复原有 MySQL/SQLite 数据库类型、连接 Secret 引用和启动参数；不要把达梦测试
   凭据改写到原组件配置中。
3. 等待 Operator 重调谐和全部原 Pod 就绪，复查平台健康状态、工作负载状态和日志。
4. 达梦 `region`/`console` 测试 schema 留作故障分析或按数据库管理员流程清理；回滚
   不会自动删除它们。

## 6. 当前边界

- 本期使用达梦原生协议和官方驱动，不使用 MySQL 协议兼容层。
- MySQL 专用原生 SQL、`gorm-bulk-upsert`、表结构补丁和其他 MySQL DDL 尚未逐项
  移植；它们不属于本次镜像连接与空库初始化验收范围。
- 不包含 MySQL 既有数据迁移，也不应将该验证视为已有平台数据库的切库方案。
- 不包含 `RainbondCluster`/Operator 自动下发数据库类型、镜像或连接参数；升级、
  重调谐后的持久化配置需要后续 Operator 支持。
