# 虚拟机镜像名称校验设计文档

## 一、项目背景

### 1.1 项目架构

Rainbond UI 通过 Console 创建虚拟机组件；Console 将镜像导入信息传给 Rainbond；Rainbond Worker 创建 KubeVirt DataVolume。

### 1.2 现有基础

URL 导入和上传镜像时，用户可填写 `image_name`。该值会进入内部 registry 引用的 tag 部分，但此前表单和创建接口均未按 OCI tag 规则校验。

### 1.3 核心需求

阻止 `DBServer(TongYong)` 这类包含括号的名称进入导入流程，避免 CDI importer 因无效镜像引用持续 CrashLoop。保持已有镜像选择流程兼容。

## 二、用户旅程

### 2.1 用户操作流程

1. 用户进入“创建组件 → 虚拟机”。
2. 用户选择 URL 导入或上传镜像并填写镜像名称。
3. 名称不符合规则时，表单立即提示且无法提交；合法名称按原流程创建。
4. 用户选择已有镜像时不新增名称限制。

### 2.2 页面原型

- 创建虚拟机表单 URL 导入区：镜像名称增加格式校验。
- 创建虚拟机表单上传区：镜像名称使用相同格式校验。
- 允许 1–128 位字母、数字、下划线、点和连字符，首字符仅允许字母、数字或下划线。

### 2.3 外部系统交互

不新增 webhook、回调、通知或第三方接口。

## 三、整体架构设计

### 3.1 系统架构图

```text
rainbond-ui 表单校验
  → rainbond-console 创建接口同规则兜底
    → rainbond Worker 校验完整 registry 引用
      → KubeVirt DataVolume
```

### 3.2 核心流程

前端负责即时反馈；Console 在创建镜像资产和组件前拒绝绕过前端的非法输入；Worker 在生成 DataVolume 前拒绝任何来源的非法 registry 引用。

## 四、数据模型设计

### 4.1 新增数据库表

无。

### 4.2 数据关系

无模型和字段变更。

## 五、API设计

### 5.1 接口列表

- 原接口：`POST /console/teams/{team_name}/apps/vm_run`
- 不新增接口，不改变成功响应结构。

### 5.2 请求/响应结构

- URL/上传请求的 `image_name` 必须匹配 `^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`。
- `event_id` 和 `vm_url` 必须为字符串。
- 非法输入返回 HTTP 400，且不创建镜像资产或组件。

## 六、核心实现设计

### 6.1 关键逻辑

1. UI 仅在 URL 和上传两个输入区增加相同正则规则；切换来源时清空上一个来源的镜像名称，避免残留值绕过校验。
2. Console 仅在存在 `event_id` 或 `vm_url` 的新导入流程校验镜像名称，保持已有镜像选择兼容。
3. Worker 校验规范化后的完整 registry 引用，非法时中止虚拟机定义转换。

### 6.2 复用现有代码

- UI 复用 Ant Design 3 Form rules。
- Console 复用 `VMRunCreateView.post` 创建入口。
- Rainbond 复用 `normalizeVMRegistryImportURL` 归一化入口。

## 七、实施计划

### 跨层覆盖检查

- [x] Go (rainbond)：需要 — DataVolume registry 引用最终校验。
- [x] Python (console)：需要 — 新导入镜像名称创建前校验。
- [x] React (rainbond-ui)：需要 — URL/上传镜像名称即时校验。
- [x] Plugin frontend (enterprise-base)：不涉及。
- [x] Plugin backend (plugin-template)：不涉及。

### Sprint 1: 最小校验闭环

#### Task 1.1: Worker 防御校验

- 仓库：rainbond
- 文件：`worker/appm/volume/vm_import.go`、`worker/appm/conversion/version.go`
- 实现内容：非法 registry 引用不生成 DataVolume，并向上返回错误。
- 验收标准：相关测试、`go build ./...`、`go vet ./...` 通过。

#### Task 1.2: Console 创建入口校验

- 仓库：rainbond-console
- 文件：`console/services/vm_boot_source.py`、`console/views/app_create/vm_run.py`
- 实现内容：新 URL/上传导入在任何持久化前校验 `image_name`。
- 验收标准：非法名称返回 400，资产和组件创建方法均未调用。

#### Task 1.3: UI 表单校验

- 仓库：rainbond-ui
- 文件：`src/components/ImageVirtualMachineForm/`
- 实现内容：URL/上传名称字段增加 OCI tag 规则和中英文提示。
- 验收标准：Node 回归测试和 `yarn build` 通过。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|------|------|------|
| UI 名称规则 | `rainbond-ui/src/components/ImageVirtualMachineForm/imageNameValidation.js` | 表单正则和切换来源清理 |
| Console 名称规则 | `rainbond-console/console/services/vm_boot_source.py` | 服务端同规则兜底 |
| Console 创建边界 | `rainbond-console/console/views/app_create/vm_run.py` | 持久化前返回 400 |
| Worker 引用校验 | `rainbond/worker/appm/volume/vm_import.go` | DataVolume 创建前最终校验 |
