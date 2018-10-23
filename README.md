<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_logo.png" width="100%">

[![Go Report Card](https://goreportcard.com/badge/github.com/goodrain/rainbond)](https://goreportcard.com/report/github.com/goodrain/rainbond)
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v3.7-brightgreen.svg)
[![Build Status](https://travis-ci.org/goodrain/rainbond.svg?branch=master)](https://travis-ci.org/goodrain/rainbond)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[Website](http://www.rainbond.com) • [Docs](https://www.rainbond.com/docs/stable/) • [Public Cloud](https://console.goodrain.com) • [中文 README](https://github.com/goodrain/rainbond/blob/master/readme_zh.md)

Rainbond is an application-centric open source PaaS. Integrates Kubernetes container management, Service Mesh microservices architecture best practices, multi-type CI/CD application building and delivering, multi-data-center resource management, Rainbond provides cloud native application  full-lifecycle solution, and build ecosystem of application and infrastructure, application and application, infrastructure and infrastructure, to meet the agile development, efficient operations and lean management needs of business.

## Features

#### Application management

- application level orchestration (for complete business system)
- integrates Kubernetes's service component level orchestration and scheduling (service discovery, dynamic routing, limiting and fuse etc.)
- provides cloud native microservices governance based on extendable service mesh grid system
- supports multiple microservices architecture (SpringCLoud, Dubbo, API-Gateway)
- supports multiple types of service-level application performance analysis
- supports for building services based on source code (Java, PHP, Python, Golang, Ruby etc.)
- supports for continuous building and deployment based on public/private Git, SVN code repositories, image repositories and third-party CI system.
- supports for building application based on docker image, docker run command and dockercompose
- supports application level full backup and recovery, migrating application between tenants and data centers.
- Service plug-in system supports flexible extension of application functions and features, such as log, firewall and traffic anaylsis.
- supports for public/private application market with complete application delivery system.

#### Resource/cloud management

- basic system of cloud-native data center
- supports plug-in deployment of different service gateway (openresty, F5 etc.), SDN network (midonet, calico etc.) and distributed storage systems (GlusterFS, Ali-NAS, Ceph etc.)
- supports multi-data-center or cluster management and application orchestration
- supports for platform high-availability
- cluster management, operation and maintenance automation
- automatic monitoring of node's physical hardware and system, and system indicators
- automatic monitoring of service instances container metrics

## Quick Start

1.  [Install Rainbond](https://www.rainbond.com/docs/stable/getting-started/installation-guide.html)
2.  [Create First Application Service](https://www.rainbond.com/docs/stable/user-manual/create-an-app.html)
3.  [Build Microservice Architecture](https://www.rainbond.com/docs/stable/microservice/service-mesh/use-case.html)

## Community

[Rainbond Community](https://t.goodrain.com)
[Rainbond Web](https://www.rainbond.com)

## Roadmap

Read the [Roadmap](http://www.rainbond.com/docs/stable/architecture/roadmap.html).

## Architecture

<img src="https://static.goodrain.com/images/docs/3.6/architecture/architecture.png" href="http://www.rainbond.com/docs/stable/architecture/architecture.html">


## Console UI show

<img src="./docs/buildfromsourcecode.gif" href="http://www.rainbond.com/docs/stable">

- Source code creation application process

<img src="./docs/topology.gif" href="http://www.rainbond.com/docs/stable">

- Business application group topology diagram,The network topology, applied relational topology and real-time monitoring are shown here.

## Contributing

You can participate in the contributions of platforms, applications, and plugins within the Rainbond community.
[Rainbond Contributor community](https://t.goodrain.com/c/contribution)

## License

Rainbond is under the LGPL-3.0 license, see [LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE) and [Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md) for details.

## Acknowledgment

Thanks for the following open source project

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

[Welcome you who love technology to join us](https://www.rainbond.com/docs/recruitment/join.html)
