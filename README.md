# Azure Developer CLI (`azd`)

> **From code to cloud in minutes.** A developer-centric CLI to build, deploy, and operate Azure applications.

[![azd version](https://img.shields.io/endpoint?url=https%3A%2F%2Fazuresdkartifacts.z5.web.core.windows.net%2Fazd%2Fstandalone%2Flatest%2Fshield.json)](https://github.com/Azure/azure-dev/releases)
[![VS Code Extension](https://img.shields.io/endpoint?url=https%3A%2F%2Fazuresdkartifacts.z5.web.core.windows.net%2Fazd%2Fvscode%2Flatest%2Fshield.json)](https://marketplace.visualstudio.com/items?itemName=ms-azuretools.azure-dev)
[![GitHub Discussions](https://img.shields.io/github/discussions/Azure/azure-dev)](https://github.com/Azure/azure-dev/discussions)

---

## Built for you

- ⚡ **Get productive fast** — Streamlined workflows to go from code to cloud in minutes
- 🏗️ **Azure recommended practices built-in** — Opinionated templates that follow Azure development standards
- 🧠 **Learn as you build** — Understand core Azure constructs through hands-on experience

📖 **[Get Started](https://aka.ms/azd)** · 💬 **[Join the Discussion](https://github.com/Azure/azure-dev/discussions)** · 📦 **[Browse Templates](https://azure.github.io/awesome-azd/)**

---

## Downloads

| Artifact | Version | Download |
| -------- | ------- | -------- |
| CLI | ![azd version](https://img.shields.io/endpoint?url=https%3A%2F%2Fazuresdkartifacts.z5.web.core.windows.net%2Fazd%2Fstandalone%2Flatest%2Fshield.json) | [Windows](https://azuresdkartifacts.z5.web.core.windows.net/azd/standalone/latest/azd-windows-amd64.zip) · [Linux](https://azuresdkartifacts.z5.web.core.windows.net/azd/standalone/latest/azd-linux-amd64.tar.gz) · [macOS](https://azuresdkartifacts.z5.web.core.windows.net/azd/standalone/latest/azd-darwin-amd64.zip) |
| VS Code Extension | ![vscode extension version](https://img.shields.io/endpoint?url=https%3A%2F%2Fazuresdkartifacts.z5.web.core.windows.net%2Fazd%2Fvscode%2Flatest%2Fshield.json) | [Marketplace](https://marketplace.visualstudio.com/items?itemName=ms-azuretools.azure-dev) |

## 🤖 AI Agents

**Contributing to this repo?** See [AGENTS.md](cli/azd/AGENTS.md) for coding standards and guidelines.

**Using `azd` with an AI coding assistant?** Check out the [docs](https://aka.ms/azd) and [templates](https://azure.github.io/awesome-azd/).

---

## Installation

Install or upgrade to the latest version. For advanced scenarios, see the [installer docs](cli/installer/README.md).

### Windows

```powershell
# Using winget (recommended)
winget install microsoft.azd

# Or Chocolatey
choco install azd

# Or install script
powershell -ex AllSigned -c "Invoke-RestMethod 'https://aka.ms/install-azd.ps1' | Invoke-Expression"
```

### macOS

```bash
brew install azure/azd/azd
```

> **Note:** If upgrading from a non-Homebrew installation, remove the existing `azd` binary first.

### Linux

```bash
curl -fsSL https://aka.ms/install-azd.sh | bash
```

### Shell Completion

Enable tab completion for `bash`, `zsh`, `fish`, or `powershell`:

```bash
azd completion [bash|zsh|fish|powershell]
```

---

## Contributing

This project welcomes contributions and suggestions. Most contributions require you to agree to a Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us the rights to use your contribution. For details, visit [https://cla.microsoft.com](https://cla.microsoft.com).

Check out our [Contributing Guide](CONTRIBUTING.md) to get started with local development and to understand our pull request process.

---

## License

Copyright (c) Microsoft Corporation. All rights reserved.

Licensed under the [MIT](LICENSE) License.