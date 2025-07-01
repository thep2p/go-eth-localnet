package node

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

// Launcher starts a Geth node, parsing StaticNodes from cfg and adding them to the P2P configuration.
type Launcher struct {
	logger       zerolog.Logger
	minerStarted bool
}

// NewLauncher returns a Launcher.
func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{logger: logger.With().Str("component", "node-launcher").Logger()}
}

// Launch creates, configures, and starts a Geth node with static peers.
func (l *Launcher) Launch(cfg model.Config) (*model.Handle, error) {
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

	stack, err := node.New(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}
	ethCfg := &ethconfig.Config{
		// Network Ids are used to differentiate between different Ethereum networks.
		// The mainnet uses 1, and private networks often use 1337.
		NetworkId: 1337,
		// Creates a genesis block for a development network.
		// Setting the gas limit to 30 million which is typical for Ethereum blocks.
		Genesis:  core.DeveloperGenesisBlock(30_000_000, nil),
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
		if l.minerStarted {
			l.logger.Fatal().Msg("multiple miners are not supported")
		}
		l.minerStarted = true
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
	return model.NewHandle(stack, stack.Server().NodeInfo().Enode, cfg), nil
}
