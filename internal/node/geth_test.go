package node_test

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
	"os"
	"testing"
)

// TestSingleNodeLaunch verifies that a single Geth node can be launched and
// returns a handle with an enode URL.
func TestSingleNodeLaunch(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	tmp := unittest.NewTempDir(t)
	p2pPort := unittest.NewPort(t)

	// TODO: use a config fixture if this pattern is repeated
	privateKey := unittest.PrivateKeyFixture(t)
	cfg := model.Config{
		ID:         enode.PubkeyToIDV4(&privateKey.PublicKey),
		DataDir:    tmp.Path(),
		P2PPort:    p2pPort,
		RPCPort:    unittest.NewPort(t),
		PrivateKey: privateKey,
	}

	launcher := node.NewLauncher(logger)
	gethNode, err := launcher.Launch(cfg)
	require.NoError(t, err)
	require.NotNil(t, gethNode)
	require.Contains(t, gethNode.Server().NodeInfo().Enode, "enode://")

	defer func() {
		unittest.RequireCallMustReturnWithinTimeout(
			t, func() {
				err := gethNode.Close()
				if err != nil {
					logger.Fatal().Err(err).Msg("failed to close node")
				}
				logger.Info().Msg("Node closed successfully")
				tmp.Remove()
			}, node.ShutdownTimeout, "failed to shutdown on time",
		)
	}()
}
