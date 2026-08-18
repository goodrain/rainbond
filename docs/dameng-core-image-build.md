# 达梦核心镜像构建

达梦 Go 驱动和 GORM v1 方言由达梦安装包提供，不能提交到本仓库。构建达梦镜像前，
从已获授权的 DM 安装介质中提取下列任一种组合到本地、已忽略的
`third_party/dameng/` 目录：

- `dmgorm1.zip`；或
- `dm-go-driver.zip`（部分版本名为 `dm.zip`）和 `gorm_v1_dialect.zip`。

构建阶段会执行 `scripts/prepare-dameng-go-driver.sh`，将驱动及方言准备成模块
`third_party/dameng/dm`。该目录在 `.gitignore` 中，禁止加入 Git 或镜像以外的
公开制品。

构建 API、Worker 和 Chaos 的达梦变体：

```bash
docker build --build-arg ENABLE_DM=true \
  -f hack/contrib/docker/api/Dockerfile \
  -t <PRIVATE_REGISTRY>/rbd-api:dm-test .

docker build --build-arg ENABLE_DM=true \
  -f hack/contrib/docker/worker/Dockerfile \
  -t <PRIVATE_REGISTRY>/rbd-worker:dm-test .

docker build --build-arg ENABLE_DM=true \
  -f hack/contrib/docker/chaos/Dockerfile \
  -t <PRIVATE_REGISTRY>/rbd-chaos:dm-test .
```

未设置 `ENABLE_DM=true` 的构建保持原有 MySQL/SQLite 行为；若在该镜像设置
`DB_TYPE=dm`，进程会立即返回“镜像未包含达梦支持”的错误，不会回退到 MySQL 或
无限重试。

达梦镜像会刻意跳过 UPX 压缩：达梦原生驱动的二进制经 UPX 压缩后无法可靠启动。
这只影响 `ENABLE_DM=true` 变体；默认镜像仍保留原有 UPX 压缩步骤。

本阶段仅支持达梦原生驱动和 GORM 建表。当前 DAO 中的 MySQL 专用
`gorm-bulk-upsert`、表结构补丁和原生 MySQL SQL 尚未逐项转换；达梦启动会跳过这
些 Bootstrap 路径，不能据此宣称已有平台数据或全部 DAO 已兼容。
