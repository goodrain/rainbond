# VM Container Disk CD-ROM 设计文档

## 一、项目背景
### 1.1 项目架构

本需求涉及 Rainbond VM 存储管理链路：

```text
rainbond-ui 存储页
  ↓ /console/teams/{team}/apps/{app}/vm-disks
rainbond-console VM 磁盘布局服务
  ↓ component_k8s_attributes.vm_disk_layout
rainbond worker
  ↓ KubeVirt VirtualMachine volumes/disks
Kubernetes / KubeVirt
```

### 1.2 现有基础

- VM 存储页已有 `/disk`、`/cdrom`、`/lun` 三种挂载格式。
- `vm_disk_layout` 已能保存 VM 磁盘顺序、启动盘和安装介质光盘。
- ISO 安装介质已使用 KubeVirt `containerDisk` 挂载成 CD-ROM。
- VM 根盘/数据盘导入已有 DataVolume 和 registry 导入路径。

### 1.3 核心需求

- VM 存储新增时隐藏 LUN。
- 选择“光盘”时，不再填写容量和存储类型。
- 光盘来源使用 Docker/OCI 镜像地址，不使用 ISO URL，也不转换成 DataVolume。
- 保存后由 worker 将 OCI 镜像作为只读 CD-ROM 挂载到 VM。

## 二、用户旅程（MUST）
### 2.1 用户操作流程

- 用户进入 VM 组件详情页 → 存储。
- 点击新增。
- 挂载格式只看到“磁盘”和“光盘”，不显示 LUN。
- 选择“磁盘”时，继续填写容量和存储类型。
- 选择“光盘”时，只填写名称和镜像地址。
- 保存后回到磁盘列表，光盘来源显示为容器镜像。
- 用户点击保存布局，系统提示重启 VM 后生效。
- VM 重启后，KubeVirt 将该 OCI 镜像作为 CD-ROM 挂入虚拟机，用户可在 VM 内安装驱动。

### 2.2 页面原型

- 组件详情页存储 Tab
  - 新增抽屉：根据挂载格式切换字段。
  - 磁盘表格：显示容器镜像光盘的来源、镜像地址，容量和存储类型显示为空。

### 2.3 外部系统交互

- 读取用户填写的 Docker/OCI registry 镜像。
- 不新增 ISO 下载、转换、DataVolume 导入链路。

## 三、整体架构设计
### 3.1 系统架构图

```text
新增光盘表单
  ↓ name + image
console 保存 vm_disk_layout(container_disk)
  ↓
worker 解析 layout
  ↓
VirtualMachine.spec.template.spec.volumes[].containerDisk
VirtualMachine.spec.template.spec.domain.devices.disks[].cdrom
```

### 3.2 核心流程

1. UI 新增 VM 光盘时提交 `source_kind=container_disk`、`device_type=cdrom` 和 `image`。
2. console 校验镜像地址和磁盘布局，持久化到 `vm_disk_layout`。
3. worker 解析 `container_disk` 项，追加 KubeVirt `ContainerDisk` volume 和 `CDRom` disk。
4. `applyVMDiskLayout()` 继续统一分配 `BootOrder`。

## 四、数据模型设计
### 4.1 新增数据库表

不新增数据库表。

### 4.2 数据关系

复用 `component_k8s_attributes.vm_disk_layout`，新增 `container_disk` 来源：

```json
{
  "disk_key": "driver-media",
  "disk_name": "driver-media",
  "disk_role": "data",
  "device_type": "cdrom",
  "source_kind": "container_disk",
  "image": "registry.example.com/team/windows-driver:virtio",
  "order_index": 2,
  "boot": false
}
```

## 五、API设计
### 5.1 接口列表

- `GET /console/teams/{tenant}/apps/{serviceAlias}/vm-disks`
- `PUT /console/teams/{tenant}/apps/{serviceAlias}/vm-disks`

不新增 region API。

### 5.2 请求/响应结构

`PUT vm-disks` 中 `disks[]` 支持：

```json
{
  "disk_key": "driver-media",
  "disk_name": "driver-media",
  "disk_role": "data",
  "device_type": "cdrom",
  "source_kind": "container_disk",
  "image": "registry.example.com/team/windows-driver:virtio",
  "order_index": 2,
  "boot": false
}
```

校验规则：

- 容器镜像光盘必须是 `device_type=cdrom`。
- 容器镜像光盘必须提供非空 `image`。
- 容器镜像光盘可删除。
- 真实存储卷仍不能通过布局接口删除。
- 根盘仍必须存在。

## 六、核心实现设计
### 6.1 关键逻辑

- UI：`AddOrEditVMVolume` 根据挂载格式切换表单；`/cdrom` 走容器镜像光盘，不要求容量和存储类型。
- console：扩展 `vm_disk_layout` 规范化逻辑，保留 `image` 字段，允许 `container_disk` 项不对应 `TenantServiceVolume`。
- worker：扩展 `buildVMDiskLayout()` 和 VM 规格生成，按布局创建 `ContainerDisk` volume 和 `CDRom` disk。

### 6.2 复用现有代码

- 复用 VM 存储页布局保存按钮和排序逻辑。
- 复用 `applyVMDiskLayout()` 的 BootOrder 分配。
- 复用 `IMAGE_PULL_SECRET` 作为 containerDisk 的拉取凭据。

## 七、实施计划
### 跨层覆盖检查（MUST）

- [x] Go (rainbond): 需要 — worker 解析 `container_disk` 并生成 KubeVirt CD-ROM。
- [x] Python (console): 需要 — `vm_disk_layout` 校验、保存、返回容器镜像光盘。
- [x] React (rainbond-ui): 需要 — VM 存储新增表单切换和表格展示。
- [ ] Plugin frontend (enterprise-base): 不涉及。
- [ ] Plugin backend (plugin-template): 不涉及。

### Sprint 1: 后端运行时
#### Task 1.1: worker 支持 containerDisk 光盘
- 仓库：rainbond
- 文件：`worker/appm/conversion/vm_runtime.go`、`worker/appm/conversion/version.go`
- 实现内容：解析 `container_disk` 布局并生成 KubeVirt volume/disk。
- 验收标准：相关 Go 单测通过。

#### Task 1.2: console 支持布局保存
- 仓库：rainbond-console
- 文件：`console/services/virtual_machine.py`
- 实现内容：规范化并校验 `container_disk` 布局项。
- 验收标准：相关 Python 单测通过。

### Sprint 2: 前端体验
#### Task 2.1: VM 存储新增表单调整
- 仓库：rainbond-ui
- 文件：`src/components/AddOrEditVMVolume/index.js`、`src/components/AppCreateConfigPort/index.js`
- 实现内容：隐藏 LUN；光盘模式仅填写镜像地址。
- 验收标准：`yarn build` 通过。

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|------|------|------|
| VM 磁盘布局 | `rainbond-console/console/services/virtual_machine.py` | list/save/normalize VM disks |
| VM KubeVirt 规格 | `rainbond/worker/appm/conversion/version.go` | 构建 VM volumes/disks |
| VM 存储页 | `rainbond-ui/src/components/AppCreateConfigPort/index.js` | 存储表格和保存布局 |
| VM 新增存储抽屉 | `rainbond-ui/src/components/AddOrEditVMVolume/index.js` | 新增/编辑表单 |
