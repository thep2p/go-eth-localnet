package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/node"
)

func main() {
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

	_, err = eth.New(stack, &eth.Config{
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
