<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/rainbond%20log_full.png" width="60%">

[![Rainbond](https://jaywcjlove.github.io/sb/lang/chinese.svg)](README.md)
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v5.X-brightgreen.svg)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[Website](http://www.rainbond.com?channel=github) • [Documentation](https://www.rainbond.com/docs?channel=github)

## What is Rainbond ?

Rainbond is a cloud native multi cloud application management platform, which is easy to use and does not need to understand containers, kubernetes and underlying complex technologies. It supports the management of multiple kubernetes clusters and the management of the whole life cycle of enterprise applications. The main functions include application development environment, application market, micro service architecture, application delivery, application operation and maintenance, application level multi cloud management, etc.

## Why Rainbond ?

Kubernetes serves as a system for managing containerized applications. It provides a basic mechanism for the deployment, maintenance and expansion of applications. However, when users transform their traditional applications to cloud native applications, they will encounter the following problems when using kubernetes:

- Container deployment of enterprise applications
- Kubernetes has a steep learning curve
- How can deployed applications be delivered and upgraded in other kubernetes environments

Rainbond follows **the design concept of application centered** and unifies the technologies related to containers, kubernetes and underlying infrastructure, so that users can focus on the business itself and avoid spending a lot of learning and management energy on technologies other than the business.
- Rainbond supports [one-step transformation of enterprise applications into cloud native applications](https://www.rainbond.com/docs/#2%E4%B8%80%E6%AD%A5%E5%B0%86%E4%BC%A0%E7%BB%9F%E5%BA%94%E7%94%A8%E5%8F%98%E6%88%90%E4%BA%91%E5%8E%9F%E7%94%9F%E5%BA%94%E7%94%A8)
- Rainbond does not need to know about kubernetes, and can [quickly install kubernetes through the web interface](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-web-%E7%95%8C%E9%9D%A2%E5%AE%89%E8%A3%85), supporting the management of multiple kubernetes clusters
- Rainbond supports multi cloud delivery, private delivery, SaaS delivery, offline delivery, personalized delivery, application market, etc., and realizes the automation of various delivery processes. Refer to the article one click [installation and upgrade of enterprise applications](https://mp.weixin.qq.com/s/2chigbtp8TzPdvJM4o7sOw)

## Rainbond function and architecture

![Rainbond-Arch](https://grstatic.oss-cn-shanghai.aliyuncs.com/case/2022/03/17/16474283190784.jpg)

Rainbond manages enterprise applications based on public cloud, private cloud and self built kubernetes, and supports [application level multi cloud management](https://www.rainbond.com/docs/#%E5%BA%94%E7%94%A8%E7%BA%A7%E5%A4%9A%E4%BA%91%E7%AE%A1%E7%90%86).

Rainbond Support [application lifecycle management](https://www.rainbond.com/docs/#%E5%BA%94%E7%94%A8%E5%85%A8%E7%94%9F%E5%91%BD%E5%91%A8%E6%9C%9F%E7%AE%A1%E7%90%86), that is, one-stop connection of development, architecture, delivery and operation and maintenance.

Components in Rainbond are independent, reusable, extensible and integrated units that support different granularity and version management. Components can be reused in different application scenarios. Components themselves can be upgraded iteratively. The accumulated components are stored in the component library, realizing [the accumulation and reuse of enterprise digital capabilities](https://www.rainbond.com/docs/#3%E5%AE%9E%E7%8E%B0%E6%95%B0%E5%AD%97%E5%8C%96%E8%83%BD%E5%8A%9B%E7%A7%AF%E7%B4%AF%E5%92%8C%E5%A4%8D%E7%94%A8).

## Installation
Rainbond supports multiple installation methods. You can install the AllInOne version through the following command to quickly experience the full functions of rainbond.

Please note that：**This method is only applicable to the rapid experience of developers and has no production availability**。For other installation methods, please refer to [Web page installation](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-web-%E7%95%8C%E9%9D%A2%E5%AE%89%E8%A3%85)、[Helm installation](https://www.rainbond.com/docs/quick-start/quick-install#%E5%9F%BA%E4%BA%8E-helm-%E5%AE%89%E8%A3%85)、[Docking with cloud service providers](https://www.rainbond.com/docs/quick-start/quick-install#%E5%AF%B9%E6%8E%A5%E4%BA%91%E6%9C%8D%E5%8A%A1%E5%95%86)、[Docking with other platforms](https://www.rainbond.com/docs/quick-start/quick-install#%E5%AF%B9%E6%8E%A5%E5%85%B6%E4%BB%96%E5%B9%B3%E5%8F%B0)、[High availability installation](https://www.rainbond.com/docs/user-operations/deploy/install-with-ui/ha-installation)

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


## Quick start

1. [Rainbond Introduction](https://www.rainbond.com/docs/)
2. [Create the first application (component)](https://www.rainbond.com/docs/use-manual/component-create/creation-process?channel=github)

## Video tutorial

1. [Rainbond Installation series collection](https://www.bilibili.com/video/BV1Vq4y1w7FQ?spm_id_from=333.999.0.0)
2. [Rainbond Introductory tutorial](https://www.bilibili.com/video/BV1ou411B7ix?spm_id_from=333.999.0.0)

## Participating communities

[Rainbond Open source community](https://t.goodrain.com) You are welcome to review or contribute to the use case usage of rainbond in the community.

[Rainbond Project official website](https://www.rainbond.com?channel=github) Check out more about rainbond.

WeChat scan code focuses on the Rainbond official account to see Rainbond best practices.

<img width="300px" src="https://static.goodrain.com/wechat/WechatQRCode.gif"/>

DingTalk Search Group : `31096419`

Add a wechat assistant to enter the Rainbond Technology Exchange Group:

<img width="300px" src="https://static.goodrain.com/wechat/weChat.jpg"/>

## Contributing

We very much welcome you to participate in the contribution and sharing of platform experience, standardized applications, plug-in sharing and other fields in the rainbond community.

If you are a user who is using rainbond, and you have a deep understanding of rainbond and agree with the technical route, and there is a great demand within your enterprise, we welcome you to [participate in the development of rainbond project](https://github.com/goodrain/rainbond/blob/V5.4/CONTRIBUTING.md)

## Related repositories

At present, the warehouse is the implementation code of the core service at the end of rainbond data center. The project also includes the following sub projects：

- [Rainbond-Console](https://github.com/goodrain/rainbond-console) Rainbond Console server project
- [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui) Rainbond Console front end project
- [Rainbond-Operator](https://github.com/goodrain/rainbond-operator) Rainbond Installation, operation and maintenance project
- [Rainbond-Cloud-adaptor](https://github.com/goodrain/cloud-adaptor) Rainbond Cluster installation driver service
- [Rainbond-Builder](https://github.com/goodrain/builder) Rainbond Source code construction Toolset
- [Rainbond-Docs](https://github.com/goodrain/rainbond-docs) Rainbond Documentation

## License

Rainbond follow LGPL-3.0 license, Details see [LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE) and [Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。
