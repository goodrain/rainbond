# Ubuntu 示例的首次登录

Ubuntu 24.04 Minimal 示例支持 SSH 密钥和用户名密码两种初始化方式。示例使用 amd64 QCOW2 镜像及 UEFI。默认用户名是 `ubuntu`；密码方式支持 Web VNC 账户登录，也支持通过已配置的网络入口使用 SSH。

这项初始化依赖镜像已经启用 cloud-init，并接受 NoCloud 数据源。当前平台只对内置 Ubuntu 示例提供密码初始化，不将该能力自动套用到自定义 URL、上传镜像、本地镜像或 ISO 安装介质。自定义镜像使用自身的账户配置。

## 发布与启用

1. 升级 Region API 和全部 Worker 到支持此功能的版本，并等待 Worker rollout 完成。
2. 升级 Console 和 UI。
3. 在 Region API 的部署配置中设置 `VM_PASSWORD_BOOTSTRAP_ENABLED=true`。由 Operator 管理的安装应将该环境变量写入对应 `RbdComponent.spec.env`，保留其他环境变量。
4. 确认 Region `/vm/capabilities` 与 Console 的 VM capability 响应均包含 `password_bootstrap_supported: true`，并确认 Console 自行返回的 `login_methods` 包含 `password`，再验证示例创建与首次登录。

开关默认关闭，只有精确字符串 `true` 才启用。它代表发布者已完成 API/Worker 配套升级，不会自动探测混合版本 Worker。新 UI 同时检查 Go 能力和 Console 的登录方式声明；旧 Console 仅透传 Go 布尔字段不足以开启密码方式。缺少声明时保留已有密钥流程。

## 账户和连接

- 密钥方式填写 SSH 公钥，使用对应私钥连接；它不会同时设置一个可用于 Web VNC 的密码。
- 密码方式填写 12–128 个字符的密码，不自动去除空格，不接受全空白或控制字符；不支持 `root` 用户。
- 两种方式均只登记集群内部 TCP/22。对外访问仍通过组件的“网络”页面单独配置。
- 创建确认只显示登录方式和用户名。密码不进入页面摘要、浏览器持久缓存、操作记录或常规日志；平台不提供密码回看。

Console 使用带随机盐的 SHA512-crypt 哈希，Region 与 cloud-init 只接收该哈希。普通属性读取和公开模板转移不包含它；私有备份保留恢复所需配置。操作系统磁盘中已有的账户属于镜像内容，发布镜像前仍需由镜像维护者处理。

官方参考：[NoCloud](https://docs.cloud-init.io/en/latest/reference/datasources/nocloud.html)、[Users and Groups](https://docs.cloud-init.io/en/latest/reference/modules.html#users-and-groups)。
