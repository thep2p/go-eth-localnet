package node_test

import (
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"os"
	"testing"
)

func TestSingleNodeLaunch(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	tmp := testutils.NewTempDir(t)
	port := testutils.RandomPort(t)

	cfg := node.Config{
		ID:      0,
		DataDir: tmp.Path(),
		P2PPort: port,
		RPCPort: port + 1000,
	}

	launcher := node.NewLauncher(logger)
	handle, err := launcher.Launch(cfg)
	require.NoError(t, err)
	require.NotNil(t, handle)
	require.Contains(t, handle.NodeURL(), "enode://")

	defer func() {
		err := handle.Close()
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to close node")
		}
		logger.Info().Msg("Node closed successfully")
	}()
}
