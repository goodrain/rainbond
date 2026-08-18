# 达梦标准镜像构建

Rainbond 的标准发布镜像同时包含 MySQL 和达梦运行时依赖。达梦由运行时的
`DB_TYPE` 选择；发布和部署过程中不再选择达梦专用镜像，也不使用 `ENABLE_DM`。

本文只描述不含凭据的构建契约。达梦驱动材料、数据库连接信息、仓库登录凭据和 CI
Secrets 均不得提交到源码仓库、写进 Actions 日志或放入镜像标签。

## 1. 私有驱动 bundle

从已获授权且与目标架构匹配的达梦安装介质提取驱动材料，制作一个**私有、不可变、
多架构 OCI 镜像**。该镜像只是 BuildKit 的命名构建上下文，不是部署给平台的运行
镜像。其根目录必须符合下面的布局：

```text
go/
  dmgorm1.zip
  # 或者 dm-go-driver.zip（部分版本名为 dm.zip）和 gorm_v1_dialect.zip
python/
  bin/
    libdmdpi.so
  include/
    ...
  drivers/python/
    dmPython/
      ...
    dmDjango/dmDjango3.0/
      ...
```

同一不可变引用必须是 OCI manifest list，其中每个目标平台都有相应架构的 Go 驱动、
`libdmdpi.so` 和 Python 编译头文件。不要把不同架构的原生库混用。发布方应在受限
环境中确认达梦驱动的再分发许可、归档版本与支持的平台，并在镜像仓库中保留可审计的
摘要。

驱动归档、源码、动态库和任何解压后的目录都不进入 `rainbond` 或
`rainbond-console` Git 历史。仓库中被忽略的 `third_party/dameng/` 只可用于受控的
本地准备与故障排查，不能作为 Actions 的普通构建上下文。

## 2. 一次性 Actions 配置

在发布仓库的 GitHub Actions **Repository variable** 中一次性设置：

```text
DAMENG_DRIVER_BUNDLE_IMAGE=<PRIVATE_OCI_IMAGE_BY_IMMUTABLE_DIGEST>
```

变量只保存私有 OCI 镜像的不可变引用，不包含用户名、密码、令牌或数据库 DSN。CI
访问私有仓库时继续使用既有受保护 Secrets；不要把它们转存到变量、Dockerfile、YAML
或命令历史。变量为空时，标准镜像工作流应在 build 前失败，不能发布缺少达梦驱动的
镜像。

### 发布前置条件

Action 不会创建或发布达梦驱动 bundle。标准镜像构建开始前，发布方必须已在获授权的
私有仓库中发布对应的多架构 bundle，并配置指向该 bundle 的不可变引用变量；任一条件
缺失时，工作流必须失败，不能改用不含驱动的镜像或回退到其他源码分支。

## 3. 标准镜像和正常 Actions

`dev-build.yml` 与 `release-v6.yml` 会把该 OCI 镜像作为名为 `dameng` 的 BuildKit
上下文传给正常构建入口：

| 标准镜像 | 正常 Dockerfile | 构建行为 |
| --- | --- | --- |
| `rbd-api` | `hack/contrib/docker/api/Dockerfile` | 准备官方 Go module 和 GORM v1 dialect，并以 `dm` build tag 编译。 |
| `rbd-worker` | `hack/contrib/docker/worker/Dockerfile` | 与 API 使用相同的 Go driver 输入。 |
| `rbd-chaos` | `hack/contrib/docker/chaos/Dockerfile` | 与 API 使用相同的 Go driver 输入。 |
| Console | `rainbond-console/Dockerfile` | 安装 dmPython、dmDjango 和运行时 DPI 动态库。 |

核心标准镜像包含达梦原生驱动时，按发布策略禁用 UPX。
Console 最终镜像仅保留运行所需的 `libdmdpi.so` 和动态链接配置；编译头文件及驱动
构建目录不会进入最终层。MySQL 客户端和原有 MySQL 驱动继续保留在这些标准镜像中。

## 4. 运行时数据库选择

标准镜像默认仍使用 MySQL：

```text
DB_TYPE=mysql
```

要连接达梦时，受管组件配置中设置：

```text
DB_TYPE=dm
```

核心服务继续通过已有受控连接参数连接 `region` 数据库；Console 使用已有的通用
`DB_*` 配置连接 `console` 数据库。用户名和密码必须来自 Secret 引用，例如
`<DB_SECRET>`，不得写为明文环境变量或命令参数。`DB_TYPE` 只选择驱动和数据库
方言，不会自动创建连接凭据，也不会修改地址、端口或 schema。

本次变更不修改 Operator、`RainbondCluster` CRD、Helm chart 或 ROI。现有安装版本若
不能持续渲染数据库类型和连接参数，应继续通过其当前受支持的组件覆盖入口进行测试；
直接修改 Deployment 可能被控制器恢复。

## 5. 冒烟验证

先记录当前镜像摘要和组件配置，然后在独立的空测试库中验证。所有名称均使用占位符：

```bash
kubectl -n <RBD_NAMESPACE> get rbdcomponent
kubectl -n <RBD_NAMESPACE> get deployment \
  -o custom-columns=NAME:.metadata.name,IMAGES:.spec.template.spec.containers[*].image
kubectl -n <RBD_NAMESPACE> get pods
kubectl -n <RBD_NAMESPACE> logs deployment/<COMPONENT_DEPLOYMENT> --tail=200
```

检查 `rbd-api`、`rbd-worker`、`rbd-chaos` 和 Console 均使用标准发布镜像且持续
Ready；在 `DB_TYPE=dm` 下不应出现驱动缺失、动态库加载失败或数据库连接循环。随后
验证空的 `region` 与 `console` schema 初始化、平台健康接口、Console 登录和一次
非破坏性基础操作。还应在 `DB_TYPE=mysql` 下完成同样的基本启动检查，证明双驱动
镜像没有破坏默认路径。

日志、事件、YAML、终端历史和验证记录中都不得包含密码、完整 DSN 或私有仓库登录
信息。

## 6. 回滚

如果冒烟验证失败：

1. 恢复预先记录的标准镜像摘要和原数据库类型配置。
2. 恢复原有的连接 Secret 引用；不要复制、打印或修改其中的凭据。
3. 等待受管组件完成重调谐并全部 Ready，再复查健康接口与日志。
4. 保留独立达梦测试库以便按数据库管理员流程诊断或清理；回滚不会自动删除它。

若失败发生在 Action 构建阶段，保留失败构建的脱敏日志，并将
`DAMENG_DRIVER_BUNDLE_IMAGE` 指向上一个已验证的不可变私有 OCI 引用后重新构建。
