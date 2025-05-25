# Ethereum Go-native Localnet
A fully Go-native tool to spin up and orchestrate a local Ethereum network composed of Geth and Prysm nodes — either in-process or inside Docker containers — without using Bash, Docker Compose, or non-Go tooling.


## 🎯 Goals

- No shell scripts, YAML, or Makefiles
- Pure Go orchestration: config, spawn, manage nodes
- Support for both modes:
  - **In-process**: embed Geth directly in Go
  - **Dockerized**: use Go SDK to control containerized nodes
- Compatible with full EL+CL setup (Geth + Prysm)

## 🚀 Features

- Launch multiple Geth nodes on localhost
- Programmatic control over ports, datadirs, peering
- Pluggable support for Prysm and future CL clients
- Clean CI, linting, and testability

## 🛠️ Getting Started

```bash
git clone https://github.com/yourusername/go-eth-localnet
cd go-eth-localnet
go run main.go
```

## 🗺️ Roadmap
- [ ] Single Geth node (in-process)
- [ ] Multi-node Geth network with peer connections
- [ ] Docker-mode node runner (via Go SDK)
- [ ] CL integration: Prysm processes and Engine API
- [ ] Config-driven topologies (mesh, star, etc.)

## Development
```makefile
make lint # running linter
make test # running tests
make build # running project
```
