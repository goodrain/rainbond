# 在 Rainbond 中部署 KubeBlocks

> [KubeBlocks](https://github.com/apecloud/kubeblocks) is an open-source Kubernetes operator for databases (more specifically, for stateful applications, including databases and middleware like message queues), enabling users to run and manage multiple types of databases on Kubernetes. 

你可以通过 [Kubeblock Adapter for Rainbond Plugin](https://github.com/furutachiKurea/kb-adapter-rbdplugin) 实现 KubeBlocks 在 Rainbond 中的集成。在绝大部分情况下，你都能像使用 Rainbond 组件一样管理通过 KubeBlocks 创建的数据库

## 安装 KubeBlocks

安装过程参考 KubeBlocks 的[安装文档](https://kubeblocks.io/docs/release-1_0_1/user_docs/overview/install-kubeblocks)，这里我们简要说明通过 Helm 安装 KubeBlocks 的步骤，下面的内容大部分来自 [KubeBlocks 官方文档](https://kubeblocks.io/docs/release-1_0_1/user_docs/overview/introduction)

### [前提条件](https://kubeblocks.io/docs/release-1_0_1/user_docs/overview/install-kubeblocks#prerequisites)

| Component 组件    | Database 数据库 | Recommendation 推荐配置                   |
| :---------------- | :-------------- | :---------------------------------------- |
| **Control Plane** | -               | 1 个节点（4 核，4 GB 内存，50GB 存储）    |
| **Data Plane **   | MySQL           | 2 个节点（2 核 CPU，4GB 内存，50GB 存储） |
|                   | PostgreSQL      | 2 个节点（2 核 CPU，4GB 内存，50GB 存储） |
|                   | Redis           | 2 个节点（2 核 CPU，4GB 内存，50GB 存储） |
|                   | MongoDB         | 3 个节点（2 核 CPU，4GB 内存，50GB 存储） |

> - Kubernetes 集群（建议 v1.21+版本）——如需可创建测试集群
> - `kubectl` 已安装并配置 v1.21+版本，具备集群访问权限
> - 已安装 Helm（[安装指南](https://helm.sh/docs/intro/install/)）
> - 已安装快照控制器 （[安装指南](https://kubeblocks.io/docs/release-1_0_1/user_docs/references/install-snapshot-controller))

### 安装 KubeBlocks

```shell
# 安装 CRDs
kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v1.0.1/kubeblocks_crds.yaml

# 设置 Helm Repository
helm repo add kubeblocks https://apecloud.github.io/helm-charts
helm repo update

# 部署 KubeBlocks
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version=v1.0.1

# 可以设置使用 KubeBlocks 提供的镜像源
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version=v1.0.1 \
--set image.registry=apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com \
--set dataProtection.image.registry=apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com \
--set addonChartsImage.registry=apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com
```

**注意，KubeBlocks 的 Addon 需要单独设置镜像源**, 参见: <https://kubeblocks.io/docs/release-1_0_1/user_docs/references/install-addons>

可以在部署时通过指定配置文件来自动创建 BackupRepo: <https://kubeblocks.io/docs/release-1_0_1/user_docs/concepts/backup-and-restore/backup/backup-repo>

在部署时创建 BackupRepo 会[简单](#配置-backuprepo)很多，下面以 Rainbond 自动创建的 minio 为例:

Rainbond 自动创建的 minio 账号密码为: `admin/admin1234`，你需要手动创建 `ACCESS KEY` 和 `SECRET KEY` 并创建一个 Bucket

```yaml
# backuprepo.yaml
backupRepo:
  create: true
  storageProvider: minio
  config:
    bucket: <BUCKET>
    endpoint: http://minio-service.rbd-system.svc.cluster.local:9000
  secrets:
    accessKeyId: <ACCESS KEY>
    secretAccessKey: <SECRET KEY>
```

部署时使用

```shell
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version=v1.0.1 \
-f backuprepo.yaml
```

### 验证安装

执行：

```shell
kubectl -n kb-system get pods
```

预期输出：

```shell
NAME                                             READY   STATUS    RESTARTS       AGE
kubeblocks-7cf7745685-ddlwk                      1/1     Running   0              4m39s
kubeblocks-dataprotection-95fbc79cc-b544l        1/1     Running   0              4m39s
```

如果 KubeBlocks 工作负载全部就绪，则表示 KubeBlocks 已成功安装。

如果你没有在安装 KubeBlocks 时跳过 Addon 的自动安装的话，KubeBlocks 会自动安装一部分 Addon

**注意**：在 Rainbond 上能够使用的数据库类型取决于你安装的 KubeBlocks Addon 和 Block Mechanica 的支持，目前支持 MySQL semisync、PostgreSQL、Redis replication、RabbitMQ

### 配置 [BackupRepo](https://kubeblocks.io/docs/release-1_0_1/user_docs/concepts/backup-and-restore/backup/backup-repo)

> backupRepo is the storage repository for backup data. Currently, KubeBlocks supports configuring various object storage services as backup repositories, including OSS (Alibaba Cloud Object Storage Service), S3 (Amazon Simple Storage Service), COS (Tencent Cloud Object Storage), GCS (Google Cloud Storage), OBS (Huawei Cloud Object Storage), Azure Blob Storage, MinIO, and other S3-compatible services.

你至少需要配置好一个 BackupRepo 才能使用 KubeBlocks 的备份功能

你可以参考官方提供的示例创建你的 BackupRepo，注意，如果你将 `accessMethod` 设置为了 `Mount`，你需要在你可能需要用到备份功能的 namespace 中都配置好 access key

下面是一个使用 `accessMethod: Tool` 的 S3 BackupRepo 示例，来自 KubeBlocks 官方文档

```shell
# Create a secret to save the access key for S3
kubectl create secret generic s3-credential-for-backuprepo \
  -n kb-system \
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY>

# Create the BackupRepo resource
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupRepo
metadata:
  name: my-repo
  annotations:
    dataprotection.kubeblocks.io/is-default-repo: "true"
spec:
  storageProviderRef: s3
  accessMethod: Tool
  pvReclaimPolicy: Retain
  volumeCapacity: 100Gi
  config:
    bucket: test-kb-backup
    endpoint: ""
    mountOptions: --memory-limit 1000 --dir-mode 0777 --file-mode 0666
    region: cn-northwest-1
  credential:
    name: s3-credential-for-backuprepo
    namespace: kb-system
  pathPrefix: ""
EOF
```

你可以通过 `kubectl get backuprepo` 获取到你创建的 BackupRepo 的状态，如果遇到问题请查看 KubeBlocks 官方文档：<https://kubeblocks.io/docs/release-1_0_1/user_docs/concepts/backup-and-restore/backup/backup-repo>

## 安装 Kubeblock Adapter for Rainbond Plugin (原 Block Mechanica)

- 使用 Kubeblock Adapter for Rainbond Plugin 提供的镜像

```shell
git clone https://github.com/furutachiKurea/kb-adapter-rbdplugin.git && cd kb-adapter-rbdplugin
make deploy
# or
kubectl apply -f https://raw.githubusercontent.com/furutachiKurea/kb-adapter-rbdplugin/refs/heads/main/deploy/k8s/deploy.yaml
```

- 或者通过手动构建镜像以使用最新版本：

```shell
git clone https://github.com/furutachiKurea/kb-adapter-rbdplugin.git && cd kb-adapter-rbdplugin
make image
# 然后 push 到你的镜像仓库
```

更新 `deploy/k8s/deploy.yaml` 中的镜像地址，然后执行

```shell
make deploy
```

Block Mechanica 需要部署在 rbd-system namespace 中，为了简化安装, rbd-api 中硬编码了 kb-adapter-rbdplugin 使用的 namespace，所以不要修改 `deploy.yaml` 中除镜像地址以外的内容，未来待 Rainbond 的插件体系完善之后将会有所优化

## 在 Rainbond 中使用 KubeBlocks

接下来只需要像使用 Rainbond 一样使用通过 KubeBlocks 创建的数据库即可，具体参见[使用文档](Use_KubeBlocks_in_Rainbond.md)