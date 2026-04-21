# Rainbond 虚拟机固定 IP 回退设计文档

## 一、项目背景
### 1.1 项目架构

本功能涉及 Rainbond 主平台三层链路：

```text
rainbond-ui (React)
  -> rainbond-console (Django)
  -> rainbond (Go)
  -> KubeVirt + CNI
```

虚拟机创建页当前通过 `network_mode=fixed` 触发固定 IP 模式，由 `rainbond-console` 保存运行时配置，再由 `rainbond` worker 读取组件 K8s 属性生成 KubeVirt `VirtualMachine`。

### 1.2 现有基础

- `rainbond-ui` 已有“随机 IP / 固定 IP”单选，但固定 IP 仅在检测到业务网络资源时才可用，并且始终强制填写业务网络。
- `rainbond-console` 当前要求 `network_mode=fixed` 时必须同时提供 `network_name` 和 `fixed_ip`。
- `rainbond` 当前仅支持“业务网络固定 IP”路径：
  - `vm_network_name` 必填
  - KubeVirt 网络为 `multus + bridge`
  - Linux 用 cloud-init 注入固定 IP
  - Windows 用 sysprep 注入固定 IP
- `rainbond` 已支持通过 `cni.projectcalico.org/ipAddrs` 注解给普通 Pod 指定 Pod IP，但 VM 固定 IP 路径未复用该能力。

### 1.3 核心需求

- 当集群没有业务网络资源时，虚拟机仍然可以选择“固定 IP”。
- 无业务网络资源时，固定 IP 直接走 Pod 固定 IP 语义，生成效果参考用户提供的 YAML：
  - VM 网络仍使用 `pod: {}`
  - 接口改用 `bridge: {}`
  - 通过 `cni.projectcalico.org/ipAddrs` 指定 Pod IP
- 有业务网络资源时，继续支持当前“业务网络 + 固定 IP”形态。
- 页面交互需根据能力动态变化：
  - 无资源：固定 IP 只显示 IP 输入项
  - 有资源：固定 IP 显示业务网络和 IP 输入项

## 二、用户旅程（MUST — 禁止跳过）
### 2.1 用户操作流程

1. 用户进入虚拟机创建页，选择“固定 IP”。
2. 若集群未发现业务网络资源：
   - 页面只显示固定 IP 输入框。
   - 用户填写固定 IP 后提交。
   - 后端按 Pod 固定 IP 路径创建虚拟机。
3. 若集群存在业务网络资源：
   - 页面显示业务网络下拉框和固定 IP 输入框。
   - 用户选择业务网络并填写 IP。
   - 后端按现有 Multus 固定网卡路径创建虚拟机。
4. 用户在组件基础信息页仍可查看运行时配置：
   - `network_mode=fixed`
   - `network_name` 可能为空
   - `fixed_ip` 始终保留

### 2.2 页面原型

- 页面：`rainbond-ui/src/components/ImageVirtualMachineForm/index.js`
  - 入口：创建虚拟机页
  - 交互：
    - 固定 IP 单选始终可用
    - `vmCapabilities.networks.length === 0` 时隐藏业务网络字段
    - 仅在存在业务网络资源时展示 `network_name`
    - `gateway`、`dns_servers` 继续仅在业务网络路径下展示
- 页面：组件概览 VM 配置展示页
  - 无新页面
  - 继续复用现有字段展示逻辑

### 2.3 外部系统交互

- Calico：通过 `cni.projectcalico.org/ipAddrs` 注解分配 Pod IP
- KubeVirt：
  - Pod 固定 IP：`pod` 网络 + `bridge` 接口
  - 业务网络固定 IP：`multus` 网络 + `bridge` 接口
- NetworkAttachmentDefinition：
  - 仅在“业务网络固定 IP”路径下需要
  - 不再作为“固定 IP”能力的前置条件

## 三、整体架构设计
### 3.1 系统架构图

```text
UI:
  fixed mode selected
    -> if networks empty: submit fixed_ip only
    -> if networks exist: submit network_name + fixed_ip (+ optional gateway/dns)

Console:
  validate fixed_ip
  network_name becomes optional
  persist vm_network_mode / vm_network_name / vm_fixed_ip

Worker:
  if fixed + network_name empty:
    use pod network + bridge interface
    annotate cni.projectcalico.org/ipAddrs with fixed_ip host part
  if fixed + network_name present:
    keep existing multus + guest network injection flow
```

### 3.2 核心流程

1. UI 获取 VM capabilities。
2. `rainbond` capabilities 接口始终声明 `fixed` 模式可用。
3. UI 根据 `networks` 列表是否为空切换固定 IP 表单结构。
4. `rainbond-console` 保存运行时配置时仅强制校验 `fixed_ip`。
5. `rainbond` worker 读取 VM 运行时属性：
   - `network_name` 为空：构建 Pod 固定 IP 运行时配置
   - `network_name` 非空：沿用 Multus 固定网卡逻辑
6. `createPodAnnotations` 在 VM Pod 固定 IP 场景写入 Calico 注解。

## 四、数据模型设计
### 4.1 新增数据库表

无新增表。

### 4.2 数据关系

继续复用组件 K8s 属性：

- `vm_network_mode`
- `vm_network_name`
- `vm_fixed_ip`
- `vm_gateway`
- `vm_dns_servers`

变更点：

- `vm_network_name` 在 `vm_network_mode=fixed` 时由“必填”改为“可空，表示走 Pod 固定 IP”。
- `vm_fixed_ip` 仍然必填。

## 五、API设计
### 5.1 接口列表

1. VM capabilities 接口
   - 文件：`rainbond/api/handler/vm_capability.go`
   - 变更：`network_modes` 始终包含 `fixed`
   - `networks` 为空时表示仅支持 Pod 固定 IP

2. VM 创建接口
   - 文件：`rainbond-console/console/views/app_create/vm_run.py`
   - 请求字段不变：
     - `network_mode`
     - `network_name`
     - `fixed_ip`
     - `gateway`
     - `dns_servers`
   - 语义变更：
     - `network_mode=fixed` + `network_name=""` => Pod 固定 IP
     - `network_mode=fixed` + `network_name!="”` => 业务网络固定 IP

### 5.2 请求/响应结构

请求示例一：无业务网络资源

```json
{
  "network_mode": "fixed",
  "network_name": "",
  "fixed_ip": "10.42.124.90/24"
}
```

请求示例二：有业务网络资源

```json
{
  "network_mode": "fixed",
  "network_name": "default/bridge-test",
  "fixed_ip": "10.250.250.10/24",
  "gateway": "10.250.250.1",
  "dns_servers": "223.5.5.5,8.8.8.8"
}
```

## 六、核心实现设计
### 6.1 关键逻辑

#### A. Worker 增加 Pod 固定 IP 分支

文件：

- `rainbond/worker/appm/conversion/vm_runtime.go`
- `rainbond/worker/appm/conversion/version.go`

设计：

- `buildVMRuntimeConfig` 在 `network_mode=fixed` 时先校验 `fixed_ip`。
- 若 `network_name` 为空：
  - 返回：
    - `Networks = [{ name: default, pod: {} }]`
    - `Interfaces = [{ name: default, bridge: {} }]`
  - 不创建 cloud-init / sysprep 网络注入卷
  - 将 `fixed_ip` 视为 Pod 固定 IP 来源
- 若 `network_name` 非空：
  - 保持现有 `multus + bridge + guest network injection`

#### B. Calico 注解写入 VM 固定 IP

文件：

- `rainbond/worker/appm/conversion/version.go:1322-1342`

设计：

- `createPodAnnotations` 在普通 Pod 的 `pod_ip` 逻辑之外，增加 VM 固定 IP 推导：
  - 条件：`vm_network_mode=fixed` 且 `vm_network_name` 为空
  - 注解值从 `vm_fixed_ip` 提取主机 IP 部分
  - 例如 `10.42.124.90/24` -> `["10.42.124.90"]`
- 为了让注解阶段拿到 VM 运行时属性，需要在 VM 路径下更早 hydrate VM 运行时配置，或让注解逻辑读取同一组属性。

#### C. 补齐 VM 运行时属性 hydration

文件：

- `rainbond/worker/appm/conversion/version.go:1356-1368`

设计：

- 将 `vm_gateway`、`vm_dns_servers` 纳入 VM 运行时属性 hydration 列表，保持 worker 读取逻辑与 console 持久化逻辑对齐。

#### D. Console 放宽校验

文件：

- `rainbond-console/console/services/virtual_machine.py:601-621`
- `rainbond-console/console/services/virtual_machine.py:812-821`

设计：

- `validate_vm_runtime_config` 中删除 `fixed network requires network_name` 校验。
- `fixed_ip` 仍为固定 IP 模式必填。
- 属性落库逻辑保持不变，允许 `vm_network_name=""`。

#### E. UI 按资源数量切换交互

文件：

- `rainbond-ui/src/components/ImageVirtualMachineForm/index.js:989-1062`
- `rainbond/api/handler/vm_capability.go:47-66`

设计：

- capabilities 接口始终返回 `network_modes=["random","fixed"]`
- 表单逻辑改为：
  - 固定 IP 单选不再因 `networks` 为空而禁用
  - `networks.length === 0`：
    - 固定 IP 区域仅显示 `fixed_ip`
    - 自动清空并隐藏 `network_name/gateway/dns_servers`
  - `networks.length > 0`：
    - 显示 `network_name`
    - 继续显示 `gateway/dns_servers`

### 6.2 复用现有代码

- 复用现有 VM 运行时属性持久化与同步能力，不新增接口字段。
- 复用现有 Calico Pod IP 注解路径，只扩展其来源。
- 复用现有 Multus 固定网卡逻辑，不改变已上线路径。
- 复用现有 VM Profile 展示字段，不新增展示协议。

## 七、实施计划
### 跨层覆盖检查（MUST）

- [x] Go (rainbond): 需要 — VM runtime 分支、Calico 注解分支、capabilities 调整、Go 单测
- [x] Python (console): 需要 — 运行时校验放宽、Python 单测
- [x] React (rainbond-ui): 需要 — 固定 IP 表单按能力动态切换、前端构建验证
- [x] Plugin: 不涉及

### Sprint 1: 固定 IP 行为打通

#### Task 1.1: Go 侧增加 Pod 固定 IP 回退
- 仓库：rainbond
- 文件：
  - `worker/appm/conversion/vm_runtime.go:58-164`
  - `worker/appm/conversion/version.go:1322-1378`
  - `worker/appm/conversion/vm_runtime_test.go`
- 实现内容：
  - 允许 `fixed + empty network_name`
  - 输出 `pod + bridge` 运行时配置
  - 从 `vm_fixed_ip` 推导 Calico 注解
  - 增加 host IP 提取和注解测试
- 验收标准：
  - Go 单测覆盖两种固定 IP 路径
  - `go test ./worker/appm/conversion`

#### Task 1.2: Console 侧放宽固定 IP 校验
- 仓库：rainbond-console
- 文件：
  - `console/services/virtual_machine.py:601-621`
  - `console/tests/virtual_machine_service_test.py`
  - `console/tests/vm_create_flow_regression_test.py`
- 实现内容：
  - 去掉固定 IP 对 `network_name` 的强依赖
  - 增加“固定 IP 但无业务网络名”的测试
- 验收标准：
  - Django 单测通过

#### Task 1.3: UI 侧切换交互
- 仓库：rainbond-ui
- 文件：
  - `src/components/ImageVirtualMachineForm/index.js:989-1062`
  - `src/locales/zh-CN/team.js`
  - `src/locales/en-US/team.js`
- 实现内容：
  - 固定 IP 单选始终可选
  - 无网络资源时只展示 IP 输入
  - 有网络资源时展示业务网络 + IP + gateway + dns
  - 必要时补充提示文案
- 验收标准：
  - `yarn build` 通过
  - 页面行为符合用户描述

### Sprint 2: 端到端回归验证

#### Task 2.1: 跨仓验证
- 仓库：rainbond / rainbond-console / rainbond-ui
- 文件：无新增
- 实现内容：
  - 分仓运行测试和构建
  - 检查接口字段兼容性
  - 确认无网络资源和有网络资源两种表单分支都能提交
- 验收标准：
  - `go test`, `go build`, `go vet`, Django 测试、`yarn build` 全部通过

## 八、关键参考代码
| 功能 | 文件 | 说明 |
|------|------|------|
| VM 运行时拼装 | `worker/appm/conversion/vm_runtime.go` | 固定 IP 当前只支持 Multus |
| VM Pod 注解 | `worker/appm/conversion/version.go` | 已有普通 Pod 的 Calico 固定 IP 注解 |
| VM capabilities | `api/handler/vm_capability.go` | 当前按业务网络资源决定是否暴露 fixed |
| VM 运行时校验 | `../rainbond-console/console/services/virtual_machine.py` | 固定 IP 当前强依赖 network_name |
| 创建页交互 | `../rainbond-ui/src/components/ImageVirtualMachineForm/index.js` | 固定 IP 当前始终要求业务网络 |
