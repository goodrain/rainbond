# Rainbond 虚拟机镜像引用防护设计文档

## 一、项目背景

### 1.1 项目架构

Rainbond UI 通过 Console 创建虚拟机组件；Console 保存虚拟机镜像资产和 `vm_disk_imports` 属性；Rainbond Worker 将该属性转换为 KubeVirt `DataVolume.spec.source.registry.url`。CDI 随后用该 OCI 镜像引用启动导入容器。

### 1.2 现有基础

虚拟机 URL 导入和上传页面把 `image_name` 作为“镜像名称/保存名称”接收。Console 直接拼接 `<tenant namespace>:<image_name>` 作为内部运行时镜像地址，Worker 只补充 `docker://` 和仓库域名而不校验引用格式。

当用户输入 `DBServer(TongYong)` 时，生成的 `goodrain.me/ceshi:DBServer(TongYong)` 违反 OCI tag 格式。Kubelet 拒绝创建 CDI server 容器，importer 一直等待 ready 文件并重启，最终 VM 为 `DataVolumeError`。

### 1.3 核心需求

1. 用户可继续使用任意可读的虚拟机镜像展示名称，包括中文和括号。
2. 新建的上传、URL 导入虚拟机镜像必须使用系统生成的合法 OCI tag。
3. Console 和 Worker 必须拒绝不合法的 registry 镜像引用，避免生成必然失败的 DataVolume。
4. UI 应明确该名称是展示名称，内部镜像标识由系统生成。

## 二、用户旅程

### 2.1 用户操作流程

1. 用户从“创建组件 → 虚拟机”选择 URL 导入或上传镜像，并填写显示名称，例如 `DBServer(TongYong)`。
2. UI 提示显示名称仅用于识别资产，系统自动生成安全的内部镜像 tag。
3. Console 创建资产时保存显示名称，同时为其生成稳定、合法的内部镜像引用；用户创建 VM 后可在资产目录看到原展示名称。
4. Worker 导入前再次校验 registry 引用。若遗留资产、模板或直接写入的属性包含非法值，部署会在 Rainbond 侧失败并报告明确错误，不会创建无限 CrashLoop 的 importer Pod。

### 2.2 页面原型

- 创建虚拟机表单（URL 导入）：镜像名称字段保留自由文本输入，增加“仅用于展示，系统会自动生成内部镜像标识”的说明。
- 创建虚拟机表单（上传镜像）：同样增加说明。
- 虚拟机镜像资产目录：继续展示资产 `name`，不展示或要求用户编辑内部 tag。
- 不新增页面、弹窗或表单。

### 2.3 外部系统交互

- CDI/KubeVirt 接收的 registry URL 始终为 OCI 合法引用。
- 不新增 webhook、回调或通知；利用现有部署失败事件返回 Worker 的明确错误。

## 三、整体架构设计

### 3.1 系统架构图

```text
UI display_name
  → Console VM create
      → generated runtime image reference
          → vm_disk_imports
              → Worker reference validation
                  → KubeVirt DataVolume → CDI importer
```

### 3.2 核心流程

Console 为新导入资产生成与展示名分离的内部 tag：tag 使用限定字符和稳定哈希，长度符合 OCI 限制。`VirtualMachineImage.name` 继续保存展示名称，`image_url` 保存内部运行时镜像地址。已存在的正常资产继续使用其 `image_url`。

Worker 在给 registry 来源补齐 registry host 后，用现有 Docker reference parser 验证完整引用。无效时返回错误并停止服务定义构造，因而不会下发 DataVolume 或让 CDI 重试。

## 四、数据模型设计

### 4.1 新增数据库表

不新增表、不迁移列。

- `VirtualMachineImage.name`：用户可读展示名称。
- `VirtualMachineImage.image_url`：系统生成或已有的运行时镜像引用。
- `ComponentK8sAttributes(vm_disk_imports)`：保存经 Console 生成或校验后的镜像引用。

### 4.2 数据关系

`VirtualMachineImage` 由 VM 创建流程引用；其 `image_url` 写入服务镜像和 `vm_disk_imports`，Worker 读取后生成 DataVolume。展示名不再参与 OCI 引用组装。

## 五、API设计

### 5.1 接口列表

现有创建虚拟机接口保持不变：

- `POST /console/teams/{tenant}/.../create/vm`
- 输入 `image_name` 的语义调整为展示名称；无新增必填字段。

### 5.2 请求/响应结构

请求仍传入：

```json
{"image_name":"DBServer(TongYong)","vm_url":"https://example.invalid/db.qcow2"}
```

服务端内部生成类似 `tenant-ns:vm-dbserver-tongyong-a1b2c3d4` 的引用。对外创建成功响应不暴露底层 tag；已有的错误响应格式保持不变。

## 六、核心实现设计

### 6.1 关键逻辑

1. Console 以租户命名空间、展示名和 SHA-256 摘要生成合法 tag；文本部分仅保留小写 ASCII 字母、数字和连字符，无法转换时使用 `vm`，再追加短摘要避免冲突。
2. Console 创建 URL/上传资产和构建运行时镜像时均调用该统一生成器，杜绝两条路径语义漂移。
3. Worker 规范化 registry URL 后调用 `github.com/docker/distribution/reference.ParseAnyReference`；注册表来源必须解析为有效命名引用。
4. 无效配置在 `ShareFileVolume.CreateVolume` 期间返回错误并记录安全、无凭据的镜像引用；不生成 DataVolume template。
5. UI 仅补充 i18n 说明，不以运行时 tag 规则限制展示名称。

### 6.2 复用现有代码

- Console 复用 `console.services.vm_boot_source` 的运行时镜像地址生成入口。
- Go 复用 Rainbond builder 已使用的 Docker distribution reference parser。
- Worker 复用现有 `vm_import.go` 规范化和 `CreateVolume` 错误传播链。

## 七、实施计划

### 跨层覆盖检查

- [x] Go (rainbond): 需要 — 校验 registry DataVolume 引用并使创建卷流程失败返回。
- [x] Python (console): 需要 — 分离展示名称与内部 tag，保存前验证 registry disk import。
- [x] React (rainbond-ui): 需要 — 将输入字段说明为展示名称并增加双语提示。
- [x] Plugin frontend (enterprise-base): 不涉及 — 功能在主平台 UI 内。
- [x] Plugin backend (plugin-template): 不涉及 — CDI DataVolume 由 Rainbond Worker 创建。

### Sprint 1: 后端安全与兼容性

#### Task 1.1: Worker 拒绝非法 registry 引用

- 仓库：rainbond
- 文件：`worker/appm/volume/vm_import.go:123-375`、`worker/appm/volume/share-file.go:51-145`、`worker/appm/volume/vm_import_test.go`
- 实现内容：规范化后解析 registry 镜像引用；将构建失败沿卷创建链返回；覆盖合法、带 scheme、短镜像和含括号的非法 tag。
- 验收标准：非法引用不能生成 DataVolume template，合法引用的现有行为不变。

#### Task 1.2: Console 生成合法内部 tag 并阻断不合法导入配置

- 仓库：rainbond-console
- 文件：`console/services/vm_boot_source.py:1-39`、`console/views/app_create/vm_run.py:138-175`、`console/services/virtual_machine.py:475-492,1018-1037`、相关测试。
- 实现内容：用展示名生成合法内部 tag；对 registry 类型 `vm_disk_imports` 做服务端 validation；覆盖带括号显示名、稳定性和已有合法引用。
- 验收标准：`DBServer(TongYong)` 创建后保存的 `image_url` 和 disk import 均为合法 tag，错误属性不会持久化。

### Sprint 2: 用户提示

#### Task 2.1: 明确显示名称语义

- 仓库：rainbond-ui
- 文件：`src/components/ImageVirtualMachineForm/index.js:1055-1101`、`src/locales/zh-CN/team.js`、`src/locales/en-US/team.js`、相关节点测试。
- 实现内容：为 URL 和上传模式的镜像名称字段使用展示名称文案和双语说明。
- 验收标准：用户可知特殊字符不会成为 OCI tag，构建通过。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|------|------|------|
| VM 表单自由文本输入 | `rainbond-ui/src/components/ImageVirtualMachineForm/index.js` | 当前仅 required 校验 |
| VM 资产创建 | `rainbond-console/console/views/app_create/vm_run.py` | 当前拼接 namespace 和 image_name |
| 运行时镜像名 | `rainbond-console/console/services/vm_boot_source.py` | 统一内部 tag 生成入口 |
| Disk import 属性 | `rainbond-console/console/services/virtual_machine.py` | 属性持久化边界 |
| DataVolume 模板 | `rainbond/worker/appm/volume/vm_import.go` | registry URL 规范化与生成 |
| 卷创建错误链 | `rainbond/worker/appm/volume/share-file.go` | 将 Worker 防线传回部署流程 |
