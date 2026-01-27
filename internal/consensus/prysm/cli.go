// Package prysm provides integration with the Prysm consensus layer client.
package prysm

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/urfave/cli/v2"

	"github.com/prysmaticlabs/prysm/v5/cmd"
	beaconflags "github.com/prysmaticlabs/prysm/v5/cmd/beacon-chain/flags"
)

// buildCLIContext constructs a cli.Context from consensus.Config for Prysm BeaconNode.
//
// Maps consensus.Config fields to Prysm command-line flags. The context is used
// to initialize a Prysm BeaconNode which requires urfave/cli for configuration.
//
// Args:
//   - cfg: consensus configuration containing network settings
//   - genesisStatePath: path to SSZ-encoded genesis state file
//
// Returns cli.Context configured for Prysm beacon node initialization.
//
// All errors are CRITICAL and indicate the CLI context cannot be built.
func buildCLIContext(cfg consensus.Config, genesisStatePath string) (*cli.Context, error) {
	// Create flag set with all required Prysm flags
	set := flag.NewFlagSet("beacon-chain", flag.ContinueOnError)

	// Register flags on the flag set
	registerFlags(set)

	// Set flag values from config
	if err := setFlagValues(set, cfg, genesisStatePath); err != nil {
		return nil, fmt.Errorf("set flag values: %w", err)
	}

	// Build cli.App and Context
	app := buildApp()
	return cli.NewContext(app, set, nil), nil
}

// genesisStateFlagName is the flag name for genesis state path in Prysm.
const genesisStateFlagName = "genesis-state"

// registerFlags registers all required Prysm flags on the flag set.
func registerFlags(set *flag.FlagSet) {
	// Data directory
	set.String(cmd.DataDirFlag.Name, "", cmd.DataDirFlag.Usage)

	// Genesis state
	set.String(genesisStateFlagName, "", "Load a genesis state from ssz file")

	// Execution layer connection
	set.String(beaconflags.ExecutionEngineEndpoint.Name, "", beaconflags.ExecutionEngineEndpoint.Usage)
	set.String(beaconflags.ExecutionJWTSecretFlag.Name, "", beaconflags.ExecutionJWTSecretFlag.Usage)

	// Network ports
	set.Int(beaconflags.RPCPort.Name, 4000, beaconflags.RPCPort.Usage)
	set.Int(beaconflags.HTTPServerPort.Name, 3500, beaconflags.HTTPServerPort.Usage)
	set.Int(cmd.P2PUDPPort.Name, 12000, cmd.P2PUDPPort.Usage)
	set.Int(cmd.P2PTCPPort.Name, 13000, cmd.P2PTCPPort.Usage)
	set.Int(cmd.P2PQUICPort.Name, 13000, cmd.P2PQUICPort.Usage)
	set.Int(beaconflags.MonitoringPortFlag.Name, 8080, beaconflags.MonitoringPortFlag.Usage)

	// P2P configuration
	set.Bool(cmd.NoDiscovery.Name, false, cmd.NoDiscovery.Usage)
	set.Int(beaconflags.MinSyncPeers.Name, 3, beaconflags.MinSyncPeers.Usage)

	// Interop mode for testing without real execution layer
	set.Bool(beaconflags.InteropMockEth1DataVotesFlag.Name, false, beaconflags.InteropMockEth1DataVotesFlag.Usage)

	// Accept terms of service (required for non-interactive)
	set.Bool(cmd.AcceptTosFlag.Name, false, cmd.AcceptTosFlag.Usage)

	// Force clear DB on startup
	set.Bool(cmd.ForceClearDB.Name, false, cmd.ForceClearDB.Usage)

	// Disable monitoring for local testing
	set.Bool(cmd.DisableMonitoringFlag.Name, false, cmd.DisableMonitoringFlag.Usage)

	// Minimal config for local testing (faster epoch times, less validators needed)
	set.Bool(cmd.MinimalConfigFlag.Name, false, cmd.MinimalConfigFlag.Usage)

	// Test flag to skip execution layer connection (for unit testing)
	set.Bool("test-skip-pow", false, "Skip execution layer connection for testing")
}

// setFlagValues sets flag values from consensus.Config.
func setFlagValues(set *flag.FlagSet, cfg consensus.Config, genesisStatePath string) error {
	// Write JWT secret to file and get path
	jwtPath, err := writeJWTSecret(cfg)
	if err != nil {
		return fmt.Errorf("write jwt secret: %w", err)
	}

	// String flags
	stringFlags := []struct {
		name  string
		value string
	}{
		{cmd.DataDirFlag.Name, cfg.DataDir},
		{genesisStateFlagName, genesisStatePath},
		{beaconflags.ExecutionEngineEndpoint.Name, cfg.EngineEndpoint},
		{beaconflags.ExecutionJWTSecretFlag.Name, jwtPath},
	}

	for _, f := range stringFlags {
		if err := set.Set(f.name, f.value); err != nil {
			return fmt.Errorf("set %s: %w", f.name, err)
		}
	}

	// Integer flags
	intFlags := []struct {
		name  string
		value int
	}{
		{beaconflags.RPCPort.Name, cfg.RPCPort},
		{beaconflags.HTTPServerPort.Name, cfg.BeaconPort},
		{cmd.P2PUDPPort.Name, cfg.P2PPort},
		{cmd.P2PTCPPort.Name, cfg.P2PPort + 1},
		{cmd.P2PQUICPort.Name, cfg.P2PPort + 2},
		{beaconflags.MinSyncPeers.Name, 0}, // No minimum peers for local testing
	}

	for _, f := range intFlags {
		if err := set.Set(f.name, fmt.Sprintf("%d", f.value)); err != nil {
			return fmt.Errorf("set %s: %w", f.name, err)
		}
	}

	// Boolean flags (set to true)
	boolFlags := []string{
		cmd.NoDiscovery.Name,
		beaconflags.InteropMockEth1DataVotesFlag.Name,
		cmd.AcceptTosFlag.Name,
		cmd.ForceClearDB.Name,
		cmd.DisableMonitoringFlag.Name,
		cmd.MinimalConfigFlag.Name,
		"test-skip-pow", // Skip execution layer for testing
	}

	for _, name := range boolFlags {
		if err := set.Set(name, "true"); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}

	return nil
}

// buildApp creates a minimal cli.App for context construction.
func buildApp() *cli.App {
	return &cli.App{
		Name: "beacon-chain",
		Flags: []cli.Flag{
			cmd.DataDirFlag,
			&cli.PathFlag{
				Name:  genesisStateFlagName,
				Usage: "Load a genesis state from ssz file",
			},
			beaconflags.ExecutionEngineEndpoint,
			beaconflags.ExecutionJWTSecretFlag,
			beaconflags.RPCPort,
			beaconflags.HTTPServerPort,
			cmd.P2PUDPPort,
			cmd.P2PTCPPort,
			cmd.P2PQUICPort,
			beaconflags.MonitoringPortFlag,
			cmd.NoDiscovery,
			beaconflags.MinSyncPeers,
			beaconflags.InteropMockEth1DataVotesFlag,
			cmd.AcceptTosFlag,
			cmd.ForceClearDB,
			cmd.DisableMonitoringFlag,
			cmd.MinimalConfigFlag,
		},
	}
}

// writeJWTSecret writes the JWT secret to a hex-encoded file in the data directory.
//
// Prysm expects the JWT secret as a path to a file containing the hex-encoded secret.
//
// Returns the path to the JWT secret file.
//
// All errors are CRITICAL and indicate the JWT file cannot be written.
func writeJWTSecret(cfg consensus.Config) (string, error) {
	jwtPath := filepath.Join(cfg.DataDir, "jwt.hex")
	hexSecret := fmt.Sprintf("%x", cfg.JWTSecret)
	if err := os.WriteFile(jwtPath, []byte(hexSecret), 0600); err != nil {
		return "", fmt.Errorf("write jwt file: %w", err)
	}
	return jwtPath, nil
}
