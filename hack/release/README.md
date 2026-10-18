# 发版发布（release-publish）

rainbond 发布 release 之后，原来要手工做的发布步骤由
[`.github/workflows/release-publish.yml`](../../.github/workflows/release-publish.yml) 完成。
本目录是这些步骤的脚本和数据。

## 发一个版本

前提：三个仓库（rainbond、rainbond-console、rainbond-ui）的 main 已经跑过一次全量 E2E
（rainbond-e2e-tests 的 `prerelease`），而且之后 main 没有新提交。

1. Actions → **release** → Run workflow，只填版本号（如 `v6.9.12-release`）。它会依次：
   - **gate**：确认最近一次全量 E2E 是绿的，且测的就是三个仓库当前 main 的 commit；
   - **build**：按 gate 固定的 commit 调用 `release-v6` 构建双架构镜像；
   - **images**：确认 15 个镜像都是 amd64 + arm64；
   - **draft**：创建草稿 release（自动生成说明，需要的话在发布前编辑）；
   - **go**：等待审批 1。
2. 在 **go** 处审批（能不能发）。审批后再次确认 main 没有变，然后发布 release，
   并自动触发 `trigger-downstream-releases`（console / ui / operator 打 tag）和 `release-publish`。
3. 在 `release-publish` 的 **publish** 处审批 2（对外发布），先看 prepare 摘要里与线上的差异。
4. **verify** 全绿即发布完成。roi 二进制和离线包目前仍需单独处理。

只想重新发布 chart / 安装脚本 / upgrade feed（例如回滚），直接运行 `release-publish`，
`dry_run` 保持勾选可以只看预览。

`release-v6` 仍可手动运行（例如构建 dev 镜像）；它拒绝在 goodrain/rainbond 以外构建
`-release` 版本，也拒绝单架构构建 `-release` 版本。

## 它替代了哪些手工步骤

| 原清单项 | 现在 |
|---|---|
| x86、arm64 所有镜像打包完成 | prepare 匿名检查 Docker Hub 与阿里云共 15 个镜像的 manifest |
| rainbond-chart 改版本号 | prepare 改 `Chart.yaml` / `values.yaml`，publish 提交后再触发 chart 打 tag |
| 更新 helm 仓库包 | publish 上传到 chart.rainbond.com；已存在的版本跳过，从不覆盖、不删除 |
| 快速安装脚本的版本 | 取 rainbond-console 对应 tag 的 `script/install-rainbond.sh`，只替换 `RAINBOND_VERSION` 默认值 |
| versions.json | 即 `upgrade-versions.json`，控制台「版本升级」读取。由本目录的同名文件加上新版本生成 |
| 文档版本号 | 中版本号变化（如 6.9→6.10）时给 rainbond-docs 提 draft PR；补丁版本不改 |

离线包（roi）仍在火山云服务器上手工打包，不在本流程内。

## 文件

| 文件 | 说明 |
|---|---|
| `upgrade-versions.json` | 线上 upgrade feed 的唯一来源。每次发布后由工作流回写，**不要手工改线上文件** |
| `release.py` | 各步骤的实现，只依赖 Python 标准库，`python3 release.py -h` 查看子命令 |
| `release_test.py` | 单元测试：`python3 -m unittest discover -s hack/release -p '*_test.py'` |

`release.py` 会拒绝不在 `registry.cn-hangzhou.aliyuncs.com/goodrain/` 下的镜像地址，
这是对 upgrade feed 的供应链保护。

## 一次性配置

在 goodrain/rainbond 的 **Settings → Environments** 新建 `release-publish`：

- **Required reviewers**：发布审批人。
- **Deployment branches**：只允许 `main`。

Secrets（只放在这个 environment 里）：

| 名称 | 用途 |
|---|---|
| `CHART_REPO_USERNAME` / `CHART_REPO_PASSWORD` | chart.rainbond.com 上传账号 |
| `TOS_ACCESS_KEY` / `TOS_SECRET_KEY` | 只授权写下面两个对象的 TOS 子账号 |

Variables（非机密，但不写进仓库）：

| 名称 | 说明 |
|---|---|
| `TOS_ENDPOINT` / `TOS_REGION` / `TOS_BUCKET` | get.rainbond.com 所在的桶 |
| `TOS_KEY_INSTALL_SCRIPT` | `https://get.rainbond.com` 对应的对象 key |
| `TOS_KEY_UPGRADE_VERSIONS` | `upgrade-versions.json` 的对象 key（也用于拼线上校验地址） |
| `TOSUTIL_URL` / `TOSUTIL_SHA256` | tosutil 下载地址与校验值，下载后先校验再执行 |

仓库已有的 `CROSS_REPO_RELEASE_TOKEN` 还需要对 rainbond-chart（contents、actions）和
rainbond-docs（contents、pull requests）有写权限。

publish 开始时会先检查上面这些配置是否齐全（只列名称，不打印值），并在任何写操作之前
确认 `tos://$TOS_BUCKET/$TOS_KEY_*` 的现有内容与对应公开地址返回的内容一致，
不一致就失败，防止 key 配错时覆盖别的文件。

另外新建 **`release-go`** environment（审批 1），同样设置 Required reviewers、只允许 `main`。
仓库级 secret **`E2E_READ_TOKEN`**：能读取 rainbond-e2e-tests Actions 运行结果和 artifact 的只读令牌。

飞书通知（可选，仓库级 secret）：`FEISHU_WEBHOOK` 为飞书群自定义机器人的 webhook，机器人开启了签名校验时再配置
`FEISHU_SECRET`。配置后会在等待审批 1、等待审批 2、发布完成和任何一步失败时发消息；未配置则跳过，
发送失败只会记为警告，不影响发版。

下游仓库（rainbond-console / ui / operator / chart）的 `auto-release-from-rainbond.yml` 需要支持可选输入 `target`，
release 流程会把 gate 固定的 commit 传过去，tag 精确打在测过的 commit 上。**这些下游改动必须先于本仓库合并**，
否则带 `target` 的触发会被下游拒绝。

## 发布目标（默认即生产，fork 演练时覆盖）

以下是**仓库级** Variables，生产环境不需要设置：

| 名称 | 默认值 |
|---|---|
| `RELEASE_RAINBOND_REPO` | `goodrain/rainbond` |
| `RELEASE_CONSOLE_REPO` | `goodrain/rainbond-console` |
| `RELEASE_CHART_REPO` | `goodrain/rainbond-chart` |
| `RELEASE_DOCS_REPO` | `goodrain/rainbond-docs` |
| `RELEASE_CHART_REPO_URL` | `https://chart.rainbond.com` |
| `RELEASE_CHART_UPLOAD` | `true`；设为 `false` 时跳过 chart 上传和校验 |
| `RELEASE_INSTALL_SCRIPT_URL` | `https://get.rainbond.com` |
| `RELEASE_UPGRADE_VERSIONS_URL` | `https://get.rainbond.com/upgrade-versions.json` |
| `RELEASE_DOWNSTREAM_REPOS` | console / ui / operator 三个 goodrain 仓库；设为 `[]` 不触发下游 |
| `RELEASE_E2E_REPO` / `RELEASE_E2E_WORKFLOW` | `goodrain/rainbond-e2e-tests` / `prerelease.yml` |
| `RELEASE_E2E_GATE` | `required`；`skip` 只允许在 fork 上使用 |
| `RELEASE_REUSE_IMAGES` | `false`；`true` 时不构建、只校验已发布镜像，只允许在 fork 上使用 |

在 fork 上演练时，把 chart / docs 指向自己的 fork、TOS 指向测试桶、两个公开地址指向测试桶
的公开访问地址，并先把线上两个文件的当前版本放进测试桶，即可端到端演练而不碰生产。

## 回滚

upgrade feed 和安装脚本都来自仓库：回退 `hack/release/upgrade-versions.json` 的提交，
再用上一个版本号手动运行 release-publish（`dry_run` 取消勾选），控制台的升级提示和
安装脚本的默认版本就会回到上一版。版本号比当前旧时，chart 和文档保持不动，
已存在的 chart 包也会跳过。已发布的镜像和 chart 不覆盖，问题版本请发补丁版本修复。
