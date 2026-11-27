<p align="center">
  <a href="https://github.com/AimAI-Labs/mihosh">
    <picture>
      <source srcset="assets/mihosh_bg.png">
      <img src="assets/mihosh_bg.png" alt="mihosh logo">
    </picture>
  </a>
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white" />
  <img alt="Bubbletea" src="https://img.shields.io/badge/Bubbletea-FF69B4?style=flat-square&logo=go&logoColor=white" />
  <img alt="Lipgloss" src="https://img.shields.io/badge/Lipgloss-9B59B6?style=flat-square&logo=go&logoColor=white" />
  <img alt="Cobra" src="https://img.shields.io/badge/Cobra-2ECC71?style=flat-square&logo=go&logoColor=white" />
  <img alt="Viper" src="https://img.shields.io/badge/Viper-E74C3C?style=flat-square&logo=go&logoColor=white" />
  <img alt="WebSocket" src="https://img.shields.io/badge/WebSocket-010101?style=flat-square&logo=socket.io&logoColor=white" />
</p>

# Mihosh

A terminal UI and CLI for mihomo. Manage proxy nodes, connections, logs, subscriptions, and configuration from your terminal.

See the [online documentation](https://aimai-labs.github.io/mihosh/) for setup, usage, configuration, and development.

## Quick Installation

Supports Linux and macOS on amd64 and arm64.

### Homebrew

```bash
brew tap AimAI-Labs/mihosh https://github.com/AimAI-Labs/mihosh
brew trust AimAI-Labs/mihosh # Required on Homebrew 6+
brew install AimAI-Labs/mihosh/mihosh
```

### Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/AimAI-Labs/mihosh/HEAD/install.sh | bash
```

### Manual Installation

Download the archive for your system from [GitHub Releases](https://github.com/AimAI-Labs/mihosh/releases/latest):

| System | Architecture | Archive |
| --- | --- | --- |
| Linux | amd64 (x86_64) | `mihosh-linux-amd64.tar.gz` |
| Linux | arm64 (aarch64) | `mihosh-linux-arm64.tar.gz` |
| macOS | amd64 (Intel) | `mihosh-darwin-amd64.tar.gz` |
| macOS | arm64 (Apple Silicon) | `mihosh-darwin-arm64.tar.gz` |

You can also download the latest archive with `wget` through [gh-proxy.org](https://gh-proxy.org) (Linux amd64 example):

```bash
wget -O mihosh-linux-amd64.tar.gz "https://gh-proxy.org/https://github.com/AimAI-Labs/mihosh/releases/latest/download/mihosh-linux-amd64.tar.gz"
```

For other systems, replace the archive name in both the URL and the `-O` argument using the table above.

Extract the archive and install the binary into a directory in your `PATH`. Replace the archive name below with the one you downloaded:

```bash
tar -xzf mihosh-linux-amd64.tar.gz
sudo install -m 755 mihosh /usr/local/bin/mihosh
mihosh version
```

Run `mihosh` to launch the TUI.

## License

[MIT](LICENSE)
