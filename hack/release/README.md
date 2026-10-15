# 发版发布（release-publish）

rainbond 发布 release 之后，原来要手工做的发布步骤由
[`.github/workflows/release-publish.yml`](../../.github/workflows/release-publish.yml) 完成。
本目录是这些步骤的脚本和数据。

## 发一个版本

1. 用 `release-v6` 构建镜像，`platforms` 选 `linux/amd64,linux/arm64`。
   版本号以 `-release` 结尾时，单架构构建会被拒绝，防止覆盖已发布的多架构镜像。
2. 在 goodrain/rainbond 发布 release（如 `v6.9.12-release`）。
   - `trigger-downstream-releases` 给 console / ui / operator 打 tag（已有）。
   - `release-publish` 自动开始。
3. 打开 `release-publish` 这次运行，看 **prepare** 的摘要：镜像检查、chart 和
   upgrade feed 的差异、安装脚本版本行、产物 sha256。
4. 确认无误后，在 **publish** 处点「Review deployments → Approve」。
5. **verify** 从线上重新下载并比对，全绿即发布完成。

想先看看会发什么而不发布：Actions → release-publish → Run workflow，填 tag，
`dry_run` 保持勾选，只跑 prepare。

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

## 回滚

upgrade feed 和安装脚本都来自仓库：回退 `hack/release/upgrade-versions.json` 的提交，
再用上一个版本号手动运行 release-publish（`dry_run` 取消勾选），控制台的升级提示和
安装脚本的默认版本就会回到上一版。版本号比当前旧时，chart 和文档保持不动，
已存在的 chart 包也会跳过。已发布的镜像和 chart 不覆盖，问题版本请发补丁版本修复。
