# 新增 Go API 端点

引导开发者在 rainbond Go 后端新增一个 API 端点。

## 请提供以下信息

1. API 路径（例如：`/v2/tenants/{tenant_name}/apps/{app_id}/something`）
2. HTTP 方法（GET/POST/PUT/DELETE）
3. 功能描述
4. 是否需要新的数据库表？

## 实施步骤

按以下顺序创建/修改文件：

### 1. 数据模型（如需新表）
- 文件：`db/model/` 下对应文件
- 定义 GORM struct，包含 `TableName()` 方法

### 2. DAO 层
- 接口：`db/dao/` 下定义接口
- 实现：`db/mysql/` 下实现接口
- 注册：在 `db/db.go` 的 Manager 接口中添加

### 3. 请求/响应结构体
- 文件：`api/model/` 下对应文件
- 使用 `json` tag 和 `validate` tag

### 4. Handler（业务逻辑）
- 文件：`api/handler/` 下对应文件
- 使用 `GetXxxHandler()` 单例模式
- 调用 DAO 层和 K8s API

### 5. Controller（HTTP 处理）
- 文件：`api/controller/` 下对应文件
- 使用 `httputil.ValidatorRequestStructAndErrorResponse` 验证请求
- 使用 `httputil.ReturnSuccess/ReturnBcodeError` 返回响应

### 6. 路由注册
- 文件：`api/api_routers/version2/v2Routers.go`
- 在对应的 Router 方法中添加路由

### 7. 验证
```bash
go build ./...
go vet ./...
```
