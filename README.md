<div align="center">
  <img src="https://static.goodrain.com/logo/logo-long.png" width="60%">

  [![Rainbond](https://jaywcjlove.github.io/sb/lang/chinese.svg)](README-zh.md)
  [![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
  ![Rainbond version](https://img.shields.io/badge/version-v5.X-brightgreen.svg)
  [![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)
  <a href="https://twitter.com/intent/follow?screen_name=Rainbond_"><img src="https://img.shields.io/twitter/follow/Rainbond?style=social" alt="follow on Twitter"></a>
  <a href="https://join.slack.com/t/rainbond-slack/shared_invite/zt-1ft4g75pg-KJ0h_IAtvG9DMgeE_BNjZQ"><img src="https://img.shields.io/badge/Slack-blueviolet?logo=slack&amp;logoColor=white"></a>
  <a href="https://discord.com/invite/czusNpcymS">
  <img src="https://img.shields.io/badge/Discord-Join-5865F2?style=flat-square&logo=discord" alt="Discord">
  </a>
  <h3>Build Enterprise Applications Like Mobile Apps</h3>

  [Website](https://www.rainbond.io?channel=github) • [Documentation](https://www.rainbond.io/docs/?channel=github)
</div>

## What is Rainbond?
Rainbond is 100% open-source, offers a serverless experience, and allows you to easily manage containerized applications without needing to understand Kubernetes. It is an integrated application management platform suitable for private deployment.

🚀 One-line Definition: Rainbond = Heroku-like Experience + Native Kubernetes Support + Self-hosted Capabilities

## Why Rainbond?

### Positioning Differences with Mainstream Platforms
| **Platform Type**      | Representative Products           | Rainbond's Differentiation                    |
|------------------------|-----------------------------------|-----------------------------------------------|
| **Developer-friendly PaaS** | Heroku, Vercel           | ▶ Self-hosted Support ▶ Full K8s Compatibility |
| **K8s Native Tools**   | Rancher, Devtron         | ▶ Application-level Abstraction ▶ Zero YAML Experience ▶ Complex Application Topology ▶ Offline Environment Support |
| **Self-hosted Solutions** | CapRover, Coolify        | ▶ Enterprise Multi-tenancy ▶ Hybrid Cloud Management |

### 🎯 What Pain Points Does It Solve?
**Developer Perspective**
- "I need to deploy a system with 20 microservices, but don't want to study K8s configs for each component"
- "The configuration differences between production and test environments make every deployment risky"
- "How to quickly deliver complex systems in customer's offline environment?"

**Ops/Platform Admin Perspective**
- "Need to give developers autonomy while ensuring cluster stability"
- "Traditional application cloud-native transformation costs too much"
- "Unified application management across multi/hybrid cloud environments"

### 🚀 Core Capabilities

- **Install Enterprise Software Like Mobile Apps**: Through the built-in application marketplace, various published microservice application templates support one-click installation and upgrades, even for systems with 100+ microservices.

- **Containerization Without Dockerfile and YAML**: The platform automatically recognizes multiple development languages like Java, Python, Golang, NodeJS, PHP, .NetCore, etc., completing build and deployment through a wizard-like process without writing Dockerfile or YAML.

- **Full Application Lifecycle Management**: Serverless experience where regular developers can manage and maintain applications and components without learning, including start, stop, build, update, auto-scaling, gateway policy management, etc., with non-invasive microservice architecture.

- **Microservice Modular Assembly**: Business components running on Rainbond support modular dependency orchestration, one-click publishing as reusable application templates, enabling business component accumulation and reuse.

### Who Is It Designed For?
👩💻 Developer Users
- Need URL access within 5 minutes from code
- Want cloud-native capabilities without learning K8s
- Zero configuration differences between dev and prod environments

👨💼 Platform Managers
- Traditional application cloud-native transformation
- Building internal PaaS platforms
- Achieving unified hybrid cloud management

### ✨ Differentiating Highlights Comparison
| Dimension          | Traditional Approach              | Rainbond Approach                |
|-------------------|----------------------------------|----------------------------------|
| **Deployment Complexity** | Requires K8s experts to write YAML | Visual orchestration, auto-generates K8s resources |
| **Environment Consistency** | Manual maintenance of multiple configs | Environment config templating, one-click deployment |
| **Delivery Form** | Docs + scripts + manual deployment | Self-contained app template (code + config) |
| **Skill Requirements** | Need full container/K8s stack skills | Operation interface based on application model abstraction |

## Getting Started

## Installation

### Minimum Requirements
- Linux OS (CentOS 7+/Ubuntu 18.04+)
- 2 CPU cores / 8GB RAM / 50GB disk space

### 3-Minute Quick Installation
You only need to execute the following command to run a container and quickly experience the full functionality of Rainbond. For more installation options, refer to [Installation and Upgrade](https://www.rainbond.io/docs/quick-start/quick-install).

```bash
curl -o install.sh https://get.rainbond.com && IMGHUB_MIRROR=rainbond bash ./install.sh
```

After the command is executed successfully, open a browser and enter `http://<IP>:7070` to access the platform and start deploying applications. `<IP>` is the IP address you selected or entered when running the script.

### Quick Start

Please refer to the [Quick Start](https://www.rainbond.io/docs/quick-start/getting-started?channel=github) documentation.

## Open Source Community

If you encounter any issues while using Rainbond and need help, please refer to the [Community Support](https://www.rainbond.io/docs/support?channel=github).

Slack: [Rainbond Slack Channel](https://join.slack.com/t/rainbond-slack/shared_invite/zt-1ft4g75pg-KJ0h_IAtvG9DMgeE_BNjZQ)

Twitter: [Rainbond Twitter](https://twitter.com/Rainbond_)

Discord: [Rainbond Discord](https://discord.com/invite/czusNpcymS)

## Contribution

We welcome contributions and sharing in the Rainbond community in areas such as platform usage experience, standardized applications, and plugin sharing.

If you are a Rainbond user who has a deep understanding of Rainbond and aligns with its technical direction, and you have significant demands within your organization, we welcome you to [contribute to Rainbond](https://www.rainbond.io/docs/contribution?channel=github).

## Related Projects

This repository contains the core service implementation code of the Rainbond data center. The project also includes the following sub-projects:

- [Rainbond-Console](https://github.com/goodrain/rainbond-console): Rainbond console server project.
- [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui): Rainbond console frontend project.
- [Rainbond-Operator](https://github.com/goodrain/rainbond-operator): Rainbond installation and operation project.
- [Rainbond-Builder](https://github.com/goodrain/builder): Rainbond source code build toolset.

## License

This project is licensed under the Apache License 2.0, with additional terms specified in `ADDITIONAL_TERMS.md`. Please read both the [Apache License 2.0](LICENSE) and [Additional Terms](ADDITIONAL_TERMS.md) before using, modifying, or distributing this software.