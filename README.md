# LocalSend Go Library & CLI

[![Go Reference](https://pkg.go.dev/badge/github.com/GennoBou/localsend.svg)](https://pkg.go.dev/github.com/GennoBou/localsend)

A Go-based implementation of the LocalSend protocol (v2.1) library and command-line interface (CLI) tool.

> [!WARNING]
> **Disclaimer**: This is an **unofficial** community implementation of the LocalSend protocol in Go. It is not affiliated with, maintained by, or endorsed by the official LocalSend project.

---

### Language Links
- [日本語 (Japanese)](README.ja.md)
- **English**

---

## Overview

This project aims to re-implement the core LocalSend protocol in Go, providing the following key assets:

1. **Go Library**: A lightweight LocalSend communication library that minimizes external dependencies and can be easily integrated into any Go project.
2. **Multilingual CLI**: A TUI-capable command-line interface supporting English (default fallback) and Japanese.
3. **Enhanced Debuggability**: Built-in options for proxy configurations, skipping self-signed TLS certificate checks (`--insecure`), and loading custom CA certificates (`--ca`) to ease integration and debugging (e.g., using Mitmproxy).

---

## Key Features & Differences from Official Clients

- **Multiple Instances Support on a Single Host**:
  While the official LocalSend client manages nearby devices using only the IP address as a key (which causes device override conflicts on a single machine), this implementation utilizes **`IP:Port` as the primary key**. It automatically searches and binds up to 10 ports starting from `53317`, enabling multiple LocalSend clients to run and communicate concurrently on a single machine.
- **Interoperability with Official Clients**:
  Despite the multi-port extension, this library remains fully compatible with official LocalSend applications (Desktop/Mobile) on the local network.
- **HTTPS Server Limitation / HTTP-focused simplicity**:
  Currently, this implementation has limited HTTPS server support for reverse transfer (browser sharing) and local APIs, operating primarily on plain HTTP for simplicity.
- **Advanced Network Debugging**:
  The CLI natively supports `--proxy` and `-k / --insecure` flags to make traffic analysis and local network debugging seamless.
- **Low Dependencies**:
  The core library (`pkg/`) is designed with Go's standard packages (such as `net`, `net/http`, `crypto`, and `encoding/json`), resulting in a small and clean footprint.

---

## Design Documents

Please refer to the following design documents for detailed specifications and architecture:

### Protocol Specifications
- [LocalSend Protocol Specification (English version)](docs/protocol_spec.md)
- [LocalSend プロトコル仕様書 (日本語版)](docs/protocol_spec_ja.md)

### Architecture & Designs
- [Architecture Design Document (English version)](docs/architecture.md)
- [アーキテクチャ設計書 (日本語版)](docs/architecture_ja.md)

---

## Roadmap

### v1.0.0 Release (Completed)
- [x] **Phase 1: Specifications & Documentation**
- [x] **Phase 2: Core Go Library Development**
  - Dynamic self-signed TLS certificate generation (`pkg/crypto`)
  - Extended device key data structures (`pkg/protocol`)
  - UDP multicast and legacy unicast HTTP scanning (`pkg/discovery`)
  - HTTP upload/download receiving server (`pkg/server`)
  - HTTP sending client with progress callbacks (`pkg/client`)
- [x] **Phase 3: CLI Tool Development**
  - Command structures with Cobra
  - Multilingual (i18n) support & TUI progress bars
  - Proxy and TLS bypass configurations
- [x] **Phase 4: Interoperability Testing & Release**
  - Transfer validation tests with the official LocalSend app
  - v1.0.0 tagged release with English documentation

### v1.1.0 (Upcoming)
- [ ] **mDNS (Multicast DNS) Support**: Integrate mDNS for improved device discovery reliability in networks where UDP multicast is restricted.

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.
