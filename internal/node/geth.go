package node

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

const (
	// StartupTimeout is the maximum time to wait for a node to start.
	StartupTimeout = 5 * time.Second
	// ShutdownTimeout is the maximum time to wait for a node to shut down.
	ShutdownTimeout = 5 * time.Second
	// OperationTimeout is the maximum time to wait for an operation to complete, e.g., RPC calls, block fetch, etc.
	OperationTimeout = 5 * time.Second
)

// Launcher starts a Geth node, parsing StaticNodes from cfg and adding them to the P2P configuration.
type Launcher struct {
	logger     zerolog.Logger
	minerCount int
}

// LaunchOption mutates the genesis block before the node starts.
type LaunchOption func(gen *core.Genesis)

// WithPreFundGenesisAccount pre-funds the given address with the provided balance.
func WithPreFundGenesisAccount(addr common.Address, bal *big.Int) LaunchOption {
	return func(gen *core.Genesis) {
		if gen.Alloc == nil {
			gen.Alloc = types.GenesisAlloc{}
		}
		acc := gen.Alloc[addr]
		if acc.Balance == nil {
			acc.Balance = new(big.Int)
		}
		acc.Balance.Set(bal)
		gen.Alloc[addr] = acc
	}
}

// NewLauncher returns a Launcher.
func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{logger: logger.With().Str("component", "node-launcher").Logger()}
}

// Launch creates, configures, and starts a Geth node with static peers.
func (l *Launcher) Launch(cfg model.Config, opts ...LaunchOption) (*node.Node, error) {
	// ensure datadir
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir datadir: %w", err)
	}

	// build P2P config
	p2pCfg := p2p.Config{
		ListenAddr:  fmt.Sprintf(":%d", cfg.P2PPort),
		PrivateKey:  cfg.PrivateKey,
		NoDiscovery: true,
		StaticNodes: make([]*enode.Node, 0, len(cfg.StaticNodes)),
		MaxPeers:    len(cfg.StaticNodes) + 1,
	}
	for _, url := range cfg.StaticNodes {
		n, err := enode.Parse(enode.ValidSchemes, url)
		if err != nil {
			return nil, fmt.Errorf("invalid static node %q: %w", url, err)
		}
		p2pCfg.StaticNodes = append(p2pCfg.StaticNodes, n)
	}

	// node config
	nodeCfg := &node.Config{
		DataDir:           cfg.DataDir,
		Name:              fmt.Sprintf("node-%s", cfg.ID.String()),
		P2P:               p2pCfg,
		HTTPHost:          "127.0.0.1",
		HTTPPort:          cfg.RPCPort,
		HTTPModules:       []string{"eth", "net", "web3", "admin"},
		UseLightweightKDF: true,
	}

	// Configure authenticated RPC for Engine API if enabled
	if cfg.EnableEngineAPI {
		nodeCfg.AuthAddr = "127.0.0.1"
		nodeCfg.AuthPort = cfg.EnginePort
		nodeCfg.AuthVirtualHosts = []string{"localhost"}
		nodeCfg.JWTSecret = cfg.JWTSecretPath
	}

	stack, err := node.New(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}

	// Creates a genesis block for a development network.
	// Setting the gas limit to 30 million which is typical for Ethereum blocks.
	genesis := core.DeveloperGenesisBlock(30_000_000, nil)
	for _, opt := range opts {
		opt(genesis)
	}
	ethCfg := &ethconfig.Config{
		// Network Ids are used to differentiate between different Ethereum networks.
		// The mainnet uses 1, and private networks often use 1337.
		NetworkId: localNetChainID,
		// Creates a genesis block for a development network.
		// Setting the gas limit to 30 million which is typical for Ethereum blocks.
		Genesis:  genesis,
		SyncMode: ethconfig.FullSync,
	}
	ethService, err := eth.New(stack, ethCfg)
	if err != nil {

		return nil, fmt.Errorf("attach eth: %w", err)
	}

	var (
		simBeacon *catalyst.SimulatedBeacon
		beaconErr error
	)
	if cfg.Mine {
		l.minerCount++
		if l.minerCount > 1 {
			l.logger.Warn().Int("miner_count", l.minerCount).Msg("multiple miners detected - only one should produce blocks to avoid conflicts")
		}
		simBeacon, beaconErr = catalyst.NewSimulatedBeacon(1, common.Address{}, ethService)
	} else {
		simBeacon, beaconErr = catalyst.NewSimulatedBeacon(0, common.Address{}, ethService)
	}
	if beaconErr != nil {
		return nil, fmt.Errorf("simulated beacon: %w", beaconErr)
	}
	catalyst.RegisterSimulatedBeaconAPIs(stack, simBeacon)
	if err := catalyst.Register(stack, ethService); err != nil {
		return nil, fmt.Errorf("register catalyst: %w", err)
	}
	stack.RegisterLifecycle(simBeacon)
	if err := stack.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}

	if cfg.Mine {
		ethService.SetSynced()
	}

	l.logger.Info().Str("enode", stack.Server().NodeInfo().Enode).Str(
		"id",
		cfg.ID.String(),
	).Msg("Node started")
	return stack, nil
}
