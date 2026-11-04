# 构建 Pod 的 CPU 和内存配置

在 `rbd-chaos` 的容器环境变量中配置以下两个可选参数：

| 环境变量 | 示例 | 作用 |
|---|---|---|
| `BUILD_POD_CPU` | `2` 或 `500m` | 为新建构建容器设置相同的 CPU request 和 limit |
| `BUILD_POD_MEMORY` | `4Gi` 或 `512Mi` | 为新建构建容器设置相同的内存 request 和 limit |

例如，在管理 `rbd-chaos` 的工作负载或 Rainbond 组件配置中添加：

```yaml
env:
  - name: BUILD_POD_CPU
    value: "2"
  - name: BUILD_POD_MEMORY
    value: "4Gi"
```

更新全部 `rbd-chaos` 实例并重启使环境变量生效后，后续新建构建 Pod 中的普通容器将具有：

```yaml
resources:
  requests:
    cpu: "2"
    memory: "4Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
```

配置覆盖统一构建控制器创建的源码、Dockerfile、CNB 和镜像打包等构建 Pod。它控制独立构建容器的资源，不改变 `rbd-chaos` 自身或业务运行容器的资源配置，也不修改已有构建 Pod。

两个变量可以单独设置。未设置或仅含空白的项保留原有行为，其他资源项及 init container 保持原样。配置必须是正数，采用 Kubernetes 资源数量格式；CPU 精度不能小于 `1m`。值非法时，该构建返回包含变量名的配置错误，拒绝创建 Pod，不会静默退回到无限制运行。

这里的数值仅为配置格式示例。应根据构建峰值、节点余量及并发量选择实际额度。request 与 limit 相同意味着调度时按完整额度申请资源；不足时构建 Pod 可能 Pending，超过内存上限时构建可能因 OOM 失败。
