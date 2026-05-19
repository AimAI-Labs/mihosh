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

A full-featured terminal UI (TUI) management tool for mihomo, allowing you to elegantly manage proxy nodes, monitor connections, and view logs directly from your terminal.

## Features

| Page | Description |
|------|-------------|
| 🎯 **Nodes** | Switch proxy nodes quickly, single/batch latency testing |
| 📊 **Connections** | Real-time active connections, traffic/memory charts, close connections |
| 📝 **Logs** | Live log streaming with level filtering and keyword search |
| 📋 **Rules** | View proxy rules with multi-keyword search |
| ⚙️ **Settings** | Modify configuration directly in the UI |
| ❓ **Help** | Built-in keyboard shortcuts reference |

## Installation

### Homebrew

```bash
brew tap AimAI-Labs/mihosh https://github.com/AimAI-Labs/mihosh
brew install mihosh
```

### Install Script

```bash
# Install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/AimAI-Labs/mihosh/refs/heads/main/install.sh | bash
```


## Quick Start

### 1. Initialize Configuration

```bash
mihosh config init
```

Enter your Mihomo API address and secret when prompted. Config is saved to `~/.mihosh/config.yaml`

### 2. Launch

```bash
mihosh
```

This opens the interactive TUI. Press `5` or `Tab` to switch to the Help page for keyboard shortcuts.

## Configuration

### File Location

- Linux / macOS: `~/.mihosh/config.yaml`

### Available Options

```yaml
api_address: http://127.0.0.1:9090
secret: your-secret-here
test_url: http://www.gstatic.com/generate_204
timeout: 5000
proxy_address: http://127.0.0.1:7890
```

### CLI Configuration Commands

```bash
mihosh config set api-address http://127.0.0.1:9090
mihosh config set secret your-secret-here
mihosh config set test-url http://www.gstatic.com/generate_204
mihosh config set timeout 5000
mihosh config set proxy-address http://127.0.0.1:7890
mihosh config show --output table
```

## CLI Mode (Optional)

In addition to the TUI, command-line operations are also supported:

```bash
mihosh list                          # List proxy groups
mihosh list --output json            # List groups in JSON
mihosh select <group> <node>         # Switch node
mihosh test                          # Test currently selected node
mihosh test --output table           # Test current node in table format
mihosh test node <node>              # Test a specific node
mihosh test group <group>            # Test all nodes in a group
mihosh connections                   # View connections
mihosh connections --output json     # View connections in JSON
mihosh config show --output table    # Show config in table format
```

## FAQ

| Issue | Solution |
|-------|----------|
| Connection failed | Check if Mihomo is running, verify API address and secret |
| Nodes not found | Ensure proxy groups are configured in mihomo config |
| Test timeout | Increase `timeout` value or change `test_url` |

## Development

```bash
go mod download  # Install dependencies
make check       # Format, vet, test, and build
```

If `make` is not available on your system, run the equivalent commands manually:

```bash
go fmt ./...
go vet ./...
go test ./...
go build .
```

Pull requests are validated by GitHub Actions for formatting, module verification,
vet, tests, build, lint, and CodeQL analysis.

## License

MIT License
