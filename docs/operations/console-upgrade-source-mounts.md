# Console 升级前的源码挂载检查

平台镜像升级接口 `POST /v2/cluster/rbd-upgrade` 继续接收组件名到镜像地址的映射。请求包含 `rbd-app-ui` 时，即使镜像地址没有变化，也会检查该 `RbdComponent.spec.volumeMounts`：开发标签的实际内容可能已经变化。

如果挂载覆盖 `/app/ui/console`、`/app/ui/www`、`/app/ui/goodrain_web` 内的核心逻辑 `.py`、`.pyc`，覆盖源码包目录或这些根目录的祖先，接口返回 HTTP 409，并列出需要复核的规范化路径。`/app/ui/console/migrations`、`/app/ui/www/migrations` 是持久迁移资产，完整子树排除在本次核心逻辑覆盖判断之外。检测到核心逻辑覆盖时，本次请求中的所有组件都不会更新镜像。检查只读取请求涉及的 RbdComponent，不读取 ConfigMap/Secret 内容，也不会自动删除未知或合法定制挂载。

标准数据、SSL、serviceaccount、Console/www 迁移目录及其单文件（包括 `.py`/`.pyc`）、静态资源、模板、配置目录挂载可继续使用。路径先规范化，因此 `migrations/../views/example.py` 仍属于核心逻辑覆盖；`static`、`templates`、`config` 中显式挂载的 `.py`/`.pyc` 文件也仍需复核。检查基于挂载路径，不能判断外部卷的真实内容、符号链接或运行中 Deployment/admission 注入的额外挂载是否兼容。

## 遇到 409 时

1. 检查提示中对应的 **RbdComponent** 的 `spec.volumeMounts` 和关联 `spec.volumes`。不要把完整 CR、ConfigMap 正文或认证信息复制到工单、Git、日志；记录对象身份、挂载路径和文件指纹即可。
2. 找到真实配置来源：Helm 的定制模板/values、GitOps 清单或运维脚本可能再次写入该挂载。仅修改生成的 Deployment 可能在 Operator reconcile、重启或升级时被恢复。
3. 核对目标镜像原生源码和保留定制的兼容性。已挂载容器中同一路径的内容不能证明镜像原生内容；应离线检查镜像或使用不带覆盖卷的隔离验证环境。
4. 如果旧补丁已经进入目标镜像，经过兼容复核后，在权威 CR/上游配置中精确退休对应挂载。如果仍需合法定制，应先把补丁合入兼容的定制镜像并验证。不要为通过检查而批量清空 `volumeMounts`，也不要把补丁改成不受检查的路径。
5. 共享 ConfigMap 卷仍被其他挂载使用时，保留该卷和 ConfigMap。不可变 ConfigMap 不能原地改写；保留旧资源供回退。VNC、relay、SSL、PVC 和迁移挂载必须逐项核对。
6. 在升级前重新核对 CR 的 resourceVersion 与挂载指纹，再重试升级。验证运行源码、业务修复、Console 就绪和 VNC HTTP/WS；再触发一次 Operator reconcile，并验证下一次镜像升级不恢复旧源码。

正常升级仍只 patch `spec.image`，保留未知 CR 字段和用户配置。读取到 resourceVersion 时，镜像 patch 带该版本的乐观锁；并发变更会进入原有逐组件更新失败列表，需要重新检查后重试，不会自动重试陈旧预检。普通组件读取或更新失败仍保留逐组件失败列表语义，这个接口不提供跨组件事务回滚。

这项预检覆盖平台升级 API。直接修改 CR、执行 Helm、使用 GitOps 或其他运维入口升级时，也应执行同样的配置复核；接口检查不能拦截这些独立入口。
