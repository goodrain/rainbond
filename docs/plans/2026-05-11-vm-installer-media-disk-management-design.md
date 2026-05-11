# Rainbond VM 安装介质磁盘管理设计文档

## 一、项目背景
### 1.1 项目架构

本需求涉及 Rainbond 主平台三层链路：

```text
rainbond-ui
  ↓ /console/*
rainbond-console
  ↓ /v2/tenants/{tenant}/...
rainbond
  ↓ KubeVirt VirtualMachine / DataVolume
Kubernetes
```

当前 VM 创建链路中，ISO 安装介质在 `rainbond` worker 组装 KubeVirt 规格时，会被作为隐式 `vmimage` 光盘插入；而普通磁盘镜像（`qcow2/img/tar`）和导出镜像则分别走根盘镜像和 DataVolume 导入路径。

### 1.2 现有基础

- `rainbond-ui` 已有 VM 创建页与 VM 存储页，VM 存储页已支持 `/disk`、`/cdrom`、`/lun` 三类挂载类型。
- `rainbond-console` 已有 VM 运行时配置持久化能力，`vm_disk_layout` 已用于保存导出镜像恢复后的磁盘布局。
- `rainbond` worker 已有磁盘顺序组装逻辑，`applyVMDiskLayout()` 会根据 `vm_disk_layout` 分配数据盘与根盘的 `BootOrder`。
- `rainbond` worker 当前对 ISO 的处理仍为隐式逻辑：ISO 启动路径下总会自动追加 `vmimage` 光盘磁盘。

### 1.3 核心需求

- 在 VM 创建完成后的组件详情页“存储”中，显示完整 VM 磁盘清单，而不只是持久化存储卷。
- ISO 安装介质需要作为“光盘”磁盘项显示出来。
- 用户可以调整 VM 磁盘顺序。
- 用户可以删除安装完成后不再需要的 ISO 光盘。
- 保存后直接修改 VM 配置，效果在下次重启时生效。
- 不允许删除当前根启动盘。

## 二、用户旅程
### 2.1 用户操作流程

- 用户在创建 VM 时，选择 ISO、磁盘镜像或导出镜像作为来源。
- VM 创建成功后，用户进入组件详情页的“存储”页。
- 页面展示完整磁盘视图：系统盘、数据盘、光盘、LUN。
- 用户可拖拽或调整顺序。
- 若当前存在安装 ISO 光盘，用户可删除该光盘。
- 用户保存配置后，页面提示“下次重启生效”。
- 用户重启 VM 后，VM 按新的磁盘顺序启动；若已删除光盘，则不再挂载安装介质。

### 2.2 页面原型

- 组件详情页 `mnt` 存储页
  - 入口：组件详情 → 存储
  - 新增能力：
    - VM 磁盘列表视图
    - 排序操作
    - 删除安装介质光盘
    - 保存磁盘布局
- VM 配置概览页 `VMProfilePanel`
  - 可补充展示当前根启动盘、光盘状态、下次重启生效提示

### 2.3 外部系统交互

- 无新的第三方交互。
- 仍复用 KubeVirt VirtualMachine、CDI DataVolume、Multus 等现有集群能力。

## 三、整体架构设计
### 3.1 系统架构图

```text
VM 创建来源
  ├─ ISO
  ├─ qcow2/img/tar
  └─ VM 导出镜像
        ↓
console 保存运行时元数据
  ├─ vm_boot_source_format
  ├─ vm_disk_layout
  └─ vm_disk_imports
        ↓
worker 组装 VM 规格
  ├─ 根盘
  ├─ 数据盘
  ├─ LUN
  └─ 安装介质光盘（可显式保留/删除）
        ↓
下次重启生效
```

### 3.2 核心流程

1. 创建 VM 时，console 根据来源类型生成初始 `vm_disk_layout`。
2. 对 ISO 来源，初始布局中显式写入两类磁盘：
   - 根系统盘
   - `installer_media` 光盘
3. 组件详情页读取 VM 磁盘列表接口，展示完整布局。
4. 用户排序、删除光盘后，console 更新 `vm_disk_layout`。
5. `rainbond` worker 在重建 VirtualMachine 规格时：
   - 根据 `vm_disk_layout` 决定是否插入 `vmimage` 光盘
   - 根据布局决定所有磁盘 `BootOrder`
6. VM 重启后按新配置生效。

## 四、数据模型设计
### 4.1 新增数据库表

无新增表。

### 4.2 数据关系

继续复用 `component_k8s_attributes` 中的 VM 运行时扩展字段：

- `vm_boot_source_format`
- `vm_disk_layout`
- `vm_disk_imports`

`vm_disk_layout` 扩展为完整磁盘布局，包含以下字段：

```json
[
  {
    "disk_key": "disk",
    "disk_name": "system-disk",
    "disk_role": "root",
    "device_type": "disk",
    "source_kind": "volume",
    "order_index": 0,
    "boot": true
  },
  {
    "disk_key": "vmimage",
    "disk_name": "installer-media",
    "disk_role": "installer",
    "device_type": "cdrom",
    "source_kind": "installer_media",
    "order_index": 1,
    "boot": false
  }
]
```

说明：

- `TenantServiceVolume` 仍只表示真实存储卷，不新增“伪光盘卷”记录。
- 安装 ISO 光盘作为布局中的特殊磁盘项存在，不进入真实卷表。

## 五、API设计
### 5.1 接口列表

新增或扩展 console 接口：

- `GET /console/teams/{tenant}/apps/{serviceAlias}/vm-disks`
  - 返回统一 VM 磁盘清单
- `PUT /console/teams/{tenant}/apps/{serviceAlias}/vm-disks`
  - 保存 VM 磁盘顺序与删除状态

复用接口：

- `GET /console/teams/{tenant}/apps/{serviceAlias}/overview`
- `GET /console/teams/{tenant}/apps/{serviceAlias}/volumes`
- `POST /console/teams/{tenant}/apps/{serviceAlias}/restart`

region 层无新路由，仍通过更新 `component_k8s_attributes` 触发 VM 规格重建。

### 5.2 请求/响应结构

`GET vm-disks` 响应：

```json
{
  "list": [
    {
      "disk_key": "disk",
      "disk_name": "system-disk",
      "disk_role": "root",
      "device_type": "disk",
      "source_kind": "volume",
      "order_index": 0,
      "boot": true,
      "deletable": false
    },
    {
      "disk_key": "vmimage",
      "disk_name": "installer-media",
      "disk_role": "installer",
      "device_type": "cdrom",
      "source_kind": "installer_media",
      "order_index": 1,
      "boot": false,
      "deletable": true
    }
  ]
}
```

`PUT vm-disks` 请求：

```json
{
  "disks": [
    {
      "disk_key": "disk",
      "disk_role": "root",
      "device_type": "disk",
      "source_kind": "volume",
      "order_index": 0,
      "boot": true
    }
  ]
}
```

校验规则：

- 至少保留一个根启动盘
- 不允许删除根启动盘
- `installer_media` 允许删除
- 允许调整顺序，但 `order_index` 必须唯一

## 六、核心实现设计
### 6.1 关键逻辑

1. **初始布局生成**
   - ISO 创建路径下，console 在首次保存 VM 运行时配置时写入“根盘 + 安装光盘”布局。
   - qcow2/img 与导出镜像路径下，仅写真实磁盘布局。

2. **磁盘列表组装**
   - console 聚合：
     - `TenantServiceVolume`
     - `vm_disk_layout`
     - ISO 来源信息 / `vm_boot_source_format`
   - 对历史 ISO VM，如果布局中没有 `installer_media`，自动补出兼容视图。

3. **保存布局**
   - 用户保存后，console 直接覆盖 `vm_disk_layout`。
   - 不新增“删除光盘”专门字段，删除即表现为布局中移除 `installer_media`。

4. **worker 规格重建**
   - `rainbond` worker 不再简单地在 ISO 路径下总是调用 `appendISOInstallerDisk()`。
   - 改为：
     - 先读取扩展后的 `vm_disk_layout`
     - 若布局中存在 `installer_media`，则组装 `vmimage` `CDRom`
     - 若不存在，则不插入安装光盘
   - 所有磁盘 `BootOrder` 统一由布局驱动。

5. **兼容现有卷模型**
   - 真实数据盘、LUN、导出镜像导入盘仍走现有 `TenantServiceVolume + DataVolumeTemplate` 流程。
   - 仅安装 ISO 光盘走“特殊磁盘项”逻辑。

### 6.2 复用现有代码

- UI 复用 VM 存储页与 `AddOrEditVMVolume`
- console 复用 `virtual_machine.py` 中 `vm_disk_layout` 读写能力
- rainbond 复用：
  - `applyVMDiskLayout()`
  - `resolveVMBootPath()`
  - `appendISOInstallerDisk()`
  - `appendVMImageRootDisk()`
  - `buildVMVolumeSource()`

## 七、实施计划
### 跨层覆盖检查

- [x] Go (rainbond): 需要 — 扩展 `vm_disk_layout` 语义，按布局控制 ISO 光盘挂载与 BootOrder
- [x] Python (console): 需要 — 新增 VM 磁盘列表/保存接口，生成与持久化完整布局
- [x] React (rainbond-ui): 需要 — 在现有 VM 存储页展示完整磁盘清单，支持排序与删除光盘
- [x] Plugin: 不涉及

### Sprint 1: VM 磁盘布局后端建模

#### Task 1.1: console VM 磁盘清单建模
- 仓库：rainbond-console
- 文件：
  - `console/services/virtual_machine.py:420-832`
  - `console/views/app_overview.py:188-260`
- 实现内容：
  - 新增 VM 磁盘列表组装逻辑
  - 扩展 `vm_disk_layout` 为完整磁盘布局
  - 兼容历史 ISO VM 的 installer 光盘补全
- 验收标准：
  - ISO VM 能读出 `installer_media`
  - qcow2/img VM 不出现 installer 光盘
  - 导出镜像 VM 保留导入盘布局

#### Task 1.2: console 保存 VM 磁盘布局
- 仓库：rainbond-console
- 文件：
  - `console/views/app_overview.py`
  - `console/services/virtual_machine.py:444-619`
- 实现内容：
  - 增加保存接口
  - 校验根盘不可删、顺序唯一、至少保留一个可启动根盘
- 验收标准：
  - 删除 installer 光盘后，`vm_disk_layout` 中不再包含该项
  - 非法删除根盘请求返回 4xx

### Sprint 2: rainbond worker 按布局装配磁盘

#### Task 2.1: 扩展 VM 磁盘布局解析
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/vm_runtime.go:396-454`
  - `worker/appm/conversion/version.go:1436-1719`
- 实现内容：
  - 扩展布局模型字段
  - 让 installer 光盘参与布局解析
  - 用布局统一分配 boot order
- 验收标准：
  - 布局中存在 installer 光盘时，仍挂载 `vmimage`
  - 布局中无 installer 光盘时，不挂载 `vmimage`

#### Task 2.2: 回归导出镜像与普通磁盘镜像路径
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/version.go:176-236`
  - `worker/appm/volume/vm_import.go:92-215`
- 实现内容：
  - 确保 `vmimage-rootdisk` 与 `imported-rootdisk` 逻辑不被 installer 盘改动误伤
- 验收标准：
  - qcow2/img 根盘路径正常
  - 导出镜像 DataVolume 导入路径正常

### Sprint 3: UI 存储页磁盘视图

#### Task 3.1: VM 存储页显示完整磁盘清单
- 仓库：rainbond-ui
- 文件：
  - `src/pages/Component/mnt.js:300-380`
  - `src/services/app.js`
- 实现内容：
  - VM 存储页改为展示完整磁盘清单
  - 补充磁盘类型、来源、启动顺序、删除按钮
- 验收标准：
  - ISO VM 可看到光盘项
  - 根盘删除按钮禁用

#### Task 3.2: VM 存储页支持排序保存
- 仓库：rainbond-ui
- 文件：
  - `src/pages/Component/mnt.js`
  - `src/locales/zh-CN/component.js`
  - `src/locales/en-US/component.js`
- 实现内容：
  - 增加排序交互与保存动作
  - 提示“下次重启生效”
- 验收标准：
  - 调整顺序后保存成功
  - 删除光盘后保存成功

## 八、关键参考代码

| 功能 | 文件 | 说明 |
|------|------|------|
| ISO / 磁盘镜像启动路径判断 | `rainbond/worker/appm/conversion/version.go:1436-1460` | `iso-installer`、`vmimage-rootdisk`、`imported-rootdisk` 分支 |
| ISO 光盘隐式插入 | `rainbond/worker/appm/conversion/version.go:1516-1539` | 当前总是自动插入 `vmimage` 光盘 |
| 根盘镜像插入 | `rainbond/worker/appm/conversion/version.go:1541-1555` | 普通磁盘镜像根盘装配 |
| 磁盘布局排序 | `rainbond/worker/appm/conversion/vm_runtime.go:396-454` | `vm_disk_layout` 解析与 `BootOrder` 应用 |
| VM 导入盘模板 | `rainbond/worker/appm/volume/vm_import.go:92-215` | 导出镜像 / HTTP 导入盘装配 |
| VM 卷转磁盘 | `rainbond/worker/appm/volume/share-file.go:61-138` | `/disk`、`/lun`、`/cdrom` 到 KubeVirt 设备映射 |
| VM 运行时属性持久化 | `rainbond-console/console/services/virtual_machine.py:420-619` | `vm_disk_layout` 现有读写逻辑 |
| VM 存储页 | `rainbond-ui/src/pages/Component/mnt.js:300-380` | 现有 VM 存储展示与删除入口 |
