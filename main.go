package main

import (
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/rs/zerolog"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/node"
)

func main() {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	// Creates a "node 0" directory to store node data, e.g., chain data, key.
	dataDir := filepath.Join(".", "node0")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Fatal().Err(err).Msg("failed to create data directory")
	}

	stack, err := node.New(&node.Config{
		DataDir: dataDir,
		Name:    "geth-local-node",
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create node")
	}

	_, err = eth.New(stack, &ethconfig.Config{
		// This is a custom network ID for local development.
		// This makes the node not connect to the mainnet or testnets,
		// rather it is a local singleton network.
		// Mainnet: 1, Ropsten: 3, Rinkeby: 4, Goerli: 5, Sepolia: 11155111
		NetworkId: 1337,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create eth service")
	}

	if err := stack.Start(); err != nil {
		logger.Fatal().Err(err).Msg("failed to start node")
	}
	defer func() {
		if err := stack.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close node")
		}
	}()

	logger.Info().Msg("✅ Geth node is running")
	time.Sleep(30 * time.Second)
}
