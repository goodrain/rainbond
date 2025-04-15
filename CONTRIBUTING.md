# CONTRIBUTING Guide ｜ [贡献指南](https://rainbond.io/docs/contribution)

First off, thank you for considering contributing to Rainbond. It's people like you that make Rainbond such a great platform.

## About Rainbond

Rainbond is a cloud native and easy-to-use application management platform, a best practice for cloud native application delivery, and easy to use. Focus on the application-centric concept. Enabling enterprises to build cloud native development cloud, cloud native delivery cloud.

This document is a guide to help you through the process of contributing to Rainbond.

## Become a contributor

You can contribute to Rainbond in several ways. Here are some examples:

* Contribute to the Rainbond codebase.
* Contribute to the [Rainbond docs](https://github.com/goodrain/rainbond-docs).
* Report bugs.
* Write technical documentation and blog posts, for users and contributors.
* Help others by answering questions about Rainbond.


## Report Bug

When you find a bug, or have questions about code, documents and project, use Issues to report and discuss.

## Feature

If you want to add some features to Rainbond and contribute relevant code. Please discuss in the Issues first, and list the effects of the functions you want to achieve, as well as the relevant design documents. After the discussion in the Issues is completed, you can carry out relevant development work and submit a pull request. We will review your code as soon as possible.

Check Pull Request is another way to contribute.

## Documents

When you find any spelling mistakes or great content to supplement on the [official website of Rainbond](https://www.rainbond.io/), you can submit a pull request to the [Rainbond docs](https://github.com/goodrain/rainbond-docs).

## Compile the project

Rainbond mainly consists of the following three projects.

- [Rainbond-UI](https://github.com/goodrain/rainbond-ui)
- [Rainbond-Console](https://github.com/goodrain/rainbond-console)

> The Rainbond-UI and Rainbond-Console combine to form the business layer. The business layer is a front-end and back-end separation mode. UI is the front-end code of the business layer, and console is the back-end code of the business layer.

- [Rainbond](https://github.com/goodrain/rainbond-console)

> Rainbond is the implementation of the data center end of the platform, which mainly interacts with the cluster.

### Business layer code compilation

#### Compile front-end code image

```
VERSION=v5.5.0-release ./build.sh
```

#### Compile backend code image

```
VERSION=v5.5.0-release ./release.sh
```

If you want to compile the front and back code together, use the following command

```
VERSION=v5.5.0-release ./release.sh allinone
```

### Data center side code compilation

#### Single component compilation

Single component compilation is often important in the actual development process. Because the system of rainbond system is relatively complex, it is usually used in the ordinary development process
After modifying a component, compile the component so that the latest component image can directly replace the image in the installed development and testing environment.

**Single component compilation supports the following components:**

- chaos
- api
- gateway
- monitor
- mq
- webcli
- worker
- eventlog
- init-probe
- mesh-data-panel
- grctl
- node

**The compilation formula is as follows:**

Take the chaos component as an example and execute it in the main directory of rainbond code

```./release.sh chaos```

#### Package and compile the complete installation package

Compile the complete installation package, which is suitable for rebuilding the installation package after many source code changes. Execute in the main record of rainbond code

```./release.sh all```
