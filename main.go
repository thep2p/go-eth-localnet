package main

import (
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/node"
)

func main() {
	// Creates a "node 0" directory to store node data, e.g., chain data, key.
	dataDir := filepath.Join(".", "node0")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("failed to create datadir: %v", err)
	}

	stack, err := node.New(&node.Config{
		DataDir: dataDir,
		Name:    "geth-local-node",
	})
	if err != nil {
		log.Fatalf("failed to create node: %v", err)
	}

	_, err = eth.New(stack, &ethconfig.Config{
		// This is a custom network ID for local development.
		// This makes the node not connect to the mainnet or testnets,
		// rather it is a local singleton network.
		// Mainnet: 1, Ropsten: 3, Rinkeby: 4, Goerli: 5, Sepolia: 11155111
		NetworkId: 1337,
	})
	if err != nil {
		log.Fatalf("failed to create eth service: %v", err)
	}

	if err := stack.Start(); err != nil {
		log.Fatalf("failed to start node: %v", err)
	}
	defer stack.Close()

	log.Println("✅ Geth node is running")
	time.Sleep(30 * time.Second)
}
