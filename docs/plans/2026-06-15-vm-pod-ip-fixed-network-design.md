# Rainbond 虚拟机 Pod IP 固定网络设计文档

## 一、项目背景
### 1.1 项目架构

本功能涉及 Rainbond 主平台三层链路：

```text
rainbond-ui (React)
  -> rainbond-console (Django)
  -> rainbond (Go)
  -> KubeVirt VirtualMachine / VMI Pod
  -> Calico IPAM
```

当前 Rainbond VM 能力已经接入 KubeVirt，并在组件概览中展示当前 Pod IP。现有 VM 网络默认由 `rainbond` worker 组装 KubeVirt `VirtualMachine`，再由 KubeVirt 创建 `virt-launcher` Pod。

### 1.2 现有基础

- `rainbond` VM runtime 当前默认生成 `pod` 网络 + `masquerade` 网卡，VM 内部 IP 与 Pod IP 不一致。
- `rainbond` 已有普通 Pod 的 Calico 固定 IP 注解路径：`cni.projectcalico.org/ipAddrs`。
- `rainbond-console` 已有 `get_vm_current_pod_ip()`，可从组件 Pod 列表中读取 VM 当前 Pod IP。
- `rainbond-ui` VM 概览页已展示当前 Pod IP 和网络信息。
- 旧设计曾考虑创建时填写固定 IP，但用户无法判断哪些 IP 可用，交互成本高且易冲突。

### 1.3 核心需求

- VM 内部 IP 直接使用 Pod IP，使 VM 在集群内表现为普通 Pod 网络端点。
- 创建 VM 时不让用户填写固定 IP，避免用户猜测可用 IP。
- VM 创建成功后，用户可打开“固定当前 IP”开关，平台将当前 Pod IP 固化为后续重启使用的固定 IP。
- 平台功能层支持 VM 与 Pod/组件之间的服务互通。
- 明确固定 IP 模式对热迁移、CPU/内存热更新、节点维护、克隆恢复等能力的影响。

## 二、用户旅程（MUST — 禁止跳过）
### 2.1 用户操作流程

1. 用户创建虚拟机。
   - 创建表单不展示固定 IP 输入框。
   - VM 使用 Pod 网络启动，Guest OS 通过 DHCP 获取 Pod IP。
2. 用户进入 VM 组件概览页。
   - 网络信息卡片展示当前 IP。
   - 展示“固定当前 IP”开关。
3. 用户打开“固定当前 IP”。
   - 页面提示：将固定当前 Pod IP，重启后生效；开启后不支持热迁移和 CPU/内存热更新。
   - 用户确认后，console 调用 region API。
   - region 读取当前 VMI/Pod IP，保存运行时属性，并在 VM template 写入 Calico 固定 IP 注解。
   - 平台提示用户重启 VM 使固定 IP 生效。
4. 用户重启 VM。
   - KubeVirt 创建新的 `virt-launcher` Pod。
   - Calico 根据 `cni.projectcalico.org/ipAddrs` 分配相同 Pod IP。
   - VM 内部 DHCP 获取同一个 IP。
5. 用户关闭“固定当前 IP”。
   - 平台清除固定 IP 运行时属性和 Calico 注解。
   - 平台释放 IP 保留记录。
   - 下次重启后 VM 恢复 Calico 自动分配 IP。

### 2.2 页面原型

- 创建 VM 页面：
  - 不展示固定 IP 输入项。
  - 不展示业务网络固定 IP配置，首版只走 Pod 网络。
- VM 概览页网络卡片：
  - 当前 IP：`10.42.x.y`
  - 固定当前 IP：开关
  - 固定状态：
    - 未固定：显示“当前 IP 将随 Pod 重建变化”
    - 已固定：显示固定 IP 和“重启后保持不变”
    - 待重启：显示“配置已更新，重启后生效”
  - 固定后限制提示：
    - 不支持热迁移
    - CPU/内存变更需重启生效
    - 节点维护时需要停机重建

### 2.3 外部系统交互

- KubeVirt：
  - VM 网络使用 `pod: {}`。
  - VM 接口使用 `bridge: {}`，让 Pod IPv4 委派给 VM 内部网卡。
  - 不使用 `masquerade`，避免 VM 内部 IP 与 Pod IP 不一致。
- Calico：
  - 固定 IP 通过 `cni.projectcalico.org/ipAddrs: '["10.42.x.y"]'` 注解实现。
  - 注解必须在 Pod 创建前存在，后加注解不会改变现有 Pod IP。
  - 通过 `IPReservation` 或平台侧保留记录避免固定 IP 被自动分配给其他 Pod。
- Kubernetes Service：
  - VM 声明端口后，Rainbond 继续创建组件 Service。
  - Pod/组件访问 VM 优先通过 Rainbond 服务名，必要时也可直接访问固定 IP。

## 三、整体架构设计
### 3.1 系统架构图

```text
创建 VM
  UI 不展示固定 IP
    -> Console 保存普通 VM runtime
      -> rainbond worker 生成 KubeVirt pod + bridge 网络
        -> Calico 自动分配 Pod IP
          -> VM guest DHCP 获取 Pod IP

开启固定当前 IP
  UI 点击固定开关
    -> Console fixed-ip API
      -> rainbond 查询当前 VMI/Pod IP
      -> 校验 IP 属于当前 VM 且可固定
      -> 保存 vm_fixed_ip_enabled / vm_fixed_ip
      -> 创建或更新 Calico IPReservation
      -> 更新 VirtualMachine template annotation
        -> 用户重启 VM
          -> 新 Pod 创建时 Calico 分配固定 IP
```

### 3.2 核心流程

1. Worker 默认 VM 网络从 `masquerade` 切换为 `bridge`。
2. VM guest 通过 DHCP 获得 Pod IP。
3. Console profile 继续展示当前 Pod IP，并增加固定 IP 状态。
4. 用户开启固定后，region 读取当前运行态 Pod IP，而不是接受用户输入。
5. Region 将当前 IP 写入组件 K8s 属性，并同步到 VM template 注解。
6. 固定 IP 开启后，能力层将 live migration、CPU hotplug、memory hotplug 标记为不可用或重启生效。
7. Service 互通继续复用 Rainbond 组件端口与 Service 机制。

## 四、数据模型设计
### 4.1 新增数据库表

首版不新增业务表，继续复用组件 K8s 属性。

建议新增属性：

- `vm_network_binding`
  - 值：`bridge`
  - 用于表达 VM 使用 Pod IP 委派模式，便于后续兼容其他绑定模式。
- `vm_fixed_ip_enabled`
  - 值：`true` / `false`
- `vm_fixed_ip`
  - 值：不带 CIDR 的 IPv4，例如 `10.42.247.130`
- `vm_fixed_ip_pending_restart`
  - 值：`true` / `false`
  - 表示 VM template 已变更，但当前运行 Pod 尚未重建。
- `vm_network_capability_mode`
  - 值：`pod_bridge_fixed_ip`
  - 便于 UI 和能力层判断热迁移/热更新限制。

### 4.2 数据关系

```text
TenantService
  -> component_k8s_attributes
      vm_network_binding=bridge
      vm_fixed_ip_enabled=true
      vm_fixed_ip=10.42.x.y
      vm_fixed_ip_pending_restart=true/false
  -> KubeVirt VirtualMachine
      spec.template.metadata.annotations["cni.projectcalico.org/ipAddrs"]
  -> Calico IPReservation
      reserved IP = vm_fixed_ip
```

删除 VM 或关闭固定 IP 时，需要同步清理：

- `vm_fixed_ip_enabled`
- `vm_fixed_ip`
- `vm_fixed_ip_pending_restart`
- Calico IPReservation
- VM template 上的 `cni.projectcalico.org/ipAddrs`

## 五、API设计
### 5.1 接口列表

#### 1. VM Profile 接口

现有接口：

```text
GET /console/teams/{tenant}/apps/{app}/overview/vm-profile
```

响应扩展：

```json
{
  "current_pod_ip": "10.42.247.130",
  "network": {
    "binding": "bridge",
    "fixed_ip_enabled": true,
    "fixed_ip": "10.42.247.130",
    "pending_restart": false,
    "live_migration_supported": false,
    "hotplug_supported": false
  }
}
```

#### 2. 开启固定当前 IP

新增 console API：

```text
PUT /console/teams/{tenant}/apps/{app}/vm-network/fixed-ip
```

请求：

```json
{
  "enabled": true
}
```

响应：

```json
{
  "fixed_ip": "10.42.247.130",
  "pending_restart": true,
  "message": "fixed ip will take effect after vm restart"
}
```

Console 调用 region API：

```text
PUT /v2/tenants/{tenant_name}/services/{service_alias}/vm-network/fixed-ip
```

#### 3. 关闭固定 IP

同一接口：

```json
{
  "enabled": false
}
```

响应：

```json
{
  "fixed_ip": "",
  "pending_restart": true,
  "message": "dynamic ip will take effect after vm restart"
}
```

### 5.2 请求/响应结构

Region 开启固定 IP 逻辑不接受客户端传入 IP。IP 来源必须是 region 侧实时查询得到的当前 VM Pod IP。

错误响应场景：

- VM 未运行：`409 vm is not running`
- 当前 Pod IP 为空：`409 vm pod ip is not ready`
- 当前 VM 使用非 bridge 网络：`409 vm network binding does not support fixed pod ip`
- Calico CRD 不存在：`409 calico ip reservation is not available`
- IPReservation 创建失败：`500 reserve fixed ip failure`
- VM template 更新失败：`500 update vm fixed ip annotation failure`

## 六、核心实现设计
### 6.1 关键逻辑

#### A. Worker 默认 VM 网络改为 pod + bridge

文件：

- `rainbond/worker/appm/conversion/vm_runtime.go`
- `rainbond/worker/appm/conversion/version.go`

设计：

- `buildVMRuntimeConfig` 默认输出：

```go
Networks: []kubevirtv1.Network{
  {Name: "default", NetworkSource: kubevirtv1.NetworkSource{Pod: &kubevirtv1.PodNetwork{}}},
}
Interfaces: []kubevirtv1.Interface{
  {Name: "default", InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{Bridge: &kubevirtv1.InterfaceBridge{}}},
}
```

- 不再默认使用 `Masquerade`。
- Guest OS 依赖 DHCP 获取 IP；Windows/Linux 均不注入静态地址。

#### B. 固定当前 IP 的 region handler

文件：

- `rainbond/api/controller/`
- `rainbond/api/handler/`
- `rainbond/api/api_routers/version2/v2Routers.go`

设计：

1. 根据 `service_alias` 查组件与 VM。
2. 查当前 service pods，选择 Running 的 virt-launcher Pod IP。
3. 校验该 Pod 属于当前 VM。
4. 保存组件 K8s 属性：
   - `vm_fixed_ip_enabled=true`
   - `vm_fixed_ip=<current_pod_ip>`
   - `vm_fixed_ip_pending_restart=true`
   - `vm_network_binding=bridge`
5. 创建或更新 Calico `IPReservation`。
6. 更新 VirtualMachine template annotation：

```json
{
  "cni.projectcalico.org/ipAddrs": "[\"10.42.x.y\"]"
}
```

#### C. 固定 IP 注解写入

文件：

- `rainbond/worker/appm/conversion/version.go:createPodAnnotations`

设计：

- VM 路径下读取 `vm_fixed_ip_enabled` 和 `vm_fixed_ip`。
- 当 `vm_fixed_ip_enabled=true` 且 `vm_fixed_ip` 非空时，写入：

```text
cni.projectcalico.org/ipAddrs=["<vm_fixed_ip>"]
```

- 普通 Pod 现有 `pod_ip` 注解逻辑保持不变。

#### D. IPReservation 管理

设计：

- 优先使用 Calico `IPReservation` CRD，名称建议：

```text
rainbond-vm-{namespace}-{service_alias}
```

- owner 信息写入 labels/annotations：
  - `rainbond.io/service-id`
  - `rainbond.io/service-alias`
  - `rainbond.io/tenant`
- 关闭固定 IP 或删除 VM 时删除对应 reservation。
- 如果集群不支持 `IPReservation`，首版建议返回明确错误，不降级为只写注解，避免 IP 被自动分配产生冲突。

#### E. Console 固定 IP 服务

文件：

- `rainbond-console/console/services/virtual_machine.py`
- `rainbond-console/console/views/app_overview.py`
- `rainbond-console/console/urls/__init__.py`
- `www/apiclient/regionapi.py`

设计：

- 增加 `set_vm_fixed_ip_enabled(tenant, service, enabled)`。
- 开启时不接受 IP 参数，只调用 region 固定当前 IP。
- profile 响应中合并运行时属性和 region 返回的当前 Pod IP。
- 根据 `vm_fixed_ip` 与 `current_pod_ip` 判断 `pending_restart`：
  - 固定启用但两者不一致：`pending_restart=true`
  - 固定启用且两者一致：`pending_restart=false`

#### F. UI 固定 IP 交互

文件：

- `rainbond-ui/src/pages/Component/component/Basic/VMProfilePanel.js`
- `rainbond-ui/src/services/app.js`
- `rainbond-ui/src/locales/zh-CN/component.js`
- `rainbond-ui/src/locales/en-US/component.js`

设计：

- 网络卡片新增 Switch：
  - Label：固定当前 IP
  - 开启前弹确认框，说明会限制热迁移和热更新。
  - 开启成功后展示“重启后生效”。
- 固定后展示：
  - 固定 IP
  - 当前 IP
  - 生效状态
- 如果 VM 未运行或无当前 IP，Switch 禁用。

#### G. 热迁移与热更新能力降级

设计规则：

- `pod + bridge` 网络模式下，不支持 KubeVirt live migration。
- 固定当前 IP 开启后：
  - live migration 禁用。
  - CPU hotplug 禁用，资源变更提示重启生效。
  - memory hotplug 禁用，资源变更提示重启生效。
  - 节点维护不能无感迁移，只能停机重建。
- 若后续需要保留 live migration，应另行设计 `masquerade + Service` 模式，但该模式不满足“VM 内部 IP 等于 Pod IP”。

#### H. Pod/VM 服务互通

设计：

- VM 组件端口继续走 Rainbond Service。
- Service selector 必须稳定匹配 VM `virt-launcher` Pod。
- 组件依赖关系仍使用服务名和端口，不要求用户直接使用 Pod IP。
- 固定 IP 只作为需要裸 IP 访问、协议发现、VM 内服务注册等场景的补充能力。

### 6.2 复用现有代码

- 复用 `get_vm_current_pod_ip()` 获取当前 VM Pod IP。
- 复用 `VMProfilePanel` 网络信息区域展示当前 IP。
- 复用 `component_k8s_attributes` 保存 VM 运行时属性。
- 复用 `syncVirtualMachineSpecForService` 将属性变化同步到 VM spec。
- 复用普通 Pod 的 Calico 注解格式，扩展 VM 来源。

## 七、实施计划
### 跨层覆盖检查（MUST）

- [x] Go (rainbond): 需要 — VM 网络默认 bridge、固定当前 IP region API、Calico 注解、IPReservation、能力降级。
- [x] Python (console): 需要 — 固定 IP 开关 API、profile 响应扩展、region client 调用。
- [x] React (rainbond-ui): 需要 — VM 概览网络卡片开关、状态展示、限制提示。
- [x] Plugin frontend (enterprise-base): 不涉及 — 除非 VM 管理页迁入插件。
- [x] Plugin backend (plugin-template): 不涉及。

### Sprint 1: VM Pod IP 网络基线

#### Task 1.1: Worker 默认 VM 网络切换为 bridge
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/vm_runtime.go`
  - `worker/appm/conversion/vm_runtime_test.go`
- 实现内容：
  - 默认接口从 `Masquerade` 改为 `Bridge`。
  - 保持 `NetworkSource.Pod`。
  - 增加单测覆盖默认 VM 网络为 `pod + bridge`。
- 验收标准：
  - VM 内部 DHCP 获取 Pod IP。
  - `go test ./worker/appm/conversion` 通过。

#### Task 1.2: 校验 VM Service 互通不回退
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/service.go`
  - `api/handler/service.go`
- 实现内容：
  - 确认 VM labels/selector 可稳定匹配 virt-launcher Pod。
  - 确认组件端口生成 Service 不依赖 masquerade。
- 验收标准：
  - Pod 可通过 Service 访问 VM 声明端口。

### Sprint 2: 固定当前 IP API

#### Task 2.1: Region 增加固定 IP handler
- 仓库：rainbond
- 文件：
  - `api/controller/`
  - `api/handler/`
  - `api/api_routers/version2/v2Routers.go`
  - `api/handler/k8s_attribute.go`
- 实现内容：
  - 增加开启/关闭固定当前 IP API。
  - 开启时读取当前 Running Pod IP。
  - 保存 VM 固定 IP 属性。
  - 同步 VM template annotation。
- 验收标准：
  - VM 未运行、无 Pod IP、非 VM 组件均返回明确错误。
  - Go 单测覆盖开启/关闭和异常分支。

#### Task 2.2: Calico IPReservation 管理
- 仓库：rainbond
- 文件：
  - `api/handler/`
  - `api/controller/`
- 实现内容：
  - 通过 dynamic client 创建/删除 `ipreservations.crd.projectcalico.org`。
  - 不支持 IPReservation 时返回错误。
  - 删除 VM 或关闭固定 IP 时释放 reservation。
- 验收标准：
  - 重启 VM 后 IP 保持不变。
  - 其他 Pod 不会自动拿到该固定 IP。

#### Task 2.3: VM 注解生成
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/version.go`
  - `worker/appm/conversion/*_test.go`
- 实现内容：
  - `createPodAnnotations` 在 VM 固定 IP 启用时写入 `ipAddrs`。
  - 普通 Pod `pod_ip` 行为不变。
- 验收标准：
  - Go 单测覆盖 VM 固定 IP 注解。

### Sprint 3: Console 与 UI

#### Task 3.1: Console API
- 仓库：rainbond-console
- 文件：
  - `console/services/virtual_machine.py`
  - `console/views/app_overview.py`
  - `console/urls/__init__.py`
  - `www/apiclient/regionapi.py`
- 实现内容：
  - 增加固定 IP 开关接口。
  - 扩展 VM profile 网络字段。
  - 通过 region API 获取固定结果。
- 验收标准：
  - Python 单测覆盖 profile 字段和开关接口。

#### Task 3.2: UI 网络卡片
- 仓库：rainbond-ui
- 文件：
  - `src/pages/Component/component/Basic/VMProfilePanel.js`
  - `src/services/app.js`
  - `src/locales/zh-CN/component.js`
  - `src/locales/en-US/component.js`
- 实现内容：
  - 展示固定当前 IP 开关。
  - 展示固定 IP、当前 IP、待重启状态。
  - 开启确认中说明热迁移和热更新限制。
- 验收标准：
  - `yarn build` 通过。
  - VM 未运行时开关禁用。

### Sprint 4: 能力限制与回归

#### Task 4.1: 热迁移/热更新能力降级
- 仓库：rainbond / rainbond-console / rainbond-ui
- 文件：
  - VM resource live update 相关 handler/service
  - VM profile / resource edit UI
- 实现内容：
  - 固定当前 IP 开启后，CPU/内存变更改为重启生效。
  - 隐藏或禁用 live migration 入口。
- 验收标准：
  - 固定 IP VM 不触发 live migration。
  - 资源变更提示准确。

#### Task 4.2: 验证
- 仓库：rainbond / rainbond-console / rainbond-ui
- 文件：无新增
- 实现内容：
  - `go test ./...`
  - `go vet ./...`
  - `go build ./...`
  - console 相关 pytest
  - `yarn build`
- 验收标准：
  - 所有质量门控通过。

## 八、关键参考代码
| 功能 | 文件 | 说明 |
|------|------|------|
| VM runtime 网络生成 | `worker/appm/conversion/vm_runtime.go` | 当前默认 `pod + masquerade`，需改为 `pod + bridge` |
| VM spec 组装 | `worker/appm/conversion/version.go` | 写入 VMI template annotations/networks/interfaces |
| Pod Calico 固定 IP 注解 | `worker/appm/conversion/version.go:createPodAnnotations` | 已有普通 Pod `pod_ip` 注解逻辑 |
| VM spec 同步 | `api/handler/k8s_attribute.go` | 属性变化后同步到 KubeVirt VM |
| Service Pod 查询 | `api/handler/service.go:GetPods` | Console 当前通过该链路获取 VM 当前 Pod IP |
| VM 当前 Pod IP | `rainbond-console/console/services/virtual_machine.py:get_vm_current_pod_ip` | 固定当前 IP 的来源 |
| VM Profile | `rainbond-console/console/views/app_overview.py:AppVMProfileView` | 需要扩展网络字段 |
| VM 概览网络卡片 | `rainbond-ui/src/pages/Component/component/Basic/VMProfilePanel.js` | 展示当前 IP 与固定开关 |

## 九、风险与决策记录

- 决策：首版不让用户创建时填写固定 IP，只支持创建后固定当前 IP。
- 决策：固定 IP 依赖 Calico `IPReservation`，不支持时不降级为裸注解。
- 决策：为了满足 VM 内部 IP 等于 Pod IP，必须使用 KubeVirt `bridge` 绑定，并接受不支持 live migration 的限制。
- 风险：Guest OS 如果禁用 DHCP，VM 内部无法拿到 Pod IP。
- 风险：从旧 `masquerade` VM 升级到 `bridge` VM 会改变网络语义，需提供版本兼容策略或仅对新建 VM 生效。
- 风险：备份、克隆、恢复到同集群时不能复制固定 IP，需要恢复流程重新确认或清除固定 IP。
