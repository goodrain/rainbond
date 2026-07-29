# 组件视图 Kubernetes 属性 cmd/args YAML 化设计文档

## 一、项目背景
### 1.1 项目架构
Rainbond 组件 Kubernetes 属性由三层协作完成：

```text
rainbond-ui 组件视图 Kubernetes 属性页
    -> rainbond-console /console/teams/{team}/components/{component}/k8s-attributes
    -> rainbond /v2/tenants/{tenant}/services/{service}/k8s-attributes
    -> worker 转换为 Kubernetes Container.Command / Container.Args
```

### 1.2 现有基础
- `rainbond` 已有 `component_k8s_attributes` 通用表，字段包含 `name`、`save_type`、`attribute_value`。
- `rainbond` 已定义 `cmd` 和 `args` 属性名，并在 `worker/appm/conversion/version.go` 中应用到容器 `Command` 和 `Args`。
- `rainbond-console` 通过 `console.services.k8s_attribute` 保存 console 侧记录并同步到 region。
- `rainbond-ui` 的组件 Kubernetes 属性页已支持 JSON、YAML、string、boolean 等编辑模式。

### 1.3 核心需求
当前 `cmd` 被 UI 当作字符串输入，region worker 在解析失败时使用 `strings.Split(value, " ")` 兜底，导致包含空格或 shell 结构的一行命令被错误拆分，例如：

```text
/entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done
```

目标是支持 `cmd` 和 `args` 以 YAML 数组格式录入、存储和解析，避免任何空格切割。

## 二、用户旅程（MUST — 禁止跳过）
### 2.1 用户操作流程
- 用户如何配置/触发该功能？（必须回答，若需要 UI 则列出页面）
  用户进入组件详情 -> Kubernetes 属性视图，点击添加属性，选择 `cmd` 或 `args`，在 YAML 编辑器中录入数组。
- 用户如何查看状态/结果？
  Kubernetes 属性列表中 `cmd` 和 `args` 按 YAML 类型展示，用户点击编辑可看到原 YAML 内容。
- 管理员/审批人如何操作？
  不涉及额外管理员或审批流程，沿用现有组件配置权限。

### 2.2 页面原型
- 页面：组件详情 -> Kubernetes 属性视图。
- 弹窗/抽屉：添加/编辑 Kubernetes 属性抽屉。
- 表单交互：
  - `cmd` 示例：
    ```yaml
    - /entrypoint.sh
    ```
  - `args` 示例：
    ```yaml
    - /entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done
    ```
  - 提示文案说明每一行 `- ...` 是 Kubernetes argv 的一个元素，不会按空格切割。

### 2.3 外部系统交互
不涉及 webhook、回调、通知或第三方集成。最终变更体现在 Kubernetes Pod template 的 `containers[].command` 和 `containers[].args` 字段。

## 三、整体架构设计
### 3.1 系统架构图
```text
UI YAML editor
  saves { name: "cmd"|"args", save_type: "yaml", attribute_value: "- ..." }
      |
Console k8s_attribute service
  stores YAML string and forwards unchanged
      |
Rainbond region API
  stores save_type/attribute_value
      |
Worker conversion
  parses YAML/JSON sequence into []string
      |
Kubernetes Container.Command / Container.Args
```

### 3.2 核心流程
1. UI 将 `cmd` 和 `args` 纳入 YAML 字段列表，并从 string 字段中移除 `cmd`。
2. UI 添加 `args` 到可选 Kubernetes 属性列表，并更新 `cmd/args` 提示示例。
3. console 对 `save_type=yaml` 保持原文存储和透传，不做空格拆分。
4. region worker 对 `cmd/args` 只解析 YAML/JSON sequence；旧 `save_type=string` 或无法解析的历史值整体作为单个 argv 元素。
5. compose 检测等内部来源继续可用，但优先保存为 YAML sequence，保证与 UI 契约一致。

## 四、数据模型设计
### 4.1 新增数据库表
不新增数据库表。

### 4.2 数据关系
继续使用 `component_k8s_attributes`：

```text
tenant_id
component_id
name            cmd / args
save_type       yaml
attribute_value YAML sequence string
```

旧数据兼容规则：
- JSON 数组旧值继续按数组解析。
- YAML 数组新值按数组解析。
- 无法解析为数组的历史字符串不再空格切割，整体作为一个参数。

## 五、API设计
### 5.1 接口列表
沿用现有接口，不新增路由：
- `GET /console/teams/{team_name}/components/{service_alias}/k8s-attributes`
- `POST /console/teams/{team_name}/components/{service_alias}/k8s-attributes`
- `PUT /console/teams/{team_name}/components/{service_alias}/k8s-attributes/{name}`
- region 对应 `/v2/tenants/{tenant_name}/services/{service_alias}/k8s-attributes`

### 5.2 请求/响应结构
新增推荐请求格式：

```json
{
  "attribute": {
    "name": "args",
    "save_type": "yaml",
    "attribute_value": "- /entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done\n"
  }
}
```

`cmd` 同样使用 `save_type=yaml`，值必须表达字符串数组。

## 六、核心实现设计
### 6.1 关键逻辑
- `rainbond`：
  - 新增或提取 `parseStringSequenceAttribute` 类辅助逻辑，按 YAML/JSON 数组解析 `cmd/args`。
  - 当解析结果不是字符串数组时返回错误，避免静默生成错误 Pod spec。
  - 对旧字符串值整体保留为 `[]string{value}`，不再使用 `strings.Split(value, " ")`。
- `rainbond-console`：
  - `app_check_service` 中自动保存 compose `command/args` 时改为 YAML sequence，`save_type=yaml`。
  - `k8s_attribute_service` 对用户提交的 `cmd/args` 做轻量校验：必须是 YAML/JSON 数组，且每项是字符串。
- `rainbond-ui`：
  - `DEFAULT_ATTRIBUTE_FIELDS` 增加 `args`。
  - `YAML_FIELDS` 增加 `cmd`、`args`；`STRING_FIELDS` 移除 `cmd`。
  - 更新 `TooltipValueArr` 和输入提示文案，突出“不按空格切割”。

### 6.2 复用现有代码
- 复用现有 `CodeMirrorForm` YAML 编辑器。
- 复用现有 `save_type=yaml` 存储和展示逻辑。
- 复用现有 console -> region API 透传链路。

## 七、实施计划
### 跨层覆盖检查（MUST）
- [ ] Go (rainbond): 需要 — worker 解析 `cmd/args` YAML/JSON sequence，禁止空格拆分 fallback，补 Go 测试。
- [ ] Python (console): 需要 — 校验并保存 `cmd/args` YAML sequence，内部检测来源也改为 YAML 存储，补 Python 测试。
- [ ] React (rainbond-ui): 需要 — 组件 Kubernetes 属性页增加 `args`，把 `cmd/args` 切为 YAML 编辑并更新提示。
- [ ] Plugin frontend (enterprise-base): 不涉及 — 该功能在主平台组件视图。
- [ ] Plugin backend (plugin-template): 不涉及 — 无插件 API 变更。

### Sprint 1: Region 解析契约
#### Task 1.1: 补充 cmd/args 解析测试
- 仓库：rainbond
- 文件：`worker/appm/conversion/version_test.go` 或新增聚焦测试文件
- 实现内容：覆盖 YAML 数组、JSON 数组、单个 shell 字符串 fallback。
- 验收标准：旧空格切割场景失败后修复通过。

#### Task 1.2: 实现 YAML/JSON sequence 解析
- 仓库：rainbond
- 文件：`worker/appm/conversion/version.go`
- 实现内容：替换 `strings.Split` fallback，解析失败时整体作为单元素。
- 验收标准：`go test ./worker/appm/conversion -run CmdArgs -v` 通过。

### Sprint 2: Console 保存与校验
#### Task 2.1: 补充 k8s attribute service 测试
- 仓库：rainbond-console
- 文件：`console/tests/k8s_attribute_service_test.py`
- 实现内容：验证 `cmd/args` YAML 数组保存透传，非数组报错。
- 验收标准：相关 pytest 通过。

#### Task 2.2: 实现 console 校验和 compose 保存格式
- 仓库：rainbond-console
- 文件：`console/services/k8s_attribute.py`、`console/services/app_check_service.py`
- 实现内容：`cmd/args` 入库前规范为 YAML sequence，内部检测保存使用 `save_type=yaml`。
- 验收标准：相关 pytest 通过。

### Sprint 3: UI 编辑体验
#### Task 3.1: 修改 Kubernetes 属性页面
- 仓库：rainbond-ui
- 文件：`src/pages/Component/kubernets.js`
- 实现内容：`cmd/args` 使用 YAML 编辑器，新增示例和提示。
- 验收标准：`yarn build` 通过。

#### Task 3.2: 更新 i18n 文案
- 仓库：rainbond-ui
- 文件：`src/locales/zh-CN/*.js`、`src/locales/en-US/*.js`
- 实现内容：更新 `cmd/args` 提示文案。
- 验收标准：页面引用的文案 key 完整。

## 八、关键参考代码
| 功能 | 文件 | 说明 |
|------|------|------|
| region API 路由 | `api/api_routers/version2/v2Routers.go:610` | `/k8s-attributes` 路由 |
| region 属性模型 | `api/model/component.go:276` | `ComponentK8sAttribute` |
| worker 容器转换 | `worker/appm/conversion/version.go:480` | `cmd/args` 应用到 Kubernetes Container |
| console 服务 | `console/services/k8s_attribute.py:15` | console 保存并同步 region |
| console 视图 | `console/views/k8s_attribute.py:14` | console k8s attribute API |
| UI 属性页面 | `src/pages/Component/kubernets.js:25` | Kubernetes 属性可选字段和编辑器选择 |
