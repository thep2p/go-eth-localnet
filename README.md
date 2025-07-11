# Ethereum Go-native Local Network
A fully Go-native tool to spin up and orchestrate a local Ethereum network composed of Geth and Prysm nodes — either in-process or inside Docker containers — without using Bash, Docker Compose, or non-Go tooling.

## Prerequisites

### Golang
The project requires a minimum of **Go 1.23.10** or higher. You can download and install the required version
from [the official Go website](https://go.dev/dl/). Make sure you have the correct version installed by running:

```bash
go version
```

If your Go version is lower than **1.23.10**, please upgrade your Go installation.

### Solidity 
The project uses the Soldity compiler for smart contract development. You can install it using the following command:

```bash
brew install solidity
```

Ensure that the Solidity compiler is correctly installed by checking its version (should be at least `0.8.30+commit.73712a01`)

```bash
solc --version
```

## 🎯 Goals

- No shell scripts, YAML, or Makefiles
- Pure Go orchestration: config, spawn, manage nodes
- Support for both modes:
  - **In-process**: embed Geth directly in Go
  - **Dockerized**: use Go SDK to control containerized nodes
- Compatible with full EL+CL setup (Geth + Prysm)

-## 🚀 Features
-
- Launch a single Geth node on localhost
- Blocks are produced using the simulated beacon
- Programmatic control over ports and datadirs
- Graceful shutdown waits for the node to close
- Pluggable support for Prysm and future CL clients
- Clean CI, linting, and testability
- Unique port allocation for reliable tests
- Explicit temp directory cleanup helpers

## 🛠️ Getting Started

```bash
git clone https://github.com/thep2p/go-eth-localnet
cd go-eth-localnet
```

```go
logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
launcher := node.NewLauncher(logger)
manager := node.NewNodeManager(logger, launcher, "./datadir", testutils.NewPort)
ctx := context.Background()
if err := manager.Start(ctx); err != nil {
    log.Fatal(err)
}
defer manager.Wait()

fmt.Println("RPC listening on", manager.Handle().RpcPort())
```

## 🗺️ Roadmap
- [x] Single Geth node (in-process)
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
