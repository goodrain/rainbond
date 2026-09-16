<div align="center">
  <img src="https://static.goodrain.com/logo/logo-long.png" width="60%" alt="Rainbond Logo" />

  <p><a href="./README-zh.md">中文</a></p>

  <p>
    <a href="https://github.com/goodrain/rainbond/stargazers">
      <img src="https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square" alt="GitHub stars" />
    </a>
    <img src="https://img.shields.io/badge/version-v6.X-brightgreen.svg?style=flat-square" alt="Rainbond version" />
    <a href="https://discord.com/invite/czusNpcymS">
      <img src="https://img.shields.io/badge/Discord-Join-5865F2?style=flat-square&amp;logo=discord" alt="Discord" />
    </a>
  </p>
</div>

<div align="center">
  <h2>An open-source container platform. No Kubernetes expertise required.</h2>
  <p>Build, deploy, assemble, and manage applications on Kubernetes through a graphical interface, without K8s expertise.</p>
  <p>
    <a href="https://www.rainbond.io?channel=github">Website</a> ·
    <a href="https://www.rainbond.io/docs/?channel=github">Documentation</a>
  </p>
</div>

## What is Rainbond?

Rainbond is an open-source container platform that requires no Kubernetes expertise. Its core capabilities are 100% open source.

It abstracts away infrastructure complexity and provides a unified way to deploy and manage business applications, AI-generated projects, open-source AI software, and large language model services. AI helps teams deploy and operate these workloads, keeping applications running reliably on their own servers or Kubernetes clusters.


## Where to start

| Your goal | Start here |
| --- | --- |
| I use AI coding and want to deploy my project | [Install RainSkills](https://github.com/goodrain/rainskills) |
| I want to try Rainbond without preparing a server | [Use Rainbond Cloud](https://run.rainbond.com) |
| I want to run applications on my own servers or Kubernetes | [Install Rainbond privately](https://www.rainbond.com/docs/quick-start/quick-install) |
| I want to deploy open-source applications such as Dify or RAGFlow | [Visit the Rainbond Application Marketplace](https://hub.rainbond.com) |
| I am evaluating open-source container platforms | [Explore Rainbond's application management and delivery capabilities](https://www.rainbond.com/compare) |

---

## What problems Rainbond solves

### 1. Deliver applications without deeply learning Kubernetes

Rainbond supports deploying applications from source code, container images, Docker Compose, Helm, or application templates. Its graphical interface and standardized workflows bring application dependencies, access, upgrades, and rollbacks into one place.

### 2. Make delivery in complex enterprise environments more reliable

Rainbond is especially suitable for:

- Private deployment
- Internal-network deployment
- Offline delivery
- Xinchuang adaptation
- x86 to ARM migration validation

### 3. Standardize delivery with marketplace and templates

Rainbond is not only about “getting apps running”. It also provides:

- Application templates
- Application marketplace
- One-click installation
- One-click upgrade
- Customer environment replication

### 4. Keep operations and delivery centered around the application

Rainbond emphasizes:

- Application-level abstraction
- Application topology and dependencies
- Application upgrades and rollbacks
- Delivery and operation across multiple environments and clusters

---

## Why Rainbond

### Lower barrier, not lower capability

Rainbond does not simply hide Kubernetes. It redistributes complexity:

- The platform and operations teams absorb low-level concerns
- Development and delivery teams focus on building, deploying, delivering, and operating applications

### Better fit for complex enterprise environments

Many platforms are strong at cluster management.  
Rainbond is stronger when the real problem is:

- How to deliver applications
- How to replicate delivery to customer environments
- How to handle upgrades in offline environments
- How to privately deploy AI applications

### Stronger marketplace and standardized delivery capability

If what you really need is:

- Template-based delivery
- Marketplace reuse
- Offline package export/import
- Versioned upgrades and rollbacks

Rainbond is closer to the actual work.

---

## How it differs from common platform choices

| Dimension | Rainbond | Rancher / KubeSphere type platforms |
| --- | --- | --- |
| Primary focus | Application delivery, application management, template reuse | Cluster governance, platform ops, Kubernetes management |
| Typical users | Developers, delivery teams, enterprise IT, platform owners | K8s admins, platform ops, cluster governance teams |
| Strongest scenarios | Private deployment, offline, Xinchuang, marketplace delivery, AI privatization | Multi-cluster governance, resource management, platform ops |
| Learning curve | Lower, exposes less K8s detail | Requires stronger K8s and platform governance knowledge |

If you are in evaluation mode, start here:

- [Rainbond vs KubeSphere](https://www.rainbond.com/compare/rainbond-vs-kubesphere?channel=github)
- [Rainbond vs Rancher](https://www.rainbond.com/compare/rainbond-vs-rancher?channel=github)
- [Rainbond vs Sealos](https://www.rainbond.com/compare/rainbond-vs-sealos?channel=github)

---

## Typical scenarios

- **Deliver applications without deeply learning Kubernetes**
- **Privately deploy AI applications**
- **Offline and internal-network delivery**
- **Xinchuang and ARM migration**
- **Enterprise marketplace and standardized delivery**

Continue with:

- [Offline / Xinchuang Topic](https://www.rainbond.com/offline-and-xinchuang?channel=github)
- [Comparison Center](https://www.rainbond.com/compare?channel=github)

---

## Quick start

### Requirements

- Linux or macOS
- Recommended: at least 2 CPU / 8GB RAM / 50GB disk

### Quick install

Run:

```bash
curl -o install.sh https://get.rainbond.com && bash ./install.sh
```

Then open:

```bash
http://<your-ip>:7070
```

### Next step

1. [Quick Install](https://www.rainbond.com/docs/quick-start/quick-install?channel=github)
2. [Deploy your first app](https://www.rainbond.com/docs/quick-start/getting-started?channel=github)
3. [Explore the Marketplace](https://hub.rainbond.com?channel=github)

---

## Community and support

- [Documentation](https://www.rainbond.com/docs?channel=github)
- [FAQ](https://www.rainbond.com/docs/faq?channel=github)
- [Community Support](https://www.rainbond.com/docs/support?channel=github)
- [GitHub Issues](https://github.com/goodrain/rainbond/issues)

---

## Contributing

If you want to contribute, start here:

- [Contribution Guide](https://www.rainbond.com/docs/contribution?channel=github)
- [Rainbond Docs](https://github.com/goodrain/rainbond-docs)
- [Open an Issue](https://github.com/goodrain/rainbond/issues)

We welcome:

- Code contributions
- Documentation improvements
- User stories and technical writeups
- Application template and plugin sharing

---

## Related projects

- [rainbond-console](https://github.com/goodrain/rainbond-console) - Console backend
- [rainbond-ui](https://github.com/goodrain/rainbond-ui) - Console frontend
- [rainbond-operator](https://github.com/goodrain/rainbond-operator) - Installation and operations
- [builder](https://github.com/goodrain/builder) - Source build toolset

---

## License

This repository is licensed under the [Rainbond Open Source License](./LICENSE), based on Apache 2.0 with additional conditions.
