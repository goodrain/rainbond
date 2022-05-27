<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/rainbond%20log_full.png" width="60%">

[![Rainbond](https://jaywcjlove.github.io/sb/lang/english.svg)](README-en.md)
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v5.X-brightgreen.svg)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[项目官网](http://www.rainbond.com?channel=github) • [文档](https://www.rainbond.com/docs?channel=github)

## Rainbond 是什么 ?

Rainbond 是一个云原生多云应用管理平台，使用简单，不需要懂容器、Kubernetes和底层复杂技术，支持管理多个Kubernetes集群，和管理企业应用全生命周期。主要功能包括应用开发环境、应用市场、微服务架构、应用交付、应用运维、应用级多云管理等。

## 为什么选择 Rainbond ?

Kubernetes 作为一个管理容器化应用程序的系统。它为应用程序的部署、维护和扩展提供了基本机制。但是在用户传统应用向云原生应用转型时，使用 Kubernetes 会遇到如下几个问题：

- 企业应用容器化部署
- Kubernetes 学习曲线陡峭
- 部署好的应用如何在其他 Kubernetes 环境中交付、升级

Rainbond 遵循 **以应用为中心** 的设计理念，统一封装容器、Kubernetes 和底层基础设施相关技术，让使用者专注于业务本身, 避免在业务以外技术上花费大量学习和管理精力。

- Rainbond 支持[一步将企业应用变成云原生应用](https://www.rainbond.com/docs/#2%E4%B8%80%E6%AD%A5%E5%B0%86%E4%BC%A0%E7%BB%9F%E5%BA%94%E7%94%A8%E5%8F%98%E6%88%90%E4%BA%91%E5%8E%9F%E7%94%9F%E5%BA%94%E7%94%A8)
- Rainbond 不需要了解 Kubernetes，并且可通过 [Web 界面快速安装 Kubernetes](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-web-%E7%95%8C%E9%9D%A2%E5%AE%89%E8%A3%85) ，支持管理多个 Kubernetes 集群
- Rainbond 支持多云交付、私有交付、SaaS交付、离线交付、个性化交付、应用市场等，实现各种交付流程自动化，可参考文章 [企业应用一键安装与升级](https://mp.weixin.qq.com/s/2chigbtp8TzPdvJM4o7sOw)

## Rainbond 的功能和架构

![Rainbond-Arch](https://grstatic.oss-cn-shanghai.aliyuncs.com/case/2022/03/17/16474283190784.jpg)

Rainbond 基于公有云、私有云、自建 Kubernetes 对企业应用进行管理，支持[应用级多云管理](https://www.rainbond.com/docs/#%E5%BA%94%E7%94%A8%E7%BA%A7%E5%A4%9A%E4%BA%91%E7%AE%A1%E7%90%86)。

Rainbond 支持应用的[全生命周期管理](https://www.rainbond.com/docs/#%E5%BA%94%E7%94%A8%E5%85%A8%E7%94%9F%E5%91%BD%E5%91%A8%E6%9C%9F%E7%AE%A1%E7%90%86)，即开发、架构、交付、运维一站式打通。

Rainbond 中的组件是独立运行、可复用、可扩展、可集成的单元，支持不同的粒度大小，支持版本管理，组件可以在不同应用场景中复用，组件自身可以迭代升级，积累的组件统一存放到组件库，实现了企业[数字化能力积累和复用](https://www.rainbond.com/docs/#3%E5%AE%9E%E7%8E%B0%E6%95%B0%E5%AD%97%E5%8C%96%E8%83%BD%E5%8A%9B%E7%A7%AF%E7%B4%AF%E5%92%8C%E5%A4%8D%E7%94%A8)。

## 安装
Rainbond 支持多种安装方式。你可以通过以下命令安装 Allinone 版本，快速体验 Rainbond 完整功能。

请注意：**该方式仅适用于开发者快速体验，不具备生产可用性**。其他安装方式请参考 [Web 页面安装](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-web-%E7%95%8C%E9%9D%A2%E5%AE%89%E8%A3%85)、[Helm 安装](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-helm-%E5%AE%89%E8%A3%85)、[对接云服务商](https://www.rainbond.com/docs/quick-start/quick-install#%E5%AF%B9%E6%8E%A5%E4%BA%91%E6%9C%8D%E5%8A%A1%E5%95%86)、[对接其他平台](https://www.rainbond.com/docs/quick-start/quick-install#%E5%AF%B9%E6%8E%A5%E5%85%B6%E4%BB%96%E5%B9%B3%E5%8F%B0)、[高可用安装](https://www.rainbond.com/docs/user-operations/deploy/install-with-ui/ha-installation)

```bash
docker run --privileged -d  -p 7070:7070 -p 80:80 -p 443:443 -p 6060:6060 -p 8443:8443 \
--name=rainbond-allinone --restart=unless-stopped \
-v ~/.ssh:/root/.ssh \
-v ~/rainbonddata:/app/data \
-v /opt/rainbond:/opt/rainbond \
-v ~/dockerdata:/var/lib/docker \
-e ENABLE_CLUSTER=true \
registry.cn-hangzhou.aliyuncs.com/goodrain/rainbond:v5.6.0-dind-allinone \
&& docker logs -f rainbond-allinone
```

## 在线体验

您可访问[试用环境](http://demo.c9f961.grapps.cn/)在线体验平台功能。由于资源有限，您仅有查看权限。如需体验更多功能，请自行部署尝试。

## 快速开始

1. [Rainbond 简介](https://www.rainbond.com)
2. [创建第一个应用（组件）](https://www.rainbond.com/docs/use-manual/component-create/creation-process?channel=github)

## 视频教程
1. [Rainbond 安装系列合集](https://www.bilibili.com/video/BV1Vq4y1w7FQ?spm_id_from=333.999.0.0)
2. [Rainbond 入门教程](https://www.bilibili.com/video/BV1ou411B7ix?spm_id_from=333.999.0.0)
3. [Rainbond 实现一体化开发环境](https://www.bilibili.com/video/BV1vS4y1w7ps)
4. [Rainbond 实现企业应用统一管理](https://www.bilibili.com/video/BV1rB4y197X4)

## 参与社区

[Rainbond 开源社区](https://t.goodrain.com) 欢迎你在社区中查阅或贡献 Rainbond 的用例用法。

[Rainbond 项目官网](https://www.rainbond.com?channel=github) 查阅关于 Rainbond 的更多信息。

微信扫码关注 Rainbond 公众号，查看 Rainbond 最佳实践。

<img width="300px" src="https://static.goodrain.com/wechat/WechatQRCode.gif"/>

钉钉搜索群号加入 Rainbond 技术交流群: `31096419`

添加微信小助手进入 Rainbond 技术交流群:

<img width="300px" src="https://static.goodrain.com/wechat/weChat.jpg"/>

## 贡献

我们非常欢迎你参与 Rainbond 社区关于平台使用经验、标准化应用、插件分享等领域的贡献和分享。

若你是正在使用 Rainbond 的用户，且对 Rainbond 有深入的了解和技术路线的认同，在你的企业内部有较大的需求，我们非常欢迎你 [参与 Rainbond 贡献](https://www.rainbond.com/docs/contributing/?channel=github)

## 相关项目

当前仓库为 Rainbond 数据中心端核心服务实现代码，项目还包括以下子项目：

- [Rainbond-Console](https://github.com/goodrain/rainbond-console) Rainbond 控制台服务端项目
- [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui) Rainbond 控制台前端项目
- [Rainbond-Operator](https://github.com/goodrain/rainbond-operator) Rainbond 安装运维项目
- [Rainbond-Cloud-adaptor](https://github.com/goodrain/cloud-adaptor) Rainbond 集群安装驱动服务
- [Rainbond-Builder](https://github.com/goodrain/builder) Rainbond 源码构建工具集
- [Rainbond-Docs](https://github.com/goodrain/rainbond-docs) Rainbond 文档

## License

Rainbond 遵循 LGPL-3.0 license 协议发布，详情查看 [LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE) 及 [Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md) 。
