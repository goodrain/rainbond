<img src="./docs/rainbond_logo.png" width="100%">

[![Go Report Card](https://goreportcard.com/badge/github.com/goodrain/rainbond)](https://goreportcard.com/report/github.com/goodrain/rainbond)
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v5.0-brightgreen.svg)
[![Build Status](https://travis-ci.org/goodrain/rainbond.svg?branch=master)](https://travis-ci.org/goodrain/rainbond)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[项目官网](http://www.rainbond.com) • [文档](https://www.rainbond.com/docs/stable/) • [在线体验](https://console.goodrain.com) • [README in English](https://github.com/goodrain/rainbond/blob/master/README_EN.md)

## **Rainbond** ENTERPRISE APPLICATION CLOUD OS

Rainbond（云帮）是企业应用的操作系统。 Rainbond支撑企业应用的开发、架构、交付和运维的全流程，通过“无侵入”架构，无缝衔接各类企业应用，底层资源可以对接和管理IaaS、虚拟机和物理服务器。

Rainbond is a cloud OS for enterprise applications. It provides complete set of supports for enterprise applications' development, architecture, delivery and operation, can seamlessly docking almost all kinds of enterprise applications through "non-invasive" platform architecture, interface and manage underlying computing resources such as IaaS, virtual machine and physical servers.

```
Enterprise Applications include：
information system, OA, CRM, ERP, database, big data, IOT, internet platform and microservice architecture etc.
```
## Be applied to

* Enterprise Application Developement

The development environment, micro-service architecture, service governance and various technical tools are “out of the box”, without changing development habits, allowing companies to focus on their business and improving efficiency by 10 times.

* Enterprise Application Delivery

Support continuous delivery, enterprise application market delivery, SaaS, enterprise application sales, secondary development and other delivery processes, unified customer management, and balanced delivery and personalized delivery.

* Enterprise Application Operation

Transparently interfaces and manages a variety of computing resources, naturally achieves cloudy and hybrid clouds, enterprise application automation and operation, and doubles resource utilization.

## Features

| Features                       | Description                                                         |
| -------------------------- | ------------------------------------------------------------ |
|  beyond Kubernetes             | based on kubernetes, but users do not need to learn and edit complex yaml files, achieved "out-of-the-box" business process by application-level graphical interface |
| native Service Mesh microservice architecture | Thanks to the cross-language, cross-protocol, code-free service Mesh microservices architecture native support, traditional applications can become microservice architecture directly. Support Spring Cloud, Dubbo, etc.,  and can easily expand the architectural capabilities and governance functions by adding plug-ins. |
| Integrated DevOps               | Integrate DevOps for demand, development, testing, construction, online, and operation and maintenance. Support for docking third party software (Jira, Sonar, Jenkins, Gitlab, etc.) |
| Enterpeise-level application market             | Not a simple mirror market and service catalog, but supports all kinds of enterprise-level applications, just like install and manage mobile apps, click-to-use, full process management (application development, application publishing, application display, application offline import/export, application installation/upgrade, application operation and maintenance)  |
| Automated operation and maintenance                 | Automated application operation and maintenance. Nodes are automatically installed, expanded, monitored, and fault tolerant. The platform supports high availability, multiple data center management, and multi-tenant management. |
| Serverless PaaS            | With the application-centric design philosophy, users do not need to understand the server-related concepts, and is simple and flexible. Quickly build industry-specific PaaS through docking industry applications. |
| Application Gateway                   | Applying access control policies based on protocols such as HTTP, HTTPs, TCP, and UDP, it is easy to operate grayscale publishing and A/B testing. |

More features： [Rainbond features description](https://www.rainbond.com/docs/stable/architecture/edition.html)

## Quick Start

1.  [install Rainbond cluster](https://www.rainbond.com/docs/stable/getting-started/installation-guide.html)
2.  [create the first application / service](https://www.rainbond.com/docs/stable/user-manual/create-an-app.html)
3.  [build ServiceMesh microservice architecture](https://www.rainbond.com/docs/stable/microservice/service-mesh/use-case.html)
4.  [Migrate existing enterprise applications]()

## Community

[Rainbond forum](https://t.goodrain.com)        

[Rainbond website](https://www.rainbond.com)

## Roadmap

See [Rainbond Roadmap](http://www.rainbond.com/docs/stable/architecture/roadmap.html)

## Architecture

<img src="https://static.goodrain.com/images/docs/5.0/architecture/architecture.svg" href="http://www.rainbond.com/docs/stable/architecture/architecture.html">

## Snapshot

<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/docs/5.0/readme/connect.gif" href="http://www.rainbond.com/docs/stable">

- Application assembly deployment diagram

<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/docs/5.0/readme/gateway.gif" href="http://www.rainbond.com/docs/stable">

- Application gateway management schematic diagram

## Contribution

You can participate in the Rainbond community's contributions and sharing on platforms, applications, plugins, and more.
[Participate in Rainbond Project](https://www.rainbond.com/docs/stable/contribute-to-rainbond.html)
[Rainbond Contribution](https://t.goodrain.com/c/contribution)

## Related Projects
   * [Rainbond-Console](https://github.com/goodrain/rainbond-console) 
   * [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui) 
   * [Rainbond-Install](https://github.com/goodrain/rainbond-ansible) 
   * [Rainbond-Builder](https://github.com/goodrain/builder) 
   * [Rainbond-Docs](https://github.com/goodrain/rainbond-docs) 
   
## License

Rainbond is released under LGPL-3.0 license, see [LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE) and [Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。

## Special THANKS

Thanks to the following open source projects

- [Kubernetes](https://github.com/kubernetes/kubernetes)
- [Docker/Moby](https://github.com/moby/moby)
- [Heroku Buildpacks](https://github.com/heroku?utf8=%E2%9C%93&q=buildpack&type=&language=)
- [OpenResty](https://github.com/openresty/)
- [Calico](https://github.com/projectcalico)
- [Midonet](https://github.com/midonet/midonet)
- [Etcd](https://github.com/coreos/etcd)
- [Prometheus](https://github.com/prometheus/prometheus)
- [GlusterFS](https://github.com/gluster/glusterfs)
- [Ceph](https://github.com/ceph/ceph)
- [CockroachDB](https://github.com/cockroachdb/cockroach)
- [MySQL](https://github.com/mysql/mysql-server)
- [Weave Scope](https://github.com/weaveworks/scope)
- [Ant Design](https://github.com/ant-design/ant-design)

## Join us

[Welcome you to join us with your passion for technology](https://www.rainbond.com/docs/recruitment/join.html)


